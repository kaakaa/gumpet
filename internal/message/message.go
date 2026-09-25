// Package message carries the text gumpet's pet displays.
package message

import (
	"strings"
	"time"
)

// Level is how loud a message is. It decides the colour of the balloon and
// nothing else: a message is not shown sooner, longer or more often for being
// an error.
//
// It exists because the programs that send gumpet messages already know this
// about what they are sending, and without somewhere to put it they resort to
// spelling it into the text — gumpet's own Claude Code hook prefixes "質問: "
// for want of a field.
type Level string

const (
	LevelInfo    Level = "info"
	LevelSuccess Level = "success"
	LevelWarn    Level = "warn"
	LevelError   Level = "error"
)

// ParseLevel reads a level sent over the API. Anything unrecognised, including
// nothing at all, is info: a message with a level nobody understands is still a
// message, and refusing it would lose it.
func ParseLevel(s string) Level {
	switch Level(strings.ToLower(strings.TrimSpace(s))) {
	case LevelSuccess:
		return LevelSuccess
	case LevelWarn:
		return LevelWarn
	case LevelError:
		return LevelError
	default:
		return LevelInfo
	}
}

// Message is one thing for the pet to say.
type Message struct {
	// ID identifies the message in the history, so the pet can report back
	// when it has actually been on screen. It may be empty for a message that
	// is not being recorded.
	ID   string
	Text string
	// Title is a heading drawn above the text, for a sender that wants to say
	// where a message came from without writing it into the message. It may be
	// empty, which is the ordinary case.
	Title string
	// Level colours the balloon. The zero value is not a valid level; use
	// [ParseLevel] to fill it in.
	Level Level
	// Duration is how long the text stays up. Zero means the configured default.
	Duration time.Duration
	// At is the moment the message is about: when a sent message reached
	// gumpet, or when a feed dated the entry a headline came from. It is not
	// when the pet gets round to saying it, which can be a good while later
	// and is nobody's business but the pet's.
	//
	// Zero means there is no such moment — a saying out of a file came from
	// nowhere in particular — and nothing is drawn.
	At time.Time
	// Seen marks a headline the pet has already said while running, so the
	// balloon can say it is a repeat. See [chatter.Seen].
	Seen bool
}

// Copied is what the balloon puts on the clipboard: the heading, if there is
// one, then the text exactly as it was sent.
//
// It is built from the message rather than from what was drawn. The balloon's
// lines were broken to fit its width, and a path or an error message pasted
// back with those breaks in it is broken in a way nobody asked for.
func (m Message) Copied() string {
	if m.Title == "" {
		return m.Text
	}
	return m.Title + "\n" + m.Text
}

// Stamp renders [Message.At] for the small line the balloon puts beside its
// heading. now is passed in rather than read so that the result can be tested.
//
// It is deliberately terse. A balloon is read at a glance and then gone, so
// the date only earns its space once the clock alone would be ambiguous: most
// messages arrived moments ago, and a headline from this morning wants the
// time far more than it wants today's date repeated back.
func Stamp(at, now time.Time) string {
	if at.IsZero() {
		return ""
	}
	// Feeds date their entries in whatever zone they please. Whoever is
	// reading the balloon is in this one.
	at, now = at.Local(), now.Local()

	ay, am, ad := at.Date()
	ny, nm, nd := now.Date()
	switch {
	case ay == ny && am == nm && ad == nd:
		return at.Format("15:04")
	case ay == ny:
		return at.Format("1/2 15:04")
	default:
		return at.Format("2006/1/2 15:04")
	}
}
