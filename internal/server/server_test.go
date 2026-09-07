package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kaakaa/gumpet/internal/config"
	"github.com/kaakaa/gumpet/internal/message"
	"github.com/kaakaa/gumpet/internal/settings"
)

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
	return New(store, out, slog.New(slog.DiscardHandler)), out, updates
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
	if body.Config != cfg {
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
	s := New(store, out, slog.New(slog.DiscardHandler))

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
	s := New(store, out, slog.New(slog.DiscardHandler))

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
