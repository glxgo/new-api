package model

import "testing"

func TestPaymentSnapshotAfterFee(t *testing.T) {
	snapshot, err := NewPaymentSnapshotFromDisplayAmount("102.50", "CNY")
	if err != nil {
		t.Fatal(err)
	}
	net, err := PaymentSnapshotAfterFee(snapshot, 2.5)
	if err != nil {
		t.Fatal(err)
	}
	if net.AmountMinor != 10000 || net.Currency != "CNY" {
		t.Fatalf("net snapshot = %#v, want 10000 CNY", net)
	}
}
