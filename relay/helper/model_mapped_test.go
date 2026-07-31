package helper

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
)

func TestModelMappedHelperSupportsReasoningEffortOnObjectTarget(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("model_mapping", `{"client-model":{"model":"fallback-model","reasoning_effort":"xhigh"}}`)

	info := &relaycommon.RelayInfo{
		OriginModelName: "client-model",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeOpenAI,
			UpstreamModelName: "client-model",
		},
	}
	request := &dto.GeneralOpenAIRequest{}

	if err := ModelMappedHelper(c, info, request); err != nil {
		t.Fatalf("ModelMappedHelper() error = %v", err)
	}
	if info.UpstreamModelName != "fallback-model" {
		t.Fatalf("UpstreamModelName = %q, want fallback-model", info.UpstreamModelName)
	}
	if info.MappedReasoningEffort != "xhigh" {
		t.Fatalf("MappedReasoningEffort = %q, want xhigh", info.MappedReasoningEffort)
	}
	if request.Model != "fallback-model" {
		t.Fatalf("request.Model = %q, want fallback-model", request.Model)
	}
}

func TestModelMappedHelperKeepsLegacyStringTarget(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("model_mapping", `{"client-model":"fallback-model"}`)

	info := &relaycommon.RelayInfo{
		OriginModelName: "client-model",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeOpenAI,
			UpstreamModelName: "client-model",
		},
	}

	if err := ModelMappedHelper(c, info, nil); err != nil {
		t.Fatalf("ModelMappedHelper() error = %v", err)
	}
	if info.UpstreamModelName != "fallback-model" {
		t.Fatalf("UpstreamModelName = %q, want fallback-model", info.UpstreamModelName)
	}
	if info.MappedReasoningEffort != "" {
		t.Fatalf("MappedReasoningEffort = %q, want empty", info.MappedReasoningEffort)
	}
}

func TestModelMappedHelperRejectsInvalidMappingReasoningEffort(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("model_mapping", `{"client-model":{"model":"fallback-model","reasoning_effort":"turbo"}}`)

	info := &relaycommon.RelayInfo{
		OriginModelName: "client-model",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeOpenAI,
			UpstreamModelName: "client-model",
		},
	}

	if err := ModelMappedHelper(c, info, nil); err == nil {
		t.Fatal("ModelMappedHelper() error = nil, want invalid reasoning effort error")
	}
}
