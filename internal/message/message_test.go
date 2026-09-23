package message

import (
	"testing"
	"time"
)

func TestParseLevelAcceptsTheLevelsThatExist(t *testing.T) {
	cases := map[string]Level{
		"info":    LevelInfo,
		"success": LevelSuccess,
		"warn":    LevelWarn,
		"error":   LevelError,
		"ERROR":   LevelError,
		"  warn ": LevelWarn,
	}
	for in, want := range cases {
		if got := ParseLevel(in); got != want {
			t.Errorf("ParseLevel(%q) = %q, want %q", in, got, want)
		}
	}
}

// A sender that makes up a level should still get its message shown, because
// the alternative is dropping a message over a cosmetic field.
func TestParseLevelFallsBackToInfo(t *testing.T) {
	for _, in := range []string{"", "critical", "warning", "URGENT!!", "1"} {
		if got := ParseLevel(in); got != LevelInfo {
			t.Errorf("ParseLevel(%q) = %q, want info", in, got)
		}
	}
}

// The stamp is the only part of a balloon that has to decide anything, so it
// is the part worth pinning down: the drawing around it cannot be tested at
// all. Times are built in the local zone so that the test says the same thing
// wherever it is run.
func TestStampShowsTheDateOnlyWhenTheClockIsNotEnough(t *testing.T) {
	now := time.Date(2026, 9, 20, 14, 30, 0, 0, time.Local)
	cases := []struct {
		name string
		at   time.Time
		want string
	}{
		{"earlier today", time.Date(2026, 9, 20, 9, 5, 0, 0, time.Local), "09:05"},
		{"a minute ago", time.Date(2026, 9, 20, 14, 29, 0, 0, time.Local), "14:29"},
		{"just after midnight", time.Date(2026, 9, 20, 0, 0, 0, 0, time.Local), "00:00"},
		{"yesterday", time.Date(2026, 9, 19, 23, 59, 0, 0, time.Local), "9/19 23:59"},
		{"earlier this year", time.Date(2026, 1, 2, 8, 0, 0, 0, time.Local), "1/2 08:00"},
		{"last year", time.Date(2025, 12, 31, 22, 15, 0, 0, time.Local), "2025/12/31 22:15"},
		// A feed that dates an entry a day ahead, or a clock that disagrees
		// with the server's, is not worth a special case: it is shown.
		{"tomorrow", time.Date(2026, 9, 21, 1, 0, 0, 0, time.Local), "9/21 01:00"},
	}
	for _, c := range cases {
		if got := Stamp(c.at, now); got != c.want {
			t.Errorf("%s: Stamp = %q, want %q", c.name, got, c.want)
		}
	}
}

// A saying out of a file happened nowhere in particular, and a balloon with
// "0001/1/1" in the corner would be worse than one with nothing.
func TestStampSaysNothingAboutTheZeroTime(t *testing.T) {
	if got := Stamp(time.Time{}, time.Now()); got != "" {
		t.Errorf("Stamp of the zero time = %q, want empty", got)
	}
}

// What is copied is what was sent, not what was drawn: a balloon wraps a long
// path across lines, and pasting it back must not break it.
func TestCopiedIsTheMessageAsSent(t *testing.T) {
	cases := []struct {
		name string
		msg  Message
		want string
	}{
		{"plain", Message{Text: "/tmp/some/very/long/path/that/wraps.log"}, "/tmp/some/very/long/path/that/wraps.log"},
		{"the sender's own line breaks survive", Message{Text: "one\ntwo"}, "one\ntwo"},
		{"heading first", Message{Title: "CI", Text: "build failed"}, "CI\nbuild failed"},
		{"a headline with its link", Message{Title: "Go Blog", Text: "Generics\nhttps://go.dev/blog/x"}, "Go Blog\nGenerics\nhttps://go.dev/blog/x"},
	}
	for _, c := range cases {
		if got := c.msg.Copied(); got != c.want {
			t.Errorf("%s: Copied = %q, want %q", c.name, got, c.want)
		}
	}
}
