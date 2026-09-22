package controller

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCapabilityRecoveryDatabaseOutageRemainsRetryable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "recovery.db")), &gorm.Config{})
	require.NoError(t, err)
	sql, err := db.DB()
	require.NoError(t, err)
	old := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = old; _ = sql.Close() })
	require.NoError(t, model.MigrateCapabilitySchema(db))
	job := model.CapabilityRecoveryJob{RunID: "missing", Kind: "logic"}
	_, status, reason := capabilityProcessRecovery(context.Background(), model.CapabilityControl{}, model.CapabilityConfig{}, job)
	require.Equal(t, "blocked", status)
	require.Equal(t, "source_missing", reason)
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register("fixture:outage", func(tx *gorm.DB) {
		tx.AddError(errors.New("temporary database outage"))
	}))
	_, status, reason = capabilityProcessRecovery(context.Background(), model.CapabilityControl{}, model.CapabilityConfig{}, job)
	require.Equal(t, "pending", status)
	require.Equal(t, "recovery_evidence_unavailable", reason)
}
