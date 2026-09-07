// Package petsrc reads a pet's artwork into plain images.
//
// It stops short of turning them into GPU textures, which keeps loading — and
// so also validating what the user typed into the settings page — free of any
// dependency on a running graphics context.
package petsrc

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/kaakaa/gumpet/assets"
	"github.com/kaakaa/gumpet/internal/gifseq"
)

// Frame is one image plus how long it stays up. A duration of zero means the
// source carries no timing of its own and the configured frame rate applies.
type Frame struct {
	Image    image.Image
	Duration time.Duration
}

// Source is a loaded pet: its animations and how big the artwork is.
type Source struct {
	// Name identifies the source in log messages.
	Name string
	// Walk is the idle/walking animation, and always has at least one frame.
	Walk []Frame
	// Talk is played while a message is up. It may be empty, in which case
	// Walk keeps playing.
	Talk []Frame
	// Size is the size of the artwork in pixels.
	Size image.Point
}

// builtinFrames are the gopher's walk cycle, bundled with gumpet.
var builtinFrames = []string{"gopher/out01.png", "gopher/out02.png", "gopher/out03.png"}

// Load reads the artwork named by a config's pet.source. An empty source means
// the bundled gopher; otherwise it is a PNG/JPEG, an animated GIF, or a
// directory of frame images.
func Load(source string) (*Source, error) {
	if source == "" {
		return Builtin()
	}
	fi, err := os.Stat(source)
	if err != nil {
		return nil, fmt.Errorf("pet source %q: %w", source, err)
	}
	if fi.IsDir() {
		return loadDir(source)
	}
	return loadFile(source)
}

// Validate reports whether Load would succeed, so the settings page can reject
// a bad path while the user is still looking at it.
func Validate(source string) error {
	_, err := Load(source)
	return err
}

// Builtin is the gopher gumpet ships with.
func Builtin() (*Source, error) {
	walk := make([]Frame, 0, len(builtinFrames))
	for _, name := range builtinFrames {
		img, err := decodeFS(assets.Gopher, name)
		if err != nil {
			return nil, err
		}
		walk = append(walk, Frame{Image: img})
	}
	return &Source{Name: "gopher (built-in)", Walk: walk, Size: walk[0].Bounds().Size()}, nil
}

// Bounds is the frame's image bounds, spelled out so callers do not have to
// reach through to the embedded image.
func (f Frame) Bounds() image.Rectangle { return f.Image.Bounds() }

func decodeFS(fsys fs.FS, name string) (image.Image, error) {
	f, err := fsys.Open(name)
	if err != nil {
		return nil, fmt.Errorf("open bundled asset %s: %w", name, err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode bundled asset %s: %w", name, err)
	}
	return img, nil
}

func loadFile(path string) (*Source, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open pet source: %w", err)
	}
	defer f.Close()

	name := filepath.Base(path)
	if strings.EqualFold(filepath.Ext(path), ".gif") {
		decoded, err := gifseq.Decode(f)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		frames := make([]Frame, 0, len(decoded))
		for _, d := range decoded {
			frames = append(frames, Frame{Image: d.Image, Duration: d.Duration})
		}
		return &Source{Name: name, Walk: frames, Size: frames[0].Bounds().Size()}, nil
	}

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", name, err)
	}
	return &Source{Name: name, Walk: []Frame{{Image: img}}, Size: img.Bounds().Size()}, nil
}

// loadDir reads every image in dir as a frame, sorted by file name. Frames
// named talk.* become the talking animation instead.
func loadDir(dir string) (*Source, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read pet source directory: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !isImage(e.Name()) {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)

	src := &Source{Name: filepath.Base(dir)}
	for _, name := range names {
		img, err := decodeFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		frame := Frame{Image: img}
		if strings.HasPrefix(strings.ToLower(name), "talk.") {
			src.Talk = append(src.Talk, frame)
		} else {
			src.Walk = append(src.Walk, frame)
		}
	}
	if len(src.Walk) == 0 {
		if len(src.Talk) == 0 {
			return nil, fmt.Errorf("pet source directory %s has no image files", dir)
		}
		src.Walk, src.Talk = src.Talk, nil
	}
	src.Size = src.Walk[0].Bounds().Size()
	return src, nil
}

func decodeFile(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	return img, nil
}

func isImage(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png", ".jpg", ".jpeg", ".gif":
		return true
	}
	return false
}
