// Package ask puts a question to the pet and hands the answer back to
// whoever asked.
//
// The asker is usually an agent waiting on a hook — Claude Code or Codex
// wanting to know whether it may run a command — and it is blocked until an
// answer comes or it stops waiting. The pet is a game loop that must never
// block. This package sits between the two: a question goes into a buffered
// channel the pet drains every frame, and the pet's answer comes back through
// a method that never waits on anyone.
//
// No answer is an answer too, and not the same as "no". A question that goes
// unanswered, is withdrawn, or reaches a pet that is keeping quiet comes back
// as [ErrNoAnswer], and the asker falls back to whatever it would have done
// without gumpet — for an agent, its own prompt in the terminal.
package ask

import (
	"context"
	"errors"
	"sync"

	"github.com/kaakaa/gumpet/internal/message"
)

// ErrNoAnswer means the question ended without a choice being made.
var ErrNoAnswer = errors.New("no answer")

// ErrBusy means the pet is not taking questions: its queue of them is full,
// which only happens when the game loop is not running.
var ErrBusy = errors.New("the pet is not taking questions")

// DefaultChoices are offered when a question names none: the answers an agent
// asking for permission wants. They are English here, and on the wire; the pet
// says them in its own language. See [Question.Localize].
var DefaultChoices = []string{"Allow", "Deny"}

// Question is one thing put to the pet.
type Question struct {
	// ID identifies the question, and is also the id of its record in the
	// history, so the page can show what was asked and what was answered.
	ID    string
	Title string
	Text  string
	Level message.Level
	// Choices are the buttons, in order. The first is the one the eye should
	// land on.
	Choices []string
	// Localize says Choices are [DefaultChoices], and the pet should say them
	// in its language rather than as given.
	Localize bool
}

// Answer is the choice that was made.
type Answer struct {
	Index int
	Label string
}

type pending struct {
	q      Question
	answer chan Answer
}

// Broker carries questions to the pet and answers back.
type Broker struct {
	questions chan Question
	withdrawn chan string

	mu      sync.Mutex
	waiting map[string]pending
}

// NewBroker returns a broker whose pet has room for a few questions at once.
// More than that waiting to be drained means the game loop has stopped.
func NewBroker() *Broker {
	return &Broker{
		questions: make(chan Question, 8),
		withdrawn: make(chan string, 8),
		waiting:   map[string]pending{},
	}
}

// Questions is what the pet drains: questions to put on screen.
func (b *Broker) Questions() <-chan Question { return b.questions }

// Withdrawn is what the pet also drains: questions nobody is waiting on any
// more, whose balloons should go.
func (b *Broker) Withdrawn() <-chan string { return b.withdrawn }

// Ask puts q to the pet and waits for an answer until ctx ends. q.ID must be
// unique among questions still waiting.
func (b *Broker) Ask(ctx context.Context, q Question) (Answer, error) {
	if len(q.Choices) == 0 {
		q.Choices, q.Localize = DefaultChoices, true
	}
	p := pending{q: q, answer: make(chan Answer, 1)}

	b.mu.Lock()
	b.waiting[q.ID] = p
	b.mu.Unlock()

	select {
	case b.questions <- q:
	default:
		b.forget(q.ID)
		return Answer{}, ErrBusy
	}

	select {
	case a, ok := <-p.answer:
		if !ok {
			return Answer{}, ErrNoAnswer
		}
		return a, nil
	case <-ctx.Done():
		if b.forget(q.ID) {
			// Nobody is waiting now, so the balloon asking must go. The pet
			// drains this every frame; if it cannot take it, the balloon is
			// answered into nothing when clicked, which is harmless.
			select {
			case b.withdrawn <- q.ID:
			default:
			}
		}
		return Answer{}, ErrNoAnswer
	}
}

// Answer delivers the choice at index to whoever asked question id. It
// reports false when there is no such question waiting — it was answered
// already, or its asker gave up — or the index is not one of its choices.
func (b *Broker) Answer(id string, index int) bool {
	b.mu.Lock()
	p, ok := b.waiting[id]
	if !ok || index < 0 || index >= len(p.q.Choices) {
		b.mu.Unlock()
		return false
	}
	delete(b.waiting, id)
	b.mu.Unlock()

	p.answer <- Answer{Index: index, Label: p.q.Choices[index]}
	return true
}

// Decline ends question id with no answer, at once. The pet declines what it
// will not show — during quiet hours — so the asker need not wait out its
// timeout to fall back.
func (b *Broker) Decline(id string) {
	b.mu.Lock()
	p, ok := b.waiting[id]
	delete(b.waiting, id)
	b.mu.Unlock()
	if ok {
		close(p.answer)
	}
}

// forget drops question id, reporting whether it was still waiting.
func (b *Broker) forget(id string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	_, ok := b.waiting[id]
	delete(b.waiting, id)
	return ok
}
