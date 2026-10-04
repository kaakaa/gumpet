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

// The icon is a red mailbox at someone's gate with a letter in its open door:
// gumpet is where messages arrive, not where they are sent from. It is drawn
// here, a pixel at a time, rather than taken from anywhere, so it is gumpet's
// own — no postal service's shape, colour or mark. There are two drawings
// because pixel art does not shrink: the 32 pixel one has room for the
// letter's flap and the door's seam, and the 16 pixel one keeps only what
// reads at that size.
var (
	drawing32 = []string{
		"................................",
		"................................",
		"................................",
		"................................",
		"..........KKKKKKKKKKKKKKKK......",
		"........KKBBBBBBBBBBBBBBBBKK....",
		".......KBBHKHHHHHHHHHHHHHBBBK...",
		".......KBHHKBBBBBBBBBBBBBSSSK...",
		"..KKKKKKKKKKKKKKBBBBBBBBBSSSK...",
		"..KEWWWWWWWWWWEKBBBBBBBBBSSSK...",
		"..KWEWWWWWWWWEWKBBBBBBBBBSSSK...",
		"..KWWEWWWWWWEWWKBBBBBBBBBSSSK...",
		"..KWWWEWWWWEWWWKBBBBBBBBBSSSK...",
		"..KWWWWEWWEWWWWKBBBBBBBBBSSSK...",
		"..KWWWWWEEWWWWWKBBBBBBBBBSSSK...",
		"..KWWWWWWWWWWWWKBBBBBBBBBSSSK...",
		"..KKKKKKKKKKKKKKSSSSSSSSSSSSK...",
		".......KKKKKKKKKKKKKKKKKKKKKK...",
		"................KKKKK...........",
		"................KPPpK...........",
		"................KPPpK...........",
		"................KPPpK...........",
		"................KPPpK...........",
		"................KPPpK...........",
		"................KPPpK...........",
		"................KPPpK...........",
		"................KKKKK...........",
		"................................",
		"................................",
		"................................",
		"................................",
		"................................",
	}
	drawing16 = []string{
		"................",
		"................",
		".....KKKKKKKKK..",
		"...KKHHHHHHHHBK.",
		"KKKKKKKKBBBBBSK.",
		"KEWWWWEKBBBBBSK.",
		"KWEWWEWKBBBBBSK.",
		"KWWEEWWKBBBBBSK.",
		"KWWWWWWKSSSSSSK.",
		"KKKKKKKKKKKKKKK.",
		"........KPK.....",
		"........KPK.....",
		"........KPK.....",
		"........KPK.....",
		"........KKK.....",
		"................",
	}
)

// palette gives each letter of the drawings its colour. '.' is transparent.
var palette = map[byte]color.NRGBA{
	'K': {0x3a, 0x16, 0x1a, 0xff}, // outline
	'B': {0xd6, 0x30, 0x2c, 0xff}, // the box
	'H': {0xff, 0x7a, 0x68, 0xff}, // where the light catches it
	'S': {0x96, 0x1e, 0x20, 0xff}, // in shadow
	'W': {0xfc, 0xf8, 0xec, 0xff}, // the letter
	'E': {0xc4, 0xb2, 0x88, 0xff}, // the letter's flap
	'P': {0x96, 0x64, 0x3a, 0xff}, // the post
	'p': {0x68, 0x44, 0x26, 0xff}, // the post in shade
}

// Icon draws the mailbox at size pixels square, from whichever drawing size
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
