package feed

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// hackerNews is trimmed from the real feed, entities and all: the titles there
// really do arrive with &#x27; in them, which is the case worth pinning down.
const hackerNews = `<rss version="2.0"><channel>
<title>Hacker News</title>
<link>https://news.ycombinator.com/</link>
<item>
  <title>An Empirical Study of Harness Design</title>
  <link>https://arxiv.org/abs/2609.20804</link>
  <comments>https://news.ycombinator.com/item?id=49753878</comments>
  <description><![CDATA[<a href="https://news.ycombinator.com/item?id=49753878">Comments</a>]]></description>
</item>
<item>
  <title>I don&#x27;t like passkeys</title>
  <link>https://hawksley.dev/blog/i-dont-like-passkeys</link>
</item>
<item>
  <title>Jemalloc &amp; friends</title>
  <link>https://github.com/jemalloc/jemalloc</link>
</item>
</channel></rss>`

const atomFeed = `<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Example</title>
  <entry>
    <title>First post</title>
    <link rel="alternate" href="https://example.com/1"/>
    <link rel="edit" href="https://example.com/edit/1"/>
  </entry>
  <entry>
    <title>Second post</title>
    <link href="https://example.com/2"/>
  </entry>
</feed>`

func TestParseReadsRSS(t *testing.T) {
	items, err := Parse([]byte(hackerNews))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3: %+v", len(items), items)
	}
	if items[0].Title != "An Empirical Study of Harness Design" {
		t.Errorf("title = %q", items[0].Title)
	}
	if items[0].Link != "https://arxiv.org/abs/2609.20804" {
		t.Errorf("link = %q, want the article rather than the comments", items[0].Link)
	}
}

// The real feed is full of these, and a headline reading "I don&#x27;t" in a
// balloon is the whole feature looking broken.
func TestParseResolvesEntities(t *testing.T) {
	items, err := Parse([]byte(hackerNews))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if items[1].Title != "I don't like passkeys" {
		t.Errorf("title = %q, want the apostrophe resolved", items[1].Title)
	}
	if items[2].Title != "Jemalloc & friends" {
		t.Errorf("title = %q, want the ampersand resolved", items[2].Title)
	}
}

func TestParseReadsAtom(t *testing.T) {
	items, err := Parse([]byte(atomFeed))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2: %+v", len(items), items)
	}
	if items[0].Link != "https://example.com/1" {
		t.Errorf("link = %q, want the alternate rather than the edit link", items[0].Link)
	}
	if items[1].Link != "https://example.com/2" {
		t.Errorf("link = %q, want the link with no rel", items[1].Link)
	}
}

// gumpet draws text, not HTML. A feed that puts markup in a title must not end
// up with angle brackets in a balloon.
func TestParseStripsMarkupFromTitles(t *testing.T) {
	const in = `<rss version="2.0"><channel><item>
	  <title>&lt;b&gt;Bold&lt;/b&gt; and &lt;i&gt;italic&lt;/i&gt;</title>
	  <link>https://example.com/x</link>
	</item></channel></rss>`

	items, err := Parse([]byte(in))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if strings.ContainsAny(items[0].Title, "<>") {
		t.Errorf("title = %q, want the tags gone", items[0].Title)
	}
	if items[0].Title != "Bold and italic" {
		t.Errorf("title = %q, want %q", items[0].Title, "Bold and italic")
	}
}

// Tags are stripped before entities are resolved, so an entity that spells a
// tag cannot survive as one.
func TestParseDoesNotLetEntitiesBecomeTags(t *testing.T) {
	const in = `<rss version="2.0"><channel><item>
	  <title>&amp;lt;script&amp;gt;alert(1)&amp;lt;/script&amp;gt;</title>
	  <link>https://example.com/x</link>
	</item></channel></rss>`

	items, err := Parse([]byte(in))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if strings.Contains(items[0].Title, "<script>") {
		t.Errorf("title = %q, want no tag reassembled from entities", items[0].Title)
	}
}

func TestParseDropsItemsItCannotUse(t *testing.T) {
	const in = `<rss version="2.0"><channel>
	<item><title>no link</title></item>
	<item><title></title><link>https://example.com/a</link></item>
	<item><title>javascript</title><link>javascript:alert(1)</link></item>
	<item><title>ftp</title><link>ftp://example.com/x</link></item>
	<item><title>good</title><link>https://example.com/good</link></item>
	</channel></rss>`

	items, err := Parse([]byte(in))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(items) != 1 || items[0].Title != "good" {
		t.Errorf("Parse = %+v, want only the usable item", items)
	}
}

func TestParseCapsARidiculousTitle(t *testing.T) {
	long := strings.Repeat("あ", 5000)
	in := `<rss version="2.0"><channel><item><title>` + long +
		`</title><link>https://example.com/x</link></item></channel></rss>`

	items, err := Parse([]byte(in))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if n := len([]rune(items[0].Title)); n > MaxTitle+1 {
		t.Errorf("title is %d runes, want it capped near %d", n, MaxTitle)
	}
}

func TestParseCollapsesWhitespace(t *testing.T) {
	const in = "<rss version=\"2.0\"><channel><item><title>\n   spread   over\n  lines\n</title><link>https://example.com/x</link></item></channel></rss>"

	items, err := Parse([]byte(in))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if items[0].Title != "spread over lines" {
		t.Errorf("title = %q, want the whitespace collapsed", items[0].Title)
	}
}

func TestParseRejectsRubbish(t *testing.T) {
	for _, in := range []string{"", "not xml at all", "<html><body>hello</body></html>", "<rss><channel></channel></rss>"} {
		if _, err := Parse([]byte(in)); err == nil {
			t.Errorf("Parse(%q) succeeded, want an error", in)
		}
	}
}

func TestTextPutsTheLinkOnItsOwnLine(t *testing.T) {
	got := Item{Title: "A headline", Link: "https://example.com/x"}.Text()
	if got != "A headline\nhttps://example.com/x" {
		t.Errorf("Text = %q", got)
	}
}

func TestFetchReadsAFeedOverHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(hackerNews))
	}))
	defer srv.Close()

	items, err := NewFetcher().Fetch(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(items) != 3 {
		t.Errorf("got %d items, want 3", len(items))
	}
}

// A config file must not be able to turn the feed reader into a file reader.
func TestFetchRefusesSchemesItShouldNotTouch(t *testing.T) {
	for _, url := range []string{
		"file:///etc/passwd",
		"ftp://example.com/feed",
		"javascript:alert(1)",
		"news.ycombinator.com/rss",
		"",
	} {
		if _, err := NewFetcher().Fetch(context.Background(), url); err == nil {
			t.Errorf("Fetch(%q) succeeded, want a refusal", url)
		}
	}
}

func TestFetchReportsABadStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))
	defer srv.Close()

	if _, err := NewFetcher().Fetch(context.Background(), srv.URL); err == nil {
		t.Error("Fetch succeeded on a 404, want an error")
	}
}

// A misconfigured URL pointing at something enormous must not be read into
// memory in full.
func TestFetchStopsReadingAnEndlessBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chunk := strings.Repeat("x", 1<<16)
		for i := 0; i < 200; i++ {
			if _, err := w.Write([]byte(chunk)); err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	// It fails to parse, which is the point: it returns rather than reading
	// 13MB of x.
	if _, err := NewFetcher().Fetch(context.Background(), srv.URL); err == nil {
		t.Error("Fetch succeeded on a body of rubbish, want an error")
	}
}

func quietLog() *slog.Logger { return slog.New(slog.DiscardHandler) }

// The requirement this pins: one broken feed must not cost the others. Point
// gumpet at a good feed and a dead one, and the good one keeps arriving.
func TestFetchAllKeepsGoingPastAFeedThatFails(t *testing.T) {
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(hackerNews))
	}))
	defer good.Close()
	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusInternalServerError)
	}))
	defer broken.Close()

	groups := NewFetcher().FetchAll(context.Background(), []Source{
		{Name: "broken", URL: broken.URL},
		{Name: "good", URL: good.URL},
		{Name: "unresolvable", URL: "https://no.such.host.invalid/rss"},
		{Name: "refused", URL: "file:///etc/passwd"},
	}, quietLog())

	if len(groups) != 1 {
		t.Fatalf("got %d groups, want only the one that answered: %+v", len(groups), groups)
	}
	if groups[0].Name != "good" {
		t.Errorf("group name = %q, want good", groups[0].Name)
	}
	if len(groups[0].Items) != 3 {
		t.Errorf("got %d items, want the 3 from the working feed", len(groups[0].Items))
	}
}

func TestFetchAllLabelsEachGroupWithItsSource(t *testing.T) {
	one := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(hackerNews))
	}))
	defer one.Close()
	two := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(atomFeed))
	}))
	defer two.Close()

	groups := NewFetcher().FetchAll(context.Background(), []Source{
		{Name: "HN", URL: one.URL},
		{Name: "", URL: two.URL},
	}, quietLog())

	if len(groups) != 2 {
		t.Fatalf("got %d groups, want 2", len(groups))
	}
	if groups[0].Name != "HN" || len(groups[0].Items) != 3 {
		t.Errorf("first group = %q with %d items", groups[0].Name, len(groups[0].Items))
	}
	// A feed with no name is allowed; the balloon just gets no heading.
	if groups[1].Name != "" || len(groups[1].Items) != 2 {
		t.Errorf("second group = %q with %d items", groups[1].Name, len(groups[1].Items))
	}
}

func TestFetchAllOfNothingIsNothing(t *testing.T) {
	if got := NewFetcher().FetchAll(context.Background(), nil, quietLog()); len(got) != 0 {
		t.Errorf("FetchAll(nil) = %+v, want nothing", got)
	}
}
