package pet

import (
	"math"
	"time"

	"github.com/kaakaa/gumpet/internal/message"
	"github.com/kaakaa/gumpet/internal/react"
)

// drainAsks puts new questions on screen and takes down withdrawn ones.
//
// A question goes straight up, rather than into the queue behind messages:
// whoever asked is waiting on it. During quiet hours it is declined at once
// instead, so that an agent falls back to asking in its own terminal without
// waiting out its timeout first.
func (g *Game) drainAsks() {
	if g.asks == nil {
		return
	}
	for {
		select {
		case q := <-g.asks.Questions():
			if g.hush.Quiet() {
				g.asks.Decline(q.ID)
				continue
			}
			labels := make([]string, len(q.Choices))
			for i, c := range q.Choices {
				labels[i] = c
				if q.Localize {
					labels[i] = g.choiceLabel(c)
				}
			}
			g.showing = append(g.showing, shown{
				msg: message.Message{
					ID: q.ID, Text: q.Text, Title: q.Title, Level: q.Level,
					At: time.Now(), AskID: q.ID, Choices: labels,
				},
				// Said all at once: the buttons are what matters, and they
				// should not wait for the text to finish arriving.
				typed: math.MaxInt32,
			})
			if g.history != nil {
				g.history.MarkShown(q.ID)
			}
			// Someone is waiting on this one, which is worth a hop at least.
			g.react(react.For(q.Level))
			g.panelDirty = true
			g.resetAnimation()
		case id := <-g.asks.Withdrawn():
			g.takeDownQuestion(id)
		default:
			return
		}
	}
}

// choiceLabel says one of the default choices in the pet's language.
func (g *Game) choiceLabel(c string) string {
	switch c {
	case "Allow":
		return g.tr("Allow")
	case "Deny":
		return g.tr("Deny")
	}
	return c
}

// answer gives the choice at index as the answer to balloon i's question, and
// takes the balloon down.
func (g *Game) answer(i, index int) {
	if g.asks != nil {
		g.asks.Answer(g.showing[i].msg.AskID, index)
	}
	g.dismiss(i)
}

// takeDownQuestion removes the balloon asking question id, if it is up.
func (g *Game) takeDownQuestion(id string) {
	for i := range g.showing {
		if g.showing[i].msg.AskID == id {
			g.dismiss(i)
			return
		}
	}
}
