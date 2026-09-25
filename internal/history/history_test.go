package history

import (
	"fmt"
	"testing"
	"time"

	"github.com/kaakaa/gumpet/internal/chatter"
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

// A remark goes straight on screen, so it is never "waiting"; and a headline
// keeps its link and date apart, which is what the page offers as a link.
func TestAddRemarkKeepsTheLinkAndDate(t *testing.T) {
	c := &clock{t: time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)}
	s := NewRemarks(config.History{Max: 10})
	s.now = c.now

	published := time.Date(2026, 9, 6, 8, 0, 0, 0, time.UTC)
	rec := s.AddRemark(chatter.Remark{
		Text: "Go 2 is out", Title: "Go Blog",
		Link: "https://go.dev/blog/go2", At: published,
	}, true)

	if rec.Text != "Go 2 is out" || rec.Title != "Go Blog" {
		t.Errorf("recorded %q under %q, want the headline under the feed's name", rec.Text, rec.Title)
	}
	if rec.Link != "https://go.dev/blog/go2" {
		t.Errorf("link = %q, want the article", rec.Link)
	}
	if !rec.Published.Equal(published) {
		t.Errorf("published = %v, want %v", rec.Published, published)
	}
	if !rec.Shown() || !rec.ShownAt.Equal(c.t) {
		t.Errorf("shown at %v, want %v: a remark is on screen as soon as it is said", rec.ShownAt, c.t)
	}
	if !rec.Seen {
		t.Error("a repeat was recorded as new")
	}
	if rec.ID != "r1" {
		t.Errorf("id = %q, want r1: remark IDs must not collide with message IDs", rec.ID)
	}
}

// The whole point of keeping remarks in a store of their own: a pet reading
// out a hundred headlines must not age a single sent message out of the
// record, however small the limit.
func TestRemarksNeverPushMessagesOut(t *testing.T) {
	limits := config.History{Max: 5}
	messages := New(limits)
	remarks := NewRemarks(limits)

	for i := range 3 {
		messages.Add(message.Message{Text: fmt.Sprintf("sent %d", i)})
	}
	for i := range 100 {
		remarks.AddRemark(chatter.Remark{Text: fmt.Sprintf("headline %d", i)}, false)
	}

	if got := messages.Len(); got != 3 {
		t.Errorf("%d messages kept after 100 remarks, want all 3", got)
	}
	if got := remarks.Len(); got != 5 {
		t.Errorf("%d remarks kept, want the limit of 5", got)
	}
	if got := remarks.List()[0].Text; got != "headline 99" {
		t.Errorf("newest remark = %q, want headline 99", got)
	}
}

// A message held during quiet hours is recorded as such, so the page does not
// show it waiting for a turn that is never coming.
func TestMarkHeld(t *testing.T) {
	s, _ := newStore(t, config.History{Max: 10})
	rec := s.Add(message.Message{Text: "while it was quiet"})
	s.MarkHeld(rec.ID)
	s.MarkHeld("nonexistent") // quietly ignored, as MarkShown does

	got := s.List()[0]
	if !got.Held || got.Shown() {
		t.Errorf("held=%v shown=%v, want held and never shown", got.Held, got.Shown())
	}
}
