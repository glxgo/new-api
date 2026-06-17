package service

import (
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// 双池扣费端到端(阶段2b): DecreaseUserQuotaDual 拆分 / 跨池 / 并发 / 补偿,
// 以及 WalletFunding 双池记账语义。复用 task_billing_test.go 的 TestMain。
// ---------------------------------------------------------------------------

func seedDualUser(t *testing.T, id, gift, principal int) *model.User {
	t.Helper()
	u := &model.User{
		Id:        id,
		Username:  "dual_pool_tester",
		Quota:     principal,
		GiftQuota: gift,
		Status:    common.UserStatusEnabled,
	}
	require.NoError(t, model.DB.Create(u).Error)
	return u
}

func reloadDualUser(t *testing.T, id int) model.User {
	t.Helper()
	var u model.User
	require.NoError(t, model.DB.First(&u, id).Error)
	return u
}

// 纯赠金余额: 全额从赠金扣, 本金不动。
func TestDualDecrease_AllGift(t *testing.T) {
	truncate(t)
	seedDualUser(t, 1, 100, 50)

	g, p, err := DecreaseUserQuotaDual(1, 30)
	require.NoError(t, err)
	assert.Equal(t, 30, g)
	assert.Equal(t, 0, p)

	after := reloadDualUser(t, 1)
	assert.Equal(t, 70, after.GiftQuota)
	assert.Equal(t, 50, after.Quota, "本金不应动")
	assert.Equal(t, 30, after.UsedGiftQuota)
}

// 跨池: 赠金不足, 拆分扣本金补差额。
func TestDualDecrease_CrossPool(t *testing.T) {
	truncate(t)
	seedDualUser(t, 1, 20, 50)

	g, p, err := DecreaseUserQuotaDual(1, 50)
	require.NoError(t, err)
	assert.Equal(t, 20, g, "赠金扣光")
	assert.Equal(t, 30, p, "差额落本金")

	after := reloadDualUser(t, 1)
	assert.Equal(t, 0, after.GiftQuota)
	assert.Equal(t, 20, after.Quota)
	assert.Equal(t, 20, after.UsedGiftQuota)
}

// 全部本金: 无赠金时全扣本金。
func TestDualDecrease_AllPrincipal(t *testing.T) {
	truncate(t)
	seedDualUser(t, 1, 0, 100)

	g, p, err := DecreaseUserQuotaDual(1, 40)
	require.NoError(t, err)
	assert.Equal(t, 0, g)
	assert.Equal(t, 40, p)

	after := reloadDualUser(t, 1)
	assert.Equal(t, 60, after.Quota)
	assert.Equal(t, 0, after.UsedGiftQuota)
}

// 两池都不足: 返回错误, 不扣。
func TestDualDecrease_Insufficient(t *testing.T) {
	truncate(t)
	seedDualUser(t, 1, 10, 10)

	_, _, err := DecreaseUserQuotaDual(1, 100)
	assert.Error(t, err)
	assert.True(t, IsDualInsufficientQuotaError(err))

	after := reloadDualUser(t, 1)
	assert.Equal(t, 10, after.GiftQuota, "不足不应扣减")
	assert.Equal(t, 10, after.Quota)
}

// 并发: 100 worker 同时扣同一用户(双池), 总扣减不超余额, used_gift_quota 准确。
func TestDualDecrease_ConcurrencyBalanced(t *testing.T) {
	truncate(t)
	const gift = 500
	const principal = 1000
	seedDualUser(t, 1, gift, principal)

	const workers = 100
	const per = 20 // 总需求 2000, 双池合计 1500, 必有失败
	var wg sync.WaitGroup
	wg.Add(workers)
	var mu sync.Mutex
	var totalGift, totalPrincipal, successCnt int

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			g, p, err := DecreaseUserQuotaDual(1, per)
			if err != nil {
				return
			}
			mu.Lock()
			totalGift += g
			totalPrincipal += p
			successCnt++
			mu.Unlock()
		}()
	}
	wg.Wait()

	after := reloadDualUser(t, 1)
	// 总账平衡: 初始合计 - 最终合计 = 成功扣减总量
	initialTotal := gift + principal
	finalTotal := after.GiftQuota + after.Quota
	assert.Equal(t, successCnt*per, initialTotal-finalTotal, "总扣减必须等于成功次数×单次额度")
	// 赠金不超扣
	assert.GreaterOrEqual(t, after.GiftQuota, 0)
	assert.GreaterOrEqual(t, after.Quota, 0)
	// used_gift_quota = 实际赠金扣减量
	assert.Equal(t, totalGift, after.UsedGiftQuota)
	// 赠金先扣, 所以 totalGift 应接近 gift(500)
	assert.LessOrEqual(t, totalGift, gift)
	t.Logf("success=%d gift扣=%d principal扣=%d finalGift=%d finalQuota=%d usedGift=%d",
		successCnt, totalGift, totalPrincipal, after.GiftQuota, after.Quota, after.UsedGiftQuota)
}

// ---------------------------------------------------------------------------
// WalletFunding 双池记账语义
// ---------------------------------------------------------------------------

// PreConsume + Settle(delta>0补扣): consumedGift/consumedPrincipal 累计 = 实际净消费。
func TestWalletFunding_GetPaidSplit(t *testing.T) {
	truncate(t)
	seedDualUser(t, 1, 30, 100) // 赠金30 本金100

	w := &WalletFunding{userId: 1}
	// 预扣 20: 赠金扣20
	require.NoError(t, w.PreConsume(20))
	g, p := w.GetPaidSplit()
	assert.Equal(t, 20, g)
	assert.Equal(t, 0, p)

	// Settle 补扣 10(实际消费30, 预扣20): 赠金只剩10, 全扣赠金
	require.NoError(t, w.Settle(10))
	g, p = w.GetPaidSplit()
	assert.Equal(t, 30, g, "累计赠金消费30")
	assert.Equal(t, 0, p)
	assert.Equal(t, 30, g+p, "GetPaidSplit 合计 = 净消费")
}

// Settle(delta<0退还): 按比例退回各自池子, consumed 减少。
func TestWalletFunding_SettleRefund(t *testing.T) {
	truncate(t)
	seedDualUser(t, 1, 50, 50)

	w := &WalletFunding{userId: 1}
	// 实际拆 20gift + 20principal = 40。手工构造: 预扣40(拆分取决于余额, 这里gift/principal都50)
	require.NoError(t, w.PreConsume(40))
	g, p := w.GetPaidSplit()
	// 余额均等, SplitPayment 先扣gift: gift=40 principal=0
	assert.Equal(t, 40, g)
	assert.Equal(t, 0, p)

	// 退还 10: 按净消费比例(40gift:0principal) → 全退gift
	require.NoError(t, w.Settle(-10))
	after := reloadDualUser(t, 1)
	assert.Equal(t, 20, after.GiftQuota, "退回10gift: 50-40+10=20")
	g, p = w.GetPaidSplit()
	assert.Equal(t, 30, g, "consumedGift 从40减到30")
}

// Refund 全额退回各自池。
func TestWalletFunding_Refund(t *testing.T) {
	truncate(t)
	seedDualUser(t, 1, 50, 50)

	w := &WalletFunding{userId: 1}
	require.NoError(t, w.PreConsume(40)) // 全扣gift

	require.NoError(t, w.Refund())
	after := reloadDualUser(t, 1)
	assert.Equal(t, 50, after.GiftQuota, "全退回")
	assert.Equal(t, 50, after.Quota)
	g, p := w.GetPaidSplit()
	assert.Equal(t, 0, g)
	assert.Equal(t, 0, p)
}

// paidSplitForLog: 有Billing取拆分, 无则全本金。
func TestPaidSplitForLog_Fallback(t *testing.T) {
	// nil relayInfo → 全本金
	g, p := paidSplitForLog(nil, 100)
	assert.Equal(t, 0, g)
	assert.Equal(t, 100, p)
}
