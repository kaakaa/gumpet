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
	"bytes"
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

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
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
	// Published is when the feed says the item appeared, or the zero time if
	// it did not say. Feeds are not obliged to date anything, and plenty do
	// not.
	Published time.Time
}

// rss is the subset of RSS 2.0 worth reading. Everything else in the document
// is ignored rather than rejected: feeds carry all sorts of extensions, and
// none of them stop the titles being readable.
type rss struct {
	Items []struct {
		Title string `xml:"title"`
		Link  string `xml:"link"`
		// RSS dates are RFC 822 with several spellings in the wild; see
		// [parseTime].
		PubDate string `xml:"pubDate"`
		Date    string `xml:"date"`
	} `xml:"channel>item"`
}

// rdf is RSS 1.0, where items are siblings of the channel rather than children
// of it. It is still widely served — Hatena Bookmark, among others — and looks
// enough like RSS 2.0 that the difference is easy to miss and total: nothing
// at all is found rather than something slightly wrong.
type rdf struct {
	Items []struct {
		Title string `xml:"title"`
		Link  string `xml:"link"`
		// RSS 1.0 dates its items with Dublin Core rather than pubDate.
		Date string `xml:"date"`
	} `xml:"item"`
}

type atom struct {
	Entries []struct {
		Title string `xml:"title"`
		Links []struct {
			Href string `xml:"href,attr"`
			Rel  string `xml:"rel,attr"`
			Type string `xml:"type,attr"`
		} `xml:"link"`
		// Published is when it first appeared and Updated when it last
		// changed. Published is preferred, since an old post edited yesterday
		// is still an old post.
		Published string `xml:"published"`
		Updated   string `xml:"updated"`
	} `xml:"entry"`
}

// Parse reads items out of an RSS or Atom document. Which of the two it is is
// decided by which one produces items, because the alternative is trusting a
// root element name that half the feeds in the world get creative with.
func Parse(data []byte) ([]Item, error) {
	var r rss
	if err := decode(data, &r); err == nil && len(r.Items) > 0 {
		out := make([]Item, 0, len(r.Items))
		for _, it := range r.Items {
			if item, ok := clean(it.Title, it.Link, firstOf(it.PubDate, it.Date)); ok {
				out = append(out, item)
			}
		}
		if len(out) > 0 {
			return out, nil
		}
	}

	// RSS 1.0 before Atom: its items are named the same as RSS 2.0's, so it is
	// the nearer neighbour, and an Atom document has no <item> to confuse it.
	var d rdf
	if err := decode(data, &d); err == nil && len(d.Items) > 0 {
		out := make([]Item, 0, len(d.Items))
		for _, it := range d.Items {
			if item, ok := clean(it.Title, it.Link, it.Date); ok {
				out = append(out, item)
			}
		}
		if len(out) > 0 {
			return out, nil
		}
	}

	var a atom
	if err := decode(data, &a); err != nil {
		return nil, fmt.Errorf("parse feed: %w", err)
	}
	out := make([]Item, 0, len(a.Entries))
	for _, e := range a.Entries {
		if item, ok := clean(e.Title, atomLink(e.Links), firstOf(e.Published, e.Updated)); ok {
			out = append(out, item)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("parse feed: no usable items")
	}
	return out, nil
}

// decode unmarshals XML that may not be UTF-8. encoding/xml refuses any other
// declared encoding unless it is told how to read one, and plenty of feeds
// still declare Latin-1 or Shift_JIS.
func decode(data []byte, v any) error {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.CharsetReader = charsetReader
	return dec.Decode(v)
}

// charsets are the declared encodings worth handling: the Latin ones that turn
// up on older English-language feeds, and the two Japanese ones still served by
// sites that predate UTF-8.
var charsets = map[string]encoding.Encoding{
	"iso-8859-1":   charmap.ISO8859_1,
	"iso8859-1":    charmap.ISO8859_1,
	"latin1":       charmap.ISO8859_1,
	"iso-8859-15":  charmap.ISO8859_15,
	"windows-1252": charmap.Windows1252,
	"cp1252":       charmap.Windows1252,
	"shift_jis":    japanese.ShiftJIS,
	"shift-jis":    japanese.ShiftJIS,
	"sjis":         japanese.ShiftJIS,
	"x-sjis":       japanese.ShiftJIS,
	"windows-31j":  japanese.ShiftJIS,
	"cp932":        japanese.ShiftJIS,
	"euc-jp":       japanese.EUCJP,
	"eucjp":        japanese.EUCJP,
	"iso-2022-jp":  japanese.ISO2022JP,
}

func charsetReader(label string, input io.Reader) (io.Reader, error) {
	name := strings.ToLower(strings.TrimSpace(label))
	switch name {
	case "", "utf-8", "utf8", "us-ascii", "ascii":
		return input, nil
	}
	enc, ok := charsets[name]
	if !ok {
		return nil, fmt.Errorf("feed declares an encoding gumpet cannot read: %q", label)
	}
	// Decoded leniently: a feed whose bytes do not match the encoding it
	// declared is a real thing, and a headline with one wrong character in it
	// is better than no feed at all.
	return enc.NewDecoder().Reader(input), nil
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
func clean(title, link, published string) (Item, bool) {
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
	return Item{Title: title, Link: link, Published: parseTime(published)}, true
}

func firstOf(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// timeFormats are the spellings actually met in the wild. RSS says RFC 822 and
// Atom says RFC 3339, but feeds are written by hand as often as not: the
// single-digit days, the missing seconds and the named zones are all things
// real feeds do.
var timeFormats = []string{
	time.RFC1123Z,
	time.RFC1123,
	time.RFC3339,
	time.RFC822Z,
	time.RFC822,
	"Mon, 2 Jan 2006 15:04:05 -0700",
	"Mon, 2 Jan 2006 15:04:05 MST",
	"Mon, 2 Jan 2006 15:04 -0700",
	"2 Jan 2006 15:04:05 -0700",
	"2006-01-02T15:04:05-07:00",
	"2006-01-02T15:04:05Z",
	"2006-01-02 15:04:05",
	"2006-01-02",
}

// parseTime reads a feed's date, returning the zero time when it cannot. An
// unreadable date is treated the same as a missing one: undated, rather than
// ancient.
func parseTime(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	for _, layout := range timeFormats {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// Recent returns the items published within maxAge of now, which is how a
// podcast's ten-year archive stops crowding out this morning's news.
//
// An item with no date is kept. A feed that does not say when something
// happened is not evidence that it happened long ago, and dropping undated
// items would silently empty every feed that omits them. maxAge of zero keeps
// everything.
func Recent(items []Item, maxAge time.Duration, now time.Time) []Item {
	if maxAge <= 0 {
		return items
	}
	cutoff := now.Add(-maxAge)
	out := make([]Item, 0, len(items))
	for _, it := range items {
		if it.Published.IsZero() || it.Published.After(cutoff) {
			out = append(out, it)
		}
	}
	return out
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
// maxAge drops items older than that; see [Recent]. A source left with nothing
// recent produces no group at all, rather than an empty one.
func (f *Fetcher) FetchAll(ctx context.Context, sources []Source, maxAge time.Duration, log *slog.Logger) []Group {
	out := make([]Group, 0, len(sources))
	for _, src := range sources {
		items, err := f.Fetch(ctx, src.URL)
		if err != nil {
			log.Error("could not read a feed, skipping it this time",
				"feed", src.URL, "name", src.Name, "error", err)
			continue
		}
		recent := Recent(items, maxAge, time.Now())
		if len(recent) == 0 {
			log.Info("a feed had nothing recent enough to say",
				"headlines", len(items), "name", src.Name, "feed", src.URL)
			continue
		}
		out = append(out, Group{Name: src.Name, Items: recent})
		log.Info("read a feed", "headlines", len(recent), "of", len(items),
			"name", src.Name, "feed", src.URL)
	}
	return out
}
