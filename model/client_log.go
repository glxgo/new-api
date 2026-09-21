package model

import (
	"context"
	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type clientLogFilterKey struct{}

func WithClientLogFilter(ctx context.Context, family string) context.Context {
	return context.WithValue(ctx, clientLogFilterKey{}, family)
}
func ClientLogFilter(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	family, _ := ctx.Value(clientLogFilterKey{}).(string)
	return family
}
func applyClientFamilyFilter(tx *gorm.DB, family string) *gorm.DB {
	if family == "unrecorded" {
		return tx.Where("client_family = ? OR client_family IS NULL", "")
	}
	if family != "" {
		return tx.Where("client_family = ?", family)
	}
	return tx
}
func WithRequestClientLog(c *gin.Context, other map[string]interface{}) map[string]interface{} {
	s, ok := common.GetClientSnapshot(c)
	if !ok {
		return other
	}
	// Copy to keep asynchronously used caller metadata immutable.
	result := make(map[string]interface{}, len(other)+2)
	for k, v := range other {
		result[k] = v
	}
	result["client"] = s
	result["coding_group"] = c.GetBool(common.ClientCodingContextKey)
	return result
}
func (log *Log) BeforeCreate(tx *gorm.DB) error {
	// Also covers asynchronous task settlement, whose original snapshot is
	// explicitly carried in Other rather than reconstructed from a worker UA.
	var other struct {
		Client *common.ClientSnapshot `json:"client"`
	}
	if log.Other != "" && common.UnmarshalJsonStr(log.Other, &other) == nil && other.Client != nil {
		log.ClientKey, log.ClientFamily = other.Client.ClientKey, other.Client.Family
	}
	return nil
}
