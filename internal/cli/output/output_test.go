package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
)

func TestParseFormat(t *testing.T) {
	cases := []struct {
		in   string
		want Format
	}{
		{"json", FormatJSON},
		{"JSON", FormatJSON},
		{"  yaml ", FormatYAML},
		{"table", FormatTable},
		{"unknown", FormatJSON}, // 默认 JSON
		{"", FormatJSON},
	}
	for _, c := range cases {
		got := ParseFormat(c.in)
		if got != c.want {
			t.Errorf("ParseFormat(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

type sample struct {
	UUID   string `json:"uuid"`
	Owner  string `json:"owner"`
	Level  uint8  `json:"level"`
	State  string `json:"state,omitempty"`
	TxHash string `json:"tx_hash,omitempty"`
}

func TestPrint_JSON(t *testing.T) {
	var buf bytes.Buffer
	s := &sample{UUID: "abc", Owner: "did:agentid:alice", Level: 1, State: "active"}
	if err := Print(&buf, FormatJSON, s); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, `"uuid": "abc"`) {
		t.Errorf("json output missing uuid: %s", out)
	}
	if !strings.Contains(out, `"owner": "did:agentid:alice"`) {
		t.Errorf("json output missing owner: %s", out)
	}
	// 必须能 round-trip
	var back sample
	if err := json.Unmarshal([]byte(out), &back); err != nil {
		t.Errorf("output is not valid JSON: %v\n%s", err, out)
	}
	if back.UUID != "abc" {
		t.Errorf("round-trip UUID = %q", back.UUID)
	}
}

func TestPrint_YAML(t *testing.T) {
	var buf bytes.Buffer
	s := &sample{UUID: "abc", Owner: "did:agentid:alice", Level: 1}
	if err := Print(&buf, FormatYAML, s); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "uuid: abc") {
		t.Errorf("yaml output missing uuid: %s", out)
	}
}

func TestPrint_Table(t *testing.T) {
	var buf bytes.Buffer
	s := &sample{UUID: "abc", Owner: "did:agentid:alice", Level: 1}
	if err := Print(&buf, FormatTable, s); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "abc") || !strings.Contains(out, "did:agentid:alice") {
		t.Errorf("table output missing fields: %s", out)
	}
}

func TestPrintList_Table(t *testing.T) {
	var buf bytes.Buffer
	list := []sample{
		{UUID: "a1", Owner: "did:agentid:alice", Level: 1},
		{UUID: "b1", Owner: "did:agentid:bob", Level: 2},
	}
	if err := PrintList(&buf, FormatTable, list, []string{"uuid", "owner", "level"}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "a1") || !strings.Contains(out, "b1") {
		t.Errorf("table list missing rows: %s", out)
	}
	if !strings.Contains(out, "uuid") || !strings.Contains(out, "owner") || !strings.Contains(out, "level") {
		t.Errorf("table list missing headers: %s", out)
	}
}

func TestPrintList_JSON(t *testing.T) {
	var buf bytes.Buffer
	list := []sample{
		{UUID: "a1", Owner: "did:agentid:alice", Level: 1},
	}
	if err := PrintList(&buf, FormatJSON, list, nil); err != nil {
		t.Fatal(err)
	}
	var back []sample
	if err := json.Unmarshal(buf.Bytes(), &back); err != nil {
		t.Errorf("output is not valid JSON array: %v\n%s", err, buf.String())
	}
	if len(back) != 1 || back[0].UUID != "a1" {
		t.Errorf("round-trip list mismatch: %+v", back)
	}
}

func TestPrintError(t *testing.T) {
	var buf bytes.Buffer
	PrintError(&buf, fmt.Errorf("boom: %s", "test"))
	out := buf.String()
	if !strings.Contains(out, "boom: test") {
		t.Errorf("PrintError output = %q", out)
	}
}

func TestMustPrint(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustPrint should panic on nil writer")
		}
	}()
	var w io.Writer
	MustPrint(w, FormatJSON, &sample{UUID: "x"})
}
