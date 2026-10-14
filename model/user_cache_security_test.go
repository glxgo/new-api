package model

import (
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSecurityProfileUpdatesPreserveConcurrentBalances(t *testing.T) {
	db := securityTestDB(t)
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	previous := common.RDB
	common.RDB = client
	common.RedisEnabled = true
	t.Cleanup(func() { common.RDB = previous; client.Close() })
	user := User{Id: 7, Username: "funds", Quota: 1000, GiftQuota: 500}
	require.NoError(t, db.Create(&user).Error)
	require.NoError(t, populateUserCache(user))
	var workers sync.WaitGroup
	errors := make(chan error, 200)
	workers.Add(2)
	go func() {
		defer workers.Done()
		for i := 0; i < 40; i++ {
			if err := db.Model(&User{}).Where("id = ?", 7).Updates(map[string]any{"quota": gorm.Expr("quota - 3"), "gift_quota": gorm.Expr("gift_quota - 2")}).Error; err != nil {
				errors <- err
			}
			if err := cacheDecrUserQuota(7, 3); err != nil {
				errors <- err
			}
			if err := cacheDecrUserGiftQuota(7, 2); err != nil {
				errors <- err
			}
		}
	}()
	go func() {
		defer workers.Done()
		for i := 0; i < 40; i++ {
			stale := user
			stale.DisplayName = "updated"
			if err := stale.Update(false); err != nil {
				errors <- err
			}
			if err := UpdateUserSetting(7, dto.UserSetting{Language: "zh"}); err != nil {
				errors <- err
			}
		}
	}()
	workers.Wait()
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	var final User
	require.NoError(t, db.First(&final, 7).Error)
	require.Equal(t, 880, final.Quota)
	require.Equal(t, 420, final.GiftQuota)
	// A final stale preference update must not restore the original Redis balances.
	require.NoError(t, UpdateUserSetting(7, dto.UserSetting{Language: "en"}))
	cache, err := GetUserCache(7)
	require.NoError(t, err)
	require.Equal(t, final.Quota, cache.Quota)
	require.Equal(t, final.GiftQuota, cache.GiftQuota)
}
