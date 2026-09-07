// Package config loads and persists gumpet's user settings.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
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
	// ModeOnMessage shows the pet only while a message is being displayed.
	ModeOnMessage Mode = "on-message"
)

// Config is the whole of gumpet's settings.
type Config struct {
	Server   Server   `yaml:"server" json:"server"`
	Window   Window   `yaml:"window" json:"window"`
	Stage    Stage    `yaml:"stage" json:"stage"`
	Pet      Pet      `yaml:"pet" json:"pet"`
	Behavior Behavior `yaml:"behavior" json:"behavior"`
	Message  Message  `yaml:"message" json:"message"`
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
	// Fullscreen lets the pet roam the whole monitor, ignoring the size and
	// placement below.
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
}

// Behavior is how the pet acts between messages.
type Behavior struct {
	Mode Mode `yaml:"mode" json:"mode"`
	// Roam is how the pet moves around the stage.
	Roam Roam `yaml:"roam" json:"roam"`
	// Speed is the walking speed in pixels per second.
	Speed float64 `yaml:"speed" json:"speed"`
}

// Message controls how long text stays up and how big it is drawn.
type Message struct {
	DurationSec float64 `yaml:"duration_sec" json:"duration_sec"`
	// MaxWidth is how wide the speech balloon may grow before the text wraps,
	// in pixels.
	MaxWidth int `yaml:"max_width" json:"max_width"`
	// MaxQueue caps the backlog of messages waiting their turn. The oldest are
	// dropped once it is full.
	MaxQueue  int     `yaml:"max_queue" json:"max_queue"`
	TextScale float64 `yaml:"text_scale" json:"text_scale"`
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
		},
		Behavior: Behavior{
			Mode:  ModeAlways,
			Roam:  RoamHorizontal,
			Speed: 45,
		},
		Message: Message{
			DurationSec: 8,
			MaxWidth:    520,
			MaxQueue:    20,
			TextScale:   2,
		},
	}
}

// ShowsPet reports whether the pet should be on screen, given whether it has
// something to say and whether its menu is open. In "on-message" mode the pet
// is drawn only when one of those is true.
func (c Config) ShowsPet(hasMessage, menuOpen bool) bool {
	if c.Behavior.Mode != ModeOnMessage {
		return true
	}
	return hasMessage || menuOpen
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
	if c.Stage.Width <= 0 || c.Stage.Height <= 0 {
		return fmt.Errorf("stage.width and stage.height must be positive, got %dx%d", c.Stage.Width, c.Stage.Height)
	}
	if c.Pet.Scale <= 0 {
		return fmt.Errorf("pet.scale must be positive, got %v", c.Pet.Scale)
	}
	if c.Pet.FPS <= 0 {
		return fmt.Errorf("pet.fps must be positive, got %v", c.Pet.FPS)
	}
	switch c.Behavior.Mode {
	case ModeAlways, ModeOnMessage:
	default:
		return fmt.Errorf("behavior.mode %q is not one of always, on-message", c.Behavior.Mode)
	}
	if !validRoam(c.Behavior.Roam) {
		return fmt.Errorf("behavior.roam %q is not one of none, horizontal, perimeter, wander", c.Behavior.Roam)
	}
	if c.Behavior.Speed < 0 {
		return fmt.Errorf("behavior.speed must not be negative, got %v", c.Behavior.Speed)
	}
	if c.Message.DurationSec <= 0 {
		return fmt.Errorf("message.duration_sec must be positive, got %v", c.Message.DurationSec)
	}
	if c.Message.MaxWidth <= 0 {
		return fmt.Errorf("message.max_width must be positive, got %d", c.Message.MaxWidth)
	}
	if c.Message.MaxQueue <= 0 {
		return fmt.Errorf("message.max_queue must be positive, got %d", c.Message.MaxQueue)
	}
	if c.Message.TextScale <= 0 {
		return fmt.Errorf("message.text_scale must be positive, got %v", c.Message.TextScale)
	}
	return nil
}

func validAnchor(a Anchor) bool {
	for _, valid := range Anchors {
		if a == valid {
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
