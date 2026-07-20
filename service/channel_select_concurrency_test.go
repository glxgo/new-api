package service

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRetryParamExcludesCapacitySkippedChannels(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)
	c.Set("use_channel", []string{"8"})
	c.Set("capacity_skipped_channel", []string{"9", "10"})
	excluded := (&RetryParam{Ctx: c}).excludedChannelIds()
	for _, id := range []int{8, 9, 10} {
		_, ok := excluded[id]
		require.True(t, ok)
	}
}
