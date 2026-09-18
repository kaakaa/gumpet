package chatter

import (
	"math/rand/v2"
	"strings"
	"testing"
	"time"
)

// seeded gives every test the same stream, so a failure can be reproduced from
// the test name alone.
func seeded() *rand.Rand { return rand.New(rand.NewPCG(1, 2)) }

func TestParseTakesOneSayingPerLine(t *testing.T) {
	got := Parse([]byte("# a comment\n\nfirst\n  second  \n\n# another\nthird\n"))
	want := []string{"first", "second", "third"}
	if len(got) != len(want) {
		t.Fatalf("Parse = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Parse[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestParseOfNothingIsNothing(t *testing.T) {
	for _, in := range []string{"", "\n\n\n", "# only comments\n# and more\n"} {
		if got := Parse([]byte(in)); len(got) != 0 {
			t.Errorf("Parse(%q) = %q, want nothing", in, got)
		}
	}
}

func TestBundledListIsUsable(t *testing.T) {
	got := Bundled()
	if len(got) < 2 {
		t.Fatalf("the bundled list has %d sayings, want several", len(got))
	}
	for _, s := range got {
		if strings.TrimSpace(s) == "" || strings.HasPrefix(s, "#") {
			t.Errorf("bundled list contains %q, which should have been filtered out", s)
		}
	}
}

func TestSaysNothingBeforeTheIntervalIsUp(t *testing.T) {
	s := New([]string{"one", "two"}, time.Minute, seeded())

	// Well inside even the shortest jittered interval.
	for i := 0; i < 20; i++ {
		if got, ok := s.Tick(time.Second, true); ok {
			t.Fatalf("said %q after %ds, want silence until about a minute", got, i+1)
		}
	}
}

func TestSaysSomethingOnceTheIntervalIsUp(t *testing.T) {
	s := New([]string{"one", "two"}, time.Minute, seeded())

	var said string
	for i := 0; i < 200; i++ {
		if got, ok := s.Tick(time.Second, true); ok {
			said = got
			break
		}
	}
	if said != "one" && said != "two" {
		t.Errorf("said %q, want one of the sayings", said)
	}
}

// The pet should not start muttering the moment a real message finishes: being
// spoken to resets the wait rather than merely pausing it.
func TestBusyTimeDoesNotCountTowardsTheNextRemark(t *testing.T) {
	s := New([]string{"one", "two"}, time.Minute, seeded())

	for i := 0; i < 500; i++ {
		if got, ok := s.Tick(time.Second, false); ok {
			t.Fatalf("said %q while busy, want silence", got)
		}
	}
	// Now quiet, but the wait starts over: a full interval must still pass.
	for i := 0; i < 20; i++ {
		if got, ok := s.Tick(time.Second, true); ok {
			t.Fatalf("said %q %ds after going quiet, want it to wait out a fresh interval", got, i+1)
		}
	}
}

func TestDoesNotSayTheSameThingTwiceRunning(t *testing.T) {
	s := New([]string{"one", "two", "three"}, time.Second, seeded())

	var previous string
	for said := 0; said < 200; {
		got, ok := s.Tick(100*time.Millisecond, true)
		if !ok {
			continue
		}
		if got == previous {
			t.Fatalf("said %q twice running", got)
		}
		previous = got
		said++
	}
}

// With one saying there is nothing else to pick, and it must not spin looking
// for one.
func TestASingleSayingIsRepeated(t *testing.T) {
	s := New([]string{"only"}, time.Second, seeded())

	for said := 0; said < 5; {
		if got, ok := s.Tick(100*time.Millisecond, true); ok {
			if got != "only" {
				t.Fatalf("said %q, want the only saying there is", got)
			}
			said++
		}
	}
}

func TestEverySayingGetsUsed(t *testing.T) {
	sayings := []string{"one", "two", "three", "four"}
	s := New(sayings, time.Second, seeded())

	seen := map[string]bool{}
	for i := 0; i < 5000 && len(seen) < len(sayings); i++ {
		if got, ok := s.Tick(100*time.Millisecond, true); ok {
			seen[got] = true
		}
	}
	for _, want := range sayings {
		if !seen[want] {
			t.Errorf("never said %q", want)
		}
	}
}

// The gap should vary, or the pet sounds like a metronome.
func TestTheGapIsNotAlwaysTheSame(t *testing.T) {
	s := New([]string{"one", "two"}, time.Minute, seeded())

	gaps := map[int]bool{}
	elapsed := 0
	for said := 0; said < 12; {
		elapsed++
		if _, ok := s.Tick(time.Second, true); ok {
			gaps[elapsed] = true
			elapsed = 0
			said++
		}
	}
	if len(gaps) < 4 {
		t.Errorf("only %d distinct gaps in 12 remarks, want the interval to vary", len(gaps))
	}
}

// Jitter must not turn into a wildly different cadence.
func TestTheGapStaysNearTheConfiguredInterval(t *testing.T) {
	s := New([]string{"one", "two"}, time.Minute, seeded())

	elapsed := 0
	for said := 0; said < 50; {
		elapsed++
		if _, ok := s.Tick(time.Second, true); ok {
			if elapsed < 40 || elapsed > 80 {
				t.Errorf("waited %ds, want roughly 60 give or take 30%%", elapsed)
			}
			elapsed = 0
			said++
		}
	}
}

// A file that turned out to be empty, or a feature left off, must leave the pet
// quiet rather than panicking it.
func TestAnEmptyListSaysNothing(t *testing.T) {
	for name, s := range map[string]*Sayer{
		"no sayings":  New(nil, time.Second, seeded()),
		"no interval": New([]string{"one"}, 0, seeded()),
		"nil sayer":   nil,
	} {
		for i := 0; i < 100; i++ {
			if got, ok := s.Tick(time.Second, true); ok {
				t.Fatalf("%s: said %q, want silence", name, got)
			}
		}
	}
}
