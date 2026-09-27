package agenthook

import (
	"encoding/json"
	"strings"
	"testing"
)

// These inputs are the examples in each agent's own hooks reference, so that a
// change on their side that breaks the shape shows up as a failing example.
const claudeCode = `{
  "session_id": "abc123",
  "transcript_path": "/Users/me/.claude/projects/x/00893aaf.jsonl",
  "cwd": "/Users/me/src/gumpet",
  "permission_mode": "default",
  "hook_event_name": "PermissionRequest",
  "tool_name": "Bash",
  "tool_input": {
    "command": "rm -rf node_modules",
    "description": "Remove node_modules directory"
  },
  "permission_suggestions": [
    {"type": "addRules", "rules": [{"toolName": "Bash", "ruleContent": "rm -rf node_modules"}], "behavior": "allow", "destination": "localSettings"}
  ]
}`

const codex = `{
  "session_id": "s1",
  "transcript_path": null,
  "cwd": "/home/me/work/api",
  "hook_event_name": "PermissionRequest",
  "model": "gpt-5-codex",
  "permission_mode": "default",
  "turn_id": "t42",
  "tool_name": "Bash",
  "tool_input": {"command": "cargo publish", "description": "Publish the crate"}
}`

func TestParseTellsTheAgentsApart(t *testing.T) {
	cases := []struct {
		name, input, agent, project string
	}{
		{"Claude Code", claudeCode, "Claude Code", "gumpet"},
		{"Codex", codex, "Codex", "api"},
	}
	for _, c := range cases {
		r, err := Parse([]byte(c.input), "")
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if r.Agent != c.agent || r.Project != c.project || r.Tool != "Bash" {
			t.Errorf("%s: parsed %+v, want %s in %s asking for Bash", c.name, r, c.agent, c.project)
		}
	}
	// An agent named on the command line is believed.
	if r, _ := Parse([]byte(claudeCode), "Codex"); r.Agent != "Codex" {
		t.Errorf("agent = %q, want the one given", r.Agent)
	}
}

func TestTitleSaysWhoIsAskingAndFromWhere(t *testing.T) {
	r, _ := Parse([]byte(claudeCode), "")
	if got := r.Title(); got != "Claude Code · gumpet" {
		t.Errorf("title = %q", got)
	}
}

func TestTextIsWhatAPersonDecidesOn(t *testing.T) {
	cases := []struct {
		name  string
		tool  string
		input map[string]any
		want  string
	}{
		{"a command, with why", "Bash", map[string]any{"command": "rm -rf node_modules", "description": "Remove node_modules directory"},
			"rm -rf node_modules\nRemove node_modules directory"},
		{"a command alone", "Bash", map[string]any{"command": "go test ./..."}, "go test ./..."},
		{"an edit", "Edit", map[string]any{"file_path": "/src/main.go", "old_string": "a", "new_string": "b"}, "Edit: /src/main.go"},
		{"a fetch", "WebFetch", map[string]any{"url": "https://example.com", "prompt": "summarise"}, "WebFetch: https://example.com"},
		{"a Codex patch", "apply_patch", map[string]any{"command": "*** Begin Patch\n*** Update File: src/lib.rs\n@@\n-a\n+b\n*** Add File: src/new.rs\n+x\n*** End Patch"},
			"apply_patch\nupdate src/lib.rs\nadd src/new.rs"},
		{"an MCP tool", "mcp__fs__read", map[string]any{"path": "/etc/hosts"}, `mcp__fs__read {"path":"/etc/hosts"}`},
	}
	for _, c := range cases {
		r := Request{Tool: c.tool, Input: c.input}
		if got := r.Text(); got != c.want {
			t.Errorf("%s: text = %q, want %q", c.name, got, c.want)
		}
	}
}

// A heredoc or a big patch is cut, not shown whole: the balloon is for
// deciding.
func TestLongTextIsCut(t *testing.T) {
	r := Request{Tool: "Bash", Input: map[string]any{"command": strings.Repeat("x", 5000)}}
	got := []rune(r.Text())
	if len(got) != maxText || got[len(got)-1] != '…' {
		t.Errorf("text is %d runes ending %q, want %d ending with …", len(got), got[len(got)-1], maxText)
	}
}

func TestParseRefusesWhatIsNotAPermissionRequest(t *testing.T) {
	for _, in := range []string{
		`not json`,
		`{"hook_event_name":"PreToolUse","tool_name":"Bash"}`,
		`{"hook_event_name":"PermissionRequest"}`,
	} {
		if _, err := Parse([]byte(in), ""); err == nil {
			t.Errorf("Parse(%s) succeeded", in)
		}
	}
}

// The decisions are the exact shape both agents document.
func TestDecisionsAreWhatTheAgentsRead(t *testing.T) {
	var allow, deny map[string]map[string]any
	if err := json.Unmarshal(Allow(), &allow); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(Deny("no from the desktop"), &deny); err != nil {
		t.Fatal(err)
	}
	for name, out := range map[string]map[string]map[string]any{"allow": allow, "deny": deny} {
		if out["hookSpecificOutput"]["hookEventName"] != "PermissionRequest" {
			t.Errorf("%s: hookEventName = %v", name, out["hookSpecificOutput"]["hookEventName"])
		}
	}
	a := allow["hookSpecificOutput"]["decision"].(map[string]any)
	d := deny["hookSpecificOutput"]["decision"].(map[string]any)
	if a["behavior"] != "allow" || len(a) != 1 {
		t.Errorf("allow decision = %v, want only behavior: allow", a)
	}
	// Codex fails closed on fields it has reserved, so nothing beyond these
	// two may be sent.
	if d["behavior"] != "deny" || d["message"] != "no from the desktop" || len(d) != 2 {
		t.Errorf("deny decision = %v, want behavior and message only", d)
	}
}
