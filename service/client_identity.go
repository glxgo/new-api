package service

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

// Re-read persisted approval/policy on every attempt, including same-channel,
// auto-group and custom-route retries. No positive cache survives revocation.
func CheckRequestClientAccess(c *gin.Context) *types.NewAPIError {
	s, ok := common.GetClientSnapshot(c)
	if !ok {
		return nil
	} // internal channel tests/background jobs have no ingress UA
	group := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	if group == "auto" {
		group = common.GetContextKeyString(c, constant.ContextKeyAutoGroup)
	}
	coding, err := model.CheckClientGroupAccess(c.Request.Context(), s, group)
	c.Set(common.ClientCodingContextKey, coding)
	if err == nil {
		return nil
	}
	return types.NewErrorWithStatusCode(err, "client_not_allowed", 403, types.ErrOptionWithSkipRetry())
}

func RecordClientAccessDenied(c *gin.Context, modelName string, err error) {
	group := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	if group == "auto" {
		group = common.GetContextKeyString(c, constant.ContextKeyAutoGroup)
	}
	model.RecordErrorLog(c, c.GetInt("id"), 0, modelName, c.GetString("token_name"), err.Error(), c.GetInt("token_id"), 0, false, group, map[string]interface{}{"client_access_denied": true})
}
