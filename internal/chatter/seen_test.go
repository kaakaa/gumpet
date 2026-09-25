package chatter

import "testing"

func TestSeenMarksAHeadlineTheSecondTime(t *testing.T) {
	var s Seen
	a := Remark{Text: "Go 2 is out", Link: "https://go.dev/blog/go2"}
	b := Remark{Text: "Something else", Link: "https://example.com/else"}

	steps := []struct {
		r    Remark
		want bool
	}{
		{a, false}, // new
		{b, false}, // new
		{a, true},  // again
		{a, true},  // and again
		// The same article under a new title is still the same article.
		{Remark{Text: "Go 2 is finally out", Link: "https://go.dev/blog/go2"}, true},
	}
	for i, st := range steps {
		if got := s.Mark(st.r); got != st.want {
			t.Errorf("step %d (%q): seen before = %v, want %v", i, st.r.Text, got, st.want)
		}
	}
}

// Sayings repeat by design and have no link to know them by; marking them
// would mark nearly every one.
func TestSeenLeavesSayingsAlone(t *testing.T) {
	var s Seen
	saying := Remark{Text: "hello"}
	for i := range 3 {
		if s.Mark(saying) {
			t.Errorf("saying %d marked as seen", i)
		}
	}
}
