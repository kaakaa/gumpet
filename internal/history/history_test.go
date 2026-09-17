package history

import (
	"testing"
	"time"

	"github.com/kaakaa/gumpet/internal/config"
	"github.com/kaakaa/gumpet/internal/message"
)

// clock is a stand-in for time.Now that only moves when a test says so.
type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func newStore(t *testing.T, cfg config.History) (*Store, *clock) {
	t.Helper()
	c := &clock{t: time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)}
	s := New(cfg)
	s.now = c.now
	return s, c
}

func texts(records []Record) []string {
	out := make([]string, len(records))
	for i, r := range records {
		out[i] = r.Text
	}
	return out
}

func TestAddAndListNewestFirst(t *testing.T) {
	s, c := newStore(t, config.History{Max: 10})

	for _, text := range []string{"one", "two", "three"} {
		s.Add(message.Message{Text: text})
		c.t = c.t.Add(time.Second)
	}

	got := texts(s.List())
	want := []string{"three", "two", "one"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("List = %q, want %q", got, want)
		}
	}
}

func TestAddReturnsAUsableID(t *testing.T) {
	s, _ := newStore(t, config.History{Max: 10})

	first := s.Add(message.Message{Text: "one"})
	second := s.Add(message.Message{Text: "two"})

	if first.ID == "" || second.ID == "" {
		t.Fatal("Add returned an empty ID")
	}
	if first.ID == second.ID {
		t.Errorf("both messages got the ID %q", first.ID)
	}
	if first.Shown() {
		t.Error("a message is shown before the pet has displayed it")
	}
}

func TestMarkShown(t *testing.T) {
	s, c := newStore(t, config.History{Max: 10})
	rec := s.Add(message.Message{Text: "hello"})

	c.t = c.t.Add(3 * time.Second)
	s.MarkShown(rec.ID)

	got := s.List()[0]
	if !got.Shown() {
		t.Fatal("the message is still marked unshown")
	}
	if !got.ShownAt.Equal(c.t) {
		t.Errorf("ShownAt = %v, want %v", got.ShownAt, c.t)
	}
}

func TestMarkShownKeepsTheFirstTime(t *testing.T) {
	s, c := newStore(t, config.History{Max: 10})
	rec := s.Add(message.Message{Text: "hello"})

	s.MarkShown(rec.ID)
	first := s.List()[0].ShownAt
	c.t = c.t.Add(time.Minute)
	s.MarkShown(rec.ID)

	if got := s.List()[0].ShownAt; !got.Equal(first) {
		t.Errorf("ShownAt moved to %v, want it to stay at %v", got, first)
	}
}

func TestMarkShownIgnoresAnUnknownID(t *testing.T) {
	s, _ := newStore(t, config.History{Max: 10})
	s.Add(message.Message{Text: "hello"})

	s.MarkShown("m999") // pruned, or from a previous run

	if s.List()[0].Shown() {
		t.Error("marked the wrong message")
	}
}

func TestMaxDropsTheOldest(t *testing.T) {
	s, c := newStore(t, config.History{Max: 3})

	for _, text := range []string{"one", "two", "three", "four", "five"} {
		s.Add(message.Message{Text: text})
		c.t = c.t.Add(time.Second)
	}

	got := texts(s.List())
	want := []string{"five", "four", "three"}
	if len(got) != len(want) {
		t.Fatalf("kept %d messages, want %d: %q", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("List = %q, want %q", got, want)
		}
	}
}

func TestRetentionDropsWhatHasAgedOut(t *testing.T) {
	s, c := newStore(t, config.History{Max: 100, Hours: 1})

	s.Add(message.Message{Text: "old"})
	c.t = c.t.Add(90 * time.Minute)
	s.Add(message.Message{Text: "new"})

	got := texts(s.List())
	if len(got) != 1 || got[0] != "new" {
		t.Errorf("List = %q, want just the recent one", got)
	}
}

func TestZeroRetentionKeepsEverythingUpToMax(t *testing.T) {
	s, c := newStore(t, config.History{Max: 100, Hours: 0})

	s.Add(message.Message{Text: "old"})
	c.t = c.t.Add(30 * 24 * time.Hour)
	s.Add(message.Message{Text: "new"})

	if got := s.Len(); got != 2 {
		t.Errorf("kept %d messages, want both", got)
	}
}

func TestSetLimitsPrunesImmediately(t *testing.T) {
	s, c := newStore(t, config.History{Max: 100})
	for _, text := range []string{"one", "two", "three"} {
		s.Add(message.Message{Text: text})
		c.t = c.t.Add(time.Second)
	}

	s.SetLimits(config.History{Max: 1})

	got := texts(s.List())
	if len(got) != 1 || got[0] != "three" {
		t.Errorf("List = %q, want just the newest", got)
	}
}

func TestListDoesNotHandOutTheStoresOwnSlice(t *testing.T) {
	s, _ := newStore(t, config.History{Max: 10})
	s.Add(message.Message{Text: "hello"})

	s.List()[0].Text = "tampered"

	if got := s.List()[0].Text; got != "hello" {
		t.Errorf("the caller changed the store: %q", got)
	}
}

func TestConcurrentUse(t *testing.T) {
	s, _ := newStore(t, config.History{Max: 50})
	s.now = time.Now

	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 200 {
			rec := s.Add(message.Message{Text: "x"})
			s.MarkShown(rec.ID)
		}
	}()
	for range 200 {
		s.List()
		s.Len()
	}
	<-done

	if got := s.Len(); got != 50 {
		t.Errorf("kept %d messages, want the 50 the settings allow", got)
	}
}

// The page shows what the sender said about the message, so the record has to
// carry it rather than reducing everything to text.
func TestAddKeepsTheTitleAndLevel(t *testing.T) {
	c := &clock{t: time.Unix(0, 0)}
	s := New(config.History{Max: 10})
	s.now = c.now

	rec := s.Add(message.Message{Text: "tests failed", Title: "CI", Level: message.LevelError})
	if rec.Title != "CI" || rec.Level != message.LevelError {
		t.Errorf("Add returned title %q level %q, want CI/error", rec.Title, rec.Level)
	}

	listed := s.List()
	if len(listed) != 1 {
		t.Fatalf("List returned %d records, want 1", len(listed))
	}
	if listed[0].Title != "CI" || listed[0].Level != message.LevelError {
		t.Errorf("listed title %q level %q, want CI/error", listed[0].Title, listed[0].Level)
	}
}
