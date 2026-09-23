package server

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/kaakaa/gumpet/internal/chatter"
	"github.com/kaakaa/gumpet/internal/config"
	"github.com/kaakaa/gumpet/internal/display"
	"github.com/kaakaa/gumpet/internal/history"
	"github.com/kaakaa/gumpet/internal/message"
	"github.com/kaakaa/gumpet/internal/petsrc"
	"github.com/kaakaa/gumpet/internal/settings"
)

// testMonitors stands in for what a machine reports, so the settings page has
// displays to name.
var testMonitors = []display.Monitor{
	{Number: 1, Name: "DELL U4320Q", Width: 3200, Height: 1800},
	{Number: 2, Name: "ASUS PB278", Width: 2560, Height: 1440},
}

func newTestServer(t *testing.T, srv config.Server, buffer int) (*Server, chan message.Message) {
	t.Helper()
	cfg := config.Default()
	cfg.Server = srv
	s, out, _ := newTestServerWithConfig(t, cfg, buffer)
	return s, out
}

func newTestServerWithConfig(t *testing.T, cfg config.Config, buffer int) (*Server, chan message.Message, <-chan config.Config) {
	t.Helper()
	out := make(chan message.Message, buffer)
	store := settings.New(cfg, filepath.Join(t.TempDir(), "config.yaml"))
	updates := store.Subscribe(4)
	srv := New(store, history.New(cfg.History), testMonitors, out, slog.New(slog.DiscardHandler))
	return srv, out, updates
}

func do(t *testing.T, s *Server, method, path, contentType, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	s.server.Handler.ServeHTTP(rec, req)
	return rec
}

func post(t *testing.T, s *Server, contentType, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	return do(t, s, http.MethodPost, "/api/v1/messages", contentType, body, headers)
}

func TestAcceptsJSONMessage(t *testing.T) {
	s, out := newTestServer(t, config.Server{Addr: "127.0.0.1:0"}, 1)

	rec := post(t, s, "application/json", `{"text":"こんにちは","duration_sec":2.5}`, nil)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d (%s)", rec.Code, http.StatusAccepted, rec.Body)
	}

	got := <-out
	if got.Text != "こんにちは" {
		t.Errorf("text = %q, want こんにちは", got.Text)
	}
	if want := 2500 * time.Millisecond; got.Duration != want {
		t.Errorf("duration = %v, want %v", got.Duration, want)
	}
}

func TestAcceptsPlainTextMessage(t *testing.T) {
	s, out := newTestServer(t, config.Server{Addr: "127.0.0.1:0"}, 1)

	rec := post(t, s, "text/plain", "hello gopher\n", nil)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d (%s)", rec.Code, http.StatusAccepted, rec.Body)
	}

	got := <-out
	if got.Text != "hello gopher" {
		t.Errorf("text = %q, want %q", got.Text, "hello gopher")
	}
	if got.Duration != 0 {
		t.Errorf("duration = %v, want 0 so the pet uses its own setting", got.Duration)
	}
}

func TestRejectsEmptyText(t *testing.T) {
	s, _ := newTestServer(t, config.Server{Addr: "127.0.0.1:0"}, 1)

	for _, body := range []string{`{"text":""}`, `{"text":"   "}`} {
		if rec := post(t, s, "application/json", body, nil); rec.Code != http.StatusBadRequest {
			t.Errorf("body %s: status = %d, want %d", body, rec.Code, http.StatusBadRequest)
		}
	}
}

func TestRejectsInvalidJSON(t *testing.T) {
	s, _ := newTestServer(t, config.Server{Addr: "127.0.0.1:0"}, 1)

	if rec := post(t, s, "application/json", "{not json", nil); rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestFullInboxIsReported(t *testing.T) {
	s, _ := newTestServer(t, config.Server{Addr: "127.0.0.1:0"}, 0)

	rec := post(t, s, "text/plain", "nobody is listening", nil)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestTokenIsRequiredWhenConfigured(t *testing.T) {
	cfg := config.Server{Addr: "127.0.0.1:0", Token: "s3cret"}

	tests := []struct {
		name    string
		headers map[string]string
		want    int
	}{
		{"no token", nil, http.StatusUnauthorized},
		{"wrong token", map[string]string{"X-Gumpet-Token": "nope"}, http.StatusUnauthorized},
		{"header token", map[string]string{"X-Gumpet-Token": "s3cret"}, http.StatusAccepted},
		{"bearer token", map[string]string{"Authorization": "Bearer s3cret"}, http.StatusAccepted},
		{"bearer is case-insensitive", map[string]string{"Authorization": "bearer s3cret"}, http.StatusAccepted},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, _ := newTestServer(t, cfg, 1)
			if rec := post(t, s, "text/plain", "hi", tt.headers); rec.Code != tt.want {
				t.Errorf("status = %d, want %d", rec.Code, tt.want)
			}
		})
	}
}

func TestHealthz(t *testing.T) {
	s, _ := newTestServer(t, config.Server{Addr: "127.0.0.1:0"}, 1)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/healthz", nil)
	rec := httptest.NewRecorder()
	s.server.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("body = %v, want status ok", body)
	}
}

func TestOversizedBodyIsRejected(t *testing.T) {
	s, _ := newTestServer(t, config.Server{Addr: "127.0.0.1:0"}, 1)

	rec := post(t, s, "text/plain", strings.Repeat("a", maxBodyBytes+1), nil)
	if rec.Code != http.StatusRequestEntityTooLarge {
		body, _ := io.ReadAll(rec.Body)
		t.Errorf("status = %d, want %d (%s)", rec.Code, http.StatusRequestEntityTooLarge, body)
	}
}

func TestGetConfigReturnsCurrentSettings(t *testing.T) {
	cfg := config.Default()
	cfg.Behavior.Roam = config.RoamNone
	s, _, _ := newTestServerWithConfig(t, cfg, 1)

	rec := do(t, s, http.MethodGet, "/api/v1/config", "", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body struct {
		Config          config.Config `json:"config"`
		Path            string        `json:"path"`
		RestartRequired []string      `json:"restart_required"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !reflect.DeepEqual(body.Config, cfg) {
		t.Errorf("config = %+v, want %+v", body.Config, cfg)
	}
	if body.Path == "" {
		t.Error("path is empty; the settings page shows it to the user")
	}
	if len(body.RestartRequired) == 0 {
		t.Error("restart_required is empty; server.addr at least cannot change live")
	}
}

func TestPutConfigSavesAndPublishes(t *testing.T) {
	s, _, updates := newTestServerWithConfig(t, config.Default(), 1)

	rec := do(t, s, http.MethodPut, "/api/v1/config", "application/json",
		`{"behavior":{"mode":"on-message","roam":"wander","speed":10}}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (%s)", rec.Code, http.StatusOK, rec.Body)
	}

	select {
	case got := <-updates:
		if got.Behavior.Mode != config.ModeOnMessage || got.Behavior.Roam != config.RoamWander {
			t.Errorf("published %+v, want the new behaviour", got.Behavior)
		}
		if got.Stage.Anchor != config.AnchorBottomRight {
			t.Errorf("stage.anchor = %q, want the untouched default", got.Stage.Anchor)
		}
	default:
		t.Fatal("nothing was published to the pet")
	}

	saved, err := config.Load(s.store.Path())
	if err != nil {
		t.Fatalf("reload the saved file: %v", err)
	}
	if saved.Behavior.Mode != config.ModeOnMessage {
		t.Errorf("saved file has mode %q, want on-message", saved.Behavior.Mode)
	}
	if s.config().Behavior.Roam != config.RoamWander {
		t.Error("the server is still serving the old config")
	}
}

func TestPutConfigRejectsInvalidSettings(t *testing.T) {
	s, _, updates := newTestServerWithConfig(t, config.Default(), 1)

	rec := do(t, s, http.MethodPut, "/api/v1/config", "application/json",
		`{"stage":{"width":0}}`, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if len(updates) != 0 {
		t.Error("an invalid config was published to the pet")
	}
	if _, err := os.Stat(s.store.Path()); !os.IsNotExist(err) {
		t.Error("an invalid config was written to disk")
	}
}

func TestPutConfigRejectsAMissingPetSource(t *testing.T) {
	s, _, _ := newTestServerWithConfig(t, config.Default(), 1)

	rec := do(t, s, http.MethodPut, "/api/v1/config", "application/json",
		`{"pet":{"source":"/nope/not-here.png","scale":1,"fps":8}}`, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (%s)", rec.Code, http.StatusBadRequest, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "pet.source") {
		t.Errorf("error does not name the field: %s", rec.Body)
	}
}

func TestPutConfigAcceptsTheBuiltinPet(t *testing.T) {
	s, _, _ := newTestServerWithConfig(t, config.Default(), 1)

	rec := do(t, s, http.MethodPut, "/api/v1/config", "application/json", `{"pet":{"source":"","scale":2,"fps":8}}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (%s)", rec.Code, http.StatusOK, rec.Body)
	}
}

func TestConfigEndpointsRequireTheToken(t *testing.T) {
	cfg := config.Default()
	cfg.Server.Token = "s3cret"
	s, _, _ := newTestServerWithConfig(t, cfg, 1)

	if rec := do(t, s, http.MethodGet, "/api/v1/config", "", "", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("GET status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if rec := do(t, s, http.MethodPut, "/api/v1/config", "application/json", "{}", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("PUT status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestSavingANewTokenTakesEffectImmediately(t *testing.T) {
	s, _, _ := newTestServerWithConfig(t, config.Default(), 1)

	rec := do(t, s, http.MethodPut, "/api/v1/config", "application/json", `{"server":{"addr":"127.0.0.1:8787","token":"new"}}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (%s)", rec.Code, http.StatusOK, rec.Body)
	}
	if rec := do(t, s, http.MethodGet, "/api/v1/config", "", "", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d: the new token should be in force", rec.Code, http.StatusUnauthorized)
	}
	if rec := do(t, s, http.MethodGet, "/api/v1/config", "", "", map[string]string{"X-Gumpet-Token": "new"}); rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d with the new token", rec.Code, http.StatusOK)
	}
}

func TestSettingsPageIsServedWithoutAToken(t *testing.T) {
	cfg := config.Default()
	cfg.Server.Token = "s3cret"
	s, _, _ := newTestServerWithConfig(t, cfg, 1)

	rec := do(t, s, http.MethodGet, "/", "", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want HTML", ct)
	}
	// The page carries no settings itself; it fetches them, and that fetch is
	// what the token guards.
	if strings.Contains(rec.Body.String(), "s3cret") {
		t.Error("the settings page leaked the token")
	}
}

// The pet's menu and the settings page write through the same store, so a
// change made in one has to be what the other reads back.
func TestTheSettingsPageSeesAChangeMadeElsewhere(t *testing.T) {
	cfg := config.Default()
	out := make(chan message.Message, 1)
	store := settings.New(cfg, filepath.Join(t.TempDir(), "config.yaml"))
	s := New(store, history.New(cfg.History), testMonitors, out, slog.New(slog.DiscardHandler))

	// Stand in for the pet's menu flipping a toggle.
	if err := store.Update(func(c *config.Config) { c.Behavior.Roam = config.RoamPerimeter }); err != nil {
		t.Fatalf("Update: %v", err)
	}

	rec := do(t, s, http.MethodGet, "/api/v1/config", "", "", nil)
	var body struct {
		Config config.Config `json:"config"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Config.Behavior.Roam != config.RoamPerimeter {
		t.Errorf("roam = %q, want the perimeter walk the menu chose", body.Config.Behavior.Roam)
	}
}

// The bug this guards against: the settings page was PUTting the whole config
// it had loaded, so a page left open in a tab silently undid anything changed
// from the pet's menu in the meantime.
func TestAPartialSaveDoesNotUndoAChangeMadeElsewhere(t *testing.T) {
	out := make(chan message.Message, 1)
	store := settings.New(config.Default(), filepath.Join(t.TempDir(), "config.yaml"))
	s := New(store, history.New(config.Default().History), testMonitors, out, slog.New(slog.DiscardHandler))

	// The pet's menu turns on "hide until a message" after the page loaded.
	if err := store.Update(func(c *config.Config) { c.Behavior.Mode = config.ModeOnMessage }); err != nil {
		t.Fatalf("Update: %v", err)
	}

	// The page then saves the one field the user actually touched.
	rec := do(t, s, http.MethodPut, "/api/v1/config", "application/json",
		`{"behavior":{"speed":120}}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (%s)", rec.Code, http.StatusOK, rec.Body)
	}

	got := store.Get()
	if got.Behavior.Mode != config.ModeOnMessage {
		t.Errorf("behavior.mode = %q, want the menu's on-message to have survived", got.Behavior.Mode)
	}
	if got.Behavior.Speed != 120 {
		t.Errorf("behavior.speed = %v, want the page's 120", got.Behavior.Speed)
	}

	saved, err := config.Load(store.Path())
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if saved.Behavior.Mode != config.ModeOnMessage {
		t.Errorf("the file says mode %q, want on-message", saved.Behavior.Mode)
	}
}

func TestMessagesAreRecordedAsTheyArrive(t *testing.T) {
	s, out := newTestServer(t, config.Server{Addr: "127.0.0.1:0"}, 4)

	for _, text := range []string{"first", "second"} {
		if rec := post(t, s, "text/plain", text, nil); rec.Code != http.StatusAccepted {
			t.Fatalf("%q: status = %d", text, rec.Code)
		}
	}

	rec := do(t, s, http.MethodGet, "/api/v1/messages", "", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var body struct {
		Messages []history.Record `json:"messages"`
		History  config.History   `json:"history"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(body.Messages) != 2 {
		t.Fatalf("got %d messages, want 2", len(body.Messages))
	}
	if body.Messages[0].Text != "second" {
		t.Errorf("first listed is %q, want the newest %q", body.Messages[0].Text, "second")
	}
	for _, m := range body.Messages {
		if m.Shown() {
			t.Errorf("%q is marked shown, but the pet has not run", m.Text)
		}
	}
	if body.History.Max != config.Default().History.Max {
		t.Errorf("history limits = %+v, want the configured ones", body.History)
	}

	// The message handed to the pet carries the ID it reports back.
	first := <-out
	if first.ID == "" {
		t.Error("the pet was given a message with no ID, so it cannot be marked shown")
	}
}

func TestARecordedMessageIsMarkedShown(t *testing.T) {
	out := make(chan message.Message, 4)
	cfg := config.Default()
	store := settings.New(cfg, filepath.Join(t.TempDir(), "config.yaml"))
	hist := history.New(cfg.History)
	s := New(store, hist, testMonitors, out, slog.New(slog.DiscardHandler))

	post(t, s, "text/plain", "hello", nil)

	// Stand in for the pet putting it on screen.
	hist.MarkShown((<-out).ID)

	rec := do(t, s, http.MethodGet, "/api/v1/messages", "", "", nil)
	var body struct {
		Messages []history.Record `json:"messages"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !body.Messages[0].Shown() {
		t.Error("the message is still listed as waiting")
	}
}

// A message that arrives with nowhere to go is still worth listing: it is the
// only sign the user gets that gumpet took it and the pet did not.
func TestAMessageIsRecordedEvenWhenThePetCannotTakeIt(t *testing.T) {
	s, _ := newTestServer(t, config.Server{Addr: "127.0.0.1:0"}, 0)

	if rec := post(t, s, "text/plain", "nobody home", nil); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}

	rec := do(t, s, http.MethodGet, "/api/v1/messages", "", "", nil)
	var body struct {
		Messages []history.Record `json:"messages"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Messages) != 1 || body.Messages[0].Shown() {
		t.Errorf("messages = %+v, want one, still waiting", body.Messages)
	}
}

func TestSavingHistoryLimitsPrunesStraightAway(t *testing.T) {
	out := make(chan message.Message, 8)
	cfg := config.Default()
	store := settings.New(cfg, filepath.Join(t.TempDir(), "config.yaml"))
	hist := history.New(cfg.History)
	s := New(store, hist, testMonitors, out, slog.New(slog.DiscardHandler))

	for range 5 {
		post(t, s, "text/plain", "hello", nil)
	}

	rec := do(t, s, http.MethodPut, "/api/v1/config", "application/json", `{"history":{"max":2}}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (%s)", rec.Code, http.StatusOK, rec.Body)
	}
	if got := hist.Len(); got != 2 {
		t.Errorf("kept %d messages, want the new limit of 2", got)
	}
}

// What the pet says of its own accord is listed apart from what was sent, so
// a client that reads "messages" still reads exactly what arrived.
func TestRemarksAreListedApartFromMessages(t *testing.T) {
	out := make(chan message.Message, 4)
	cfg := config.Default()
	store := settings.New(cfg, filepath.Join(t.TempDir(), "config.yaml"))
	remarks := history.NewRemarks(cfg.History)
	s := New(store, history.New(cfg.History), testMonitors, out, slog.New(slog.DiscardHandler))
	s.SetRemarks(remarks)

	post(t, s, "text/plain", "sent by someone", nil)
	remarks.AddRemark(chatter.Remark{Text: "Go 2 is out", Title: "Go Blog", Link: "https://go.dev/blog/go2"})

	rec := do(t, s, http.MethodGet, "/api/v1/messages", "", "", nil)
	var body struct {
		Messages []history.Record `json:"messages"`
		Remarks  []history.Record `json:"remarks"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Messages) != 1 || body.Messages[0].Text != "sent by someone" {
		t.Errorf("messages = %+v, want only what was sent", body.Messages)
	}
	if len(body.Remarks) != 1 || body.Remarks[0].Link != "https://go.dev/blog/go2" {
		t.Errorf("remarks = %+v, want the headline with its link", body.Remarks)
	}
}

// A server that was never given a record of remarks still answers with an
// empty list rather than null, so the page has one shape to deal with.
func TestRemarksAreAnEmptyListWhenNotRecorded(t *testing.T) {
	s, _ := newTestServer(t, config.Server{Addr: "127.0.0.1:0"}, 1)

	rec := do(t, s, http.MethodGet, "/api/v1/messages", "", "", nil)
	var body map[string]json.RawMessage
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got := string(body["remarks"]); got != "[]" {
		t.Errorf("remarks = %s, want []", got)
	}
}

// The history limits cover both records, each counted on its own.
func TestSavingHistoryLimitsPrunesRemarksToo(t *testing.T) {
	out := make(chan message.Message, 8)
	cfg := config.Default()
	store := settings.New(cfg, filepath.Join(t.TempDir(), "config.yaml"))
	remarks := history.NewRemarks(cfg.History)
	s := New(store, history.New(cfg.History), testMonitors, out, slog.New(slog.DiscardHandler))
	s.SetRemarks(remarks)

	for range 5 {
		remarks.AddRemark(chatter.Remark{Text: "headline"})
	}
	rec := do(t, s, http.MethodPut, "/api/v1/config", "application/json", `{"history":{"max":2}}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (%s)", rec.Code, http.StatusOK, rec.Body)
	}
	if got := remarks.Len(); got != 2 {
		t.Errorf("kept %d remarks, want the new limit of 2", got)
	}
}

func TestMessagesPageIsServed(t *testing.T) {
	s, _ := newTestServer(t, config.Server{Addr: "127.0.0.1:0"}, 1)

	rec := do(t, s, http.MethodGet, "/messages", "", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want HTML", ct)
	}
}

func TestListingMessagesRequiresTheToken(t *testing.T) {
	cfg := config.Default()
	cfg.Server.Token = "s3cret"
	s, _, _ := newTestServerWithConfig(t, cfg, 1)

	if rec := do(t, s, http.MethodGet, "/api/v1/messages", "", "", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestConfigCarriesTheMonitorsToChooseFrom(t *testing.T) {
	s, _, _ := newTestServerWithConfig(t, config.Default(), 1)

	rec := do(t, s, http.MethodGet, "/api/v1/config", "", "", nil)
	var body struct {
		Monitors []display.Monitor `json:"monitors"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(body.Monitors) != len(testMonitors) {
		t.Fatalf("got %d monitors, want %d", len(body.Monitors), len(testMonitors))
	}
	if body.Monitors[1].Name != "ASUS PB278" || body.Monitors[1].Number != 2 {
		t.Errorf("second monitor = %+v, want the ASUS numbered 2", body.Monitors[1])
	}
}

// A machine with nothing to report must still give the page an array to read,
// not a null it would have to guard against.
func TestConfigCarriesAnEmptyListRatherThanNull(t *testing.T) {
	out := make(chan message.Message, 1)
	store := settings.New(config.Default(), filepath.Join(t.TempDir(), "config.yaml"))
	s := New(store, history.New(config.Default().History), nil, out, slog.New(slog.DiscardHandler))

	rec := do(t, s, http.MethodGet, "/api/v1/config", "", "", nil)
	if !strings.Contains(rec.Body.String(), `"monitors":[]`) {
		t.Errorf("monitors is not an empty array: %s", rec.Body)
	}
}

func TestAMessageCarriesItsTitleAndLevel(t *testing.T) {
	s, out := newTestServer(t, config.Server{}, 1)

	rec := post(t, s, "application/json",
		`{"text":"tests failed","title":"CI","level":"error"}`, nil)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status %d, want %d: %s", rec.Code, http.StatusAccepted, rec.Body)
	}

	msg := <-out
	if msg.Title != "CI" {
		t.Errorf("title %q, want CI", msg.Title)
	}
	if msg.Level != message.LevelError {
		t.Errorf("level %q, want error", msg.Level)
	}
}

// A message is worth more than the field that describes it, so a level nobody
// recognises must not cost the sender the message.
func TestAnUnknownLevelIsTakenAsInfo(t *testing.T) {
	s, out := newTestServer(t, config.Server{}, 1)

	rec := post(t, s, "application/json", `{"text":"hello","level":"CRITICAL"}`, nil)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status %d, want %d: %s", rec.Code, http.StatusAccepted, rec.Body)
	}
	if msg := <-out; msg.Level != message.LevelInfo {
		t.Errorf("level %q, want info", msg.Level)
	}
}

func TestAMessageWithNoLevelIsInfo(t *testing.T) {
	s, out := newTestServer(t, config.Server{}, 1)

	post(t, s, "application/json", `{"text":"hello"}`, nil)
	msg := <-out
	if msg.Level != message.LevelInfo {
		t.Errorf("level %q, want info", msg.Level)
	}
	if msg.Title != "" {
		t.Errorf("title %q, want empty", msg.Title)
	}
}

func TestPlainTextMessagesStillArriveWithALevel(t *testing.T) {
	s, out := newTestServer(t, config.Server{}, 1)

	post(t, s, "text/plain", "just text", nil)
	if msg := <-out; msg.Level != message.LevelInfo {
		t.Errorf("level %q, want info", msg.Level)
	}
}

func TestTheMessagesListShowsTitleAndLevel(t *testing.T) {
	s, out := newTestServer(t, config.Server{}, 1)
	post(t, s, "application/json", `{"text":"deployed","title":"release","level":"success"}`, nil)
	<-out

	rec := do(t, s, http.MethodGet, "/api/v1/messages", "", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200: %s", rec.Code, rec.Body)
	}
	var got struct {
		Messages []struct {
			Text  string `json:"text"`
			Title string `json:"title"`
			Level string `json:"level"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if len(got.Messages) != 1 {
		t.Fatalf("listed %d messages, want 1", len(got.Messages))
	}
	if got.Messages[0].Title != "release" || got.Messages[0].Level != "success" {
		t.Errorf("listed title %q level %q, want release/success",
			got.Messages[0].Title, got.Messages[0].Level)
	}
}

// behavior.chatter.* is the first setting nested three deep. The page sends
// only what changed, so if the merge does not reach that far a save would
// quietly wipe the neighbouring keys.
func TestAThreeLevelSettingMergesWithoutDisturbingItsSiblings(t *testing.T) {
	cfg := config.Default()
	cfg.Behavior.Chatter = config.Chatter{
		Enabled: false, IntervalSec: 600, Source: "/tmp/mine.txt", FetchIntervalSec: 1800,
	}
	s, _, updates := newTestServerWithConfig(t, cfg, 1)

	rec := do(t, s, http.MethodPut, "/api/v1/config", "application/json",
		`{"behavior":{"chatter":{"enabled":true}}}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200: %s", rec.Code, rec.Body)
	}

	got := <-updates
	if !got.Behavior.Chatter.Enabled {
		t.Error("enabled did not take")
	}
	if got.Behavior.Chatter.IntervalSec != 600 {
		t.Errorf("interval_sec = %v, want the 600 that was not mentioned", got.Behavior.Chatter.IntervalSec)
	}
	if got.Behavior.Chatter.Source != "/tmp/mine.txt" {
		t.Errorf("source = %q, want the path that was not mentioned", got.Behavior.Chatter.Source)
	}
	if got.Behavior.Roam != cfg.Behavior.Roam || got.Behavior.Speed != cfg.Behavior.Speed {
		t.Errorf("the rest of behavior changed: roam %q speed %v", got.Behavior.Roam, got.Behavior.Speed)
	}
}

func TestAnUnusableChatterIntervalIsRejected(t *testing.T) {
	s, _, _ := newTestServerWithConfig(t, config.Default(), 1)

	rec := do(t, s, http.MethodPut, "/api/v1/config", "application/json",
		`{"behavior":{"chatter":{"interval_sec":0}}}`, nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status %d, want 400: %s", rec.Code, rec.Body)
	}
}

// The bug this pins: a second gumpet used to start happily and receive
// nothing, because binding was folded into Serve and its error was not read
// until the process exited. Listen exists so the failure arrives up front.
func TestListenReportsAnAddressAlreadyInUse(t *testing.T) {
	first, _, _ := newTestServerWithConfig(t, addrConfig(t, "127.0.0.1:0"), 1)
	ln, err := first.Listen()
	if err != nil {
		t.Fatalf("first Listen: %v", err)
	}
	defer ln.Close()

	// A second server on the address the first one actually got.
	second, _, _ := newTestServerWithConfig(t, addrConfig(t, ln.Addr().String()), 1)
	got, err := second.Listen()
	if err == nil {
		got.Close()
		t.Fatal("second Listen succeeded on an address already in use, want an error")
	}
	if !strings.Contains(err.Error(), ln.Addr().String()) {
		t.Errorf("error %q does not name the address that could not be had", err)
	}
}

func TestListenThenServeAnswersRequests(t *testing.T) {
	s, _, _ := newTestServerWithConfig(t, addrConfig(t, "127.0.0.1:0"), 1)
	ln, err := s.Listen()
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx, ln) }()

	resp, err := http.Get("http://" + ln.Addr().String() + "/api/v1/healthz")
	if err != nil {
		t.Fatalf("GET healthz: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("healthz returned %d, want 200", resp.StatusCode)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Serve returned %v, want a clean shutdown", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("Serve did not return after the context was cancelled")
	}
}

func addrConfig(t *testing.T, addr string) config.Config {
	t.Helper()
	cfg := config.Default()
	cfg.Server.Addr = addr
	return cfg
}

func TestFeedsCanBeSavedAsAList(t *testing.T) {
	s, _, updates := newTestServerWithConfig(t, config.Default(), 1)

	rec := do(t, s, http.MethodPut, "/api/v1/config", "application/json",
		`{"behavior":{"chatter":{"feeds":[`+
			`{"name":"HN","url":"https://news.ycombinator.com/rss"},`+
			`{"name":"","url":"https://go.dev/blog/feed.atom"}]}}}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200: %s", rec.Code, rec.Body)
	}

	got := (<-updates).Behavior.Chatter.Feeds
	if len(got) != 2 {
		t.Fatalf("feeds = %+v, want 2", got)
	}
	if got[0].Name != "HN" || got[0].URL != "https://news.ycombinator.com/rss" {
		t.Errorf("feeds[0] = %+v", got[0])
	}
	if got[1].Name != "" || got[1].URL != "https://go.dev/blog/feed.atom" {
		t.Errorf("feeds[1] = %+v", got[1])
	}
}

// The page sends a list whole, so emptying it has to mean "no feeds" rather
// than "leave them alone" — otherwise a feed can be added but never removed.
func TestAnEmptyFeedListTurnsFeedsOff(t *testing.T) {
	cfg := config.Default()
	cfg.Behavior.Chatter.Feeds = []config.Feed{{Name: "HN", URL: "https://news.ycombinator.com/rss"}}
	s, _, updates := newTestServerWithConfig(t, cfg, 1)

	rec := do(t, s, http.MethodPut, "/api/v1/config", "application/json",
		`{"behavior":{"chatter":{"feeds":[]}}}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200: %s", rec.Code, rec.Body)
	}
	if got := (<-updates).Behavior.Chatter.Feeds; len(got) != 0 {
		t.Errorf("feeds = %+v, want none left", got)
	}
}

func TestAFeedURLThatIsNotHTTPIsRejected(t *testing.T) {
	s, _, _ := newTestServerWithConfig(t, config.Default(), 1)

	for _, body := range []string{
		`{"behavior":{"chatter":{"feeds":[{"url":"file:///etc/passwd"}]}}}`,
		`{"behavior":{"chatter":{"feeds":[{"url":"news.ycombinator.com/rss"}]}}}`,
		`{"behavior":{"chatter":{"feeds":[{"name":"one\ntwo","url":"https://example.com/f"}]}}}`,
	} {
		rec := do(t, s, http.MethodPut, "/api/v1/config", "application/json", body, nil)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("status %d for %s, want 400", rec.Code, body)
		}
	}
}

// The page iterates this list, so it must never arrive as null.
func TestConfigCarriesAnEmptyFeedListRatherThanNull(t *testing.T) {
	s, _, _ := newTestServerWithConfig(t, config.Default(), 1)

	rec := do(t, s, http.MethodGet, "/api/v1/config", "", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"feeds":[]`) {
		t.Errorf("response does not carry an empty feeds list: %s", rec.Body)
	}
}

// The settings page builds its pet picker from this, so it has to name every
// bundled pet rather than a list written out by hand in the HTML.
func TestConfigCarriesTheBundledPets(t *testing.T) {
	s, _, _ := newTestServerWithConfig(t, config.Default(), 1)

	rec := do(t, s, http.MethodGet, "/api/v1/config", "", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", rec.Code)
	}
	var got struct {
		Pets []struct {
			Name  string `json:"name"`
			Label string `json:"label"`
		} `json:"pets"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got.Pets) != len(petsrc.Builtins) {
		t.Fatalf("listed %d pets, want %d", len(got.Pets), len(petsrc.Builtins))
	}
	for i, p := range got.Pets {
		if p.Name != petsrc.Builtins[i].Name || p.Label != petsrc.Builtins[i].Label {
			t.Errorf("pets[%d] = %+v, want %+v", i, p, petsrc.Builtins[i])
		}
		if p.Label == "" {
			t.Errorf("pets[%d] has no label to show", i)
		}
	}
}

// Every name the page offers has to be one the config will accept, or picking
// a pet from the list would be rejected on save.
func TestEveryOfferedPetIsAValidSetting(t *testing.T) {
	s, _, updates := newTestServerWithConfig(t, config.Default(), 1)

	for _, pet := range petsrc.Builtins {
		body := `{"pet":{"source":"` + pet.Name + `"}}`
		rec := do(t, s, http.MethodPut, "/api/v1/config", "application/json", body, nil)
		if rec.Code != http.StatusOK {
			t.Errorf("saving %q: status %d: %s", pet.Name, rec.Code, rec.Body)
			continue
		}
		if got := (<-updates).Pet.Source; got != pet.Name {
			t.Errorf("saved %q, got %q", pet.Name, got)
		}
	}
}
