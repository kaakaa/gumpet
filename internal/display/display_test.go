package display

import "testing"

func TestPick(t *testing.T) {
	tests := []struct {
		name            string
		want, available int
		index           int
	}{
		{"the first, which is the default", 1, 2, 0},
		{"the second", 2, 2, 1},
		{"the third of three", 3, 3, 2},
		{"only one monitor", 1, 1, 0},
		{"asking for a monitor that was unplugged", 2, 1, 0},
		{"asking for one far beyond what is there", 99, 2, 0},
		{"counting from zero by mistake", 0, 2, 0},
		{"a negative number", -1, 2, 0},
		{"no monitors reported at all", 1, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Pick(tt.want, tt.available); got != tt.index {
				t.Errorf("Pick(%d, %d) = %d, want %d", tt.want, tt.available, got, tt.index)
			}
		})
	}
}

// Whatever it returns has to be safe to index a slice of that length with.
func TestPickNeverIndexesPastTheMonitorsThereAre(t *testing.T) {
	for available := range 5 {
		for want := -2; want < 8; want++ {
			got := Pick(want, available)
			if got < 0 {
				t.Fatalf("Pick(%d, %d) = %d, which is negative", want, available, got)
			}
			if available > 0 && got >= available {
				t.Fatalf("Pick(%d, %d) = %d, past the %d monitors there are", want, available, got, available)
			}
		}
	}
}

func TestLabelNamesTheNumberToSet(t *testing.T) {
	m := Monitor{Number: 2, Name: "ASUS PB278", Width: 2560, Height: 1440}
	if got, want := m.Label(), "2. ASUS PB278 (2560×1440)"; got != want {
		t.Errorf("Label = %q, want %q", got, want)
	}
}
