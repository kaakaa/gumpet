package winres

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"testing"

	"github.com/kaakaa/gumpet/internal/icon"
	"github.com/kaakaa/gumpet/internal/petsrc"
)

var update = flag.Bool("update", false, "rewrite the icon PNGs from the default gopher")

// gopherIcons are the icons the window would get for the default pet, one per
// size in icon.Sizes.
func gopherIcons(t *testing.T) []image.Image {
	t.Helper()
	src, err := petsrc.Builtin()
	if err != nil {
		t.Fatal(err)
	}
	return icon.From(src.Walk[0].Image, src.Smooth)
}

func name(size int) string { return fmt.Sprintf("icon_%d.png", size) }

// The executable's icon is the gopher's window icon, size for size. It is
// committed rather than built on every compile, so this is what notices when
// the gopher or the icon code changes and the PNGs were not regenerated.
func TestTheExecutableIconIsTheGophersWindowIcon(t *testing.T) {
	icons := gopherIcons(t)
	for i, size := range icon.Sizes {
		var buf bytes.Buffer
		if err := png.Encode(&buf, icons[i]); err != nil {
			t.Fatal(err)
		}
		if *update {
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
		if !samePixels(got, icons[i]) {
			t.Errorf("%s no longer matches the gopher's icon; run go generate ./cmd/gumpet", name(size))
		}
	}
}

// Every size the window offers is in the resource, so Windows can pick the
// one that suits Explorer's view and the screen's density.
func TestTheResourceListsEverySize(t *testing.T) {
	data, err := os.ReadFile("winres.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, size := range icon.Sizes {
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
