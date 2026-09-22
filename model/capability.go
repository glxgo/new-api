package model

import (
	"errors"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrCapabilityConflict = errors.New("configuration changed; reload before saving")

type CapabilityControl struct {
	ID                uint   `json:"-" gorm:"primaryKey;autoIncrement:false"`
	Revision          int64  `json:"revision"`
	ExecutionRevision int64  `json:"execution_revision"`
	Running           bool   `json:"running"`
	Visible           bool   `json:"visible"`
	Settings          string `json:"-" gorm:"type:text"`
	Draft             string `json:"-" gorm:"type:text"`
	Presentation      string `json:"-" gorm:"type:text"`
	UpdatedAt         int64  `json:"updated_at"`
}

type CapabilityConfig struct {
	IntervalMinutes   int                   `json:"interval_minutes"`
	Anchor            int64                 `json:"anchor"`
	Timezone          string                `json:"timezone"`
	Weekdays          []int                 `json:"weekdays"`
	WindowStart       int                   `json:"window_start"`
	WindowEnd         int                   `json:"window_end"`
	AllDay            bool                  `json:"all_day"`
	Models            []CapabilityProfile   `json:"models"`
	Excluded          []CapabilityExclusion `json:"excluded"`
	DailyBudgetMicros int64                 `json:"daily_budget_micros"`
	CallReserveMicros int64                 `json:"call_reserve_micros"`
	JudgeChannelID    int                   `json:"judge_channel_id"`
	JudgeModel        string                `json:"judge_model"`
}
type CapabilityProfile struct {
	Model     string `json:"model"`
	Protocol  string `json:"protocol"`
	MaxTokens uint   `json:"max_tokens"`
	Enabled   bool   `json:"enabled"`
}
type CapabilityExclusion struct {
	ChannelID int    `json:"channel_id"`
	GroupUID  string `json:"group_uid"`
	Model     string `json:"model"`
}
type CapabilityPresentation struct {
	Groups      map[string]CapabilityGroupOverride `json:"groups"`
	Order       []string                           `json:"order"`
	Copy        map[string]string                  `json:"copy"`
	ShowMethod  bool                               `json:"show_method"`
	ShowHistory bool                               `json:"show_history"`
	GallerySize int                                `json:"gallery_size"`
}
type CapabilityGroupOverride struct {
	Name            *string `json:"name"`
	DescriptionMode string  `json:"description_mode"`
	Description     string  `json:"description"`
	Hidden          bool    `json:"hidden"`
}

// Archived identities have a NULL routing key. Display text never identifies
// an object, and a re-created business group receives a fresh UUID.
type CapabilityGroupPresentation struct {
	GroupUID    string  `json:"group_uid" gorm:"primaryKey;type:varchar(36)"`
	RoutingKey  *string `json:"routing_key" gorm:"uniqueIndex;type:varchar(191)"`
	PreviousKey string  `json:"previous_key" gorm:"type:varchar(191)"`
	CreatedAt   int64   `json:"created_at"`
	ArchivedAt  int64   `json:"archived_at"`
}
type CapabilityEvent struct {
	ID        uint   `json:"id" gorm:"primaryKey"`
	Revision  int64  `json:"revision" gorm:"index"`
	ActorID   int    `json:"actor_id"`
	Action    string `json:"action" gorm:"type:varchar(40)"`
	CreatedAt int64  `json:"created_at"`
	Before    string `json:"before" gorm:"type:text"`
	After     string `json:"after" gorm:"type:text"`
}

func DefaultCapabilityConfig() CapabilityConfig {
	return CapabilityConfig{IntervalMinutes: 30, Timezone: "Asia/Shanghai", Weekdays: []int{0, 1, 2, 3, 4, 5, 6}, AllDay: true, Models: []CapabilityProfile{}, Excluded: []CapabilityExclusion{}}
}
func DefaultCapabilityPresentation() CapabilityPresentation {
	return CapabilityPresentation{Groups: map[string]CapabilityGroupOverride{}, Order: []string{}, Copy: map[string]string{}, ShowMethod: true, ShowHistory: true, GallerySize: 3}
}
func defaultCapabilityControl() CapabilityControl {
	config, _ := common.Marshal(DefaultCapabilityConfig())
	view, _ := common.Marshal(DefaultCapabilityPresentation())
	return CapabilityControl{ID: 1, Settings: string(config), Draft: string(view), Presentation: string(view)}
}

// Reads never create rows, including on slave candidates.
func GetCapabilityControl() (CapabilityControl, error) {
	var row CapabilityControl
	err := DB.First(&row, 1).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return defaultCapabilityControl(), nil
	}
	return row, err
}
func (row CapabilityControl) Config() (CapabilityConfig, error) {
	v := DefaultCapabilityConfig()
	err := common.UnmarshalJsonStr(row.Settings, &v)
	return v, err
}
func (row CapabilityControl) View(draft bool) (CapabilityPresentation, error) {
	v := DefaultCapabilityPresentation()
	data := row.Presentation
	if draft {
		data = row.Draft
	}
	err := common.UnmarshalJsonStr(data, &v)
	return v, err
}
func UpdateCapabilityControl(expected int64, actor int, action string, mutate func(*CapabilityControl) error) (CapabilityControl, error) {
	var updated CapabilityControl
	err := DB.Transaction(func(tx *gorm.DB) error {
		seed := defaultCapabilityControl()
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&seed).Error; err != nil {
			return err
		}
		if err := tx.First(&updated, 1).Error; err != nil {
			return err
		}
		if updated.Revision != expected {
			return ErrCapabilityConflict
		}
		before, err := capabilityAuditSnapshot(updated)
		if err != nil {
			return err
		}
		if err := mutate(&updated); err != nil {
			return err
		}
		if action == "save_draft" || action == "publish" || action == "set_order" {
			v, err := updated.View(true)
			if err != nil {
				return err
			}
			if err = validateCapabilityGroupReferences(tx, v); err != nil {
				return err
			}
		}
		updated.Revision++
		updated.UpdatedAt = time.Now().Unix()
		r := tx.Model(&CapabilityControl{}).Where("id = ? AND revision = ?", 1, expected).Updates(map[string]any{"revision": updated.Revision, "execution_revision": updated.ExecutionRevision, "running": updated.Running, "visible": updated.Visible, "settings": updated.Settings, "draft": updated.Draft, "presentation": updated.Presentation, "updated_at": updated.UpdatedAt})
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected != 1 {
			return ErrCapabilityConflict
		}
		after, err := capabilityAuditSnapshot(updated)
		if err != nil {
			return err
		}
		return tx.Create(&CapabilityEvent{Revision: updated.Revision, ActorID: actor, Action: action, CreatedAt: updated.UpdatedAt, Before: before, After: after}).Error
	})
	return updated, err
}

func capabilityAuditSnapshot(row CapabilityControl) (string, error) {
	// Test configuration never contains channel credentials or client headers.
	value, err := common.Marshal(map[string]any{"running": row.Running, "visible": row.Visible, "settings": row.Settings, "draft": row.Draft, "presentation": row.Presentation})
	return string(value), err
}

// Only explicit admin actions and the master worker reconcile identities.
func ReconcileCapabilityGroups() error {
	groups := ratio_setting.GetGroupRatioCopy()
	delete(groups, "auto")
	delete(groups, "")
	return DB.Transaction(func(tx *gorm.DB) error {
		var existing []CapabilityGroupPresentation
		if err := tx.Where("routing_key IS NOT NULL").Find(&existing).Error; err != nil {
			return err
		}
		seen := map[string]bool{}
		for _, g := range existing {
			key := *g.RoutingKey
			seen[key] = true
			if _, ok := groups[key]; !ok {
				if err := tx.Model(&g).Updates(map[string]any{"routing_key": nil, "previous_key": key, "archived_at": time.Now().Unix()}).Error; err != nil {
					return err
				}
			}
		}
		for key := range groups {
			if !seen[key] {
				key := key
				g := CapabilityGroupPresentation{GroupUID: uuid.NewString(), RoutingKey: &key, CreatedAt: time.Now().Unix()}
				if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&g).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func ValidateCapabilityPresentation(v CapabilityPresentation) error {
	if v.GallerySize < 1 || v.GallerySize > 3 {
		return errors.New("gallery_size must be 1–3")
	}
	for _, g := range v.Groups {
		if g.Name != nil && (strings.TrimSpace(*g.Name) == "" || len([]rune(*g.Name)) > 40) {
			return errors.New("display name must be 1–40 characters")
		}
		if g.DescriptionMode != "" && g.DescriptionMode != "inherit" && g.DescriptionMode != "custom" && g.DescriptionMode != "hidden" {
			return errors.New("invalid description mode")
		}
		if len([]rune(g.Description)) > 160 || (g.DescriptionMode == "custom" && strings.TrimSpace(g.Description) == "") {
			return errors.New("custom description must be 1–160 characters")
		}
	}
	slots := map[string]bool{"page_title": true, "page_intro": true, "gallery_title": true, "method_intro": true, "empty_text": true}
	for _, key := range []string{"method_scope_title", "method_evidence_title", "method_selection_title", "method_reading_title", "method_trace_title", "method_limits_title"} {
		slots[key] = true
	}
	for key, value := range v.Copy {
		if !slots[key] || len([]rune(value)) > 500 {
			return errors.New("invalid copy slot or copy longer than 500 characters")
		}
		if strings.Contains(value, "正确性优先 · 美观另评") {
			return errors.New("please use formal public copy")
		}
	}
	seen := map[string]bool{}
	for _, uid := range v.Order {
		if seen[uid] {
			return errors.New("duplicate group in order")
		}
		seen[uid] = true
	}
	return nil
}

func ValidateCapabilityGroupReferences(v CapabilityPresentation) error {
	return validateCapabilityGroupReferences(DB, v)
}
func validateCapabilityGroupReferences(db *gorm.DB, v CapabilityPresentation) error {
	var rows []CapabilityGroupPresentation
	if err := db.Where("routing_key IS NOT NULL").Find(&rows).Error; err != nil {
		return err
	}
	known := map[string]bool{}
	for _, row := range rows {
		known[row.GroupUID] = true
	}
	for uid := range v.Groups {
		if !known[uid] {
			return errors.New("unknown or archived group")
		}
	}
	for _, uid := range v.Order {
		if !known[uid] {
			return errors.New("unknown or archived group in order")
		}
	}
	return nil
}
