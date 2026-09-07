// Package settings is the one place gumpet's live configuration lives.
//
// Both the settings page and the pet's own menu change settings, and both need
// to see what the other did, so neither owns the config: this does, and it
// tells everyone who is listening whenever it changes.
package settings

import (
	"sync"

	"github.com/kaakaa/gumpet/internal/config"
)

// Store holds the current settings and publishes every change.
type Store struct {
	mu   sync.RWMutex
	cfg  config.Config
	path string
	subs []chan config.Config
}

// New returns a store holding cfg, which is saved back to path.
func New(cfg config.Config, path string) *Store {
	return &Store{cfg: cfg, path: path}
}

// Get returns the current settings.
func (s *Store) Get() config.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

// Path is the config file the store writes to.
func (s *Store) Path() string { return s.path }

// Subscribe returns a channel that receives every later change. The buffer
// should be big enough that a subscriber which is a little behind does not miss
// one; a subscriber that has stopped reading is skipped rather than blocking
// the save.
func (s *Store) Subscribe(buffer int) <-chan config.Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch := make(chan config.Config, buffer)
	s.subs = append(s.subs, ch)
	return ch
}

// Save validates cfg, writes it to the config file, and publishes it. Nothing
// is published if the settings are rejected or the file cannot be written, so
// a failed save leaves everyone on the settings they already had.
func (s *Store) Save(cfg config.Config) error {
	return s.Update(func(c *config.Config) { *c = cfg })
}

// Update applies change to the current settings and saves the result. It is
// what the pet's menu uses to flip one setting without having to know about
// all the others.
//
// The whole read-modify-write happens under the lock: the menu and the settings
// page both save, and a change that was read before the other's write would
// otherwise be written back over it.
func (s *Store) Update(change func(*config.Config)) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cfg := s.cfg
	change(&cfg)
	if err := cfg.Validate(); err != nil {
		return err
	}
	if err := config.Save(s.path, cfg); err != nil {
		return err
	}
	s.cfg = cfg

	for _, ch := range s.subs {
		select {
		case ch <- cfg:
		default:
		}
	}
	return nil
}
