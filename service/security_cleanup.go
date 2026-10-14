package service

import (
	"context"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"sync"
	"time"
)

var securityCleanupOnce sync.Once

func StartSecurityStateCleanup() {
	securityCleanupOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		common.BackgroundCtxGo("general", context.Background(), func() {
			ticker := time.NewTicker(time.Minute)
			defer ticker.Stop()
			for range ticker.C {
				for _, table := range []any{&model.SecurityProof{}, &model.SecurityCeremony{}, &model.DashboardSession{}} {
					var ids []string
					// Fixed per-tick work; indexed expiration avoids scanning all history.
					if err := model.DB.Model(table).Where("expires_at <= ?", time.Now().Unix()).Order("expires_at").Limit(1000).Pluck("id", &ids).Error; err != nil {
						common.SysError("security state cleanup failed")
						continue
					}
					if len(ids) > 0 {
						if err := model.DB.Where("id IN ?", ids).Delete(table).Error; err != nil {
							common.SysError("security state cleanup failed")
						}
					}
				}
			}
		})
	})
}
