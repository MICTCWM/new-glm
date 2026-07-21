package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// visionRouteTimeout 视觉路由调用 Kimi 的超时时间
const visionRouteTimeout = 30 * time.Second

// ShouldVisionRoute 检测是否需要视觉路由
// 条件：模型为 glm-5.2 + 协议为 OpenAI 或 Anthropic + 请求含图片
func ShouldVisionRoute(relayFormat types.RelayFormat, modelName string, request dto.Request) bool {
	if modelName != common.VisionRouteSourceModel {
		return false
	}
	if request == nil {
		return false
	}
	switch relayFormat {
	case types.RelayFormatOpenAI:
		openaiReq, ok := request.(*dto.GeneralOpenAIRequest)
		if !ok || openaiReq == nil {
			return false
		}
		return openaiRequestHasImage(openaiReq)
	case types.RelayFormatClaude:
		claudeReq, ok := request.(*dto.ClaudeRequest)
		if !ok || claudeReq == nil {
			return false
		}
		return claudeRequestHasImage(claudeReq)
	}
	return false
}

// openaiRequestHasImage 检查 OpenAI 请求是否包含图片
func openaiRequestHasImage(req *dto.GeneralOpenAIRequest) bool {
	for i := range req.Messages {
		contents := req.Messages[i].ParseContent()
		for _, c := range contents {
			if c.Type == dto.ContentTypeImageURL {
				return true
			}
		}
	}
	return false
}

// claudeRequestHasImage 检查 Claude 请求是否包含图片
func claudeRequestHasImage(req *dto.ClaudeRequest) bool {
	for i := range req.Messages {
		contents, err := req.Messages[i].ParseContent()
		if err != nil {
			continue
		}
		for _, c := range contents {
			if c.Type == "image" {
				return true
			}
		}
	}
	return false
}

// ProcessVisionRoute 执行视觉路由：复用缓存的图片描述，仅将未命中的图片发送给 Kimi，
// 然后替换原请求中的图片位置。如果 Kimi 调用失败，返回错误，由调用方进行降级处理
func ProcessVisionRoute(c *gin.Context, relayInfo *relaycommon.RelayInfo, request dto.Request) (dto.Request, error) {
	switch relayInfo.RelayFormat {
	case types.RelayFormatOpenAI:
		openaiReq, ok := request.(*dto.GeneralOpenAIRequest)
		if !ok || openaiReq == nil {
			return request, fmt.Errorf("invalid openai request for vision route")
		}
		return processOpenAIVisionRoute(c, relayInfo, openaiReq)
	case types.RelayFormatClaude:
		claudeReq, ok := request.(*dto.ClaudeRequest)
		if !ok || claudeReq == nil {
			return request, fmt.Errorf("invalid claude request for vision route")
		}
		return processClaudeVisionRoute(c, relayInfo, claudeReq)
	}
	return request, fmt.Errorf("unsupported relay format for vision route: %s", relayInfo.RelayFormat)
}

// processOpenAIVisionRoute 处理 OpenAI 格式的视觉路由
func processOpenAIVisionRoute(c *gin.Context, relayInfo *relaycommon.RelayInfo, req *dto.GeneralOpenAIRequest) (*dto.GeneralOpenAIRequest, error) {
	// 收集所有图片内容
	var imageContents []dto.MediaContent
	for i := range req.Messages {
		contents := req.Messages[i].ParseContent()
		for _, content := range contents {
			if content.Type == dto.ContentTypeImageURL {
				imageContents = append(imageContents, content)
			}
		}
	}
	if len(imageContents) == 0 {
		return req, nil
	}

	descriptions, created, hits, err := resolveVisionImageDescriptions(c, relayInfo, imageContents)
	if err != nil {
		return req, err
	}
	relayInfo.VisionRouteCacheCreated += created
	relayInfo.VisionRouteCacheHits += hits

	imageIndex := 0
	for i := range req.Messages {
		contents := req.Messages[i].ParseContent()
		if len(contents) == 0 {
			continue
		}
		modified := false
		for j := range contents {
			if contents[j].Type != dto.ContentTypeImageURL {
				continue
			}
			if imageIndex >= len(descriptions) {
				return req, fmt.Errorf("vision route description count mismatch")
			}
			contents[j] = dto.MediaContent{
				Type: dto.ContentTypeText,
				Text: "[图片描述] " + descriptions[imageIndex],
			}
			imageIndex++
			modified = true
		}
		if modified {
			req.Messages[i].SetMediaContent(contents)
		}
	}
	return req, nil
}

// processClaudeVisionRoute 处理 Claude 格式的视觉路由
func processClaudeVisionRoute(c *gin.Context, relayInfo *relaycommon.RelayInfo, req *dto.ClaudeRequest) (*dto.ClaudeRequest, error) {
	// 收集所有图片内容（转换为 OpenAI 格式以便发送给 Kimi）
	var imageContents []dto.MediaContent
	for i := range req.Messages {
		contents, err := req.Messages[i].ParseContent()
		if err != nil {
			continue
		}
		for _, content := range contents {
			if content.Type == "image" {
				mediaContent, ok := claudeImageToOpenAI(content)
				if !ok {
					continue
				}
				imageContents = append(imageContents, mediaContent)
			}
		}
	}
	if len(imageContents) == 0 {
		return req, nil
	}

	descriptions, created, hits, err := resolveVisionImageDescriptions(c, relayInfo, imageContents)
	if err != nil {
		return req, err
	}
	relayInfo.VisionRouteCacheCreated += created
	relayInfo.VisionRouteCacheHits += hits

	imageIndex := 0
	for i := range req.Messages {
		contents, err := req.Messages[i].ParseContent()
		if err != nil || len(contents) == 0 {
			continue
		}
		modified := false
		for j := range contents {
			if contents[j].Type != "image" {
				continue
			}
			if _, ok := claudeImageToOpenAI(contents[j]); !ok {
				continue
			}
			if imageIndex >= len(descriptions) {
				return req, fmt.Errorf("vision route description count mismatch")
			}
			text := "[图片描述] " + descriptions[imageIndex]
			contents[j] = dto.ClaudeMediaMessage{
				Type: dto.ContentTypeText,
				Text: &text,
			}
			imageIndex++
			modified = true
		}
		if modified {
			req.Messages[i].SetContent(contents)
		}
	}
	return req, nil
}

type visionRouteDescriptionResult struct {
	description string
	created     bool
}

// resolveVisionImageDescriptions resolves each image independently. This keeps a
// cached description reusable even when the editor sends a different message around it.
func resolveVisionImageDescriptions(c *gin.Context, relayInfo *relaycommon.RelayInfo, imageContents []dto.MediaContent) ([]string, int, int, error) {
	descriptions := make([]string, 0, len(imageContents))
	resolved := make(map[string]string)
	created := 0
	hits := 0

	for _, imageContent := range imageContents {
		key := visionRouteImageCacheKey(imageContent)
		if key != "" {
			if description, ok := resolved[key]; ok {
				descriptions = append(descriptions, description)
				continue
			}
			if description, ok := getVisionRouteCache(key, time.Now()); ok {
				resolved[key] = description
				descriptions = append(descriptions, description)
				hits++
				continue
			}
		}

		if key == "" {
			description, err := callKimiForImageDescription(c, relayInfo, []dto.MediaContent{imageContent})
			if err != nil {
				return nil, 0, 0, err
			}
			descriptions = append(descriptions, description)
			created++
			continue
		}

		result, err, shared := visionRouteDescriptionGroup.Do(key, func() (interface{}, error) {
			// Recheck after entering singleflight: another request may have filled
			// the cache just before this request joined the group.
			if description, ok := getVisionRouteCache(key, time.Now()); ok {
				return visionRouteDescriptionResult{description: description}, nil
			}
			description, err := callKimiForImageDescription(c, relayInfo, []dto.MediaContent{imageContent})
			if err != nil {
				return nil, err
			}
			putVisionRouteCache(key, description, time.Now())
			return visionRouteDescriptionResult{description: description, created: true}, nil
		})
		if err != nil {
			return nil, 0, 0, err
		}
		cacheResult, ok := result.(visionRouteDescriptionResult)
		if !ok || cacheResult.description == "" {
			return nil, 0, 0, fmt.Errorf("invalid vision route cache result")
		}
		resolved[key] = cacheResult.description
		descriptions = append(descriptions, cacheResult.description)
		if cacheResult.created && !shared {
			created++
		} else {
			hits++
		}
	}
	return descriptions, created, hits, nil
}

// visionRouteImageCacheKey hashes the image reference without retaining the image bytes.
func visionRouteImageCacheKey(content dto.MediaContent) string {
	image := content.GetImageMedia()
	if image == nil || image.Url == "" {
		return ""
	}
	digest := sha256.Sum256([]byte(image.MimeType + "\x00" + image.Url))
	return hex.EncodeToString(digest[:])
}

// claudeImageToOpenAI 将 Claude 图片内容转换为 OpenAI 格式
// 返回值 bool 表示是否成功转换，失败时调用方应跳过该项
func claudeImageToOpenAI(c dto.ClaudeMediaMessage) (dto.MediaContent, bool) {
	if c.Source == nil {
		return dto.MediaContent{}, false
	}
	url := c.Source.Url
	if url == "" && c.Source.Type == "base64" {
		data := common.Interface2String(c.Source.Data)
		if data != "" {
			mediaType := c.Source.MediaType
			if mediaType == "" {
				mediaType = "image/png"
			}
			url = "data:" + mediaType + ";base64," + data
		}
	}
	// 如果 url 为空且不是 base64，返回 false
	if url == "" && c.Source.Type != "base64" {
		return dto.MediaContent{}, false
	}
	return dto.MediaContent{
		Type: dto.ContentTypeImageURL,
		ImageUrl: &dto.MessageImageUrl{
			Url:    url,
			Detail: "high",
		},
	}, true
}

// callKimiForImageDescription 调用 Kimi 模型描述图片
// 内部调用不扣费，扣费在 glm-5.2 请求结算时统一处理
func callKimiForImageDescription(c *gin.Context, relayInfo *relaycommon.RelayInfo, imageContents []dto.MediaContent) (string, error) {
	// 选择 Kimi 渠道
	group := relayInfo.UsingGroup
	if group == "" {
		group = relayInfo.UserGroup
	}
	channel, err := model.GetRandomSatisfiedChannel(group, common.VisionRouteTargetModel, 0, nil, relayInfo.UserId)
	if err != nil {
		return "", fmt.Errorf("failed to get kimi channel: %w", err)
	}
	if channel == nil {
		return "", fmt.Errorf("no available kimi channel for model %s in group %s", common.VisionRouteTargetModel, group)
	}

	// 获取 API key
	key, _, keyErr := channel.GetNextEnabledKey()
	if keyErr != nil {
		return "", fmt.Errorf("no available kimi channel key: %v", keyErr)
	}
	if key == "" {
		return "", fmt.Errorf("kimi channel key is empty")
	}

	logger.LogInfo(c, fmt.Sprintf("vision route: calling kimi channel %d for image description", channel.Id))

	// 构造 OpenAI 格式请求（提示词 + 所有图片）
	contentList := []dto.MediaContent{
		{
			Type: dto.ContentTypeText,
			Text: "让你描述这个图片，让无视觉能力模型更好的理解",
		},
	}
	contentList = append(contentList, imageContents...)

	stream := false
	maxTokens := uint(1000)
	kimiReq := &dto.GeneralOpenAIRequest{
		Model: common.VisionRouteTargetModel,
		Messages: []dto.Message{
			{
				Role:    "user",
				Content: contentList,
			},
		},
		Stream:    &stream,
		MaxTokens: &maxTokens,
	}

	reqBody, err := common.Marshal(kimiReq)
	if err != nil {
		return "", fmt.Errorf("failed to marshal kimi request: %w", err)
	}

	// 构造请求 URL
	baseURL := channel.GetBaseURL()
	if baseURL == "" {
		return "", fmt.Errorf("kimi channel base url is empty")
	}
	requestURL := strings.TrimRight(baseURL, "/") + "/v1/chat/completions"

	// 创建 HTTP 请求
	httpReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, requestURL, bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create kimi request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+key)

	// 发送请求（带超时）
	client := &http.Client{
		Timeout: visionRouteTimeout,
	}
	start := time.Now()
	resp, err := client.Do(httpReq)
	if err != nil {
		logger.LogError(c, fmt.Sprintf("vision route: kimi call failed: %v", err))
		return "", fmt.Errorf("failed to call kimi: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		logger.LogError(c, fmt.Sprintf("vision route: kimi returned non-200 status: %d, body: %s", resp.StatusCode, string(body)))
		return "", fmt.Errorf("kimi returned non-200 status: %d, body: %s", resp.StatusCode, string(body))
	}

	// 解析响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read kimi response: %w", err)
	}

	var kimiResp dto.OpenAITextResponse
	if err := common.Unmarshal(body, &kimiResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal kimi response: %w", err)
	}

	if len(kimiResp.Choices) == 0 {
		return "", fmt.Errorf("kimi returned no choices")
	}

	description := kimiResp.Choices[0].Message.StringContent()
	if description == "" {
		return "", fmt.Errorf("kimi returned empty description")
	}

	elapsed := time.Since(start)
	logger.LogInfo(c, fmt.Sprintf("vision route: kimi responded in %s, description length: %d", elapsed, len(description)))

	return description, nil
}

// StripImagesFromRequest 降级处理：丢弃请求中的所有图片，仅保留文本部分
func StripImagesFromRequest(relayFormat types.RelayFormat, request dto.Request) dto.Request {
	switch relayFormat {
	case types.RelayFormatOpenAI:
		openaiReq, ok := request.(*dto.GeneralOpenAIRequest)
		if !ok || openaiReq == nil {
			return request
		}
		return stripOpenAIImages(openaiReq)
	case types.RelayFormatClaude:
		claudeReq, ok := request.(*dto.ClaudeRequest)
		if !ok || claudeReq == nil {
			return request
		}
		return stripClaudeImages(claudeReq)
	}
	return request
}

// stripOpenAIImages 移除 OpenAI 请求中的所有图片
func stripOpenAIImages(req *dto.GeneralOpenAIRequest) *dto.GeneralOpenAIRequest {
	for i := range req.Messages {
		contents := req.Messages[i].ParseContent()
		if len(contents) == 0 {
			continue
		}
		var filtered []dto.MediaContent
		for _, content := range contents {
			if content.Type != dto.ContentTypeImageURL {
				filtered = append(filtered, content)
			}
		}
		// 只有当有图片被移除时才重新设置
		if len(filtered) != len(contents) {
			if len(filtered) == 0 {
				req.Messages[i].SetStringContent("")
			} else {
				req.Messages[i].SetMediaContent(filtered)
			}
		}
	}
	return req
}

// stripClaudeImages 移除 Claude 请求中的所有图片
func stripClaudeImages(req *dto.ClaudeRequest) *dto.ClaudeRequest {
	for i := range req.Messages {
		contents, err := req.Messages[i].ParseContent()
		if err != nil || len(contents) == 0 {
			continue
		}
		var filtered []dto.ClaudeMediaMessage
		for _, content := range contents {
			if content.Type != "image" {
				filtered = append(filtered, content)
			}
		}
		if len(filtered) != len(contents) {
			if len(filtered) == 0 {
				req.Messages[i].SetStringContent("")
			} else {
				req.Messages[i].SetContent(filtered)
			}
		}
	}
	return req
}

// CalcVisionRouteFeeQuota 计算单张未命中缓存的视觉路由扣费金额。
func CalcVisionRouteFeeQuota(groupRatio float64) int {
	return calcVisionRouteFeeQuota(groupRatio, 1, 0)
}

// CalcVisionRouteFeeQuotaByCache charges each newly described image at the full
// route fee and each cached image at the cheaper GLM-only fee.
func CalcVisionRouteFeeQuotaByCache(groupRatio float64, created, hits int) int {
	if created <= 0 && hits <= 0 {
		created = 1
	}
	return calcVisionRouteFeeQuota(groupRatio, created, hits)
}

func calcVisionRouteFeeQuota(groupRatio float64, created, hits int) int {
	if groupRatio <= 0 || (created <= 0 && hits <= 0) {
		return 0
	}
	fee := decimal.NewFromFloat(common.VisionRouteFixedFee).Mul(decimal.NewFromInt(int64(created)))
	if hits > 0 {
		fee = fee.Add(decimal.NewFromFloat(common.VisionRouteCacheHitFee).Mul(decimal.NewFromInt(int64(hits))))
	}
	quota := fee.
		Mul(decimal.NewFromFloat(common.QuotaPerUnit)).
		Mul(decimal.NewFromFloat(groupRatio)).
		Round(0).
		IntPart()
	if quota <= 0 {
		return 0
	}
	return int(quota)
}
