package domain

import (
	"strings"
	"testing"
	"time"
)

func TestNewAgent_Valid(t *testing.T) {
	now := time.Now()
	a, err := NewAgent("01234567-89ab-cdef-0123-456789abcdef", "test-user-001", LevelBasic, "pubkey-1", now)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if a == nil {
		t.Fatal("nil agent")
	}
	if a.UUID != "01234567-89ab-cdef-0123-456789abcdef" {
		t.Errorf("UUID = %s", a.UUID)
	}
	if a.State != StateRegistered {
		t.Errorf("State = %s, want registered", a.State)
	}
}

func TestNewAgent_InvalidUUID(t *testing.T) {
	_, err := NewAgent("not-a-uuid", "test-user-001", LevelBasic, "pk", time.Now())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewAgent_InvalidOwner(t *testing.T) {
	_, err := NewAgent("01234567-89ab-cdef-0123-456789abcdef", "", LevelBasic, "pk", time.Now())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewAgent_EmptyPubKey(t *testing.T) {
	_, err := NewAgent("01234567-89ab-cdef-0123-456789abcdef", "test-user-001", LevelBasic, "", time.Now())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewAgent_LevelExceedsMax(t *testing.T) {
	_, err := NewAgent("01234567-89ab-cdef-0123-456789abcdef", "test-user-001", LevelType(99), "pk", time.Now())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "level") {
		t.Errorf("err = %v, want level-related", err)
	}
}
