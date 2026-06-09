// Package fixtures 提供测试夹具加载器。
//
// 用法：
//
//	func TestSomething(t *testing.T) {
//	    f := fixtures.Load(t, "testdata/fixtures.yaml")
//	    aliceUser := f.User("did:agentid:alice")
//	    aliceAgent := f.Agent("019eab1a-b761-7a60-955c-37f926faa100")
//	}
package fixtures

import (
	"os"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// Fixtures 全部测试夹具的根结构。
type Fixtures struct {
	Users       []UserFixture       `yaml:"users"`
	Agents      []AgentFixture      `yaml:"agents"`
	AuditLogs   []AuditLogFixture   `yaml:"audit_logs"`
	OutboxEvents []OutboxEventFixture `yaml:"outbox_events"`
}

// UserFixture User 夹具。
type UserFixture struct {
	ID          string    `yaml:"id"`
	DID         string    `yaml:"did"`
	Email       string    `yaml:"email"`
	DisplayName string    `yaml:"display_name"`
	KYCPassed   bool      `yaml:"kyc_passed"`
	CreatedAt   time.Time `yaml:"created_at"`
}

// AgentFixture Agent 夹具。
type AgentFixture struct {
	ID          string    `yaml:"id"`
	OwnerDID    string    `yaml:"owner_did"`
	Name        string    `yaml:"name"`
	Level       uint8     `yaml:"level"`
	Permissions uint64    `yaml:"permissions"`
	PublicKey   string    `yaml:"public_key"`
	Status      string    `yaml:"status"` // active|banned|unregistered
	BannedAt    time.Time `yaml:"banned_at,omitempty"`
	BanReason   string    `yaml:"ban_reason,omitempty"`
	CreatedAt   time.Time `yaml:"created_at"`
	UpdatedAt   time.Time `yaml:"updated_at"`
}

// AuditLogFixture AuditLog 夹具。
type AuditLogFixture struct {
	ID        uint64                 `yaml:"id"`
	Actor     string                 `yaml:"actor"`
	Action    string                 `yaml:"action"`
	Target    string                 `yaml:"target"`
	Metadata  map[string]interface{} `yaml:"metadata"`
	CreatedAt time.Time              `yaml:"created_at"`
}

// OutboxEventFixture OutboxEvent 夹具。
type OutboxEventFixture struct {
	ID          string                 `yaml:"id"`
	Aggregate   string                 `yaml:"aggregate"`
	AggregateID string                 `yaml:"aggregate_id"`
	EventType   string                 `yaml:"event_type"`
	Payload     map[string]interface{} `yaml:"payload"`
	Status      string                 `yaml:"status"`
	CreatedAt   time.Time              `yaml:"created_at"`
}

// Load 从 path 加载 fixtures。
// t.Helper() 自动应用。
func Load(t *testing.T, path string) *Fixtures {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("testdata: read %s: %v", path, err)
	}
	var f Fixtures
	if err := yaml.Unmarshal(data, &f); err != nil {
		t.Fatalf("testdata: unmarshal %s: %v", path, err)
	}
	return &f
}

// User 通过 DID 查找 User 夹具。
// 找不到时 t.Fatal。
func (f *Fixtures) User(did string) *UserFixture {
	for i := range f.Users {
		if f.Users[i].DID == did {
			return &f.Users[i]
		}
	}
	return nil
}

// Agent 通过 ID 查找 Agent 夹具。
func (f *Fixtures) Agent(id string) *AgentFixture {
	for i := range f.Agents {
		if f.Agents[i].ID == id {
			return &f.Agents[i]
		}
	}
	return nil
}

// AgentByName 通过 name 查找 Agent 夹具。
func (f *Fixtures) AgentByName(name string) *AgentFixture {
	for i := range f.Agents {
		if f.Agents[i].Name == name {
			return &f.Agents[i]
		}
	}
	return nil
}

// AuditLog 通过 ID 查找 AuditLog 夹具。
func (f *Fixtures) AuditLog(id uint64) *AuditLogFixture {
	for i := range f.AuditLogs {
		if f.AuditLogs[i].ID == id {
			return &f.AuditLogs[i]
		}
	}
	return nil
}
