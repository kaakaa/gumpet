package winres

import (
	"image"
	"image/color"
)

// Sizes are the icons in the resource, largest first. Each is a whole
// multiple of one of the drawings below, so every pixel of the drawing stays
// a square block; 24, which neither divides into, is left to Windows to scale
// down from 32.
var Sizes = []int{256, 128, 64, 48, 32, 16}

// The icon is a red post box with a letter half way into its slot: gumpet is
// where messages arrive. It is drawn here, a pixel at a time, rather than
// taken from anywhere, so it is gumpet's own. There are two drawings because
// pixel art does not shrink: the 32 pixel one has room for the letter's flap
// and a plate on the box, and the 16 pixel one keeps only what reads at that
// size.
var (
	drawing32 = []string{
		"................................",
		"................................",
		"................................",
		".............KKKKKK.............",
		"...........KKHHRRRRKK...........",
		"..........KHHRRRRRRRRK..........",
		".........KHHRRRRRRRRRRK.........",
		"........KHHHHHHHRRRRRRRK........",
		"........KRRKKKKKKKKKKRRK........",
		"........KSSKEWWWWWWEKSSK........",
		".........KHKWEWWWWEWKSK.........",
		".........KHKWWEWWEWWKSK.........",
		".........KHKWWWEEWWWKSK.........",
		".........KDDDDDDDDDDDDK.........",
		".........KDDDDDDDDDDDDK.........",
		".........KHHRRRRRRRRSSK.........",
		".........KHHRRRRRRRRSSK.........",
		".........KHHRKKKKKKRSSK.........",
		".........KHHRKPPPPKRSSK.........",
		".........KHHRKPLLPKRSSK.........",
		".........KHHRKPPPPKRSSK.........",
		".........KHHRKKKKKKRSSK.........",
		".........KHHRRRRRRRRSSK.........",
		".........KHHRRRRRRRRSSK.........",
		".........KHHRRRRRRRRSSK.........",
		".........KRRRRRRRRRRRRK.........",
		".......KggggggggggggggggK.......",
		".......KGGGGGGGGGGGGGGGGK.......",
		".......KKKKKKKKKKKKKKKKKK.......",
		"................................",
		"................................",
		"................................",
	}
	drawing16 = []string{
		"................",
		"................",
		"......KKKK......",
		".....KHRRRK.....",
		"...KHHKKKKRKK...",
		"...KSSKWWKSSK...",
		"....KHKWWKSK....",
		"....KDDDDDDK....",
		"....KHRRRRSK....",
		"....KHKKKKSK....",
		"....KHKPPKSK....",
		"....KHKKKKSK....",
		"....KHRRRRSK....",
		"...KggggggggK...",
		"...KKKKKKKKKK...",
		"................",
	}
)

// palette gives each letter of the drawings its colour. '.' is transparent.
var palette = map[byte]color.NRGBA{
	'K': {0x3a, 0x16, 0x1a, 0xff}, // outline
	'R': {0xd6, 0x30, 0x2c, 0xff}, // the box
	'H': {0xff, 0x7a, 0x68, 0xff}, // where the light catches it
	'S': {0x96, 0x1e, 0x20, 0xff}, // in shadow
	'D': {0x28, 0x0e, 0x10, 0xff}, // the slot
	'W': {0xfc, 0xf8, 0xec, 0xff}, // the letter
	'E': {0xc4, 0xb2, 0x88, 0xff}, // the letter's flap
	'P': {0xf6, 0xec, 0xd6, 0xff}, // the plate
	'L': {0x96, 0x8c, 0x78, 0xff}, // writing on the plate, too small to read
	'G': {0x70, 0x70, 0x7a, 0xff}, // the base
	'g': {0x4e, 0x4e, 0x58, 0xff}, // the base's top edge
}

// Icon draws the post box at size pixels square, from whichever drawing size
// is a whole multiple of. It returns nil for a size neither divides.
func Icon(size int) image.Image {
	var drawing []string
	switch {
	case size%32 == 0:
		drawing = drawing32
	case size%16 == 0:
		drawing = drawing16
	default:
		return nil
	}
	scale := size / len(drawing)
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y, row := range drawing {
		for x := 0; x < len(row); x++ {
			c, ok := palette[row[x]]
			if !ok {
				continue
			}
			for dy := 0; dy < scale; dy++ {
				for dx := 0; dx < scale; dx++ {
					img.SetNRGBA(x*scale+dx, y*scale+dy, c)
				}
			}
		}
	}
	return img
}
