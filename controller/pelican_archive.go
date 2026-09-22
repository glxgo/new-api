package controller

import (
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/pelicanarchive"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func pelicanControlDTO(row model.PelicanControl) (gin.H, error) {
	view, err := row.View()
	if err != nil {
		return nil, err
	}
	var cfg *pelicanarchive.Config
	if row.SourceConfig != "" {
		var c pelicanarchive.Config
		if common.UnmarshalJsonStr(row.SourceConfig, &c) != nil {
			return nil, errors.New("来源配置异常")
		}
		cfg = &c
	}
	return gin.H{"control": row, "presentation": view, "source_config": cfg, "source_configured": os.Getenv("PELICAN_ARCHIVE_FILE") != "" && os.Getenv("PELICAN_SOURCE_ID") != "", "writable_node": common.IsMasterNode}, nil
}
func GetPelicanAdmin(c *gin.Context) {
	db := model.DB.WithContext(c.Request.Context())
	row, err := model.GetPelicanControl(db)
	if err != nil {
		capabilityError(c, errors.New("无法读取存档设置"))
		return
	}
	data, err := pelicanControlDTO(row)
	if err != nil {
		capabilityError(c, err)
		return
	}
	var targets []model.PelicanTarget
	var events []model.PelicanEvent
	if db.Order("provider_name, model_name").Find(&targets).Error != nil || db.Order("id DESC").Limit(30).Find(&events).Error != nil {
		capabilityError(c, errors.New("无法读取存档管理数据"))
		return
	}
	allowed := map[string]string{}
	for key := range ratio_setting.GetGroupRatioCopy() {
		if key != "auto" {
			allowed[key] = setting.GetUsableGroupDescription(key)
		}
	}
	groups, err := service.PelicanGroups(allowed, model.DefaultCapabilityPresentation())
	if err != nil {
		capabilityError(c, errors.New("无法读取分组"))
		return
	}
	view, err := row.View()
	if err != nil {
		capabilityError(c, err)
		return
	}
	reports, channels, err := service.PelicanMappingReports(db, targets, groups, view)
	if err != nil {
		capabilityError(c, errors.New("无法核对渠道与分组关联"))
		return
	}
	data["targets"], data["channels"], data["groups"], data["events"], data["mapping_reports"] = targets, channels, groups, events, reports
	c.JSON(200, gin.H{"success": true, "data": data})
}
func UpdatePelicanAdmin(c *gin.Context) {
	if !common.IsMasterNode {
		capabilityError(c, errors.New("候选实例只读"))
		return
	}
	var input struct {
		Revision        *int64                        `json:"revision"`
		Action          string                        `json:"action"`
		IntervalMinutes int                           `json:"interval_minutes"`
		Presentation    *model.CapabilityPresentation `json:"presentation"`
		TargetID        string                        `json:"target_id"`
		ChannelID       int                           `json:"channel_id"`
		DisplayModel    string                        `json:"display_model"`
		Hidden          bool                          `json:"hidden"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 128<<10)
	if c.ShouldBindJSON(&input) != nil || input.Revision == nil {
		capabilityError(c, errors.New("请提交配置版本和操作"))
		return
	}
	if err := model.ReconcileCapabilityGroups(); err != nil {
		capabilityError(c, errors.New("无法同步分组目录"))
		return
	}
	if input.Action == "presentation" && input.Presentation != nil {
		if err := model.ValidateCapabilityGroupReferences(*input.Presentation); err != nil {
			capabilityError(c, err)
			return
		}
	}
	row, err := model.UpdatePelicanControl(model.DB.WithContext(c.Request.Context()), *input.Revision, c.GetInt("id"), input.Action, func(tx *gorm.DB, row *model.PelicanControl) error {
		switch input.Action {
		case "hide_and_pause":
			row.Visible = false
			row.SyncEnabled = false
		case "hide":
			row.Visible = false
		case "show":
			row.Visible = true
		case "pause":
			row.SyncEnabled = false
		case "resume":
			row.SyncEnabled = true
		case "interval":
			row.IntervalMinutes = input.IntervalMinutes
		case "presentation":
			if input.Presentation == nil {
				return errors.New("缺少展示设置")
			}
			if err := model.ValidateCapabilityPresentation(*input.Presentation); err != nil {
				return err
			}
			// References checked before transaction below to avoid nested SQLite reads.
			raw, err := common.Marshal(input.Presentation)
			if err != nil {
				return err
			}
			row.Presentation = string(raw)
		case "mapping":
			if err := model.SavePelicanMapping(tx, input.TargetID, input.ChannelID, input.DisplayModel, input.Hidden); err != nil {
				return err
			}
			raw, _ := common.Marshal(map[string]any{"target_id": input.TargetID, "channel_id": input.ChannelID, "model": input.DisplayModel, "hidden": input.Hidden})
			if err := tx.Create(&model.PelicanEvent{ActorID: c.GetInt("id"), Action: "mapping_detail", At: time.Now().Unix(), Detail: string(raw)}).Error; err != nil {
				return err
			}
		default:
			return errors.New("未知操作")
		}
		return nil
	})
	if err != nil {
		capabilityError(c, err)
		return
	}
	data, err := pelicanControlDTO(row)
	if err != nil {
		capabilityError(c, err)
		return
	}
	c.JSON(200, gin.H{"success": true, "data": data})
}
func SyncPelicanAdmin(c *gin.Context) {
	n, err := service.SyncPelicanArchive(c.Request.Context())
	if err != nil {
		capabilityError(c, err)
		return
	}
	c.JSON(200, gin.H{"success": true, "imported": n})
}
func pelicanVisibleGroups(c *gin.Context, preview bool) (model.PelicanControl, model.CapabilityPresentation, []service.CapabilityGroupView, error) {
	row, err := model.GetPelicanControl(model.DB.WithContext(c.Request.Context()))
	if err != nil {
		return row, model.CapabilityPresentation{}, nil, err
	}
	view, err := row.View()
	if err != nil {
		return row, view, nil, err
	}
	if !row.Visible && !preview {
		return row, view, []service.CapabilityGroupView{}, nil
	}
	var allowed map[string]string
	if preview {
		allowed = map[string]string{}
		for k := range ratio_setting.GetGroupRatioCopy() {
			if k != "auto" {
				allowed[k] = setting.GetUsableGroupDescription(k)
			}
		}
	} else {
		allowed, err = service.CapabilityAllowedGroups(c.GetInt("id"))
		if err != nil {
			return row, view, nil, err
		}
	}
	groups, err := service.PelicanGroups(allowed, view)
	return row, view, groups, err
}
func GetPelicanOverview(c *gin.Context) { pelicanOverview(c, false) }
func GetPelicanPreview(c *gin.Context)  { pelicanOverview(c, true) }
func pelicanOverview(c *gin.Context, preview bool) {
	row, view, groups, err := pelicanVisibleGroups(c, preview)
	if err != nil {
		capabilityError(c, errors.New("无法读取测试记录"))
		return
	}
	var cfg pelicanarchive.Config
	_ = common.UnmarshalJsonStr(row.SourceConfig, &cfg)
	if !row.Visible && !preview {
		c.JSON(200, gin.H{"success": true, "visible": false, "data": []any{}})
		return
	}
	// Group overrides can mention private groups. Public presentation contains
	// only the controls consumed by this page; visible group text is in groups.
	view.Groups = map[string]model.CapabilityGroupOverride{}
	view.Order = []string{}
	c.JSON(200, gin.H{"success": true, "visible": true, "data": groups, "presentation": view, "last_imported_at": row.LastImportedAt, "source_captured_at": row.LastCapturedAt, "source_interval_minutes": cfg.IntervalMinutes, "source_auto_run": cfg.AutoRun, "preview": preview})
}
func GetPelicanResults(c *gin.Context)        { pelicanResults(c, false) }
func GetPelicanPreviewResults(c *gin.Context) { pelicanResults(c, true) }
func pelicanResults(c *gin.Context, preview bool) {
	_, view, groups, err := pelicanVisibleGroups(c, preview)
	if err != nil {
		capabilityError(c, errors.New("无法读取测试记录"))
		return
	}
	for _, g := range groups {
		if g.GroupUID == c.Param("group_uid") {
			name := c.Query("model")
			if name == "" {
				capabilityError(c, errors.New("请选择模型"))
				return
			}
			data, err := service.PelicanResults(model.DB.WithContext(c.Request.Context()), g, name, view.ShowHistory, view.GallerySize)
			if err != nil {
				capabilityError(c, errors.New("无法读取测试记录"))
				return
			}
			c.JSON(200, gin.H{"success": true, "data": data})
			return
		}
	}
	c.Status(http.StatusNotFound)
}
func GetPelicanRecord(c *gin.Context)      { pelicanRecord(c, false) }
func GetPelicanAdminRecord(c *gin.Context) { pelicanRecord(c, true) }
func pelicanRecord(c *gin.Context, admin bool) {
	if kind := c.Param("kind"); kind != "" && kind != "artwork" {
		c.Status(404)
		return
	}
	db := model.DB.WithContext(c.Request.Context())
	var rec model.PelicanRecord
	if db.First(&rec, "id = ?", c.Param("id")).Error != nil {
		c.Status(404)
		return
	}
	displayModel := ""
	if !admin {
		_, view, groups, err := pelicanVisibleGroups(c, false)
		if err != nil {
			c.Status(404)
			return
		}
		allowed := false
		for _, g := range groups {
			targets, err := service.PelicanTargetsForGroup(db, g.RoutingKey, "")
			if err != nil {
				c.Status(404)
				return
			}
			for _, t := range targets {
				if t.ID == rec.TargetID && rec.Active {
					var latest model.PelicanRecord
					if !view.ShowHistory {
						if db.Select("id").Where("target_id = ? AND active = ?", t.ID, true).Order("external_id DESC").First(&latest).Error != nil || latest.ID != rec.ID {
							continue
						}
					}
					allowed = true
					displayModel = t.DisplayModel
					break
				}
			}
			if allowed {
				break
			}
		}
		if !allowed {
			c.Status(404)
			return
		}
	}
	r, err := rec.Run()
	if err != nil {
		c.Status(500)
		return
	}
	if c.Param("kind") == "artwork" {
		if !rec.Preview || !pelicanarchive.SafeSVG(r.SVG) {
			c.Status(404)
			return
		}
		c.Header("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("Content-Disposition", "inline; filename=pelican.svg")
		c.Data(200, "image/svg+xml", []byte(r.SVG))
		return
	}
	// No provider names/IDs, errors, endpoints, raw replies or source internals
	// enter a user DTO. Raw replies can contain echoed provider diagnostics.
	data := gin.H{"id": rec.ID, "time": rec.TestedAt, "grade": r.Grade, "expected_answer": r.Expected, "reported_answer": r.Reported, "answer_in_svg": r.AnswerInSVG == 1, "answer_in_text": r.AnswerInText == 1, "truncated": r.Truncated == 1, "has_artwork": rec.Preview, "prompt": rec.Prompt, "prompt_hash": r.PromptHash, "prompt_source": "current_config_hash_match", "latency_ms": r.LatencyMS, "ttft_ms": r.TTFTMS, "input_tokens": r.InputTokens, "output_tokens": r.OutputTokens, "attempts": r.Attempts}
	if rec.Prompt == "" {
		data["prompt_source"] = "unavailable"
	}
	if admin {
		var target model.PelicanTarget
		if db.Select("display_model").First(&target, "id = ?", rec.TargetID).Error == nil {
			displayModel = target.DisplayModel
		}
		data["source_record"] = r
		data["record"] = rec
	}
	data["model"] = displayModel
	c.JSON(200, gin.H{"success": true, "data": data})
}
func GetPelicanAdminRecords(c *gin.Context) {
	page := common.GetPageQuery(c)
	var records []model.PelicanRecord
	query := model.DB.WithContext(c.Request.Context()).Model(&model.PelicanRecord{})
	if id := c.Query("target_id"); id != "" {
		query = query.Where("target_id = ?", id)
	}
	var count int64
	if query.Count(&count).Error != nil {
		c.Status(500)
		return
	}
	if query.Omit("payload", "prompt").Order("tested_at DESC, external_id DESC").Limit(page.GetPageSize()).Offset(page.GetStartIdx()).Find(&records).Error != nil {
		c.Status(500)
		return
	}
	c.JSON(200, gin.H{"success": true, "data": records, "total": count})
}
