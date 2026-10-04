package textwrap

import (
	"strings"
	"testing"
)

// monoMeasurer is a stand-in for a real font: one unit per Latin rune, two per
// East Asian one, which is how the bundled bitmap font actually behaves.
type monoMeasurer struct{}

func (monoMeasurer) Advance(s string) float64 {
	w := 0.0
	for _, r := range s {
		if isCJK(r) {
			w += 2
		} else {
			w += 1
		}
	}
	return w
}

var m monoMeasurer

// wrap is WrapSpans for unstyled text, as plain lines. The rules for where a
// line may break are easiest to read and check this way.
func wrap(s string, m Measurer, maxWidth float64) []string {
	lines := WrapSpans([]Span{{Text: s}}, m, maxWidth)
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = text(l)
	}
	return out
}

// text is a line as a plain string, with the styling dropped.
func text(l Line) string {
	var b strings.Builder
	for _, r := range l.Runs {
		b.WriteString(r.Text)
	}
	return b.String()
}

func TestWrapKeepsExplicitLineBreaks(t *testing.T) {
	got := wrap("one\ntwo", m, 100)
	if len(got) != 2 || got[0] != "one" || got[1] != "two" {
		t.Errorf("wrap = %q, want [one two]", got)
	}
}

func TestWrapBreaksLatinAtSpaces(t *testing.T) {
	got := wrap("hello world", m, 7)
	if len(got) != 2 || got[0] != "hello" || got[1] != "world" {
		t.Errorf("wrap = %q, want [hello world]", got)
	}
}

func TestWrapDoesNotSplitAWordItCanAvoidSplitting(t *testing.T) {
	got := wrap("a bcdef", m, 6)
	if len(got) != 2 || got[0] != "a" || got[1] != "bcdef" {
		t.Errorf("wrap = %q, want [a bcdef]", got)
	}
}

func TestWrapBreaksJapaneseAnywhere(t *testing.T) {
	const s = "こんにちは世界"
	got := wrap(s, m, 6) // three full-width runes per line
	if len(got) != 3 {
		t.Fatalf("wrap = %q, want 3 lines", got)
	}
	if joined := strings.Join(got, ""); joined != s {
		t.Errorf("wrap lost or added text: %q, want %q", joined, s)
	}
	for _, line := range got {
		if w := m.Advance(line); w > 6 {
			t.Errorf("line %q is %v wide, over the limit of 6", line, w)
		}
	}
}

func TestWrapKeepsProhibitedRunesOffLineStart(t *testing.T) {
	// Without kinsoku handling the break would land before "、".
	got := wrap("ねこ、いぬ", m, 6)
	for _, line := range got {
		if line == "" {
			continue
		}
		if r := []rune(line)[0]; prohibitedAtLineStart(r) {
			t.Errorf("line %q starts with %q, which may not begin a line", line, r)
		}
	}
}

func TestWrapTerminatesWhenEveryRuneIsTooWide(t *testing.T) {
	got := wrap("abc", m, 0.5)
	if len(got) != 3 {
		t.Errorf("wrap = %q, want one rune per line", got)
	}
}

func TestWrapEmptyString(t *testing.T) {
	if got := wrap("", m, 10); len(got) != 1 || got[0] != "" {
		t.Errorf("wrap = %q, want one empty line", got)
	}
}

func TestBlockWidth(t *testing.T) {
	if got := BlockWidth([]string{"ab", "abcd", "a"}, m); got != 4 {
		t.Errorf("BlockWidth = %v, want 4", got)
	}
}

// describe renders a line as "text/style text/style", which makes a wrong
// split or a style that leaked across a boundary obvious in a failure message.
func describe(l Line) string {
	var parts []string
	for _, r := range l.Runs {
		parts = append(parts, r.Text+"/"+style(r.Style))
	}
	return strings.Join(parts, " ")
}

func style(s any) string {
	if s == nil {
		return "-"
	}
	return s.(string)
}

func describeAll(lines []Line) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = describe(l)
	}
	return out
}

func TestWrapSpansKeepsEachPieceWithItsStyle(t *testing.T) {
	got := WrapSpans([]Span{
		{Text: "see "},
		{Text: "here", Style: "link"},
		{Text: " now"},
	}, m, 100)

	want := []string{"see /- here/link  now/-"}
	if diff := describeAll(got); !equal(diff, want) {
		t.Errorf("WrapSpans = %q, want %q", diff, want)
	}
}

func TestWrapSpansSplitsOneSpanAcrossLines(t *testing.T) {
	// The styled span is too long for one line, so it has to be broken and the
	// style has to survive the break.
	got := WrapSpans([]Span{
		{Text: "aaa bbb ccc", Style: "link"},
	}, m, 7)

	want := []string{"aaa/link", "bbb ccc/link"}
	if diff := describeAll(got); !equal(diff, want) {
		t.Errorf("WrapSpans = %q, want %q", diff, want)
	}
}

// Styled text must break in the same places as the same text unstyled: a link
// in a message must not move where its lines end.
func TestStylingDoesNotMoveALineBreak(t *testing.T) {
	const s = "aaa bbb ccc 日本語のテキストが続く and then some latin"
	for _, width := range []float64{7, 12, 20, 33} {
		plain := wrap(s, m, width)
		var styled []string
		for _, l := range WrapSpans([]Span{{Text: s, Style: "link"}}, m, width) {
			styled = append(styled, text(l))
		}
		if !equal(plain, styled) {
			t.Errorf("width %v: unstyled breaks as %q, styled as %q", width, plain, styled)
		}
	}
}

func TestWrapSpansBreaksBetweenSpans(t *testing.T) {
	got := WrapSpans([]Span{
		{Text: "hello "},
		{Text: "world", Style: "link"},
	}, m, 7)

	want := []string{"hello/-", "world/link"}
	if diff := describeAll(got); !equal(diff, want) {
		t.Errorf("WrapSpans = %q, want %q", diff, want)
	}
}

func TestWrapSpansPlacesRunsLeftToRight(t *testing.T) {
	got := WrapSpans([]Span{
		{Text: "ab"},
		{Text: "cde", Style: "link"},
	}, m, 100)

	if len(got) != 1 || len(got[0].Runs) != 2 {
		t.Fatalf("WrapSpans = %q, want one line of two runs", describeAll(got))
	}
	line := got[0]
	if line.Runs[0].X != 0 || line.Runs[0].Width != 2 {
		t.Errorf("first run at X=%v width=%v, want X=0 width=2", line.Runs[0].X, line.Runs[0].Width)
	}
	if line.Runs[1].X != 2 || line.Runs[1].Width != 3 {
		t.Errorf("second run at X=%v width=%v, want X=2 width=3", line.Runs[1].X, line.Runs[1].Width)
	}
	if line.Width != 5 {
		t.Errorf("line width %v, want 5", line.Width)
	}
}

func TestWrapSpansKeepsExplicitBreaksInsideASpan(t *testing.T) {
	got := WrapSpans([]Span{{Text: "one\ntwo", Style: "link"}}, m, 100)

	want := []string{"one/link", "two/link"}
	if diff := describeAll(got); !equal(diff, want) {
		t.Errorf("WrapSpans = %q, want %q", diff, want)
	}
}

func TestWrapSpansGivesEmptyTextOneEmptyLine(t *testing.T) {
	got := WrapSpans(nil, m, 100)
	if len(got) != 1 || len(got[0].Runs) != 0 {
		t.Errorf("WrapSpans(nil) = %q, want a single empty line", describeAll(got))
	}
}

// A run's width has to agree with the width the wrapper broke the line at, or
// a balloon sized from one and drawn with the other is wrong.
func TestWrapSpansLineWidthIsTheSumOfItsRuns(t *testing.T) {
	lines := WrapSpans([]Span{
		{Text: "日本語の"},
		{Text: "テキスト", Style: "link"},
		{Text: " and latin"},
	}, m, 12)

	for i, l := range lines {
		sum := 0.0
		for _, r := range l.Runs {
			sum += r.Width
		}
		if sum != l.Width {
			t.Errorf("line %d: runs total %v, line says %v", i, sum, l.Width)
		}
		if l.Width > 12 {
			t.Errorf("line %d is %v wide, over the 12 it was wrapped to: %q", i, l.Width, describe(l))
		}
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
