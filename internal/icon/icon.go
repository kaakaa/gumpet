// Package icon makes the window's icon out of the pet.
//
// Without one, Windows puts a blank placeholder on gumpet's taskbar button,
// and Linux desktops do much the same. The pet is the obvious picture, and
// taking it from the pet in use means the icon changes when the pet does.
//
// It is plain image arithmetic, kept apart from the drawing so that it can be
// tested without a screen. macOS ignores window icons entirely — an app's icon
// there comes from its bundle — so on a Mac none of this is ever seen.
package icon

import (
	"image"

	"golang.org/x/image/draw"
)

// Sizes are the icons made. The window system picks whichever suits it: a
// small one for the title bar and the taskbar at normal density, larger ones
// for high-density screens and the task switcher.
var Sizes = []int{16, 24, 32, 48, 64, 128, 256}

// From makes one icon per size from img. The picture is cropped to what is
// actually drawn — pets are mostly transparent margin, and an icon of margin
// is an icon of nothing — then centred on a square, so a tall or wide pet
// keeps its shape rather than being stretched.
//
// smooth says how to scale, the same way it does for the pet itself: pixel art
// is scaled by nearest neighbour so its edges stay hard, anything else is
// smoothed.
func From(img image.Image, smooth bool) []image.Image {
	src := crop(img)
	b := src.Bounds()
	side := max(b.Dx(), b.Dy())
	if side == 0 {
		return nil
	}

	scaler := draw.Interpolator(draw.CatmullRom)
	if !smooth {
		scaler = draw.NearestNeighbor
	}

	out := make([]image.Image, 0, len(Sizes))
	for _, size := range Sizes {
		dst := image.NewNRGBA(image.Rect(0, 0, size, size))
		// Fit the longer side to the icon, and centre the shorter one.
		w := b.Dx() * size / side
		h := b.Dy() * size / side
		w, h = max(w, 1), max(h, 1)
		x := (size - w) / 2
		y := (size - h) / 2
		scaler.Scale(dst, image.Rect(x, y, x+w, y+h), src, b, draw.Over, nil)
		out = append(out, dst)
	}
	return out
}

// crop returns the smallest part of img that has anything drawn in it. A
// picture with nothing drawn at all is returned as it is.
func crop(img image.Image) image.Image {
	b := img.Bounds()
	minX, minY, maxX, maxY := b.Max.X, b.Max.Y, b.Min.X-1, b.Min.Y-1
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a == 0 {
				continue
			}
			minX, minY = min(minX, x), min(minY, y)
			maxX, maxY = max(maxX, x), max(maxY, y)
		}
	}
	if maxX < minX {
		return img
	}
	r := image.Rect(minX, minY, maxX+1, maxY+1)
	if sub, ok := img.(interface {
		SubImage(image.Rectangle) image.Image
	}); ok {
		return sub.SubImage(r)
	}
	// Every image type in the standard library has SubImage; this is for one
	// that does not.
	dst := image.NewNRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	draw.Draw(dst, dst.Bounds(), img, r.Min, draw.Src)
	return dst
}
