package service

import (
	"context"
	"errors"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/pelicanarchive"
	"gorm.io/gorm"
)

var pelicanSyncMu sync.Mutex

func SyncPelicanArchive(ctx context.Context) (int, error) {
	if !common.IsMasterNode {
		return 0, errors.New("候选实例不执行同步")
	}
	if !pelicanSyncMu.TryLock() {
		return 0, errors.New("同步正在进行")
	}
	defer pelicanSyncMu.Unlock()
	db := model.DB.WithContext(ctx)
	row, err := model.GetPelicanControl(db)
	if err != nil {
		return 0, errors.New("无法读取同步设置")
	}
	if !row.SyncEnabled {
		return 0, errors.New("同步已暂停，请先恢复同步")
	}
	path, source := os.Getenv("PELICAN_ARCHIVE_FILE"), os.Getenv("PELICAN_SOURCE_ID")
	fail := func(code string) (int, error) {
		_ = db.Model(&model.PelicanControl{}).Where("id = ? AND revision = ?", 1, row.Revision).Update("last_error", code).Error
		return 0, errors.New(code)
	}
	if path == "" || source == "" {
		return fail("archive_source_not_configured")
	}
	f, err := os.Open(path)
	if err != nil {
		return fail("archive_file_unavailable")
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > pelicanarchive.MaxSnapshotBytes {
		return fail("archive_file_invalid")
	}
	snapshot, err := pelicanarchive.Decode(f, source, time.Now())
	if err != nil {
		return fail(err.Error())
	}
	n, err := model.ImportPelicanSnapshot(db, snapshot, row.Revision)
	if err != nil {
		if err.Error() == "archive_capacity_reached" {
			return fail("archive_capacity_reached")
		}
		return fail("archive_import_rejected")
	}
	return n, nil
}
func StartPelicanArchiveWorker() {
	if !common.IsMasterNode {
		return
	}
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		lastAttempt := time.Time{}
		for range ticker.C {
			// Identity discovery never invokes the old test executor.
			if err := model.ReconcileCapabilityGroups(); err != nil {
				continue
			}
			row, err := model.GetPelicanControl(model.DB)
			if err != nil || !row.SyncEnabled {
				continue
			}
			if time.Since(lastAttempt) < time.Duration(row.IntervalMinutes)*time.Minute {
				continue
			}
			lastAttempt = time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			_, _ = SyncPelicanArchive(ctx)
			cancel()
		}
	}()
}

type PelicanSummary struct {
	ID         string `json:"id"`
	Time       int64  `json:"time"`
	Grade      string `json:"grade"`
	HasArtwork bool   `json:"has_artwork"`
	Model      string `json:"model"`
}
type PelicanGroupResults struct {
	Group   CapabilityGroupView `json:"group"`
	Model   string              `json:"model"`
	Gallery []PelicanSummary    `json:"gallery"`
	History []PelicanSummary    `json:"history"`
	Targets int                 `json:"targets"`
}

// Eligibility is recalculated from live channel + ability relationships on
// every request, including artifact/record direct links.
func PelicanTargetsForGroup(db *gorm.DB, group, modelName string) ([]model.PelicanTarget, error) {
	var targets []model.PelicanTarget
	if err := db.Where("present = ? AND enabled = ? AND hidden = ? AND channel_id > ?", true, true, false, 0).Find(&targets).Error; err != nil {
		return nil, err
	}
	topology, _, err := loadPelicanTopology(db)
	if err != nil {
		return nil, err
	}
	out := []model.PelicanTarget{}
	for _, t := range targets {
		if !topology.groupEligible(t, group) {
			continue
		}
		if modelName != "" && modelName != t.DisplayModel {
			continue
		}
		out = append(out, t)
	}
	return out, nil
}
func splitPelican(s string) []string { // existing channel representation
	return strings.Split(s, ",")
}

func PelicanResults(db *gorm.DB, group CapabilityGroupView, modelName string, history bool, size int) (PelicanGroupResults, error) {
	result := PelicanGroupResults{Group: group, Model: modelName, Gallery: []PelicanSummary{}, History: []PelicanSummary{}}
	targets, err := PelicanTargetsForGroup(db, group.RoutingKey, modelName)
	if err != nil {
		return result, err
	}
	result.Targets = len(targets)
	seen := map[string]bool{}
	for _, target := range targets {
		var rows []model.PelicanRecord
		if err = db.Select("id", "tested_at", "grade", "preview", "external_id").Where("target_id = ? AND active = ?", target.ID, true).Order("external_id DESC").Limit(30).Find(&rows).Error; err != nil {
			return result, err
		}
		if len(rows) == 0 {
			continue
		}
		// Only the latest completed result represents this target. Never silently
		// replace a new non-matching result with an older passing one.
		for i, r := range rows {
			summary := PelicanSummary{ID: r.ID, Time: r.TestedAt, Grade: r.Grade, HasArtwork: r.Preview, Model: target.DisplayModel}
			if i == 0 {
				result.Gallery = append(result.Gallery, summary)
			}
			if history && !seen[r.ID] {
				result.History = append(result.History, summary)
				seen[r.ID] = true
			}
		}
	}
	sort.Slice(result.Gallery, func(i, j int) bool {
		a, b := result.Gallery[i], result.Gallery[j]
		if a.HasArtwork != b.HasArtwork {
			return a.HasArtwork
		}
		if (a.Grade == "correct") != (b.Grade == "correct") {
			return a.Grade == "correct"
		}
		if a.Time != b.Time {
			return a.Time > b.Time
		}
		return a.ID < b.ID
	})
	if size < 1 || size > 3 {
		size = 3
	}
	if len(result.Gallery) > size {
		result.Gallery = result.Gallery[:size]
	}
	sort.Slice(result.History, func(i, j int) bool {
		if result.History[i].Time == result.History[j].Time {
			return result.History[i].ID < result.History[j].ID
		}
		return result.History[i].Time > result.History[j].Time
	})
	if len(result.History) > 90 {
		result.History = result.History[:90]
	}
	return result, nil
}
