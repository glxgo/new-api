package volcengine

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSeedreamGroupReservationAndActualPayloads(t *testing.T) {
	for _, count := range []int{2, 99} {
		writer := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(writer)
		c.Request = httptest.NewRequest("POST", "/v1/images/generations", strings.NewReader(`{"model":"alias","prompt":"test","sequential_image_generation":"auto","sequential_image_generation_options":{"max_images":4}}`))
		c.Request.Header.Set("Content-Type", "application/json")
		common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeVolcEngine)
		info := &relaycommon.RelayInfo{OriginModelName: "alias", Request: &dto.ImageRequest{Model: "alias"}, ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "doubao-seedream-5-0-lite-260128"}}
		_, err := helper.PrepareSeedreamImage(c, info)
		require.NoError(t, err)
		require.EqualValues(t, 4, helper.SeedreamReservationCount(info))
		body, err := common.Marshal(map[string]any{"data": []any{map[string]any{"url": "https://example.com/1", "size": "2048x2048"}, map[string]any{"error": map[string]any{"message": "failed"}}, map[string]any{"url": "https://example.com/2", "size": "2048x2048"}}, "usage": map[string]any{"generated_images": count}})
		require.NoError(t, err)
		response := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(string(body)))}
		_, apiErr := seedreamResponse(c, response, info)
		require.Nil(t, apiErr)
		require.EqualValues(t, 2, info.PriceData.OtherRatios["n"])
		want := 2
		if count == 99 {
			want = 4
		}
		require.EqualValues(t, want, info.BillingRequestInput.Usage["images_above_1_5k"])
	}
}
