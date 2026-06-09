package fixtures

import (
	"testing"
)

// fixturesPath 指向仓库根的 testdata/fixtures.yaml。
const fixturesPath = "../../../testdata/fixtures.yaml"

func TestLoad(t *testing.T) {
	f := Load(t, fixturesPath)
	if len(f.Users) == 0 {
		t.Error("expected users, got 0")
	}
	if len(f.Agents) == 0 {
		t.Error("expected agents, got 0")
	}
}

func TestUser_Found(t *testing.T) {
	f := Load(t, fixturesPath)
	alice := f.User("did:agentid:alice")
	if alice == nil {
		t.Fatal("alice should be found")
	}
	if alice.Email != "alice@example.com" {
		t.Errorf("email = %q", alice.Email)
	}
	if !alice.KYCPassed {
		t.Error("alice should be KYC passed")
	}
}

func TestUser_NotFound(t *testing.T) {
	f := Load(t, fixturesPath)
	u := f.User("did:agentid:unknown")
	if u != nil {
		t.Error("expected nil for unknown user")
	}
}

func TestAgent_Found(t *testing.T) {
	f := Load(t, fixturesPath)
	a := f.Agent("019eab1a-b761-7a60-955c-37f926faa100")
	if a == nil {
		t.Fatal("agent should be found")
	}
	if a.Level != 1 {
		t.Errorf("level = %d, want 1", a.Level)
	}
	if a.Status != "active" {
		t.Errorf("status = %q, want active", a.Status)
	}
}

func TestAgentByName(t *testing.T) {
	f := Load(t, fixturesPath)
	a := f.AgentByName("bob-helper")
	if a == nil {
		t.Fatal("bob-helper should be found")
	}
	if a.Status != "banned" {
		t.Errorf("status = %q, want banned", a.Status)
	}
}

func TestAuditLog_Found(t *testing.T) {
	f := Load(t, fixturesPath)
	l := f.AuditLog(1)
	if l == nil {
		t.Fatal("audit log 1 should be found")
	}
	if l.Action != "agent.register" {
		t.Errorf("action = %q", l.Action)
	}
}
