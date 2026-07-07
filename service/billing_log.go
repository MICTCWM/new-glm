package service

import "strings"

const gptBillingLogPrefix = "[GPT扣费]"

// PrefixBillingLogContent adds an explicit GPT marker for logs billed from the GPT wallet.
func PrefixBillingLogContent(billingSource, content string) string {
	if billingSource != BillingSourceGptWallet {
		return content
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return gptBillingLogPrefix
	}
	if strings.HasPrefix(content, gptBillingLogPrefix) {
		return content
	}
	return gptBillingLogPrefix + " " + content
}
