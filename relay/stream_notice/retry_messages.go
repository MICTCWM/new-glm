package stream_notice

import "github.com/QuantumNous/new-api/common"

// retryMessages contains short, user-facing status updates. It must never
// contain internal reasoning, diagnostics, provider details, or a simulated
// chain of thought.
var retryMessages = []string{common.UserMessageRpmQueuedThinking}

// RandomRetryMessage returns a short status message suitable for a thinking
// or reasoning delta. The trailing newline keeps successive notices readable.
func RandomRetryMessage() string {
	return retryMessages[0] + "\n"
}
