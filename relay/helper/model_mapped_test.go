package helper

import (
	"encoding/json"
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

func TestApplyModelMappingToRawJSONPreservesUnknownFieldsAndReasoning(t *testing.T) {
	info := &relaycommon.RelayInfo{
		OriginModelName:       "client-model",
		MappedReasoningEffort: "xhigh",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeOpenAI,
			UpstreamModelName: "upstream-model",
		},
	}
	info.IsModelMapped = true

	body, err := ApplyModelMappingToRawJSON([]byte(`{"model":"client-model","messages":[],"vendor_field":{"keep":true}}`), info, false)
	if err != nil {
		t.Fatalf("ApplyModelMappingToRawJSON() error = %v", err)
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("patched body is invalid JSON: %v", err)
	}
	if string(payload["model"]) != `"upstream-model"` {
		t.Fatalf("model = %s, want upstream-model", payload["model"])
	}
	if string(payload["reasoning_effort"]) != `"xhigh"` {
		t.Fatalf("reasoning_effort = %s, want xhigh", payload["reasoning_effort"])
	}
	if string(payload["vendor_field"]) != `{"keep":true}` {
		t.Fatalf("vendor_field was not preserved: %s", payload["vendor_field"])
	}
}

func TestApplyModelMappingToRawJSONUsesResponsesReasoningShape(t *testing.T) {
	info := &relaycommon.RelayInfo{MappedReasoningEffort: "xhigh"}
	body, err := ApplyModelMappingToRawJSON([]byte(`{"model":"gpt-5"}`), info, true)
	if err != nil {
		t.Fatalf("ApplyModelMappingToRawJSON() error = %v", err)
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("patched body is invalid JSON: %v", err)
	}
	var reasoning dto.Reasoning
	if err := json.Unmarshal(payload["reasoning"], &reasoning); err != nil {
		t.Fatalf("reasoning is invalid JSON: %v", err)
	}
	if reasoning.Effort != "xhigh" || reasoning.Summary != "detailed" {
		t.Fatalf("reasoning = %+v, want xhigh/detailed", reasoning)
	}
}
