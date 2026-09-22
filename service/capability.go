package service

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"gorm.io/gorm"
)

type CapabilityTarget struct {
	GroupUID    string `json:"group_uid"`
	Group       string `json:"group"`
	Model       string `json:"model"`
	ChannelID   int    `json:"channel_id"`
	ChannelName string `json:"channel_name"`
	Eligible    bool   `json:"eligible"`
	Reason      string `json:"reason"`
}

func DiscoverCapabilityTargets(config model.CapabilityConfig) ([]CapabilityTarget, error) {
	return DiscoverCapabilityTargetsWithDB(model.DB, config)
}
func DiscoverCapabilityTargetsWithDB(db *gorm.DB, config model.CapabilityConfig) ([]CapabilityTarget, error) {
	var abilities []model.Ability
	var channels []model.Channel
	var groups []model.CapabilityGroupPresentation
	if err := db.Find(&abilities).Error; err != nil {
		return nil, err
	}
	if err := db.Select("id", "name", "status", "group", "models").Find(&channels).Error; err != nil {
		return nil, err
	}
	if err := db.Where("routing_key IS NOT NULL").Find(&groups).Error; err != nil {
		return nil, err
	}
	cm := map[int]model.Channel{}
	gm := map[string]string{}
	pm := map[string]bool{}
	for _, c := range channels {
		cm[c.Id] = c
	}
	for _, g := range groups {
		gm[*g.RoutingKey] = g.GroupUID
	}
	for _, p := range config.Models {
		pm[p.Model] = p.Enabled
	}
	result := make([]CapabilityTarget, 0, len(abilities))
	known := map[string]bool{}
	abilityKey := func(group, model string, id int) string { return fmt.Sprintf("%s\x00%s\x00%d", group, model, id) }
	for _, a := range abilities {
		known[abilityKey(a.Group, a.Model, a.ChannelId)] = true
	}
	missing := map[string]bool{}
	for _, channel := range channels {
		for _, group := range strings.Split(channel.Group, ",") {
			for _, name := range strings.Split(channel.Models, ",") {
				group, name = strings.TrimSpace(group), strings.TrimSpace(name)
				if group == "" || name == "" || group == "auto" {
					continue
				}
				key := abilityKey(group, name, channel.Id)
				if !known[key] {
					abilities = append(abilities, model.Ability{Group: group, Model: name, ChannelId: channel.Id})
					missing[key] = true
					known[key] = true
				}
			}
		}
	}
	for _, a := range abilities {
		if a.Group == "auto" {
			continue
		}
		c, ok := cm[a.ChannelId]
		t := CapabilityTarget{GroupUID: gm[a.Group], Group: a.Group, Model: a.Model, ChannelID: a.ChannelId, ChannelName: c.Name}
		switch {
		case !ok:
			t.Reason = "channel_missing"
		case t.GroupUID == "":
			t.Reason = "identity_pending"
		case missing[abilityKey(a.Group, a.Model, a.ChannelId)]:
			t.Reason = "ability_missing"
		case !a.Enabled:
			t.Reason = "ability_disabled"
		case c.Status != common.ChannelStatusEnabled:
			t.Reason = "channel_disabled"
		case !capabilityContains(strings.Split(c.Group, ","), a.Group) || !capabilityContains(strings.Split(c.Models, ","), a.Model):
			t.Reason = "topology_inconsistent"
		case !pm[a.Model]:
			t.Reason = "model_not_approved"
		}
		for _, e := range config.Excluded {
			if (e.ChannelID == 0 || e.ChannelID == a.ChannelId) && (e.GroupUID == "" || e.GroupUID == t.GroupUID) && (e.Model == "" || e.Model == a.Model) {
				t.Reason = "test_excluded"
			}
		}
		t.Eligible = t.Reason == ""
		result = append(result, t)
	}
	sort.Slice(result, func(i, j int) bool {
		a, b := result[i], result[j]
		if a.Group != b.Group {
			return a.Group < b.Group
		}
		if a.Model != b.Model {
			return a.Model < b.Model
		}
		return a.ChannelID < b.ChannelID
	})
	return result, nil
}
func capabilityContains(values []string, value string) bool {
	for _, v := range values {
		if strings.TrimSpace(v) == value {
			return true
		}
	}
	return false
}

type CapabilityGroupView struct {
	GroupUID    string   `json:"group_uid"`
	RoutingKey  string   `json:"routing_key"`
	DisplayName string   `json:"display_name"`
	Description string   `json:"description"`
	Ratio       *float64 `json:"ratio"`
	IconType    int      `json:"icon_type"`
	Models      []string `json:"models"`
}

func CapabilityAllowedGroups(userID int) (map[string]string, error) {
	userGroup, err := model.GetUserGroup(userID, false)
	if err != nil {
		return nil, err
	}
	usable := GetUserUsableGroups(userGroup)
	allowed := map[string]string{}
	for key, desc := range usable {
		if key != "auto" && ratio_setting.ContainsGroupRatio(key) {
			allowed[key] = desc
		}
	}
	subs, err := model.GetActiveUserSubscriptionAllowedGroups(userID)
	if err != nil {
		return nil, err
	}
	for _, key := range subs {
		if _, ok := allowed[key]; !ok {
			allowed[key] = setting.GetUsableGroupDescription(key)
		}
	}
	memberships, err := model.GetActiveUserVirtualMembershipAllowedGroups(userID)
	if err != nil {
		return nil, err
	}
	for _, key := range memberships {
		if _, ok := allowed[key]; !ok {
			allowed[key] = "虚拟会员专属分组"
		}
	}
	delete(allowed, "auto")
	return allowed, nil
}
func CapabilityGroups(allowed map[string]string, view model.CapabilityPresentation) ([]CapabilityGroupView, error) {
	var registered []model.CapabilityGroupPresentation
	var abilities []model.Ability
	if err := model.DB.Where("routing_key IS NOT NULL").Find(&registered).Error; err != nil {
		return nil, err
	}
	if err := model.DB.Where(map[string]any{"enabled": true}).Find(&abilities).Error; err != nil {
		return nil, err
	}
	icons := setting.GetGroupIconTypesCopy()
	out := make([]CapabilityGroupView, 0)
	for _, g := range registered {
		key := *g.RoutingKey
		desc, ok := allowed[key]
		if !ok {
			continue
		}
		override := view.Groups[g.GroupUID]
		if override.Hidden {
			continue
		}
		name := key
		if override.Name != nil {
			name = *override.Name
		}
		if override.DescriptionMode == "hidden" {
			desc = ""
		} else if override.DescriptionMode == "custom" {
			desc = override.Description
		}
		var ratio *float64
		if ratio_setting.ContainsGroupRatio(key) {
			r := ratio_setting.GetGroupRatio(key)
			ratio = &r
		}
		models := []string{}
		seen := map[string]bool{}
		for _, a := range abilities {
			if a.Group == key && !seen[a.Model] {
				models = append(models, a.Model)
				seen[a.Model] = true
			}
		}
		sort.Strings(models)
		out = append(out, CapabilityGroupView{GroupUID: g.GroupUID, RoutingKey: key, DisplayName: name, Description: desc, Ratio: ratio, IconType: icons[key], Models: models})
	}
	order := map[string]int{}
	for i, k := range setting.GetGroupOrderCopy() {
		order[k] = i
	}
	custom := map[string]int{}
	for i, k := range view.Order {
		custom[k] = i
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		ai, aok := custom[a.GroupUID]
		bi, bok := custom[b.GroupUID]
		if aok != bok {
			return aok
		}
		if aok && ai != bi {
			return ai < bi
		}
		ai, aok = order[a.RoutingKey]
		bi, bok = order[b.RoutingKey]
		if aok != bok {
			return aok
		}
		if aok && ai != bi {
			return ai < bi
		}
		return a.RoutingKey < b.RoutingKey
	})
	return out, nil
}

func ValidateCapabilityConfig(c model.CapabilityConfig) error {
	if c.IntervalMinutes < 1 || c.IntervalMinutes > 1440 {
		return errors.New("interval must be 1–1440 minutes")
	}
	if _, err := time.LoadLocation(c.Timezone); err != nil {
		return errors.New("invalid timezone")
	}
	if len(c.Weekdays) == 0 || len(c.Weekdays) > 7 {
		return errors.New("select at least one weekday")
	}
	seen := map[int]bool{}
	for _, day := range c.Weekdays {
		if day < 0 || day > 6 || seen[day] {
			return errors.New("invalid weekdays")
		}
		seen[day] = true
	}
	if c.WindowStart < 0 || c.WindowStart > 1439 || c.WindowEnd < 0 || c.WindowEnd > 1439 || (!c.AllDay && c.WindowStart == c.WindowEnd) {
		return errors.New("invalid daily window")
	}
	if c.DailyBudgetMicros < 0 || c.CallReserveMicros < 0 {
		return errors.New("budget cannot be negative")
	}
	if c.DailyBudgetMicros > 1_000_000_000_000 || c.CallReserveMicros > 1_000_000_000 {
		return errors.New("budget exceeds supported bounds")
	}
	models := map[string]bool{}
	for _, p := range c.Models {
		if p.Model == "" || models[p.Model] || p.MaxTokens < 128 || p.MaxTokens > 16384 {
			return errors.New("invalid model profile")
		}
		models[p.Model] = true
		if p.Protocol != "chat" && p.Protocol != "responses" {
			return errors.New("unsupported protocol profile")
		}
	}
	for _, e := range c.Excluded {
		if e.ChannelID < 0 || (e.ChannelID == 0 && e.GroupUID == "" && e.Model == "") {
			return errors.New("exclusion needs an explicit scope")
		}
	}
	return nil
}

func CapabilityScheduleTimes(c model.CapabilityConfig, after time.Time, count int) []int64 {
	if ValidateCapabilityConfig(c) != nil {
		return []int64{}
	}
	loc, _ := time.LoadLocation(c.Timezone)
	step := int64(c.IntervalMinutes) * 60
	n := (after.Unix()-c.Anchor)/step + 1
	if after.Unix() < c.Anchor {
		n = 0
	}
	out := []int64{}
	for candidate, limit := c.Anchor+n*step, after.AddDate(0, 0, 30).Unix(); candidate <= limit && len(out) < count; candidate += step {
		local := time.Unix(candidate, 0).In(loc)
		minute := local.Hour()*60 + local.Minute()
		windowDay := local
		if !c.AllDay {
			if c.WindowStart < c.WindowEnd {
				if minute < c.WindowStart || minute >= c.WindowEnd {
					continue
				}
			} else {
				if minute >= c.WindowEnd && minute < c.WindowStart {
					continue
				}
				if minute < c.WindowEnd {
					windowDay = local.AddDate(0, 0, -1)
				}
			}
		}
		for _, day := range c.Weekdays {
			if int(windowDay.Weekday()) == day {
				out = append(out, candidate)
				break
			}
		}
	}
	return out
}
