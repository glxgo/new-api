package model

import (
	"fmt"
	"github.com/QuantumNous/new-api/pkg/pelicanarchive"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"testing"
	"time"
)

func TestPelicanImportAtomicDedupRevisionsAndPause(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	sql, _ := db.DB()
	sql.SetMaxOpenConns(1)
	defer sql.Close()
	verifyPelicanImportAtomicDedupRevisionsAndPause(t, db)
}

// Shared assertions also run against opt-in local MySQL/PostgreSQL engines.
func verifyPelicanImportAtomicDedupRevisionsAndPause(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, MigratePelicanSchema(db))
	require.NoError(t, db.AutoMigrate(&Channel{}))
	row, err := UpdatePelicanControl(db, 0, 1, "resume", func(_ *gorm.DB, c *PelicanControl) error { c.SyncEnabled = true; return nil })
	require.NoError(t, err)
	now := time.Now().UTC()
	s := pelicanarchive.Snapshot{SourceID: "fixture", CapturedAt: now.Format(time.RFC3339Nano), Config: pelicanarchive.Config{Prompt: "question", PromptHash: pelicanarchive.PromptHash("question")}, Targets: []pelicanarchive.Target{{ProviderID: "p", Model: "m", Name: "first", Enabled: 1}}, Runs: []pelicanarchive.Run{{ID: 1, ProviderID: "p", Model: "m", Name: "first", Grade: "correct", Expected: 21, Attempts: 1, CreatedAt: now.Format(time.RFC3339Nano), SVG: `<svg><text>21</text></svg>`, PromptHash: pelicanarchive.PromptHash("question")}}}
	n, err := ImportPelicanSnapshot(db, s, row.Revision)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	n, err = ImportPelicanSnapshot(db, s, row.Revision)
	require.NoError(t, err)
	require.Zero(t, n)
	require.NoError(t, db.Create(&Channel{Id: 1, Key: "fixture-secret", Models: "m", Group: "A,B"}).Error)
	targetID := PelicanTargetID("fixture", "p", "m")
	require.NoError(t, SavePelicanMapping(db, targetID, 1, "m", false))
	require.NoError(t, db.Create(&PelicanTarget{ID: "second-target", Present: true}).Error)
	require.Error(t, SavePelicanMapping(db, "second-target", 1, "m", false))
	require.NoError(t, db.Delete(&PelicanTarget{}, "id = ?", "second-target").Error)
	s.CapturedAt = now.Add(time.Second).Format(time.RFC3339Nano)
	s.Targets[0].Name = "renamed"
	n, err = ImportPelicanSnapshot(db, s, row.Revision)
	require.NoError(t, err)
	require.Zero(t, n)
	var target PelicanTarget
	require.NoError(t, db.First(&target, "id = ?", targetID).Error)
	require.Equal(t, 1, target.ChannelID)
	require.Equal(t, "renamed", target.ProviderName)
	s.CapturedAt = now.Add(2 * time.Second).Format(time.RFC3339Nano)
	s.Runs[0].Grade = "wrong"
	n, err = ImportPelicanSnapshot(db, s, row.Revision)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	var records []PelicanRecord
	require.NoError(t, db.Find(&records).Error)
	require.Len(t, records, 2)
	var count int64
	require.NoError(t, db.Model(&PelicanRecord{}).Where("active = ?", true).Count(&count).Error)
	require.EqualValues(t, 1, count)
	for _, r := range records {
		decoded, err := r.Run()
		require.NoError(t, err)
		if r.Active {
			require.Equal(t, "wrong", decoded.Grade)
		} else {
			require.Equal(t, "correct", decoded.Grade)
		}
	}
	// Restoration reuses the original evidence ID without creating a third row.
	s.CapturedAt = now.Add(2500 * time.Millisecond).Format(time.RFC3339Nano)
	s.Runs[0].Grade = "correct"
	n, err = ImportPelicanSnapshot(db, s, row.Revision)
	require.NoError(t, err)
	require.Zero(t, n)
	var restored PelicanRecord
	require.NoError(t, db.Where("active = ?", true).First(&restored).Error)
	require.Equal(t, "correct", restored.Grade)
	// Capacity rejection must roll back records and the snapshot cursor together.
	require.NoError(t, db.Model(&restored).Update("stored_bytes", int64(512<<20)).Error)
	s.CapturedAt = now.Add(2600 * time.Millisecond).Format(time.RFC3339Nano)
	s.Runs[0].Grade = "no_svg"
	_, err = ImportPelicanSnapshot(db, s, row.Revision)
	require.EqualError(t, err, "archive_capacity_reached")
	current, err := GetPelicanControl(db)
	require.NoError(t, err)
	require.Equal(t, now.Add(2500*time.Millisecond).Format(time.RFC3339Nano), current.LastCapturedAt)
	require.NoError(t, db.Where("active = ?", true).First(&restored).Error)
	require.Equal(t, "correct", restored.Grade)
	s.SourceID = "wrong-source"
	_, err = ImportPelicanSnapshot(db, s, row.Revision)
	require.Error(t, err)
	require.NoError(t, db.Model(&PelicanTarget{}).Where("present = ?", true).Count(&count).Error)
	require.EqualValues(t, 1, count)
	row, err = UpdatePelicanControl(db, row.Revision, 1, "hide_and_pause", func(_ *gorm.DB, c *PelicanControl) error { c.Visible = false; c.SyncEnabled = false; return nil })
	require.NoError(t, err)
	s.SourceID = "fixture"
	s.CapturedAt = now.Add(3 * time.Second).Format(time.RFC3339Nano)
	_, err = ImportPelicanSnapshot(db, s, row.Revision)
	require.Error(t, err)
	require.NoError(t, db.Model(&PelicanRecord{}).Count(&count).Error)
	require.EqualValues(t, 2, count)
	_, err = UpdatePelicanControl(db, 0, 1, "show", func(_ *gorm.DB, c *PelicanControl) error { c.Visible = true; return nil })
	require.ErrorIs(t, err, ErrCapabilityConflict)
}
