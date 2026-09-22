package service

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/pelicanarchive"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PelicanBindings struct {
	SourceID string           `json:"source_id"`
	Bindings []PelicanBinding `json:"bindings"`
}

type PelicanBinding struct {
	ProviderID string `json:"provider_id"`
	Model      string `json:"model"`
	ChannelID  int    `json:"channel_id"`
}

type PelicanBootstrapReport struct {
	PlanHash    string `json:"plan_hash"`
	Applied     bool   `json:"applied"`
	Targets     int    `json:"targets"`
	Records     int    `json:"records"`
	Mappings    int    `json:"mappings"`
	Groups      int    `json:"groups"`
	Memberships int    `json:"memberships"`
	CapturedAt  string `json:"captured_at"`
	Visible     bool   `json:"visible"`
	SyncEnabled bool   `json:"sync_enabled"`
}

// BootstrapPelicanArchive is a one-time data initializer, never a migration or
// worker. Dry runs issue only SELECTs. Apply requires a matching reviewed plan,
// uses a single transaction, and commits hidden/paused. Existing data is refused
// rather than resetting operators' settings. No business rows are updated.
func BootstrapPelicanArchive(db *gorm.DB, snapshot pelicanarchive.Snapshot, bindings PelicanBindings, actor int, apply bool, expectedPlan string) (PelicanBootstrapReport, error) {
	var report PelicanBootstrapReport
	raw, err := common.Marshal(snapshot)
	if err != nil {
		return report, err
	}
	snapshot, err = pelicanarchive.Decode(bytes.NewReader(raw), bindings.SourceID, time.Now())
	if err != nil {
		return report, err
	}
	if actor <= 0 || len(bindings.Bindings) == 0 || len(bindings.Bindings) > 2000 || len(snapshot.Runs) == 0 {
		return report, errors.New("bootstrap_requires_actor_bindings_and_records")
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		var user model.User
		if err := tx.Select("id", "role", "status").First(&user, actor).Error; err != nil || user.Role != common.RoleRootUser || user.Status != common.UserStatusEnabled {
			return errors.New("bootstrap_requires_existing_enabled_root")
		}
		for _, table := range []any{&model.PelicanControl{}, &model.PelicanTarget{}, &model.PelicanRecord{}, &model.PelicanEvent{}} {
			var count int64
			if err := tx.Model(table).Count(&count).Error; err != nil {
				return errors.New("bootstrap_schema_unavailable")
			}
			if count != 0 {
				return errors.New("bootstrap_requires_empty_archive_tables")
			}
		}
		var option model.Option
		if err := tx.Where(map[string]any{"key": "GroupRatio"}).First(&option).Error; err != nil {
			return errors.New("bootstrap_group_configuration_missing")
		}
		var ratios map[string]float64
		if common.UnmarshalJsonStr(option.Value, &ratios) != nil || len(ratios) == 0 {
			return errors.New("bootstrap_invalid_groups")
		}
		delete(ratios, "auto")
		delete(ratios, "")
		keys := make([]string, 0, len(ratios))
		for key, ratio := range ratios {
			if strings.TrimSpace(key) != key || len(key) > 191 || math.IsNaN(ratio) || math.IsInf(ratio, 0) || ratio < 0 {
				return errors.New("bootstrap_invalid_groups")
			}
			keys = append(keys, key)
		}
		sort.Strings(keys)
		if len(keys) == 0 {
			return errors.New("bootstrap_invalid_groups")
		}
		var identities []model.CapabilityGroupPresentation
		if err := tx.Order("group_uid").Find(&identities).Error; err != nil {
			return errors.New("bootstrap_schema_unavailable")
		}
		topology, channels, err := loadPelicanTopology(tx)
		if err != nil {
			return err
		}
		var abilities []model.Ability
		if err := tx.Select("channel_id", "group", "model", "enabled").Order("channel_id").Order(clause.OrderByColumn{Column: clause.Column{Name: "group"}}).Order("model").Find(&abilities).Error; err != nil {
			return err
		}
		targets := map[string]pelicanarchive.Target{}
		for _, target := range snapshot.Targets {
			targets[model.PelicanTargetID(snapshot.SourceID, target.ProviderID, target.Model)] = target
		}
		seen, channelModels := map[string]bool{}, map[string]bool{}
		memberships := 0
		for _, binding := range bindings.Bindings {
			id := model.PelicanTargetID(snapshot.SourceID, binding.ProviderID, binding.Model)
			target, exists := targets[id]
			pair := fmt.Sprintf("%d\x00%s", binding.ChannelID, binding.Model)
			if !exists || seen[id] || channelModels[pair] || binding.ChannelID <= 0 {
				return errors.New("bootstrap_invalid_or_duplicate_binding")
			}
			seen[id], channelModels[pair] = true, true
			mapped := model.PelicanTarget{Present: true, Enabled: target.Enabled == 1, ChannelID: binding.ChannelID, DisplayModel: binding.Model}
			eligible := 0
			for _, key := range keys {
				if topology.groupEligible(mapped, key) {
					eligible++
				}
			}
			if eligible == 0 {
				return errors.New("bootstrap_binding_has_no_eligible_group")
			}
			memberships += eligible
		}
		// Whitelist-only topology: never hash or read keys, upstream addresses or
		// unrelated options. Any reviewed input changing requires a new dry run.
		plan, err := common.Marshal(struct {
			Snapshot   string
			Bindings   PelicanBindings
			Ratios     map[string]float64
			Channels   []PelicanChannel
			Abilities  []model.Ability
			Identities []model.CapabilityGroupPresentation
			Actor      int
		}{pelicanarchive.Hash(raw), bindings, ratios, channels, abilities, identities, actor})
		if err != nil {
			return err
		}
		report = PelicanBootstrapReport{PlanHash: pelicanarchive.Hash(plan), Targets: len(snapshot.Targets), Records: len(snapshot.Runs), Mappings: len(bindings.Bindings), Groups: len(keys), Memberships: memberships, CapturedAt: snapshot.CapturedAt}
		if !apply {
			return nil
		}
		if expectedPlan == "" || expectedPlan != report.PlanHash {
			return errors.New("bootstrap_plan_changed_rerun_check")
		}
		seed := model.DefaultPelicanControl()
		seed.SyncEnabled = true // uncommitted, required only for the bounded import
		if err := tx.Create(&seed).Error; err != nil {
			return err
		}
		for _, key := range keys {
			key := key
			identity := model.CapabilityGroupPresentation{GroupUID: uuid.NewString(), RoutingKey: &key, CreatedAt: time.Now().Unix()}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&identity).Error; err != nil {
				return err
			}
		}
		if _, err := model.ImportPelicanSnapshot(tx, snapshot, 0); err != nil {
			return err
		}
		for _, binding := range bindings.Bindings {
			if err := model.SavePelicanMapping(tx, model.PelicanTargetID(snapshot.SourceID, binding.ProviderID, binding.Model), binding.ChannelID, binding.Model, false); err != nil {
				return err
			}
		}
		if err := tx.Model(&model.PelicanControl{}).Where("id = ?", 1).Updates(map[string]any{"sync_enabled": false, "visible": false, "revision": 1}).Error; err != nil {
			return err
		}
		report.Applied = true
		detail, err := common.Marshal(report)
		if err != nil {
			return err
		}
		if err := tx.Create(&model.PelicanEvent{ActorID: actor, Action: "bootstrap", At: time.Now().Unix(), Detail: string(detail)}).Error; err != nil {
			return err
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return PelicanBootstrapReport{}, err
	}
	return report, nil
}
