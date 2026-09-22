package controller

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Finalize and move all group/model publication pointers in one transaction.
// Failed rounds retain the last published pointer, not an empty replacement.
func capabilityPublishRound(round model.CapabilityRound, status string) error {
	return model.DB.Transaction(func(tx *gorm.DB) error {
		// Serialize publication with stop/hide-and-stop. Holding the same
		// control lock as dispatch gives both actions a definite ordering.
		if err := tx.Model(&model.CapabilityControl{}).Where("id = ?", 1).UpdateColumn("revision", gorm.Expr("revision")).Error; err != nil {
			return err
		}
		var control model.CapabilityControl
		if err := tx.First(&control, 1).Error; err != nil {
			return err
		}
		if status == "complete" && (!control.Running || control.ExecutionRevision != round.ExecutionRevision) {
			status = "interrupted"
		}
		if err := tx.Model(&model.CapabilityRound{}).Where("id = ?", round.ID).UpdateColumn("completed_at", gorm.Expr("completed_at")).Error; err != nil {
			return err
		}
		var current model.CapabilityRound
		if err := tx.First(&current, "id = ? AND owner = ?", round.ID, round.Owner).Error; err != nil {
			return err
		}
		if current.Status != "running" {
			return errors.New("round_already_finalized")
		}
		var worker model.CapabilityWorker
		if err := tx.Model(&model.CapabilityWorker{}).Where("id = ?", 1).UpdateColumn("heartbeat", gorm.Expr("heartbeat")).Error; err != nil {
			return err
		}
		if err := tx.First(&worker, 1).Error; err != nil {
			return err
		}
		if worker.Owner != round.Owner || worker.Heartbeat < time.Now().Unix()-45 {
			return errors.New("lease_expired")
		}
		var runs []model.CapabilityRun
		if err := tx.Where("round_id = ?", round.ID).Find(&runs).Error; err != nil {
			return err
		}
		byID := map[string]model.CapabilityRun{}
		for _, run := range runs {
			if run.Status == "running" {
				return errors.New("round_has_unfinished_runs")
			}
			byID[run.ID] = run
		}
		var bindings []model.CapabilityBinding
		if len(runs) > 0 {
			ids := make([]string, 0, len(runs))
			for _, run := range runs {
				ids = append(ids, run.ID)
			}
			for start := 0; start < len(ids); start += 200 {
				var batch []model.CapabilityBinding
				if err := tx.Where("run_id IN ? AND withdrawn = ? AND dispatched_at > ?", ids[start:min(start+200, len(ids))], false, 0).Find(&batch).Error; err != nil {
					return err
				}
				bindings = append(bindings, batch...)
			}
		}
		grouped := map[string][]string{}
		snapshots := map[string]model.CapabilitySnapshot{}
		hasWork := map[string]bool{}
		for _, binding := range bindings {
			run := byID[binding.RunID]
			if run.Status != "complete" && run.Status != "cancelled" {
				continue
			}
			var items []capabilityItem
			if common.UnmarshalJsonStr(run.Result, &items) != nil {
				continue
			}
			id := fmt.Sprintf("%x", sha256.Sum256([]byte(binding.GroupUID+"\x00"+run.Model)))
			for _, item := range items {
				if item.Artifact != "" {
					hasWork[id] = true
				}
			}
			grouped[id] = append(grouped[id], binding.PublicID)
			snapshots[id] = model.CapabilitySnapshot{ID: id, GroupUID: binding.GroupUID, Model: run.Model, RoundID: round.ID, Slot: round.Slot, PublishedAt: time.Now().Unix()}
		}
		for id, ids := range grouped {
			if !hasWork[id] || status != "complete" {
				continue
			}
			snapshot := snapshots[id]
			raw, err := common.Marshal(ids)
			if err != nil {
				return err
			}
			snapshot.Bindings = string(raw)
			if err = tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&snapshot).Error; err != nil {
				return err
			}
		}
		return tx.Model(&model.CapabilityRound{}).Where("id = ? AND owner = ? AND status = ?", round.ID, round.Owner, "running").Updates(map[string]any{"status": status, "completed_at": time.Now().Unix()}).Error
	})
}
