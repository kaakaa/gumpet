// Package richtext works out which parts of a message get drawn differently
// from the rest, and lays them out into lines.
//
// The point of it is that the sender writes nothing special. A URL is spotted
// because it looks like a URL, not because anyone marked it up, so a program
// that just pastes a link gets a working link for free.
//
// It imports nothing that draws, so all of this is testable without a screen.
package richtext

import (
	"regexp"
	"strings"

	"github.com/kaakaa/gumpet/internal/textwrap"
)

// Style is how one piece of a message differs from plain text. It is a struct
// with one field rather than an enum because more ways to differ are coming,
// and they combine: a link can also be bold.
type Style struct {
	// Link is the URL to open when this piece is clicked. Empty means the
	// piece is ordinary text.
	Link string
}

// IsLink reports whether this piece is worth drawing as one.
func (s Style) IsLink() bool { return s.Link != "" }

// Span is a piece of a message that shares one style.
type Span struct {
	Text  string
	Style Style
}

// Run is the part of one line that came from a single span.
type Run struct {
	Text  string
	Style Style
	// X is the offset from the left edge of the line.
	X float64
	// Width is how wide Text is.
	Width float64
}

// Line is one wrapped line of a message.
type Line struct {
	Runs  []Run
	Width float64
}

// Text is the line as a plain string.
func (l Line) Text() string {
	var b strings.Builder
	for _, r := range l.Runs {
		b.WriteString(r.Text)
	}
	return b.String()
}

// urlPattern is deliberately blunt: it takes everything up to whitespace and
// lets [trimTrailing] sort out the end. Recognising exactly the characters a
// URL may contain is a losing game, and getting it slightly wrong at the end
// of a sentence is cheap to fix and expensive to prevent.
//
// Only http and https are matched. See [Openable] for why.
var urlPattern = regexp.MustCompile(`https?://[^\s]+`)

// Parse finds the parts of s worth drawing differently. Text that is not
// special comes back as a span with the zero Style, so the whole message is
// always covered and nothing has to be reassembled by the caller.
func Parse(s string) []Span {
	var spans []Span
	at := 0
	for _, loc := range urlPattern.FindAllStringIndex(s, -1) {
		start, end := loc[0], loc[1]
		// Punctuation swept up by the pattern above belongs to the sentence,
		// not to the link, and must not end up inside the clickable area.
		end = start + len(trimTrailing(s[start:end]))

		if start > at {
			spans = append(spans, Span{Text: s[at:start]})
		}
		url := s[start:end]
		spans = append(spans, Span{Text: url, Style: Style{Link: url}})
		at = end
	}
	if at < len(s) {
		spans = append(spans, Span{Text: s[at:]})
	}
	if len(spans) == 0 {
		spans = append(spans, Span{Text: s})
	}
	return spans
}

// trailingJunk is punctuation far likelier to belong to the sentence around a
// URL than to the URL itself. A closing bracket is included because a link is
// so often put in brackets; a link that really ends in one loses a character,
// which beats a sentence's full stop becoming part of every link.
const trailingJunk = `.,;:!?'"。、）)]}>»」』】`

func trimTrailing(url string) string {
	return strings.TrimRight(url, trailingJunk)
}

// Openable reports whether a URL is one gumpet will hand to the browser.
//
// Only http and https. Anything that can be sent a message can put a link in
// front of the person at this desktop, so the schemes that do something other
// than fetch a page — javascript:, file:, and whatever an installed
// application has registered for itself — are not on offer. Clicking is
// required either way, but a click should not be able to do more than open a
// web page.
func Openable(url string) bool {
	lower := strings.ToLower(url)
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}

// Wrap breaks spans into lines no wider than maxWidth, keeping each piece's
// style with it. The line breaking itself is [textwrap]'s, so styled and plain
// text break in exactly the same places.
func Wrap(spans []Span, m textwrap.Measurer, maxWidth float64) []Line {
	in := make([]textwrap.Span, len(spans))
	for i, s := range spans {
		in[i] = textwrap.Span{Text: s.Text, Style: s.Style}
	}

	wrapped := textwrap.WrapSpans(in, m, maxWidth)
	out := make([]Line, len(wrapped))
	for i, l := range wrapped {
		line := Line{Width: l.Width, Runs: make([]Run, len(l.Runs))}
		for j, r := range l.Runs {
			style, _ := r.Style.(Style)
			line.Runs[j] = Run{Text: r.Text, Style: style, X: r.X, Width: r.Width}
		}
		out[i] = line
	}
	return out
}

// Runes counts the characters in a block of lines, which is how far a
// typewriter effect has to get before the whole thing is on screen.
func Runes(lines []Line) int {
	n := 0
	for _, l := range lines {
		for _, r := range l.Runs {
			n += len([]rune(r.Text))
		}
	}
	return n
}

// Reveal returns the first n runes of a block, laid out exactly where they
// were. Lines and runs keep the offsets the wrapper gave them, so revealing
// text a character at a time never re-wraps it: the words stay where they will
// end up, and the balloon around them never has to change size.
//
// The run a reveal stops inside has to be measured again, since its width is
// what the underline under a link is drawn from.
func Reveal(lines []Line, n int, m textwrap.Measurer) []Line {
	if n >= Runes(lines) {
		return lines
	}
	out := make([]Line, 0, len(lines))
	left := n
	for _, l := range lines {
		if left <= 0 {
			break
		}
		cut := Line{Width: l.Width, Runs: make([]Run, 0, len(l.Runs))}
		for _, r := range l.Runs {
			if left <= 0 {
				break
			}
			runes := []rune(r.Text)
			if len(runes) <= left {
				cut.Runs = append(cut.Runs, r)
				left -= len(runes)
				continue
			}
			text := string(runes[:left])
			cut.Runs = append(cut.Runs, Run{
				Text:  text,
				Style: r.Style,
				X:     r.X,
				Width: m.Advance(text),
			})
			left = 0
		}
		out = append(out, cut)
	}
	return out
}

// BlockWidth is the width of the widest line.
func BlockWidth(lines []Line) float64 {
	widest := 0.0
	for _, l := range lines {
		if l.Width > widest {
			widest = l.Width
		}
	}
	return widest
}
