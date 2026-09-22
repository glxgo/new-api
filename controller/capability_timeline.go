package controller

import (
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type capabilityTimelineItem struct {
	Kind      string `json:"kind"`
	Score     *int   `json:"score"`
	Maximum   int    `json:"maximum"`
	Evaluated int    `json:"evaluated"`
	PublicID  string `json:"public_id"`
}
type capabilityTimelineRound struct {
	Slot        int64                    `json:"slot"`
	CompletedAt int64                    `json:"completed_at"`
	Suite       string                   `json:"suite"`
	Samples     int                      `json:"samples"`
	Items       []capabilityTimelineItem `json:"items"`
}

// Each cell represents a completed historical round, never an invented time
// bucket. Select a real best-scoring sample for that task, with its detail ID.
func GetCapabilityTimeline(c *gin.Context) {
	uid := c.Param("group_uid")
	control, _, ok := capabilityAuthorizeGroup(c, uid)
	if !ok {
		return
	}
	view, err := control.View(false)
	if err != nil {
		c.Status(503)
		return
	}
	if !view.ShowHistory {
		c.Status(404)
		return
	}
	name := c.Query("model")
	if name == "" || len(name) > 191 {
		c.Status(400)
		return
	}
	bindingIDs := model.DB.Model(&model.CapabilityBinding{}).Select("run_id").Where("group_uid = ? AND withdrawn = ? AND dispatched_at > ?", uid, false, 0)
	roundIDs := model.DB.Model(&model.CapabilityRun{}).Select("round_id").Where("model = ? AND id IN (?)", name, bindingIDs)
	var rounds []model.CapabilityRound
	if err := model.DB.Select("id", "slot", "completed_at", "suite").Where("id IN (?) AND status <> ?", roundIDs, "running").Order("slot desc").Limit(48).Find(&rounds).Error; err != nil {
		c.Status(503)
		return
	}
	out := make([]capabilityTimelineRound, len(rounds))
	positions := map[string]int{}
	ids := []string{}
	for i, round := range rounds {
		positions[round.ID] = i
		ids = append(ids, round.ID)
		out[i] = capabilityTimelineRound{Slot: round.Slot, CompletedAt: round.CompletedAt, Suite: round.Suite, Items: []capabilityTimelineItem{{Kind: "logic", Maximum: 2}, {Kind: "geometry", Maximum: 5}, {Kind: "scene", Maximum: 6}}}
	}
	if len(ids) > 0 {
		var batch []model.CapabilityRun
		err = model.DB.Where("round_id IN ? AND model = ? AND id IN (?) AND status IN ?", ids, name, bindingIDs, []string{"complete", "cancelled"}).FindInBatches(&batch, 100, func(tx *gorm.DB, _ int) error {
			runIDs := make([]string, 0, len(batch))
			for _, run := range batch {
				runIDs = append(runIDs, run.ID)
			}
			var bindings []model.CapabilityBinding
			if err := model.DB.Where("group_uid = ? AND run_id IN ? AND withdrawn = ? AND dispatched_at > ?", uid, runIDs, false, 0).Find(&bindings).Error; err != nil {
				return err
			}
			byRun := map[string]model.CapabilityBinding{}
			for _, binding := range bindings {
				byRun[binding.RunID] = binding
			}
			for _, run := range batch {
				binding, exists := byRun[run.ID]
				if !exists {
					continue
				}
				index := positions[run.RoundID]
				point := &out[index]
				point.Samples++
				dto := capabilityRunDTO(binding, run, rounds[index], false)
				for k := range point.Items {
					item := &point.Items[k]
					if dto.Score[k] != nil {
						item.Evaluated++
						if item.Score == nil || *dto.Score[k] > *item.Score || (*dto.Score[k] == *item.Score && dto.PublicID < item.PublicID) {
							score := *dto.Score[k]
							item.Score = &score
							item.PublicID = dto.PublicID
						}
					} else if item.PublicID == "" {
						item.PublicID = dto.PublicID
					}
				}
			}
			return nil
		}).Error
		if err != nil {
			c.Status(503)
			return
		}
	}
	// Chronological order, with the newest cell on the right.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	c.JSON(200, gin.H{"success": true, "data": out, "model": name, "selection": "best_per_task", "revision": control.Revision})
}
