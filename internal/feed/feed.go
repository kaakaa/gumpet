// Package feed reads headlines out of an RSS or Atom feed.
//
// It is the first thing in gumpet that reaches out to the network rather than
// waiting to be spoken to, and it is written on that assumption: what comes
// back is somebody else's XML, so it is bounded, stripped of markup, and never
// trusted to be well-behaved.
//
// Parsing is separate from fetching so that the awkward parts — entities,
// Atom's several kinds of link, titles with tags in them — are tested against
// stored bytes rather than against whatever the internet is serving today.
package feed

import (
	"context"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// MaxBody is how much of a feed is read before giving up. Feeds are tens of
// kilobytes; a response far larger than that is a misconfigured URL, and
// reading it into memory is the pet's problem rather than the server's.
const MaxBody = 4 << 20

// MaxTitle caps a headline in runes. A feed can say anything, and a balloon
// the height of the screen is a poor way to find that out.
const MaxTitle = 200

// Timeout bounds a single fetch. Nothing waits on it — the pet carries on
// while it runs — but a request that never finishes would leak a goroutine per
// attempt.
const Timeout = 5 * time.Second

// Item is one headline and where it points.
type Item struct {
	Title string
	Link  string
}

// rss is the subset of RSS 2.0 worth reading. Everything else in the document
// is ignored rather than rejected: feeds carry all sorts of extensions, and
// none of them stop the titles being readable.
type rss struct {
	Items []struct {
		Title string `xml:"title"`
		Link  string `xml:"link"`
	} `xml:"channel>item"`
}

type atom struct {
	Entries []struct {
		Title string `xml:"title"`
		Links []struct {
			Href string `xml:"href,attr"`
			Rel  string `xml:"rel,attr"`
			Type string `xml:"type,attr"`
		} `xml:"link"`
	} `xml:"entry"`
}

// Parse reads items out of an RSS or Atom document. Which of the two it is is
// decided by which one produces items, because the alternative is trusting a
// root element name that half the feeds in the world get creative with.
func Parse(data []byte) ([]Item, error) {
	var r rss
	if err := xml.Unmarshal(data, &r); err == nil && len(r.Items) > 0 {
		out := make([]Item, 0, len(r.Items))
		for _, it := range r.Items {
			if item, ok := clean(it.Title, it.Link); ok {
				out = append(out, item)
			}
		}
		if len(out) > 0 {
			return out, nil
		}
	}

	var a atom
	if err := xml.Unmarshal(data, &a); err != nil {
		return nil, fmt.Errorf("parse feed: %w", err)
	}
	out := make([]Item, 0, len(a.Entries))
	for _, e := range a.Entries {
		if item, ok := clean(e.Title, atomLink(e.Links)); ok {
			out = append(out, item)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("parse feed: no usable items")
	}
	return out, nil
}

// atomLink picks the entry's page. Atom entries carry several links; the
// alternate one is the article, and a link with no rel at all means the same
// thing by the specification's own default.
func atomLink(links []struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr"`
}) string {
	for _, l := range links {
		if l.Rel == "alternate" || l.Rel == "" {
			return l.Href
		}
	}
	if len(links) > 0 {
		return links[0].Href
	}
	return ""
}

var tagPattern = regexp.MustCompile(`<[^>]*>`)

// clean turns one feed entry into something safe to draw, and reports whether
// anything usable was left.
func clean(title, link string) (Item, bool) {
	// Some feeds put markup in the title, and gumpet draws text rather than
	// HTML. Entities are resolved first and tags stripped second, so that a
	// title spelling a tag as &amp;lt;script&amp;gt; cannot come out of this
	// as one: stripping has to be the last thing that runs, or it is stripping
	// a string that is still about to change.
	title = tagPattern.ReplaceAllString(html.UnescapeString(title), "")
	title = strings.Join(strings.Fields(title), " ")
	if r := []rune(title); len(r) > MaxTitle {
		title = strings.TrimSpace(string(r[:MaxTitle])) + "…"
	}

	link = strings.TrimSpace(html.UnescapeString(link))
	if title == "" || !Openable(link) {
		return Item{}, false
	}
	return Item{Title: title, Link: link}, true
}

// Openable reports whether a URL is one gumpet will put in front of anyone.
// Only http and https: a feed is somebody else's document, and a headline is
// not permission to hand the desktop an arbitrary scheme.
func Openable(url string) bool {
	lower := strings.ToLower(url)
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}

// Fetcher retrieves a feed. The client is a field so a test can serve one from
// memory rather than over a socket.
type Fetcher struct {
	Client *http.Client
}

// NewFetcher returns a Fetcher with sensible bounds.
func NewFetcher() *Fetcher {
	return &Fetcher{Client: &http.Client{Timeout: Timeout}}
}

// Fetch reads the feed at url. It refuses anything but http and https before
// making a request at all, so a config file cannot turn this into a way of
// reading local files.
func (f *Fetcher) Fetch(ctx context.Context, url string) ([]Item, error) {
	if !Openable(url) {
		return nil, fmt.Errorf("feed URL must be http or https: %q", url)
	}

	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "gumpet")
	req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/xml, text/xml")

	client := f.Client
	if client == nil {
		client = &http.Client{Timeout: Timeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: %s", url, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxBody))
	if err != nil {
		return nil, err
	}
	return Parse(body)
}

// Source is a feed to read, together with the name headlines from it are
// labelled with.
type Source struct {
	Name string
	URL  string
}

// Group is what one source had to say. It is kept separate from the others so
// that a source publishing thirty things does not drown out one publishing
// three.
type Group struct {
	Name  string
	Items []Item
}

// FetchAll reads every source and returns a group for each that answered.
//
// The sources are read one after another rather than all at once: there are a
// handful of them and a long wait between rounds, so there is nothing to gain
// from a burst of connections and a good deal of manners in not making one.
//
// A source that fails costs one line in the log and nothing else. The others
// are still read, and the failed one is tried again next time — one feed being
// down must not take the rest with it.
func (f *Fetcher) FetchAll(ctx context.Context, sources []Source, log *slog.Logger) []Group {
	out := make([]Group, 0, len(sources))
	for _, src := range sources {
		items, err := f.Fetch(ctx, src.URL)
		if err != nil {
			log.Error("could not read a feed, skipping it this time",
				"feed", src.URL, "name", src.Name, "error", err)
			continue
		}
		out = append(out, Group{Name: src.Name, Items: items})
		log.Info("read a feed", "headlines", len(items), "name", src.Name, "feed", src.URL)
	}
	return out
}

// Text renders one item the way the pet says it: the headline, then the URL on
// its own line. The URL is left as plain text because gumpet already spots one
// and draws it as a link — there is nothing for this package to mark up.
func (i Item) Text() string { return i.Title + "\n" + i.Link }
