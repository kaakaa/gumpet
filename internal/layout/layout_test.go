package layout

import (
	"testing"

	"github.com/kaakaa/gumpet/internal/config"
)

var monitor = Rect{W: 1920, H: 1080}

func TestStageAnchors(t *testing.T) {
	stage := config.Stage{Width: 520, Height: 360, MarginX: 24, MarginY: 24, X: 100, Y: 200}

	tests := []struct {
		anchor config.Anchor
		x, y   float64
	}{
		{config.AnchorTopLeft, 24, 24},
		{config.AnchorTopRight, 1920 - 520 - 24, 24},
		{config.AnchorBottomLeft, 24, 1080 - 360 - 24},
		{config.AnchorBottomRight, 1920 - 520 - 24, 1080 - 360 - 24},
		{config.AnchorCenter, (1920 - 520) / 2, (1080 - 360) / 2},
		{config.AnchorCustom, 100, 200},
	}
	for _, tt := range tests {
		t.Run(string(tt.anchor), func(t *testing.T) {
			stage.Anchor = tt.anchor
			got := Stage(stage, 1920, 1080)
			if got.X != tt.x || got.Y != tt.y {
				t.Errorf("origin = (%v, %v), want (%v, %v)", got.X, got.Y, tt.x, tt.y)
			}
			if got.W != 520 || got.H != 360 {
				t.Errorf("size = %vx%v, want 520x360", got.W, got.H)
			}
		})
	}
}

func TestStageFullscreenCoversTheMonitor(t *testing.T) {
	stage := config.Default().Stage
	stage.Fullscreen = true

	got := Stage(stage, 1920, 1080)
	want := Rect{W: 1920, H: 1080}
	if got != want {
		t.Errorf("Stage = %+v, want the whole monitor %+v", got, want)
	}
}

func TestStageIsNeverBiggerThanTheMonitor(t *testing.T) {
	stage := config.Stage{Width: 4000, Height: 4000, Anchor: config.AnchorBottomRight}

	got := Stage(stage, 1920, 1080)
	if got.W > 1920 || got.H > 1080 {
		t.Errorf("Stage = %+v, larger than the monitor", got)
	}
}

func TestDefaultStageLandsInTheBottomRight(t *testing.T) {
	cfg := config.Default()
	got := Stage(cfg.Stage, 1920, 1080)
	if got.X <= 1920/2 || got.Y <= 1080/2 {
		t.Errorf("default stage at (%v, %v), want the bottom-right quadrant", got.X, got.Y)
	}
	if got.X+got.W > 1920 || got.Y+got.H > 1080 {
		t.Errorf("default stage %+v runs off the monitor", got)
	}
}

const (
	petW = 200.0
	petH = 200.0
	gap  = 6.0
)

func TestWindowWithNoPanelIsJustThePet(t *testing.T) {
	win := PlaceWindow(500, 600, petW, petH, Panel{}, gap, monitor)

	if win.W != petW || win.H != petH {
		t.Errorf("size = %vx%v, want the pet's %vx%v", win.W, win.H, petW, petH)
	}
	if win.X != 500 || win.Y != 600 {
		t.Errorf("origin = (%v, %v), want the pet's (500, 600)", win.X, win.Y)
	}
	if win.PetX != 0 || win.PetY != 0 {
		t.Errorf("pet offset = (%v, %v), want (0, 0)", win.PetX, win.PetY)
	}
}

func TestWindowGrowsUpwardsForAPanel(t *testing.T) {
	panel := Panel{W: 400, H: 90}
	win := PlaceWindow(500, 600, petW, petH, panel, gap, monitor)

	if win.W != panel.W {
		t.Errorf("width = %v, want the panel's %v", win.W, panel.W)
	}
	if want := petH + panel.H + gap; win.H != want {
		t.Errorf("height = %v, want %v", win.H, want)
	}
	// The pet must still land exactly where the caller asked.
	if got := win.X + win.PetX; got != 500 {
		t.Errorf("pet is at x = %v on the monitor, want 500", got)
	}
	if got := win.Y + win.PetY; got != 600 {
		t.Errorf("pet is at y = %v on the monitor, want 600", got)
	}
	if win.PanelY != 0 {
		t.Errorf("panel y = %v, want it at the top of the window", win.PanelY)
	}
}

func TestPanelIsCentredOnThePet(t *testing.T) {
	panel := Panel{W: 400, H: 90}
	win := PlaceWindow(500, 600, petW, petH, panel, gap, monitor)

	petCentre := win.PetX + petW/2
	panelCentre := win.PanelX + panel.W/2
	if petCentre != panelCentre {
		t.Errorf("panel centre %v, pet centre %v", panelCentre, petCentre)
	}
	if win.TailX != petCentre {
		t.Errorf("TailX = %v, want the pet's centre %v", win.TailX, petCentre)
	}
}

// A pet in a corner is the case where the naive placement would push half the
// balloon off the monitor.
func TestWindowStaysOnTheMonitor(t *testing.T) {
	panel := Panel{W: 500, H: 120}

	tests := []struct {
		name       string
		petX, petY float64
	}{
		{"bottom right", monitor.W - petW, monitor.H - petH},
		{"bottom left", 0, monitor.H - petH},
		{"top left", 0, 0},
		{"top right", monitor.W - petW, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			win := PlaceWindow(tt.petX, tt.petY, petW, petH, panel, gap, monitor)

			if win.X < monitor.X || win.X+win.W > monitor.X+monitor.W {
				t.Errorf("window spans x %v..%v, off a monitor %v wide", win.X, win.X+win.W, monitor.W)
			}
			if win.Y < monitor.Y || win.Y+win.H > monitor.Y+monitor.H {
				t.Errorf("window spans y %v..%v, off a monitor %v tall", win.Y, win.Y+win.H, monitor.H)
			}
			// Clamping the window must not drag the pet away from its position.
			if got := win.X + win.PetX; got != tt.petX {
				t.Errorf("pet x = %v, want %v", got, tt.petX)
			}
			if got := win.Y + win.PetY; got != tt.petY {
				t.Errorf("pet y = %v, want %v", got, tt.petY)
			}
			// And the panel must stay inside the window it was sized for.
			if win.PanelX < 0 || win.PanelX+panel.W > win.W {
				t.Errorf("panel spans %v..%v inside a window %v wide", win.PanelX, win.PanelX+panel.W, win.W)
			}
			if win.PanelY < 0 || win.PanelY+panel.H > win.H {
				t.Errorf("panel spans %v..%v inside a window %v tall", win.PanelY, win.PanelY+panel.H, win.H)
			}
		})
	}
}

func TestTailStaysWithinThePanel(t *testing.T) {
	// A narrow panel beside a pet at the edge is where the tail would otherwise
	// end up pointing at thin air.
	panel := Panel{W: 80, H: 60}
	win := PlaceWindow(monitor.W-petW, monitor.H-petH, petW, petH, panel, gap, monitor)

	if win.TailX < win.PanelX || win.TailX > win.PanelX+panel.W {
		t.Errorf("TailX = %v, outside the panel spanning %v..%v", win.TailX, win.PanelX, win.PanelX+panel.W)
	}
}

func TestWindowOnASecondMonitor(t *testing.T) {
	// A monitor to the right of the primary one: nothing may assume an origin
	// of (0, 0).
	second := Rect{X: 1920, Y: 0, W: 1280, H: 800}
	panel := Panel{W: 400, H: 90}

	win := PlaceWindow(second.X+second.W-petW, second.Y+second.H-petH, petW, petH, panel, gap, second)

	if win.X < second.X || win.X+win.W > second.X+second.W {
		t.Errorf("window spans x %v..%v, off the monitor %+v", win.X, win.X+win.W, second)
	}
}

func TestClamp(t *testing.T) {
	tests := []struct{ v, lo, hi, want float64 }{
		{5, 0, 10, 5},
		{-1, 0, 10, 0},
		{11, 0, 10, 10},
		{5, 10, 0, 10}, // empty range: lo wins
	}
	for _, tt := range tests {
		if got := Clamp(tt.v, tt.lo, tt.hi); got != tt.want {
			t.Errorf("Clamp(%v, %v, %v) = %v, want %v", tt.v, tt.lo, tt.hi, got, tt.want)
		}
	}
}
