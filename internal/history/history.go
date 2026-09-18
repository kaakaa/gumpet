// Package history keeps a record of the messages gumpet has been sent, so the
// messages page can show what arrived and whether the pet has said it yet.
//
// The record lives in memory only: it is a log of what a running gumpet has
// been up to, not a mailbox, and it starts empty on every run.
package history

import (
	"fmt"
	"sync"
	"time"

	"github.com/kaakaa/gumpet/internal/config"
	"github.com/kaakaa/gumpet/internal/message"
)

// Record is one message and what became of it.
type Record struct {
	ID   string `json:"id"`
	Text string `json:"text"`
	// Title is the heading the sender gave the message, usually empty.
	Title string `json:"title,omitempty"`
	// Level is how loud the message was said to be.
	Level message.Level `json:"level,omitempty"`
	// Duration is the display time asked for when the message was sent, or
	// zero to use the configured default.
	Duration time.Duration `json:"-"`
	// QueuedAt is when gumpet received the message.
	QueuedAt time.Time `json:"queued_at"`
	// ShownAt is when the pet put it on screen, or the zero time if it is
	// still waiting its turn.
	ShownAt time.Time `json:"shown_at,omitzero"`
}

// Shown reports whether the pet has displayed this message.
func (r Record) Shown() bool { return !r.ShownAt.IsZero() }

// Store is the record of received messages, newest last.
type Store struct {
	mu      sync.Mutex
	cfg     config.History
	records []Record
	// nextID numbers messages within this run. The record does not outlive the
	// process, so a counter is identifier enough.
	nextID int64
	// now is swappable for tests.
	now func() time.Time
}

// New returns an empty store keeping as much as cfg allows.
func New(cfg config.History) *Store {
	return &Store{cfg: cfg, now: time.Now}
}

// SetLimits adopts new retention settings and prunes to match them.
func (s *Store) SetLimits(cfg config.History) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg = cfg
	s.prune()
}

// Add records a newly received message and returns it, with the ID the pet
// will report back when it shows it. Everything about the message except its
// ID and its timestamps comes from the caller, so that what the page shows is
// what the pet was actually asked to say.
func (s *Store) Add(msg message.Message) Record {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.nextID++
	rec := Record{
		ID:       fmt.Sprintf("m%d", s.nextID),
		Text:     msg.Text,
		Title:    msg.Title,
		Level:    msg.Level,
		Duration: msg.Duration,
		QueuedAt: s.now(),
	}
	s.records = append(s.records, rec)
	s.prune()
	return rec
}

// MarkShown notes that the pet has put a message on screen. A message that has
// already been pruned is quietly ignored: the pet may still be displaying one
// that has aged out of the record.
func (s *Store) MarkShown(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.records {
		if s.records[i].ID == id {
			if s.records[i].ShownAt.IsZero() {
				s.records[i].ShownAt = s.now()
			}
			return
		}
	}
}

// List returns the record newest first, pruned to the current settings.
func (s *Store) List() []Record {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prune()

	out := make([]Record, len(s.records))
	for i, rec := range s.records {
		out[len(s.records)-1-i] = rec
	}
	return out
}

// Len is how many messages are currently kept.
func (s *Store) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prune()
	return len(s.records)
}

// prune drops what the settings no longer allow. Callers hold the lock.
func (s *Store) prune() {
	if retention := s.cfg.Retention(); retention > 0 {
		cutoff := s.now().Add(-retention)
		keep := 0
		for keep < len(s.records) && s.records[keep].QueuedAt.Before(cutoff) {
			keep++
		}
		if keep > 0 {
			s.records = append(s.records[:0], s.records[keep:]...)
		}
	}
	if s.cfg.Max > 0 && len(s.records) > s.cfg.Max {
		drop := len(s.records) - s.cfg.Max
		s.records = append(s.records[:0], s.records[drop:]...)
	}
}
