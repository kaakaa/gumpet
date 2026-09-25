package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/kaakaa/gumpet/internal/config"
	"github.com/kaakaa/gumpet/internal/feed"
)

// feedServer serves body as a feed, and a server configured to read it and
// one other that has not been read.
func feedServer(t *testing.T, body string, token string) (*Server, string) {
	t.Helper()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(upstream.Close)

	cfg := config.Default()
	cfg.Server = config.Server{Addr: "127.0.0.1:0", Token: token}
	cfg.Behavior.Chatter.Enabled = true
	cfg.Behavior.Chatter.Feeds = []config.Feed{{Name: "Read", URL: upstream.URL}, {Name: "Waiting", URL: "https://waiting.example/feed"}}
	s, _, _ := newTestServerWithConfig(t, cfg, 1)

	reports := feed.NewReports()
	f := feed.NewFetcher()
	f.Reports = reports
	f.FetchAll(context.Background(), []feed.Source{{URL: upstream.URL}}, 30*24*time.Hour, slog.New(slog.DiscardHandler))
	s.SetFeedReports(reports)
	return s, upstream.URL
}

const oneItem = `<rss version="2.0"><channel><item><title>A headline</title><link>https://example.com/a</link></item></channel></rss>`

func TestFeedsAreReportedInTheOrderTheSettingsList(t *testing.T) {
	s, readURL := feedServer(t, oneItem, "")

	rec := do(t, s, http.MethodGet, "/api/v1/feeds", "", "", nil)
	var body struct {
		Reading bool          `json:"reading"`
		Feeds   []feed.Report `json:"feeds"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if !body.Reading {
		t.Error("reading = false, though the pet is set to talk to itself")
	}
	if len(body.Feeds) != 2 || body.Feeds[0].URL != readURL || body.Feeds[1].Name != "Waiting" {
		t.Fatalf("feeds = %+v, want the two configured, in order", body.Feeds)
	}
	if body.Feeds[0].Found != 1 || !body.Feeds[0].HasSource || body.Feeds[0].FetchedAt.IsZero() {
		t.Errorf("first = %+v, want a read with one headline and its source", body.Feeds[0])
	}
	if !body.Feeds[1].FetchedAt.IsZero() {
		t.Errorf("second = %+v, want it reported as not read yet", body.Feeds[1])
	}
}

func TestAFeedsSourceIsServedAsText(t *testing.T) {
	s, readURL := feedServer(t, oneItem, "")

	rec := do(t, s, http.MethodGet, "/api/v1/feeds/source?url="+url.QueryEscape(readURL), "", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", rec.Code, rec.Body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" && ct != "application/json; charset=utf-8" {
		t.Errorf("content type = %q; somebody else's feed must not come back as anything a browser renders", ct)
	}
	var body struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Text != oneItem {
		t.Errorf("text = %q, want what the feed served", body.Text)
	}
}

// Only a configured feed is answered for, and only once it has served
// something: this must not become a way to ask gumpet about any URL.
func TestOnlyAConfiguredFeedsSourceIsServed(t *testing.T) {
	s, _ := feedServer(t, oneItem, "")
	for _, u := range []string{"https://waiting.example/feed", "https://not.configured/feed", ""} {
		rec := do(t, s, http.MethodGet, "/api/v1/feeds/source?url="+url.QueryEscape(u), "", "", nil)
		if rec.Code != http.StatusNotFound {
			t.Errorf("source of %q = %d, want 404", u, rec.Code)
		}
	}
}

func TestFeedReportsNeedTheToken(t *testing.T) {
	s, readURL := feedServer(t, oneItem, "s3cret")
	for _, path := range []string{"/api/v1/feeds", "/api/v1/feeds/source?url=" + url.QueryEscape(readURL)} {
		if rec := do(t, s, http.MethodGet, path, "", "", nil); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s without the token = %d, want 401", path, rec.Code)
		}
	}
}

func TestFeedsAreAnEmptyListWithoutReports(t *testing.T) {
	s, _ := newTestServer(t, config.Server{Addr: "127.0.0.1:0"}, 1)
	rec := do(t, s, http.MethodGet, "/api/v1/feeds", "", "", nil)
	var body map[string]json.RawMessage
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if string(body["feeds"]) != "[]" {
		t.Errorf("feeds = %s, want []", body["feeds"])
	}
}
