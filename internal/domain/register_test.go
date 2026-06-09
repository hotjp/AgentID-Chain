package domain

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// stubUUIDGenerator 测试用 UUID 生成器（固定返回）。
func stubUUIDGenerator(uuid UUID) UUIDGenerator {
	return func() (UUID, error) { return uuid, nil }
}

func TestRegisterAgent_HappyPath(t *testing.T) {
	now := time.Now()
	uuid := UUID("01234567-89ab-cdef-0123-456789abcdef")
	gen := stubUUIDGenerator(uuid)
	in := RegisterInput{
		OwnerDID:    "did:agentid:user:owner1",
		Owner:       "test-owner-1",
		Level:       LevelBasic,
		PublicKey:   "pubkey-1",
		OperatorDID: "did:agentid:user:admin",
		Now:         now,
	}
	out, err := RegisterAgent(gen, in)
	if err != nil {
		t.Fatalf("RegisterAgent: %v", err)
	}
	if out.UUID != uuid {
		t.Errorf("UUID = %s, want %s", out.UUID, uuid)
	}
	if out.Agent.Level != LevelBasic {
		t.Errorf("Level = %s, want %s", out.Agent.Level, LevelBasic)
	}
	if out.Event == nil {
		t.Fatal("nil event")
	}
	if out.Event.EventType != EventAgentRegisteredV1 {
		t.Errorf("EventType = %s", out.Event.EventType)
	}
}

func TestRegisterAgent_InvalidLevel(t *testing.T) {
	_, err := RegisterAgent(nil, RegisterInput{
		Owner:       "test-owner",
		Level:       LevelType(99),
		PublicKey:   "pk",
		OperatorDID: "did",
		Now:         time.Now(),
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRegisterAgent_EmptyPubKey(t *testing.T) {
	_, err := RegisterAgent(nil, RegisterInput{
		Owner:       "test-owner",
		Level:       LevelBasic,
		PublicKey:   "",
		OperatorDID: "did",
		Now:         time.Now(),
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRegisterAgent_EmptyOperator(t *testing.T) {
	_, err := RegisterAgent(nil, RegisterInput{
		Owner:       "test-owner",
		Level:       LevelBasic,
		PublicKey:   "pk",
		OperatorDID: "",
		Now:         time.Now(),
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRegisterAgent_ZeroNow(t *testing.T) {
	_, err := RegisterAgent(nil, RegisterInput{
		Owner:       "test-owner",
		Level:       LevelBasic,
		PublicKey:   "pk",
		OperatorDID: "did",
		Now:         time.Time{},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRegisterAgent_GeneratorFails(t *testing.T) {
	gen := func() (UUID, error) { return "", errors.New("gen fail") }
	_, err := RegisterAgent(gen, RegisterInput{
		Owner:       "test-owner",
		Level:       LevelBasic,
		PublicKey:   "pk",
		OperatorDID: "did",
		Now:         time.Now(),
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "gen fail") {
		t.Errorf("err=%v", err)
	}
}

// stubOutboxWriter 收集写入的 envelope（用于 Collect 测试）。
type stubOutboxWriter struct {
	envelopes []OutboxEnvelope
	err       error
}

func (s *stubOutboxWriter) WriteOutboxEvent(_ context.Context, e OutboxEnvelope) error {
	if s.err != nil {
		return s.err
	}
	s.envelopes = append(s.envelopes, e)
	return nil
}

func TestCollectOutbox_Registered(t *testing.T) {
	w := &stubOutboxWriter{}
	now := time.Now()
	uuid := UUID("01234567-89ab-cdef-0123-456789abcdef")
	out, _ := RegisterAgent(stubUUIDGenerator(uuid), RegisterInput{
		OwnerDID:    "did:agentid:user:owner1",
		Owner:       "test-owner",
		Level:       LevelBasic,
		PublicKey:   "pk",
		OperatorDID: "did:agentid:user:admin",
		Now:         now,
	})
	if err := CollectOutbox(context.Background(), w, out); err != nil {
		t.Fatalf("CollectOutbox: %v", err)
	}
	if len(w.envelopes) != 1 {
		t.Fatalf("got %d envelopes, want 1", len(w.envelopes))
	}
	e := w.envelopes[0]
	if e.AggregateType != "agent" {
		t.Errorf("AggregateType = %s", e.AggregateType)
	}
	if e.EventType != EventAgentRegisteredV1 {
		t.Errorf("EventType = %s", e.EventType)
	}
	if e.AggregateID != string(uuid) {
		t.Errorf("AggregateID = %s", e.AggregateID)
	}
}

func TestCollectOutbox_NilWriter(t *testing.T) {
	now := time.Now()
	out, _ := RegisterAgent(stubUUIDGenerator("01234567-89ab-cdef-0123-456789abcdef"), RegisterInput{
		Owner: "test-owner", Level: LevelBasic, PublicKey: "pk", OperatorDID: "did", Now: now,
	})
	if err := CollectOutbox(context.Background(), nil, out); err == nil {
		t.Error("expected error for nil writer")
	}
}

func TestCollectOutbox_NilOutput(t *testing.T) {
	if err := CollectOutbox(context.Background(), &stubOutboxWriter{}, nil); err == nil {
		t.Error("expected error for nil output")
	}
}

func TestFromDomainEvent_UnsupportedType(t *testing.T) {
	_, err := FromDomainEvent("not an event")
	if err == nil {
		t.Error("expected error")
	}
}
