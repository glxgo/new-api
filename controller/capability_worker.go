package controller

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/capabilitytest"
	"github.com/QuantumNous/new-api/service"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type capabilityItem struct {
	Kind      string                       `json:"kind"`
	Question  capabilitytest.Question      `json:"question"`
	Answer    string                       `json:"answer"`
	Status    string                       `json:"status"`
	Reason    string                       `json:"reason,omitempty"`
	Checks    []bool                       `json:"checks,omitempty"`
	Judgment  *capabilitytest.Judgment     `json:"judgment,omitempty"`
	Artifact  string                       `json:"artifact,omitempty"`
	Animation *service.CapabilityAnimation `json:"animation,omitempty"`
}
type capabilityWork struct {
	run      model.CapabilityRun
	targets  []service.CapabilityTarget
	profile  model.CapabilityProfile
	channel  *model.Channel
	key      string
	slot     int
	recovery *model.CapabilityRecoveryJob
}

func capabilitySecret() []byte { return []byte(os.Getenv("CAPABILITY_FINGERPRINT_SECRET")) }
func capabilityReadiness(c model.CapabilityConfig) []string {
	reasons := []string{}
	if os.Getenv("CAPABILITY_EXECUTION_ENABLED") != "true" {
		reasons = append(reasons, "deployment_execution_gate_closed")
	}
	if !common.RedisEnabled || common.RDB == nil {
		reasons = append(reasons, "redis_required")
	}
	if len(capabilitySecret()) < 32 {
		reasons = append(reasons, "fingerprint_secret_required")
	}
	if os.Getenv("CAPABILITY_RENDERER_SOCKET") == "" || service.CapabilityArtifactRoot() == "" {
		reasons = append(reasons, "renderer_and_storage_required")
	}
	if c.DailyBudgetMicros <= 0 || c.CallReserveMicros <= 0 || c.DailyBudgetMicros < c.CallReserveMicros*4 {
		reasons = append(reasons, "approved_budget_required")
	}
	if len(c.Models) == 0 {
		reasons = append(reasons, "model_profiles_required")
	}
	if c.JudgeChannelID <= 0 || c.JudgeModel == "" {
		reasons = append(reasons, "vision_judge_required")
	}
	// Calibration evidence is an operator-reviewed deployment artifact. A
	// database toggle alone must not turn uncalibrated scores into public facts.
	if !capabilityCalibrationValid(c) {
		reasons = append(reasons, "reviewed_calibration_required")
	}
	return reasons
}
func capabilityCalibrationValid(c model.CapabilityConfig) bool {
	path := os.Getenv("CAPABILITY_CALIBRATION_FILE")
	if path == "" {
		return false
	}
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) > 1<<20 {
		return false
	}
	var report struct {
		Suite          string `json:"suite"`
		Renderer       string `json:"renderer"`
		Browser        string `json:"browser"`
		JudgeChannelID int    `json:"judge_channel_id"`
		JudgeModel     string `json:"judge_model"`
		Reviewer       string `json:"reviewer"`
		Samples        []struct {
			TemplateID  string `json:"template_id"`
			Category    string `json:"category"`
			Expected    []bool `json:"expected"`
			Observed    []bool `json:"observed"`
			ImageSHA256 string `json:"image_sha256"`
		} `json:"samples"`
	}
	if common.Unmarshal(raw, &report) != nil || report.Suite != capabilitytest.SuiteVersion || report.Renderer != service.CapabilityAnimationVersion || report.Browser != service.CapabilityAnimationBrowser || report.JudgeChannelID != c.JudgeChannelID || report.JudgeModel != c.JudgeModel || report.Reviewer == "" {
		return false
	}
	categories := []string{"correct", "missing_requirement", "text_substitution", "attractive_incorrect", "blank_truncated", "prompt_injection"}
	counts := map[string]map[string]int{}
	categoryCounts := map[string]int{}
	matches, totals := map[string]int{}, map[string]int{}
	for _, id := range capabilitytest.AnimationTemplateIDs() {
		counts[id] = map[string]int{}
	}
	seen := map[string]bool{}
	for _, s := range report.Samples {
		_, hashErr := hex.DecodeString(s.ImageSHA256)
		if len(s.Expected) != 6 || len(s.Observed) != 6 || len(s.ImageSHA256) != 64 || hashErr != nil || seen[s.ImageSHA256] || counts[s.TemplateID] == nil {
			return false
		}
		knownCategory, allCorrect := false, true
		for _, category := range categories {
			knownCategory = knownCategory || s.Category == category
		}
		for _, want := range s.Expected {
			allCorrect = allCorrect && want
		}
		if !knownCategory || (s.Category == "correct" && !allCorrect) || (s.Category != "correct" && allCorrect) {
			return false
		}
		seen[s.ImageSHA256] = true
		counts[s.TemplateID][s.Category]++
		categoryCounts[s.Category]++
		for i, want := range s.Expected {
			totals[s.TemplateID]++
			if s.Observed[i] == want {
				matches[s.TemplateID]++
			}
			if !want && s.Observed[i] {
				return false
			}
		}
	}
	for id, byCategory := range counts {
		for _, category := range categories {
			if byCategory[category] < 10 || categoryCounts[category] < 10 {
				return false
			}
		}
		if totals[id] == 0 || float64(matches[id])/float64(totals[id]) < .9 {
			return false
		}
	}
	return true
}

// StartCapabilityWorker never schedules on blue/green slave candidates.
func StartCapabilityWorker() {
	if !common.IsMasterNode {
		return
	}
	go capabilityWorkerLoop(context.Background())
}
func capabilityWorkerLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	lastSync := time.Time{}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if time.Since(lastSync) >= time.Minute {
				if model.ReconcileCapabilityGroups() == nil {
					lastSync = time.Now()
				}
			}
			row, err := model.GetCapabilityControl()
			if err != nil || !row.Running {
				continue
			}
			config, err := row.Config()
			if err != nil || len(capabilityReadiness(config)) > 0 {
				continue
			}
			next := service.CapabilityScheduleTimes(config, time.Now().Add(-6*time.Second), 1)
			slot := int64(0)
			if len(next) > 0 && next[0] <= time.Now().Unix() {
				slot = next[0]
			}
			if slot == 0 {
				var count int64
				if model.DB.Model(&model.CapabilityRecoveryJob{}).Where("status IN ? AND next_at <= ?", []string{"pending", "running"}, time.Now().Unix()).Limit(1).Count(&count).Error != nil || count == 0 {
					continue
				}
			}
			capabilityWithLease(ctx, row, config, slot, lastSync.Unix())
		}
	}
}
func capabilityWithLease(parent context.Context, row model.CapabilityControl, config model.CapabilityConfig, slot, reconciled int64) {
	owner := uuid.NewString()
	leaseCtx, cancelLease := context.WithTimeout(parent, 3*time.Second)
	acquired, err := common.RDB.SetNX(leaseCtx, "capability:leader", owner, 90*time.Second).Result()
	cancelLease()
	if err != nil || !acquired {
		return
	}
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	defer func() {
		releaseCtx, done := context.WithTimeout(context.Background(), 2*time.Second)
		defer done()
		_ = common.RDB.Eval(releaseCtx, `if redis.call('get',KEYS[1])==ARGV[1] then return redis.call('del',KEYS[1]) else return 0 end`, []string{"capability:leader"}, owner).Err()
	}()
	worker := model.CapabilityWorker{ID: 1, Owner: owner, Revision: row.Revision, Heartbeat: time.Now().Unix(), ReconciledAt: reconciled, State: "running"}
	if model.DB.Clauses(clause.OnConflict{UpdateAll: true}).Create(&worker).Error != nil {
		return
	}
	if model.RecoverCapabilityRuns(owner) != nil {
		return
	}
	var heartbeat sync.WaitGroup
	heartbeat.Add(1)
	go func() {
		defer heartbeat.Done()
		tick := time.NewTicker(time.Second)
		defer tick.Stop()
		renewed := time.Now()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				current, e := model.GetCapabilityControl()
				if e != nil || !current.Running || current.ExecutionRevision != row.ExecutionRevision {
					cancel()
					return
				}
				if time.Since(renewed) < 20*time.Second {
					continue
				}
				renewCtx, done := context.WithTimeout(ctx, 3*time.Second)
				v, e := common.RDB.Eval(renewCtx, `if redis.call('get',KEYS[1])==ARGV[1] then return redis.call('expire',KEYS[1],90) else return 0 end`, []string{"capability:leader"}, owner).Int()
				done()
				if e != nil || v != 1 {
					cancel()
					return
				}
				r := model.DB.Model(&model.CapabilityWorker{}).Where("id = ? AND owner = ?", 1, owner).Update("heartbeat", time.Now().Unix())
				if r.Error != nil || r.RowsAffected != 1 {
					cancel()
					return
				}
				renewed = time.Now()
			}
		}
	}()
	if slot > 0 {
		capabilityExecuteRound(ctx, row, config, slot, owner)
	}
	// Current-round generation has priority. Recovery is bounded by the next
	// scheduled slot, never delaying it to catch up an old answer.
	if ctx.Err() == nil {
		capabilityRecoverOne(ctx, row, config, owner)
	}
	cancel()
	heartbeat.Wait()
	_ = model.DB.Model(&model.CapabilityWorker{}).Where("id = ? AND owner = ?", 1, owner).Updates(map[string]any{"state": "idle", "heartbeat": time.Now().Unix()}).Error
}

func capabilityExecuteRound(parent context.Context, row model.CapabilityControl, config model.CapabilityConfig, slot int64, owner string) {
	deadline := time.Unix(slot+int64(config.IntervalMinutes)*50, 0)
	ctx, cancel := context.WithDeadline(parent, deadline)
	defer cancel()
	round := model.CapabilityRound{ID: uuid.NewString(), Slot: slot, Revision: row.Revision, ExecutionRevision: row.ExecutionRevision, Suite: capabilitytest.SuiteVersion, Seed: uuid.NewString(), Config: row.Settings, Status: "running", Owner: owner, Deadline: deadline.Unix(), CreatedAt: time.Now().Unix()}
	questions := capabilitytest.Generate(round.Seed)
	raw, _ := common.Marshal(questions)
	round.Questions = string(raw)
	r := model.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&round)
	if r.Error != nil || r.RowsAffected != 1 {
		return
	}
	targets, err := service.DiscoverCapabilityTargets(config)
	if err != nil {
		if e := capabilityPublishRound(round, "interrupted"); e != nil {
			common.SysError("capability discovery/finalization failed; previous snapshot retained")
		}
		return
	}
	recordSkipped := func(target service.CapabilityTarget, reason string) {
		if err := capabilityRecordSkipped(round, target, reason); err != nil {
			common.SysError("capability skipped-target evidence could not be saved")
			cancel()
		}
	}
	profiles := map[string]model.CapabilityProfile{}
	for _, p := range config.Models {
		profiles[p.Model] = p
	}
	keys := map[int]*capabilityWork{}
	jobs := map[string]*capabilityWork{}
	ordered := []*capabilityWork{}
	for _, target := range targets {
		if ctx.Err() != nil {
			break
		}
		if !target.Eligible {
			recordSkipped(target, target.Reason)
			continue
		}
		keyInfo, ok := keys[target.ChannelID]
		if !ok {
			ch, e := model.GetChannelById(target.ChannelID, true)
			if e != nil {
				recordSkipped(target, "channel_missing")
				continue
			}
			key, index, keyErr := ch.GetNextEnabledKey()
			if keyErr != nil {
				recordSkipped(target, "credential_unavailable")
				continue
			}
			keyInfo = &capabilityWork{channel: ch, key: key, slot: index}
			keys[target.ChannelID] = keyInfo
		}
		profile := profiles[target.Model]
		signatures := []string{}
		reason := ""
		for _, q := range questions {
			prepared, e := prepareCapability(ctx, target, profile, q.Prompt, "", keyInfo.channel, keyInfo.key, keyInfo.slot, capabilitySecret())
			if e != nil {
				reason = e.Error()
				break
			}
			signatures = append(signatures, prepared.fingerprint)
		}
		if reason != "" {
			recordSkipped(target, reason)
			continue
		}
		fingerprint := fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(signatures, "|"))))
		if job, ok := jobs[fingerprint]; ok {
			job.targets = append(job.targets, target)
			continue
		}
		identity := fmt.Sprintf("%x", sha256.Sum256([]byte(round.ID+fingerprint)))
		job := &capabilityWork{run: model.CapabilityRun{ID: uuid.NewString(), RoundID: round.ID, Identity: identity, ChannelID: target.ChannelID, Model: target.Model, Protocol: profile.Protocol, Fingerprint: fingerprint, KeySlot: keyInfo.slot, Status: "running", Owner: owner, StartedAt: time.Now().Unix()}, targets: []service.CapabilityTarget{target}, profile: profile, channel: keyInfo.channel, key: keyInfo.key, slot: keyInfo.slot}
		jobs[fingerprint] = job
		job.run.ConfigurationHash = service.CapabilityConfigurationHash(keyInfo.channel, profile, capabilitySecret())
		ordered = append(ordered, job)
	}
	// Rotate deterministically each round; visual order never controls dispatch.
	sort.SliceStable(ordered, func(i, j int) bool {
		a := sha256.Sum256([]byte(round.Seed + ordered[i].targets[0].GroupUID))
		b := sha256.Sum256([]byte(round.Seed + ordered[j].targets[0].GroupUID))
		return strings.Compare(fmt.Sprintf("%x", a), fmt.Sprintf("%x", b)) < 0
	})
	ordered = capabilityCoverageOrder(ordered)
	ready := []*capabilityWork{}
	for _, job := range ordered {
		if ctx.Err() != nil {
			break
		}
		if err = model.DB.Transaction(func(tx *gorm.DB) error {
			if e := tx.Create(&job.run).Error; e != nil {
				return e
			}
			for _, t := range job.targets {
				if e := tx.Create(&model.CapabilityBinding{PublicID: uuid.NewString(), RunID: job.run.ID, GroupUID: t.GroupUID, RoutingKey: t.Group, CreatedAt: time.Now().Unix()}).Error; e != nil {
					return e
				}
			}
			return nil
		}); err != nil {
			cancel()
			break
		}
		ready = append(ready, job)
	}
	capabilityScheduleJobs(ctx, ready, func(job *capabilityWork) {
		if err := capabilityExecuteWork(ctx, row, config, round, questions, job); err != nil {
			common.SysError("capability result checkpoint failed; round publication cancelled")
			cancel()
		}
	})
	status := "complete"
	if ctx.Err() != nil {
		status = "interrupted"
		// Scheduled jobs that never started must not remain running forever.
		if err := model.DB.Model(&model.CapabilityRun{}).Where("round_id = ? AND owner = ? AND status = ?", round.ID, owner, "running").Updates(map[string]any{"status": "cancelled", "reason": "round_interrupted", "completed_at": time.Now().Unix()}).Error; err != nil {
			common.SysError("capability interrupted jobs could not be finalized")
			return
		}
	}
	if err := capabilityPublishRound(round, status); err != nil {
		common.SysError("capability round publication failed; previous snapshot retained")
	}
}

// Launch in planned order, at most four active targets and one per channel.
// Waiting for a busy channel must not occupy a global execution slot.
func capabilityScheduleJobs(ctx context.Context, jobs []*capabilityWork, execute func(*capabilityWork)) {
	pending := append([]*capabilityWork(nil), jobs...)
	active := map[int]bool{}
	done := make(chan int, 4)
	for len(pending) > 0 || len(active) > 0 {
		if ctx.Err() != nil {
			pending = nil
		}
		for len(active) < 4 && len(pending) > 0 {
			index := -1
			for i, job := range pending {
				if !active[job.channel.Id] {
					index = i
					break
				}
			}
			if index < 0 {
				break
			}
			job := pending[index]
			pending = append(pending[:index], pending[index+1:]...)
			active[job.channel.Id] = true
			go func() { defer func() { done <- job.channel.Id }(); execute(job) }()
		}
		if len(active) > 0 {
			delete(active, <-done)
		}
	}
}

// Cover underserved groups before their second target. Presentation order
// cannot buy execution priority, and shared work advances all subscribers.
func capabilityCoverageOrder(jobs []*capabilityWork) []*capabilityWork {
	remaining := append([]*capabilityWork(nil), jobs...)
	out := make([]*capabilityWork, 0, len(jobs))
	coverage := map[string]int{}
	for len(remaining) > 0 {
		best, bestCoverage := 0, int(^uint(0)>>1)
		for i, job := range remaining {
			level := int(^uint(0) >> 1)
			for _, target := range job.targets {
				level = min(level, coverage[target.GroupUID])
			}
			if level < bestCoverage {
				best, bestCoverage = i, level
			}
		}
		job := remaining[best]
		out = append(out, job)
		for _, target := range job.targets {
			coverage[target.GroupUID]++
		}
		remaining = append(remaining[:best], remaining[best+1:]...)
	}
	return out
}
func capabilityRecordSkipped(round model.CapabilityRound, t service.CapabilityTarget, reason string) error {
	identity := fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%s/%s/%d/%s", round.ID, t.GroupUID, t.ChannelID, t.Model))))
	run := model.CapabilityRun{ID: uuid.NewString(), RoundID: round.ID, Identity: identity, ChannelID: t.ChannelID, Model: t.Model, Status: "skipped", Reason: reason, Owner: round.Owner, CompletedAt: time.Now().Unix()}
	return model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&run).Error; err != nil {
			return err
		}
		if t.GroupUID == "" {
			return nil
		}
		return tx.Create(&model.CapabilityBinding{PublicID: uuid.NewString(), RunID: run.ID, GroupUID: t.GroupUID, RoutingKey: t.Group, CreatedAt: time.Now().Unix()}).Error
	})
}

func capabilityExecuteWork(ctx context.Context, row model.CapabilityControl, config model.CapabilityConfig, round model.CapabilityRound, questions []capabilitytest.Question, job *capabilityWork) error {
	items := []capabilityItem{}
	var persistErr error
	appendItem := func(item capabilityItem) {
		items = append(items, item)
		persistErr = capabilitySaveWork(job, items, "running")
	}
	for _, q := range questions {
		if persistErr != nil {
			return persistErr
		}
		item := capabilityItem{Kind: q.Kind, Question: q, Status: "pending"}
		response := capabilityCall(ctx, row, config, round, job, q.Kind, q.Prompt, "")
		item.Answer = response.Answer
		if response.Reason != "" {
			item.Status = "unavailable"
			item.Reason = response.Reason
			appendItem(item)
			continue
		}
		if q.Kind == "logic" {
			checks, err := capabilitytest.ParseLogic(response.Answer, q.Expected)
			if err != nil {
				item.Status = "ungraded"
				item.Reason = err.Error()
			} else {
				item.Status = "graded"
				item.Checks = checks
			}
			appendItem(item)
			continue
		}
		// Persist paid output before entering a separate rendering process.
		item.Status = "pending_render"
		if err := capabilitySaveWork(job, append(items, item), "running"); err != nil {
			return err
		}
		png, err := capabilityRenderItem(ctx, &item)
		if err != nil {
			item.Reason = err.Error()
			appendItem(item)
			continue
		}
		if q.Kind == "geometry" {
			item.Status = "graded"
		} else {
			item.Status = "pending_review"
			if err := capabilitySaveWork(job, append(items, item), "running"); err != nil {
				return err
			}
			judgeCh, e := model.GetChannelById(config.JudgeChannelID, true)
			if e == nil && judgeCh.Status == common.ChannelStatusEnabled && judgeCh.Id != job.channel.Id {
				key, slot, keyErr := judgeCh.GetNextEnabledKey()
				if keyErr == nil {
					judgeJob := *job
					judgeJob.channel = judgeCh
					judgeJob.key = key
					judgeJob.slot = slot
					judgeJob.profile = model.CapabilityProfile{Model: config.JudgeModel, Protocol: "chat", MaxTokens: 1024, Enabled: true}
					judged := capabilityCall(ctx, row, config, round, &judgeJob, "judge", capabilitytest.JudgePrompt(q), "data:image/png;base64,"+base64.StdEncoding.EncodeToString(png))
					if judged.Reason == "" {
						j, err := capabilitytest.ParseJudgment(judged.Answer)
						if err == nil {
							item.Judgment = &j
							capabilityApplyMotionEvidence(&item)
							item.Status = "graded"
							for _, v := range j.Items {
								if v.Status == "uncertain" {
									item.Status = "pending_review"
								}
							}
						} else {
							item.Reason = err.Error()
						}
					} else {
						item.Reason = judged.Reason
					}
				}
			} else {
				item.Reason = "independent_judge_required"
			}
		}
		appendItem(item)
	}
	if persistErr != nil {
		return persistErr
	}
	status := "complete"
	if ctx.Err() != nil {
		status = "cancelled"
	}
	return capabilitySaveWork(job, items, status)
}

// Checkpoints are private until publication. A restart retains generated
// answers and rendered works, without replaying a paid upstream request.
func capabilitySaveWork(job *capabilityWork, items []capabilityItem, status string) error {
	data, err := common.Marshal(items)
	if err != nil {
		return err
	}
	return model.DB.Transaction(func(tx *gorm.DB) error {
		var worker model.CapabilityWorker
		if err := tx.Model(&model.CapabilityWorker{}).Where("id = ?", 1).UpdateColumn("heartbeat", gorm.Expr("heartbeat")).Error; err != nil {
			return err
		}
		if err := tx.First(&worker, 1).Error; err != nil {
			return err
		}
		if worker.Owner != job.run.Owner || worker.Heartbeat < time.Now().Unix()-45 {
			return errors.New("lease_expired")
		}
		values := map[string]any{"status": status, "result": string(data)}
		if status != "running" {
			values["completed_at"] = time.Now().Unix()
		}
		r := tx.Model(&model.CapabilityRun{}).Where("id = ? AND owner = ? AND status = ?", job.run.ID, job.run.Owner, "running").Updates(values)
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected != 1 {
			return errors.New("run_no_longer_owned")
		}
		for _, item := range items {
			if item.Status == "graded" || item.Status == "ungraded" {
				if err := tx.Model(&model.CapabilityRecoveryJob{}).Where("run_id = ? AND kind = ? AND status = ?", job.run.ID, item.Kind, "pending").Updates(map[string]any{"status": "complete", "reason": "already_evaluated", "updated_at": time.Now().Unix()}).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func capabilityCall(ctx context.Context, row model.CapabilityControl, config model.CapabilityConfig, round model.CapabilityRound, job *capabilityWork, kind, prompt, imageData string) capabilityResponse {
	fail := func(reason string) capabilityResponse { return capabilityResponse{Reason: reason} }
	if ctx.Err() != nil {
		return fail("cancelled")
	}
	if job.recovery == nil && time.Now().Unix() >= round.Slot+int64(config.IntervalMinutes)*40 {
		return fail("dispatch_window_closed")
	}
	live, err := model.GetCapabilityControl()
	if err != nil || !live.Running || live.ExecutionRevision != row.ExecutionRevision {
		return fail("control_changed")
	}
	liveConfig, err := live.Config()
	if err != nil {
		return fail("control_unavailable")
	}
	targets, err := service.DiscoverCapabilityTargets(liveConfig)
	if err != nil {
		return fail("topology_unavailable")
	}
	var target *service.CapabilityTarget
	for _, allowed := range job.targets {
		for _, current := range targets {
			if current.Eligible && current.GroupUID == allowed.GroupUID && current.ChannelID == allowed.ChannelID && current.Model == allowed.Model {
				copy := current
				target = &copy
				break
			}
		}
		if target != nil {
			break
		}
	}
	if target == nil {
		return fail("target_no_longer_eligible")
	}
	if !capabilityProfileCurrent(liveConfig, job, kind) {
		return fail("profile_changed")
	}
	ch, err := model.GetChannelById(job.channel.Id, true)
	if err != nil || ch.Status != common.ChannelStatusEnabled {
		return fail("channel_unavailable")
	}
	key := ch.Key
	if ch.ChannelInfo.IsMultiKey {
		keys := ch.GetKeys()
		if job.slot >= len(keys) {
			return fail("credential_changed")
		}
		key = keys[job.slot]
		if status, ok := ch.ChannelInfo.MultiKeyStatusList[job.slot]; ok && status != common.ChannelStatusEnabled {
			return fail("credential_disabled")
		}
	}
	if key != job.key {
		return fail("credential_changed")
	}
	if kind == "judge" {
		if liveConfig.JudgeChannelID != config.JudgeChannelID || liveConfig.JudgeModel != config.JudgeModel {
			return fail("judge_configuration_changed")
		}
		target.ChannelID = ch.Id
		target.Model = config.JudgeModel
	}
	callCtx, cancel := context.WithTimeout(ctx, 180*time.Second)
	defer cancel()
	prepared, err := prepareCapability(callCtx, *target, job.profile, prompt, imageData, ch, key, job.slot, capabilitySecret())
	if err != nil {
		return fail(err.Error())
	}
	if kind != "judge" {
		old, err := prepareCapability(callCtx, *target, job.profile, prompt, imageData, job.channel, job.key, job.slot, capabilitySecret())
		if err != nil || old.fingerprint != prepared.fingerprint {
			return fail("execution_configuration_changed")
		}
	}
	lease, acquired, _ := service.AcquireChannelCapacity(ch.Id, ch.ConcurrencyLimit, ch.RPMLimit)
	if !acquired {
		return fail("business_capacity_reserved")
	}
	defer lease.Release()
	attempt := model.CapabilityAttempt{Kind: kind, ChannelID: ch.Id, Prompt: prompt, RequestHash: prepared.requestHash, Endpoint: prepared.endpointLabel, KeySlot: job.slot}
	if kind == "judge" {
		png, e := base64.StdEncoding.DecodeString(strings.TrimPrefix(imageData, "data:image/png;base64,"))
		if e != nil {
			return fail("invalid_judge_image")
		}
		attempt.ImageSHA256 = fmt.Sprintf("%x", sha256.Sum256(png))
	}
	guards := []func(*gorm.DB, model.CapabilityControl) error{capabilityDispatchGuard(live.Revision, job, ch, *target)}
	if job.recovery != nil {
		guards = append(guards, capabilityRecoverySourceGuard(job.run, round))
		err = model.DispatchCapabilityRecoveryAttempt(job.run, &attempt, *job.recovery, config.DailyBudgetMicros, config.CallReserveMicros, guards...)
	} else {
		err = model.DispatchCapabilityAttempt(job.run, &attempt, row.ExecutionRevision, config.DailyBudgetMicros, config.CallReserveMicros, guards...)
	}
	if err != nil {
		return fail("dispatch_permission_denied")
	}
	stopWatching := capabilityWatchCall(callCtx, cancel, row, job, ch, kind)
	response := prepared.execute()
	stopWatching()
	response.Answer = strings.ReplaceAll(response.Answer, key, "[REDACTED]")
	response.Usage = strings.ReplaceAll(response.Usage, key, "[REDACTED]")
	response.RequestID = strings.ReplaceAll(response.RequestID, key, "[REDACTED]")
	response.ReportedModel = strings.ReplaceAll(response.ReportedModel, key, "[REDACTED]")
	usageSource := "unavailable"
	if response.Usage != "" {
		usageSource = "provider_reported"
	}
	status := "complete"
	if response.Reason != "" {
		status = "outcome_unknown"
	}
	if err = model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&attempt).Updates(map[string]any{"status": status, "answer": response.Answer, "usage": response.Usage, "usage_source": usageSource, "request_id": response.RequestID, "reported_model": response.ReportedModel, "upstream_model": response.UpstreamModel, "reason": response.Reason, "completed_at": time.Now().Unix()}).Error; err != nil {
			return err
		}
		if status == "complete" && kind != "judge" {
			return model.EnqueueCapabilityRecovery(tx, job.run.ID, kind, row.ExecutionRevision)
		}
		return nil
	}); err != nil {
		return fail("evidence_persistence_failed")
	}
	return response
}
