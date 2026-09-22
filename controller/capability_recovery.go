package controller

import (
	"context"
	"encoding/base64"
	"errors"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/capabilitytest"
	"github.com/QuantumNous/new-api/service"
	"gorm.io/gorm"
)

var errCapabilityRecoverySourceChanged = errors.New("source_configuration_changed")

func capabilityRecoveryProfile(round model.CapabilityRound, run model.CapabilityRun) (model.CapabilityProfile, error) {
	var config model.CapabilityConfig
	if err := common.UnmarshalJsonStr(round.Config, &config); err != nil {
		return model.CapabilityProfile{}, err
	}
	for _, profile := range config.Models {
		if profile.Model == run.Model && profile.Enabled {
			return profile, nil
		}
	}
	return model.CapabilityProfile{}, errors.New("source_profile_missing")
}

func capabilityRecoverySourceGuard(run model.CapabilityRun, round model.CapabilityRound) func(*gorm.DB, model.CapabilityControl) error {
	return func(tx *gorm.DB, control model.CapabilityControl) error {
		profile, err := capabilityRecoveryProfile(round, run)
		if err != nil {
			return err
		}
		config, err := control.Config()
		if err != nil {
			return err
		}
		if !capabilityProfileCurrent(config, &capabilityWork{run: run, profile: profile}, "source") {
			return errCapabilityRecoverySourceChanged
		}
		var channel model.Channel
		if err := tx.First(&channel, "id = ?", run.ChannelID).Error; err != nil {
			return err
		}
		if channel.Status != common.ChannelStatusEnabled || run.ConfigurationHash != service.CapabilityConfigurationHash(&channel, profile, capabilitySecret()) {
			return errCapabilityRecoverySourceChanged
		}
		return nil
	}
}

func capabilityRecoverOne(ctx context.Context, row model.CapabilityControl, config model.CapabilityConfig, owner string) {
	deadline := time.Now().Add(185 * time.Second)
	if next := service.CapabilityScheduleTimes(config, time.Now(), 1); len(next) > 0 {
		deadline = minTime(deadline, time.Unix(next[0]-2, 0))
	}
	if time.Until(deadline) < 12*time.Second {
		return
	}
	ctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	job, err := model.ClaimCapabilityRecovery(owner, row.ExecutionRevision)
	if err != nil || job == nil {
		return
	}
	item, status, reason := capabilityProcessRecovery(ctx, row, config, *job)
	raw := ""
	if item != nil {
		data, err := common.Marshal(item)
		if err != nil {
			return
		}
		raw = string(data)
	}
	if err := model.FinishCapabilityRecovery(*job, status, reason, raw); err != nil {
		common.SysError("capability recovery checkpoint not committed; original snapshot retained")
	}
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

// Reuses complete generation evidence, including the crash window between
// saving an attempt and the normal worker's result checkpoint. Never calls
// the tested model. Old snapshots and test timestamps remain immutable.
func capabilityProcessRecovery(ctx context.Context, row model.CapabilityControl, config model.CapabilityConfig, job model.CapabilityRecoveryJob) (*capabilityItem, string, string) {
	var run model.CapabilityRun
	var round model.CapabilityRound
	if err := model.DB.First(&run, "id = ?", job.RunID).Error; err != nil {
		return capabilityRecoveryReadFailure(err, "source_missing")
	}
	if err := model.DB.First(&round, "id = ?", run.RoundID).Error; err != nil {
		return capabilityRecoveryReadFailure(err, "source_missing")
	}
	if round.Suite != capabilitytest.SuiteVersion {
		return nil, "blocked", "suite_version_changed"
	}
	if err := capabilityRecoverySourceGuard(run, round)(model.DB, row); err != nil {
		if errors.Is(err, errCapabilityRecoverySourceChanged) || errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, "cancelled", "source_configuration_changed"
		}
		return nil, "pending", "source_configuration_unavailable"
	}
	var bindings []model.CapabilityBinding
	if model.DB.Where("run_id = ? AND withdrawn = ? AND dispatched_at > ?", run.ID, false, 0).Find(&bindings).Error != nil {
		return nil, "pending", "topology_unavailable"
	}
	work := capabilityWork{run: run, recovery: &job}
	work.run.Owner = job.Owner
	for _, binding := range bindings {
		work.targets = append(work.targets, service.CapabilityTarget{GroupUID: binding.GroupUID, Group: binding.RoutingKey, ChannelID: run.ChannelID, Model: run.Model})
	}
	live, err := capabilityLiveSubscribers(model.DB, config, &work)
	if err != nil {
		return nil, "pending", "topology_unavailable"
	}
	if len(live) == 0 {
		return nil, "cancelled", "target_no_longer_eligible"
	}
	var questions []capabilitytest.Question
	if common.UnmarshalJsonStr(round.Questions, &questions) != nil {
		return nil, "blocked", "question_evidence_invalid"
	}
	var question *capabilitytest.Question
	for i := range questions {
		if questions[i].Kind == job.Kind {
			question = &questions[i]
			break
		}
	}
	if question == nil {
		return nil, "blocked", "question_evidence_missing"
	}
	var attempt model.CapabilityAttempt
	if err := model.DB.First(&attempt, "run_id = ? AND kind = ?", run.ID, job.Kind).Error; err != nil {
		return capabilityRecoveryReadFailure(err, "complete_generation_evidence_required")
	}
	if attempt.Status != "complete" || attempt.Prompt != question.Prompt {
		return nil, "blocked", "complete_generation_evidence_required"
	}
	item := capabilityItem{Kind: job.Kind, Question: *question, Answer: attempt.Answer, Status: "pending"}
	var original []capabilityItem
	if run.Result != "" && common.UnmarshalJsonStr(run.Result, &original) != nil {
		return nil, "blocked", "result_evidence_invalid"
	}
	for _, saved := range original {
		if saved.Kind == job.Kind {
			if saved.Status == "graded" || saved.Status == "ungraded" {
				return nil, "complete", "already_evaluated"
			}
			item.Artifact = saved.Artifact
			item.Animation = saved.Animation
		}
	}
	// A previous local retry may already have rendered this paid answer.
	var prior model.CapabilityEvaluation
	if err := model.DB.Where("recovery_id = ?", job.ID).Order("revision desc").First(&prior).Error; err == nil {
		var saved capabilityItem
		if common.UnmarshalJsonStr(prior.Result, &saved) == nil && saved.Answer == item.Answer && saved.Question.Hash == item.Question.Hash {
			item.Artifact = saved.Artifact
			item.Animation = saved.Animation
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, "pending", "recovery_evidence_unavailable"
	}
	if job.Kind == "logic" {
		item.Checks, err = capabilitytest.ParseLogic(item.Answer, item.Question.Expected)
		if err != nil {
			item.Status, item.Reason = "ungraded", "invalid_logic_response"
		} else {
			item.Status = "graded"
		}
		return &item, "complete", item.Reason
	}
	png, err := capabilityRenderItem(ctx, &item)
	if err != nil {
		item.Reason = err.Error()
		if item.Status == "ungraded" {
			return &item, "complete", item.Reason
		}
		return &item, "pending", item.Reason
	}
	if job.Kind == "geometry" {
		return &item, "complete", ""
	}
	item.Status = "pending_review"
	var judgeAttempt model.CapabilityAttempt
	err = model.DB.First(&judgeAttempt, "run_id = ? AND kind = ?", run.ID, "judge").Error
	answer := ""
	if err == nil {
		// Never duplicate an uncertain paid dispatch, nor retry an incorrect,
		// malformed or inconclusive completed review just to get a nicer score.
		if judgeAttempt.Status != "complete" {
			item.Reason = "judge_outcome_unknown"
			return &item, "blocked", item.Reason
		}
		if judgeAttempt.ImageSHA256 != capabilityJudgeArtifact(item) || judgeAttempt.Prompt != capabilitytest.JudgePrompt(item.Question) {
			return &item, "blocked", "judge_evidence_mismatch"
		}
		answer = judgeAttempt.Answer
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return &item, "pending", "judge_evidence_unavailable"
	} else {
		if ctx.Err() != nil {
			return &item, "pending", "recovery_window_closed"
		}
		// Use the available schedule window, as normal round calls do. Requiring
		// the full 180s maximum would starve recovery under short custom plans.
		// A timed-out dispatched review stays unknown and is never resent.
		if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) < 12*time.Second {
			return &item, "pending", "recovery_window_closed"
		}
		var frozen model.CapabilityConfig
		if common.UnmarshalJsonStr(round.Config, &frozen) != nil || frozen.JudgeChannelID != config.JudgeChannelID || frozen.JudgeModel != config.JudgeModel {
			return &item, "blocked", "judge_configuration_changed"
		}
		ch, err := model.GetChannelById(config.JudgeChannelID, true)
		if err != nil || ch.Status != common.ChannelStatusEnabled || ch.Id == run.ChannelID {
			return &item, "pending", "independent_judge_unavailable"
		}
		key, slot, keyErr := ch.GetNextEnabledKey()
		if keyErr != nil {
			return &item, "pending", "judge_credential_unavailable"
		}
		work.channel, work.key, work.slot = ch, key, slot
		work.profile = model.CapabilityProfile{Model: config.JudgeModel, Protocol: "chat", MaxTokens: 1024, Enabled: true}
		response := capabilityCall(ctx, row, config, round, &work, "judge", capabilitytest.JudgePrompt(item.Question), "data:image/png;base64,"+base64.StdEncoding.EncodeToString(png))
		if response.Reason != "" {
			var count int64
			if model.DB.Model(&model.CapabilityAttempt{}).Where("run_id = ? AND kind = ?", run.ID, "judge").Count(&count).Error != nil {
				return &item, "pending", "judge_evidence_unavailable"
			}
			if count > 0 {
				item.Reason = "judge_outcome_unknown"
				return &item, "blocked", item.Reason
			}
			item.Reason = response.Reason
			return &item, "pending", item.Reason
		}
		answer = response.Answer
	}
	judgment, err := capabilitytest.ParseJudgment(answer)
	if err != nil {
		item.Reason = "invalid_judge_response"
		return &item, "blocked", item.Reason
	}
	item.Judgment = &judgment
	capabilityApplyMotionEvidence(&item)
	item.Status = "graded"
	for _, check := range judgment.Items {
		if check.Status == "uncertain" {
			item.Status, item.Reason = "pending_review", "judge_uncertain"
			return &item, "blocked", item.Reason
		}
	}
	return &item, "complete", ""
}

// A temporary storage outage must not permanently discard durable work.
func capabilityRecoveryReadFailure(err error, missingReason string) (*capabilityItem, string, string) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, "blocked", missingReason
	}
	return nil, "pending", "recovery_evidence_unavailable"
}
