package pet

import (
	"math"
	"time"

	"github.com/kaakaa/gumpet/internal/layout"
	"github.com/kaakaa/gumpet/internal/message"
	"github.com/kaakaa/gumpet/internal/react"
)

// shown is a message currently on screen and how long it has left.
type shown struct {
	msg       message.Message
	remaining time.Duration
	balloon   *balloon
	// typed is how many characters have appeared so far, kept as a float so a
	// slow speed still advances on a frame that is worth less than one
	// character.
	typed float64
	// idle marks a remark the pet made up itself. It is never recorded, never
	// queued, and gives way the moment a real message arrives.
	idle bool
	// flash counts down the moment after the balloon was copied, while its
	// border says so.
	flash time.Duration
}

// drainInbox takes everything the HTTP server has published since the last tick.
func (g *Game) drainInbox() {
	for {
		select {
		case msg := <-g.inbox:
			if g.hush.Quiet() {
				g.hold(msg)
			} else {
				g.enqueue(msg)
			}
		default:
			return
		}
	}
}

// hold keeps a message back during quiet hours: counted for the summary, and
// marked on the messages page so it does not look like it is still waiting.
func (g *Game) hold(msg message.Message) {
	g.hush.Hold()
	if g.history != nil && msg.ID != "" {
		g.history.MarkHeld(msg.ID)
	}
}

func (g *Game) enqueue(msg message.Message) {
	g.queue = append(g.queue, msg)
	if over := len(g.queue) - g.cfg.Message.MaxQueue; over > 0 {
		g.queue = g.queue[over:]
	}
}

// advanceMessages ages out what is on screen and takes the next messages off
// the queue, oldest first. Several can be up at once: a burst that arrives
// together is shown together rather than made to queue politely.
func (g *Game) advanceMessages(dt time.Duration) {
	// The copy flash runs even while the stack is held: the cursor is on the
	// balloon when it is right-clicked, so waiting for it to leave would leave
	// the border lit for as long as someone kept reading.
	for i := range g.showing {
		if g.showing[i].flash > 0 {
			g.showing[i].flash -= dt
		}
	}

	// The whole stack is held, not just the balloon under the cursor: someone
	// reading one of them is reading the pile, and having the others time out
	// from under it would shuffle the stack while they read.
	if !g.reading {
		g.advanceTyping(dt)

		kept := g.showing[:0]
		for _, s := range g.showing {
			// A question stays until it is answered or withdrawn: someone is
			// waiting on it, and it timing out here would only make them wait
			// for nothing.
			if s.msg.AskID != "" {
				kept = append(kept, s)
				continue
			}
			// A message that is still being said has not started its time on
			// screen yet. Counting it down while it types would give a long
			// message less time to be read than a short one.
			if s.stillTyping() {
				kept = append(kept, s)
				continue
			}
			if s.remaining -= dt; s.remaining > 0 {
				kept = append(kept, s)
			}
		}
		if len(kept) != len(g.showing) {
			g.panelDirty = true
		}
		g.showing = kept
	}

	if len(g.queue) > 0 {
		g.dropIdleTalk()
	}

	arrived := react.None
	for len(g.showing) < g.cfg.Message.MaxVisible && len(g.queue) > 0 {
		msg := g.queue[0]
		arrived = react.Stronger(arrived, react.For(msg.Level))
		g.queue = g.queue[1:]

		remaining := msg.Duration
		if remaining <= 0 {
			remaining = time.Duration(g.cfg.Message.DurationSec * float64(time.Second))
		}
		g.showing = append(g.showing, shown{msg: msg, remaining: remaining})
		g.panelDirty = true

		if g.history != nil && msg.ID != "" {
			g.history.MarkShown(msg.ID)
		}
	}
	g.react(arrived)
	if g.panelDirty {
		g.resetAnimation()
	}
}

// react starts the pet moving in answer to a message just shown. A stronger
// reaction replaces a weaker one under way; a weaker one leaves it be. Held
// by the cursor, the pet does not react: it would jerk out from under it.
func (g *Game) react(k react.Kind) {
	if k == react.None || !g.cfg.Behavior.React || g.drag.Pressed() {
		return
	}
	if g.reaction != react.None && react.Stronger(g.reaction, k) == g.reaction {
		return
	}
	g.reaction, g.reactFor = k, 0
}

// advanceReaction moves a reaction on, and ends it when it is over.
func (g *Game) advanceReaction(dt time.Duration) {
	if g.reaction == react.None {
		return
	}
	if g.drag.Pressed() {
		g.reaction = react.None
		return
	}
	g.reactFor += dt
	if _, _, done := react.Offset(g.reaction, g.reactFor); done {
		g.reaction = react.None
	}
}

// dropIdleTalk takes down anything the pet was saying to itself. A message
// somebody actually sent takes the screen back at once rather than queueing
// behind a proverb.
func (g *Game) dropIdleTalk() {
	kept := g.showing[:0]
	for _, s := range g.showing {
		if !s.idle {
			kept = append(kept, s)
		}
	}
	if len(kept) != len(g.showing) {
		g.panelDirty = true
	}
	g.showing = kept
}

// advanceTyping lets each balloon say a few more characters.
//
// Every balloon types at the same rate but keeps its own count, so several
// arriving together are said at once rather than in turn — which is what makes
// a burst read as a crowd talking.
func (g *Game) advanceTyping(dt time.Duration) {
	speed := g.cfg.Message.TypeSpeed
	for i := range g.showing {
		if speed <= 0 {
			g.showing[i].typed = math.Inf(1)
			continue
		}
		g.showing[i].typed += speed * dt.Seconds()
	}
}

// stillTyping reports whether there is more of this message to appear.
func (s shown) stillTyping() bool {
	return s.balloon != nil && int(s.typed) < s.balloon.runes
}

// revealAll finishes a message at once, for someone who has read it faster
// than the pet can say it.
func (g *Game) revealAll(i int) {
	if i >= 0 && i < len(g.showing) {
		g.showing[i].typed = math.Inf(1)
	}
}

// dismiss takes one balloon off the screen early. Whatever is next in the
// queue takes its place on the following tick, as if it had timed out.
func (g *Game) dismiss(i int) {
	if i < 0 || i >= len(g.showing) {
		return
	}
	g.showing = append(g.showing[:i], g.showing[i+1:]...)
	g.panelDirty = true
}

// rebuildPanel lays the balloons out into one stack above the pet.
func (g *Game) rebuildPanel() {
	if !g.panelDirty {
		return
	}
	g.panelDirty = false

	if len(g.showing) == 0 {
		g.panel, g.placed = layout.Panel{}, nil
		return
	}
	sizes := make([]layout.Size, len(g.showing))
	for i := range g.showing {
		b := g.layoutBalloon(g.showing[i].msg)
		g.showing[i].balloon = b
		sizes[i] = layout.Size{W: b.width, H: b.height}
	}
	g.panel, g.placed = layout.StackBalloons(sizes, balloonSideStep, balloonStackGap)
}
