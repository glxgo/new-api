package service

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestPelicanMappingReportsFollowLiveTopology(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	defer pool.Close()
	pool.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.PelicanTarget{}))
	c := model.Channel{Id: 12, Name: "source name differs", Key: "private-key", Status: common.ChannelStatusEnabled, Group: "A,B", Models: "m"}
	require.NoError(t, db.Create(&c).Error)
	require.NoError(t, db.Create(&[]model.Ability{{ChannelId: 12, Group: "A", Model: "m", Enabled: true}, {ChannelId: 12, Group: "B", Model: "m", Enabled: true}, {ChannelId: 12, Group: "C", Model: "m", Enabled: true}}).Error)
	target := model.PelicanTarget{ID: "external-stable-id", Present: true, Enabled: true, ChannelID: 12, DisplayModel: "m"}
	require.NoError(t, db.Create(&target).Error)
	ratio := 1.25
	groups := []CapabilityGroupView{{GroupUID: "uid-a", RoutingKey: "A", Ratio: &ratio}, {GroupUID: "uid-b", RoutingKey: "B"}, {GroupUID: "uid-c", RoutingKey: "C"}}
	view := model.DefaultCapabilityPresentation()
	alias := "Display alias"
	view.Groups["uid-a"] = model.CapabilityGroupOverride{Name: &alias}
	read := func() PelicanMappingReport {
		var targets []model.PelicanTarget
		require.NoError(t, db.Find(&targets).Error)
		reports, channels, err := PelicanMappingReports(db, targets, groups, view)
		require.NoError(t, err)
		require.Len(t, reports, 1)
		raw, err := common.Marshal(channels)
		require.NoError(t, err)
		require.NotContains(t, string(raw), "private-key")
		for _, g := range reports[0].Groups {
			public, err := PelicanTargetsForGroup(db, g.RoutingKey, "m")
			require.NoError(t, err)
			if g.Reason != "group_hidden" && g.Reason != "group_unavailable" {
				require.Equal(t, len(public) == 1, g.Eligible)
			}
		}
		return reports[0]
	}
	r := read()
	require.Empty(t, r.Reason)
	require.Len(t, r.Groups, 2)
	require.Equal(t, alias, r.Groups[0].DisplayName)
	require.Equal(t, &ratio, r.Groups[0].Ratio)
	require.NoError(t, db.Model(&c).Updates(map[string]any{"name": "renamed", "group": "B,C"}).Error)
	r = read()
	require.Equal(t, "B", r.Groups[0].RoutingKey)
	require.Equal(t, "C", r.Groups[1].RoutingKey)
	require.NoError(t, db.Model(&model.Ability{}).Where(map[string]any{"channel_id": 12, "group": "B"}).Update("enabled", false).Error)
	r = read()
	require.Equal(t, "ability_unavailable", r.Groups[0].Reason)
	require.True(t, r.Groups[1].Eligible)
	view.Groups["uid-c"] = model.CapabilityGroupOverride{Hidden: true}
	r = read()
	require.Equal(t, "no_eligible_group", r.Reason)
	require.Equal(t, "group_hidden", r.Groups[1].Reason)
	delete(view.Groups, "uid-c")
	for _, tc := range []struct {
		field  string
		value  any
		reason string
	}{{"status", common.ChannelStatusManuallyDisabled, ""}, {"status", common.ChannelStatusEnabled, ""}, {"models", "other", "model_unavailable"}, {"models", "m", ""}} {
		require.NoError(t, db.Model(&c).Update(tc.field, tc.value).Error)
		require.Equal(t, tc.reason, read().Reason)
		if tc.field == "status" {
			require.Equal(t, tc.value != common.ChannelStatusEnabled, read().ChannelDisabled)
		}
	}
	require.NoError(t, db.Model(&target).Update("hidden", true).Error)
	require.Equal(t, "target_hidden", read().Reason)
	require.NoError(t, db.Model(&target).Update("hidden", false).Error)
	require.NoError(t, db.Model(&target).Update("channel_id", 0).Error)
	require.Equal(t, "unmapped", read().Reason)
	require.NoError(t, db.Model(&target).Update("channel_id", 999).Error)
	require.Equal(t, "channel_missing", read().Reason)
	require.NoError(t, db.Model(&target).Update("present", false).Error)
	require.Equal(t, "source_removed", read().Reason)
}
