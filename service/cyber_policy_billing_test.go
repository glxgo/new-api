package service

import (
	"net/http/httptest"
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func TestCalculateCyberPolicyOrderSummaryChargesEstimatedInput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	relayInfo := &relaycommon.RelayInfo{
		OriginModelName: "gpt-test",
		StartTime:       time.Now(),
		ChannelMeta:     &relaycommon.ChannelMeta{},
		PriceData: types.PriceData{
			ModelRatio:      2,
			CompletionRatio: 3,
			GroupRatioInfo: types.GroupRatioInfo{
				GroupRatio: 1.5,
			},
		},
	}
	relayInfo.SetEstimatePromptTokens(100)

	summary, tiered, result := calculateCyberPolicyOrderSummary(ctx, relayInfo)
	if tiered || result != nil {
		t.Fatalf("unexpected tiered result: applied=%v result=%+v", tiered, result)
	}
	if summary.PromptTokens != 100 || summary.CompletionTokens != 0 || summary.Quota != 300 {
		t.Fatalf("summary = %+v, want prompt=100 completion=0 quota=300", summary)
	}
}
