package petsrc

import (
	"bytes"
	"image"
	"image/color"
	"image/color/palette"
	"image/gif"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writePNG(t *testing.T, path string, w, h int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.RGBA{R: 0xff, A: 0xff})
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func writeGIF(t *testing.T, path string, frames int) {
	t.Helper()
	g := &gif.GIF{Config: image.Config{Width: 4, Height: 4}}
	for range frames {
		p := image.NewPaletted(image.Rect(0, 0, 4, 4), palette.Plan9)
		g.Image = append(g.Image, p)
		g.Delay = append(g.Delay, 5)
		g.Disposal = append(g.Disposal, gif.DisposalNone)
	}
	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, g); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBuiltinGopher(t *testing.T) {
	src, err := Builtin()
	if err != nil {
		t.Fatalf("Builtin: %v", err)
	}
	if len(src.Walk) != 3 {
		t.Errorf("got %d walk frames, want the gopher's 3", len(src.Walk))
	}
	if len(src.Talk) != 0 {
		t.Errorf("got %d talk frames, want none: gumpet draws its own balloon", len(src.Talk))
	}
	if want := (image.Point{X: 200, Y: 200}); src.Size != want {
		t.Errorf("Size = %v, want %v", src.Size, want)
	}
}

func TestLoadEmptySourceIsTheBuiltin(t *testing.T) {
	src, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(src.Walk) != 3 {
		t.Errorf("got %d walk frames, want the built-in gopher's 3", len(src.Walk))
	}
}

func TestLoadSinglePNG(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cat.png")
	writePNG(t, path, 32, 48)

	src, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(src.Walk) != 1 {
		t.Errorf("got %d frames, want 1", len(src.Walk))
	}
	if want := (image.Point{X: 32, Y: 48}); src.Size != want {
		t.Errorf("Size = %v, want %v", src.Size, want)
	}
	if src.Walk[0].Duration != 0 {
		t.Errorf("Duration = %v, want 0 so pet.fps applies", src.Walk[0].Duration)
	}
}

func TestLoadAnimatedGIFUsesItsOwnTiming(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cat.gif")
	writeGIF(t, path, 3)

	src, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(src.Walk) != 3 {
		t.Fatalf("got %d frames, want 3", len(src.Walk))
	}
	if want := 50 * time.Millisecond; src.Walk[0].Duration != want {
		t.Errorf("Duration = %v, want the GIF's own %v", src.Walk[0].Duration, want)
	}
}

func TestLoadDirectorySortsFramesByName(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"03.png", "01.png", "02.png"} {
		writePNG(t, filepath.Join(dir, name), 10, 10)
	}
	writePNG(t, filepath.Join(dir, "notes.txt.png"), 10, 10)

	src, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(src.Walk) != 4 {
		t.Errorf("got %d frames, want 4", len(src.Walk))
	}
}

func TestLoadDirectoryPicksUpTalkFrames(t *testing.T) {
	dir := t.TempDir()
	writePNG(t, filepath.Join(dir, "01.png"), 10, 10)
	writePNG(t, filepath.Join(dir, "02.png"), 10, 10)
	writePNG(t, filepath.Join(dir, "talk.png"), 10, 10)

	src, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(src.Walk) != 2 {
		t.Errorf("got %d walk frames, want 2", len(src.Walk))
	}
	if len(src.Talk) != 1 {
		t.Errorf("got %d talk frames, want 1", len(src.Talk))
	}
}

func TestLoadDirectoryOfOnlyTalkFramesStillWalks(t *testing.T) {
	dir := t.TempDir()
	writePNG(t, filepath.Join(dir, "talk.png"), 10, 10)

	src, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(src.Walk) != 1 {
		t.Errorf("got %d walk frames, want the talk frame promoted", len(src.Walk))
	}
}

func TestLoadIgnoresNonImages(t *testing.T) {
	dir := t.TempDir()
	writePNG(t, filepath.Join(dir, "01.png"), 10, 10)
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	src, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(src.Walk) != 1 {
		t.Errorf("got %d frames, want just the PNG", len(src.Walk))
	}
}

func TestValidateRejectsBadSources(t *testing.T) {
	dir := t.TempDir()
	notAnImage := filepath.Join(dir, "broken.png")
	if err := os.WriteFile(notAnImage, []byte("not a png"), 0o644); err != nil {
		t.Fatal(err)
	}

	emptyDir := filepath.Join(dir, "empty")
	if err := os.Mkdir(emptyDir, 0o755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		source string
	}{
		{"missing file", filepath.Join(dir, "nope.png")},
		{"undecodable file", notAnImage},
		{"empty directory", emptyDir},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Validate(tt.source); err == nil {
				t.Error("Validate accepted it")
			}
		})
	}
}

func TestValidateAcceptsTheBuiltin(t *testing.T) {
	if err := Validate(""); err != nil {
		t.Errorf("Validate(\"\") = %v, want nil", err)
	}
}
