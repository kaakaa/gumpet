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

func TestWrapKeepsExplicitLineBreaks(t *testing.T) {
	got := Wrap("one\ntwo", m, 100)
	if len(got) != 2 || got[0] != "one" || got[1] != "two" {
		t.Errorf("Wrap = %q, want [one two]", got)
	}
}

func TestWrapBreaksLatinAtSpaces(t *testing.T) {
	got := Wrap("hello world", m, 7)
	if len(got) != 2 || got[0] != "hello" || got[1] != "world" {
		t.Errorf("Wrap = %q, want [hello world]", got)
	}
}

func TestWrapDoesNotSplitAWordItCanAvoidSplitting(t *testing.T) {
	got := Wrap("a bcdef", m, 6)
	if len(got) != 2 || got[0] != "a" || got[1] != "bcdef" {
		t.Errorf("Wrap = %q, want [a bcdef]", got)
	}
}

func TestWrapBreaksJapaneseAnywhere(t *testing.T) {
	const s = "こんにちは世界"
	got := Wrap(s, m, 6) // three full-width runes per line
	if len(got) != 3 {
		t.Fatalf("Wrap = %q, want 3 lines", got)
	}
	if joined := strings.Join(got, ""); joined != s {
		t.Errorf("Wrap lost or added text: %q, want %q", joined, s)
	}
	for _, line := range got {
		if w := m.Advance(line); w > 6 {
			t.Errorf("line %q is %v wide, over the limit of 6", line, w)
		}
	}
}

func TestWrapKeepsProhibitedRunesOffLineStart(t *testing.T) {
	// Without kinsoku handling the break would land before "、".
	got := Wrap("ねこ、いぬ", m, 6)
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
	got := Wrap("abc", m, 0.5)
	if len(got) != 3 {
		t.Errorf("Wrap = %q, want one rune per line", got)
	}
}

func TestWrapEmptyString(t *testing.T) {
	if got := Wrap("", m, 10); len(got) != 1 || got[0] != "" {
		t.Errorf("Wrap = %q, want one empty line", got)
	}
}

func TestBlockWidth(t *testing.T) {
	if got := BlockWidth([]string{"ab", "abcd", "a"}, m); got != 4 {
		t.Errorf("BlockWidth = %v, want 4", got)
	}
}
