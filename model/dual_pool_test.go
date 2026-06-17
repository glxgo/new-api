package model

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// 双池扣费原语 + DecreaseUserGiftQuotaGuarded/DecreaseUserQuotaGuarded 单元测试(阶段2b)
// 复用 task_cas_test.go 的 TestMain(in-memory SQLite)。
// ---------------------------------------------------------------------------

func insertDualUser(t *testing.T, gift, principal int) *User {
	t.Helper()
	u := &User{
		Username:  "dual_tester",
		Password:  "x",
		Quota:     principal,
		GiftQuota: gift,
		Status:    1,
	}
	require.NoError(t, DB.Create(u).Error)
	return u
}

func reloadUser(t *testing.T, id int) User {
	t.Helper()
	var u User
	require.NoError(t, DB.First(&u, id).Error)
	return u
}

// 纯赠金扣减: 守卫命中, gift_quota 减、used_gift_quota 增, quota 不动。
func TestDecreaseGiftQuotaGuarded_Hit(t *testing.T) {
	truncateTables(t)
	u := insertDualUser(t, 100, 50)

	ok, err := DecreaseUserGiftQuotaGuarded(u.Id, 30)
	require.NoError(t, err)
	assert.True(t, ok)

	after := reloadUser(t, u.Id)
	assert.Equal(t, 70, after.GiftQuota)
	assert.Equal(t, 30, after.UsedGiftQuota, "used_gift_quota 应累加")
	assert.Equal(t, 50, after.Quota, "本金池不应变动")
}

// 守卫拦截: 并发抢占导致余额不足时 ok=false, 不扣。
func TestDecreaseGiftQuotaGuarded_Miss(t *testing.T) {
	truncateTables(t)
	u := insertDualUser(t, 10, 50)

	ok, err := DecreaseUserGiftQuotaGuarded(u.Id, 30)
	require.NoError(t, err)
	assert.False(t, ok, "余额不足应返回 false 不扣减")

	after := reloadUser(t, u.Id)
	assert.Equal(t, 10, after.GiftQuota, "未扣减")
	assert.Equal(t, 0, after.UsedGiftQuota)
}

// 本金守卫同理。
func TestDecreaseUserQuotaGuarded_Miss(t *testing.T) {
	truncateTables(t)
	u := insertDualUser(t, 100, 10)

	ok, err := DecreaseUserQuotaGuarded(u.Id, 30)
	require.NoError(t, err)
	assert.False(t, ok)

	after := reloadUser(t, u.Id)
	assert.Equal(t, 10, after.Quota, "未扣减")
}

// GetUserGiftQuota 从 DB 正确读取。
func TestGetUserGiftQuota(t *testing.T) {
	truncateTables(t)
	u := insertDualUser(t, 77, 88)
	g, err := GetUserGiftQuota(u.Id, true)
	require.NoError(t, err)
	assert.Equal(t, 77, g)
}

// ---------------------------------------------------------------------------
// 并发: 多 goroutine 同时扣同一用户, WHERE 守卫保证不超扣。
// 注意 TestMain 设 MaxOpenConns(1), SQLite 写串行化, 此测试验证逻辑正确性
// (每个成功扣减都满足守卫、总扣减不超余额、used_gift_quota 累计正确);
// 真正的高并发竞争由 MySQL 实测覆盖。
// ---------------------------------------------------------------------------

func TestGiftGuardedConcurrency_NoOverSpend(t *testing.T) {
	truncateTables(t)
	const initialGift = 1000
	u := insertDualUser(t, initialGift, 100000)

	const workers = 100
	const perConsume = 30 // 总需求 3000 > 1000, 必然有失败
	var wg sync.WaitGroup
	wg.Add(workers)
	var successCnt, failCnt int
	var mu sync.Mutex

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			ok, err := DecreaseUserGiftQuotaGuarded(u.Id, perConsume)
			if err != nil {
				return
			}
			mu.Lock()
			if ok {
				successCnt++
			} else {
				failCnt++
			}
			mu.Unlock()
		}()
	}
	wg.Wait()

	after := reloadUser(t, u.Id)
	// 成功次数 × 30 不能超过初始赠金 1000 → 成功 ≤ 33
	assert.LessOrEqual(t, successCnt, initialGift/perConsume, "成功扣减次数不能导致超扣")
	// 最终赠金 = 初始 - 成功扣减总量(必然 ≥ 0, 不为负)
	expectedGift := initialGift - successCnt*perConsume
	assert.Equal(t, expectedGift, after.GiftQuota, "gift_quota 应精确等于初始-成功扣减")
	assert.Equal(t, successCnt*perConsume, after.UsedGiftQuota, "used_gift_quota 应等于成功扣减总量")
	assert.Greater(t, successCnt, 0)
	t.Logf("workers=%d success=%d fail=%d finalGift=%d", workers, successCnt, failCnt, after.GiftQuota)
}
