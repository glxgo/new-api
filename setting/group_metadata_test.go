package setting

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
)

func TestGroupMetadataNormalizesLegacyNull(t *testing.T) {
	originalOrder := GetGroupOrderCopy()
	originalIconTypes := GetGroupIconTypesCopy()
	t.Cleanup(func() {
		orderJSON, _ := common.Marshal(originalOrder)
		iconTypesJSON, _ := common.Marshal(originalIconTypes)
		_ = UpdateGroupOrderByJSONString(string(orderJSON))
		_ = UpdateGroupIconTypesByJSONString(string(iconTypesJSON))
	})

	if err := UpdateGroupOrderByJSONString("null"); err != nil {
		t.Fatalf("legacy null group order should be accepted during loading: %v", err)
	}
	if got := GroupOrder2JSONString(); got != "[]" {
		t.Fatalf("expected canonical empty group order, got %s", got)
	}
	if _, err := NormalizeGroupOrderJSONString("null"); err != nil {
		t.Fatalf("legacy null group order should normalize: %v", err)
	}
	if err := ValidateGroupOrder("null"); err == nil {
		t.Fatal("strict validation must reject null group order")
	}

	if err := UpdateGroupIconTypesByJSONString("null"); err != nil {
		t.Fatalf("legacy null icon metadata should be accepted during loading: %v", err)
	}
	if got := GroupIconTypes2JSONString(); got != "{}" {
		t.Fatalf("expected canonical empty icon metadata, got %s", got)
	}
	if _, err := NormalizeGroupIconTypesJSONString("null"); err != nil {
		t.Fatalf("legacy null icon metadata should normalize: %v", err)
	}
	if err := ValidateGroupIconTypes("null"); err == nil {
		t.Fatal("strict validation must reject null icon metadata")
	}
}

func TestGroupMetadataValidationRejectsInvalidValues(t *testing.T) {
	if err := ValidateGroupOrder(`["default","default"]`); err == nil {
		t.Fatal("duplicate group names must be rejected")
	}
	if err := ValidateGroupIconTypes(`{"default":-1}`); err == nil {
		t.Fatal("negative icon types must be rejected")
	}
	if err := ValidateGroupIconTypes(`{"default":"1"}`); err == nil {
		t.Fatal("non-integer icon types must be rejected")
	}
}
