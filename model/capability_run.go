package model

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CapabilityRound struct {
	ID                string `json:"id" gorm:"primaryKey;type:varchar(36)"`
	Slot              int64  `json:"slot" gorm:"uniqueIndex"`
	Revision          int64  `json:"revision"`
	ExecutionRevision int64  `json:"execution_revision"`
	Suite             string `json:"suite" gorm:"type:varchar(80)"`
	Seed              string `json:"-" gorm:"type:varchar(36)"`
	Config            string `json:"-" gorm:"type:text"`
	Questions         string `json:"questions" gorm:"type:text"`
	Status            string `json:"status" gorm:"type:varchar(32)"`
	Owner             string `json:"-" gorm:"type:varchar(36)"`
	Deadline          int64  `json:"deadline"`
	CreatedAt         int64  `json:"created_at"`
	CompletedAt       int64  `json:"completed_at"`
}
type CapabilityRun struct {
	ID                string `json:"id" gorm:"primaryKey;type:varchar(36)"`
	RoundID           string `json:"round_id" gorm:"index;type:varchar(36)"`
	Identity          string `json:"-" gorm:"uniqueIndex;type:varchar(64)"`
	ChannelID         int    `json:"channel_id" gorm:"index"`
	Model             string `json:"model" gorm:"type:varchar(191)"`
	Protocol          string `json:"protocol" gorm:"type:varchar(32)"`
	Fingerprint       string `json:"fingerprint" gorm:"type:varchar(64)"`
	ConfigurationHash string `json:"configuration_hash" gorm:"type:varchar(64)"`
	KeySlot           int    `json:"key_slot"`
	Status            string `json:"status" gorm:"index;type:varchar(32)"`
	Reason            string `json:"reason" gorm:"type:varchar(80)"`
	Result            string `json:"result" gorm:"type:text"`
	Owner             string `json:"-" gorm:"type:varchar(36)"`
	StartedAt         int64  `json:"started_at"`
	CompletedAt       int64  `json:"completed_at"`
}
type CapabilityBinding struct {
	PublicID     string `json:"public_id" gorm:"primaryKey;type:varchar(36)"`
	RunID        string `json:"-" gorm:"index;type:varchar(36)"`
	GroupUID     string `json:"group_uid" gorm:"index;type:varchar(36)"`
	RoutingKey   string `json:"routing_key" gorm:"type:varchar(191)"`
	CreatedAt    int64  `json:"created_at"`
	DispatchedAt int64  `json:"dispatched_at"`
	Withdrawn    bool   `json:"withdrawn"`
}
type CapabilityAttempt struct {
	ID            string `json:"id" gorm:"primaryKey;type:varchar(36)"`
	RunID         string `json:"run_id" gorm:"index;type:varchar(36)"`
	Kind          string `json:"kind" gorm:"type:varchar(32)"`
	ChannelID     int    `json:"channel_id"`
	Status        string `json:"status" gorm:"type:varchar(32)"`
	Prompt        string `json:"prompt" gorm:"type:text"`
	Answer        string `json:"answer" gorm:"type:text"`
	Usage         string `json:"usage" gorm:"type:text"`
	UpstreamModel string `json:"upstream_model" gorm:"type:varchar(191)"`
	ReportedModel string `json:"reported_model" gorm:"type:varchar(191)"`
	RequestID     string `json:"request_id" gorm:"type:varchar(256)"`
	Endpoint      string `json:"endpoint" gorm:"type:varchar(512)"`
	KeySlot       int    `json:"key_slot"`
	UsageSource   string `json:"usage_source" gorm:"type:varchar(32)"`
	RequestHash   string `json:"request_hash" gorm:"type:varchar(64)"`
	ImageSHA256   string `json:"image_sha256,omitempty" gorm:"type:varchar(64)"`
	Reason        string `json:"reason" gorm:"type:varchar(80)"`
	ReserveMicros int64  `json:"reserve_micros"`
	CostSource    string `json:"cost_source" gorm:"type:varchar(40)"`
	StartedAt     int64  `json:"started_at"`
	CompletedAt   int64  `json:"completed_at"`
}
type CapabilityBudgetDay struct {
	Day            string `json:"day" gorm:"primaryKey;type:varchar(10)"`
	ReservedMicros int64  `json:"reserved_micros"`
}
type CapabilityWorker struct {
	ID           uint   `json:"-" gorm:"primaryKey;autoIncrement:false"`
	Owner        string `json:"-" gorm:"type:varchar(36)"`
	Revision     int64  `json:"applied_revision"`
	Heartbeat    int64  `json:"heartbeat"`
	ReconciledAt int64  `json:"reconciled_at"`
	State        string `json:"state" gorm:"type:varchar(32)"`
	Reason       string `json:"reason" gorm:"type:varchar(80)"`
}

// One immutable round selection per group/model. The pointer changes only
// after the round is finalized; in-progress work never changes public works.
type CapabilitySnapshot struct {
	ID          string `json:"-" gorm:"primaryKey;type:varchar(64)"`
	GroupUID    string `json:"group_uid" gorm:"index;type:varchar(36)"`
	Model       string `json:"model" gorm:"type:varchar(191)"`
	RoundID     string `json:"round_id" gorm:"type:varchar(36)"`
	Slot        int64  `json:"slot"`
	Bindings    string `json:"-" gorm:"type:text"`
	PublishedAt int64  `json:"published_at"`
}

func MigrateCapabilitySchema(db *gorm.DB) error {
	return db.AutoMigrate(&CapabilityControl{}, &CapabilityGroupPresentation{}, &CapabilityEvent{}, &CapabilityRound{}, &CapabilityRun{}, &CapabilityBinding{}, &CapabilityAttempt{}, &CapabilityBudgetDay{}, &CapabilityWorker{}, &CapabilitySnapshot{}, &CapabilityRecoveryJob{}, &CapabilityEvaluation{})
}

// DispatchCapabilityAttempt serializes the stop/revision check and budget
// reservation with the dispatch marker. No upstream request may precede it.
// A retained reservation is a conservative bound, NOT reported actual cost.
func DispatchCapabilityAttempt(run CapabilityRun, attempt *CapabilityAttempt, expected int64, limit, reserve int64, guards ...func(*gorm.DB, CapabilityControl) error) error {
	return dispatchCapabilityAttempt(run, attempt, expected, limit, reserve, nil, guards...)
}

func DispatchCapabilityRecoveryAttempt(run CapabilityRun, attempt *CapabilityAttempt, job CapabilityRecoveryJob, limit, reserve int64, guards ...func(*gorm.DB, CapabilityControl) error) error {
	if attempt.Kind != "judge" || job.Kind != "scene" || job.RunID != run.ID {
		return errors.New("recovery_cannot_generate")
	}
	return dispatchCapabilityAttempt(run, attempt, job.ExecutionRevision, limit, reserve, &job, guards...)
}

func dispatchCapabilityAttempt(run CapabilityRun, attempt *CapabilityAttempt, expected int64, limit, reserve int64, recovery *CapabilityRecoveryJob, guards ...func(*gorm.DB, CapabilityControl) error) error {
	if reserve <= 0 || limit < reserve {
		return errors.New("budget_not_configured")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		// UPDATE takes a database write lock in SQLite as well as row locks in
		// MySQL/PostgreSQL; false values are included explicitly.
		r := tx.Model(&CapabilityControl{}).Where("id = ? AND execution_revision = ? AND running = ?", 1, expected, true).UpdateColumn("revision", gorm.Expr("revision"))
		if r.Error != nil {
			return r.Error
		}
		var control CapabilityControl
		if err := tx.First(&control, 1).Error; err != nil {
			return err
		}
		if !control.Running || control.ExecutionRevision != expected {
			return errors.New("control_changed")
		}
		for _, guard := range guards {
			if err := guard(tx, control); err != nil {
				return err
			}
		}
		config, err := control.Config()
		if err != nil {
			return err
		}
		limit = min(limit, config.DailyBudgetMicros)
		reserve = max(reserve, config.CallReserveMicros)
		if reserve <= 0 || limit < reserve {
			return errors.New("budget_not_configured")
		}
		var current CapabilityRun
		if recovery == nil {
			if err := tx.First(&current, "id = ? AND owner = ? AND status = ?", run.ID, run.Owner, "running").Error; err != nil {
				return errors.New("lease_or_run_changed")
			}
		} else {
			if err := tx.First(&current, "id = ? AND status IN ?", run.ID, []string{"complete", "cancelled", "outcome_unknown"}).Error; err != nil {
				return errors.New("recovery_source_unavailable")
			}
			var claimed CapabilityRecoveryJob
			if err := tx.First(&claimed, "id = ? AND status = ? AND owner = ? AND attempts = ? AND execution_revision = ?", recovery.ID, "running", run.Owner, recovery.Attempts, expected).Error; err != nil {
				return errors.New("recovery_claim_lost")
			}
		}
		var existing int64
		if err := tx.Model(&CapabilityAttempt{}).Where("run_id = ? AND kind = ?", run.ID, attempt.Kind).Count(&existing).Error; err != nil {
			return err
		}
		if existing != 0 {
			return errors.New("attempt_already_dispatched")
		}
		var worker CapabilityWorker
		if err := tx.Model(&CapabilityWorker{}).Where("id = ?", 1).UpdateColumn("heartbeat", gorm.Expr("heartbeat")).Error; err != nil {
			return err
		}
		if err := tx.First(&worker, 1).Error; err != nil || worker.Owner != run.Owner || worker.Heartbeat < time.Now().Unix()-45 {
			return errors.New("lease_expired")
		}
		day := time.Now().UTC().Format("2006-01-02")
		seed := CapabilityBudgetDay{Day: day}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&seed).Error; err != nil {
			return err
		}
		r = tx.Model(&CapabilityBudgetDay{}).Where("day = ? AND reserved_micros <= ?", day, limit-reserve).UpdateColumn("reserved_micros", gorm.Expr("reserved_micros + ?", reserve))
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected != 1 {
			return errors.New("daily_budget_exhausted")
		}
		attempt.ID = uuid.NewString()
		attempt.RunID = run.ID
		attempt.Status = "dispatching"
		attempt.ReserveMicros = reserve
		attempt.CostSource = "approved_upper_bound"
		attempt.StartedAt = time.Now().Unix()
		return tx.Create(attempt).Error
	})
}

// Abandoned dispatched calls are never automatically retried: their actual
// upstream completion/billing is unknown after a process crash.
func RecoverCapabilityRuns(owner string) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&CapabilityAttempt{}).Where("status = ?", "dispatching").Updates(map[string]any{"status": "outcome_unknown", "reason": "worker_restarted", "completed_at": time.Now().Unix()}).Error; err != nil {
			return err
		}
		if err := tx.Model(&CapabilityRun{}).Where("status = ? AND owner <> ?", "running", owner).Updates(map[string]any{"status": "outcome_unknown", "reason": "worker_restarted", "completed_at": time.Now().Unix()}).Error; err != nil {
			return err
		}
		return tx.Model(&CapabilityRound{}).Where("status = ? AND owner <> ?", "running", owner).Updates(map[string]any{"status": "interrupted", "completed_at": time.Now().Unix()}).Error
	})
}
