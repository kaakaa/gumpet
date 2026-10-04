package server

import (
	"bytes"
	"errors"
	"image/png"
	"net/http"
	"sync"

	"github.com/kaakaa/gumpet/internal/icon"
	"github.com/kaakaa/gumpet/internal/petsrc"
)

// faviconSize is the icon served to the pages. A tab draws it at 16 or 32
// pixels; 64 leaves room for a high-density screen without sending the 256.
const faviconSize = 64

// favicon is the pages' icon: the pet in use, made by the same code as the
// window's icon, so the tab and the taskbar show the same picture. It is kept
// until the pet changes, since loading artwork means decoding every frame.
type favicon struct {
	mu     sync.Mutex
	source string
	png    []byte
}

func (s *Server) handleFavicon(w http.ResponseWriter, r *http.Request) {
	body, err := s.favicon.get(s.config().Pet.Source)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	// Revalidated on every load, so a new pet shows up in the tab without
	// waiting out a browser cache.
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(body)
}

func (f *favicon) get(source string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.png != nil && f.source == source {
		return f.png, nil
	}
	body, err := faviconFor(source)
	if err != nil {
		// Artwork that has gone missing since it was chosen still leaves the
		// tab an icon. Not kept, so the pet's own comes back if the file does.
		return faviconFor("")
	}
	f.source, f.png = source, body
	return body, nil
}

func faviconFor(source string) ([]byte, error) {
	src, err := petsrc.Load(source)
	if err != nil {
		return nil, err
	}
	icons := icon.From(src.Walk[0].Image, src.Smooth)
	if len(icons) == 0 {
		return nil, errNoIcon
	}
	pick := icons[len(icons)-1]
	for _, img := range icons {
		if img.Bounds().Dx() == faviconSize {
			pick = img
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, pick); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// errNoIcon is artwork with nothing drawn in it to make an icon of.
var errNoIcon = errors.New("the pet has nothing to make an icon from")
