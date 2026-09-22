package model

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Recovery only consumes durable answers. It never regenerates a test answer.
type CapabilityRecoveryJob struct {
	ID                string `json:"id" gorm:"primaryKey;type:varchar(36)"`
	RunID             string `json:"run_id" gorm:"uniqueIndex:idx_capability_recovery_task;type:varchar(36)"`
	Kind              string `json:"kind" gorm:"uniqueIndex:idx_capability_recovery_task;type:varchar(32)"`
	ExecutionRevision int64  `json:"execution_revision"`
	Status            string `json:"status" gorm:"index;type:varchar(32)"`
	Reason            string `json:"reason" gorm:"type:varchar(80)"`
	Owner             string `json:"-" gorm:"type:varchar(36)"`
	Attempts          int    `json:"attempts"`
	NextAt            int64  `json:"next_at" gorm:"index"`
	CreatedAt         int64  `json:"created_at"`
	UpdatedAt         int64  `json:"updated_at"`
}

// An append-only evaluation, separate from the original public round snapshot.
type CapabilityEvaluation struct {
	ID         string `json:"id" gorm:"primaryKey;type:varchar(36)"`
	RecoveryID string `json:"recovery_id" gorm:"uniqueIndex:idx_capability_evaluation_revision;type:varchar(36)"`
	Revision   int    `json:"revision" gorm:"uniqueIndex:idx_capability_evaluation_revision"`
	RunID      string `json:"run_id" gorm:"index;type:varchar(36)"`
	Kind       string `json:"kind" gorm:"type:varchar(32)"`
	Result     string `json:"-" gorm:"type:text"`
	CreatedAt  int64  `json:"created_at"`
}

func EnqueueCapabilityRecovery(tx *gorm.DB, runID, kind string, revision int64) error {
	if kind != "logic" && kind != "geometry" && kind != "scene" {
		return errors.New("invalid_recovery_kind")
	}
	now := time.Now().Unix()
	job := CapabilityRecoveryJob{ID: uuid.NewString(), RunID: runID, Kind: kind, ExecutionRevision: revision, Status: "pending", NextAt: now, CreatedAt: now, UpdatedAt: now}
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&job).Error
}

// The control lock serializes stop with claims and completed revisions on all
// three databases; the worker lock fences a stale leader after lease takeover.
func LockCapabilityRecovery(tx *gorm.DB, owner string, revision int64) error {
	if err := tx.Model(&CapabilityControl{}).Where("id = ?", 1).UpdateColumn("revision", gorm.Expr("revision")).Error; err != nil {
		return err
	}
	var control CapabilityControl
	if err := tx.First(&control, 1).Error; err != nil {
		return err
	}
	if !control.Running || control.ExecutionRevision != revision {
		return errors.New("control_changed")
	}
	if err := tx.Model(&CapabilityWorker{}).Where("id = ?", 1).UpdateColumn("heartbeat", gorm.Expr("heartbeat")).Error; err != nil {
		return err
	}
	var worker CapabilityWorker
	if err := tx.First(&worker, 1).Error; err != nil {
		return err
	}
	if worker.Owner != owner || worker.Heartbeat < time.Now().Unix()-45 {
		return errors.New("lease_expired")
	}
	return nil
}

func ClaimCapabilityRecovery(owner string, revision int64) (*CapabilityRecoveryJob, error) {
	var job CapabilityRecoveryJob
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := LockCapabilityRecovery(tx, owner, revision); err != nil {
			return err
		}
		now := time.Now().Unix()
		// Operator stop/resume never resurrects previously queued work.
		if err := tx.Model(&CapabilityRecoveryJob{}).Where("status IN ? AND (execution_revision <> ? OR created_at < ?)", []string{"pending", "running"}, revision, now-30*86400).Updates(map[string]any{"status": "cancelled", "reason": "expired_or_control_changed", "updated_at": now}).Error; err != nil {
			return err
		}
		runs := tx.Model(&CapabilityRun{}).Select("id").Where("status IN ?", []string{"complete", "cancelled", "outcome_unknown"})
		found := tx.Where("run_id IN (?) AND execution_revision = ? AND ((status = ? AND next_at <= ?) OR (status = ? AND owner <> ?))", runs, revision, "pending", now, "running", owner).Order("next_at asc, id asc").Limit(1).Find(&job)
		if found.Error != nil {
			return found.Error
		}
		if found.RowsAffected == 0 {
			return nil
		}
		result := tx.Model(&CapabilityRecoveryJob{}).Where("id = ? AND status = ? AND owner = ?", job.ID, job.Status, job.Owner).Updates(map[string]any{"status": "running", "owner": owner, "attempts": job.Attempts + 1, "updated_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("recovery_claim_lost")
		}
		job.Status, job.Owner, job.Attempts = "running", owner, job.Attempts+1
		return nil
	})
	if err == nil && job.ID == "" {
		return nil, nil
	}
	return &job, err
}

func FinishCapabilityRecovery(job CapabilityRecoveryJob, status, reason, item string) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := LockCapabilityRecovery(tx, job.Owner, job.ExecutionRevision); err != nil {
			return err
		}
		var current CapabilityRecoveryJob
		if err := tx.First(&current, "id = ? AND status = ? AND owner = ? AND attempts = ?", job.ID, "running", job.Owner, job.Attempts).Error; err != nil {
			return errors.New("recovery_claim_lost")
		}
		now := time.Now().Unix()
		if item != "" {
			var prior CapabilityEvaluation
			found := tx.Where("recovery_id = ?", job.ID).Order("revision desc").Limit(1).Find(&prior)
			if found.Error != nil {
				return found.Error
			}
			// Repeated infrastructure checks with identical evidence are not new
			// evaluations. Avoid duplicating large saved answers on every retry.
			if found.RowsAffected == 0 || prior.Result != item {
				revision := CapabilityEvaluation{ID: uuid.NewString(), RecoveryID: job.ID, Revision: prior.Revision + 1, RunID: job.RunID, Kind: job.Kind, Result: item, CreatedAt: now}
				if err := tx.Create(&revision).Error; err != nil {
					return err
				}
			}
		}
		delay := int64(60) << min(job.Attempts-1, 6)
		return tx.Model(&current).Updates(map[string]any{"status": status, "reason": reason, "next_at": now + min(delay, 3600), "updated_at": now}).Error
	})
}
