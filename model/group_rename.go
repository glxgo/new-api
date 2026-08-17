package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/bytedance/gopkg/util/gopool"
	"gorm.io/gorm"
)

func normalizeGroupRenames(value string, renames map[string]string) (map[string]string, error) {
	var nextGroups map[string]float64
	if err := json.Unmarshal([]byte(value), &nextGroups); err != nil {
		return nil, err
	}
	normalized := make(map[string]string, len(renames))
	for oldName, newName := range renames {
		oldName = strings.TrimSpace(oldName)
		newName = strings.TrimSpace(newName)
		if oldName == "" || newName == "" || oldName == newName {
			return nil, errors.New("分组改名映射无效")
		}
		if _, ok := nextGroups[oldName]; ok {
			return nil, fmt.Errorf("原分组 %s 仍存在，无法判定为改名", oldName)
		}
		if _, ok := nextGroups[newName]; !ok {
			return nil, fmt.Errorf("改名后的分组 %s 不在新配置中", newName)
		}
		normalized[oldName] = newName
	}
	return normalized, nil
}

// UpdateGroupRatioWithRenames persists the ratio option and synchronizes every
// API Key reference in one transaction. Explicit mappings keep deletions and
// newly added groups from being guessed as renames.
func UpdateGroupRatioWithRenames(value string, renames map[string]string) error {
	normalized, err := normalizeGroupRenames(value, renames)
	if err != nil {
		return err
	}
	if len(normalized) == 0 {
		return UpdateOption("GroupRatio", value)
	}

	affectedKeys := make(map[string]struct{})
	err = DB.Transaction(func(tx *gorm.DB) error {
		for oldName, newName := range normalized {
			var conflictCount int64
			if tx.Migrator().HasTable(&TokenRouteStep{}) {
				if err := tx.Table("token_route_steps AS old_step").
					Joins("JOIN token_route_steps AS new_step ON new_step.token_id = old_step.token_id AND new_step.group_name = ?", newName).
					Where("old_step.group_name = ?", oldName).
					Count(&conflictCount).Error; err != nil {
					return err
				}
				if conflictCount > 0 {
					return fmt.Errorf("部分 API Key 同时选择了 %s 与 %s，请先合并重复路由", oldName, newName)
				}
			}

			var tokens []Token
			query := tx.Select("id", commonKeyCol).
				Where(map[string]interface{}{"group": oldName}).
				Or(map[string]interface{}{"planned_subscription_group": oldName})
			if tx.Migrator().HasTable(&TokenRouteStep{}) {
				query = query.Or("id IN (?)", tx.Model(&TokenRouteStep{}).Select("token_id").Where("group_name = ?", oldName))
			}
			if err := query.Find(&tokens).Error; err != nil {
				return err
			}
			for _, token := range tokens {
				if token.Key != "" {
					affectedKeys[token.Key] = struct{}{}
				}
			}

			if err := tx.Model(&Token{}).Where(map[string]interface{}{"group": oldName}).Update("group", newName).Error; err != nil {
				return err
			}
			if err := tx.Model(&Token{}).Where("planned_subscription_group = ?", oldName).
				Update("planned_subscription_group", newName).Error; err != nil {
				return err
			}
			if tx.Migrator().HasTable(&TokenRouteStep{}) {
				if err := tx.Model(&TokenRouteStep{}).Where("group_name = ?", oldName).
					Update("group_name", newName).Error; err != nil {
					return err
				}
			}
		}

		option := Option{Key: "GroupRatio"}
		if err := tx.FirstOrCreate(&option, Option{Key: "GroupRatio"}).Error; err != nil {
			return err
		}
		option.Value = value
		return tx.Save(&option).Error
	})
	if err != nil {
		return err
	}
	if err := updateOptionMap("GroupRatio", value); err != nil {
		return err
	}
	if common.RedisEnabled && len(affectedKeys) > 0 {
		gopool.Go(func() {
			for key := range affectedKeys {
				_ = cacheDeleteToken(key)
			}
		})
	}
	return nil
}
