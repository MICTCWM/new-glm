package limiter

import (
	"context"
	_ "embed"
	"fmt"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/go-redis/redis/v8"
)

const slidingWindowScript = `
local now = redis.call('TIME')
local nowMs = tonumber(now[1]) * 1000 + math.floor(tonumber(now[2]) / 1000)
local windowMs = tonumber(ARGV[2]) * 1000
local maxCount = tonumber(ARGV[1])
if maxCount <= 0 then return {1, ''} end
redis.call('ZREMRANGEBYSCORE', KEYS[1], 0, nowMs - windowMs)
if redis.call('ZCARD', KEYS[1]) >= maxCount then return {0, ''} end
local sequence = redis.call('INCR', KEYS[2])
local member = tostring(nowMs) .. ':' .. tostring(sequence)
redis.call('ZADD', KEYS[1], nowMs, member)
redis.call('EXPIRE', KEYS[1], tonumber(ARGV[3]))
redis.call('EXPIRE', KEYS[2], tonumber(ARGV[3]))
return {1, member}
`

const slidingWindowRollbackScript = `
return redis.call('ZREM', KEYS[1], ARGV[1])
`

// AllowSlidingWindow atomically checks and reserves a fixed-window slot.
func AllowSlidingWindow(ctx context.Context, r *redis.Client, key string, maxCount int, duration int64) (string, bool, error) {
	if maxCount <= 0 {
		return "", true, nil
	}
	expiration := duration + 60
	if expiration < 60 {
		expiration = 60
	}
	result, err := r.Eval(ctx, slidingWindowScript, []string{key, key + ":seq"}, maxCount, duration, expiration).Result()
	if err != nil {
		return "", false, fmt.Errorf("sliding window rate limit failed: %w", err)
	}
	values, ok := result.([]interface{})
	if !ok || len(values) != 2 {
		return "", false, fmt.Errorf("unexpected sliding window response")
	}
	allowed, ok := values[0].(int64)
	if !ok {
		return "", false, fmt.Errorf("unexpected sliding window decision")
	}
	token, _ := values[1].(string)
	return token, allowed == 1, nil
}

func RollbackSlidingWindow(ctx context.Context, r *redis.Client, key, token string) error {
	if token == "" {
		return nil
	}
	_, err := r.Eval(ctx, slidingWindowRollbackScript, []string{key}, token).Result()
	return err
}

//go:embed lua/rate_limit.lua
var rateLimitScript string

type RedisLimiter struct {
	client         *redis.Client
	limitScriptSHA string
}

var (
	instance *RedisLimiter
	once     sync.Once
)

func New(ctx context.Context, r *redis.Client) *RedisLimiter {
	once.Do(func() {
		// 预加载脚本
		limitSHA, err := r.ScriptLoad(ctx, rateLimitScript).Result()
		if err != nil {
			common.SysLog(fmt.Sprintf("Failed to load rate limit script: %v", err))
		}
		instance = &RedisLimiter{
			client:         r,
			limitScriptSHA: limitSHA,
		}
	})

	return instance
}

func (rl *RedisLimiter) Allow(ctx context.Context, key string, opts ...Option) (bool, error) {
	// 默认配置
	config := &Config{
		Capacity:  10,
		Rate:      1,
		Requested: 1,
	}

	// 应用选项模式
	for _, opt := range opts {
		opt(config)
	}

	// 执行限流
	result, err := rl.client.EvalSha(
		ctx,
		rl.limitScriptSHA,
		[]string{key},
		config.Requested,
		config.Rate,
		config.Capacity,
	).Int()

	if err != nil {
		return false, fmt.Errorf("rate limit failed: %w", err)
	}
	return result == 1, nil
}

// Config 配置选项模式
type Config struct {
	Capacity  int64
	Rate      int64
	Requested int64
}

type Option func(*Config)

func WithCapacity(c int64) Option {
	return func(cfg *Config) { cfg.Capacity = c }
}

func WithRate(r int64) Option {
	return func(cfg *Config) { cfg.Rate = r }
}

func WithRequested(n int64) Option {
	return func(cfg *Config) { cfg.Requested = n }
}
