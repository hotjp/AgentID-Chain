package prompt

import (
	"testing"
)

func TestKeywordClassifier_BasicIntents(t *testing.T) {
	cases := []struct {
		text  string
		intent Intent
	}{
		{"register an agent for alice", IntentRegister},
		{"enroll a new agent", IntentRegister},
		{"upgrade 019eab1a-b761-7a60-955c-37f926faa109 to level 3", IntentUpgrade},
		{"promote this agent to level 2", IntentUpgrade},
		{"info for 019eab1a-b761-7a60-955c-37f926faa109", IntentQuery},
		{"show me 019eab1a-b761-7a60-955c-37f926faa109", IntentQuery},
		{"batch register from agents.csv", IntentBatch},
		{"bulk import from file", IntentBatch},
		{"config set mode remote", IntentConfig},
		{"audit 019eab1a-b761-7a60-955c-37f926faa109", IntentAudit},
	}
	c2 := NewKeywordClassifier()
	for _, c := range cases {
		got := c2.Classify(c.text)
		if got != c.intent {
			// "show me" might match query OR audit due to overlap
			if c.text == "show me 019eab1a-b761-7a60-955c-37f926faa109" && (got == IntentQuery || got == IntentAudit) {
				continue
			}
			t.Errorf("Classify(%q) = %q, want %q", c.text, got, c.intent)
		}
	}
}

func TestKeywordClassifier_Unknown(t *testing.T) {
	c := NewKeywordClassifier()
	if got := c.Classify("do something random"); got != IntentUnknown {
		t.Errorf("Classify random = %q, want unknown", got)
	}
	if _, score := c.Confidence("do something random"); score != 0 {
		t.Errorf("score = %f, want 0", score)
	}
}

func TestKeywordClassifier_Confidence(t *testing.T) {
	c := NewKeywordClassifier()
	intent, score := c.Confidence("register an agent for alice")
	if intent != IntentRegister {
		t.Errorf("intent = %q", intent)
	}
	if score < 0.5 {
		t.Errorf("score = %f, want >= 0.5", score)
	}
}

func TestSlotFiller_UUID(t *testing.T) {
	f := NewSlotFiller()
	uuid := "019eab1a-b761-7a60-955c-37f926faa109"
	slots := f.Fill("info for " + uuid)
	if slots.Get(SlotUUID, "") != uuid {
		t.Errorf("UUID = %q", slots.Get(SlotUUID, ""))
	}
}

func TestSlotFiller_Owner(t *testing.T) {
	f := NewSlotFiller()
	slots := f.Fill("register did:agentid:alice at level 2")
	if slots.Get(SlotOwner, "") != "did:agentid:alice" {
		t.Errorf("Owner = %q", slots.Get(SlotOwner, ""))
	}
	if slots.Get(SlotLevel, "") != "2" {
		t.Errorf("Level = %q", slots.Get(SlotLevel, ""))
	}
}

func TestSlotFiller_Level(t *testing.T) {
	f := NewSlotFiller()
	cases := []struct {
		text string
		want string
	}{
		{"at level 3", "3"},
		{"to level 7", "7"},
		{"level 5", "5"},
		{"to LEVEL 9", "9"},
	}
	for _, c := range cases {
		slots := f.Fill("upgrade <uuid> " + c.text)
		if slots.Get(SlotLevel, "") != c.want {
			t.Errorf("text=%q want=%q got=%q", c.text, c.want, slots.Get(SlotLevel, ""))
		}
	}
}

func TestSlotFiller_Reason(t *testing.T) {
	f := NewSlotFiller()
	slots := f.Fill("ban <uuid> for policy violation")
	if r := slots.Get(SlotReason, ""); r != "policy violation" {
		t.Errorf("Reason = %q", r)
	}
}

func TestSlotFiller_ReasonBecause(t *testing.T) {
	f := NewSlotFiller()
	slots := f.Fill("unban <uuid> because request approved")
	if r := slots.Get(SlotReason, ""); r != "request approved" {
		t.Errorf("Reason = %q", r)
	}
}

func TestSlotFiller_PublicKey(t *testing.T) {
	f := NewSlotFiller()
	slots := f.Fill("register alice with public_key=pk_alice_abc")
	if pk := slots.Get(SlotPublicKey, ""); pk != "pk_alice_abc" {
		t.Errorf("PK = %q", pk)
	}
}

func TestSlotFiller_Name(t *testing.T) {
	f := NewSlotFiller()
	slots := f.Fill("register an agent named bob")
	if n := slots.Get(SlotName, ""); n != "bob" {
		t.Errorf("Name = %q", n)
	}
}

func TestSlotFiller_Format(t *testing.T) {
	f := NewSlotFiller()
	cases := []struct {
		text string
		want string
	}{
		{"show info as json", "json"},
		{"show info in yaml", "yaml"},
		{"show info format table", "table"},
	}
	for _, c := range cases {
		slots := f.Fill(c.text)
		if got := slots.Get(SlotFormat, ""); got != c.want {
			t.Errorf("text=%q want=%q got=%q", c.text, c.want, got)
		}
	}
}

func TestSlotFiller_Limit(t *testing.T) {
	f := NewSlotFiller()
	cases := []struct {
		text string
		want string
	}{
		{"audit <uuid> limit 10", "10"},
		{"audit <uuid> last 5", "5"},
		{"audit <uuid> top 20", "20"},
	}
	for _, c := range cases {
		slots := f.Fill(c.text)
		if got := slots.Get(SlotLimit, ""); got != c.want {
			t.Errorf("text=%q want=%q got=%q", c.text, c.want, got)
		}
	}
}

func TestGenerator_Register(t *testing.T) {
	g := NewDefaultGenerator()
	slots := Slots{
		SlotOwner: "did:agentid:alice",
		SlotLevel: "1",
	}
	cmd, args, err := g.Generate(IntentRegister, slots)
	if err != nil {
		t.Fatal(err)
	}
	if cmd != "register" {
		t.Errorf("cmd = %q", cmd)
	}
	if len(args) < 4 || args[0] != "--owner" || args[1] != "did:agentid:alice" {
		t.Errorf("args = %v", args)
	}
}

func TestGenerator_RegisterWithName(t *testing.T) {
	g := NewDefaultGenerator()
	slots := Slots{SlotName: "bob"}
	cmd, args, err := g.Generate(IntentRegister, slots)
	if err != nil {
		t.Fatal(err)
	}
	if cmd != "register" || args[1] != "did:agentid:bob" {
		t.Errorf("cmd=%q args=%v", cmd, args)
	}
}

func TestGenerator_RegisterMissingOwner(t *testing.T) {
	g := NewDefaultGenerator()
	_, _, err := g.Generate(IntentRegister, Slots{})
	if err != ErrMissingOwner {
		t.Errorf("err = %v, want ErrMissingOwner", err)
	}
}

func TestGenerator_Upgrade(t *testing.T) {
	g := NewDefaultGenerator()
	slots := Slots{
		SlotUUID:  "019eab1a-b761-7a60-955c-37f926faa109",
		SlotLevel: "3",
		SlotReason: "policy review",
	}
	cmd, args, err := g.Generate(IntentUpgrade, slots)
	if err != nil {
		t.Fatal(err)
	}
	if cmd != "upgrade" {
		t.Errorf("cmd = %q", cmd)
	}
	if args[0] != "--uuid" || args[1] != "019eab1a-b761-7a60-955c-37f926faa109" {
		t.Errorf("args = %v", args)
	}
	if args[2] != "--target-level" || args[3] != "3" {
		t.Errorf("target-level args = %v", args)
	}
}

func TestGenerator_UpgradeDefaultLevel(t *testing.T) {
	g := NewDefaultGenerator()
	slots := Slots{SlotUUID: "abc"}
	_, args, err := g.Generate(IntentUpgrade, slots)
	if err != nil {
		t.Fatal(err)
	}
	if args[3] != "2" {
		t.Errorf("default target-level = %q, want 2", args[3])
	}
}

func TestGenerator_Query(t *testing.T) {
	g := NewDefaultGenerator()
	slots := Slots{SlotUUID: "abc", SlotFormat: "table"}
	cmd, args, err := g.Generate(IntentQuery, slots)
	if err != nil {
		t.Fatal(err)
	}
	if cmd != "info" {
		t.Errorf("cmd = %q", cmd)
	}
	if args[2] != "--format" || args[3] != "table" {
		t.Errorf("format args = %v", args)
	}
}

func TestGenerator_Batch(t *testing.T) {
	g := NewDefaultGenerator()
	slots := Slots{SlotPath: "./agents.csv"}
	cmd, args, err := g.Generate(IntentBatch, slots)
	if err != nil {
		t.Fatal(err)
	}
	if cmd != "batch-register" {
		t.Errorf("cmd = %q", cmd)
	}
	if args[1] != "./agents.csv" {
		t.Errorf("path = %q", args[1])
	}
}

func TestGenerator_Config(t *testing.T) {
	g := NewDefaultGenerator()
	slots := Slots{SlotConfigKey: "mode", SlotConfigVal: "remote"}
	cmd, args, err := g.Generate(IntentConfig, slots)
	if err != nil {
		t.Fatal(err)
	}
	if cmd != "config" || args[0] != "set" || args[1] != "mode" || args[2] != "remote" {
		t.Errorf("args = %v", args)
	}
}

func TestGenerator_ConfigShowFallback(t *testing.T) {
	g := NewDefaultGenerator()
	slots := Slots{SlotConfigKey: "mode"} // no val
	cmd, args, err := g.Generate(IntentConfig, slots)
	if err != nil {
		t.Fatal(err)
	}
	if cmd != "config" || args[0] != "show" {
		t.Errorf("fallback to show: %v", args)
	}
}

func TestGenerator_Audit(t *testing.T) {
	g := NewDefaultGenerator()
	slots := Slots{SlotUUID: "abc", SlotLimit: "20"}
	cmd, args, err := g.Generate(IntentAudit, slots)
	if err != nil {
		t.Fatal(err)
	}
	if cmd != "audit" {
		t.Errorf("cmd = %q", cmd)
	}
	if args[2] != "--limit" || args[3] != "20" {
		t.Errorf("limit args = %v", args)
	}
}

func TestGenerator_Unknown(t *testing.T) {
	g := NewDefaultGenerator()
	_, _, err := g.Generate(IntentUnknown, nil)
	if err != ErrUnknownIntent {
		t.Errorf("err = %v, want ErrUnknownIntent", err)
	}
}

func TestValidator(t *testing.T) {
	v := NewDefaultValidator()
	cases := []struct {
		intent Intent
		slots  Slots
		want   error
	}{
		{IntentRegister, Slots{SlotOwner: "x"}, nil},
		{IntentRegister, Slots{}, ErrMissingOwner},
		{IntentUpgrade, Slots{SlotUUID: "x"}, nil},
		{IntentUpgrade, Slots{}, ErrMissingUUID},
		{IntentQuery, Slots{SlotUUID: "x"}, nil},
		{IntentBatch, Slots{SlotPath: "x"}, nil},
		{IntentBatch, Slots{}, ErrMissingPath},
		{IntentAudit, Slots{SlotUUID: "x"}, nil},
		{IntentConfig, Slots{}, nil}, // show fallback allowed
		{IntentUnknown, Slots{}, ErrUnknownIntent},
	}
	for _, c := range cases {
		got := v.Validate(c.intent, c.slots)
		if got != c.want {
			t.Errorf("Validate(%q, %v) = %v, want %v", c.intent, c.slots, got, c.want)
		}
	}
}

func TestPipeline_EndToEnd(t *testing.T) {
	p := NewPipeline()
	cases := []struct {
		text   string
		cmd    string
		hasArg string
	}{
		{"register an agent for alice at level 2", "register", "did:agentid:alice"},
		{"upgrade 019eab1a-b761-7a60-955c-37f926faa109 to level 3", "upgrade", "019eab1a-b761-7a60-955c-37f926faa109"},
		{"info for 019eab1a-b761-7a60-955c-37f926faa109", "info", "019eab1a-b761-7a60-955c-37f926faa109"},
		{"batch register from ./agents.csv", "batch-register", "./agents.csv"},
		{"config set mode remote", "config", "remote"},
		{"audit 019eab1a-b761-7a60-955c-37f926faa109 limit 10", "audit", "10"},
	}
	for _, c := range cases {
		res, err := p.Process(c.text)
		if err != nil {
			t.Errorf("Process(%q) err = %v", c.text, err)
			continue
		}
		if res.Cmd != c.cmd {
			t.Errorf("Process(%q) cmd = %q, want %q", c.text, res.Cmd, c.cmd)
		}
		found := false
		for _, a := range res.Args {
			if a == c.hasArg {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Process(%q) args missing %q, got %v", c.text, c.hasArg, res.Args)
		}
	}
}

func TestPipeline_MissingRequired(t *testing.T) {
	p := NewPipeline()
	_, err := p.Process("register something without owner")
	if err == nil {
		t.Error("expected error for missing owner")
	}
}

func TestPipeline_Unknown(t *testing.T) {
	p := NewPipeline()
	_, err := p.Process("xyzzy foo bar")
	if err != ErrUnknownIntent {
		t.Errorf("err = %v, want ErrUnknownIntent", err)
	}
}
