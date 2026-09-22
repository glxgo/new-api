package controller

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

func TestCapabilityRoundReplacesSnapshotOnlyWhenFinished(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	sql, err := db.DB()
	require.NoError(t, err)
	sql.SetMaxOpenConns(1)
	old := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = old; _ = sql.Close() })
	require.NoError(t, model.MigrateCapabilitySchema(db))
	_, err = model.UpdateCapabilityControl(0, 1, "resume", func(c *model.CapabilityControl) error { c.Running = true; return nil })
	require.NoError(t, err)
	require.NoError(t, db.Create(&model.CapabilityWorker{ID: 1, Owner: "owner", Heartbeat: time.Now().Unix()}).Error)
	create := func(id string, slot int64, work bool) model.CapabilityRound {
		round := model.CapabilityRound{ID: id, Slot: slot, Owner: "owner", Status: "running"}
		require.NoError(t, db.Create(&round).Error)
		artifact := ""
		if work {
			artifact = "image-" + id
		}
		raw, err := common.Marshal([]capabilityItem{{Kind: "scene", Artifact: artifact, Status: "graded"}})
		require.NoError(t, err)
		require.NoError(t, db.Create(&model.CapabilityRun{ID: id, RoundID: id, Identity: id, Model: "m", Status: "complete", Result: string(raw)}).Error)
		require.NoError(t, db.Create(&model.CapabilityBinding{PublicID: id, RunID: id, GroupUID: "g", DispatchedAt: slot}).Error)
		return round
	}
	first := create("first", 100, true)
	require.NoError(t, capabilityPublishRound(first, "complete"))
	second := create("second", 200, true)
	var snapshot model.CapabilitySnapshot
	require.NoError(t, db.First(&snapshot).Error)
	require.Equal(t, "first", snapshot.RoundID)
	// A completed channel in an unfinished round must not swap the page.
	require.Equal(t, `["first"]`, snapshot.Bindings)
	require.NoError(t, capabilityPublishRound(second, "complete"))
	require.NoError(t, db.First(&snapshot).Error)
	require.Equal(t, "second", snapshot.RoundID)
	failed := create("failed", 300, false)
	require.NoError(t, capabilityPublishRound(failed, "complete"))
	require.NoError(t, db.First(&snapshot).Error)
	require.Equal(t, "second", snapshot.RoundID)
	var previous model.CapabilityRun
	require.NoError(t, db.First(&previous, "id = ?", "first").Error)
	require.Contains(t, previous.Result, "image-first")
	// A committed stop wins over a stale worker trying to publish.
	stopped := create("stopped", 400, true)
	require.NoError(t, db.Model(&model.CapabilityControl{}).Where("id = ?", 1).Update("running", false).Error)
	require.NoError(t, capabilityPublishRound(stopped, "complete"))
	require.NoError(t, db.First(&snapshot).Error)
	require.Equal(t, "second", snapshot.RoundID)
	require.NoError(t, db.First(&stopped, "id = ?", "stopped").Error)
	require.Equal(t, "interrupted", stopped.Status)
}
