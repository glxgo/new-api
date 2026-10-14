package dto

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestGeminiCumulativeUsageDoesNotMultiplyMedia(t *testing.T) {
	frame := &GeminiUsageMetadata{PromptTokenCount: 1200, TotalTokenCount: 1210, CandidatesTokenCount: 10, PromptTokensDetails: []GeminiPromptTokensDetails{{Modality: "IMAGE", TokenCount: 1089}, {Modality: "AUDIO", TokenCount: 11}}}
	var result *GeminiUsageMetadata
	for i := 0; i < 3; i++ {
		result = MergeGeminiUsageMetadataNonZero(result, frame)
	}
	require.Equal(t, 1200, result.PromptTokenCount)
	require.Equal(t, 1089, result.PromptTokensDetails[0].TokenCount)
	result = MergeGeminiUsageMetadataNonZero(result, &GeminiUsageMetadata{CandidatesTokenCount: 20, PromptTokensDetails: []GeminiPromptTokensDetails{{Modality: "image", TokenCount: 1090}}})
	require.Equal(t, []GeminiPromptTokensDetails{{Modality: "AUDIO", TokenCount: 11}, {Modality: "image", TokenCount: 1090}}, result.PromptTokensDetails)
	require.Equal(t, 1220, result.TotalTokenCount)
	require.Equal(t, 1089, frame.PromptTokensDetails[0].TokenCount)
}
