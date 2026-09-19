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

// remarks builds a plain group, since most tests do not care about titles.
func remarks(texts ...string) []Remark {
	out := make([]Remark, len(texts))
	for i, t := range texts {
		out[i] = Remark{Text: t}
	}
	return out
}

func TestParseTakesOneSayingPerLine(t *testing.T) {
	got := Parse([]byte("# a comment\n\nfirst\n  second  \n\n# another\nthird\n"))
	want := []string{"first", "second", "third"}
	if len(got) != len(want) {
		t.Fatalf("Parse = %+v, want %q", got, want)
	}
	for i := range want {
		if got[i].Text != want[i] {
			t.Errorf("Parse[%d] = %q, want %q", i, got[i].Text, want[i])
		}
		if got[i].Title != "" {
			t.Errorf("Parse[%d] has title %q, want none", i, got[i].Title)
		}
	}
}

func TestParseOfNothingIsNothing(t *testing.T) {
	for _, in := range []string{"", "\n\n\n", "# only comments\n# and more\n"} {
		if got := Parse([]byte(in)); len(got) != 0 {
			t.Errorf("Parse(%q) = %+v, want nothing", in, got)
		}
	}
}

func TestBundledListIsUsable(t *testing.T) {
	got := Bundled()
	if len(got) < 2 {
		t.Fatalf("the bundled list has %d sayings, want several", len(got))
	}
	for _, r := range got {
		if strings.TrimSpace(r.Text) == "" || strings.HasPrefix(r.Text, "#") {
			t.Errorf("bundled list contains %q, which should have been filtered out", r.Text)
		}
	}
}

func TestSaysNothingBeforeTheIntervalIsUp(t *testing.T) {
	s := New(remarks("one", "two"), time.Minute, seeded())

	// Well inside even the shortest jittered interval.
	for i := 0; i < 20; i++ {
		if got, ok := s.Tick(time.Second, true); ok {
			t.Fatalf("said %q after %ds, want silence until about a minute", got.Text, i+1)
		}
	}
}

func TestSaysSomethingOnceTheIntervalIsUp(t *testing.T) {
	s := New(remarks("one", "two"), time.Minute, seeded())

	var said string
	for i := 0; i < 200; i++ {
		if got, ok := s.Tick(time.Second, true); ok {
			said = got.Text
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
	s := New(remarks("one", "two"), time.Minute, seeded())

	for i := 0; i < 500; i++ {
		if got, ok := s.Tick(time.Second, false); ok {
			t.Fatalf("said %q while busy, want silence", got.Text)
		}
	}
	// Now quiet, but the wait starts over: a full interval must still pass.
	for i := 0; i < 20; i++ {
		if got, ok := s.Tick(time.Second, true); ok {
			t.Fatalf("said %q %ds after going quiet, want it to wait out a fresh interval", got.Text, i+1)
		}
	}
}

func TestDoesNotSayTheSameThingTwiceRunning(t *testing.T) {
	s := New(remarks("one", "two", "three"), time.Second, seeded())

	var previous string
	for said := 0; said < 200; {
		got, ok := s.Tick(100*time.Millisecond, true)
		if !ok {
			continue
		}
		if got.Text == previous {
			t.Fatalf("said %q twice running", got.Text)
		}
		previous = got.Text
		said++
	}
}

// With one saying there is nothing else to pick, and it must not spin looking
// for one.
func TestASingleSayingIsRepeated(t *testing.T) {
	s := New(remarks("only"), time.Second, seeded())

	for said := 0; said < 5; {
		if got, ok := s.Tick(100*time.Millisecond, true); ok {
			if got.Text != "only" {
				t.Fatalf("said %q, want the only saying there is", got.Text)
			}
			said++
		}
	}
}

func TestEverySayingGetsUsed(t *testing.T) {
	sayings := []string{"one", "two", "three", "four"}
	s := New(remarks(sayings...), time.Second, seeded())

	seen := map[string]bool{}
	for i := 0; i < 5000 && len(seen) < len(sayings); i++ {
		if got, ok := s.Tick(100*time.Millisecond, true); ok {
			seen[got.Text] = true
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
	s := New(remarks("one", "two"), time.Minute, seeded())

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
	s := New(remarks("one", "two"), time.Minute, seeded())

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
		"no interval": New(remarks("one"), 0, seeded()),
		"nil sayer":   nil,
	} {
		for i := 0; i < 100; i++ {
			if got, ok := s.Tick(time.Second, true); ok {
				t.Fatalf("%s: said %q, want silence", name, got.Text)
			}
		}
	}
}

// The point of grouping: a feed with thirty headlines and one with three
// should be heard from about equally, rather than in proportion to how much
// each happens to publish.
func TestEachSourceIsHeardFromAboutEqually(t *testing.T) {
	big := make([]Remark, 30)
	for i := range big {
		big[i] = Remark{Text: "big", Title: "BIG"}
	}
	small := []Remark{{Text: "a", Title: "SMALL"}, {Text: "b", Title: "SMALL"}, {Text: "c", Title: "SMALL"}}

	s := NewGrouped([][]Remark{big, small}, time.Second, seeded())

	counts := map[string]int{}
	for said := 0; said < 600; {
		if got, ok := s.Tick(100*time.Millisecond, true); ok {
			counts[got.Title]++
			said++
		}
	}
	// Ten times more material in one, so anything near 50/50 proves the split
	// is by source rather than by item.
	for _, title := range []string{"BIG", "SMALL"} {
		if counts[title] < 200 || counts[title] > 400 {
			t.Errorf("%s said %d times out of 600, want roughly half", title, counts[title])
		}
	}
}

func TestARemarkCarriesItsTitle(t *testing.T) {
	s := NewGrouped([][]Remark{
		{{Text: "headline one", Title: "HN"}, {Text: "headline two", Title: "HN"}},
	}, time.Second, seeded())

	for said := 0; said < 10; {
		if got, ok := s.Tick(100*time.Millisecond, true); ok {
			if got.Title != "HN" {
				t.Fatalf("said %q with title %q, want HN", got.Text, got.Title)
			}
			said++
		}
	}
}

// A feed that returned nothing must not take its turn and produce silence.
func TestAnEmptySourceIsSkipped(t *testing.T) {
	s := NewGrouped([][]Remark{
		{},
		{{Text: "only", Title: "REAL"}},
		nil,
	}, time.Second, seeded())

	for said := 0; said < 20; {
		got, ok := s.Tick(100*time.Millisecond, true)
		if !ok {
			continue
		}
		if got.Text != "only" {
			t.Fatalf("said %q, want the only real remark", got.Text)
		}
		said++
	}
}

func TestSetGroupsSwapsTheMaterialWithoutSayingAnythingSooner(t *testing.T) {
	s := New(remarks("old"), time.Minute, seeded())

	// Run most of the way to the next remark, then swap.
	for i := 0; i < 30; i++ {
		if _, ok := s.Tick(time.Second, true); ok {
			t.Fatal("said something far too early")
		}
	}
	s.SetGroups([][]Remark{{{Text: "new", Title: "FEED"}}})

	// It must still be a while, rather than firing because the list changed.
	for i := 0; i < 5; i++ {
		if got, ok := s.Tick(time.Second, true); ok {
			t.Fatalf("said %q right after the swap, want the countdown left alone", got.Text)
		}
	}
	for said := 0; said < 3; {
		if got, ok := s.Tick(time.Second, true); ok {
			if got.Text != "new" || got.Title != "FEED" {
				t.Fatalf("said %+v, want the new material", got)
			}
			said++
		}
	}
}

// A fetch that came back with nothing must not leave the pet mute.
func TestSetGroupsIgnoresNothingAtAll(t *testing.T) {
	s := New(remarks("kept"), time.Second, seeded())
	s.SetGroups(nil)
	s.SetGroups([][]Remark{{}, {}})

	for said := 0; said < 5; {
		if got, ok := s.Tick(100*time.Millisecond, true); ok {
			if got.Text != "kept" {
				t.Fatalf("said %q, want what it already had", got.Text)
			}
			said++
		}
	}
}
