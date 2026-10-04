package winres

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the icon PNGs from the drawing")

func name(size int) string { return fmt.Sprintf("icon_%d.png", size) }

// The PNGs are committed rather than drawn on every build, so this is what
// notices when the drawing changes and they were not regenerated.
func TestTheIconPNGsAreTheDrawing(t *testing.T) {
	for _, size := range Sizes {
		img := Icon(size)
		if img == nil {
			t.Errorf("Icon(%d) drew nothing; every size in Sizes must divide by 16 or 32", size)
			continue
		}
		if *update {
			var buf bytes.Buffer
			if err := png.Encode(&buf, img); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(name(size), buf.Bytes(), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		f, err := os.Open(name(size))
		if err != nil {
			t.Errorf("%s is missing; run go generate ./cmd/gumpet", name(size))
			continue
		}
		got, err := png.Decode(f)
		f.Close()
		if err != nil {
			t.Errorf("%s: %v", name(size), err)
			continue
		}
		if !samePixels(got, img) {
			t.Errorf("%s no longer matches the drawing; run go generate ./cmd/gumpet", name(size))
		}
	}
}

// A drawing with a short row, or a letter missing from the palette, would
// come out with a hole in it rather than fail.
func TestTheDrawingsAreWellFormed(t *testing.T) {
	for _, d := range []struct {
		name    string
		drawing []string
		size    int
	}{{"drawing16", drawing16, 16}, {"drawing32", drawing32, 32}} {
		if len(d.drawing) != d.size {
			t.Errorf("%s has %d rows, want %d", d.name, len(d.drawing), d.size)
		}
		for y, row := range d.drawing {
			if len(row) != d.size {
				t.Errorf("%s row %d is %d wide, want %d", d.name, y, len(row), d.size)
			}
			for x := 0; x < len(row); x++ {
				if _, ok := palette[row[x]]; !ok && row[x] != '.' {
					t.Errorf("%s row %d column %d: %q has no colour", d.name, y, x, row[x])
				}
			}
		}
	}
}

// Every size is in the resource, so Windows can pick the one that suits
// Explorer's view and the screen's density.
func TestTheResourceListsEverySize(t *testing.T) {
	data, err := os.ReadFile("winres.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, size := range Sizes {
		if !bytes.Contains(data, []byte(`"`+name(size)+`"`)) {
			t.Errorf("winres.json does not list %s", name(size))
		}
	}
}

func samePixels(a, b image.Image) bool {
	if a.Bounds() != b.Bounds() {
		return false
	}
	r := a.Bounds()
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			ar, ag, ab, aa := a.At(x, y).RGBA()
			br, bg, bb, ba := b.At(x, y).RGBA()
			if ar != br || ag != bg || ab != bb || aa != ba {
				return false
			}
		}
	}
	return true
}
