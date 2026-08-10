package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

var tokenUsageRateLimitTestSequence uint64

func TestTokenUsageRateLimitDoesNotConsumeCriticalBucket(t *testing.T) {
	testSequence := atomic.AddUint64(&tokenUsageRateLimitTestSequence, 1)
	testUserID := int(testSequence)
	testRemoteAddr := fmt.Sprintf(
		"198.%d.%d.%d:54321",
		(testSequence>>16)%256,
		(testSequence>>8)%256,
		testSequence%256,
	)
	previousRedisEnabled := common.RedisEnabled
	previousTokenUsageEnabled := common.TokenUsageRateLimitEnable
	previousTokenUsageNum := common.TokenUsageRateLimitNum
	previousTokenUsageDuration := common.TokenUsageRateLimitDuration
	previousCriticalEnabled := common.CriticalRateLimitEnable
	previousCriticalNum := common.CriticalRateLimitNum
	previousCriticalDuration := common.CriticalRateLimitDuration
	t.Cleanup(func() {
		common.RedisEnabled = previousRedisEnabled
		common.TokenUsageRateLimitEnable = previousTokenUsageEnabled
		common.TokenUsageRateLimitNum = previousTokenUsageNum
		common.TokenUsageRateLimitDuration = previousTokenUsageDuration
		common.CriticalRateLimitEnable = previousCriticalEnabled
		common.CriticalRateLimitNum = previousCriticalNum
		common.CriticalRateLimitDuration = previousCriticalDuration
	})

	common.RedisEnabled = false
	common.TokenUsageRateLimitEnable = true
	common.TokenUsageRateLimitNum = 2
	common.TokenUsageRateLimitDuration = 60
	common.CriticalRateLimitEnable = true
	common.CriticalRateLimitNum = 2
	common.CriticalRateLimitDuration = 60
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET(
		"/usage",
		func(c *gin.Context) {
			c.Set("id", testUserID)
			c.Next()
		},
		TokenUsageRateLimit(),
		func(c *gin.Context) { c.Status(http.StatusOK) },
	)
	router.POST(
		"/login",
		CriticalRateLimit(),
		func(c *gin.Context) { c.Status(http.StatusOK) },
	)

	request := func(method string, path string) int {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, nil)
		req.RemoteAddr = testRemoteAddr
		router.ServeHTTP(recorder, req)
		return recorder.Code
	}

	require.Equal(t, http.StatusOK, request(http.MethodGet, "/usage"))
	require.Equal(t, http.StatusOK, request(http.MethodGet, "/usage"))
	require.Equal(t, http.StatusTooManyRequests, request(http.MethodGet, "/usage"))

	// Exhausting the read-only usage bucket must not consume login capacity.
	require.Equal(t, http.StatusOK, request(http.MethodPost, "/login"))
	require.Equal(t, http.StatusOK, request(http.MethodPost, "/login"))
	require.Equal(t, http.StatusTooManyRequests, request(http.MethodPost, "/login"))
}
