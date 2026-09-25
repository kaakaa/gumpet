package feed

import (
	"bytes"
	"io"
	"regexp"
	"sync"
	"time"
	"unicode/utf8"
)

// Report is what the last attempt to read one feed came to.
//
// A feed that stops producing headlines looks, from the pet, exactly like a
// quiet one. The messages page shows a report per feed so the two can be told
// apart — when it was last read, whether that worked, how much it had — and
// keeps what the feed actually served, for when "it worked but found nothing"
// needs looking into.
type Report struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	// FetchedAt is when it was last read, or the zero time if it has not been
	// yet.
	FetchedAt time.Time `json:"fetched_at,omitzero"`
	// Error is why the last read failed, and empty when it did not.
	Error string `json:"error,omitempty"`
	// Status is the HTTP status the server answered with, when it answered.
	Status string `json:"status,omitempty"`
	// Found is how many headlines the feed held, and Recent how many of those
	// were new enough to say. See [Recent].
	Found  int `json:"found"`
	Recent int `json:"recent"`
	// Size is how many bytes it served.
	Size int `json:"size"`
	// HasSource says there is a body to show: a feed that could not be
	// reached at all served nothing.
	HasSource bool `json:"has_source"`

	body []byte
}

// Reports keeps the latest Report for each feed. It is written by whatever
// reads the feeds and read by the server, from different goroutines.
type Reports struct {
	mu    sync.Mutex
	byURL map[string]Report
}

// NewReports returns an empty set.
func NewReports() *Reports { return &Reports{byURL: map[string]Report{}} }

// put records r, replacing whatever the last read of the same feed said.
func (rs *Reports) put(r Report) {
	if rs == nil {
		return
	}
	r.HasSource = r.body != nil
	r.Size = len(r.body)
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.byURL[r.URL] = r
}

// For returns a report for each of sources, in their order. It follows the
// configuration rather than history: a feed taken out of the list is not
// reported on, and one just added is reported as not read yet.
func (rs *Reports) For(sources []Source) []Report {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	out := make([]Report, 0, len(sources))
	for _, s := range sources {
		r, ok := rs.byURL[s.URL]
		if !ok {
			r = Report{URL: s.URL}
		}
		// The name is today's, not the one it had when it was read.
		r.Name = s.Name
		out = append(out, r)
	}
	return out
}

// Source returns what the feed at url last served, as text. False means there
// is nothing to show: it has not been read, or it could not be reached.
func (rs *Reports) Source(url string) (string, bool) {
	rs.mu.Lock()
	r, ok := rs.byURL[url]
	rs.mu.Unlock()
	if !ok || r.body == nil {
		return "", false
	}
	return Readable(r.body), true
}

var declaredEncoding = regexp.MustCompile(`^\s*<\?xml[^>]*\bencoding\s*=\s*["']([^"']+)["']`)

// Readable turns a feed's bytes into text a page can show. A feed in Shift_JIS
// or Latin-1 is decoded by the encoding it declares, the same way it is
// decoded to be parsed, so its source reads as it was meant to. Bytes that
// cannot be made sense of are shown as they are, with the invalid parts
// replaced, rather than not at all: seeing what went wrong is the point.
func Readable(body []byte) string {
	if m := declaredEncoding.FindSubmatch(body[:min(len(body), 200)]); m != nil {
		if r, err := charsetReader(string(m[1]), bytes.NewReader(body)); err == nil {
			if text, err := io.ReadAll(r); err == nil {
				return string(text)
			}
		}
	}
	if utf8.Valid(body) {
		return string(body)
	}
	return string(bytes.ToValidUTF8(body, []byte("�")))
}
