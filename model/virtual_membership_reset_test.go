package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestVirtualMembershipActiveResetCreditsGrantAndConsume(t *testing.T) {
	db := setupVirtualMembershipTestDB(t)
	now := common.GetTimestamp()
	membership := UserVirtualMembership{
		UserId: 101, PlanId: 1, PlanTitle: "reset-plan", PlanCode: "reset-plan",
		GroupSize: 1, WeeklyQuota: 1_000, WeeklyUsed: 420,
		FiveHourActive: true, FiveHourQuota: 500, FiveHourUsed: 210,
		WeeklyResetAt: now + 3600, FiveHourResetAt: now + 1800,
		StartTime: now - 60, EndTime: now + 86400,
		Status: VirtualMembershipStatusActive,
	}
	require.NoError(t, db.Create(&membership).Error)

	_, err := ActiveResetVirtualMembership(101, membership.Id)
	require.ErrorIs(t, err, ErrVirtualMembershipResetCreditsInsufficient)

	updated, err := GrantVirtualMembershipResetCredits(membership.Id, 2)
	require.NoError(t, err)
	require.Equal(t, 2, updated.ActiveResetCredits)
	_, err = ActiveResetVirtualMembership(999, membership.Id)
	require.Error(t, err, "ownership must be enforced")

	before := common.GetTimestamp()
	reset, err := ActiveResetVirtualMembership(101, membership.Id)
	require.NoError(t, err)
	require.Zero(t, reset.WeeklyUsed)
	require.Zero(t, reset.FiveHourUsed)
	require.Equal(t, 1, reset.ActiveResetCredits)
	require.GreaterOrEqual(t, reset.WeeklyResetAt, before+7*86400)
	require.GreaterOrEqual(t, reset.FiveHourResetAt, before+5*3600)

	_, err = ActiveResetVirtualMembership(101, membership.Id)
	require.NoError(t, err)
	_, err = ActiveResetVirtualMembership(101, membership.Id)
	require.ErrorIs(t, err, ErrVirtualMembershipResetCreditsInsufficient)
}

func TestVirtualMembershipActiveResetBlocksFreshPendingSettlement(t *testing.T) {
	db := setupVirtualMembershipTestDB(t)
	now := common.GetTimestamp()
	membership := UserVirtualMembership{
		UserId: 106, PlanId: 1, PlanTitle: "fresh-pending", PlanCode: "fresh-pending",
		GroupSize: 1, WeeklyQuota: 1_000, WeeklyUsed: 420,
		ActiveResetCredits: 1, StartTime: now - 60, EndTime: now + 86400,
		Status: VirtualMembershipStatusActive,
	}
	require.NoError(t, db.Create(&membership).Error)
	require.NoError(t, db.Create(&VirtualMembershipPreConsumeRecord{
		RequestId: "fresh-pending-reset", MembershipId: membership.Id, UserId: membership.UserId,
		PreConsumed: 10, Status: VirtualMembershipRecordPending, CreatedAt: now - 60, UpdatedAt: now - 60,
	}).Error)

	_, err := ActiveResetVirtualMembership(membership.UserId, membership.Id)
	require.ErrorIs(t, err, ErrVirtualMembershipSettlementInProgress)

	var unchanged UserVirtualMembership
	require.NoError(t, db.First(&unchanged, membership.Id).Error)
	require.EqualValues(t, 420, unchanged.WeeklyUsed)
	require.Equal(t, 1, unchanged.ActiveResetCredits)
}

func TestVirtualMembershipActiveResetRecoversStalePendingSettlement(t *testing.T) {
	db := setupVirtualMembershipTestDB(t)
	now := common.GetTimestamp()
	membership := UserVirtualMembership{
		UserId: 107, PlanId: 1, PlanTitle: "stale-pending", PlanCode: "stale-pending",
		GroupSize: 1, WeeklyQuota: 1_000, WeeklyUsed: 420,
		FiveHourActive: true, FiveHourQuota: 500, FiveHourUsed: 210,
		ActiveResetCredits: 1, WeeklyResetAt: now + 3600, FiveHourResetAt: now + 1800,
		StartTime: now - 60, EndTime: now + 86400,
		Status: VirtualMembershipStatusActive,
	}
	require.NoError(t, db.Create(&membership).Error)
	require.NoError(t, db.Create(&VirtualMembershipPreConsumeRecord{
		RequestId: "stale-pending-reset", MembershipId: membership.Id, UserId: membership.UserId,
		PreConsumed: 10, Status: VirtualMembershipRecordPending,
		CreatedAt: now - int64(virtualMembershipPendingSettlementTimeout()/time.Second) - 60,
		UpdatedAt: now - int64(virtualMembershipPendingSettlementTimeout()/time.Second) - 60,
	}).Error)

	reset, err := ActiveResetVirtualMembership(membership.UserId, membership.Id)
	require.NoError(t, err)
	require.Zero(t, reset.WeeklyUsed)
	require.Zero(t, reset.FiveHourUsed)
	require.Zero(t, reset.ActiveResetCredits)

	var refunded VirtualMembershipPreConsumeRecord
	require.NoError(t, db.Where("request_id = ?", "stale-pending-reset").First(&refunded).Error)
	require.Equal(t, VirtualMembershipRecordRefunded, refunded.Status)

	// A late callback must not add the old request back into the new window.
	require.NoError(t, PostConsumeVirtualMembershipDelta(refunded.RequestId, 25))
	var refreshed UserVirtualMembership
	require.NoError(t, db.First(&refreshed, membership.Id).Error)
	require.Zero(t, refreshed.WeeklyUsed)
	require.Zero(t, refreshed.FiveHourUsed)
}

func TestVirtualMembershipResetOrderUsesPriceSnapshotAndCompletesIdempotently(t *testing.T) {
	db := setupVirtualMembershipTestDB(t)
	require.NoError(t, db.AutoMigrate(&VirtualMembershipResetOrder{}))
	now := common.GetTimestamp()
	membership := UserVirtualMembership{
		UserId: 102, PlanId: 1, PlanTitle: "snapshot-plan", PlanCode: "snapshot-plan",
		GroupSize: 1, PurchasePriceAmount: 80,
		WeeklyQuota: 1_000, StartTime: now - 60, EndTime: now + 86400,
		Status: VirtualMembershipStatusActive,
	}
	require.NoError(t, db.Create(&membership).Error)

	order, err := CreateVirtualMembershipResetEpayOrder(102, membership.Id, "reset-order-1", "alipay", "虚拟会员主动重置次数")
	require.NoError(t, err)
	require.Equal(t, 24.0, order.Money)
	require.EqualValues(t, 2_400, order.ExpectedPaymentAmountMinor)

	// A second browser click reuses the pending immutable order instead of
	// creating a second charge.
	reused, err := CreateVirtualMembershipResetEpayOrder(102, membership.Id, "reset-order-2", "wechat", "ignored")
	require.NoError(t, err)
	require.Equal(t, order.Id, reused.Id)
	require.Equal(t, "reset-order-1", reused.TradeNo)

	actual, err := NewPaymentSnapshotFromMoney(24, "CNY")
	require.NoError(t, err)
	require.NoError(t, CompleteVirtualMembershipPayment(order.TradeNo, "paid", PaymentProviderEpay, "alipay", actual))
	require.NoError(t, CompleteVirtualMembershipPayment(order.TradeNo, "paid-replay", PaymentProviderEpay, "alipay", actual))

	var refreshed UserVirtualMembership
	require.NoError(t, db.First(&refreshed, membership.Id).Error)
	require.Equal(t, 1, refreshed.ActiveResetCredits)
	var completed VirtualMembershipResetOrder
	require.NoError(t, db.First(&completed, order.Id).Error)
	require.Equal(t, VirtualMembershipOrderSuccess, completed.Status)
	require.EqualValues(t, 2_400, completed.ActualPaymentAmountMinor)

	wrongAmount, err := NewPaymentSnapshotFromMoney(23.99, "CNY")
	require.NoError(t, err)
	require.Error(t, CompleteVirtualMembershipResetOrder(order.TradeNo, "tampered", PaymentProviderEpay, "alipay", wrongAmount))
}

func TestVirtualMembershipResetOrderRejectsPurchaseWhenCreditAlreadyExists(t *testing.T) {
	db := setupVirtualMembershipTestDB(t)
	now := common.GetTimestamp()
	membership := UserVirtualMembership{
		UserId: 103, PlanId: 1, PlanTitle: "duplicate-guard", PurchasePriceAmount: 50,
		WeeklyQuota: 100, StartTime: now - 60, EndTime: now + 86400,
		Status: VirtualMembershipStatusActive, ActiveResetCredits: 1,
	}
	require.NoError(t, db.Create(&membership).Error)

	_, err := CreateVirtualMembershipResetEpayOrder(103, membership.Id, "reset-order-credit-exists", "alipay", "ignored")
	require.EqualError(t, err, "已有主动重置次数，请先使用现有次数")
}

func TestVirtualMembershipResetFinalizesPendingSettlement(t *testing.T) {
	db := setupVirtualMembershipTestDB(t)
	now := common.GetTimestamp()
	memberships := []UserVirtualMembership{
		{UserId: 104, PlanId: 1, PlanCode: "pending", WeeklyQuota: 100, WeeklyUsed: 70, StartTime: now - 60, EndTime: now + 86400, Status: VirtualMembershipStatusActive},
		{UserId: 105, PlanId: 1, PlanCode: "pending", WeeklyQuota: 100, WeeklyUsed: 80, StartTime: now - 60, EndTime: now + 86400, Status: VirtualMembershipStatusActive},
	}
	require.NoError(t, db.Create(&memberships).Error)
	require.NoError(t, db.Create(&VirtualMembershipPreConsumeRecord{
		RequestId: "pending-reset-usage", MembershipId: memberships[0].Id, UserId: 104,
		PreConsumed: 5, Status: VirtualMembershipRecordPending, CreatedAt: now, UpdatedAt: now,
	}).Error)

	affected, err := ResetVirtualMemberships(VirtualMembershipResetScope{PlanCode: "pending"})
	require.NoError(t, err)
	require.EqualValues(t, 2, affected)
	var refreshed UserVirtualMembership
	require.NoError(t, db.First(&refreshed, memberships[0].Id).Error)
	require.Zero(t, refreshed.WeeklyUsed)
	refreshed = UserVirtualMembership{}
	require.NoError(t, db.First(&refreshed, memberships[1].Id).Error)
	require.Zero(t, refreshed.WeeklyUsed)
	var pending VirtualMembershipPreConsumeRecord
	require.NoError(t, db.Where("membership_id = ?", memberships[0].Id).First(&pending).Error)
	require.Equal(t, VirtualMembershipRecordRefunded, pending.Status)

	// A response that settles after an administrator reset must not add its
	// pre-consumed amount back into the newly reset quota window.
	require.NoError(t, PostConsumeVirtualMembershipDelta(pending.RequestId, 25))
	refreshed = UserVirtualMembership{}
	require.NoError(t, db.First(&refreshed, memberships[0].Id).Error)
	require.Zero(t, refreshed.WeeklyUsed)

	_, err = GrantVirtualMembershipResetCredits(memberships[0].Id, 1)
	require.NoError(t, err)
	_, err = ActiveResetVirtualMembership(104, memberships[0].Id)
	require.NoError(t, err)
}
