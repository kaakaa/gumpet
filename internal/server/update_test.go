package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/kaakaa/gumpet/internal/config"
	"github.com/kaakaa/gumpet/internal/update"
)

// fakeUpdater stands in for GitHub.
type fakeUpdater struct {
	status update.Status
	err    error
	// hold, if set, keeps Apply waiting until it is closed.
	hold chan struct{}

	mu      sync.Mutex
	applied int
}

func (f *fakeUpdater) Version() string { return f.status.Current }

func (f *fakeUpdater) Check(context.Context) (update.Status, error) { return f.status, f.err }

func (f *fakeUpdater) Apply(context.Context) (update.Status, error) {
	if f.hold != nil {
		<-f.hold
	}
	f.mu.Lock()
	f.applied++
	f.mu.Unlock()
	return f.status, f.err
}

var newer = update.Status{Current: "v0.3.0", Latest: "v0.3.1", Available: true, Installable: true,
	URL: "https://github.com/kaakaa/gumpet/releases/tag/v0.3.1"}

// A gumpet that was not given an updater says so, rather than pretending.
func TestUpdatesAreNotOfferedWithoutAnUpdater(t *testing.T) {
	s, _ := newTestServer(t, config.Server{Addr: "127.0.0.1:0"}, 1)
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		if rec := do(t, s, method, "/api/v1/update", "", "", nil); rec.Code != http.StatusNotFound {
			t.Errorf("%s /api/v1/update = %d, want 404", method, rec.Code)
		}
	}
}

// The page shows the running version without asking GitHub anything.
func TestTheSettingsPageIsToldTheVersion(t *testing.T) {
	s, _ := newTestServer(t, config.Server{Addr: "127.0.0.1:0"}, 1)
	s.SetUpdater(&fakeUpdater{status: newer}, nil)

	rec := do(t, s, http.MethodGet, "/api/v1/config", "", "", nil)
	var body struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Version != "v0.3.0" {
		t.Errorf("version = %q, want v0.3.0", body.Version)
	}
}

func TestCheckingReportsWhatWasFound(t *testing.T) {
	s, _ := newTestServer(t, config.Server{Addr: "127.0.0.1:0"}, 1)
	s.SetUpdater(&fakeUpdater{status: newer}, nil)

	rec := do(t, s, http.MethodGet, "/api/v1/update", "", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", rec.Code, rec.Body)
	}
	var got update.Status
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got != newer {
		t.Errorf("got %+v, want %+v", got, newer)
	}
}

// GitHub being out of reach is news for the page, not a crash.
func TestACheckThatCannotReachGitHubSaysSo(t *testing.T) {
	s, _ := newTestServer(t, config.Server{Addr: "127.0.0.1:0"}, 1)
	s.SetUpdater(&fakeUpdater{err: errors.New("dial tcp: no route to host")}, nil)

	rec := do(t, s, http.MethodGet, "/api/v1/update", "", "", nil)
	if rec.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", rec.Code)
	}
}

// An update installs, answers the page, and then restarts — in that order,
// so the page hears that it worked before the process goes away.
func TestAnUpdateAnswersThenRestarts(t *testing.T) {
	s, _ := newTestServer(t, config.Server{Addr: "127.0.0.1:0"}, 1)
	restarted := make(chan struct{})
	s.SetUpdater(&fakeUpdater{status: newer}, func() { close(restarted) })

	rec := do(t, s, http.MethodPost, "/api/v1/update", "", "", nil)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d (%s), want 202", rec.Code, rec.Body)
	}
	select {
	case <-restarted:
	case <-time.After(3 * time.Second):
		t.Fatal("installed, but never restarted")
	}
}

// A failed update must leave gumpet running, and must be possible to try again.
func TestAFailedUpdateDoesNotRestartAndCanBeRetried(t *testing.T) {
	s, _ := newTestServer(t, config.Server{Addr: "127.0.0.1:0"}, 1)
	f := &fakeUpdater{status: newer, err: errors.New("checksum mismatch")}
	restarted := false
	s.SetUpdater(f, func() { restarted = true })

	for i := range 2 {
		if rec := do(t, s, http.MethodPost, "/api/v1/update", "", "", nil); rec.Code != http.StatusBadGateway {
			t.Errorf("attempt %d: status = %d, want 502", i+1, rec.Code)
		}
	}
	time.Sleep(restartDelay + 200*time.Millisecond)
	if restarted {
		t.Error("restarted after an update that failed")
	}
	if f.applied != 2 {
		t.Errorf("applied %d times, want the retry to have run too", f.applied)
	}
}

// Two clicks must not install twice over each other.
func TestOnlyOneUpdateRunsAtATime(t *testing.T) {
	s, _ := newTestServer(t, config.Server{Addr: "127.0.0.1:0"}, 1)
	f := &fakeUpdater{status: newer, hold: make(chan struct{})}
	s.SetUpdater(f, func() {})

	first := make(chan int)
	go func() { first <- do(t, s, http.MethodPost, "/api/v1/update", "", "", nil).Code }()
	// Give the first one time to take the lock and block inside Apply.
	time.Sleep(100 * time.Millisecond)

	if rec := do(t, s, http.MethodPost, "/api/v1/update", "", "", nil); rec.Code != http.StatusConflict {
		t.Errorf("second update = %d, want 409 while the first is running", rec.Code)
	}
	close(f.hold)
	if code := <-first; code != http.StatusAccepted {
		t.Errorf("first update = %d, want 202", code)
	}
}

// Updating replaces the program, so it needs the token as much as saving
// settings does.
func TestUpdatingNeedsTheToken(t *testing.T) {
	s, _ := newTestServer(t, config.Server{Addr: "127.0.0.1:0", Token: "s3cret"}, 1)
	f := &fakeUpdater{status: newer}
	s.SetUpdater(f, func() {})

	for _, method := range []string{http.MethodGet, http.MethodPost} {
		if rec := do(t, s, method, "/api/v1/update", "", "", nil); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s without the token = %d, want 401", method, rec.Code)
		}
	}
	if f.applied != 0 {
		t.Error("an update ran without the token")
	}
}
