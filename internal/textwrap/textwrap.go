// Package textwrap breaks message text into lines that fit a given width.
//
// It knows nothing about fonts or rendering: callers supply a Measurer, which
// keeps this logic testable without a graphics context.
package textwrap

import (
	"strings"
	"unicode"
)

// Measurer reports how wide a string is when drawn.
type Measurer interface {
	Advance(s string) float64
}

// Wrap breaks s into lines no wider than maxWidth. Explicit line breaks in s
// are kept, and a rune wider than maxWidth still gets a line of its own rather
// than looping forever.
func Wrap(s string, m Measurer, maxWidth float64) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	var out []string
	for _, para := range strings.Split(s, "\n") {
		out = append(out, wrapParagraph(para, m, maxWidth)...)
	}
	return out
}

// wrapParagraph fills a line greedily, then backs up to the last position a
// break was allowed at.
func wrapParagraph(p string, m Measurer, maxWidth float64) []string {
	runes := []rune(p)
	if len(runes) == 0 {
		return []string{""}
	}

	var lines []string
	for start := 0; start < len(runes); {
		if len(lines) > 0 {
			// A line beginning with the spaces we broke on looks indented.
			for start < len(runes) && runes[start] == ' ' {
				start++
			}
			if start >= len(runes) {
				break
			}
		}

		width := 0.0
		end := start
		lastBreak := -1
		for end < len(runes) {
			w := m.Advance(string(runes[end]))
			if end > start && width+w > maxWidth {
				break
			}
			width += w
			end++
			if end < len(runes) && breakable(runes[end-1], runes[end]) {
				lastBreak = end
			}
		}
		if end < len(runes) && lastBreak > start {
			end = lastBreak
		}
		lines = append(lines, strings.TrimRight(string(runes[start:end]), " "))
		start = end
	}
	return lines
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
