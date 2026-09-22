package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func previewTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = pool.Close() })
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.PelicanTarget{}, &model.Option{}))
	return db
}

func previewTestJSON(t *testing.T, data any) string {
	t.Helper()
	raw, err := common.Marshal(data)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "fixture.json")
	require.NoError(t, os.WriteFile(path, raw, 0600))
	return path
}

func TestPreviewMappingsAtomicAndIdentityChecked(t *testing.T) {
	db := previewTestDB(t)
	require.NoError(t, db.Create(&model.Channel{Id: 8, Models: "m"}).Error)
	id := model.PelicanTargetID("fixture", "provider-a", "m")
	require.NoError(t, db.Create(&model.PelicanTarget{ID: id, Present: true}).Error)
	valid := map[string]any{"provider_id": "provider-a", "model": "m", "channel_id": 8}
	missing := map[string]any{"provider_id": "missing", "model": "m", "channel_id": 8}
	for _, tc := range []struct {
		name, source string
		bindings     []map[string]any
	}{
		{"wrong-source", "other", []map[string]any{valid}},
		{"missing-after-valid", "fixture", []map[string]any{valid, missing}},
		{"duplicate-after-valid", "fixture", []map[string]any{valid, valid}},
		{"missing-channel", "fixture", []map[string]any{{"provider_id": "provider-a", "model": "m", "channel_id": 99}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := previewTestJSON(t, map[string]any{"source_id": tc.source, "bindings": tc.bindings})
			n, err := applyPreviewMappings(db, path, "fixture")
			require.Error(t, err)
			require.Zero(t, n)
			var target model.PelicanTarget
			require.NoError(t, db.First(&target, "id = ?", id).Error)
			require.Zero(t, target.ChannelID, "failed batches must not leave partial associations")
		})
	}
	path := previewTestJSON(t, map[string]any{"source_id": "fixture", "bindings": []any{valid}})
	n, err := applyPreviewMappings(db, path, "fixture")
	require.NoError(t, err)
	require.Equal(t, 1, n)
	// A confirmed identity alone cannot override a model removed at the station.
	require.NoError(t, db.Model(&model.Channel{}).Where("id = ?", 8).Update("models", "other").Error)
	_, err = applyPreviewMappings(db, path, "fixture")
	require.Error(t, err)
}

func TestPreviewTopologyRejectsInvalidSnapshotsWithoutRows(t *testing.T) {
	for _, kind := range []string{"stale", "future", "missing-options", "duplicate-channel", "orphan-ability"} {
		t.Run(kind, func(t *testing.T) {
			db := previewTestDB(t)
			now := time.Now().UTC()
			s := map[string]any{"schema_version": 1, "captured_at": now, "channels": []any{map[string]any{"id": 8, "models": "m"}}, "options": map[string]string{"GroupRatio": "{}", "UserUsableGroups": "{}"}}
			switch kind {
			case "stale":
				s["captured_at"] = now.Add(-25 * time.Hour)
			case "future":
				s["captured_at"] = now.Add(time.Hour)
			case "missing-options":
				delete(s, "options")
			case "duplicate-channel":
				s["channels"] = []any{map[string]any{"id": 8}, map[string]any{"id": 8}}
			case "orphan-ability":
				s["abilities"] = []any{map[string]any{"channel_id": 99, "group": "a", "model": "m", "enabled": true}}
			}
			_, err := loadPreviewTopology(db, previewTestJSON(t, s))
			require.Error(t, err)
			var count int64
			require.NoError(t, db.Model(&model.Channel{}).Count(&count).Error)
			require.Zero(t, count)
		})
	}
}
