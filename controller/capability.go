package controller

import (
	"errors"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/capabilitytest"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

func capabilityError(c *gin.Context, err error) {
	status := http.StatusBadRequest
	if errors.Is(err, model.ErrCapabilityConflict) {
		status = http.StatusConflict
	}
	c.JSON(status, gin.H{"success": false, "message": err.Error()})
}
func capabilityControlDTO(row model.CapabilityControl) (gin.H, error) {
	config, err := row.Config()
	if err != nil {
		return nil, err
	}
	draft, err := row.View(true)
	if err != nil {
		return nil, err
	}
	view, err := row.View(false)
	if err != nil {
		return nil, err
	}
	return gin.H{"revision": row.Revision, "running": row.Running, "visible": row.Visible, "updated_at": row.UpdatedAt, "config": config, "draft": draft, "presentation": view, "next_times": service.CapabilityScheduleTimes(config, time.Now(), 5)}, nil
}
func GetCapabilityAdminControl(c *gin.Context) {
	row, err := model.GetCapabilityControl()
	if err != nil {
		capabilityError(c, errors.New("无法读取测试配置"))
		return
	}
	data, err := capabilityControlDTO(row)
	if err != nil {
		capabilityError(c, errors.New("测试配置格式异常"))
		return
	}
	c.JSON(200, gin.H{"success": true, "data": data})
}
func UpdateCapabilityAdminControl(c *gin.Context) {
	var input struct {
		Revision     *int64                        `json:"revision"`
		Action       string                        `json:"action"`
		Config       *model.CapabilityConfig       `json:"config"`
		Presentation *model.CapabilityPresentation `json:"presentation"`
		Order        *[]string                     `json:"order"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 128<<10)
	if err := c.ShouldBindJSON(&input); err != nil || input.Revision == nil {
		capabilityError(c, errors.New("请提交操作类型和配置版本"))
		return
	}
	row, err := model.UpdateCapabilityControl(*input.Revision, c.GetInt("id"), input.Action, func(row *model.CapabilityControl) error {
		switch input.Action {
		case "stop":
			row.Running = false
		case "hide_and_stop":
			row.Visible = false
			row.Running = false
		case "hide_only":
			row.Visible = false
		case "show_only":
			row.Visible = true
		case "resume":
			config, err := row.Config()
			if err != nil {
				return err
			}
			if err = service.ValidateCapabilityConfig(config); err != nil {
				return err
			}
			if len(capabilityReadiness(config)) > 0 {
				return errors.New("启用条件未满足，请查看运行诊断")
			}
			row.Running = true
		case "save_config":
			if input.Config == nil {
				return errors.New("缺少测试配置")
			}
			if err := service.ValidateCapabilityConfig(*input.Config); err != nil {
				return err
			}
			raw, err := common.Marshal(input.Config)
			if err != nil {
				return err
			}
			row.Settings = string(raw)
		case "save_draft":
			if input.Presentation == nil {
				return errors.New("缺少展示配置")
			}
			if err := model.ValidateCapabilityPresentation(*input.Presentation); err != nil {
				return err
			}
			raw, err := common.Marshal(input.Presentation)
			if err != nil {
				return err
			}
			row.Draft = string(raw)
		case "publish":
			if input.Presentation != nil {
				if err := model.ValidateCapabilityPresentation(*input.Presentation); err != nil {
					return err
				}
				raw, err := common.Marshal(input.Presentation)
				if err != nil {
					return err
				}
				row.Draft = string(raw)
			}
			v, err := row.View(true)
			if err != nil {
				return err
			}
			if err = model.ValidateCapabilityPresentation(v); err != nil {
				return err
			}
			row.Presentation = row.Draft
		case "set_order":
			if input.Order == nil {
				return errors.New("缺少展示顺序")
			}
			for _, isDraft := range []bool{true, false} {
				view, err := row.View(isDraft)
				if err != nil {
					return err
				}
				view.Order = *input.Order
				if err = model.ValidateCapabilityPresentation(view); err != nil {
					return err
				}
				raw, err := common.Marshal(view)
				if err != nil {
					return err
				}
				if isDraft {
					row.Draft = string(raw)
				} else {
					row.Presentation = string(raw)
				}
			}
		default:
			return errors.New("未知操作")
		}
		if input.Action == "stop" || input.Action == "hide_and_stop" || input.Action == "resume" {
			row.ExecutionRevision++
		}
		return nil
	})
	if err != nil {
		capabilityError(c, err)
		return
	}
	data, err := capabilityControlDTO(row)
	if err != nil {
		capabilityError(c, errors.New("无法读取保存结果"))
		return
	}
	c.JSON(200, gin.H{"success": true, "data": data})
}
func ReconcileCapabilityGroups(c *gin.Context) {
	if err := model.ReconcileCapabilityGroups(); err != nil {
		capabilityError(c, errors.New("分组同步失败"))
		return
	}
	GetCapabilityAdminGroups(c)
}
func GetCapabilityAdminGroups(c *gin.Context) {
	row, err := model.GetCapabilityControl()
	if err != nil {
		capabilityError(c, errors.New("读取配置失败"))
		return
	}
	view, err := row.View(true)
	if err != nil {
		capabilityError(c, err)
		return
	}
	allowed := map[string]string{}
	for key := range ratio_setting.GetGroupRatioCopy() {
		if key != "auto" {
			allowed[key] = setting.GetUsableGroupDescription(key)
		}
	}
	// Management must include hidden groups so they remain editable.
	for uid, v := range view.Groups {
		v.Hidden = false
		view.Groups[uid] = v
	}
	groups, err := service.CapabilityGroups(allowed, view)
	if err != nil {
		capabilityError(c, errors.New("读取分组失败"))
		return
	}
	c.JSON(200, gin.H{"success": true, "data": groups, "revision": row.Revision})
}
func GetCapabilityAdminTargets(c *gin.Context) {
	row, err := model.GetCapabilityControl()
	if err != nil {
		capabilityError(c, errors.New("读取配置失败"))
		return
	}
	config, err := row.Config()
	if err != nil {
		capabilityError(c, err)
		return
	}
	targets, err := service.DiscoverCapabilityTargets(config)
	if err != nil {
		capabilityError(c, errors.New("读取测试范围失败"))
		return
	}
	c.JSON(200, gin.H{"success": true, "data": targets, "revision": row.Revision, "observed_at": time.Now().Unix()})
}
func GetCapabilityGroups(c *gin.Context) {
	row, err := model.GetCapabilityControl()
	if err != nil {
		capabilityError(c, errors.New("测试暂不可用"))
		return
	}
	if !row.Visible {
		c.JSON(200, gin.H{"success": true, "data": []any{}, "visible": false, "revision": row.Revision})
		return
	}
	allowed, err := service.CapabilityAllowedGroups(c.GetInt("id"))
	if err != nil {
		capabilityError(c, errors.New("无法核验分组权限"))
		return
	}
	view, err := row.View(false)
	if err != nil {
		capabilityError(c, errors.New("测试暂不可用"))
		return
	}
	groups, err := service.CapabilityGroups(allowed, view)
	if err != nil {
		capabilityError(c, errors.New("测试暂不可用"))
		return
	}
	config, err := row.Config()
	if err != nil {
		capabilityError(c, errors.New("测试暂不可用"))
		return
	}
	c.JSON(200, gin.H{"success": true, "data": groups, "visible": true, "running": row.Running, "revision": row.Revision, "copy": view.Copy, "show_method": view.ShowMethod, "show_history": view.ShowHistory, "gallery_size": view.GallerySize, "next_times": service.CapabilityScheduleTimes(config, time.Now(), 1), "method": gin.H{"suite": capabilitytest.SuiteVersion, "interval_minutes": config.IntervalMinutes, "timezone": config.Timezone, "all_day": config.AllDay, "window_start": config.WindowStart, "window_end": config.WindowEnd, "weekdays": config.Weekdays}})
}
