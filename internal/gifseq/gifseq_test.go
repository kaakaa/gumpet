package gifseq

import (
	"bytes"
	"image"
	"image/color"
	"image/color/palette"
	"image/gif"
	"testing"
	"time"
)

// buildGIF encodes a 2x1 animation: a red left pixel, then a green right pixel
// drawn as a partial frame covering only the right half.
func buildGIF(t *testing.T, disposal []byte, delays []int) []byte {
	t.Helper()

	first := image.NewPaletted(image.Rect(0, 0, 2, 1), palette.Plan9)
	first.Set(0, 0, color.RGBA{R: 0xff, A: 0xff})

	second := image.NewPaletted(image.Rect(1, 0, 2, 1), palette.Plan9)
	second.Set(1, 0, color.RGBA{G: 0xff, A: 0xff})

	var buf bytes.Buffer
	err := gif.EncodeAll(&buf, &gif.GIF{
		Image:    []*image.Paletted{first, second},
		Delay:    delays,
		Disposal: disposal,
		Config:   image.Config{Width: 2, Height: 1},
	})
	if err != nil {
		t.Fatalf("encode gif: %v", err)
	}
	return buf.Bytes()
}

func TestDecodeCompositesPartialFrames(t *testing.T) {
	data := buildGIF(t, []byte{gif.DisposalNone, gif.DisposalNone}, []int{10, 10})

	frames, err := Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(frames) != 2 {
		t.Fatalf("got %d frames, want 2", len(frames))
	}

	// The second frame only paints the right pixel, so the left one has to come
	// from the frame before it.
	if r, _, _, _ := frames[1].Image.At(0, 0).RGBA(); r < 0x8000 {
		t.Errorf("second frame lost the first frame's red pixel: %v", frames[1].Image.At(0, 0))
	}
	if _, g, _, _ := frames[1].Image.At(1, 0).RGBA(); g < 0x8000 {
		t.Errorf("second frame is missing its own green pixel: %v", frames[1].Image.At(1, 0))
	}
}

func TestDecodeHonoursDisposalBackground(t *testing.T) {
	// Disposing the first frame to the background clears the whole canvas
	// before the second is drawn.
	data := buildGIF(t, []byte{gif.DisposalBackground, gif.DisposalNone}, []int{10, 10})

	frames, err := Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if _, _, _, a := frames[1].Image.At(0, 0).RGBA(); a != 0 {
		t.Errorf("left pixel should have been disposed to transparent, got %v", frames[1].Image.At(0, 0))
	}
}

func TestDecodeUsesFrameDelays(t *testing.T) {
	data := buildGIF(t, []byte{gif.DisposalNone, gif.DisposalNone}, []int{7, 25})

	frames, err := Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if want := 70 * time.Millisecond; frames[0].Duration != want {
		t.Errorf("frame 0 duration = %v, want %v", frames[0].Duration, want)
	}
	if want := 250 * time.Millisecond; frames[1].Duration != want {
		t.Errorf("frame 1 duration = %v, want %v", frames[1].Duration, want)
	}
}

func TestDecodeTreatsZeroDelayAsATenthOfASecond(t *testing.T) {
	data := buildGIF(t, []byte{gif.DisposalNone, gif.DisposalNone}, []int{0, 1})

	frames, err := Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	for i, f := range frames {
		if f.Duration != defaultDelay {
			t.Errorf("frame %d duration = %v, want %v", i, f.Duration, defaultDelay)
		}
	}
}

func TestDecodeRejectsGarbage(t *testing.T) {
	if _, err := Decode(bytes.NewReader([]byte("not a gif"))); err == nil {
		t.Error("Decode accepted a non-GIF")
	}
}
