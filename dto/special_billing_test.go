package dto

import "testing"

func TestResolveSpecialBillingPriceUsesInputTokenTier(t *testing.T) {
	limit := int64(272000)
	settings := ChannelSettings{
		SpecialBilling: true,
		SpecialBillingPrices: map[string][]SpecialBillingPrice{
			"gpt-4o": {{MaxInputTokens: &limit, Price: 1}, {Price: 2}},
		},
	}
	if price, ok := settings.ResolveSpecialBillingPrice("gpt-4o", 272000); !ok || price != 1 {
		t.Fatalf("expected $1 at boundary, got %v, %v", price, ok)
	}
	if price, ok := settings.ResolveSpecialBillingPrice("gpt-4o", 272001); !ok || price != 2 {
		t.Fatalf("expected $2 above boundary, got %v, %v", price, ok)
	}
}

func TestResolveSpecialBillingPriceFallsBackWhenModelMissing(t *testing.T) {
	settings := ChannelSettings{SpecialBilling: true, SpecialBillingPrices: map[string][]SpecialBillingPrice{}}
	if _, ok := settings.ResolveSpecialBillingPrice("missing", 1); ok {
		t.Fatal("expected missing model to fall back")
	}
}
