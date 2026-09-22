package main

import (
	"errors"
	"io"
	"os"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"gorm.io/gorm"
)

type topologySnapshot struct {
	SchemaVersion int                      `json:"schema_version"`
	CapturedAt    time.Time                `json:"captured_at"`
	Channels      []service.PelicanChannel `json:"channels"`
	Abilities     []struct {
		ChannelID int    `json:"channel_id"`
		Group     string `json:"group"`
		Model     string `json:"model"`
		Enabled   bool   `json:"enabled"`
	} `json:"abilities"`
	Options map[string]string `json:"options"`
}

type confirmedMappings struct {
	SourceID string `json:"source_id"`
	Bindings []struct {
		ProviderID string `json:"provider_id"`
		Model      string `json:"model"`
		ChannelID  int    `json:"channel_id"`
	} `json:"bindings"`
}

func readPreviewJSON(path string, value any) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 32<<20 {
		return errors.New("invalid local snapshot file")
	}
	raw, err := io.ReadAll(io.LimitReader(f, (32<<20)+1))
	if err != nil {
		return err
	}
	return common.Unmarshal(raw, value)
}

// Only called from the isolated SQLite preview binary. Never loads credentials,
// endpoints, users or subscriptions from production, nor writes to that server.
func loadPreviewTopology(db *gorm.DB, path string) (time.Time, error) {
	var s topologySnapshot
	if err := readPreviewJSON(path, &s); err != nil {
		return s.CapturedAt, err
	}
	if s.SchemaVersion != 1 || s.CapturedAt.IsZero() || time.Since(s.CapturedAt) > 24*time.Hour || time.Until(s.CapturedAt) > 5*time.Minute || len(s.Channels) == 0 {
		return s.CapturedAt, errors.New("invalid or stale topology snapshot")
	}
	if s.Options["GroupRatio"] == "" || s.Options["UserUsableGroups"] == "" {
		return s.CapturedAt, errors.New("missing group configuration")
	}
	channels := map[int]bool{}
	for _, c := range s.Channels {
		if c.ID <= 0 || channels[c.ID] {
			return s.CapturedAt, errors.New("invalid or duplicate channel identity")
		}
		channels[c.ID] = true
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		for _, c := range s.Channels {
			if err := tx.Create(&model.Channel{Id: c.ID, Name: c.Name, Status: c.Status, Group: c.Group, Models: c.Models, Key: "local-placeholder-not-a-credential"}).Error; err != nil {
				return err
			}
		}
		for _, a := range s.Abilities {
			if !channels[a.ChannelID] {
				return errors.New("ability references missing channel")
			}
			if err := tx.Create(&model.Ability{ChannelId: a.ChannelID, Group: a.Group, Model: a.Model, Enabled: a.Enabled}).Error; err != nil {
				return err
			}
		}
		for _, key := range []string{"GroupRatio", "UserUsableGroups", "GroupOrder", "GroupIconTypes", "group_ratio_setting.group_special_usable_group"} {
			if value, ok := s.Options[key]; ok {
				if err := tx.Create(&model.Option{Key: key, Value: value}).Error; err != nil {
					return err
				}
			}
		}
		return nil
	}); err != nil {
		return s.CapturedAt, err
	}
	for _, option := range []struct {
		key      string
		fallback string
		apply    func(string) error
	}{
		{"GroupRatio", "", ratio_setting.UpdateGroupRatioByJSONString},
		{"UserUsableGroups", "", setting.UpdateUserUsableGroupsByJSONString},
		{"GroupOrder", "[]", setting.UpdateGroupOrderByJSONString},
		{"GroupIconTypes", "{}", setting.UpdateGroupIconTypesByJSONString},
	} {
		value := s.Options[option.key]
		if value == "" {
			value = option.fallback
		}
		if err := option.apply(value); err != nil {
			return s.CapturedAt, err
		}
	}
	if raw := s.Options["group_ratio_setting.group_special_usable_group"]; raw != "" {
		if err := config.UpdateConfigFromMap(ratio_setting.GetGroupRatioSetting(), map[string]string{"group_special_usable_group": raw}); err != nil {
			return s.CapturedAt, err
		}
	}
	return s.CapturedAt, nil
}

func applyPreviewMappings(db *gorm.DB, path, source string) (int, error) {
	var m confirmedMappings
	if err := readPreviewJSON(path, &m); err != nil {
		return 0, err
	}
	if m.SourceID != source || len(m.Bindings) == 0 {
		return 0, errors.New("mapping source mismatch or empty bindings")
	}
	count := 0
	err := db.Transaction(func(tx *gorm.DB) error {
		seen := map[string]bool{}
		for _, b := range m.Bindings {
			id := model.PelicanTargetID(source, b.ProviderID, b.Model)
			if strings.TrimSpace(b.ProviderID) == "" || b.ChannelID <= 0 || seen[id] {
				return errors.New("invalid or duplicate mapping identity")
			}
			seen[id] = true
			var target model.PelicanTarget
			if err := tx.First(&target, "id = ? AND present = ?", id, true).Error; err != nil {
				return errors.New("confirmed target not found in current archive")
			}
			if err := model.SavePelicanMapping(tx, id, b.ChannelID, b.Model, false); err != nil {
				return err
			}
			count++
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return count, nil
}
