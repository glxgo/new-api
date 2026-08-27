package setting

import (
	"fmt"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

// GroupOrder and GroupIconTypes are presentation metadata only. They do not
// participate in channel routing, model availability, or billing.
var (
	groupOrder      = []string{}
	groupIconTypes  = map[string]int{}
	groupMetadataMu sync.RWMutex
)

func GroupOrder2JSONString() string {
	groupMetadataMu.RLock()
	defer groupMetadataMu.RUnlock()
	if groupOrder == nil {
		return "[]"
	}
	value, err := common.Marshal(groupOrder)
	if err != nil {
		common.SysLog("error marshalling group order: " + err.Error())
		return "[]"
	}
	return string(value)
}

func GetGroupOrderCopy() []string {
	groupMetadataMu.RLock()
	defer groupMetadataMu.RUnlock()
	return append([]string{}, groupOrder...)
}

func UpdateGroupOrderByJSONString(jsonStr string) error {
	parsed, err := parseGroupOrderJSONString(jsonStr, true)
	if err != nil {
		return err
	}
	groupMetadataMu.Lock()
	groupOrder = append([]string(nil), parsed...)
	groupMetadataMu.Unlock()
	return nil
}

func ValidateGroupOrder(jsonStr string) error {
	_, err := parseGroupOrderJSONString(jsonStr, false)
	return err
}

// NormalizeGroupOrderJSONString converts the legacy null/empty representation
// to the canonical JSON array used by the runtime and API responses.
func NormalizeGroupOrderJSONString(jsonStr string) (string, error) {
	parsed, err := parseGroupOrderJSONString(jsonStr, true)
	if err != nil {
		return "", err
	}
	value, err := common.Marshal(parsed)
	if err != nil {
		return "", err
	}
	return string(value), nil
}

func parseGroupOrderJSONString(jsonStr string, allowLegacyNull bool) ([]string, error) {
	trimmed := strings.TrimSpace(jsonStr)
	if allowLegacyNull && (trimmed == "" || trimmed == "null") {
		return []string{}, nil
	}

	var parsed []string
	if err := common.UnmarshalJsonStr(trimmed, &parsed); err != nil {
		return nil, err
	}
	if parsed == nil {
		if allowLegacyNull {
			return []string{}, nil
		}
		return nil, fmt.Errorf("group order must be a JSON array")
	}
	if err := validateGroupOrder(parsed); err != nil {
		return nil, err
	}
	return parsed, nil
}

func validateGroupOrder(parsed []string) error {
	seen := make(map[string]struct{}, len(parsed))
	for _, name := range parsed {
		if name == "" {
			return fmt.Errorf("group order contains an empty group name")
		}
		if _, ok := seen[name]; ok {
			return fmt.Errorf("group order contains duplicate group: %s", name)
		}
		seen[name] = struct{}{}
	}
	return nil
}

func GroupIconTypes2JSONString() string {
	groupMetadataMu.RLock()
	defer groupMetadataMu.RUnlock()
	if groupIconTypes == nil {
		return "{}"
	}
	value, err := common.Marshal(groupIconTypes)
	if err != nil {
		common.SysLog("error marshalling group icon types: " + err.Error())
		return "{}"
	}
	return string(value)
}

func GetGroupIconTypesCopy() map[string]int {
	groupMetadataMu.RLock()
	defer groupMetadataMu.RUnlock()
	copyValue := make(map[string]int, len(groupIconTypes))
	for name, channelType := range groupIconTypes {
		copyValue[name] = channelType
	}
	return copyValue
}

func UpdateGroupIconTypesByJSONString(jsonStr string) error {
	parsed, err := parseGroupIconTypesJSONString(jsonStr, true)
	if err != nil {
		return err
	}
	groupMetadataMu.Lock()
	groupIconTypes = parsed
	groupMetadataMu.Unlock()
	return nil
}

func ValidateGroupIconTypes(jsonStr string) error {
	_, err := parseGroupIconTypesJSONString(jsonStr, false)
	return err
}

// NormalizeGroupIconTypesJSONString converts the legacy null/empty
// representation to the canonical JSON object used by the runtime and API.
func NormalizeGroupIconTypesJSONString(jsonStr string) (string, error) {
	parsed, err := parseGroupIconTypesJSONString(jsonStr, true)
	if err != nil {
		return "", err
	}
	value, err := common.Marshal(parsed)
	if err != nil {
		return "", err
	}
	return string(value), nil
}

func parseGroupIconTypesJSONString(jsonStr string, allowLegacyNull bool) (map[string]int, error) {
	trimmed := strings.TrimSpace(jsonStr)
	if allowLegacyNull && (trimmed == "" || trimmed == "null") {
		return map[string]int{}, nil
	}

	var parsed map[string]int
	if err := common.UnmarshalJsonStr(trimmed, &parsed); err != nil {
		return nil, err
	}
	if parsed == nil {
		if allowLegacyNull {
			return map[string]int{}, nil
		}
		return nil, fmt.Errorf("group icon types must be a JSON object")
	}
	if err := validateGroupIconTypes(parsed); err != nil {
		return nil, err
	}
	return parsed, nil
}

func validateGroupIconTypes(parsed map[string]int) error {
	if parsed == nil {
		return fmt.Errorf("group icon types must be a JSON object")
	}
	for name, channelType := range parsed {
		if name == "" {
			return fmt.Errorf("group icon types contains an empty group name")
		}
		if channelType < 0 {
			return fmt.Errorf("group icon type must not be negative: %s", name)
		}
	}
	return nil
}
