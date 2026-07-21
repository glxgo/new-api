package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSettleOrderDividendCreditsAgentWithdrawableCommission(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:agent_order_dividend?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}, &DividendRecord{}))

	oldDB := DB
	oldRate := common.AgentOrderAffiliateDirectRate
	DB = db
	common.AgentOrderAffiliateDirectRate = 0.20
	t.Cleanup(func() {
		DB = oldDB
		common.AgentOrderAffiliateDirectRate = oldRate
	})

	root := User{Username: "commission-root", Role: common.RoleRootUser, AffCode: "cr01"}
	require.NoError(t, db.Create(&root).Error)
	agent := User{Username: "commission-agent", Role: common.RoleAgentUser, AffCode: "ca01"}
	require.NoError(t, db.Create(&agent).Error)
	buyer := User{Username: "commission-buyer", Role: common.RoleCommonUser, AffCode: "cb01", InviterId: agent.Id}
	require.NoError(t, db.Create(&buyer).Error)

	SettleOrderDividend(buyer.Id, 1000, "test-agent-profit-base")

	var refreshed User
	require.NoError(t, db.First(&refreshed, agent.Id).Error)
	require.Equal(t, 200, refreshed.DividendBalance)
	require.Equal(t, 200, refreshed.DividendTotal)
	require.Zero(t, refreshed.GiftQuota)

	var record DividendRecord
	require.NoError(t, db.Where("user_id = ? AND source_ref = ?", agent.Id, "test-agent-profit-base").First(&record).Error)
	require.Equal(t, 200, record.Amount)
}
