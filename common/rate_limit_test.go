package common

import "testing"

func TestInMemoryRateLimiterUnlimitedAndRollback(t *testing.T) {
	var limiter InMemoryRateLimiter
	limiter.Init(0)
	if token, allowed := limiter.RequestToken("unlimited", 0, 60); !allowed || token != 0 {
		t.Fatalf("unlimited RequestToken() = (%d, %v)", token, allowed)
	}

	limiter.Init(0)
	token, allowed := limiter.RequestToken("success", 1, 60)
	if !allowed || token == 0 {
		t.Fatalf("first RequestToken() = (%d, %v)", token, allowed)
	}
	limiter.Rollback("success", token)
	if _, allowed := limiter.RequestToken("success", 1, 60); !allowed {
		t.Fatal("a rolled-back request should not consume the slot")
	}
}
