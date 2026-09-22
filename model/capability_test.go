package model

import (
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func capabilityTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	sql, err := db.DB()
	require.NoError(t, err)
	sql.SetMaxOpenConns(1)
	old := DB
	DB = db
	t.Cleanup(func() { DB = old; _ = sql.Close() })
	require.NoError(t, MigrateCapabilitySchema(db))
	return db
}
func TestCapabilityReadHasNoWriteAndCAS(t *testing.T) {
	db := capabilityTestDB(t)
	row, err := GetCapabilityControl()
	require.NoError(t, err)
	require.False(t, row.Running)
	require.False(t, row.Visible)
	var count int64
	require.NoError(t, db.Model(&CapabilityControl{}).Count(&count).Error)
	require.Zero(t, count)
	row, err = UpdateCapabilityControl(0, 1, "show_only", func(row *CapabilityControl) error { row.Visible = true; return nil })
	require.NoError(t, err)
	require.EqualValues(t, 1, row.Revision)
	require.False(t, row.Running)
	_, err = UpdateCapabilityControl(0, 2, "stop", func(row *CapabilityControl) error { row.Running = false; return nil })
	require.ErrorIs(t, err, ErrCapabilityConflict)
	_, err = UpdateCapabilityControl(1, 1, "hide_and_stop", func(row *CapabilityControl) error { row.Running = false; row.Visible = false; return nil })
	require.NoError(t, err)
	row, err = GetCapabilityControl()
	require.NoError(t, err)
	require.False(t, row.Visible)
	require.NoError(t, db.Model(&CapabilityEvent{}).Count(&count).Error)
	require.EqualValues(t, 2, count)
}
func TestCapabilityBudgetDispatchAndStop(t *testing.T) {
	db := capabilityTestDB(t)
	row, err := UpdateCapabilityControl(0, 1, "resume", func(row *CapabilityControl) error {
		row.Running = true
		row.ExecutionRevision = 1
		config := DefaultCapabilityConfig()
		config.DailyBudgetMicros = 10000
		config.CallReserveMicros = 50
		raw, _ := common.Marshal(config)
		row.Settings = string(raw)
		return nil
	})
	require.NoError(t, err)
	require.NoError(t, db.Create(&CapabilityWorker{ID: 1, Owner: "owner", Heartbeat: time.Now().Unix()}).Error)
	run := CapabilityRun{ID: "run", Identity: "identity", Owner: "owner", Status: "running"}
	require.NoError(t, db.Create(&run).Error)
	for _, kind := range []string{"logic", "geometry"} {
		a := CapabilityAttempt{Kind: kind}
		require.NoError(t, DispatchCapabilityAttempt(run, &a, row.ExecutionRevision, 100, 50))
		require.NotEmpty(t, a.ID)
	}
	duplicate := CapabilityAttempt{Kind: "logic"}
	require.EqualError(t, DispatchCapabilityAttempt(run, &duplicate, row.ExecutionRevision, 10000, 50), "attempt_already_dispatched")
	a := CapabilityAttempt{Kind: "scene"}
	require.Error(t, DispatchCapabilityAttempt(run, &a, row.ExecutionRevision, 100, 50))
	var count int64
	require.NoError(t, db.Model(&CapabilityAttempt{}).Count(&count).Error)
	require.EqualValues(t, 2, count)
	_, err = UpdateCapabilityControl(row.Revision, 1, "stop", func(row *CapabilityControl) error { row.Running = false; row.ExecutionRevision++; return nil })
	require.NoError(t, err)
	require.Error(t, DispatchCapabilityAttempt(run, &a, row.ExecutionRevision, 10000, 50))
	var budget CapabilityBudgetDay
	require.NoError(t, db.First(&budget).Error)
	require.EqualValues(t, 100, budget.ReservedMicros)
}
func TestCapabilitySchemaIdempotentAndFalseValues(t *testing.T) {
	db := capabilityTestDB(t)
	require.NoError(t, MigrateCapabilitySchema(db))
	raw, err := common.Marshal(DefaultCapabilityConfig())
	require.NoError(t, err)
	require.Contains(t, string(raw), `"interval_minutes":30`)
}
