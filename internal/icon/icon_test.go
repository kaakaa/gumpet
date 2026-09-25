package icon

import (
	"image"
	"image/color"
	"testing"
)

// pet draws a solid w×h block at (x, y) on a transparent canvas, the way a
// pet sits inside a mostly empty frame.
func pet(canvasW, canvasH, x, y, w, h int, c color.NRGBA) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, canvasW, canvasH))
	for py := y; py < y+h; py++ {
		for px := x; px < x+w; px++ {
			img.SetNRGBA(px, py, c)
		}
	}
	return img
}

var (
	red         = color.NRGBA{R: 0xe0, G: 0x20, B: 0x20, A: 0xff}
	transparent = color.NRGBA{}
)

func TestEverySizeIsMadeSquare(t *testing.T) {
	icons := From(pet(200, 120, 10, 10, 50, 100, red), true)
	if len(icons) != len(Sizes) {
		t.Fatalf("made %d icons, want %d", len(icons), len(Sizes))
	}
	for i, ic := range icons {
		b := ic.Bounds()
		if b.Dx() != Sizes[i] || b.Dy() != Sizes[i] {
			t.Errorf("icon %d is %dx%d, want %dx%d", i, b.Dx(), b.Dy(), Sizes[i], Sizes[i])
		}
	}
}

// A pet is mostly empty frame. The icon must be the pet, filling the square,
// not a speck in the middle of the frame's margin.
func TestTheMarginIsCroppedAway(t *testing.T) {
	// A 20×20 pet in the corner of a 200×200 frame.
	icons := From(pet(200, 200, 150, 150, 20, 20, red), false)
	big := icons[len(icons)-1]
	for _, p := range []image.Point{{0, 0}, {255, 255}, {128, 128}, {0, 255}} {
		if _, _, _, a := big.At(p.X, p.Y).RGBA(); a == 0 {
			t.Errorf("(%d,%d) is empty; the pet should fill the icon once its margin is gone", p.X, p.Y)
		}
	}
}

// A tall pet keeps its shape: centred, with empty space either side, never
// stretched to fill the square.
func TestATallPetIsCentredNotStretched(t *testing.T) {
	// 50 wide, 100 tall.
	big := From(pet(200, 200, 0, 0, 50, 100, red), false)[len(Sizes)-1]
	// The 256 square holds it 128 wide, from 64 to 192.
	cases := []struct {
		x      int
		filled bool
	}{{10, false}, {63, false}, {64, true}, {128, true}, {191, true}, {192, false}, {250, false}}
	for _, c := range cases {
		_, _, _, a := big.At(c.x, 128).RGBA()
		if (a != 0) != c.filled {
			t.Errorf("x=%d filled=%v, want %v", c.x, a != 0, c.filled)
		}
	}
}

// Pixel art is scaled by nearest neighbour, the way the pet itself is drawn,
// so the icon has no colours the artwork did not: no blurred in-betweens.
func TestPixelArtStaysHard(t *testing.T) {
	blue := color.NRGBA{B: 0xff, A: 0xff}
	img := pet(4, 4, 0, 0, 2, 4, red)
	for y := range 4 {
		for x := 2; x < 4; x++ {
			img.SetNRGBA(x, y, blue)
		}
	}
	for _, ic := range From(img, false) {
		b := ic.Bounds()
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				c := color.NRGBAModel.Convert(ic.At(x, y)).(color.NRGBA)
				if c != red && c != blue && c != transparent {
					t.Fatalf("%dpx icon has %v at (%d,%d): a colour the artwork does not have", b.Dx(), c, x, y)
				}
			}
		}
	}
}

// A frame with nothing drawn in it — a broken file, a blank first frame — must
// not bring gumpet down; it gets a blank icon rather than none at all.
func TestAnEmptyPictureStillMakesIcons(t *testing.T) {
	icons := From(image.NewNRGBA(image.Rect(0, 0, 10, 10)), true)
	if len(icons) != len(Sizes) {
		t.Errorf("made %d icons from an empty picture, want %d", len(icons), len(Sizes))
	}
}
