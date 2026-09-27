package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/kaakaa/gumpet/internal/agenthook"
	"github.com/kaakaa/gumpet/internal/config"
)

// exitNoAnswer is how `gumpetctl ask` says nobody answered, so a script can
// tell that from a choice (0) and from gumpet not being there at all (1).
const exitNoAnswer = 3

// denyMessage is what the agent is told when its request is refused from the
// pet. It is read by the model, so it says plainly that a person decided.
const denyMessage = "The user denied this from gumpet, their desktop pet."

// target is where a running gumpet listens, from the config unless overridden.
type target struct{ addr, token string }

func targetFlags(fs *flag.FlagSet) func() (target, error) {
	defaultPath, _ := config.DefaultPath()
	configPath := fs.String("config", defaultPath, "config file to read the address and token from")
	addr := fs.String("addr", "", "gumpet address, overriding the config file")
	token := fs.String("token", "", "auth token, overriding the config file")
	return func() (target, error) {
		t := target{addr: *addr, token: *token}
		if t.addr != "" && t.token != "" {
			return t, nil
		}
		cfg, err := config.Load(*configPath)
		if err != nil {
			return t, err
		}
		if t.addr == "" {
			t.addr = cfg.Server.Addr
		}
		if t.token == "" {
			t.token = cfg.Server.Token
		}
		return t, nil
	}
}

type question struct {
	Text       string   `json:"text"`
	Title      string   `json:"title,omitempty"`
	Level      string   `json:"level,omitempty"`
	Choices    []string `json:"choices,omitempty"`
	TimeoutSec float64  `json:"timeout_sec"`
}

type reply struct {
	Answered bool   `json:"answered"`
	Index    int    `json:"index"`
	Choice   string `json:"choice"`
}

// errNoAnswer is a question that ended without a choice.
var errNoAnswer = errors.New("no answer")

// put asks gumpet q and waits for the answer. gumpet enforces the timeout; the
// client's own is a little longer, only so a gumpet that has hung does not
// hang the asker too.
func put(t target, q question) (reply, error) {
	body, err := json.Marshal(q)
	if err != nil {
		return reply{}, err
	}
	req, err := http.NewRequest(http.MethodPost, "http://"+t.addr+"/api/v1/ask", bytes.NewReader(body))
	if err != nil {
		return reply{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if t.token != "" {
		req.Header.Set("X-Gumpet-Token", t.token)
	}
	client := &http.Client{Timeout: time.Duration(q.TimeoutSec*float64(time.Second)) + 10*time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return reply{}, fmt.Errorf("ask %s: %w (is gumpet running?)", t.addr, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return reply{}, fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(msg)))
	}
	var r reply
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return reply{}, fmt.Errorf("read the answer: %w", err)
	}
	if !r.Answered {
		return r, errNoAnswer
	}
	return r, nil
}

// runAsk is `gumpetctl ask`: put a question on the pet, print the choice.
func runAsk(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("ask", flag.ContinueOnError)
	fs.SetOutput(stderr)
	where := targetFlags(fs)
	title := fs.String("title", "", "heading shown above the question")
	level := fs.String("level", "", "info, success, warn or error (default warn)")
	choices := fs.String("choices", "", "comma-separated answers, first is the default-looking one (default Allow,Deny)")
	timeout := fs.Float64("timeout", 60, "seconds to wait for an answer")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "usage: gumpetctl ask [flags] <question>\n\n"+
			"Prints the choice made and exits 0; exits %d if nobody answered.\n\n", exitNoAnswer)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	text, err := readText(fs.Args())
	if err != nil || strings.TrimSpace(text) == "" {
		fs.Usage()
		return 2
	}
	t, err := where()
	if err != nil {
		fmt.Fprintf(stderr, "gumpetctl: %v\n", err)
		return 1
	}
	q := question{Text: text, Title: *title, Level: *level, TimeoutSec: *timeout}
	for _, c := range strings.Split(*choices, ",") {
		if c = strings.TrimSpace(c); c != "" {
			q.Choices = append(q.Choices, c)
		}
	}
	r, err := put(t, q)
	switch {
	case errors.Is(err, errNoAnswer):
		return exitNoAnswer
	case err != nil:
		fmt.Fprintf(stderr, "gumpetctl: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, r.Choice)
	return 0
}

// runHook is `gumpetctl hook permission`: the PermissionRequest hook that
// Claude Code and Codex both run.
//
// It never stands in the way. Anything short of a button pressed — no answer
// in time, gumpet not running, input it cannot read — prints nothing and exits
// 0, which both agents take as "no decision" and ask in their own terminal as
// usual. So the pet is a shortcut and never the only way through.
func runHook(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "permission" {
		fmt.Fprintln(stderr, "usage: gumpetctl hook permission [flags]  (reads the hook's JSON on stdin)")
		return 2
	}
	fs := flag.NewFlagSet("hook permission", flag.ContinueOnError)
	fs.SetOutput(stderr)
	where := targetFlags(fs)
	agent := fs.String("agent", "", `who is asking: "claude" or "codex" (default: tell from the input)`)
	timeout := fs.Float64("timeout", 60, "seconds to wait for an answer; keep it under the hook's own timeout")
	if err := fs.Parse(args[1:]); err != nil {
		return 0
	}
	note := func(format string, a ...any) { fmt.Fprintf(stderr, "gumpetctl hook: "+format+"\n", a...) }

	input, err := io.ReadAll(io.LimitReader(stdin, 1<<20))
	if err != nil {
		note("read stdin: %v", err)
		return 0
	}
	name := map[string]string{"claude": "Claude Code", "codex": "Codex"}[strings.ToLower(*agent)]
	req, err := agenthook.Parse(input, name)
	if err != nil {
		note("%v", err)
		return 0
	}
	t, err := where()
	if err != nil {
		note("%v", err)
		return 0
	}
	r, err := put(t, question{Text: req.Text(), Title: req.Title(), Level: "warn", TimeoutSec: *timeout})
	if err != nil {
		if !errors.Is(err, errNoAnswer) {
			note("%v", err)
		}
		return 0
	}
	switch r.Index {
	case 0:
		stdout.Write(agenthook.Allow())
	case 1:
		stdout.Write(agenthook.Deny(denyMessage))
	}
	return 0
}
