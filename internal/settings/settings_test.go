package settings

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/kaakaa/gumpet/internal/config"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	return New(config.Default(), filepath.Join(t.TempDir(), "config.yaml"))
}

func TestSaveUpdatesGetAndTheFile(t *testing.T) {
	s := newStore(t)

	cfg := config.Default()
	cfg.Behavior.Roam = config.RoamWander
	if err := s.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if got := s.Get(); got.Behavior.Roam != config.RoamWander {
		t.Errorf("Get().Behavior.Roam = %q, want wander", got.Behavior.Roam)
	}
	reloaded, err := config.Load(s.Path())
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !reflect.DeepEqual(reloaded, cfg) {
		t.Errorf("file holds %+v, want %+v", reloaded, cfg)
	}
}

func TestSubscribersSeeEveryChange(t *testing.T) {
	s := newStore(t)
	a := s.Subscribe(4)
	b := s.Subscribe(4)

	cfg := config.Default()
	cfg.Message.TextScale = 3
	if err := s.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	for name, ch := range map[string]<-chan config.Config{"a": a, "b": b} {
		select {
		case got := <-ch:
			if got.Message.TextScale != 3 {
				t.Errorf("%s got text scale %v, want 3", name, got.Message.TextScale)
			}
		default:
			t.Errorf("%s was told nothing", name)
		}
	}
}

func TestUpdateChangesOneSetting(t *testing.T) {
	s := newStore(t)

	if err := s.Update(func(c *config.Config) { c.Window.AlwaysOnTop = false }); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got := s.Get()
	if got.Window.AlwaysOnTop {
		t.Error("always_on_top was not turned off")
	}
	if got.Stage != config.Default().Stage {
		t.Error("Update changed settings it was not asked to")
	}
}

func TestInvalidSettingsAreNotSavedOrPublished(t *testing.T) {
	s := newStore(t)
	sub := s.Subscribe(4)

	err := s.Update(func(c *config.Config) { c.Behavior.Roam = "sideways" })
	if err == nil {
		t.Fatal("Update accepted an unknown roaming style")
	}
	if got := s.Get(); got.Behavior.Roam != config.Default().Behavior.Roam {
		t.Errorf("the rejected change stuck: roam = %q", got.Behavior.Roam)
	}
	if len(sub) != 0 {
		t.Error("a rejected change was published")
	}
}

// A subscriber that has stopped reading must not be able to wedge a save.
func TestASlowSubscriberDoesNotBlockSaving(t *testing.T) {
	s := newStore(t)
	s.Subscribe(1)

	for range 5 {
		if err := s.Update(func(c *config.Config) { c.Behavior.Speed++ }); err != nil {
			t.Fatalf("Update: %v", err)
		}
	}
	if got := s.Get().Behavior.Speed; got != config.Default().Behavior.Speed+5 {
		t.Errorf("speed = %v, want the five increments to have landed", got)
	}
}
