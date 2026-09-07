// Package message carries the text gumpet's pet displays.
package message

import "time"

// Message is one thing for the pet to say.
type Message struct {
	// ID identifies the message in the history, so the pet can report back
	// when it has actually been on screen. It may be empty for a message that
	// is not being recorded.
	ID   string
	Text string
	// Duration is how long the text stays up. Zero means the configured default.
	Duration time.Duration
}
