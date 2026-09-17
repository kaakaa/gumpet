package message

import "testing"

func TestParseLevelAcceptsTheLevelsThatExist(t *testing.T) {
	cases := map[string]Level{
		"info":    LevelInfo,
		"success": LevelSuccess,
		"warn":    LevelWarn,
		"error":   LevelError,
		"ERROR":   LevelError,
		"  warn ": LevelWarn,
	}
	for in, want := range cases {
		if got := ParseLevel(in); got != want {
			t.Errorf("ParseLevel(%q) = %q, want %q", in, got, want)
		}
	}
}

// A sender that makes up a level should still get its message shown, because
// the alternative is dropping a message over a cosmetic field.
func TestParseLevelFallsBackToInfo(t *testing.T) {
	for _, in := range []string{"", "critical", "warning", "URGENT!!", "1"} {
		if got := ParseLevel(in); got != LevelInfo {
			t.Errorf("ParseLevel(%q) = %q, want info", in, got)
		}
	}
}
