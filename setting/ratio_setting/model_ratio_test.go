package ratio_setting

import "testing"

func TestGPT56CompletionRatioMatchesConfiguredSolPricing(t *testing.T) {
	for _, model := range []string{
		"gpt-5.6",
		"gpt-5.6-sol",
		"gpt-5.6-terra",
		"gpt-5.6-luna",
	} {
		ratio, locked := getHardcodedCompletionModelRatio(model)
		if ratio != 6 {
			t.Fatalf("%s completion ratio = %v, want 6", model, ratio)
		}
		if !locked {
			t.Fatalf("%s completion ratio is not locked", model)
		}
	}
}
