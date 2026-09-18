// Package textwrap breaks message text into lines that fit a given width.
//
// It knows nothing about fonts or rendering: callers supply a Measurer, which
// keeps this logic testable without a graphics context. It knows nothing about
// styling either — a [Span] carries whatever the caller wants to remember about
// a piece of text, and this package only passes it through.
package textwrap

import (
	"strings"
	"unicode"
)

// Measurer reports how wide a string is when drawn.
type Measurer interface {
	Advance(s string) float64
}

// Span is a piece of text that shares one style. Style is opaque here: it is
// carried onto the [Run]s the text ends up in, and what it means is the
// caller's business.
type Span struct {
	Text  string
	Style any
}

// Run is the part of one line that came from a single span. A line is split
// into several runs when the styling changes partway along it.
type Run struct {
	Text  string
	Style any
	// X is the offset from the left edge of the line.
	X float64
	// Width is how wide Text is, by the same Measurer that laid it out.
	Width float64
}

// Line is one wrapped line.
type Line struct {
	Runs  []Run
	Width float64
}

// Text is the line as a plain string, with the styling dropped.
func (l Line) Text() string {
	var b strings.Builder
	for _, r := range l.Runs {
		b.WriteString(r.Text)
	}
	return b.String()
}

// Wrap breaks s into lines no wider than maxWidth. Explicit line breaks in s
// are kept, and a rune wider than maxWidth still gets a line of its own rather
// than looping forever.
func Wrap(s string, m Measurer, maxWidth float64) []string {
	lines := WrapSpans([]Span{{Text: s}}, m, maxWidth)
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = l.Text()
	}
	return out
}

// WrapSpans breaks styled text into lines no wider than maxWidth, keeping each
// piece's style with it. A line is broken wherever [Wrap] would break the same
// text: styling changes nothing about where a break may go.
func WrapSpans(spans []Span, m Measurer, maxWidth float64) []Line {
	var out []Line
	for _, para := range paragraphs(spans) {
		out = append(out, wrapParagraph(para, m, maxWidth)...)
	}
	return out
}

// BlockWidth is the width of the widest line.
func BlockWidth(lines []string, m Measurer) float64 {
	widest := 0.0
	for _, line := range lines {
		if w := m.Advance(line); w > widest {
			widest = w
		}
	}
	return widest
}

// BlockWidthOf is [BlockWidth] for lines that have already been measured.
func BlockWidthOf(lines []Line) float64 {
	widest := 0.0
	for _, l := range lines {
		if l.Width > widest {
			widest = l.Width
		}
	}
	return widest
}

// styled is one rune together with the style it inherited from its span.
// Wrapping works on these rather than on strings, because a break may land in
// the middle of a span and a span may hold several lines.
type styled struct {
	r     rune
	style any
}

// paragraphs flattens the spans to runes and cuts them at explicit line breaks.
// There is always at least one paragraph, so that empty text still produces an
// empty line rather than nothing at all.
func paragraphs(spans []Span) [][]styled {
	var flat []styled
	for _, s := range spans {
		for _, r := range strings.ReplaceAll(s.Text, "\r\n", "\n") {
			flat = append(flat, styled{r: r, style: s.Style})
		}
	}

	out := [][]styled{{}}
	for _, s := range flat {
		if s.r == '\n' {
			out = append(out, []styled{})
			continue
		}
		out[len(out)-1] = append(out[len(out)-1], s)
	}
	return out
}

// wrapParagraph fills a line greedily, then backs up to the last position a
// break was allowed at.
func wrapParagraph(p []styled, m Measurer, maxWidth float64) []Line {
	if len(p) == 0 {
		return []Line{{}}
	}

	var lines []Line
	for start := 0; start < len(p); {
		if len(lines) > 0 {
			// A line beginning with the spaces we broke on looks indented.
			for start < len(p) && p[start].r == ' ' {
				start++
			}
			if start >= len(p) {
				break
			}
		}

		width := 0.0
		end := start
		lastBreak := -1
		for end < len(p) {
			w := m.Advance(string(p[end].r))
			if end > start && width+w > maxWidth {
				break
			}
			width += w
			end++
			if end < len(p) && breakable(p[end-1].r, p[end].r) {
				lastBreak = end
			}
		}
		if end < len(p) && lastBreak > start {
			end = lastBreak
		}
		lines = append(lines, buildLine(p[start:end], m))
		start = end
	}
	return lines
}

// buildLine groups a line's runes into runs of one style each, dropping the
// trailing spaces it was broken on so they do not pad the balloon out.
func buildLine(rs []styled, m Measurer) Line {
	for len(rs) > 0 && rs[len(rs)-1].r == ' ' {
		rs = rs[:len(rs)-1]
	}

	var line Line
	for i := 0; i < len(rs); {
		j := i
		var b strings.Builder
		for j < len(rs) && rs[j].style == rs[i].style {
			b.WriteRune(rs[j].r)
			j++
		}
		text := b.String()
		// Measured rune by rune, the way the fill loop above measured it, so
		// that a run's width and the width it was wrapped at agree.
		width := 0.0
		for _, r := range text {
			width += m.Advance(string(r))
		}
		line.Runs = append(line.Runs, Run{
			Text:  text,
			Style: rs[i].style,
			X:     line.Width,
			Width: width,
		})
		line.Width += width
		i = j
	}
	return line
}

// breakable reports whether a line may be broken between prev and next.
// Latin text breaks at spaces; CJK text breaks almost anywhere, except right
// before a character that may not start a line.
func breakable(prev, next rune) bool {
	if prev == ' ' {
		return true
	}
	if isCJK(prev) || isCJK(next) {
		return !prohibitedAtLineStart(next)
	}
	return false
}

func isCJK(r rune) bool {
	switch {
	case unicode.Is(unicode.Han, r),
		unicode.Is(unicode.Hiragana, r),
		unicode.Is(unicode.Katakana, r),
		unicode.Is(unicode.Hangul, r):
		return true
	}
	// CJK symbols and punctuation, and fullwidth forms, which the tables above
	// do not cover.
	return (r >= 0x3000 && r <= 0x303F) || (r >= 0xFF00 && r <= 0xFF60)
}

// prohibitedAtLineStart lists the characters Japanese typesetting keeps off the
// head of a line (kinsoku shori).
func prohibitedAtLineStart(r rune) bool {
	return strings.ContainsRune("、。，．・：；？！ヽヾゝゞーァィゥェォッャュョ々〜）〕］｝〉》」』】’”)]},.:;?!", r)
}
