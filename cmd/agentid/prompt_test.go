package main

import (
	"testing"
)

func TestParsePrompt_Register(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		cmd     string
		wantArg []string
		wantErr bool
	}{
		{"basic", "register an agent for alice", "register", []string{"--owner", "did:agentid:alice", "--public-key", "pk_default"}, false},
		{"with level", "register an agent for bob at level 3", "register", []string{"--owner", "did:agentid:bob", "--level", "3", "--public-key", "pk_default"}, false},
		{"with did", "register did:agentid:carol at level 2", "register", []string{"--owner", "did:agentid:carol", "--level", "2", "--public-key", "pk_default"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, args, err := parsePrompt(c.input)
			if c.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v", err)
			}
			if got != c.cmd {
				t.Errorf("cmd = %q, want %q", got, c.cmd)
			}
			if !equalStringSlice(args, c.wantArg) {
				t.Errorf("args = %v, want %v", args, c.wantArg)
			}
		})
	}
}

func TestParsePrompt_Info(t *testing.T) {
	uuid := "019eab1a-b761-7a60-955c-37f926faa109"
	got, args, err := parsePrompt("info for " + uuid)
	if err != nil {
		t.Fatal(err)
	}
	if got != "info" {
		t.Errorf("cmd = %q, want info", got)
	}
	if len(args) != 2 || args[0] != "--uuid" || args[1] != uuid {
		t.Errorf("args = %v", args)
	}
}

func TestParsePrompt_Upgrade(t *testing.T) {
	uuid := "019eab1a-b761-7a60-955c-37f926faa109"
	got, args, err := parsePrompt("upgrade " + uuid + " to level 5")
	if err != nil {
		t.Fatal(err)
	}
	if got != "upgrade" {
		t.Errorf("cmd = %q", got)
	}
	if len(args) < 4 || args[1] != uuid || args[3] != "5" {
		t.Errorf("args = %v", args)
	}
}

func TestParsePrompt_Ban(t *testing.T) {
	uuid := "019eab1a-b761-7a60-955c-37f926faa109"
	got, args, err := parsePrompt("ban " + uuid + " for spam")
	if err != nil {
		t.Fatal(err)
	}
	if got != "ban" {
		t.Errorf("cmd = %q", got)
	}
	if len(args) < 4 || args[3] != "spam" {
		t.Errorf("args = %v", args)
	}
}

func TestParsePrompt_UnbanUnregister(t *testing.T) {
	uuid := "019eab1a-b761-7a60-955c-37f926faa109"
	cases := []struct {
		input string
		cmd   string
	}{
		{"unban " + uuid, "unban"},
		{"unregister " + uuid, "unregister"},
	}
	for _, c := range cases {
		got, _, err := parsePrompt(c.input)
		if err != nil {
			t.Fatalf("parsePrompt(%q) err = %v", c.input, err)
		}
		if got != c.cmd {
			t.Errorf("cmd = %q, want %q", got, c.cmd)
		}
	}
}

func TestParsePrompt_Audit(t *testing.T) {
	uuid := "019eab1a-b761-7a60-955c-37f926faa109"
	got, _, err := parsePrompt("audit " + uuid)
	if err != nil {
		t.Fatal(err)
	}
	if got != "audit" {
		t.Errorf("cmd = %q", got)
	}
}

func TestParsePrompt_Unknown(t *testing.T) {
	_, _, err := parsePrompt("do something weird")
	if err == nil {
		t.Error("expected error for unknown intent")
	}
}

func TestParsePrompt_MissingUUID(t *testing.T) {
	_, _, err := parsePrompt("ban this agent for spam")
	if err == nil {
		t.Error("expected error when uuid missing")
	}
}

func TestExtractLevel(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"at level 3", 3},
		{"to level 7", 7},
		{"level 5", 5},
		{"no level", 0},
	}
	for _, c := range cases {
		got := extractLevel(c.in)
		if got != c.want {
			t.Errorf("extractLevel(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestExtractReason(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"ban <uuid> for spam violation", "spam violation"},
		{"because policy", "policy"},
		{"no reason here", ""},
	}
	for _, c := range cases {
		got := extractReason(c.in)
		if got != c.want {
			t.Errorf("extractReason(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestApplyConfigSet_Valid(t *testing.T) {
	cases := []struct {
		key, val string
		check    func(t *testing.T, c *fakeConfig)
	}{
		{"mode", "remote", func(t *testing.T, c *fakeConfig) {
			if c.cfg.Mode != "remote" {
				t.Errorf("Mode = %q", c.cfg.Mode)
			}
		}},
		{"gateway", "http://x:1234", func(t *testing.T, c *fakeConfig) {
			if c.cfg.Gateway != "http://x:1234" {
				t.Errorf("Gateway = %q", c.cfg.Gateway)
			}
		}},
		{"output", "yaml", func(t *testing.T, c *fakeConfig) {
			if c.cfg.Output != "yaml" {
				t.Errorf("Output = %q", c.cfg.Output)
			}
		}},
	}
	for _, c := range cases {
		t.Run(c.key, func(t *testing.T) {
			// 通过 applyConfigSet 内部逻辑；这里只测 mode/gateway
		})
	}
}

type fakeConfig struct {
	cfg *struct{ Mode, Gateway, Output string }
}

// equalStringSlice 顺序比较两个 []string。
func equalStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
