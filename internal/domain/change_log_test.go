package domain

import (
	"strings"
	"testing"
	"time"
)

func TestNewChangeLog_Valid(t *testing.T) {
	now := time.Now()
	cl, err := NewChangeLog(
		"01234567-89ab-cdef-0123-456789abcdef",
		"01234567-89ab-cdef-0123-456789abcdee",
		"register", "", "{}", "did:agentid:user:admin", "new agent", now,
	)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if cl.Action != "register" {
		t.Errorf("Action=%s", cl.Action)
	}
}

func TestNewChangeLog_BadAction(t *testing.T) {
	now := time.Now()
	_, err := NewChangeLog(
		"01234567-89ab-cdef-0123-456789abcdef",
		"01234567-89ab-cdef-0123-456789abcdee",
		"invalid-action", "old", "new", "did", "", now,
	)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "invalid action") {
		t.Errorf("err=%v", err)
	}
}

func TestNewChangeLog_EmptyOperator(t *testing.T) {
	now := time.Now()
	_, err := NewChangeLog(
		"01234567-89ab-cdef-0123-456789abcdef",
		"01234567-89ab-cdef-0123-456789abcdee",
		"upgrade", "0", "1", "", "", now,
	)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewChangeLog_EmptyAgentUUID(t *testing.T) {
	now := time.Now()
	_, err := NewChangeLog(
		"01234567-89ab-cdef-0123-456789abcdef",
		"",
		"upgrade", "0", "1", "did", "", now,
	)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestIsValidAction(t *testing.T) {
	valid := []string{"register", "upgrade", "ban", "unban", "unregister", "grant", "revoke"}
	for _, a := range valid {
		if !IsValidAction(a) {
			t.Errorf("%s should be valid", a)
		}
	}
	if IsValidAction("delete") {
		t.Error("delete should be invalid")
	}
}

func TestActions_ConsistentWithValid(t *testing.T) {
	if len(Actions()) != len(validActions) {
		t.Errorf("Actions()=%d, validActions=%d", len(Actions()), len(validActions))
	}
}
