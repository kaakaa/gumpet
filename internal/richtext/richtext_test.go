package richtext

import (
	"strings"
	"testing"
)

// monoMeasurer is a stand-in for a real font: one unit per Latin rune.
type monoMeasurer struct{}

func (monoMeasurer) Advance(s string) float64 { return float64(len([]rune(s))) }

var m monoMeasurer

// describe renders spans as "text" or "text->url", so a link that swallowed
// the wrong characters is visible in the failure message.
func describe(spans []Span) []string {
	out := make([]string, len(spans))
	for i, s := range spans {
		if s.Style.IsLink() {
			out[i] = s.Text + "->" + s.Style.Link
			continue
		}
		out[i] = s.Text
	}
	return out
}

func TestParseLeavesPlainTextAlone(t *testing.T) {
	got := describe(Parse("nothing to see here"))
	if len(got) != 1 || got[0] != "nothing to see here" {
		t.Errorf("Parse = %q, want one plain span", got)
	}
}

func TestParseFindsAURLInTheMiddleOfASentence(t *testing.T) {
	got := describe(Parse("see https://example.com/x now"))
	want := []string{"see ", "https://example.com/x->https://example.com/x", " now"}
	if !equal(got, want) {
		t.Errorf("Parse = %q, want %q", got, want)
	}
}

func TestParseFindsSeveralURLs(t *testing.T) {
	got := describe(Parse("http://a.example http://b.example"))
	if len(got) != 3 {
		t.Fatalf("Parse = %q, want two links with a space between", got)
	}
	if got[0] != "http://a.example->http://a.example" || got[2] != "http://b.example->http://b.example" {
		t.Errorf("Parse = %q, want both links marked", got)
	}
}

// The end of a URL is where most autolinkers go wrong, and a link that eats the
// sentence's punctuation opens the wrong page.
func TestParseKeepsSentencePunctuationOutOfTheLink(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"go to https://example.com.", "https://example.com"},
		{"go to https://example.com、", "https://example.com"},
		{"(see https://example.com)", "https://example.com"},
		{"「https://example.com」", "https://example.com"},
		{"https://example.com/a?b=c", "https://example.com/a?b=c"},
		{"https://example.com/path/", "https://example.com/path/"},
		{"https://example.com/a_(b)", "https://example.com/a_(b"},
	}
	for _, c := range cases {
		var link string
		for _, s := range Parse(c.in) {
			if s.Style.IsLink() {
				link = s.Style.Link
			}
		}
		if link != c.want {
			t.Errorf("Parse(%q) linked %q, want %q", c.in, link, c.want)
		}
	}
}

func TestParseIgnoresSchemesItWillNotOpen(t *testing.T) {
	for _, in := range []string{
		"javascript:alert(1)",
		"file:///etc/passwd",
		"data:text/html,<script>",
		"ftp://example.com",
		"steam://run/1234",
	} {
		for _, s := range Parse("look at " + in) {
			if s.Style.IsLink() {
				t.Errorf("Parse(%q) made a link of %q", in, s.Style.Link)
			}
		}
	}
}

// Whatever Parse marks as a link must be something Openable agrees to open, or
// the pet draws a link that does nothing when clicked.
func TestEverythingParseLinksIsOpenable(t *testing.T) {
	text := "a https://example.com b http://x.example/y?z=1 c javascript:no d ftp://no"
	for _, s := range Parse(text) {
		if s.Style.IsLink() && !Openable(s.Style.Link) {
			t.Errorf("Parse linked %q, which Openable refuses", s.Style.Link)
		}
	}
}

func TestOpenableRefusesAnythingButHTTP(t *testing.T) {
	yes := []string{"http://example.com", "https://example.com", "HTTPS://EXAMPLE.COM"}
	no := []string{
		"javascript:alert(1)",
		"file:///etc/passwd",
		"data:text/html,x",
		"ftp://example.com",
		"steam://run/1",
		"example.com",
		"",
		" https://example.com",
		"https:/example.com",
	}
	for _, u := range yes {
		if !Openable(u) {
			t.Errorf("Openable(%q) = false, want true", u)
		}
	}
	for _, u := range no {
		if Openable(u) {
			t.Errorf("Openable(%q) = true, want false", u)
		}
	}
}

func TestWrapKeepsTheLinkWithItsRunsWhenItBreaks(t *testing.T) {
	spans := Parse("see https://example.com/a/very/long/path now")
	lines := Wrap(spans, m, 12)

	if len(lines) < 2 {
		t.Fatalf("Wrap gave %d line(s), want the link broken over several", len(lines))
	}
	// Every piece of the URL, wherever it ended up, still knows where it goes.
	var linked strings.Builder
	for _, l := range lines {
		for _, r := range l.Runs {
			if !r.Style.IsLink() {
				continue
			}
			linked.WriteString(r.Text)
			if r.Style.Link != "https://example.com/a/very/long/path" {
				t.Errorf("run %q points at %q", r.Text, r.Style.Link)
			}
		}
	}
	if got := linked.String(); got != "https://example.com/a/very/long/path" {
		t.Errorf("the link's runs spell %q, want the whole URL", got)
	}
}

func TestWrapReportsWidthsThatAddUp(t *testing.T) {
	lines := Wrap(Parse("see https://example.com/x now please"), m, 15)
	for i, l := range lines {
		sum := 0.0
		for _, r := range l.Runs {
			if r.X != sum {
				t.Errorf("line %d: run %q starts at %v, want %v", i, r.Text, r.X, sum)
			}
			sum += r.Width
		}
		if sum != l.Width {
			t.Errorf("line %d: runs total %v, line says %v", i, sum, l.Width)
		}
		if l.Width > 15 {
			t.Errorf("line %d is %v wide, over the 15 it was wrapped to", i, l.Width)
		}
	}
}

func TestBlockWidthIsTheWidestLine(t *testing.T) {
	lines := Wrap([]Span{{Text: "ab\nabcd\nabc"}}, m, 100)
	if got := BlockWidth(lines); got != 4 {
		t.Errorf("BlockWidth = %v, want 4", got)
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

func TestRunesCountsTheWholeBlock(t *testing.T) {
	lines := Wrap(Parse("see https://example.com/x now"), m, 100)
	if got, want := Runes(lines), len([]rune("see https://example.com/x now")); got != want {
		t.Errorf("Runes = %d, want %d", got, want)
	}
}

func TestRunesCountsEachLineOfAWrappedBlock(t *testing.T) {
	lines := Wrap([]Span{{Text: "abc\ndef"}}, m, 100)
	if got := Runes(lines); got != 6 {
		t.Errorf("Runes = %d, want 6 — the newline is not a character on any line", got)
	}
}

func TestRevealGivesBackTheFirstNCharacters(t *testing.T) {
	lines := Wrap([]Span{{Text: "abcdefghij"}}, m, 100)
	for n := 0; n <= 10; n++ {
		got := Reveal(lines, n, m)
		var b strings.Builder
		for _, l := range got {
			b.WriteString(l.Text())
		}
		if want := "abcdefghij"[:n]; b.String() != want {
			t.Errorf("Reveal(%d) = %q, want %q", n, b.String(), want)
		}
	}
}

// The whole point: revealing must not move anything. A word that will end up
// on the second line starts on the second line.
func TestRevealKeepsEveryRunWhereItWillEndUp(t *testing.T) {
	full := Wrap(Parse("see https://example.com/abc now please"), m, 15)
	total := Runes(full)

	for n := 1; n <= total; n++ {
		got := Reveal(full, n, m)
		if len(got) > len(full) {
			t.Fatalf("Reveal(%d) produced %d lines, more than the %d it wraps to", n, len(got), len(full))
		}
		for i, l := range got {
			for j, r := range l.Runs {
				if r.X != full[i].Runs[j].X {
					t.Errorf("Reveal(%d) line %d run %d moved to X=%v, want %v",
						n, i, j, r.X, full[i].Runs[j].X)
				}
			}
		}
	}
}

// A link only half typed out still has to have its underline the width of the
// characters actually on screen.
func TestRevealRemeasuresTheRunItStopsInside(t *testing.T) {
	lines := Wrap(Parse("go https://example.com now"), m, 100)
	// Far enough in to be partway through the URL.
	got := Reveal(lines, 10, m)

	var last Run
	for _, l := range got {
		if len(l.Runs) > 0 {
			last = l.Runs[len(l.Runs)-1]
		}
	}
	if want := m.Advance(last.Text); last.Width != want {
		t.Errorf("partial run %q has width %v, want %v", last.Text, last.Width, want)
	}
	if last.Width == 0 && last.Text != "" {
		t.Error("partial run has no width at all")
	}
}

func TestRevealKeepsTheStyleOfAPartialRun(t *testing.T) {
	lines := Wrap(Parse("go https://example.com now"), m, 100)
	got := Reveal(lines, 10, m)

	var sawLink bool
	for _, l := range got {
		for _, r := range l.Runs {
			if r.Style.IsLink() {
				sawLink = true
				if r.Style.Link != "https://example.com" {
					t.Errorf("partial link points at %q", r.Style.Link)
				}
			}
		}
	}
	if !sawLink {
		t.Error("the half-revealed link lost its style")
	}
}

func TestRevealOfEverythingIsEverything(t *testing.T) {
	lines := Wrap(Parse("see https://example.com/x now"), m, 12)
	for _, n := range []int{Runes(lines), Runes(lines) + 1, 9999} {
		got := Reveal(lines, n, m)
		if len(got) != len(lines) {
			t.Errorf("Reveal(%d) gave %d lines, want all %d", n, len(got), len(lines))
		}
	}
}

func TestRevealOfNothingIsNothing(t *testing.T) {
	lines := Wrap([]Span{{Text: "abc"}}, m, 100)
	if got := Reveal(lines, 0, m); Runes(got) != 0 {
		t.Errorf("Reveal(0) showed %d runes, want none", Runes(got))
	}
}
