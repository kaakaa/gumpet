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
}
