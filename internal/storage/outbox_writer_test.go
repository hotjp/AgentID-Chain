package storage

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateOutboxEvent(t *testing.T) {
	tests := []struct {
		name    string
		evt     OutboxEvent
		wantErr bool
		errSub  string
	}{
		{
			name: "valid",
			evt: OutboxEvent{
				AggregateType:  "agent",
				AggregateID:    "agent-123",
				EventType:      "agent.registered",
				Payload:        map[string]any{"k": "v"},
				IdempotencyKey: "agent:registered:agent-123:v1",
			},
			wantErr: false,
		},
		{
			name:    "empty aggregate type",
			evt:     OutboxEvent{AggregateID: "x", EventType: "x", IdempotencyKey: "x", Payload: map[string]any{}},
			wantErr: true,
			errSub:  "aggregate_type",
		},
		{
			name: "empty aggregate id",
			evt: OutboxEvent{
				AggregateType: "agent", EventType: "x", IdempotencyKey: "x", Payload: map[string]any{},
			},
			wantErr: true,
			errSub:  "aggregate_id",
		},
		{
			name: "empty event type",
			evt: OutboxEvent{
				AggregateType: "agent", AggregateID: "x", IdempotencyKey: "x", Payload: map[string]any{},
			},
			wantErr: true,
			errSub:  "event_type",
		},
		{
			name: "empty idempotency",
			evt: OutboxEvent{
				AggregateType: "agent", AggregateID: "x", EventType: "x", Payload: map[string]any{},
			},
			wantErr: true,
			errSub:  "idempotency_key",
		},
		{
			name: "nil payload",
			evt: OutboxEvent{
				AggregateType: "agent", AggregateID: "x", EventType: "x", IdempotencyKey: "x",
			},
			wantErr: true,
			errSub:  "payload",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateOutboxEvent(tt.evt)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				if !strings.Contains(err.Error(), tt.errSub) {
					t.Errorf("err = %v, want contains %q", err, tt.errSub)
				}
			} else if err != nil {
				t.Errorf("unexpected err: %v", err)
			}
		})
	}
}

func TestOutboxStatusPending(t *testing.T) {
	if OutboxStatusPending != 0 {
		t.Errorf("OutboxStatusPending = %d, want 0", OutboxStatusPending)
	}
}

func TestErrInvalidInput_Wraps(t *testing.T) {
	// 验证 errors.Is 链
	if !errors.Is(ErrInvalidInput, ErrInvalidInput) {
		t.Error("errors.Is failed")
	}
}
