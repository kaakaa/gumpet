package config

import (
	"fmt"
	"strconv"
	"strings"
	"text/template"
)

// configTemplate is the config file gumpet writes: the settings, with the
// explanation of each one kept alongside it. Rendering rather than marshalling
// is what lets the settings page save a change without throwing the comments
// away.
const configTemplate = `# gumpet configuration
#
# Edit this file directly, or open the settings page with "gumpetctl -settings".
# Everything except server.addr and window.skip_taskbar applies immediately.

server:
  # Where gumpet listens for messages and serves its settings page. Keep this
  # on the loopback interface unless you really want the rest of the network
  # talking to your desktop. Takes effect on the next start.
  addr: {{ q .Server.Addr }}
  # When set, API requests must carry it as "Authorization: Bearer <token>" or
  # "X-Gumpet-Token: <token>".
  token: {{ q .Server.Token }}

window:
  always_on_top: {{ .Window.AlwaysOnTop }}
  # Let clicks pass through to whatever is behind the pet. Turning this on
  # means clicking the pet no longer opens its menu.
  click_through: {{ .Window.ClickThrough }}
  # Hide the icon from the Windows taskbar. Takes effect on the next start.
  skip_taskbar: {{ .Window.SkipTaskbar }}

# The stage is the part of the monitor the pet may move around in. gumpet's
# window is only as big as the pet and follows it around the stage, so the
# stage itself costs you no screen space.
stage:
  # Let the pet roam the whole monitor, ignoring everything below.
  fullscreen: {{ .Stage.Fullscreen }}
  width: {{ .Stage.Width }}
  height: {{ .Stage.Height }}
  # top-left | top-right | bottom-left | bottom-right | center | custom
  anchor: {{ .Stage.Anchor }}
  margin_x: {{ .Stage.MarginX }}
  margin_y: {{ .Stage.MarginY }}
  # Monitor coordinates, used only when anchor is "custom".
  x: {{ .Stage.X }}
  y: {{ .Stage.Y }}

pet:
  # A PNG/JPEG file, an animated GIF, or a directory of frames.
  # Empty means the bundled gopher.
  source: {{ q .Pet.Source }}
  scale: {{ n .Pet.Scale }}
  # Frame rate for sources with no timing of their own. Animated GIFs use the
  # delays stored in the file instead.
  fps: {{ n .Pet.FPS }}
  # Mirror the artwork when walking right (the bundled gopher faces left).
  flip_when_facing_right: {{ .Pet.FlipWhenFacingRight }}

behavior:
  # always     - the pet is always on screen
  # faded      - the pet stays on screen but goes faint when it has nothing
  #              to say, so it is there without being in the way
  # on-message - the pet appears only while a message is up
  mode: {{ .Behavior.Mode }}
  # How solid the pet is in "faded" mode while idle: 1 is fully opaque, and
  # lower is fainter. It is drawn in full whenever it has something to say.
  idle_opacity: {{ n .Behavior.IdleOpacity }}
  # How the pet gets around the stage.
  #   none       - stands still
  #   horizontal - walks back and forth along the floor
  #   perimeter  - walks round and round the edge of the stage
  #   wander     - goes wherever it likes
  roam: {{ .Behavior.Roam }}
  # Walking speed, pixels per second.
  speed: {{ n .Behavior.Speed }}

message:
  duration_sec: {{ n .Message.DurationSec }}
  # How many balloons may be on screen at once. Messages that arrive together
  # are shown together, up to this many.
  max_visible: {{ .Message.MaxVisible }}
  # How wide the speech balloon may grow before the text wraps, in pixels.
  max_width: {{ .Message.MaxWidth }}
  # How many messages may wait their turn before the oldest are dropped.
  max_queue: {{ .Message.MaxQueue }}
  # Size of the message text relative to the font's own 12px.
  text_scale: {{ n .Message.TextScale }}

# What the messages page remembers. The record is kept in memory, so it starts
# empty every time gumpet runs.
history:
  # How many messages to keep, newest first.
  max: {{ .History.Max }}
  # How long to keep them, in hours. 0 keeps them until max is reached.
  hours: {{ n .History.Hours }}
`

var tmpl = template.Must(template.New("config").Funcs(template.FuncMap{
	"q": yamlString,
	"n": yamlFloat,
}).Parse(configTemplate))

// Render turns cfg into the annotated config file.
func Render(cfg Config) (string, error) {
	var out strings.Builder
	if err := tmpl.Execute(&out, cfg); err != nil {
		return "", fmt.Errorf("render config: %w", err)
	}
	return out.String(), nil
}

// DefaultYAML is the annotated config file with nothing changed.
func DefaultYAML() string {
	// Rendering the defaults cannot fail; the template and Config are both
	// fixed at compile time, and TestRenderRoundTrips proves it.
	out, err := Render(Default())
	if err != nil {
		panic(err)
	}
	return out
}

// yamlString quotes a string so that any path, including one with spaces or
// non-ASCII characters, survives the round trip.
func yamlString(s string) string {
	return strconv.Quote(s)
}

// yamlFloat writes a float without an exponent or a trailing ".0", so a scale
// of 1 reads as "1" rather than "1e+00".
func yamlFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}
