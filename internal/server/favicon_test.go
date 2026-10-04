package server

import (
	"bytes"
	"image"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaakaa/gumpet/internal/config"
	"github.com/kaakaa/gumpet/internal/icon"
	"github.com/kaakaa/gumpet/internal/petsrc"
)

// iconOf is the window icon gumpet would make for source, at the favicon's
// size — what the tab is meant to show.
func iconOf(t *testing.T, source string) image.Image {
	t.Helper()
	src, err := petsrc.Load(source)
	if err != nil {
		t.Fatal(err)
	}
	for _, img := range icon.From(src.Walk[0].Image, src.Smooth) {
		if img.Bounds().Dx() == faviconSize {
			return img
		}
	}
	t.Fatalf("icon.From made no %dpx icon", faviconSize)
	return nil
}

func getFavicon(t *testing.T, s *Server, path string) image.Image {
	t.Helper()
	rec := do(t, s, http.MethodGet, path, "", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s: status %d", path, rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "image/png" {
		t.Errorf("GET %s: Content-Type %q, want image/png", path, ct)
	}
	img, err := png.Decode(bytes.NewReader(rec.Body.Bytes()))
	if err != nil {
		t.Fatalf("GET %s: not a PNG: %v", path, err)
	}
	return img
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

// The tab shows the pet in use, the same picture as the window's icon, and
// follows a change of pet without a restart.
func TestTheFaviconIsThePetInUse(t *testing.T) {
	s, _, _ := newTestServerWithConfig(t, config.Default(), 1)
	gopher := iconOf(t, "")
	for _, path := range []string{"/favicon.png", "/favicon.ico"} {
		if got := getFavicon(t, s, path); !samePixels(got, gopher) {
			t.Errorf("GET %s is not the gopher's icon", path)
		}
	}

	if err := s.store.Update(func(c *config.Config) { c.Pet.Source = "pixel" }); err != nil {
		t.Fatal(err)
	}
	got := getFavicon(t, s, "/favicon.png")
	if !samePixels(got, iconOf(t, "pixel")) {
		t.Error("after switching to the pixel pet, the favicon is not its icon")
	}
	if samePixels(got, gopher) {
		t.Error("after switching pets, the favicon is still the gopher")
	}
}

// Artwork deleted after it was chosen must not leave the tab without an icon.
func TestTheFaviconFallsBackToTheGopher(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mine.png")
	img := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	img.Pix[3] = 0xff
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Pet.Source = path
	s, _, _ := newTestServerWithConfig(t, cfg, 1)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	if got := getFavicon(t, s, "/favicon.png"); !samePixels(got, iconOf(t, "")) {
		t.Error("with the pet's artwork gone, the favicon is not the gopher")
	}
}

// The pages ask for the icon, and their own policy lets them load it: with
// default-src 'none' and no img-src, the browser would refuse it.
func TestThePagesLinkTheFavicon(t *testing.T) {
	s, _, _ := newTestServerWithConfig(t, config.Default(), 1)
	for _, path := range []string{"/", "/messages"} {
		rec := do(t, s, http.MethodGet, path, "", "", nil)
		if !strings.Contains(rec.Body.String(), `<link rel="icon" type="image/png" href="/favicon.png">`) {
			t.Errorf("%s does not link the favicon", path)
		}
		if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "img-src 'self'") {
			t.Errorf("%s: Content-Security-Policy %q would block the favicon", path, csp)
		}
	}
}
