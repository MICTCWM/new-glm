package types

import (
	"errors"
	"net/http"
	"testing"
)

func TestSetMessageOverridesOpenAIRelayErrorMessage(t *testing.T) {
	apiErr := WithOpenAIError(OpenAIError{
		Message: "upstream raw detail",
		Type:    "upstream_error",
		Code:    "bad_response",
	}, http.StatusBadGateway)

	apiErr.SetMessage("friendly message")

	if got := apiErr.Error(); got != "friendly message" {
		t.Fatalf("Error() = %q", got)
	}
	if got := apiErr.ToOpenAIError().Message; got != "friendly message" {
		t.Fatalf("ToOpenAIError().Message = %q", got)
	}
	if got := apiErr.ToClaudeError().Message; got != "friendly message" {
		t.Fatalf("ToClaudeError().Message = %q", got)
	}
}

func TestSetMessageOverridesClaudeRelayErrorMessage(t *testing.T) {
	apiErr := WithClaudeError(ClaudeError{
		Message: "claude raw detail",
		Type:    "api_error",
	}, http.StatusBadGateway)

	apiErr.SetMessage("friendly message")

	if got := apiErr.Error(); got != "friendly message" {
		t.Fatalf("Error() = %q", got)
	}
	if got := apiErr.ToClaudeError().Message; got != "friendly message" {
		t.Fatalf("ToClaudeError().Message = %q", got)
	}
	if got := apiErr.ToOpenAIError().Message; got != "friendly message" {
		t.Fatalf("ToOpenAIError().Message = %q", got)
	}
}

func TestSetMessageOverridesNewAPIErrorMessage(t *testing.T) {
	apiErr := NewError(errors.New("internal raw detail"), ErrorCodeBadResponse)

	apiErr.SetMessage("friendly message")

	if got := apiErr.ToOpenAIError().Message; got != "friendly message" {
		t.Fatalf("ToOpenAIError().Message = %q", got)
	}
}
