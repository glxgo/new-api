package service

import (
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

// This whitelist is shared by the admin catalogue and public eligibility checks.
// Channel credentials must never be loaded for archive presentation.
type PelicanChannel struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Status int    `json:"status"`
	Group  string `json:"group"`
	Models string `json:"models"`
}

type pelicanTopology struct {
	channels  map[int]PelicanChannel
	abilities map[int]map[string]map[string]bool
}

func loadPelicanTopology(db *gorm.DB) (pelicanTopology, []PelicanChannel, error) {
	topology := pelicanTopology{channels: map[int]PelicanChannel{}, abilities: map[int]map[string]map[string]bool{}}
	channels := []PelicanChannel{}
	var abilities []model.Ability
	if err := db.Model(&model.Channel{}).Select("id", "name", "status", "group", "models").Order("id").Scan(&channels).Error; err != nil {
		return topology, nil, err
	}
	if err := db.Select("channel_id", "group", "model", "enabled").Find(&abilities).Error; err != nil {
		return topology, nil, err
	}
	for _, c := range channels {
		topology.channels[c.ID] = c
	}
	for _, a := range abilities {
		if topology.abilities[a.ChannelId] == nil {
			topology.abilities[a.ChannelId] = map[string]map[string]bool{}
		}
		if topology.abilities[a.ChannelId][a.Group] == nil {
			topology.abilities[a.ChannelId][a.Group] = map[string]bool{}
		}
		topology.abilities[a.ChannelId][a.Group][a.Model] = a.Enabled
	}
	return topology, channels, nil
}

func (p pelicanTopology) targetReason(t model.PelicanTarget) string {
	if !t.Present {
		return "source_removed"
	}
	if !t.Enabled {
		return "source_disabled"
	}
	if t.Hidden {
		return "target_hidden"
	}
	if t.ChannelID == 0 {
		return "unmapped"
	}
	c, ok := p.channels[t.ChannelID]
	if !ok {
		return "channel_missing"
	}
	if t.DisplayModel == "" || !capabilityContains(splitPelican(c.Models), t.DisplayModel) {
		return "model_unavailable"
	}
	return ""
}

func (p pelicanTopology) groupEligible(t model.PelicanTarget, group string) bool {
	enabled, exists := p.abilities[t.ChannelID][group][t.DisplayModel]
	c := p.channels[t.ChannelID]
	// Disabling a business channel also disables its abilities. That does not
	// revoke external archive display, but membership/model removal still does.
	return p.targetReason(t) == "" && capabilityContains(splitPelican(c.Group), group) && exists && (enabled || c.Status != common.ChannelStatusEnabled)
}

// Keep the existing permission/identity/presentation rules, but list only
// models with eligible archive associations (including disabled channels).
func PelicanGroups(allowed map[string]string, view model.CapabilityPresentation) ([]CapabilityGroupView, error) {
	groups, err := CapabilityGroups(allowed, view)
	if err != nil {
		return nil, err
	}
	topology, _, err := loadPelicanTopology(model.DB)
	if err != nil {
		return nil, err
	}
	var targets []model.PelicanTarget
	if err := model.DB.Find(&targets).Error; err != nil {
		return nil, err
	}
	for i := range groups {
		groups[i].Models = []string{}
		for _, target := range targets {
			if topology.groupEligible(target, groups[i].RoutingKey) && !capabilityContains(groups[i].Models, target.DisplayModel) {
				groups[i].Models = append(groups[i].Models, target.DisplayModel)
			}
		}
		sort.Strings(groups[i].Models)
	}
	return groups, nil
}

type PelicanMappingGroup struct {
	GroupUID    string   `json:"group_uid"`
	RoutingKey  string   `json:"routing_key"`
	DisplayName string   `json:"display_name"`
	Ratio       *float64 `json:"ratio"`
	Eligible    bool     `json:"eligible"`
	Reason      string   `json:"reason"`
}
type PelicanMappingReport struct {
	TargetID        string                `json:"target_id"`
	Reason          string                `json:"reason"`
	ChannelDisabled bool                  `json:"channel_disabled"`
	Groups          []PelicanMappingGroup `json:"groups"`
}

// Reports describe saved relationships, not unsaved form values or a promise
// that a target wins selection. Public authorization remains request-specific.
func PelicanMappingReports(db *gorm.DB, targets []model.PelicanTarget, groups []CapabilityGroupView, view model.CapabilityPresentation) ([]PelicanMappingReport, []PelicanChannel, error) {
	topology, channels, err := loadPelicanTopology(db)
	if err != nil {
		return nil, nil, err
	}
	registered := map[string]CapabilityGroupView{}
	for _, g := range groups {
		registered[g.RoutingKey] = g
	}
	reports := make([]PelicanMappingReport, 0, len(targets))
	for _, target := range targets {
		r := PelicanMappingReport{TargetID: target.ID, Reason: topology.targetReason(target), Groups: []PelicanMappingGroup{}}
		if c, exists := topology.channels[target.ChannelID]; exists {
			r.ChannelDisabled = c.Status != common.ChannelStatusEnabled
		}
		seen := map[string]bool{}
		eligible := false
		for _, key := range splitPelican(topology.channels[target.ChannelID].Group) {
			key = strings.TrimSpace(key)
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			g, exists := registered[key]
			item := PelicanMappingGroup{GroupUID: g.GroupUID, RoutingKey: key, DisplayName: key, Ratio: g.Ratio, Reason: r.Reason}
			override := view.Groups[g.GroupUID]
			if override.Name != nil {
				item.DisplayName = *override.Name
			}
			if item.Reason == "" {
				switch {
				case !exists:
					item.Reason = "group_unavailable"
				case override.Hidden:
					item.Reason = "group_hidden"
				case !topology.groupEligible(target, key):
					item.Reason = "ability_unavailable"
				default:
					item.Eligible = true
					eligible = true
				}
			}
			r.Groups = append(r.Groups, item)
		}
		if r.Reason == "" && !eligible {
			r.Reason = "no_eligible_group"
		}
		sort.Slice(r.Groups, func(i, j int) bool { return r.Groups[i].RoutingKey < r.Groups[j].RoutingKey })
		reports = append(reports, r)
	}
	return reports, channels, nil
}
