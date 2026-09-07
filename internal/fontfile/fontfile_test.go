package fontfile

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("not really a font"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestFirstExistingPrefersTheEarlierCandidate(t *testing.T) {
	dir := t.TempDir()
	second := writeFile(t, dir, "second.ttf")
	first := writeFile(t, dir, "first.ttf")

	got := FirstExisting([]string{filepath.Join(dir, "missing.ttf"), first, second})
	if got != first {
		t.Errorf("FirstExisting = %q, want %q", got, first)
	}
}

func TestFirstExistingWithNothingThere(t *testing.T) {
	if got := FirstExisting([]string{filepath.Join(t.TempDir(), "nope.ttf")}); got != "" {
		t.Errorf("FirstExisting = %q, want empty", got)
	}
}

func TestFirstExistingIgnoresDirectories(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "fonts")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	file := writeFile(t, dir, "real.ttf")

	if got := FirstExisting([]string{sub, file}); got != file {
		t.Errorf("FirstExisting = %q, want the file %q", got, file)
	}
}

func TestResolveUsesTheConfiguredFont(t *testing.T) {
	path := writeFile(t, t.TempDir(), "mine.ttf")

	got, err := Resolve(path, true)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != path {
		t.Errorf("Resolve = %q, want %q", got, path)
	}
}

// A font the user asked for by name and that is not there is a mistake worth
// telling them about, not something to silently replace.
func TestResolveReportsAMissingConfiguredFont(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope.ttf")

	if _, err := Resolve(missing, true); err == nil {
		t.Error("Resolve accepted a font that is not there")
	}
}

func TestResolveWithoutSystemFontsFallsBackToTheBundledOne(t *testing.T) {
	got, err := Resolve("", false)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != "" {
		t.Errorf("Resolve = %q, want empty so the bundled font is used", got)
	}
}

// Not an assertion about any particular machine: only that the list is a
// plausible one, since a typo here is invisible until someone runs it.
func TestSystemCandidatesAreAbsolutePaths(t *testing.T) {
	candidates := SystemCandidates()
	if len(candidates) == 0 {
		t.Fatal("no candidates for this platform")
	}
	for _, path := range candidates {
		if !filepath.IsAbs(path) && path[0] != 'C' {
			t.Errorf("candidate %q is not an absolute path", path)
		}
	}
}
