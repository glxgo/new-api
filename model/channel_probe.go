package model

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	ChannelProbeStatusChecking  = "checking"
	ChannelProbeStatusHealthy   = "healthy"
	ChannelProbeStatusDegraded  = "degraded"
	ChannelProbeStatusUnhealthy = "unhealthy"
)

// ChannelProbeRecord is synthetic monitoring data. It is deliberately stored
// outside perf_metrics so active probes can never inflate user request health.
type ChannelProbeRecord struct {
	Id            int    `json:"id"`
	ChannelId     int    `json:"channel_id" gorm:"index:idx_channel_probe_channel_ts,priority:1"`
	ModelName     string `json:"model_name" gorm:"type:varchar(191)"`
	Groups        string `json:"groups" gorm:"type:text"`
	ProbeTs       int64  `json:"probe_ts" gorm:"bigint;index:idx_channel_probe_channel_ts,priority:2;index"`
	Success       bool   `json:"success"`
	LatencyMs     int64  `json:"latency_ms" gorm:"bigint"`
	TtftMs        int64  `json:"ttft_ms" gorm:"bigint"`
	HasTtft       bool   `json:"has_ttft"`
	HttpStatus    int    `json:"http_status"`
	ErrorCode     string `json:"error_code" gorm:"type:varchar(128)"`
	ErrorCategory string `json:"error_category" gorm:"type:varchar(64)"`
	ErrorMessage  string `json:"error_message" gorm:"type:text"`
}

type ChannelProbeState struct {
	ChannelId            int    `json:"channel_id" gorm:"primaryKey"`
	Status               string `json:"status" gorm:"type:varchar(32);index"`
	ModelName            string `json:"model_name" gorm:"type:varchar(191)"`
	LastProbeTs          int64  `json:"last_probe_ts" gorm:"bigint;index"`
	LastSuccessTs        int64  `json:"last_success_ts" gorm:"bigint"`
	LastFailureTs        int64  `json:"last_failure_ts" gorm:"bigint"`
	LastLatencyMs        int64  `json:"last_latency_ms" gorm:"bigint"`
	LastTtftMs           int64  `json:"last_ttft_ms" gorm:"bigint"`
	HasTtft              bool   `json:"has_ttft"`
	LastHttpStatus       int    `json:"last_http_status"`
	LastErrorCode        string `json:"last_error_code" gorm:"type:varchar(128)"`
	LastErrorCategory    string `json:"last_error_category" gorm:"type:varchar(64)"`
	LastErrorMessage     string `json:"last_error_message" gorm:"type:text"`
	ConsecutiveFailures  int    `json:"consecutive_failures"`
	ConsecutiveSuccesses int    `json:"consecutive_successes"`
}

func RecordChannelProbe(record *ChannelProbeRecord, failureThreshold, recoveryThreshold int) (*ChannelProbeState, error) {
	if failureThreshold < 1 {
		failureThreshold = 3
	}
	if recoveryThreshold < 1 {
		recoveryThreshold = 2
	}
	if record.ProbeTs == 0 {
		record.ProbeTs = time.Now().Unix()
	}

	var saved ChannelProbeState
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(record).Error; err != nil {
			return err
		}

		state := ChannelProbeState{ChannelId: record.ChannelId}
		loadErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&state, "channel_id = ?", record.ChannelId).Error
		isNew := loadErr == gorm.ErrRecordNotFound
		if loadErr != nil && !isNew {
			return loadErr
		}

		previousStatus := state.Status
		state.ModelName = record.ModelName
		state.LastProbeTs = record.ProbeTs
		state.LastLatencyMs = record.LatencyMs
		state.LastTtftMs = record.TtftMs
		state.HasTtft = record.HasTtft
		state.LastHttpStatus = record.HttpStatus

		if record.Success {
			state.LastSuccessTs = record.ProbeTs
			state.ConsecutiveSuccesses++
			state.ConsecutiveFailures = 0
			state.LastErrorCode = ""
			state.LastErrorCategory = ""
			state.LastErrorMessage = ""
			if isNew || previousStatus == "" || previousStatus == ChannelProbeStatusHealthy || state.ConsecutiveSuccesses >= recoveryThreshold {
				state.Status = ChannelProbeStatusHealthy
			} else {
				state.Status = ChannelProbeStatusChecking
			}
		} else {
			state.LastFailureTs = record.ProbeTs
			state.ConsecutiveFailures++
			state.ConsecutiveSuccesses = 0
			state.LastErrorCode = record.ErrorCode
			state.LastErrorCategory = record.ErrorCategory
			state.LastErrorMessage = record.ErrorMessage
			if state.ConsecutiveFailures >= failureThreshold {
				state.Status = ChannelProbeStatusUnhealthy
			} else {
				state.Status = ChannelProbeStatusDegraded
			}
		}

		if err := tx.Save(&state).Error; err != nil {
			return err
		}
		saved = state
		return nil
	})
	return &saved, err
}

func GetChannelProbeStates() ([]ChannelProbeState, error) {
	var states []ChannelProbeState
	err := DB.Order("channel_id ASC").Find(&states).Error
	return states, err
}

func GetChannelProbeRecordsSince(since int64) ([]ChannelProbeRecord, error) {
	var records []ChannelProbeRecord
	err := DB.Where("probe_ts >= ?", since).Order("probe_ts ASC").Find(&records).Error
	return records, err
}

func DeleteChannelProbeRecordsBefore(before int64) error {
	return DB.Where("probe_ts < ?", before).Delete(&ChannelProbeRecord{}).Error
}
