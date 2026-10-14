package dto

import "strings"

func MergeGeminiUsageMetadataNonZero(current *GeminiUsageMetadata, incoming *GeminiUsageMetadata) *GeminiUsageMetadata {
	if current == nil && incoming == nil {
		return nil
	}
	if current == nil {
		metadata := cloneGeminiUsageMetadata(*incoming)
		return &metadata
	}

	merged := cloneGeminiUsageMetadata(*current)
	if incoming == nil {
		return &merged
	}
	if incoming.PromptTokenCount > 0 {
		merged.PromptTokenCount = incoming.PromptTokenCount
	}
	if incoming.ToolUsePromptTokenCount > 0 {
		merged.ToolUsePromptTokenCount = incoming.ToolUsePromptTokenCount
	}
	if incoming.CandidatesTokenCount > 0 {
		merged.CandidatesTokenCount = incoming.CandidatesTokenCount
		merged.ThoughtsTokenCount = incoming.ThoughtsTokenCount
	} else if incoming.ThoughtsTokenCount > 0 {
		merged.ThoughtsTokenCount = incoming.ThoughtsTokenCount
	}
	if incoming.TotalTokenCount > 0 {
		merged.TotalTokenCount = incoming.TotalTokenCount
	}
	if incoming.CachedContentTokenCount > 0 {
		merged.CachedContentTokenCount = incoming.CachedContentTokenCount
	}
	merged.PromptTokensDetails = mergeGeminiTokenDetails(merged.PromptTokensDetails, incoming.PromptTokensDetails)
	merged.ToolUsePromptTokensDetails = mergeGeminiTokenDetails(merged.ToolUsePromptTokensDetails, incoming.ToolUsePromptTokensDetails)
	merged.CandidatesTokensDetails = mergeGeminiTokenDetails(merged.CandidatesTokensDetails, incoming.CandidatesTokensDetails)
	if total := merged.PromptTokenCount + merged.ToolUsePromptTokenCount + merged.CandidatesTokenCount + merged.ThoughtsTokenCount; total > merged.TotalTokenCount {
		merged.TotalTokenCount = total
	}
	return &merged
}

func normalizeGeminiModality(modality string) string {
	return strings.ToUpper(strings.TrimSpace(modality))
}

// mergeGeminiTokenDetails overlays per-modality counts the same way as the
// scalar counts: every stream frame carries a cumulative snapshot, so a
// modality the incoming frame reports replaces all earlier entries for it,
// and a modality it omits keeps its earlier value. Duplicate entries inside
// one snapshot are kept as-is; settlement sums them.
func mergeGeminiTokenDetails(current []GeminiPromptTokensDetails, incoming []GeminiPromptTokensDetails) []GeminiPromptTokensDetails {
	reported := make(map[string]bool, len(incoming))
	for _, detail := range incoming {
		if detail.TokenCount > 0 {
			reported[normalizeGeminiModality(detail.Modality)] = true
		}
	}
	merged := make([]GeminiPromptTokensDetails, 0, len(current)+len(incoming))
	for _, detail := range current {
		if !reported[normalizeGeminiModality(detail.Modality)] {
			merged = append(merged, detail)
		}
	}
	for _, detail := range incoming {
		if detail.TokenCount > 0 {
			merged = append(merged, detail)
		}
	}
	return merged
}

func cloneGeminiUsageMetadata(in GeminiUsageMetadata) GeminiUsageMetadata {
	in.PromptTokensDetails = append([]GeminiPromptTokensDetails(nil), in.PromptTokensDetails...)
	in.ToolUsePromptTokensDetails = append([]GeminiPromptTokensDetails(nil), in.ToolUsePromptTokensDetails...)
	in.CandidatesTokensDetails = append([]GeminiPromptTokensDetails(nil), in.CandidatesTokensDetails...)
	return in
}
