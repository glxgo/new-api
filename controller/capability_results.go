package controller

import (
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type capabilityPublicRun struct {
	PublicID string                             `json:"public_id"`
	Model    string                             `json:"model"`
	Sample   string                             `json:"sample"`
	Time     int64                              `json:"time"`
	Slot     int64                              `json:"slot"`
	Status   string                             `json:"status"`
	Items    []capabilityItem                   `json:"items"`
	Score    []*int                             `json:"score"`
	Ranked   bool                               `json:"ranked"`
	Suite    string                             `json:"suite"`
	Metrics  map[string]capabilityPublicMetrics `json:"metrics,omitempty"`
}

func GetCapabilityRuntime(c *gin.Context) {
	row, err := model.GetCapabilityControl()
	if err != nil {
		capabilityError(c, errors.New("无法读取运行状态"))
		return
	}
	config, err := row.Config()
	if err != nil {
		capabilityError(c, err)
		return
	}
	var worker model.CapabilityWorker
	model.DB.First(&worker, 1)
	var budget model.CapabilityBudgetDay
	model.DB.First(&budget, "day = ?", time.Now().UTC().Format("2006-01-02"))
	var active int64
	model.DB.Model(&model.CapabilityAttempt{}).Where("status = ?", "dispatching").Count(&active)
	var recovery []struct {
		Status string `json:"status"`
		Count  int64  `json:"count"`
	}
	if model.DB.Model(&model.CapabilityRecoveryJob{}).Select("status, COUNT(*) AS count").Group("status").Scan(&recovery).Error != nil {
		c.Status(503)
		return
	}
	c.JSON(200, gin.H{"success": true, "data": gin.H{"worker": worker, "budget": budget, "active_calls": active, "recovery": recovery, "readiness": capabilityReadiness(config), "revision": row.Revision, "execution_revision": row.ExecutionRevision, "running": row.Running}})
}
func GetCapabilityAdminRuns(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	if page > 10000 {
		page = 10000
	}
	query := model.DB.Model(&model.CapabilityRun{})
	if channelID, _ := strconv.Atoi(c.Query("channel_id")); channelID > 0 {
		query = query.Where("channel_id = ?", channelID)
	}
	if name := c.Query("model"); name != "" {
		query = query.Where("model = ?", name)
	}
	if state := c.Query("status"); state != "" {
		query = query.Where("status = ?", state)
	}
	var runs []model.CapabilityRun
	if err := query.Omit("result").Order("started_at desc, id desc").Limit(51).Offset((page - 1) * 50).Find(&runs).Error; err != nil {
		capabilityError(c, errors.New("读取运行记录失败"))
		return
	}
	more := len(runs) > 50
	if more {
		runs = runs[:50]
	}
	c.JSON(200, gin.H{"success": true, "data": runs, "has_more": more})
}
func GetCapabilityAdminRun(c *gin.Context) {
	var run model.CapabilityRun
	if model.DB.First(&run, "id = ?", c.Param("id")).Error != nil {
		c.Status(404)
		return
	}
	var attempts []model.CapabilityAttempt
	var bindings []model.CapabilityBinding
	if model.DB.Where("run_id = ?", run.ID).Order("started_at asc").Find(&attempts).Error != nil || model.DB.Where("run_id = ?", run.ID).Find(&bindings).Error != nil {
		capabilityError(c, errors.New("读取调用证据失败"))
		return
	}
	items := []capabilityItem{}
	if run.Result != "" && common.UnmarshalJsonStr(run.Result, &items) != nil {
		c.Status(503)
		return
	}
	for i := range items {
		if items[i].Artifact != "" {
			capabilityArtifactURLs(&items[i], "/api/capability/admin/runs/"+run.ID+"/artifacts/"+items[i].Kind)
		}
	}
	var recovery []model.CapabilityRecoveryJob
	var revisions []model.CapabilityEvaluation
	if model.DB.Where("run_id = ?", run.ID).Find(&recovery).Error != nil || model.DB.Where("run_id = ?", run.ID).Order("created_at desc, revision desc").Limit(100).Find(&revisions).Error != nil {
		c.Status(503)
		return
	}
	evaluations := []gin.H{}
	for _, revision := range revisions {
		var item capabilityItem
		if common.UnmarshalJsonStr(revision.Result, &item) != nil {
			c.Status(503)
			return
		}
		if item.Artifact != "" {
			capabilityArtifactURLs(&item, "/api/capability/admin/runs/"+run.ID+"/evaluations/"+revision.ID+"/artifact")
		}
		evaluations = append(evaluations, gin.H{"id": revision.ID, "revision": revision.Revision, "created_at": revision.CreatedAt, "item": item})
	}
	c.JSON(200, gin.H{"success": true, "data": gin.H{"run": run, "items": items, "attempts": attempts, "bindings": bindings, "recovery": recovery, "evaluations": evaluations}})
}

func GetCapabilityEvaluationArtifact(c *gin.Context) {
	var revision model.CapabilityEvaluation
	if model.DB.First(&revision, "id = ? AND run_id = ?", c.Param("evaluation_id"), c.Param("id")).Error != nil {
		c.Status(404)
		return
	}
	var item capabilityItem
	if common.UnmarshalJsonStr(revision.Result, &item) != nil || item.Artifact == "" {
		c.Status(404)
		return
	}
	capabilityServeArtifact(c, item)
}

func GetCapabilityAdminArtifact(c *gin.Context) {
	var run model.CapabilityRun
	if model.DB.First(&run, "id = ?", c.Param("id")).Error != nil {
		c.Status(404)
		return
	}
	var items []capabilityItem
	if common.UnmarshalJsonStr(run.Result, &items) != nil {
		c.Status(404)
		return
	}
	for _, item := range items {
		if item.Kind == c.Param("kind") && item.Artifact != "" {
			capabilityServeArtifact(c, item)
			return
		}
	}
	c.Status(404)
}
func capabilityAuthorizeGroup(c *gin.Context, uid string) (model.CapabilityControl, model.CapabilityGroupPresentation, bool) {
	row, err := model.GetCapabilityControl()
	var group model.CapabilityGroupPresentation
	if err != nil || !row.Visible {
		c.Status(404)
		return row, group, false
	}
	view, err := row.View(false)
	if err != nil || view.Groups[uid].Hidden {
		c.Status(404)
		return row, group, false
	}
	if model.DB.First(&group, "group_uid = ? AND routing_key IS NOT NULL", uid).Error != nil {
		c.Status(404)
		return row, group, false
	}
	allowed, err := service.CapabilityAllowedGroups(c.GetInt("id"))
	if err != nil {
		c.Status(503)
		return row, group, false
	}
	if _, ok := allowed[*group.RoutingKey]; !ok {
		c.Status(404)
		return row, group, false
	}
	return row, group, true
}
func capabilityRunDTO(binding model.CapabilityBinding, run model.CapabilityRun, round model.CapabilityRound, detail bool) capabilityPublicRun {
	items := []capabilityItem{}
	_ = common.UnmarshalJsonStr(run.Result, &items)
	mac := hmac.New(sha256.New, capabilitySecret())
	_, _ = mac.Write([]byte(fmt.Sprintf("%s:%d", binding.GroupUID, run.ChannelID)))
	sample := fmt.Sprintf("%x", mac.Sum(nil))[:6]
	// Missing or uncertain grades remain null. Zero is a real measured score.
	score := make([]*int, 4)
	for i := range items {
		item := &items[i]
		if item.Status == "graded" {
			index, required := 0, 2
			if item.Kind == "geometry" {
				index, required = 1, 5
			}
			if (item.Kind == "logic" || item.Kind == "geometry") && len(item.Checks) == required {
				value := 0
				for _, pass := range item.Checks {
					if pass {
						value++
					}
				}
				score[index] = &value
			}
			if item.Kind == "scene" && item.Judgment != nil && len(item.Judgment.Items) == 6 {
				value, known := 0, true
				for _, j := range item.Judgment.Items {
					if j.Status == "pass" {
						value++
					} else if j.Status != "fail" {
						known = false
					}
				}
				if known && item.Judgment.Aesthetic >= 1 && item.Judgment.Aesthetic <= 5 {
					score[2], score[3] = &value, &item.Judgment.Aesthetic
				}
			}
		}
		if !detail {
			item.Answer = ""
			item.Question.Prompt = ""
		}
		if item.Artifact != "" {
			capabilityArtifactURLs(item, "/api/capability/runs/"+binding.PublicID+"/artifacts/"+item.Kind)
		}
		item.Reason = ""
	}
	ranked := true
	for _, value := range score {
		ranked = ranked && value != nil
	}
	return capabilityPublicRun{PublicID: binding.PublicID, Model: run.Model, Sample: sample, Time: run.CompletedAt, Slot: round.Slot, Status: run.Status, Items: items, Score: score, Ranked: ranked, Suite: round.Suite}
}
func GetCapabilityResults(c *gin.Context) {
	uid := c.Param("group_uid")
	row, _, ok := capabilityAuthorizeGroup(c, uid)
	if !ok {
		return
	}
	config, err := row.Config()
	if err != nil {
		c.Status(503)
		return
	}
	current, err := service.DiscoverCapabilityTargets(config)
	if err != nil {
		c.Status(503)
		return
	}
	eligible := map[string]bool{}
	for _, t := range current {
		if t.GroupUID == uid && t.Eligible {
			eligible[fmt.Sprintf("%d:%s", t.ChannelID, t.Model)] = true
		}
	}
	history := c.Query("history") == "true"
	view, viewErr := row.View(false)
	if viewErr != nil {
		c.Status(503)
		return
	}
	if history && !view.ShowHistory {
		c.Status(404)
		return
	}
	profiles := map[string]model.CapabilityProfile{}
	for _, p := range config.Models {
		profiles[p.Model] = p
	}
	channels := map[int]*model.Channel{}
	days := 1
	if c.Query("range") == "3d" {
		days = 3
	}
	if c.Query("range") == "7d" {
		days = 7
	}
	var bindings []model.CapabilityBinding
	latest := map[string]int64{}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	if page > 10000 {
		page = 10000
	}
	query := model.DB.Where("group_uid = ?", uid)
	if history {
		query = query.Where("created_at >= ?", time.Now().Add(-time.Duration(days)*24*time.Hour).Unix()).Order("created_at desc, public_id desc").Limit(101).Offset((page - 1) * 100)
	} else {
		var snapshots []model.CapabilitySnapshot
		if err = model.DB.Where("group_uid = ?", uid).Find(&snapshots).Error; err != nil {
			c.Status(503)
			return
		}
		ids := []string{}
		for _, snapshot := range snapshots {
			if name := c.Query("model"); name != "" && snapshot.Model != name {
				continue
			}
			var members []string
			if common.UnmarshalJsonStr(snapshot.Bindings, &members) != nil {
				c.Status(503)
				return
			}
			ids = append(ids, members...)
			latest[snapshot.Model] = snapshot.Slot
		}
		if len(ids) == 0 {
			c.JSON(200, gin.H{"success": true, "data": []capabilityPublicRun{}, "latest": latest, "revision": row.Revision, "has_more": false})
			return
		}
		bindings, err = capabilityFindByIDs[model.CapabilityBinding](query, "public_id", ids)
	}
	if history {
		err = query.Find(&bindings).Error
	}
	if err != nil {
		c.Status(503)
		return
	}
	result := []capabilityPublicRun{}
	more := history && len(bindings) > 100
	if more {
		bindings = bindings[:100]
	}
	ids := []string{}
	for _, b := range bindings {
		ids = append(ids, b.RunID)
	}
	var runs []model.CapabilityRun
	if len(ids) > 0 {
		runs, err = capabilityFindByIDs[model.CapabilityRun](model.DB, "id", ids)
		if err != nil {
			c.Status(503)
			return
		}
	}
	runMap := map[string]model.CapabilityRun{}
	roundIDs := []string{}
	for _, r := range runs {
		runMap[r.ID] = r
		roundIDs = append(roundIDs, r.RoundID)
	}
	var rounds []model.CapabilityRound
	if len(roundIDs) > 0 {
		rounds, err = capabilityFindByIDs[model.CapabilityRound](model.DB, "id", roundIDs)
		if err != nil {
			c.Status(503)
			return
		}
	}
	roundMap := map[string]model.CapabilityRound{}
	for _, r := range rounds {
		roundMap[r.ID] = r
	}
	for _, binding := range bindings {
		run, ok := runMap[binding.RunID]
		if !ok {
			continue
		}
		round, ok := roundMap[run.RoundID]
		if !ok {
			continue
		}
		if name := c.Query("model"); name != "" && run.Model != name {
			continue
		}
		if round.Status == "running" {
			continue
		}
		if !history && (!eligible[fmt.Sprintf("%d:%s", run.ChannelID, run.Model)] || (run.Status != "complete" && run.Status != "cancelled")) {
			continue
		}
		if binding.Withdrawn || binding.DispatchedAt == 0 {
			continue
		}
		if !history {
			ch, exists := channels[run.ChannelID]
			if !exists {
				ch, err = model.GetChannelById(run.ChannelID, true)
				if err != nil {
					continue
				}
				channels[run.ChannelID] = ch
			}
			if run.ConfigurationHash != service.CapabilityConfigurationHash(ch, profiles[run.Model], capabilitySecret()) {
				continue
			}
		}
		dto := capabilityRunDTO(binding, run, round, false)
		result = append(result, dto)
	}
	sort.SliceStable(result, func(i, j int) bool {
		a, b := result[i], result[j]
		if a.Slot != b.Slot {
			return a.Slot > b.Slot
		}
		if a.Ranked != b.Ranked {
			return a.Ranked
		}
		if a.Ranked {
			for k := range a.Score {
				if *a.Score[k] != *b.Score[k] {
					return *a.Score[k] > *b.Score[k]
				}
			}
		}
		return a.PublicID < b.PublicID
	})
	c.JSON(200, gin.H{"success": true, "data": result, "latest": latest, "revision": row.Revision, "has_more": more})
}
func GetCapabilityPublicRun(c *gin.Context) {
	var binding model.CapabilityBinding
	if model.DB.First(&binding, "public_id = ? AND withdrawn = ? AND dispatched_at > ?", c.Param("id"), false, 0).Error != nil {
		c.Status(404)
		return
	}
	control, _, ok := capabilityAuthorizeGroup(c, binding.GroupUID)
	if !ok {
		return
	}
	var run model.CapabilityRun
	var round model.CapabilityRound
	if model.DB.First(&run, "id = ? AND status IN ?", binding.RunID, []string{"complete", "cancelled"}).Error != nil || model.DB.First(&round, "id = ?", run.RoundID).Error != nil {
		c.Status(404)
		return
	}
	if round.Status == "running" {
		c.Status(404)
		return
	}
	view, err := control.View(false)
	if err != nil {
		c.Status(503)
		return
	}
	if !view.ShowHistory {
		// Hiding history is enforced on direct detail/image URLs too.
		var snapshot model.CapabilitySnapshot
		if model.DB.First(&snapshot, "group_uid = ? AND model = ?", binding.GroupUID, run.Model).Error != nil {
			c.Status(404)
			return
		}
		var ids []string
		if common.UnmarshalJsonStr(snapshot.Bindings, &ids) != nil {
			c.Status(503)
			return
		}
		found := false
		for _, id := range ids {
			found = found || id == binding.PublicID
		}
		if !found {
			c.Status(404)
			return
		}
	}
	if kind := c.Param("kind"); kind != "" {
		var items []capabilityItem
		if common.UnmarshalJsonStr(run.Result, &items) != nil {
			c.Status(404)
			return
		}
		for _, item := range items {
			if item.Kind == kind && item.Artifact != "" {
				capabilityServeArtifact(c, item)
				return
			}
		}
		c.Status(404)
		return
	}
	result := capabilityRunDTO(binding, run, round, true)
	var attempts []model.CapabilityAttempt
	// Explicit allowlist: never expose channel IDs, provider URLs, key slots,
	// request IDs, raw usage objects or administrator diagnostics publicly.
	if model.DB.WithContext(c.Request.Context()).Select("kind", "status", "started_at", "completed_at", "usage", "usage_source").Where("run_id = ? AND kind IN ?", run.ID, []string{"logic", "geometry", "scene"}).Find(&attempts).Error != nil {
		c.Status(503)
		return
	}
	result.Metrics = capabilityPublicCallMetrics(attempts)
	c.JSON(200, gin.H{"success": true, "data": result})
}

func capabilityArtifactURLs(item *capabilityItem, base string) {
	item.Artifact = base
	if item.Animation != nil {
		copy := *item.Animation
		copy.Artifact = base + "?view=animation"
		copy.Evidence = base + "?view=evidence"
		item.Animation = &copy
	}
}

// All variants pass the same run/group/history authorization as the poster.
func capabilityServeArtifact(c *gin.Context, item capabilityItem) {
	var raw []byte
	var err error
	switch c.Query("view") {
	case "", "poster":
		raw, err = service.ReadCapabilityPNG(item.Artifact)
	case "animation", "evidence":
		if item.Animation == nil {
			c.Status(404)
			return
		}
		id := item.Animation.Artifact
		if c.Query("view") == "evidence" {
			id = item.Animation.Evidence
		}
		raw, err = service.ReadCapabilityAnimationAsset(id, c.Query("view"))
	default:
		c.Status(404)
		return
	}
	if err != nil {
		c.Status(404)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Security-Policy", "default-src 'none'; sandbox")
	c.Data(200, "image/png", raw)
}

// Keep below SQLite's conservative bind-variable limit even for large
// installations; duplicate shared IDs are read only once.
func capabilityFindByIDs[T any](db *gorm.DB, column string, ids []string) ([]T, error) {
	unique := make([]string, 0, len(ids))
	seen := map[string]bool{}
	for _, id := range ids {
		if !seen[id] {
			unique = append(unique, id)
			seen[id] = true
		}
	}
	rows := []T{}
	for start := 0; start < len(unique); start += 200 {
		var batch []T
		if err := db.Where(column+" IN ?", unique[start:min(start+200, len(unique))]).Find(&batch).Error; err != nil {
			return nil, err
		}
		rows = append(rows, batch...)
	}
	return rows, nil
}
