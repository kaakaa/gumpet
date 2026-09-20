// Package config loads and persists gumpet's user settings.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/kaakaa/gumpet/internal/feed"
)

// Anchor is the corner of the monitor the stage window is pinned to.
type Anchor string

// Supported stage anchors.
const (
	AnchorTopLeft     Anchor = "top-left"
	AnchorTopRight    Anchor = "top-right"
	AnchorBottomLeft  Anchor = "bottom-left"
	AnchorBottomRight Anchor = "bottom-right"
	AnchorCenter      Anchor = "center"
	AnchorCustom      Anchor = "custom"
)

// Anchors lists every valid anchor, in the order the settings page shows them.
var Anchors = []Anchor{
	AnchorTopLeft, AnchorTopRight, AnchorBottomLeft, AnchorBottomRight,
	AnchorCenter, AnchorCustom,
}

// Roam is how the pet moves around its stage.
type Roam string

// Supported roaming styles.
const (
	// RoamNone leaves the pet standing where it starts.
	RoamNone Roam = "none"
	// RoamHorizontal walks the pet back and forth along the floor.
	RoamHorizontal Roam = "horizontal"
	// RoamPerimeter walks the pet round and round the edge of the stage.
	RoamPerimeter Roam = "perimeter"
	// RoamWander sends the pet off in any direction it likes.
	RoamWander Roam = "wander"
)

// Roams lists every roaming style, in the order the settings page shows them.
var Roams = []Roam{RoamNone, RoamHorizontal, RoamPerimeter, RoamWander}

// UnmarshalYAML accepts the true/false that behavior.roam used to be, so a
// config written by an older gumpet still loads.
func (r *Roam) UnmarshalYAML(value *yaml.Node) error {
	var b bool
	if err := value.Decode(&b); err == nil {
		if b {
			*r = RoamHorizontal
		} else {
			*r = RoamNone
		}
		return nil
	}
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	*r = Roam(s)
	return nil
}

// Mode decides when the pet is on screen.
type Mode string

// Supported visibility modes.
const (
	// ModeAlways keeps the pet on screen all the time.
	ModeAlways Mode = "always"
	// ModeFaded keeps the pet on screen but faint until it has something to
	// say, so it stays out of the way without disappearing.
	ModeFaded Mode = "faded"
	// ModeOnMessage shows the pet only while a message is being displayed.
	ModeOnMessage Mode = "on-message"
)

// Modes lists every visibility mode, in the order the settings page shows them.
var Modes = []Mode{ModeAlways, ModeFaded, ModeOnMessage}

// Config is the whole of gumpet's settings.
type Config struct {
	Server   Server   `yaml:"server" json:"server"`
	Window   Window   `yaml:"window" json:"window"`
	Stage    Stage    `yaml:"stage" json:"stage"`
	Pet      Pet      `yaml:"pet" json:"pet"`
	Behavior Behavior `yaml:"behavior" json:"behavior"`
	Message  Message  `yaml:"message" json:"message"`
	Font     Font     `yaml:"font" json:"font"`
	History  History  `yaml:"history" json:"history"`
}

// Font is what gumpet draws text with.
type Font struct {
	// Path is a .ttf, .otf or .ttc to use. Empty looks for one on this machine.
	Path string `yaml:"path" json:"path"`
	// System allows falling back to a font found on this machine. With it off,
	// and no Path, the bundled bitmap font is used.
	System bool `yaml:"system" json:"system"`
}

// Server configures the HTTP endpoint that receives messages and serves the
// settings page.
type Server struct {
	// Addr is the listen address. Keep it on the loopback interface unless you
	// really do want the rest of the network sending messages to your desktop.
	// Changing it takes effect on the next start.
	Addr string `yaml:"addr" json:"addr"`
	// Token, when set, must be presented as a Bearer token or X-Gumpet-Token
	// header on every API request.
	Token string `yaml:"token" json:"token"`
}

// Window configures the transparent window the pet lives in.
type Window struct {
	AlwaysOnTop bool `yaml:"always_on_top" json:"always_on_top"`
	// ClickThrough lets mouse events pass to whatever is behind the window, so
	// the pet never steals a click.
	ClickThrough bool `yaml:"click_through" json:"click_through"`
	// SkipTaskbar hides the application icon from the Windows taskbar.
	// Changing it takes effect on the next start.
	SkipTaskbar bool `yaml:"skip_taskbar" json:"skip_taskbar"`
}

// Stage is the area of the monitor the pet is allowed to move around in. It is
// not the window: gumpet's window hugs the pet and follows it around the stage.
type Stage struct {
	// Display is which monitor to put the pet on, counting from one in the
	// order the system reports them. A number that names no monitor falls back
	// to the first.
	Display int `yaml:"display" json:"display"`
	// Fullscreen lets the pet roam that monitor entirely, ignoring the size
	// and placement below.
	Fullscreen bool   `yaml:"fullscreen" json:"fullscreen"`
	Width      int    `yaml:"width" json:"width"`
	Height     int    `yaml:"height" json:"height"`
	Anchor     Anchor `yaml:"anchor" json:"anchor"`
	MarginX    int    `yaml:"margin_x" json:"margin_x"`
	MarginY    int    `yaml:"margin_y" json:"margin_y"`
	// X and Y are the monitor coordinates used when Anchor is "custom".
	X int `yaml:"x" json:"x"`
	Y int `yaml:"y" json:"y"`
}

// Pet describes the artwork and how fast it animates.
type Pet struct {
	// Source is a PNG/JPEG file, an animated GIF, or a directory of frames.
	// Empty means the bundled gopher.
	Source string  `yaml:"source" json:"source"`
	Scale  float64 `yaml:"scale" json:"scale"`
	// FPS is the frame rate for sources that carry no timing of their own.
	// Animated GIFs use the delays stored in the file.
	FPS float64 `yaml:"fps" json:"fps"`
	// FlipWhenFacingRight mirrors the artwork when the pet walks right, which
	// is what you want for a pet drawn facing left.
	FlipWhenFacingRight bool `yaml:"flip_when_facing_right" json:"flip_when_facing_right"`
	// Smooth decides how the artwork is enlarged: "auto" lets the artwork
	// decide, which is right for everything bundled, and "on"/"off" overrides
	// it for artwork of your own. Pixel art wants "off" — smoothing it only
	// blurs the squares it is drawn from.
	Smooth Smoothing `yaml:"smooth" json:"smooth"`
}

// Smoothing is how a pet's artwork is enlarged.
type Smoothing string

// The ways artwork can be enlarged.
const (
	SmoothAuto Smoothing = "auto"
	SmoothOn   Smoothing = "on"
	SmoothOff  Smoothing = "off"
)

// Smoothings lists the valid values, for validation and for the settings page.
var Smoothings = []Smoothing{SmoothAuto, SmoothOn, SmoothOff}

// Smoothed reports whether artwork should be smoothed, given what the artwork
// itself asks for. "auto" defers to the artwork, which is how a bundled pixel
// pet stays sharp without anyone touching a setting.
func (s Smoothing) Smoothed(artworkWants bool) bool {
	switch s {
	case SmoothOn:
		return true
	case SmoothOff:
		return false
	default:
		return artworkWants
	}
}

// Behavior is how the pet acts between messages.
type Behavior struct {
	Mode Mode `yaml:"mode" json:"mode"`
	// IdleOpacity is how solid the pet is drawn in "faded" mode when it has
	// nothing to say: 1 is fully opaque, and lower is fainter.
	IdleOpacity float64 `yaml:"idle_opacity" json:"idle_opacity"`
	// Roam is how the pet moves around the stage.
	Roam Roam `yaml:"roam" json:"roam"`
	// Speed is the walking speed in pixels per second.
	Speed float64 `yaml:"speed" json:"speed"`
	// Jump lets the pet hop now and then while it is on the floor, which is
	// most of what stops a walk from reading as a patrol.
	Jump bool `yaml:"jump" json:"jump"`
	// Chatter is the pet talking to itself when nobody has sent it anything.
	Chatter Chatter `yaml:"chatter" json:"chatter"`
}

// Chatter is what the pet says of its own accord between messages.
type Chatter struct {
	// Enabled is off by default. A pet that starts talking unprompted is a
	// surprise, so it is asked for rather than assumed.
	Enabled bool `yaml:"enabled" json:"enabled"`
	// IntervalSec is roughly how long between remarks. Roughly, because the
	// wait is varied either side of it: exactly the same gap every time sounds
	// like a machine.
	IntervalSec float64 `yaml:"interval_sec" json:"interval_sec"`
	// Source is a file of sayings, one per line. Empty uses the bundled list.
	Source string `yaml:"source" json:"source"`
	// Feeds are RSS or Atom sources whose headlines the pet reads out instead
	// of the sayings above. Empty, which is the default, means gumpet makes no
	// outgoing connections at all.
	Feeds []Feed `yaml:"feeds" json:"feeds"`
	// MaxAgeDays drops headlines older than this, so that a podcast archive of
	// five hundred episodes does not bury this morning's news. Zero keeps
	// everything. Items a feed did not date are always kept.
	MaxAgeDays float64 `yaml:"max_age_days" json:"max_age_days"`
	// FetchIntervalSec is how often the feeds are re-read. It is separate from
	// IntervalSec because there is no reason to fetch once per remark, and it
	// is shared by all of them because nobody has wanted otherwise.
	FetchIntervalSec float64 `yaml:"fetch_interval_sec" json:"fetch_interval_sec"`
}

// MaxFeeds caps how many sources are read. The limit is not about gumpet: it
// is about not turning a config file into a way of hitting twenty servers on a
// timer.
const MaxFeeds = 10

// MaxFeedName caps a feed's label. It is drawn as the balloon's heading, and a
// name pasted in from somewhere else should not be able to fill the screen.
const MaxFeedName = 40

// Feed is one source of headlines.
type Feed struct {
	// Name is shown as the heading on the balloon, so a headline says where it
	// came from. Optional: without one the balloon simply has no heading.
	Name string `yaml:"name" json:"name"`
	URL  string `yaml:"url" json:"url"`
}

// UnmarshalYAML accepts the single `feed: "https://..."` that this setting used
// to be, so a config written by an older gumpet still loads. It becomes one
// unnamed entry, and is written back out as a list on the next save.
func (c *Chatter) UnmarshalYAML(value *yaml.Node) error {
	// An alias, so decoding into it does not call this method again.
	type chatter Chatter
	var raw struct {
		chatter `yaml:",inline"`
		Feed    string `yaml:"feed"`
	}
	// Defaults survive keys the file leaves out, which is what Load relies on.
	raw.chatter = chatter(*c)

	if err := value.Decode(&raw); err != nil {
		return err
	}
	*c = Chatter(raw.chatter)
	if raw.Feed != "" && len(c.Feeds) == 0 {
		c.Feeds = []Feed{{URL: raw.Feed}}
	}
	if c.Feeds == nil {
		c.Feeds = []Feed{}
	}
	return nil
}

// Message controls how long text stays up and how big it is drawn.
type Message struct {
	DurationSec float64 `yaml:"duration_sec" json:"duration_sec"`
	// MaxVisible is how many balloons may be on screen at once. Messages that
	// arrive together are shown together, up to this many, which is what makes
	// a burst look like a crowd rather than a queue.
	MaxVisible int `yaml:"max_visible" json:"max_visible"`
	// MaxWidth is how wide the speech balloon may grow before the text wraps,
	// in pixels.
	MaxWidth int `yaml:"max_width" json:"max_width"`
	// MaxQueue caps the backlog of messages waiting their turn. The oldest are
	// dropped once it is full.
	MaxQueue  int     `yaml:"max_queue" json:"max_queue"`
	TextScale float64 `yaml:"text_scale" json:"text_scale"`
	// TypeSpeed is how many characters a second appear when the pet says
	// something, so that a message reads as being spoken rather than simply
	// appearing. Zero shows the whole thing at once.
	TypeSpeed float64 `yaml:"type_speed" json:"type_speed"`
	// OpenLinks says whether clicking a URL in a message opens it. Links are
	// drawn as links either way; this only decides whether they do anything.
	// Anything that can reach the API can put a link in front of the person at
	// this desktop, so it is worth being able to turn off.
	OpenLinks bool `yaml:"open_links" json:"open_links"`
}

// History is how much of what the pet has said is kept for the messages page.
// The record lives in memory, so it starts empty on every run.
type History struct {
	// Max is how many messages to keep, newest first.
	Max int `yaml:"max" json:"max"`
	// Hours is how long to keep them. Zero keeps them until Max is reached.
	Hours float64 `yaml:"hours" json:"hours"`
}

// Retention is how long a message is kept, or zero for no time limit.
func (h History) Retention() time.Duration {
	return time.Duration(h.Hours * float64(time.Hour))
}

// Default returns the settings gumpet uses before anyone changes anything.
func Default() Config {
	return Config{
		Server: Server{
			Addr:  "127.0.0.1:8787",
			Token: "",
		},
		Window: Window{
			AlwaysOnTop: true,
			// Off by default so that clicking the pet opens its menu. The
			// window is only as big as the pet, so little of the screen is
			// covered.
			ClickThrough: false,
			SkipTaskbar:  true,
		},
		Stage: Stage{
			Display:    1,
			Fullscreen: false,
			Width:      520,
			Height:     360,
			Anchor:     AnchorBottomRight,
			MarginX:    24,
			MarginY:    24,
			X:          0,
			Y:          0,
		},
		Pet: Pet{
			Source:              "",
			Scale:               1,
			FPS:                 8,
			FlipWhenFacingRight: true,
			Smooth:              SmoothAuto,
		},
		Behavior: Behavior{
			Mode:        ModeAlways,
			IdleOpacity: 0.35,
			Roam:        RoamHorizontal,
			Speed:       45,
			Jump:        true,
			Chatter: Chatter{
				Enabled:     false,
				IntervalSec: 600,
				Source:      "",
				// Empty rather than nil: this is marshalled to the settings
				// page as JSON, where nil would arrive as null and the page
				// would have a list it cannot iterate.
				Feeds:            []Feed{},
				MaxAgeDays:       30,
				FetchIntervalSec: 1800,
			},
		},
		Message: Message{
			DurationSec: 8,
			MaxVisible:  3,
			MaxWidth:    640,
			MaxQueue:    20,
			TextScale:   1.5,
			TypeSpeed:   45,
			OpenLinks:   true,
		},
		Font: Font{
			Path:   "",
			System: true,
		},
		History: History{
			Max:   200,
			Hours: 24,
		},
	}
}

// ShowsPet reports whether the pet should be drawn at all, given whether it has
// something to say and whether its menu is open. Only "on-message" mode takes
// it off the screen entirely.
func (c Config) ShowsPet(hasMessage, menuOpen bool) bool {
	if c.Behavior.Mode != ModeOnMessage {
		return true
	}
	return hasMessage || menuOpen
}

// PetOpacity is how solid to draw the pet: 1 normally, and the configured idle
// opacity in "faded" mode while there is nothing going on. A pet that is saying
// something, or showing its menu, is always drawn in full.
func (c Config) PetOpacity(hasMessage, menuOpen bool) float64 {
	if c.Behavior.Mode != ModeFaded || hasMessage || menuOpen {
		return 1
	}
	return c.Behavior.IdleOpacity
}

// Validate reports the first setting that gumpet cannot work with. Its messages
// are shown to the user by the settings page, so they name the field.
func (c Config) Validate() error {
	if c.Server.Addr == "" {
		return fmt.Errorf("server.addr must not be empty")
	}
	if !validAnchor(c.Stage.Anchor) {
		return fmt.Errorf("stage.anchor %q is not one of top-left, top-right, bottom-left, bottom-right, center, custom", c.Stage.Anchor)
	}
	if c.Stage.Display < 1 {
		return fmt.Errorf("stage.display counts from 1, got %d", c.Stage.Display)
	}
	if c.Stage.Width <= 0 || c.Stage.Height <= 0 {
		return fmt.Errorf("stage.width and stage.height must be positive, got %dx%d", c.Stage.Width, c.Stage.Height)
	}
	if c.Pet.Scale <= 0 {
		return fmt.Errorf("pet.scale must be positive, got %v", c.Pet.Scale)
	}
	if c.Pet.FPS <= 0 {
		return fmt.Errorf("pet.fps must be positive, got %v", c.Pet.FPS)
	}
	if !validMode(c.Behavior.Mode) {
		return fmt.Errorf("behavior.mode %q is not one of always, faded, on-message", c.Behavior.Mode)
	}
	if c.Behavior.IdleOpacity <= 0 || c.Behavior.IdleOpacity > 1 {
		return fmt.Errorf("behavior.idle_opacity must be above 0 and at most 1, got %v", c.Behavior.IdleOpacity)
	}
	if !validRoam(c.Behavior.Roam) {
		return fmt.Errorf("behavior.roam %q is not one of none, horizontal, perimeter, wander", c.Behavior.Roam)
	}
	if c.Behavior.Speed < 0 {
		return fmt.Errorf("behavior.speed must not be negative, got %v", c.Behavior.Speed)
	}
	if c.Behavior.Chatter.IntervalSec <= 0 {
		return fmt.Errorf("behavior.chatter.interval_sec must be positive, got %v", c.Behavior.Chatter.IntervalSec)
	}
	if c.Behavior.Chatter.MaxAgeDays < 0 {
		return fmt.Errorf("behavior.chatter.max_age_days must not be negative, got %v", c.Behavior.Chatter.MaxAgeDays)
	}
	if c.Behavior.Chatter.FetchIntervalSec <= 0 {
		return fmt.Errorf("behavior.chatter.fetch_interval_sec must be positive, got %v", c.Behavior.Chatter.FetchIntervalSec)
	}
	if n := len(c.Behavior.Chatter.Feeds); n > MaxFeeds {
		return fmt.Errorf("behavior.chatter.feeds has %d entries, at most %d are allowed", n, MaxFeeds)
	}
	for i, f := range c.Behavior.Chatter.Feeds {
		if !feed.Openable(f.URL) {
			return fmt.Errorf("behavior.chatter.feeds[%d].url must be an http or https URL, got %q", i, f.URL)
		}
		if len([]rune(f.Name)) > MaxFeedName {
			return fmt.Errorf("behavior.chatter.feeds[%d].name is longer than %d characters", i, MaxFeedName)
		}
		if strings.ContainsAny(f.Name, "\n\r") {
			return fmt.Errorf("behavior.chatter.feeds[%d].name must be a single line", i)
		}
	}
	if !validSmoothing(c.Pet.Smooth) {
		return fmt.Errorf("pet.smooth must be one of %v, got %q", Smoothings, c.Pet.Smooth)
	}
	if c.Message.DurationSec <= 0 {
		return fmt.Errorf("message.duration_sec must be positive, got %v", c.Message.DurationSec)
	}
	if c.Message.MaxVisible <= 0 {
		return fmt.Errorf("message.max_visible must be positive, got %d", c.Message.MaxVisible)
	}
	if c.Message.MaxWidth <= 0 {
		return fmt.Errorf("message.max_width must be positive, got %d", c.Message.MaxWidth)
	}
	if c.Message.MaxQueue <= 0 {
		return fmt.Errorf("message.max_queue must be positive, got %d", c.Message.MaxQueue)
	}
	if c.Message.TypeSpeed < 0 {
		return fmt.Errorf("message.type_speed must not be negative, got %v", c.Message.TypeSpeed)
	}
	if c.Message.TextScale <= 0 {
		return fmt.Errorf("message.text_scale must be positive, got %v", c.Message.TextScale)
	}
	if c.History.Max <= 0 {
		return fmt.Errorf("history.max must be positive, got %d", c.History.Max)
	}
	if c.History.Hours < 0 {
		return fmt.Errorf("history.hours must not be negative, got %v", c.History.Hours)
	}
	return nil
}

func validSmoothing(s Smoothing) bool {
	for _, valid := range Smoothings {
		if s == valid {
			return true
		}
	}
	return false
}

func validAnchor(a Anchor) bool {
	for _, valid := range Anchors {
		if a == valid {
			return true
		}
	}
	return false
}

func validMode(m Mode) bool {
	for _, valid := range Modes {
		if m == valid {
			return true
		}
	}
	return false
}

func validRoam(r Roam) bool {
	for _, valid := range Roams {
		if r == valid {
			return true
		}
	}
	return false
}

// DefaultPath is where gumpet keeps its config file. $GUMPET_CONFIG overrides
// it, which is handy for running two pets side by side.
func DefaultPath() (string, error) {
	if p := os.Getenv("GUMPET_CONFIG"); p != "" {
		return p, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate user config directory: %w", err)
	}
	return filepath.Join(dir, "gumpet", "config.yaml"), nil
}

// Load reads the config at path. A missing file is not an error: the annotated
// defaults are written there and returned, so the first run leaves the user
// something to edit.
func Load(path string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		// A read-only config directory should not stop the pet from running.
		_ = Save(path, cfg)
		return cfg, nil
	}
	if err != nil {
		return cfg, fmt.Errorf("read %s: %w", path, err)
	}
	// Unmarshalling over the defaults leaves omitted keys alone.
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return cfg, fmt.Errorf("%s: %w", path, err)
	}
	return cfg, nil
}

// Save writes cfg to path, comments and all, by rendering it through the same
// annotated template the first run uses. The write goes through a temporary
// file so an interrupted save cannot leave a half-written config behind.
func Save(path string, cfg Config) error {
	out, err := Render(cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.yaml")
	if err != nil {
		return fmt.Errorf("create temporary config: %w", err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.WriteString(out); err != nil {
		tmp.Close()
		return fmt.Errorf("write temporary config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write temporary config: %w", err)
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return fmt.Errorf("set config permissions: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}
