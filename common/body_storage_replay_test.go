package common

import (
	"errors"
	"io"
	"testing"
)

func TestDiskStorageNewReaderReturnsIndependentReaders(t *testing.T) {
	payload := []byte(`{"model":"gpt-test","input":"disk-replay"}`)
	storage, err := newDiskStorage(payload, "")
	if err != nil {
		t.Fatalf("newDiskStorage() error = %v", err)
	}
	defer storage.Close()

	first, err := storage.NewReader()
	if err != nil {
		t.Fatalf("first NewReader() error = %v", err)
	}
	second, err := storage.NewReader()
	if err != nil {
		t.Fatalf("second NewReader() error = %v", err)
	}
	defer first.Close()
	defer second.Close()

	prefix := make([]byte, 5)
	if _, err = io.ReadFull(first, prefix); err != nil {
		t.Fatalf("partial first read = %v", err)
	}
	secondBytes, err := io.ReadAll(second)
	if err != nil {
		t.Fatalf("second reader read = %v", err)
	}
	if string(secondBytes) != string(payload) {
		t.Fatalf("second reader = %q, want %q", secondBytes, payload)
	}

	if err = storage.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if _, err = storage.NewReader(); !errors.Is(err, ErrStorageClosed) {
		t.Fatalf("NewReader() after Close error = %v, want %v", err, ErrStorageClosed)
	}
}
