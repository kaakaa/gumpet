package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestRenderRoundTrips(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{"defaults", Default()},
		{
			name: "everything changed",
			cfg: Config{
				Language: "ja",
				Server:   Server{Addr: "127.0.0.1:9999", Token: "s3cret"},
				Window:   Window{AlwaysOnTop: false, ClickThrough: false, SkipTaskbar: false},
				Stage:    Stage{Display: 2, Fullscreen: true, Width: 800, Height: 600, Anchor: AnchorCustom, MarginX: 1, MarginY: 2, X: 30, Y: 40},
				Pet:      Pet{Source: "/tmp/cat.gif", Scale: 0.75, FPS: 12.5, FlipWhenFacingRight: false, Smooth: SmoothOff},
				Behavior: Behavior{Mode: ModeOnMessage, IdleOpacity: 0.2, Roam: RoamWander, Speed: 0, Jump: false, React: false,
					Quiet: Quiet{From: "22:00", To: "08:30"},
					Chatter: Chatter{Enabled: true, IntervalSec: 90.5, Source: "/tmp/my sayings.txt",
						Feeds: []Feed{
							{Name: "HN", URL: "https://news.ycombinator.com/rss"},
							// A feed with no name at all: the balloon simply
							// gets no heading, and this must survive a save.
							{Name: "", URL: "https://go.dev/blog/feed.atom"},
							{Name: "名前に空白と記号: ok", URL: "https://example.com/feed?a=b&c=d"},
						},
						MaxAgeDays: 7, FetchIntervalSec: 60}},
				Message: Message{DurationSec: 12.25, MaxVisible: 1, MaxWidth: 300, MaxQueue: 3, TextScale: 1.5, TypeSpeed: 0, OpenLinks: false},
				Font:    Font{Path: "/tmp/My Font.ttc", System: false},
				History: History{Max: 10, Hours: 0.5},
			},
		},
		{
			name: "path with non-ASCII characters and a space",
			cfg: func() Config {
				c := Default()
				c.Pet.Source = "/Users/kaakaa/画像 と gopher/ねこ.gif"
				return c
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := Render(tt.cfg)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			var got Config
			if err := yaml.Unmarshal([]byte(out), &got); err != nil {
				t.Fatalf("parse rendered config: %v\n%s", err, out)
			}
			if !reflect.DeepEqual(got, tt.cfg) {
				t.Errorf("round trip changed the config:\n got %+v\nwant %+v\n\n%s", got, tt.cfg, out)
			}
		})
	}
}

func TestRenderKeepsComments(t *testing.T) {
	out := DefaultYAML()
	if !strings.Contains(out, "# The stage is the part of the monitor the pet may move around in.") {
		t.Errorf("rendered config lost its comments:\n%s", out)
	}
}

func TestDefaultIsValid(t *testing.T) {
	if err := Default().Validate(); err != nil {
		t.Errorf("Default() is not valid: %v", err)
	}
}

func TestLoadWritesDefaultWhenMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "config.yaml")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(cfg, Default()) {
		t.Errorf("Load returned %+v, want defaults", cfg)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("Load did not write a config file: %v", err)
	}
}

func TestSaveThenLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")

	want := Default()
	want.Stage.Anchor = AnchorTopLeft
	want.Pet.Scale = 0.5
	want.Behavior.Roam = RoamWander
	if err := Save(path, want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Load = %+v, want %+v", got, want)
	}
}

func TestSaveLeavesNoTemporaryFilesBehind(t *testing.T) {
	dir := t.TempDir()
	if err := Save(filepath.Join(dir, "config.yaml"), Default()); err != nil {
		t.Fatalf("Save: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "config.yaml" {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Errorf("directory holds %v, want just config.yaml", names)
	}
}

func TestLoadKeepsDefaultsForOmittedKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("behavior:\n  roam: perimeter\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Behavior.Roam != RoamPerimeter {
		t.Errorf("behavior.roam = %q, want perimeter", cfg.Behavior.Roam)
	}
	if cfg.Stage.Anchor != AnchorBottomRight {
		t.Errorf("stage.anchor = %q, want the default %q", cfg.Stage.Anchor, AnchorBottomRight)
	}
}

func TestLoadRejectsInvalidConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("behavior:\n  mode: sometimes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Error("Load accepted an unknown behavior.mode")
	}
}

func TestValidateRejectsBadValues(t *testing.T) {
	tests := []struct {
		name string
		mod  func(*Config)
	}{
		{"empty addr", func(c *Config) { c.Server.Addr = "" }},
		{"a language with no dictionary", func(c *Config) { c.Language = "fr" }},
		{"an empty language", func(c *Config) { c.Language = "" }},
		{"unknown anchor", func(c *Config) { c.Stage.Anchor = "middle" }},
		{"zero width", func(c *Config) { c.Stage.Width = 0 }},
		{"display counted from zero", func(c *Config) { c.Stage.Display = 0 }},
		{"negative display", func(c *Config) { c.Stage.Display = -1 }},
		{"negative scale", func(c *Config) { c.Pet.Scale = -1 }},
		{"zero fps", func(c *Config) { c.Pet.FPS = 0 }},
		{"unknown smoothing", func(c *Config) { c.Pet.Smooth = "blurry" }},
		{"empty smoothing", func(c *Config) { c.Pet.Smooth = "" }},
		{"unknown mode", func(c *Config) { c.Behavior.Mode = "sometimes" }},
		{"zero idle opacity", func(c *Config) { c.Behavior.IdleOpacity = 0 }},
		{"negative idle opacity", func(c *Config) { c.Behavior.IdleOpacity = -0.5 }},
		{"idle opacity above 1", func(c *Config) { c.Behavior.IdleOpacity = 1.5 }},
		{"unknown roam", func(c *Config) { c.Behavior.Roam = "sideways" }},
		{"zero balloon width", func(c *Config) { c.Message.MaxWidth = 0 }},
		{"negative speed", func(c *Config) { c.Behavior.Speed = -1 }},
		{"zero duration", func(c *Config) { c.Message.DurationSec = 0 }},
		{"zero queue", func(c *Config) { c.Message.MaxQueue = 0 }},
		{"zero text scale", func(c *Config) { c.Message.TextScale = 0 }},
		{"negative type speed", func(c *Config) { c.Message.TypeSpeed = -1 }},
		{"zero visible balloons", func(c *Config) { c.Message.MaxVisible = 0 }},
		{"zero chatter interval", func(c *Config) { c.Behavior.Chatter.IntervalSec = 0 }},
		{"negative chatter interval", func(c *Config) { c.Behavior.Chatter.IntervalSec = -1 }},
		{"zero fetch interval", func(c *Config) { c.Behavior.Chatter.FetchIntervalSec = 0 }},
		{"negative max age", func(c *Config) { c.Behavior.Chatter.MaxAgeDays = -1 }},
		// Half a quiet window would silently do nothing, which is the one
		// thing a setting must not do.
		{"quiet with only a start", func(c *Config) { c.Behavior.Quiet = Quiet{From: "22:00"} }},
		{"quiet with only an end", func(c *Config) { c.Behavior.Quiet = Quiet{To: "08:30"} }},
		{"quiet at an hour that does not exist", func(c *Config) { c.Behavior.Quiet = Quiet{From: "25:00", To: "08:30"} }},
		{"quiet that is not a time", func(c *Config) { c.Behavior.Quiet = Quiet{From: "10pm", To: "8am"} }},
		{"feed that is not a URL", func(c *Config) {
			c.Behavior.Chatter.Feeds = []Feed{{URL: "news.ycombinator.com/rss"}}
		}},
		{"feed reading a local file", func(c *Config) {
			c.Behavior.Chatter.Feeds = []Feed{{URL: "file:///etc/passwd"}}
		}},
		{"too many feeds", func(c *Config) {
			for i := 0; i <= MaxFeeds; i++ {
				c.Behavior.Chatter.Feeds = append(c.Behavior.Chatter.Feeds, Feed{URL: "https://example.com/f"})
			}
		}},
		{"feed name far too long", func(c *Config) {
			c.Behavior.Chatter.Feeds = []Feed{{Name: strings.Repeat("あ", MaxFeedName+1), URL: "https://example.com/f"}}
		}},
		{"feed name spanning lines", func(c *Config) {
			c.Behavior.Chatter.Feeds = []Feed{{Name: "one\ntwo", URL: "https://example.com/f"}}
		}},
		{"zero history", func(c *Config) { c.History.Max = 0 }},
		{"negative retention", func(c *Config) { c.History.Hours = -1 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			tt.mod(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Error("Validate accepted it")
			}
		})
	}
}

func TestRoamAcceptsTheOldBooleanForm(t *testing.T) {
	tests := []struct {
		yaml string
		want Roam
	}{
		{"behavior:\n  roam: true\n", RoamHorizontal},
		{"behavior:\n  roam: false\n", RoamNone},
	}
	for _, tt := range tests {
		path := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(path, []byte(tt.yaml), 0o644); err != nil {
			t.Fatal(err)
		}
		cfg, err := Load(path)
		if err != nil {
			t.Fatalf("Load %q: %v", tt.yaml, err)
		}
		if cfg.Behavior.Roam != tt.want {
			t.Errorf("roam from %q = %q, want %q", tt.yaml, cfg.Behavior.Roam, tt.want)
		}
	}
}

func TestShowsPet(t *testing.T) {
	tests := []struct {
		name       string
		mode       Mode
		hasMessage bool
		menuOpen   bool
		want       bool
	}{
		{"always, nothing happening", ModeAlways, false, false, true},
		{"always, with a message", ModeAlways, true, false, true},
		{"on-message, nothing happening", ModeOnMessage, false, false, false},
		{"on-message, with a message", ModeOnMessage, true, false, true},
		{"on-message, menu open", ModeOnMessage, false, true, true},
		{"on-message, message just expired", ModeOnMessage, false, false, false},
		{"faded, nothing happening", ModeFaded, false, false, true},
		{"faded, with a message", ModeFaded, true, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			cfg.Behavior.Mode = tt.mode
			if got := cfg.ShowsPet(tt.hasMessage, tt.menuOpen); got != tt.want {
				t.Errorf("ShowsPet(%v, %v) = %v, want %v", tt.hasMessage, tt.menuOpen, got, tt.want)
			}
		})
	}
}

func TestHistoryRetention(t *testing.T) {
	if got := (History{Hours: 1.5}).Retention(); got != 90*time.Minute {
		t.Errorf("Retention = %v, want 90m", got)
	}
	if got := (History{Hours: 0}).Retention(); got != 0 {
		t.Errorf("Retention = %v, want 0 for no time limit", got)
	}
}

func TestPetOpacity(t *testing.T) {
	tests := []struct {
		name       string
		mode       Mode
		idle       float64
		hasMessage bool
		menuOpen   bool
		want       float64
	}{
		{"always is never faded", ModeAlways, 0.3, false, false, 1},
		{"on-message is drawn in full when it is drawn at all", ModeOnMessage, 0.3, true, false, 1},
		{"faded and idle", ModeFaded, 0.3, false, false, 0.3},
		{"faded but talking", ModeFaded, 0.3, true, false, 1},
		{"faded but showing its menu", ModeFaded, 0.3, false, true, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			cfg.Behavior.Mode = tt.mode
			cfg.Behavior.IdleOpacity = tt.idle
			if got := cfg.PetOpacity(tt.hasMessage, tt.menuOpen); got != tt.want {
				t.Errorf("PetOpacity(%v, %v) = %v, want %v", tt.hasMessage, tt.menuOpen, got, tt.want)
			}
		})
	}
}

// A faded pet is still on screen, so it stays clickable and its menu still
// opens. Only on-message mode takes it away.
func TestFadedPetIsStillShown(t *testing.T) {
	cfg := Default()
	cfg.Behavior.Mode = ModeFaded
	if !cfg.ShowsPet(false, false) {
		t.Error("a faded pet is not drawn at all")
	}
}

// A config written before feeds became a list still has to load, or an upgrade
// silently turns the pet's feed off.
func TestChatterAcceptsTheOldSingleFeed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	const old = "behavior:\n  chatter:\n    enabled: true\n    feed: \"https://news.ycombinator.com/rss\"\n"
	if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Behavior.Chatter.Feeds) != 1 {
		t.Fatalf("feeds = %+v, want the old single feed carried over", cfg.Behavior.Chatter.Feeds)
	}
	if got := cfg.Behavior.Chatter.Feeds[0].URL; got != "https://news.ycombinator.com/rss" {
		t.Errorf("feeds[0].url = %q", got)
	}
	if !cfg.Behavior.Chatter.Enabled {
		t.Error("the rest of the chatter block was lost")
	}
	// Keys the old file did not mention keep their defaults.
	if cfg.Behavior.Chatter.FetchIntervalSec != Default().Behavior.Chatter.FetchIntervalSec {
		t.Errorf("fetch_interval_sec = %v, want the default", cfg.Behavior.Chatter.FetchIntervalSec)
	}
}

// The new list wins, so a file holding both is not ambiguous.
func TestANewFeedListBeatsTheOldKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	const both = "behavior:\n  chatter:\n    feed: \"https://old.example.com/rss\"\n" +
		"    feeds:\n      - name: New\n        url: \"https://new.example.com/rss\"\n"
	if err := os.WriteFile(path, []byte(both), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Behavior.Chatter.Feeds) != 1 || cfg.Behavior.Chatter.Feeds[0].Name != "New" {
		t.Errorf("feeds = %+v, want only the new list", cfg.Behavior.Chatter.Feeds)
	}
}

// An old config, once saved, should come back as a list rather than reverting.
func TestTheOldFeedIsWrittenBackAsAList(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	const old = "behavior:\n  chatter:\n    feed: \"https://news.ycombinator.com/rss\"\n"
	if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	again, err := Load(path)
	if err != nil {
		t.Fatalf("Load after Save: %v", err)
	}
	if !reflect.DeepEqual(again.Behavior.Chatter.Feeds, cfg.Behavior.Chatter.Feeds) {
		t.Errorf("after a save the feeds became %+v, want %+v",
			again.Behavior.Chatter.Feeds, cfg.Behavior.Chatter.Feeds)
	}
}

// "auto" is what makes a bundled pixel pet look right without anyone touching
// a setting, so the three values have to mean what they say.
func TestSmoothingDefersToTheArtworkOnlyWhenAuto(t *testing.T) {
	cases := []struct {
		smoothing Smoothing
		artwork   bool
		want      bool
	}{
		{SmoothAuto, true, true},
		{SmoothAuto, false, false},
		{SmoothOn, false, true},
		{SmoothOff, true, false},
	}
	for _, c := range cases {
		if got := c.smoothing.Smoothed(c.artwork); got != c.want {
			t.Errorf("%q.Smoothed(%v) = %v, want %v", c.smoothing, c.artwork, got, c.want)
		}
	}
}
