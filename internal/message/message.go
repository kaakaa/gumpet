// Package message carries the text gumpet's pet displays.
package message

import "time"

// Message is one thing for the pet to say.
type Message struct {
	Text string
	// Duration is how long the text stays up. Zero means the configured default.
	Duration time.Duration
}
