package model

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
)

func TestSanitizeLogDetailRedactsUpstreamRequestOnly(t *testing.T) {
	prompt := constant.GetForceSystemPrompt("glm-5.2")
	detail := &LogDetail{
		UserRequestBody:      "user request",
		UpstreamRequestBody:  `{"instructions":"` + prompt + `\ncustom instructions"}`,
		UpstreamResponseBody: "upstream response",
	}

	sanitized := SanitizeLogDetail(detail)
	if sanitized == detail {
		t.Fatal("SanitizeLogDetail() should return a copy")
	}
	if sanitized.UpstreamRequestBody == detail.UpstreamRequestBody {
		t.Fatal("SanitizeLogDetail() did not redact the upstream prompt")
	}
	if sanitized.UserRequestBody != detail.UserRequestBody || sanitized.UpstreamResponseBody != detail.UpstreamResponseBody {
		t.Fatal("SanitizeLogDetail() changed unrelated fields")
	}
	if !strings.Contains(detail.UpstreamRequestBody, prompt) || strings.Contains(sanitized.UpstreamRequestBody, prompt) {
		t.Fatal("SanitizeLogDetail() mutated the original or left the prompt visible")
	}
}
