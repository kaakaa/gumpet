package pet

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"os"
	"time"

	"github.com/kaakaa/gumpet/internal/chatter"
	"github.com/kaakaa/gumpet/internal/config"
	"github.com/kaakaa/gumpet/internal/feed"
	"github.com/kaakaa/gumpet/internal/message"
	"github.com/kaakaa/gumpet/internal/quiet"
)

// setQuietWindow adopts the configured window. The config has already been
// validated, so a window that will not parse means the file changed under us;
// having no quiet hours is the safe reading of that, since the alternative is
// a pet that goes silent for no reason anyone can see.
func (g *Game) setQuietWindow(cfg config.Quiet) {
	w, err := quiet.Parse(cfg.From, cfg.To)
	if err != nil {
		g.log.Error("ignoring the quiet window", "error", err)
		w = quiet.Window{}
	}
	g.hush.SetWindow(w)
}

// advanceQuiet asks whether it is quiet now, and deals with the two moments
// that matter: quiet beginning, and quiet ending with messages held.
//
// Held messages are not queued for later. They are already on the messages
// page, which is where they are meant to be read; queueing them would either
// overflow message.max_queue the moment quiet ended or change what the queue
// is for. The pet says once how many there were, and where.
func (g *Game) advanceQuiet() {
	hushed, ended := g.hush.Update(time.Now())
	if hushed && !g.wasQuiet {
		// Anything still waiting its turn when quiet begins is held with the
		// rest. Only then, not every frame: the menu's own test message goes
		// straight into the queue, and someone who asks the pet to say
		// something during quiet hours means it.
		for _, msg := range g.queue {
			g.hold(msg)
		}
		g.queue = g.queue[:0]
		g.log.Info("quiet hours begin")
	}
	g.wasQuiet = hushed

	if ended > 0 {
		g.log.Info("quiet hours end", "held", ended)
		g.enqueue(message.Message{
			Title: g.tr("While it was quiet"),
			Text:  quiet.Summary(ended, "http://"+g.cfg.Server.Addr+"/messages", g.tr),
			Level: message.LevelInfo,
			At:    time.Now(),
		})
	}
}

// advanceChatter lets the pet say something of its own accord.
//
// A remark is put straight on screen rather than into the queue: the queue is
// for messages somebody sent, and a remark must not take a place in it or push
// a real message out of one. For the same reason it is recorded apart from
// them, in a store with its own limit: a pet reading out headlines says a
// great deal, and sharing one record would age real messages out of it.
func (g *Game) advanceChatter(dt time.Duration) {
	g.ensureChatter()
	if g.chatter == nil {
		return
	}

	// Anything else on screen or waiting means the pet has better to do. So
	// does an open menu: interrupting someone reading it would be rude.
	// Quiet hours silence the pet's own remarks too: a pet that keeps the
	// messages to itself and then talks anyway is not being quiet.
	idle := len(g.showing) == 0 && len(g.queue) == 0 && g.menu == nil && !g.hush.Quiet()
	remark, ok := g.chatter.Tick(dt, idle)
	if !ok {
		return
	}
	seen := g.seen.Mark(remark)

	g.showing = append(g.showing, shown{
		// The title names the feed a headline came from, so that two feeds
		// mixed together can still be told apart at a glance. A saying out of
		// a file has none, and the balloon simply gets no heading.
		msg: message.Message{
			Text:  remark.Said(),
			Title: remark.Title,
			Level: message.LevelInfo,
			At:    remark.At,
			Seen:  seen,
		},
		remaining: time.Duration(g.cfg.Message.DurationSec * float64(time.Second)),
		idle:      true,
	})
	g.panelDirty = true
	g.resetAnimation()

	// A balloon goes by in seconds, and a headline is often the one thing on
	// screen worth following up — usually noticed just as it goes.
	if g.remarks != nil {
		g.remarks.AddRemark(remark, seen)
	}
}

// ensureChatter builds the Sayer when the settings behind it change, and drops
// it when the feature is off.
//
// In "on-message" mode the pet is hidden until something arrives, so a remark
// would summon it onto the screen and defeat the mode. It stays quiet there
// whatever the chatter setting says.
func (g *Game) ensureChatter() {
	want := g.cfg.Behavior.Chatter
	if !want.Enabled || g.cfg.Behavior.Mode == config.ModeOnMessage {
		g.chatter, g.chatterSrc = nil, config.Chatter{}
		return
	}
	if g.chatter != nil && sameChatter(g.chatterSrc, want) {
		return
	}

	sayings, from := g.sayings(want.Source)
	g.localSayings = sayings
	g.chatterSrc = want
	g.chatter = chatter.New(
		sayings,
		time.Duration(want.IntervalSec*float64(time.Second)),
		rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())),
	)
	if len(want.Feeds) > 0 {
		// Nothing is fetched yet: the pet talks from the local list until the
		// first fetch lands, rather than standing mute waiting for a network
		// round trip.
		from = fmt.Sprintf("%d feed(s), not fetched yet", len(want.Feeds))
		g.fetchWait = 0
	}
	g.log.Info("idle chatter on", "sayings", len(sayings), "from", from,
		"every", time.Duration(want.IntervalSec*float64(time.Second)))
}

// advanceFeed re-reads the configured feed now and then, in the background.
//
// The game loop must not wait for the network, so the fetch runs in a
// goroutine and posts its result to a channel that the loop drains. A fetch
// that fails, hangs or returns rubbish costs the pet nothing: it goes on
// saying whatever it already had.
func (g *Game) advanceFeed(dt time.Duration) {
	cfg := g.cfg.Behavior.Chatter

	select {
	case groups := <-g.headlines:
		g.fetching = false
		if g.chatter != nil {
			if len(groups) == 0 {
				// Every feed was unreachable, or had nothing recent enough.
				// Go back to the sayings rather than keeping headlines that
				// are no longer in any feed — silence, or stale news, would
				// both be worse than a proverb.
				g.log.Info("no feed had anything to say, falling back to the sayings")
				groups = [][]chatter.Remark{g.localSayings}
			}
			g.chatter.SetGroups(groups)
		}
	default:
	}

	if !cfg.Enabled || len(cfg.Feeds) == 0 || g.chatter == nil || g.fetching {
		return
	}
	if g.fetchWait -= dt; g.fetchWait > 0 {
		return
	}
	g.fetchWait = time.Duration(cfg.FetchIntervalSec * float64(time.Second))
	g.fetching = true

	// Copied, because the settings can change while the fetch is in flight.
	feeds := append([]config.Feed(nil), cfg.Feeds...)
	maxAge := time.Duration(cfg.MaxAgeDays * float64(24*time.Hour))
	log := g.log
	out := g.headlines
	reports := g.feedReports
	go func() {
		out <- fetchAll(feeds, maxAge, reports, log)
	}()
}

// fetchAll reads the configured feeds and turns each one's headlines into a
// group of remarks labelled with that feed's name.
func fetchAll(feeds []config.Feed, maxAge time.Duration, reports *feed.Reports, log *slog.Logger) [][]chatter.Remark {
	sources := make([]feed.Source, len(feeds))
	for i, f := range feeds {
		sources[i] = feed.Source{Name: f.Name, URL: f.URL}
	}

	var groups [][]chatter.Remark
	fetcher := feed.NewFetcher()
	fetcher.Reports = reports
	for _, g := range fetcher.FetchAll(context.Background(), sources, maxAge, log) {
		remarks := make([]chatter.Remark, 0, len(g.Items))
		for _, it := range g.Items {
			// Published is carried rather than written into the text: the
			// balloon draws it small and off to the side, and a date in the
			// middle of a headline would be read as part of it.
			remarks = append(remarks, chatter.Remark{Text: it.Title, Title: g.Name, At: it.Published, Link: it.Link})
		}
		groups = append(groups, remarks)
	}
	return groups
}

// sameChatter reports whether two chatter settings would build the same Sayer.
// config.Chatter holds a slice, so it cannot be compared with ==.
func sameChatter(a, b config.Chatter) bool {
	if a.Enabled != b.Enabled || a.IntervalSec != b.IntervalSec ||
		a.Source != b.Source || a.FetchIntervalSec != b.FetchIntervalSec ||
		a.MaxAgeDays != b.MaxAgeDays || len(a.Feeds) != len(b.Feeds) {
		return false
	}
	for i := range a.Feeds {
		if a.Feeds[i] != b.Feeds[i] {
			return false
		}
	}
	return true
}

// sayings reads the configured file, falling back to the bundled list. A
// missing or unreadable file is worth a line in the log and nothing more: the
// pet should carry on talking, not stop working over a list of proverbs.
func (g *Game) sayings(path string) ([]chatter.Remark, string) {
	if path == "" {
		return chatter.Bundled(), "the bundled list"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		g.log.Error("could not read the sayings file, using the bundled list",
			"path", path, "error", err)
		return chatter.Bundled(), "the bundled list"
	}
	sayings := chatter.Parse(data)
	if len(sayings) == 0 {
		g.log.Error("the sayings file has nothing in it, using the bundled list", "path", path)
		return chatter.Bundled(), "the bundled list"
	}
	return sayings, path
}
