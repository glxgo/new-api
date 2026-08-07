package model

import (
	"reflect"
	"regexp"
	"strconv"
	"testing"
)

func TestLogBillingSourceColumnFitsVirtualMembership(t *testing.T) {
	field, ok := reflect.TypeOf(Log{}).FieldByName("BillingSource")
	if !ok {
		t.Fatal("Log.BillingSource field is missing")
	}
	match := regexp.MustCompile(`varchar\((\d+)\)`).FindStringSubmatch(field.Tag.Get("gorm"))
	if len(match) != 2 {
		t.Fatalf("BillingSource gorm tag = %q, want explicit varchar size", field.Tag.Get("gorm"))
	}
	width, err := strconv.Atoi(match[1])
	if err != nil {
		t.Fatalf("parse BillingSource width: %v", err)
	}
	if width < len("virtual_membership") {
		t.Fatalf("BillingSource varchar width = %d, need at least %d", width, len("virtual_membership"))
	}
}
