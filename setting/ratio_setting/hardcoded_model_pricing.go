package ratio_setting

// hardcodedModelPricing contains pricing that must not be overridden by the
// editable ratio/price settings. Values are expressed using the internal
// pricing unit where 1.0 == $2 per 1M input tokens.
type hardcodedModelPricing struct {
	ModelRatio      float64
	CompletionRatio float64
	CacheRatio      float64
}

var hardcodedModelPricingMap = map[string]hardcodedModelPricing{
	// GPT-5.6 Sol: $5 input, $0.50 cached input, $30 output per 1M tokens.
	"gpt-5.6-sol": {ModelRatio: 2.5, CompletionRatio: 6, CacheRatio: 0.1},
	// GPT-5.6 Terra: $2.50 input, $0.25 cached input, $15 output per 1M tokens.
	"gpt-5.6-terra": {ModelRatio: 1.25, CompletionRatio: 6, CacheRatio: 0.1},
	// GPT-5.6 Luna: $1 input, $0.10 cached input, $6 output per 1M tokens.
	"gpt-5.6-luna": {ModelRatio: 0.5, CompletionRatio: 6, CacheRatio: 0.1},
}

func getHardcodedModelPricing(name string) (hardcodedModelPricing, bool) {
	pricing, ok := hardcodedModelPricingMap[FormatMatchingModelName(name)]
	return pricing, ok
}

// IsHardcodedModelPricing reports whether a model's billing must bypass all
// editable ratio, per-request price, and tiered-expression settings.
func IsHardcodedModelPricing(name string) bool {
	_, ok := getHardcodedModelPricing(name)
	return ok
}
