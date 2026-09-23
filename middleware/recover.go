package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

func RelayPanicRecover() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				common.SysLog(fmt.Sprintf("stacktrace from panic: %s", string(debug.Stack())))
				RecoverResponse(c, err)
			}
		}()
		c.Next()
	}
}

// RecoverResponse keeps panic diagnostics in server logs. A started stream must
// not receive an unrelated JSON document after its headers have been committed.
func RecoverResponse(c *gin.Context, err any) {
	common.SysLog(fmt.Sprintf("panic detected: %v", err))
	c.Abort()
	if c.Writer.Written() {
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{
		"error": gin.H{"message": "Internal server error", "type": "new_api_panic"},
	})
}
