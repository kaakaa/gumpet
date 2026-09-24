package quiet

import (
	"testing"
	"time"

	"github.com/kaakaa/gumpet/internal/lang"
)

// at builds a time of day on an arbitrary date, in UTC so that the test reads
// the same everywhere; Contains reads the time in its own location.
func at(hh, mm int) time.Time { return time.Date(2026, 9, 23, hh, mm, 0, 0, time.UTC) }

func mustParse(t *testing.T, from, to string) Window {
	t.Helper()
	w, err := Parse(from, to)
	if err != nil {
		t.Fatalf("Parse(%q, %q): %v", from, to, err)
	}
	return w
}

func TestContainsWithinOneDay(t *testing.T) {
	w := mustParse(t, "13:00", "14:30")
	cases := []struct {
		t    time.Time
		want bool
	}{
		{at(12, 59), false},
		{at(13, 0), true}, // from is inclusive
		{at(14, 29), true},
		{at(14, 30), false}, // to is not: "until 14:30" is over at 14:30
		{at(23, 0), false},
	}
	for _, c := range cases {
		if got := w.Contains(c.t); got != c.want {
			t.Errorf("13:00–14:30 contains %s = %v, want %v", c.t.Format("15:04"), got, c.want)
		}
	}
}

// The window people actually set is the night, and it crosses midnight.
func TestContainsAcrossMidnight(t *testing.T) {
	w := mustParse(t, "22:00", "08:30")
	cases := []struct {
		t    time.Time
		want bool
	}{
		{at(21, 59), false},
		{at(22, 0), true},
		{at(23, 59), true},
		{at(0, 0), true}, // midnight itself
		{at(3, 0), true},
		{at(8, 29), true},
		{at(8, 30), false},
		{at(12, 0), false},
	}
	for _, c := range cases {
		if got := w.Contains(c.t); got != c.want {
			t.Errorf("22:00–08:30 contains %s = %v, want %v", c.t.Format("15:04"), got, c.want)
		}
	}
}

func TestAWindowStartingAtMidnight(t *testing.T) {
	w := mustParse(t, "00:00", "06:00")
	if !w.Contains(at(0, 0)) || w.Contains(at(6, 0)) || w.Contains(at(23, 59)) {
		t.Error("00:00–06:00 should hold midnight, and neither 06:00 nor 23:59")
	}
}

// No window must mean exactly today's behaviour: never quiet.
func TestNoWindowIsNeverQuiet(t *testing.T) {
	cases := [][2]string{{"", ""}, {"09:00", "09:00"}, {" ", " "}}
	for _, c := range cases {
		w := mustParse(t, c[0], c[1])
		if w.Set() {
			t.Errorf("Parse(%q, %q) is set, want no window", c[0], c[1])
		}
		for h := range 24 {
			if w.Contains(at(h, 0)) {
				t.Errorf("Parse(%q, %q) contains %02d:00, want never", c[0], c[1], h)
			}
		}
	}
}

func TestParseAcceptsHowPeopleWriteTimes(t *testing.T) {
	cases := [][2]string{{"22:00", "08:30"}, {"8:30", "9:15"}, {"00:00", "23:59"}}
	for _, c := range cases {
		if _, err := Parse(c[0], c[1]); err != nil {
			t.Errorf("Parse(%q, %q): %v", c[0], c[1], err)
		}
	}
}

func TestParseRejectsWhatIsNotATime(t *testing.T) {
	cases := [][2]string{
		{"22:00", ""}, // half a window
		{"", "08:30"},
		{"24:00", "08:00"},
		{"22:60", "08:00"},
		{"22", "08:00"},
		{"22:0", "08:00"},
		{"ten", "08:00"},
		{"-1:00", "08:00"},
		{"123:00", "08:00"},
	}
	for _, c := range cases {
		if _, err := Parse(c[0], c[1]); err == nil {
			t.Errorf("Parse(%q, %q) succeeded, want an error", c[0], c[1])
		}
	}
}

// The whole sequence: quiet begins, messages are held, it ends, and the count
// comes out once.
func TestHushReportsTheEndOnceWithTheCount(t *testing.T) {
	var h Hush
	h.SetWindow(mustParse(t, "22:00", "08:30"))

	if quiet, ended := h.Update(at(21, 0)); quiet || ended != 0 {
		t.Fatalf("21:00: quiet=%v ended=%d, want neither", quiet, ended)
	}
	if quiet, _ := h.Update(at(22, 0)); !quiet {
		t.Fatal("22:00: not quiet")
	}
	for range 12 {
		h.Hold()
	}
	if quiet, ended := h.Update(at(3, 0)); !quiet || ended != 0 {
		t.Fatalf("03:00: quiet=%v ended=%d, want still quiet and nothing ended", quiet, ended)
	}
	if quiet, ended := h.Update(at(8, 30)); quiet || ended != 12 {
		t.Fatalf("08:30: quiet=%v ended=%d, want over, with 12", quiet, ended)
	}
	// Said once, not every frame after.
	if _, ended := h.Update(at(8, 31)); ended != 0 {
		t.Errorf("08:31: ended=%d again, want the count reported only once", ended)
	}
}

// Nothing arrived, so nothing is said.
func TestHushSaysNothingAfterAnEmptyNight(t *testing.T) {
	var h Hush
	h.SetWindow(mustParse(t, "22:00", "08:30"))
	h.Update(at(23, 0))
	if _, ended := h.Update(at(9, 0)); ended != 0 {
		t.Errorf("ended=%d after nothing arrived, want 0", ended)
	}
}

// Starting gumpet in the middle of the night is quiet from the first frame,
// and nothing "ended" just because there was no previous frame.
func TestHushStartingInsideTheWindow(t *testing.T) {
	var h Hush
	h.SetWindow(mustParse(t, "22:00", "08:30"))
	if quiet, ended := h.Update(at(2, 0)); !quiet || ended != 0 {
		t.Errorf("first update at 02:00: quiet=%v ended=%d, want quiet and nothing ended", quiet, ended)
	}
}

// Taking the window away while messages are held ends the quiet the way the
// clock would have, so they are not silently forgotten.
func TestHushRemovingTheWindowEndsIt(t *testing.T) {
	var h Hush
	h.SetWindow(mustParse(t, "22:00", "08:30"))
	h.Update(at(23, 0))
	h.Hold()
	h.Hold()

	h.SetWindow(Window{})
	if quiet, ended := h.Update(at(23, 1)); quiet || ended != 2 {
		t.Errorf("after removing the window: quiet=%v ended=%d, want over, with 2", quiet, ended)
	}
}

// A gumpet asleep across the whole window — a laptop lid closed at 21:00 and
// opened at 09:00 — never saw it start, and holds nothing, so says nothing.
func TestHushThatNeverSawTheWindow(t *testing.T) {
	var h Hush
	h.SetWindow(mustParse(t, "22:00", "08:30"))
	h.Update(at(21, 0))
	if quiet, ended := h.Update(at(9, 0)); quiet || ended != 0 {
		t.Errorf("quiet=%v ended=%d, want neither", quiet, ended)
	}
}

func TestSummaryIsOneSentenceWithWhereToRead(t *testing.T) {
	cases := []struct {
		n    int
		want string
	}{
		{1, "1 message arrived while it was quiet.\nhttp://127.0.0.1:8765/messages"},
		{12, "12 messages arrived while it was quiet.\nhttp://127.0.0.1:8765/messages"},
	}
	for _, c := range cases {
		if got := Summary(c.n, "http://127.0.0.1:8765/messages", lang.English.T); got != c.want {
			t.Errorf("Summary(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}

// The summary is the pet's own sentence, so it is said in the pet's language;
// the link to the page is left as it is.
func TestSummaryInJapanese(t *testing.T) {
	got := Summary(12, "http://127.0.0.1:8765/messages", lang.Japanese.T)
	want := "静かにしている間に 12 件のメッセージが届きました。\nhttp://127.0.0.1:8765/messages"
	if got != want {
		t.Errorf("Summary = %q, want %q", got, want)
	}
}
