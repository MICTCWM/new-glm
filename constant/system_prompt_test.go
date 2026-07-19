package constant

import "testing"

func TestRedactForceSystemPrompts(t *testing.T) {
	input := "prefix\n" + GetForceSystemPrompt("glm-5.2") + "\nuser content\n" + GetForceSystemPrompt("kimi-k2.6")

	got := RedactForceSystemPrompts(input)
	if got != "prefix\n[REDACTED]\nuser content\n[REDACTED]" {
		t.Fatalf("RedactForceSystemPrompts() = %q", got)
	}
}

func TestRedactForceSystemPromptsLeavesUnrelatedText(t *testing.T) {
	const input = `{"messages":[{"role":"user","content":"hello"}]}`

	if got := RedactForceSystemPrompts(input); got != input {
		t.Fatalf("RedactForceSystemPrompts() changed unrelated text: %q", got)
	}
}
