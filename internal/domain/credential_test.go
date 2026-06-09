package domain

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"testing"
	"time"
)

func newTestSignature(t *testing.T) string {
	t.Helper()
	b := make([]byte, 64)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}

func TestNewAgentCredential_Valid(t *testing.T) {
	sig := newTestSignature(t)
	now := time.Now()
	c, err := NewAgentCredential(
		"01234567-89ab-cdef-0123-456789abcdef",
		"01234567-89ab-cdef-0123-456789abcdee",
		sig,
		"did:agentid:issuer:test",
		now,
		now.Add(time.Hour),
	)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if c == nil {
		t.Fatal("nil cred")
	}
}

func TestNewAgentCredential_EmptyUUID(t *testing.T) {
	sig := newTestSignature(t)
	now := time.Now()
	_, err := NewAgentCredential("", "01234567-89ab-cdef-0123-456789abcdee", sig, "did", now, now.Add(time.Hour))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewAgentCredential_BadSignature(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name string
		sig  string
	}{
		{"empty", ""},
		{"not hex", "zzzz"},
		{"too short", "deadbeef"},
		{"too long", strings.Repeat("ab", 100)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewAgentCredential(
				"01234567-89ab-cdef-0123-456789abcdef",
				"01234567-89ab-cdef-0123-456789abcdee",
				tt.sig,
				"did",
				now,
				now.Add(time.Hour),
			)
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestNewAgentCredential_ExpiresBeforeIssued(t *testing.T) {
	sig := newTestSignature(t)
	now := time.Now()
	_, err := NewAgentCredential(
		"01234567-89ab-cdef-0123-456789abcdef",
		"01234567-89ab-cdef-0123-456789abcdee",
		sig,
		"did",
		now,
		now.Add(-time.Hour), // 过期时间在 issued 之前
	)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCredential_IsValid(t *testing.T) {
	sig := newTestSignature(t)
	now := time.Now()
	c, _ := NewAgentCredential(
		"01234567-89ab-cdef-0123-456789abcdef",
		"01234567-89ab-cdef-0123-456789abcdee",
		sig, "did", now, now.Add(time.Hour),
	)
	if !c.IsValid(now) {
		t.Error("should be valid at issued time")
	}
	if !c.IsValid(now.Add(30 * time.Minute)) {
		t.Error("should be valid mid-window")
	}
	if c.IsValid(now.Add(2 * time.Hour)) {
		t.Error("should be invalid after expiry")
	}
}

func TestCredential_IsExpired_Effective(t *testing.T) {
	sig := newTestSignature(t)
	now := time.Now()
	c, _ := NewAgentCredential(
		"01234567-89ab-cdef-0123-456789abcdef",
		"01234567-89ab-cdef-0123-456789abcdee",
		sig, "did", now, now.Add(time.Hour),
	)
	if c.IsExpired(now) {
		t.Error("not yet expired at now")
	}
	if !c.IsEffective(now) {
		t.Error("should be effective at issued time")
	}
}

func TestCredential_RemainingLifetime(t *testing.T) {
	sig := newTestSignature(t)
	now := time.Now()
	c, _ := NewAgentCredential(
		"01234567-89ab-cdef-0123-456789abcdef",
		"01234567-89ab-cdef-0123-456789abcdee",
		sig, "did", now, now.Add(time.Hour),
	)
	if d := c.RemainingLifetime(now); d != time.Hour {
		t.Errorf("remaining=%v, want 1h", d)
	}
	if d := c.RemainingLifetime(now.Add(30 * time.Minute)); d != 30*time.Minute {
		t.Errorf("remaining=%v, want 30m", d)
	}
	if d := c.RemainingLifetime(now.Add(2 * time.Hour)); d >= 0 {
		t.Errorf("remaining=%v, want negative", d)
	}
}

func TestCredential_IssueNew(t *testing.T) {
	sig := newTestSignature(t)
	sig2 := newTestSignature(t)
	now := time.Now()
	c1, _ := NewAgentCredential(
		"01234567-89ab-cdef-0123-456789abcdef",
		"01234567-89ab-cdef-0123-456789abcdee",
		sig, "did:agentid:issuer:old", now, now.Add(time.Hour),
	)
	c2, err := c1.IssueNew(
		"01234567-89ab-cdef-0123-456789abcdff",
		sig2,
		"did:agentid:issuer:new",
		now.Add(time.Minute),
		now.Add(2*time.Hour),
	)
	if err != nil {
		t.Fatalf("IssueNew: %v", err)
	}
	if c2.AgentUUID != c1.AgentUUID {
		t.Error("agent_uuid should be preserved")
	}
	if c2.Signature == c1.Signature {
		t.Error("signature should change")
	}
	if c2.UUID == c1.UUID {
		t.Error("uuid should change")
	}
}
