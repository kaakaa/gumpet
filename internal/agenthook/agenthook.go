// Package agenthook speaks the permission hook that Claude Code and Codex
// share.
//
// Both agents run a PermissionRequest hook when they are about to ask whether
// they may use a tool, and both read and write the same JSON: the tool and its
// input come in on stdin, and a decision object goes back on stdout. A hook
// that prints nothing leaves the agent to ask in its own terminal as usual,
// which is what gumpet relies on to never be the only way through.
//
// This package only reads and writes that JSON. Asking the pet, and waiting,
// is gumpetctl's job.
package agenthook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// Request is a permission request, read from the hook's input.
type Request struct {
	// Agent is who is asking: "Claude Code" or "Codex".
	Agent string
	// Project is the working directory's last element, so that two sessions
	// in two repositories can be told apart at a glance.
	Project string
	// Tool is the tool name as the agent gives it: Bash, Edit, apply_patch…
	Tool string
	// Input is the tool's input, as the agent sent it.
	Input map[string]any
}

type payload struct {
	HookEventName string         `json:"hook_event_name"`
	CWD           string         `json:"cwd"`
	ToolName      string         `json:"tool_name"`
	ToolInput     map[string]any `json:"tool_input"`
	// Codex adds these; Claude Code does not. See [Parse].
	TurnID *string `json:"turn_id"`
	Model  *string `json:"model"`
}

// Parse reads a hook's input. agent names who sent it; empty works it out:
// Codex marks its turns with turn_id and names the model, and Claude Code
// does neither.
func Parse(input []byte, agent string) (Request, error) {
	var p payload
	if err := json.Unmarshal(input, &p); err != nil {
		return Request{}, fmt.Errorf("read the hook's input: %w", err)
	}
	if p.HookEventName != "" && p.HookEventName != "PermissionRequest" {
		return Request{}, fmt.Errorf("this is a %s hook, not PermissionRequest", p.HookEventName)
	}
	if p.ToolName == "" {
		return Request{}, fmt.Errorf("the hook's input names no tool")
	}
	if agent == "" {
		agent = "Claude Code"
		if p.TurnID != nil || p.Model != nil {
			agent = "Codex"
		}
	}
	r := Request{Agent: agent, Tool: p.ToolName, Input: p.ToolInput}
	if p.CWD != "" {
		r.Project = filepath.Base(p.CWD)
	}
	return r, nil
}

// maxText caps what the balloon is asked to show. A patch or a heredoc can be
// pages long, and the balloon is for deciding, not reading.
const maxText = 400

// Title is the balloon's heading: who is asking, and from where.
func (r Request) Title() string {
	if r.Project == "" {
		return r.Agent
	}
	return r.Agent + " · " + r.Project
}

// Text is what the balloon says is being asked for, in the terms the agent
// used: the command for a shell, the file for an edit, the files a patch
// touches, and for anything else the tool and its arguments.
func (r Request) Text() string {
	str := func(k string) string {
		s, _ := r.Input[k].(string)
		return strings.TrimSpace(s)
	}
	var text string
	switch {
	case r.Tool == "apply_patch":
		// Codex hands a patch over as its "command". The files are what a
		// person decides on.
		text = patchSummary(str("command"))
	case str("command") != "":
		text = str("command")
		if d := str("description"); d != "" {
			text += "\n" + d
		}
	case str("file_path") != "":
		text = r.Tool + ": " + str("file_path")
	case str("url") != "":
		text = r.Tool + ": " + str("url")
	default:
		args, _ := json.Marshal(r.Input)
		text = r.Tool
		if len(r.Input) > 0 {
			text += " " + string(args)
		}
	}
	return clip(text, maxText)
}

var patchFile = regexp.MustCompile(`(?m)^\*\*\* (Add|Update|Delete) File: (.+)$`)

// patchSummary lists the files an apply_patch patch touches.
func patchSummary(patch string) string {
	var lines []string
	for _, m := range patchFile.FindAllStringSubmatch(patch, -1) {
		lines = append(lines, strings.ToLower(m[1])+" "+strings.TrimSpace(m[2]))
	}
	if len(lines) == 0 {
		return "apply_patch\n" + patch
	}
	return "apply_patch\n" + strings.Join(lines, "\n")
}

// clip cuts s to at most n runes, marking the cut.
func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// Allow is what the hook prints to let the tool run.
func Allow() []byte {
	return decision(map[string]any{"behavior": "allow"})
}

// Deny is what the hook prints to refuse it. message is told to the agent,
// which tells the model, so it knows it was a person saying no and not a
// failure.
func Deny(message string) []byte {
	return decision(map[string]any{"behavior": "deny", "message": message})
}

func decision(d map[string]any) []byte {
	var buf bytes.Buffer
	_ = json.NewEncoder(&buf).Encode(map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName": "PermissionRequest",
			"decision":      d,
		},
	})
	return buf.Bytes()
}
