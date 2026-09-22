package controller

import (
	"context"
	"errors"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"gorm.io/gorm"
)

// A shared request belongs only to the subscriptions frozen for that round.
// A new membership never retroactively inherits a completed sample.
func capabilityLiveSubscribers(db *gorm.DB, config model.CapabilityConfig, job *capabilityWork) (map[string]bool, error) {
	targets, err := service.DiscoverCapabilityTargetsWithDB(db, config)
	if err != nil {
		return nil, err
	}
	live := map[string]bool{}
	var withdrawn []model.CapabilityBinding
	if err := db.Where("run_id = ? AND withdrawn = ?", job.run.ID, true).Find(&withdrawn).Error; err != nil {
		return nil, err
	}
	removed := map[string]bool{}
	for _, binding := range withdrawn {
		removed[binding.GroupUID] = true
	}
	for _, old := range job.targets {
		if removed[old.GroupUID] {
			continue
		}
		for _, current := range targets {
			if current.Eligible && old.GroupUID == current.GroupUID && old.ChannelID == current.ChannelID && old.Model == current.Model {
				live[old.GroupUID] = true
			}
		}
	}
	return live, nil
}

func capabilityDispatchGuard(liveRevision int64, job *capabilityWork, channel *model.Channel, target service.CapabilityTarget) func(*gorm.DB, model.CapabilityControl) error {
	return func(tx *gorm.DB, control model.CapabilityControl) error {
		if control.Revision != liveRevision {
			return errors.New("configuration_changed_before_dispatch")
		}
		config, err := control.Config()
		if err != nil {
			return err
		}
		var fresh model.Channel
		if err := tx.First(&fresh, "id = ?", channel.Id).Error; err != nil {
			return err
		}
		if fresh.Status != common.ChannelStatusEnabled || service.CapabilityConfigurationHash(&fresh, job.profile, capabilitySecret()) != service.CapabilityConfigurationHash(channel, job.profile, capabilitySecret()) {
			return errors.New("channel_changed_before_dispatch")
		}
		live, err := capabilityLiveSubscribers(tx, config, job)
		if err != nil {
			return err
		}
		if !live[target.GroupUID] {
			return errors.New("target_changed_before_dispatch")
		}
		for _, previous := range job.targets {
			query := tx.Model(&model.CapabilityBinding{}).Where("run_id = ? AND group_uid = ?", job.run.ID, previous.GroupUID)
			if !live[previous.GroupUID] {
				if err := query.Update("withdrawn", true).Error; err != nil {
					return err
				}
			} else if err := query.Where("dispatched_at = ?", 0).Update("dispatched_at", time.Now().Unix()).Error; err != nil {
				return err
			}
		}
		return nil
	}
}

// Best-effort cancellation cannot undo upstream billing. Dispatch markers are
// retained even when the last subscriber disappears while a call is in flight.
func capabilityWatchCall(ctx context.Context, cancel context.CancelFunc, row model.CapabilityControl, job *capabilityWork, channel *model.Channel, kind string) func() {
	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-done:
				return
			case <-ticker.C:
				current, err := model.GetCapabilityControl()
				if err != nil || !current.Running || current.ExecutionRevision != row.ExecutionRevision {
					cancel()
					return
				}
				config, err := current.Config()
				if err != nil {
					cancel()
					return
				}
				if !capabilityProfileCurrent(config, job, kind) {
					cancel()
					return
				}
				live, err := capabilityLiveSubscribers(model.DB, config, job)
				if err != nil {
					cancel()
					return
				}
				for _, previous := range job.targets {
					if !live[previous.GroupUID] {
						if err := model.DB.Model(&model.CapabilityBinding{}).Where("run_id = ? AND group_uid = ?", job.run.ID, previous.GroupUID).Update("withdrawn", true).Error; err != nil {
							cancel()
							return
						}
					}
				}
				if len(live) == 0 {
					cancel()
					return
				}
				fresh, err := model.GetChannelById(channel.Id, true)
				if err != nil || fresh.Status != common.ChannelStatusEnabled || service.CapabilityConfigurationHash(fresh, job.profile, capabilitySecret()) != service.CapabilityConfigurationHash(channel, job.profile, capabilitySecret()) {
					cancel()
					return
				}
			}
		}
	}()
	return func() { close(done); <-stopped }
}

func capabilityProfileCurrent(config model.CapabilityConfig, job *capabilityWork, kind string) bool {
	if kind == "judge" {
		return config.JudgeChannelID == job.channel.Id && config.JudgeModel == job.profile.Model
	}
	for _, profile := range config.Models {
		if profile.Model == job.run.Model {
			return profile.Enabled && profile == job.profile
		}
	}
	return false
}
