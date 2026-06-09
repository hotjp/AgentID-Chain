package domain

import (
	"sort"
	"testing"
)

func TestCanTransitionTo_Valid(t *testing.T) {
	tests := []struct {
		from, to AgentStatus
		want     bool
	}{
		{AgentStatusPending, AgentStatusActive, true},
		{AgentStatusPending, AgentStatusRevoked, true},
		{AgentStatusActive, AgentStatusBanned, true},
		{AgentStatusActive, AgentStatusRevoked, true},
		{AgentStatusBanned, AgentStatusActive, true},
		{AgentStatusBanned, AgentStatusRevoked, true},
	}
	for _, tt := range tests {
		got := CanTransitionTo(tt.from, tt.to)
		if got != tt.want {
			t.Errorf("%s → %s = %v, want %v", tt.from, tt.to, got, tt.want)
		}
	}
}

func TestCanTransitionTo_Invalid(t *testing.T) {
	tests := []struct {
		from, to AgentStatus
	}{
		{AgentStatusPending, AgentStatusPending},  // 自环
		{AgentStatusRevoked, AgentStatusActive},   // 终态不可回
		{AgentStatusRevoked, AgentStatusBanned},   // 终态不可变
		{AgentStatusRevoked, AgentStatusPending},  // 终态不可变
		{AgentStatusActive, AgentStatusPending},   // 不可回退到 pending
	}
	for _, tt := range tests {
		got := CanTransitionTo(tt.from, tt.to)
		if got {
			t.Errorf("%s → %s should be invalid", tt.from, tt.to)
		}
	}
}

func TestCanTransitionTo_UnknownStatus(t *testing.T) {
	if CanTransitionTo(AgentStatus("unknown"), AgentStatusActive) {
		t.Error("unknown → active should be invalid")
	}
	if CanTransitionTo(AgentStatusActive, AgentStatus("unknown")) {
		t.Error("active → unknown should be invalid")
	}
}

func TestAllowedTransitions(t *testing.T) {
	tests := []struct {
		from AgentStatus
		want []AgentStatus
	}{
		{AgentStatusPending, []AgentStatus{AgentStatusActive, AgentStatusRevoked, AgentStatusBanned}},
		{AgentStatusActive, []AgentStatus{AgentStatusBanned, AgentStatusRevoked}},
		{AgentStatusBanned, []AgentStatus{AgentStatusActive, AgentStatusRevoked}},
		{AgentStatusRevoked, []AgentStatus{}},
	}
	for _, tt := range tests {
		got := AllowedTransitions(tt.from)
		sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })
		want := append([]AgentStatus{}, tt.want...)
		sort.Slice(want, func(i, j int) bool { return want[i] < want[j] })
		if len(got) != len(want) {
			t.Errorf("%s: got %v, want %v", tt.from, got, want)
			continue
		}
		for i := range got {
			if got[i] != want[i] {
				t.Errorf("%s[%d]: got %s, want %s", tt.from, i, got[i], want[i])
			}
		}
	}
}

func TestAgentStatus_IsTerminal(t *testing.T) {
	if !AgentStatusRevoked.IsTerminal() {
		t.Error("Revoked should be terminal")
	}
	if AgentStatusActive.IsTerminal() {
		t.Error("Active should not be terminal")
	}
}

func TestAgentStateToStatus(t *testing.T) {
	if StateActive.toStatus() != AgentStatusActive {
		t.Error("StateActive → AgentStatusActive")
	}
	if StateBanned.toStatus() != AgentStatusBanned {
		t.Error("StateBanned → AgentStatusBanned")
	}
	if StateUnregistered.toStatus() != AgentStatusRevoked {
		t.Error("StateUnregistered → AgentStatusRevoked")
	}
	if StateRegistered.toStatus() != AgentStatusPending {
		t.Error("StateRegistered → AgentStatusPending")
	}
}
