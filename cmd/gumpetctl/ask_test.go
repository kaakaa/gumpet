package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaakaa/gumpet/internal/ask"
	"github.com/kaakaa/gumpet/internal/config"
	"github.com/kaakaa/gumpet/internal/history"
	"github.com/kaakaa/gumpet/internal/message"
	"github.com/kaakaa/gumpet/internal/server"
	"github.com/kaakaa/gumpet/internal/settings"
)

// A Claude Code PermissionRequest, as its hooks reference shows one.
const claudeCodeRequest = `{
  "session_id": "abc123",
  "cwd": "/Users/me/src/gumpet",
  "permission_mode": "default",
  "hook_event_name": "PermissionRequest",
  "tool_name": "Bash",
  "tool_input": {"command": "rm -rf node_modules", "description": "Remove node_modules directory"}
}`

// A Codex PermissionRequest, as its hooks reference shows one.
const codexRequest = `{
  "session_id": "s1", "cwd": "/home/me/work/api", "hook_event_name": "PermissionRequest",
  "model": "gpt-5-codex", "turn_id": "t42", "permission_mode": "default",
  "tool_name": "Bash", "tool_input": {"command": "cargo publish"}
}`

// gumpet runs a real gumpet server on a free port, with a pretend pet that
// hands every question it is shown to pet, and returns the address.
func gumpet(t *testing.T, pet func(b *ask.Broker, q ask.Question)) string {
	t.Helper()
	cfg := config.Default()
	cfg.Server.Addr = "127.0.0.1:0"
	inbox := make(chan message.Message, 4)
	srv := server.New(settings.New(cfg, filepath.Join(t.TempDir(), "config.yaml")),
		history.New(cfg.History), nil, inbox, slog.New(slog.DiscardHandler))
	b := ask.NewBroker()
	srv.SetAsker(b)
	go func() {
		for q := range b.Questions() {
			pet(b, q)
		}
	}()
	ln, err := srv.Listen()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go srv.Serve(ctx, ln)
	return ln.Addr().String()
}

func hook(t *testing.T, addr, input string, extra ...string) (code int, stdout string) {
	t.Helper()
	var out, errs bytes.Buffer
	args := append([]string{"permission", "-addr", addr, "-token", "unused-but-skips-the-config"}, extra...)
	code = runHook(args, strings.NewReader(input), &out, &errs)
	return code, out.String()
}

func decision(t *testing.T, out string) map[string]any {
	t.Helper()
	var d struct {
		HookSpecificOutput struct {
			HookEventName string         `json:"hookEventName"`
			Decision      map[string]any `json:"decision"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(out), &d); err != nil {
		t.Fatalf("output %q is not a decision: %v", out, err)
	}
	if d.HookSpecificOutput.HookEventName != "PermissionRequest" {
		t.Errorf("hookEventName = %q", d.HookSpecificOutput.HookEventName)
	}
	return d.HookSpecificOutput.Decision
}

// The whole way through: the agent's hook input becomes a balloon saying who
// asks and what for, and the button pressed becomes the agent's decision.
func TestThePetsAnswerBecomesTheAgentsDecision(t *testing.T) {
	cases := []struct {
		name, input, title, text string
		press                    int
		want                     string
	}{
		{"Claude Code, allowed", claudeCodeRequest, "Claude Code · gumpet", "rm -rf node_modules\nRemove node_modules directory", 0, "allow"},
		{"Claude Code, denied", claudeCodeRequest, "Claude Code · gumpet", "rm -rf node_modules\nRemove node_modules directory", 1, "deny"},
		{"Codex, allowed", codexRequest, "Codex · api", "cargo publish", 0, "allow"},
	}
	for _, c := range cases {
		shown := make(chan ask.Question, 1)
		addr := gumpet(t, func(b *ask.Broker, q ask.Question) {
			shown <- q
			b.Answer(q.ID, c.press)
		})
		code, out := hook(t, addr, c.input)
		if code != 0 {
			t.Errorf("%s: exit %d", c.name, code)
		}
		d := decision(t, out)
		if d["behavior"] != c.want {
			t.Errorf("%s: behavior = %v, want %s", c.name, d["behavior"], c.want)
		}
		if c.want == "deny" && d["message"] != denyMessage {
			t.Errorf("%s: message = %v, want the agent told a person said no", c.name, d["message"])
		}
		q := <-shown
		if q.Title != c.title || q.Text != c.text || !q.Localize {
			t.Errorf("%s: the pet was shown %q / %q (localize %v), want %q / %q with its own Allow and Deny",
				c.name, q.Title, q.Text, q.Localize, c.title, c.text)
		}
	}
}

// Anything short of a button pressed prints nothing and exits 0, which both
// agents read as "no decision": they ask in the terminal as they always did.
// The pet must never be the only way through.
func TestTheHookStepsAsideWhenThereIsNoAnswer(t *testing.T) {
	quiet := gumpet(t, func(*ask.Broker, ask.Question) {}) // nobody clicks
	declining := gumpet(t, func(b *ask.Broker, q ask.Question) { b.Decline(q.ID) })
	closed, _ := net.Listen("tcp", "127.0.0.1:0")
	gone := closed.Addr().String()
	closed.Close()

	cases := []struct {
		name, addr, input string
		extra             []string
	}{
		{"nobody answers in time", quiet, claudeCodeRequest, []string{"-timeout", "0.2"}},
		{"the pet is keeping quiet", declining, claudeCodeRequest, nil},
		{"gumpet is not running", gone, claudeCodeRequest, nil},
		{"input that is not a request", quiet, `{"hook_event_name":"Stop"}`, nil},
		{"input that is not JSON", quiet, `garbage`, nil},
	}
	for _, c := range cases {
		code, out := hook(t, c.addr, c.input, c.extra...)
		if code != 0 || out != "" {
			t.Errorf("%s: exit %d, printed %q; want exit 0 and nothing", c.name, code, out)
		}
	}
}

func TestAskPrintsTheChoiceOrSaysNobodyAnswered(t *testing.T) {
	answering := gumpet(t, func(b *ask.Broker, q ask.Question) { b.Answer(q.ID, 1) })
	silent := gumpet(t, func(*ask.Broker, ask.Question) {})

	var out, errs bytes.Buffer
	code := runAsk([]string{"-addr", answering, "-token", "x", "-choices", "Deploy, Wait", "main is green. Deploy?"}, &out, &errs)
	if code != 0 || strings.TrimSpace(out.String()) != "Wait" {
		t.Errorf("exit %d, printed %q (%s); want 0 and Wait", code, out.String(), errs.String())
	}

	out.Reset()
	code = runAsk([]string{"-addr", silent, "-token", "x", "-timeout", "0.2", "anyone?"}, &out, &errs)
	if code != exitNoAnswer || out.Len() != 0 {
		t.Errorf("exit %d, printed %q; want %d and nothing", code, out.String(), exitNoAnswer)
	}
}
