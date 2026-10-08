package seedream

import (
	"context"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSeedreamFlashValidationAndMixedTierBilling(t *testing.T) {
	ctx := context.Background()
	model := "doubao-seedream-5-0-flash-260915"
	request, err := Decode(ctx, model, "generate", map[string]any{"kind": "json", "value": map[string]any{"prompt": "test", "size": "1.5K", "watermark": false}})
	require.NoError(t, err)
	body, estimate, err := request.Build(ctx, model)
	require.NoError(t, err)
	require.Equal(t, false, body["watermark"])
	require.EqualValues(t, 1, estimate["images_up_to_1_5k"])
	facts, err := request.ActualUsage(ctx, model, map[string]any{"data": []any{map[string]any{"url": "https://example.com/1", "size": "1024x1024"}, map[string]any{"url": "https://example.com/2", "size": "2048x2048"}}, "usage": map[string]any{"generated_images": 2}})
	require.NoError(t, err)
	require.EqualValues(t, 1, facts["images_up_to_1_5k"])
	require.EqualValues(t, 1, facts["images_above_1_5k"])
	raw, _, err := billingexpr.RunExprWithRequest(`(u("images_up_to_1_5k")*0.01+u("images_above_1_5k")*0.02)*1000000`, billingexpr.TokenParams{}, billingexpr.RequestInput{Usage: facts})
	require.NoError(t, err)
	require.InDelta(t, 30000, raw, 0.001)
	for _, value := range []map[string]any{{"prompt": "test", "n": 2}, {"prompt": "test", "stream": true}, {"prompt": "test", "layer_decomposition": true}, {"prompt": "test", "size": "8K"}} {
		_, err := Decode(ctx, model, "generate", map[string]any{"kind": "json", "value": value})
		require.Error(t, err)
	}
}
