package middleware

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

func CaptureRequestClient() gin.HandlerFunc {
	return func(c *gin.Context) { common.CaptureClientSnapshot(c); c.Next() }
}
