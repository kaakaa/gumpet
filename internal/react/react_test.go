package react

import (
	"math"
	"testing"
	"time"

	"github.com/kaakaa/gumpet/internal/message"
)

func TestEachLevelHasItsReaction(t *testing.T) {
	cases := map[message.Level]Kind{
		message.LevelError:   Shiver,
		message.LevelWarn:    Hop,
		message.LevelSuccess: Jump,
		message.LevelInfo:    None,
		"":                   None,
	}
	for level, want := range cases {
		if got := For(level); got != want {
			t.Errorf("For(%q) = %v, want %v", level, got, want)
		}
	}
}

// Messages that arrive together get one reaction, and an error among them is
// the one worth noticing.
func TestTheStrongerReactionWins(t *testing.T) {
	cases := []struct{ a, b, want Kind }{
		{None, Hop, Hop},
		{Hop, Jump, Jump},
		{Jump, Shiver, Shiver},
		{Shiver, Hop, Shiver},
		{None, None, None},
	}
	for _, c := range cases {
		if got := Stronger(c.a, c.b); got != c.want {
			t.Errorf("Stronger(%v, %v) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

// Every reaction starts and ends where the pet is, so it never jolts on the
// way in or leaves the pet somewhere else on the way out.
func TestReactionsStartAndEndInPlace(t *testing.T) {
	for _, k := range []Kind{Shiver, Hop, Jump} {
		dx, dy, done := Offset(k, 0)
		if dx != 0 || dy != 0 || done {
			t.Errorf("%v at 0: (%v, %v) done=%v, want in place and under way", k, dx, dy, done)
		}
		dx, dy, done = Offset(k, time.Second)
		if dx != 0 || dy != 0 || !done {
			t.Errorf("%v after a second: (%v, %v) done=%v, want in place and over", k, dx, dy, done)
		}
	}
	if _, _, done := Offset(None, 0); !done {
		t.Error("no reaction is not over at once")
	}
}

// A shiver goes side to side, both ways, never up or down, and dies away.
func TestAShiverTremblesSidewaysAndFades(t *testing.T) {
	var left, right bool
	early, late := 0.0, 0.0
	for ms := 0; ms < 600; ms += 5 {
		dx, dy, _ := Offset(Shiver, time.Duration(ms)*time.Millisecond)
		if dy != 0 {
			t.Fatalf("shiver moved up or down at %dms", ms)
		}
		if math.Abs(dx) > shiverWidth {
			t.Fatalf("shiver went %v wide at %dms, past %v", dx, ms, shiverWidth)
		}
		left, right = left || dx < -1, right || dx > 1
		if ms < 150 {
			early = math.Max(early, math.Abs(dx))
		}
		if ms > 450 {
			late = math.Max(late, math.Abs(dx))
		}
	}
	if !left || !right {
		t.Error("shiver did not go both ways")
	}
	if late >= early {
		t.Errorf("shiver did not die away: %v late against %v early", late, early)
	}
}

// A hop and a jump only go up, peak in the middle, and the jump is the higher.
func TestHopsGoUpAndComeBack(t *testing.T) {
	peak := func(k Kind, d time.Duration) float64 {
		top := 0.0
		for t0 := time.Duration(0); t0 < d; t0 += 5 * time.Millisecond {
			dx, dy, _ := Offset(k, t0)
			if dx != 0 || dy > 0 {
				panic("a hop moved sideways or down")
			}
			top = math.Min(top, dy)
		}
		return -top
	}
	hop, jump := peak(Hop, hopFor), peak(Jump, jumpFor)
	if math.Abs(hop-hopHeight) > 0.5 || math.Abs(jump-jumpHeight) > 0.5 {
		t.Errorf("peaks %v and %v, want about %v and %v", hop, jump, hopHeight, jumpHeight)
	}
	if _, mid, _ := Offset(Jump, jumpFor/2); math.Abs(-mid-jumpHeight) > 0.01 {
		t.Errorf("jump at half time is %v up, want its full height %v", -mid, jumpHeight)
	}
}
