package model

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/pelicanarchive"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PelicanControl struct {
	ID              int    `json:"-" gorm:"primaryKey;autoIncrement:false"`
	Revision        int64  `json:"revision"`
	Visible         bool   `json:"visible"`
	SyncEnabled     bool   `json:"sync_enabled"`
	IntervalMinutes int    `json:"interval_minutes"`
	Presentation    string `json:"-"`
	LastImportedAt  int64  `json:"last_imported_at"`
	LastCapturedAt  string `json:"last_captured_at" gorm:"type:varchar(64)"`
	LastError       string `json:"last_error" gorm:"type:varchar(255)"`
	SourceID        string `json:"source_id" gorm:"type:varchar(191)"`
	SourceConfig    string `json:"-"`
}
type PelicanTarget struct {
	ID              string `json:"id" gorm:"primaryKey;type:varchar(64)"`
	SourceID        string `json:"source_id" gorm:"type:varchar(191);index"`
	ProviderID      string `json:"provider_id" gorm:"type:varchar(191)"`
	ModelName       string `json:"model_name" gorm:"type:varchar(255)"`
	ProviderName    string `json:"provider_name" gorm:"type:text"`
	IntervalMinutes int    `json:"interval_minutes"`
	Enabled         bool   `json:"enabled"`
	Present         bool   `json:"present"`
	ChannelID       int    `json:"channel_id" gorm:"index"`
	DisplayModel    string `json:"display_model" gorm:"type:varchar(255)"`
	Hidden          bool   `json:"hidden"`
}
type PelicanRecord struct {
	ID          string `json:"id" gorm:"primaryKey;type:varchar(64)"`
	TargetID    string `json:"target_id" gorm:"type:varchar(64);index:idx_pelican_record_target"`
	SourceID    string `json:"source_id" gorm:"type:varchar(191);index:idx_pelican_record_source"`
	ExternalID  int64  `json:"external_id" gorm:"index:idx_pelican_record_source"`
	Digest      string `json:"digest" gorm:"type:varchar(64)"`
	Active      bool   `json:"active"`
	Grade       string `json:"grade" gorm:"type:varchar(32)"`
	Preview     bool   `json:"preview"`
	TestedAt    int64  `json:"tested_at"`
	ImportedAt  int64  `json:"imported_at"`
	StoredBytes int64  `json:"stored_bytes"`
	Payload     string `json:"-"`
	Prompt      string `json:"-"`
}
type PelicanEvent struct {
	ID      uint   `json:"id" gorm:"primaryKey"`
	ActorID int    `json:"actor_id"`
	Action  string `json:"action" gorm:"type:varchar(40)"`
	At      int64  `json:"at"`
	Detail  string `json:"detail" gorm:"type:text"`
}

func MigratePelicanSchema(db *gorm.DB) error {
	return db.AutoMigrate(&PelicanControl{}, &PelicanTarget{}, &PelicanRecord{}, &PelicanEvent{}, &CapabilityGroupPresentation{})
}
func DefaultPelicanControl() PelicanControl {
	view, _ := common.Marshal(DefaultCapabilityPresentation())
	return PelicanControl{ID: 1, IntervalMinutes: 5, Presentation: string(view)}
}
func GetPelicanControl(db *gorm.DB) (PelicanControl, error) {
	var row PelicanControl
	err := db.First(&row, 1).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return DefaultPelicanControl(), nil
	}
	return row, err
}
func (p PelicanControl) View() (CapabilityPresentation, error) {
	v := DefaultCapabilityPresentation()
	err := common.UnmarshalJsonStr(p.Presentation, &v)
	return v, err
}
func (p PelicanRecord) Run() (pelicanarchive.Run, error) {
	var r pelicanarchive.Run
	err := common.UnmarshalJsonStr(p.Payload, &r)
	return r, err
}
func UpdatePelicanControl(db *gorm.DB, revision int64, actor int, action string, mutate func(*gorm.DB, *PelicanControl) error) (PelicanControl, error) {
	var row PelicanControl
	err := db.Transaction(func(tx *gorm.DB) error {
		seed := DefaultPelicanControl()
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&seed).Error; err != nil {
			return err
		}
		var err error
		row, err = GetPelicanControl(tx)
		if err != nil {
			return err
		}
		if row.Revision != revision {
			return ErrCapabilityConflict
		}
		if err = mutate(tx, &row); err != nil {
			return err
		}
		if row.IntervalMinutes < 1 || row.IntervalMinutes > 1440 {
			return errors.New("同步间隔须为1至1440分钟")
		}
		row.Revision++
		result := tx.Model(&PelicanControl{}).Where("id = ? AND revision = ?", 1, revision).Updates(map[string]any{"revision": row.Revision, "visible": row.Visible, "sync_enabled": row.SyncEnabled, "interval_minutes": row.IntervalMinutes, "presentation": row.Presentation})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrCapabilityConflict
		}
		detail, _ := common.Marshal(map[string]any{"revision": row.Revision, "visible": row.Visible, "sync_enabled": row.SyncEnabled, "interval_minutes": row.IntervalMinutes, "presentation": row.Presentation})
		return tx.Create(&PelicanEvent{ActorID: actor, Action: action, At: time.Now().Unix(), Detail: string(detail)}).Error
	})
	return row, err
}
func PelicanTargetID(source, provider, model string) string {
	return pelicanarchive.Hash([]byte(source + "\x00" + provider + "\x00" + model))
}

// A whole snapshot is committed or rejected. Old records survive missing or
// failed exports, and changed source records become separately stored revisions.
func ImportPelicanSnapshot(db *gorm.DB, s pelicanarchive.Snapshot, revision int64) (int, error) {
	imported := 0
	err := db.Transaction(func(tx *gorm.DB) error {
		row, err := GetPelicanControl(tx)
		if err != nil {
			return err
		}
		if !row.SyncEnabled || row.Revision != revision {
			return errors.New("sync_paused_or_configuration_changed")
		}
		if row.SourceID != "" && row.SourceID != s.SourceID {
			return errors.New("source_identity_changed")
		}
		capture, _ := time.Parse(time.RFC3339Nano, s.CapturedAt)
		previous, _ := time.Parse(time.RFC3339Nano, row.LastCapturedAt)
		if capture.Before(previous) {
			return errors.New("snapshot_older_than_current")
		}
		if capture.Equal(previous) {
			return nil
		}
		// Claim the control row before touching data. A concurrent hide/pause/import
		// either happens first or waits for this bounded transaction to finish.
		res := tx.Model(&PelicanControl{}).Where("id = ? AND revision = ? AND last_captured_at = ?", 1, revision, row.LastCapturedAt).Update("last_captured_at", s.CapturedAt)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return ErrCapabilityConflict
		}
		if err = tx.Model(&PelicanTarget{}).Where("source_id = ?", s.SourceID).Update("present", false).Error; err != nil {
			return err
		}
		for _, t := range s.Targets {
			target := PelicanTarget{ID: PelicanTargetID(s.SourceID, t.ProviderID, t.Model), SourceID: s.SourceID, ProviderID: t.ProviderID, ModelName: t.Model, ProviderName: t.Name, Present: true, Enabled: t.Enabled == 1, IntervalMinutes: t.IntervalMinutes}
			if err = tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "id"}}, DoUpdates: clause.AssignmentColumns([]string{"provider_name", "present", "enabled", "interval_minutes"})}).Create(&target).Error; err != nil {
				return err
			}
		}
		// Fail closed before an oversized import can exhaust database storage.
		// No automatic removal of evidence; operators archive before increasing limits.
		var usage struct {
			Count int64
			Bytes int64
		}
		if err = tx.Model(&PelicanRecord{}).Select("COUNT(*) AS count, COALESCE(SUM(stored_bytes), 0) AS bytes").Scan(&usage).Error; err != nil {
			return err
		}
		for _, r := range s.Runs {
			raw, err := common.Marshal(r)
			if err != nil {
				return err
			}
			digest := pelicanarchive.Hash(raw)
			recordID := pelicanarchive.Hash([]byte(fmt.Sprintf("%s\x00%d\x00%s", s.SourceID, r.ID, digest)))
			var exists int64
			if err = tx.Model(&PelicanRecord{}).Where("id = ?", recordID).Count(&exists).Error; err != nil {
				return err
			}
			if exists > 0 {
				// A source may restore an earlier version of a record. Activate the
				// matching saved revision rather than leaving the newer one current.
				if err = tx.Model(&PelicanRecord{}).Where("source_id = ? AND external_id = ? AND id <> ?", s.SourceID, r.ID, recordID).Update("active", false).Error; err != nil {
					return err
				}
				if err = tx.Model(&PelicanRecord{}).Where("id = ?", recordID).Updates(map[string]any{"active": true, "preview": pelicanarchive.SafeSVG(r.SVG)}).Error; err != nil {
					return err
				}
				continue
			}
			if err = tx.Model(&PelicanRecord{}).Where("source_id = ? AND external_id = ?", s.SourceID, r.ID).Update("active", false).Error; err != nil {
				return err
			}
			tested, _ := time.Parse(time.RFC3339Nano, r.CreatedAt)
			rec := PelicanRecord{ID: recordID, TargetID: PelicanTargetID(s.SourceID, r.ProviderID, r.Model), SourceID: s.SourceID, ExternalID: r.ID, Digest: digest, Active: true, Grade: r.Grade, Preview: pelicanarchive.SafeSVG(r.SVG), TestedAt: tested.Unix(), ImportedAt: time.Now().Unix(), Payload: string(raw)}
			// Current config is only a hash-matched reconstruction, never a claim
			// that the source stored the historical prompt. The DTO labels this.
			if r.PromptHash == s.Config.PromptHash {
				rec.Prompt = s.Config.Prompt
			}
			rec.StoredBytes = int64(len(rec.Payload) + len(rec.Prompt))
			usage.Count++
			usage.Bytes += rec.StoredBytes
			if usage.Count > 100000 || usage.Bytes > 512<<20 {
				return errors.New("archive_capacity_reached")
			}
			if err = tx.Create(&rec).Error; err != nil {
				return err
			}
			imported++
		}
		config, _ := common.Marshal(s.Config)
		if err = tx.Model(&PelicanControl{}).Where("id = ?", 1).Updates(map[string]any{"last_imported_at": time.Now().Unix(), "last_error": "", "source_id": s.SourceID, "source_config": string(config)}).Error; err != nil {
			return err
		}
		return tx.Create(&PelicanEvent{Action: "sync", At: time.Now().Unix(), Detail: fmt.Sprintf("source=%s records=%d imported=%d", s.SourceID, len(s.Runs), imported)}).Error
	})
	return imported, err
}
func SavePelicanMapping(tx *gorm.DB, id string, channelID int, displayModel string, hidden bool) error {
	var target PelicanTarget
	if err := tx.First(&target, "id = ?", id).Error; err != nil {
		return err
	}
	if channelID != 0 {
		var channel Channel
		if err := tx.Select("id", "models").First(&channel, channelID).Error; err != nil {
			return errors.New("渠道不存在")
		}
		found := false
		for _, m := range strings.Split(channel.Models, ",") {
			found = found || strings.TrimSpace(m) == displayModel
		}
		if !found || displayModel == "" {
			return errors.New("请选择渠道实际支持的模型")
		}
		var duplicates int64
		if err := tx.Model(&PelicanTarget{}).Where("id <> ? AND channel_id = ? AND display_model = ? AND present = ?", id, channelID, displayModel, true).Count(&duplicates).Error; err != nil {
			return err
		}
		if duplicates > 0 {
			return errors.New("此渠道与模型已有来源关联，请先解除原关联")
		}
	} else {
		displayModel = ""
	}
	return tx.Model(&target).Updates(map[string]any{"channel_id": channelID, "display_model": displayModel, "hidden": hidden}).Error
}
