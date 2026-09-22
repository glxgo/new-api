package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCapabilityScheduleCustomCrossMidnight(t *testing.T) {
	c := model.DefaultCapabilityConfig()
	c.IntervalMinutes = 37
	c.Anchor = time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC).Unix()
	after := time.Unix(c.Anchor, 0)
	times := CapabilityScheduleTimes(c, after, 45)
	require.Len(t, times, 45)
	for i := 1; i < len(times); i++ {
		require.EqualValues(t, 37*60, times[i]-times[i-1])
	}
	c.Timezone = "UTC"
	c.IntervalMinutes = 30
	c.AllDay = false
	c.WindowStart = 23 * 60
	c.WindowEnd = 60
	c.Weekdays = []int{1}
	times = CapabilityScheduleTimes(c, time.Date(2026, 9, 21, 22, 59, 0, 0, time.UTC), 5)
	require.Equal(t, time.Date(2026, 9, 22, 0, 30, 0, 0, time.UTC).Unix(), times[3])
	require.Equal(t, time.Date(2026, 9, 28, 23, 0, 0, 0, time.UTC).Unix(), times[4])
	c.IntervalMinutes = 0
	require.Empty(t, CapabilityScheduleTimes(c, after, 5))
}
func TestCapabilityDiscoverySharedExcludedAndMove(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	sql, _ := db.DB()
	old := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = old; _ = sql.Close() })
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.CapabilityGroupPresentation{}))
	a, b := "A", "B"
	require.NoError(t, db.Create(&[]model.CapabilityGroupPresentation{{GroupUID: "a", RoutingKey: &a}, {GroupUID: "b", RoutingKey: &b}}).Error)
	ch := model.Channel{Id: 7, Name: "Shared", Group: "A,B", Models: "m,missing", Status: common.ChannelStatusEnabled}
	require.NoError(t, db.Create(&ch).Error)
	require.NoError(t, db.Create(&[]model.Ability{{Group: "A", Model: "m", ChannelId: 7, Enabled: true}, {Group: "B", Model: "m", ChannelId: 7, Enabled: true}}).Error)
	c := model.DefaultCapabilityConfig()
	c.Models = []model.CapabilityProfile{{Model: "m", Protocol: "chat", MaxTokens: 1024, Enabled: true}}
	c.Excluded = []model.CapabilityExclusion{{GroupUID: "a", ChannelID: 7}}
	targets, err := DiscoverCapabilityTargets(c)
	require.NoError(t, err)
	require.Len(t, targets, 4)
	for _, v := range targets {
		if v.Model == "m" {
			require.Equal(t, v.Group == "B", v.Eligible)
		}
	}
	require.NoError(t, db.Model(&ch).Update("group", "B").Error)
	targets, err = DiscoverCapabilityTargets(c)
	require.NoError(t, err)
	for _, v := range targets {
		if v.Group == "A" {
			require.False(t, v.Eligible)
		}
	}
	require.NoError(t, db.Model(&ch).Update("status", common.ChannelStatusManuallyDisabled).Error)
	targets, err = DiscoverCapabilityTargets(c)
	require.NoError(t, err)
	for _, v := range targets {
		require.False(t, v.Eligible)
	}
}
