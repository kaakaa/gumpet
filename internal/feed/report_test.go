package feed

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/text/encoding/japanese"
)

const twoItems = `<?xml version="1.0"?>
<rss version="2.0"><channel><title>t</title>
<item><title>new one</title><link>https://example.com/new</link><pubDate>` + "%NOW%" + `</pubDate></item>
<item><title>old one</title><link>https://example.com/old</link><pubDate>Mon, 02 Jan 2006 15:04:05 GMT</pubDate></item>
</channel></rss>`

// serve answers every request with status and body.
func serve(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func readAll(t *testing.T, sources []Source) *Reports {
	t.Helper()
	reports := NewReports()
	f := NewFetcher()
	f.Reports = reports
	f.FetchAll(context.Background(), sources, 30*24*time.Hour, slog.New(slog.DiscardHandler))
	return reports
}

// A feed that works is reported with what it held and how much of it was new
// enough to say, and what it served can be shown.
func TestAReadIsReported(t *testing.T) {
	body := strings.Replace(twoItems, "%NOW%", time.Now().UTC().Format(time.RFC1123), 1)
	srv := serve(t, http.StatusOK, body)
	src := []Source{{Name: "Example", URL: srv.URL}}

	reports := readAll(t, src)
	got := reports.For(src)[0]

	if got.FetchedAt.IsZero() || got.Error != "" || got.Status != "200 OK" {
		t.Errorf("report = %+v, want a successful read", got)
	}
	if got.Found != 2 || got.Recent != 1 {
		t.Errorf("found %d, recent %d; want 2 and 1 (the old one is past the age limit)", got.Found, got.Recent)
	}
	if !got.HasSource || got.Size != len(body) {
		t.Errorf("has source %v, size %d; want the %d bytes it served", got.HasSource, got.Size, len(body))
	}
	if text, ok := reports.Source(srv.URL); !ok || text != body {
		t.Errorf("source = %q, %v; want what the feed served", text, ok)
	}
}

// Each way of failing says why, and keeps whatever the server did send: a
// 404 page or an HTML page where a feed should be is what someone looking into
// it most wants to see.
func TestAFailureIsReportedWithWhatWasServed(t *testing.T) {
	gone := serve(t, http.StatusNotFound, "no feed here")
	html := serve(t, http.StatusOK, "<html><body>We moved!</body></html>")
	closed := httptest.NewServer(http.NotFoundHandler())
	closed.Close()

	cases := []struct {
		name       string
		url        string
		status     string
		wantSource string
	}{
		{"a 404", gone.URL, "404 Not Found", "no feed here"},
		{"a web page, not a feed", html.URL, "200 OK", "<html><body>We moved!</body></html>"},
		{"nothing listening", closed.URL, "", ""},
	}
	for _, c := range cases {
		src := []Source{{URL: c.url}}
		reports := readAll(t, src)
		got := reports.For(src)[0]
		if got.Error == "" || got.FetchedAt.IsZero() {
			t.Errorf("%s: report = %+v, want a failed read with a reason", c.name, got)
		}
		if got.Status != c.status {
			t.Errorf("%s: status = %q, want %q", c.name, got.Status, c.status)
		}
		text, ok := reports.Source(c.url)
		if ok != (c.wantSource != "") || text != c.wantSource {
			t.Errorf("%s: source = %q, %v; want %q", c.name, text, ok, c.wantSource)
		}
	}
}

// Reports follow the configuration: in its order, with the names it gives
// today, a feed just added shown as not read yet, and one taken out not shown.
func TestReportsFollowTheConfiguration(t *testing.T) {
	srv := serve(t, http.StatusOK, strings.Replace(twoItems, "%NOW%", time.Now().UTC().Format(time.RFC1123), 1))
	reports := readAll(t, []Source{{Name: "old name", URL: srv.URL}, {URL: "https://removed.example/feed"}})

	got := reports.For([]Source{{URL: "https://added.example/feed"}, {Name: "new name", URL: srv.URL}})
	if len(got) != 2 {
		t.Fatalf("got %d reports, want one per configured feed", len(got))
	}
	if got[0].URL != "https://added.example/feed" || !got[0].FetchedAt.IsZero() {
		t.Errorf("first = %+v, want the added feed, not read yet", got[0])
	}
	if got[1].Name != "new name" || got[1].FetchedAt.IsZero() {
		t.Errorf("second = %+v, want the read feed under its new name", got[1])
	}
}

// A Shift_JIS feed's source is shown in Japanese, decoded by the encoding it
// declares, not as mojibake.
func TestReadableDecodesWhatTheFeedDeclares(t *testing.T) {
	text := `<?xml version="1.0" encoding="Shift_JIS"?><rss><channel><title>日本語のフィード</title></channel></rss>`
	sjis, err := japanese.ShiftJIS.NewEncoder().String(text)
	if err != nil {
		t.Fatal(err)
	}
	if got := Readable([]byte(sjis)); got != text {
		t.Errorf("Readable = %q, want %q", got, text)
	}
	// Bytes that make no sense are still shown, marked, rather than hidden.
	if got := Readable([]byte("ok \xff\xfe bad")); got != "ok � bad" {
		t.Errorf("Readable of broken bytes = %q", got)
	}
}
