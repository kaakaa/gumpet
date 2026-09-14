// Package server receives messages over HTTP, hands them to the pet, and serves
// gumpet's settings page.
package server

import (
	"context"
	"crypto/subtle"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/kaakaa/gumpet/internal/config"
	"github.com/kaakaa/gumpet/internal/display"
	"github.com/kaakaa/gumpet/internal/fontfile"
	"github.com/kaakaa/gumpet/internal/history"
	"github.com/kaakaa/gumpet/internal/message"
	"github.com/kaakaa/gumpet/internal/petsrc"
	"github.com/kaakaa/gumpet/internal/settings"
)

// maxBodyBytes caps a request body. Messages are meant to be short, and the
// listener is reachable by anything running as the user.
const maxBodyBytes = 64 << 10

//go:embed ui
var ui embed.FS

// Server is gumpet's HTTP listener.
type Server struct {
	// store is shared with the pet, so a change made from its menu shows up on
	// the settings page and the other way round.
	store *settings.Store
	// history records every message, and whether the pet has said it yet.
	history *history.Store
	// monitors is what the machine reported at startup, so the settings page
	// can name the displays rather than asking for a number on faith.
	monitors []display.Monitor
	out      chan<- message.Message
	log      *slog.Logger

	server *http.Server
	// addr is fixed when Serve binds, so that editing server.addr cannot leave
	// the running listener and the reported address disagreeing.
	addr string
}

// messageRequest is the JSON body of POST /api/v1/messages.
type messageRequest struct {
	Text string `json:"text"`
	// DurationSec overrides message.duration_sec for this one message.
	DurationSec float64 `json:"duration_sec"`
}

// New builds a server that publishes received messages to out, records them in
// hist, and saves settings through store. monitors is what the settings page
// offers as the choice of display.
func New(store *settings.Store, hist *history.Store, monitors []display.Monitor, out chan<- message.Message, log *slog.Logger) *Server {
	s := &Server{
		store:    store,
		history:  hist,
		monitors: monitors,
		out:      out,
		log:      log,
		addr:     store.Get().Server.Addr,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.page("ui/settings.html"))
	mux.HandleFunc("GET /messages", s.page("ui/messages.html"))
	mux.HandleFunc("POST /api/v1/messages", s.authed(s.handleMessage))
	mux.HandleFunc("GET /api/v1/messages", s.authed(s.handleListMessages))
	mux.HandleFunc("GET /api/v1/config", s.authed(s.handleGetConfig))
	mux.HandleFunc("PUT /api/v1/config", s.authed(s.handlePutConfig))
	mux.HandleFunc("GET /api/v1/healthz", s.handleHealth)

	s.server = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return s
}

// Serve listens and blocks until ctx is cancelled or the listener fails. The
// address is read once, here: changing it needs a restart.
func (s *Server) Serve(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", s.addr, err)
	}
	s.log.Info("listening", "addr", ln.Addr().String(), "settings", "http://"+s.addr+"/")

	errc := make(chan error, 1)
	go func() { errc <- s.server.Serve(ln) }()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		return s.server.Shutdown(shutdownCtx)
	case err := <-errc:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func (s *Server) config() config.Config { return s.store.Get() }

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// page serves one of the bundled pages, without a token. The pages hold no
// data of their own — they ask the API for it, and the API is what checks the
// token.
func (s *Server) page(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := ui.ReadFile(name)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "page is missing from this build")
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// The pages are self-contained, so nothing may be loaded from anywhere
		// else.
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; connect-src 'self'; form-action 'none'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		_, _ = w.Write(body)
	}
}

// handleListMessages is what the messages page reads: everything gumpet has
// been sent that is still within the retention settings, newest first.
func (s *Server) handleListMessages(w http.ResponseWriter, r *http.Request) {
	records := s.history.List()
	if records == nil {
		records = []history.Record{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"messages": records,
		"history":  s.config().History,
	})
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	monitors := s.monitors
	if monitors == nil {
		monitors = []display.Monitor{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"config":   s.config(),
		"path":     s.store.Path(),
		"monitors": monitors,
		// Anything the running process cannot change on the fly.
		"restart_required": []string{"server.addr", "window.skip_taskbar"},
	})
}

// handlePutConfig validates a whole config, writes it to disk, and hands it to
// the pet. Anything invalid is rejected before either happens, so a bad save
// cannot leave the pet or the file in a broken state.
func (s *Server) handlePutConfig(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "settings body is too large")
		return
	}

	// Decoding over the current config means a partial body only changes what
	// it mentions.
	cfg := s.config()
	if err := json.Unmarshal(body, &cfg); err != nil {
		writeError(w, http.StatusBadRequest, "body is not valid JSON: "+err.Error())
		return
	}
	if err := cfg.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := petsrc.Validate(cfg.Pet.Source); err != nil {
		writeError(w, http.StatusBadRequest, "pet.source: "+err.Error())
		return
	}
	if _, err := fontfile.Resolve(cfg.Font.Path, cfg.Font.System); err != nil {
		writeError(w, http.StatusBadRequest, "font.path: "+err.Error())
		return
	}

	// The store validates again, writes the file and tells the pet.
	if err := s.store.Save(cfg); err != nil {
		s.log.Error("could not save settings", "path", s.store.Path(), "error", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.history.SetLimits(cfg.History)
	s.log.Info("saved settings", "path", s.store.Path())

	writeJSON(w, http.StatusOK, map[string]any{"config": cfg, "path": s.store.Path()})
}

// handleMessage accepts either a JSON body or, for the convenience of a bare
// `curl --data-binary`, a plain-text one.
func (s *Server) handleMessage(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "message body is too large")
		return
	}

	var req messageRequest
	if isJSON(r.Header.Get("Content-Type")) {
		if err := json.Unmarshal(body, &req); err != nil {
			writeError(w, http.StatusBadRequest, "body is not valid JSON: "+err.Error())
			return
		}
	} else {
		req.Text = string(body)
	}

	req.Text = strings.TrimRight(req.Text, "\r\n")
	if strings.TrimSpace(req.Text) == "" {
		writeError(w, http.StatusBadRequest, "text must not be empty")
		return
	}
	if req.DurationSec < 0 {
		writeError(w, http.StatusBadRequest, "duration_sec must not be negative")
		return
	}

	duration := time.Duration(req.DurationSec * float64(time.Second))
	// Record it before handing it over, so that a message the pet never gets
	// round to showing still appears on the messages page as pending.
	rec := s.history.Add(req.Text, duration)

	msg := message.Message{ID: rec.ID, Text: req.Text, Duration: duration}
	select {
	case s.out <- msg:
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted", "id": rec.ID})
	default:
		// The pet drains this channel every frame, so a full buffer means it is
		// not running rather than merely busy.
		writeError(w, http.StatusServiceUnavailable, "the pet is not accepting messages right now")
	}
}

// authed wraps h with the shared-token check, when a token is configured.
func (s *Server) authed(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := s.config().Server.Token
		if token == "" {
			h(w, r)
			return
		}
		if !tokenMatches(token, presentedToken(r)) {
			writeError(w, http.StatusUnauthorized, "missing or invalid token")
			return
		}
		h(w, r)
	}
}

func presentedToken(r *http.Request) string {
	if t := r.Header.Get("X-Gumpet-Token"); t != "" {
		return t
	}
	if a := r.Header.Get("Authorization"); len(a) > 7 && strings.EqualFold(a[:7], "bearer ") {
		return a[7:]
	}
	return ""
}

func tokenMatches(want, got string) bool {
	return subtle.ConstantTimeCompare([]byte(want), []byte(got)) == 1
}

func isJSON(contentType string) bool {
	mediaType, _, _ := strings.Cut(contentType, ";")
	return strings.EqualFold(strings.TrimSpace(mediaType), "application/json")
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
