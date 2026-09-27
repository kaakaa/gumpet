package server

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/kaakaa/gumpet/internal/ask"
	"github.com/kaakaa/gumpet/internal/config"
	"github.com/kaakaa/gumpet/internal/history"
)

type askReply struct {
	ID       string `json:"id"`
	Answered bool   `json:"answered"`
	Index    int    `json:"index"`
	Choice   string `json:"choice"`
}

// askServer is a server with a broker, and a pretend pet that does whatever
// answer says with each question it is shown.
func askServer(t *testing.T, token string, answer func(b *ask.Broker, q ask.Question)) *Server {
	t.Helper()
	s, _ := newTestServer(t, config.Server{Addr: "127.0.0.1:0", Token: token}, 1)
	b := ask.NewBroker()
	s.SetAsker(b)
	go func() {
		for q := range b.Questions() {
			answer(b, q)
		}
	}()
	return s
}

func postAsk(t *testing.T, s *Server, body string) (int, askReply) {
	t.Helper()
	rec := do(t, s, http.MethodPost, "/api/v1/ask", "application/json", body, nil)
	var r askReply
	if rec.Code == http.StatusOK {
		if err := json.NewDecoder(rec.Body).Decode(&r); err != nil {
			t.Fatal(err)
		}
	}
	return rec.Code, r
}

func TestAQuestionIsAnsweredWithTheChoiceMade(t *testing.T) {
	s := askServer(t, "", func(b *ask.Broker, q ask.Question) { b.Answer(q.ID, 1) })

	code, r := postAsk(t, s, `{"text":"Deploy to production?","title":"deploy.sh","choices":["Go","Wait"]}`)
	if code != http.StatusOK || !r.Answered || r.Index != 1 || r.Choice != "Wait" {
		t.Fatalf("got %d %+v, want answered with Wait", code, r)
	}

	// The page shows what was asked and what was said.
	var page struct {
		Messages []history.Record `json:"messages"`
	}
	rec := do(t, s, http.MethodGet, "/api/v1/messages", "", "", nil)
	if err := json.NewDecoder(rec.Body).Decode(&page); err != nil {
		t.Fatal(err)
	}
	if len(page.Messages) != 1 || page.Messages[0].Answer != "Wait" || len(page.Messages[0].Choices) != 2 {
		t.Errorf("recorded %+v, want the question with its choices and answer", page.Messages)
	}
}

// With no choices it is a permission request: Allow or Deny, and the answer
// comes back in those words whatever language the buttons were in.
func TestAQuestionWithNoChoicesIsAllowOrDeny(t *testing.T) {
	s := askServer(t, "", func(b *ask.Broker, q ask.Question) {
		if !q.Localize {
			t.Error("default choices were not marked for translation")
		}
		b.Answer(q.ID, 0)
	})
	if _, r := postAsk(t, s, `{"text":"rm -rf build"}`); r.Choice != "Allow" {
		t.Errorf("choice = %q, want Allow", r.Choice)
	}
}

// Nobody answering is not an error: the asker learns there was no answer, and
// does whatever it would have done without gumpet.
func TestAnUnansweredQuestionSaysSo(t *testing.T) {
	s := askServer(t, "", func(*ask.Broker, ask.Question) {})
	code, r := postAsk(t, s, `{"text":"anyone?","timeout_sec":0.05}`)
	if code != http.StatusOK || r.Answered {
		t.Errorf("got %d %+v, want 200 and not answered", code, r)
	}
}

func TestABadQuestionIsRefused(t *testing.T) {
	s := askServer(t, "", func(b *ask.Broker, q ask.Question) { b.Answer(q.ID, 0) })
	for _, body := range []string{
		`{"text":""}`,
		`{"text":"   "}`,
		`{"text":"?","choices":["a","b","c","d","e"]}`,
		`{"text":"?","choices":["", "b"]}`,
		`not json`,
	} {
		if code, _ := postAsk(t, s, body); code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", body, code)
		}
	}
}

// Answering for someone is exactly what a token is for.
func TestAskingNeedsTheToken(t *testing.T) {
	s := askServer(t, "s3cret", func(b *ask.Broker, q ask.Question) { b.Answer(q.ID, 0) })
	if code, _ := postAsk(t, s, `{"text":"?"}`); code != http.StatusUnauthorized {
		t.Errorf("status %d without the token, want 401", code)
	}
}

func TestAServerWithNoBrokerTakesNoQuestions(t *testing.T) {
	s, _ := newTestServer(t, config.Server{Addr: "127.0.0.1:0"}, 1)
	if code, _ := postAsk(t, s, `{"text":"?"}`); code != http.StatusNotFound {
		t.Errorf("status %d, want 404", code)
	}
}
