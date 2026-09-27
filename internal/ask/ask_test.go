package ask

import (
	"context"
	"errors"
	"testing"
	"time"
)

// pet stands in for the game loop: it takes the next question and does
// whatever do says with it.
func pet(b *Broker, do func(q Question)) {
	go func() { do(<-b.Questions()) }()
}

func TestAnAnswerComesBackToTheAsker(t *testing.T) {
	b := NewBroker()
	pet(b, func(q Question) { b.Answer(q.ID, 1) })

	got, err := b.Ask(context.Background(), Question{ID: "q1", Text: "rm -rf build?", Choices: []string{"Yes", "No"}})
	if err != nil {
		t.Fatal(err)
	}
	if got.Index != 1 || got.Label != "No" {
		t.Errorf("answer = %+v, want 1 No", got)
	}
}

// A question with no choices is asking permission, and gets the two an agent
// needs — marked so the pet says them in its own language.
func TestAQuestionWithNoChoicesAsksForPermission(t *testing.T) {
	b := NewBroker()
	var seen Question
	pet(b, func(q Question) { seen = q; b.Answer(q.ID, 0) })

	got, err := b.Ask(context.Background(), Question{ID: "q1", Text: "go test ./..."})
	if err != nil {
		t.Fatal(err)
	}
	if !seen.Localize || len(seen.Choices) != 2 || seen.Choices[0] != "Allow" {
		t.Errorf("pet was shown %+v, want Allow/Deny to be localised", seen)
	}
	// The answer is the canonical English, whatever the button said.
	if got.Label != "Allow" {
		t.Errorf("label = %q, want Allow", got.Label)
	}
}

// Giving up is not "no": the asker gets ErrNoAnswer, and the pet is told to
// take the balloon down.
func TestAnAskerThatStopsWaitingWithdrawsTheQuestion(t *testing.T) {
	b := NewBroker()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	pet(b, func(Question) {}) // the pet shows it, and nobody clicks

	if _, err := b.Ask(ctx, Question{ID: "q1", Text: "?"}); !errors.Is(err, ErrNoAnswer) {
		t.Fatalf("err = %v, want ErrNoAnswer", err)
	}
	select {
	case id := <-b.Withdrawn():
		if id != "q1" {
			t.Errorf("withdrew %q, want q1", id)
		}
	case <-time.After(time.Second):
		t.Fatal("the pet was never told to take the question down")
	}
	// A click on a balloon that lingered must be harmless.
	if b.Answer("q1", 0) {
		t.Error("an answer to a withdrawn question was delivered")
	}
}

// A pet keeping quiet declines at once, rather than making the agent wait out
// its whole timeout before falling back.
func TestADeclinedQuestionEndsAtOnce(t *testing.T) {
	b := NewBroker()
	pet(b, func(q Question) { b.Decline(q.ID) })

	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := b.Ask(ctx, Question{ID: "q1", Text: "?"}); !errors.Is(err, ErrNoAnswer) {
		t.Fatalf("err = %v, want ErrNoAnswer", err)
	}
	if time.Since(start) > time.Second {
		t.Error("a declined question waited for its timeout")
	}
}

// A second click, or an index that is not a choice, delivers nothing.
func TestOnlyOneValidAnswerIsDelivered(t *testing.T) {
	b := NewBroker()
	results := make(chan [3]bool, 1)
	pet(b, func(q Question) {
		results <- [3]bool{b.Answer(q.ID, 5), b.Answer(q.ID, 0), b.Answer(q.ID, 1)}
	})
	got, err := b.Ask(context.Background(), Question{ID: "q1", Text: "?", Choices: []string{"A", "B"}})
	if err != nil || got.Label != "A" {
		t.Fatalf("answer = %+v, %v; want A", got, err)
	}
	if r := <-results; r != [3]bool{false, true, false} {
		t.Errorf("deliveries = %v, want only the first valid one", r)
	}
}

// A pet that is not draining questions — the game loop has stopped — must not
// leave askers blocked forever.
func TestAPetThatIsNotListeningIsBusy(t *testing.T) {
	b := NewBroker()
	for i := range cap(b.questions) {
		go b.Ask(context.Background(), Question{ID: string(rune('a' + i)), Text: "?"})
	}
	// Wait for the buffer to fill.
	deadline := time.Now().Add(time.Second)
	for len(b.questions) < cap(b.questions) && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if _, err := b.Ask(context.Background(), Question{ID: "overflow", Text: "?"}); !errors.Is(err, ErrBusy) {
		t.Errorf("err = %v, want ErrBusy", err)
	}
}
