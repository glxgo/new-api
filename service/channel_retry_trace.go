package service

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

const ginKeyChannelRetryAttempts = "channel_retry_attempts"

// RecordChannelRetryAttempt keeps an intermediate channel failure in the
// request context. It is later attached to admin_info on the final success or
// failure log, so users do not see transient failures that were recovered by
// another channel.
func RecordChannelRetryAttempt(c *gin.Context, channel types.ChannelError, err *types.NewAPIError) {
	if c == nil || err == nil {
		return
	}
	attempts, _ := c.Get(ginKeyChannelRetryAttempts)
	retryAttempts, _ := attempts.([]map[string]interface{})
	retryAttempts = append(retryAttempts, map[string]interface{}{
		"attempt":      len(retryAttempts) + 1,
		"channel_id":   channel.ChannelId,
		"channel_name": channel.ChannelName,
		"channel_type": channel.ChannelType,
		"status_code":  err.StatusCode,
		"error_type":   err.GetErrorType(),
		"error_code":   err.GetErrorCode(),
		"error":        common.LocalLogPreview(err.MaskSensitiveErrorWithStatusCode()),
	})
	c.Set(ginKeyChannelRetryAttempts, retryAttempts)
}

func AppendChannelRetryAttemptsAdminInfo(c *gin.Context, adminInfo map[string]interface{}) {
	if c == nil || adminInfo == nil {
		return
	}
	value, ok := c.Get(ginKeyChannelRetryAttempts)
	if !ok {
		return
	}
	attempts, ok := value.([]map[string]interface{})
	if !ok || len(attempts) == 0 {
		return
	}
	adminInfo["retry_attempts"] = attempts
}
