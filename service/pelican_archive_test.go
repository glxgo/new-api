package service

import (
	"fmt"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"testing"
)

func TestPelicanSharedResultsFollowCurrentMembership(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	sql, _ := db.DB()
	sql.SetMaxOpenConns(1)
	defer sql.Close()
	require.NoError(t, model.MigratePelicanSchema(db))
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}))
	channel := model.Channel{Id: 1, Key: "never-read", Name: "original", Status: common.ChannelStatusEnabled, Group: "A,B", Models: "m"}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, db.Create(&[]model.Ability{{Group: "A", Model: "m", ChannelId: 1, Enabled: true}, {Group: "B", Model: "m", ChannelId: 1, Enabled: true}, {Group: "C", Model: "m", ChannelId: 1, Enabled: true}}).Error)
	require.NoError(t, db.Create(&model.PelicanTarget{ID: "target", ChannelID: 1, DisplayModel: "m", Present: true, Enabled: true}).Error)
	require.NoError(t, db.Create(&[]model.PelicanRecord{{ID: "older-match", TargetID: "target", ExternalID: 1, Active: true, Grade: "correct", Preview: true, TestedAt: 1}, {ID: "latest", TargetID: "target", ExternalID: 2, Active: true, Grade: "wrong", Preview: true, TestedAt: 2}}).Error)
	read := func(group string) PelicanGroupResults {
		out, err := PelicanResults(db, CapabilityGroupView{RoutingKey: group}, "m", true, 3)
		require.NoError(t, err)
		return out
	}
	a, b := read("A"), read("B")
	require.Equal(t, a.Gallery, b.Gallery)
	require.Equal(t, "latest", a.Gallery[0].ID)
	require.Len(t, a.Gallery, 1)
	require.Len(t, a.History, 2)
	require.NoError(t, db.Model(&channel).Updates(map[string]any{"name": "renamed", "group": "B,C"}).Error)
	require.Empty(t, read("A").Gallery)
	require.Equal(t, read("B").Gallery, read("C").Gallery)
	require.NoError(t, db.Model(&channel).Update("status", common.ChannelStatusManuallyDisabled).Error)
	require.NoError(t, db.Model(&model.Ability{}).Where("channel_id = ?", channel.Id).Update("enabled", false).Error)
	require.Equal(t, "latest", read("B").Gallery[0].ID)
	require.Equal(t, read("B").Gallery, read("C").Gallery)
	require.NoError(t, db.Model(&channel).Update("status", common.ChannelStatusAutoDisabled).Error)
	require.Equal(t, "latest", read("B").Gallery[0].ID)
	require.NoError(t, db.Model(&model.PelicanTarget{}).Where("id = ?", "target").Update("hidden", true).Error)
	require.Empty(t, read("B").Gallery)
	require.NoError(t, db.Model(&model.PelicanTarget{}).Where("id = ?", "target").Update("hidden", false).Error)
	var disabled model.Channel
	require.NoError(t, db.Select("status").First(&disabled, 1).Error)
	require.Equal(t, common.ChannelStatusAutoDisabled, disabled.Status)
	require.NoError(t, db.Model(&channel).Update("status", common.ChannelStatusEnabled).Error)
	require.Empty(t, read("B").Gallery) // independently disabled ability is still excluded
	require.NoError(t, db.Model(&model.Ability{}).Where("channel_id = ?", channel.Id).Update("enabled", true).Error)
	require.NoError(t, db.Model(&model.PelicanTarget{}).Where("id = ?", "target").Update("hidden", true).Error)
	require.Empty(t, read("B").Gallery)
	var stored model.Channel
	require.NoError(t, db.Select("status").First(&stored, 1).Error)
	require.Equal(t, common.ChannelStatusEnabled, stored.Status)
	var count int64
	require.NoError(t, db.Model(&model.PelicanRecord{}).Count(&count).Error)
	require.EqualValues(t, 2, count)
}
