package ratio_setting

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGPT56PricingCannotBeOverriddenByEditableSettings(t *testing.T) {
	modelRatios := modelRatioMap.ReadAll()
	modelPrices := modelPriceMap.ReadAll()
	completionRatios := completionRatioMap.ReadAll()
	cacheRatios := cacheRatioMap.ReadAll()
	t.Cleanup(func() {
		modelRatioMap.Clear()
		modelRatioMap.AddAll(modelRatios)
		modelPriceMap.Clear()
		modelPriceMap.AddAll(modelPrices)
		completionRatioMap.Clear()
		completionRatioMap.AddAll(completionRatios)
		cacheRatioMap.Clear()
		cacheRatioMap.AddAll(cacheRatios)
	})

	for model := range hardcodedModelPricingMap {
		modelRatioMap.Set(model, 999)
		modelPriceMap.Set(model, 999)
		completionRatioMap.Set(model, 999)
		cacheRatioMap.Set(model, 999)
	}

	expected := map[string]hardcodedModelPricing{
		"gpt-5.6-sol":   {ModelRatio: 2.5, CompletionRatio: 6, CacheRatio: 0.1},
		"gpt-5.6-terra": {ModelRatio: 1.25, CompletionRatio: 6, CacheRatio: 0.1},
		"gpt-5.6-luna":  {ModelRatio: 0.5, CompletionRatio: 6, CacheRatio: 0.1},
	}

	for model, pricing := range expected {
		modelRatio, ok, matched := GetModelRatio(model)
		require.True(t, ok)
		require.Equal(t, model, matched)
		require.Equal(t, pricing.ModelRatio, modelRatio)
		require.Equal(t, pricing.CompletionRatio, GetCompletionRatio(model))
		require.Equal(t, pricing.CompletionRatio, GetCompletionRatioInfo(model).Ratio)
		require.True(t, GetCompletionRatioInfo(model).Locked)
		require.Equal(t, pricing.CacheRatio, mustGetCacheRatio(t, model))
		_, usePrice := GetModelPrice(model, false)
		require.False(t, usePrice)
	}
}

func TestGPT56PricingIsExposedWithFixedValues(t *testing.T) {
	modelRatios := GetModelRatioCopy()
	completionRatios := GetCompletionRatioCopy()
	cacheRatios := GetCacheRatioCopy()
	modelPrices := GetModelPriceCopy()

	require.Equal(t, 2.5, modelRatios["gpt-5.6-sol"])
	require.Equal(t, 1.25, modelRatios["gpt-5.6-terra"])
	require.Equal(t, 0.5, modelRatios["gpt-5.6-luna"])
	require.Equal(t, 6.0, completionRatios["gpt-5.6-sol"])
	require.Equal(t, 0.1, cacheRatios["gpt-5.6-sol"])
	_, exists := modelPrices["gpt-5.6-sol"]
	require.False(t, exists)
}

func mustGetCacheRatio(t *testing.T, model string) float64 {
	t.Helper()
	ratio, ok := GetCacheRatio(model)
	require.True(t, ok)
	return ratio
}
