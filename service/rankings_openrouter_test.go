package service

import (
	"testing"
	"time"
)

func TestBuildOpenRouterRankingsSnapshotKeepsModelHistoryYearlyForShortPeriods(t *testing.T) {
	config, err := rankingConfig("today")
	if err != nil {
		t.Fatal(err)
	}

	payload := openRouterFetchResult{
		models: openRouterModelsResponse{
			Data: []openRouterModelRow{
				{
					Date:                  "2026-06-19 00:00:00",
					ModelPermaslug:        "anthropic/claude-sonnet-4",
					VariantPermaslug:      "anthropic/claude-sonnet-4",
					TotalPromptTokens:     100,
					TotalCompletionTokens: 50,
				},
			},
		},
		chart: openRouterTimeseriesResponse{
			Data: openRouterTimeseriesData{
				Data: []openRouterTimeseriesPoint{
					{
						X: "2026-01-05",
						Ys: map[string]int64{
							"openai/gpt-4o": 300,
						},
					},
					{
						X: "2026-02-02",
						Ys: map[string]int64{
							"openai/gpt-4o": 700,
						},
					},
				},
			},
		},
	}

	snapshot := buildOpenRouterRankingsSnapshotFromPayload(config, time.Date(2026, time.June, 19, 12, 0, 0, 0, time.UTC), payload)
	if len(snapshot.Models) == 0 || snapshot.Models[0].ModelName != "anthropic/claude-sonnet-4" {
		t.Fatalf("models should use selected-period rows, got %#v", snapshot.Models)
	}

	if snapshot.ModelsHistory.Buckets != 2 {
		t.Fatalf("model history should use yearly chart buckets, got %d", snapshot.ModelsHistory.Buckets)
	}
	for _, point := range snapshot.ModelsHistory.Points {
		if point.Model == "anthropic/claude-sonnet-4" {
			t.Fatalf("model history should not use today rows in OpenRouter short periods, got %#v", snapshot.ModelsHistory.Points)
		}
	}
}
