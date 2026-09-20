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
	// Scale is how much this artwork wants to be enlarged before pet.scale is
	// applied, so that "1" means the same thing to someone choosing a pet
	// whatever its artwork happens to measure. A sprite drawn at twelve pixels
	// and an illustration drawn at five thousand should not need the setting
	// changed between them.
	Scale float64
	// Smooth is whether enlarging this artwork should be smoothed. Pixel art
	// says no: for it, sharp edges are the drawing.
	Smooth bool
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

// Builtins are the pets gumpet ships with, in the order they are offered.
//
// A name here beats a path of the same spelling. The names have no separator
// and no extension, so a file that collides with one has to have been asked
// for by a path that says so — "./pink" rather than "pink".
var Builtins = []Pet{
	{Name: "gopher", Label: "Gopher", Scale: 1, Smooth: true},
	{Name: "pixel", Label: "Pixel gopher", file: "pixel/gopher.gif", Scale: 1, Smooth: false},
	{Name: "astro", Label: "Astronaut", file: "astro/pet.gif", Scale: 1, Smooth: false},
	{Name: "rose", Label: "Rose", file: "rose/pet.gif", Scale: 1, Smooth: false},
	{Name: "flier", Label: "Flier", file: "flier/pet.gif", Scale: 1, Smooth: false},
}

// Pet is one of the bundled pets.
type Pet struct {
	// Name is what goes in pet.source.
	Name string
	// Label is what the menu and the settings page show.
	Label string
	// Scale and Smooth are this artwork's own idea of how it should be drawn.
	Scale  float64
	Smooth bool
	// file is the path inside the embedded assets. Empty means the original
	// gopher, which is several files rather than one.
	file string
}

// BuiltinNamed finds a bundled pet by the name written in pet.source.
func BuiltinNamed(name string) (Pet, bool) {
	for _, p := range Builtins {
		if p.Name == name {
			return p, true
		}
	}
	return Pet{}, false
}

// Load reads the artwork named by a config's pet.source. An empty source means
// the bundled gopher; otherwise it is a PNG/JPEG, an animated GIF, or a
// directory of frame images.
func Load(source string) (*Source, error) {
	if source == "" {
		return Builtin()
	}
	if pet, ok := BuiltinNamed(source); ok {
		return loadBuiltin(pet)
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

// Builtin is the gopher gumpet ships with, and the pet used when nothing is
// configured.
func Builtin() (*Source, error) {
	walk := make([]Frame, 0, len(builtinFrames))
	for _, name := range builtinFrames {
		img, err := decodeFS(assets.Gopher, name)
		if err != nil {
			return nil, err
		}
		walk = append(walk, Frame{Image: img})
	}
	return &Source{
		Name:   "gopher (built-in)",
		Walk:   walk,
		Size:   walk[0].Bounds().Size(),
		Scale:  1,
		Smooth: true,
	}, nil
}

// loadBuiltin reads one of the bundled pets out of the embedded assets.
func loadBuiltin(pet Pet) (*Source, error) {
	if pet.file == "" {
		return Builtin()
	}

	f, err := assets.Pets.Open(pet.file)
	if err != nil {
		return nil, fmt.Errorf("open bundled pet %s: %w", pet.Name, err)
	}
	defer f.Close()

	var frames []Frame
	if strings.EqualFold(filepath.Ext(pet.file), ".gif") {
		decoded, err := gifseq.Decode(f)
		if err != nil {
			return nil, fmt.Errorf("bundled pet %s: %w", pet.Name, err)
		}
		for _, d := range decoded {
			frames = append(frames, Frame{Image: d.Image, Duration: d.Duration})
		}
	} else {
		img, _, err := image.Decode(f)
		if err != nil {
			return nil, fmt.Errorf("decode bundled pet %s: %w", pet.Name, err)
		}
		frames = []Frame{{Image: img}}
	}

	return &Source{
		Name:   pet.Name + " (built-in)",
		Walk:   frames,
		Size:   frames[0].Bounds().Size(),
		Scale:  pet.Scale,
		Smooth: pet.Smooth,
	}, nil
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
		return &Source{Name: name, Walk: frames, Size: frames[0].Bounds().Size(), Scale: 1, Smooth: true}, nil
	}

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", name, err)
	}
	return &Source{Name: name, Walk: []Frame{{Image: img}}, Size: img.Bounds().Size(), Scale: 1, Smooth: true}, nil
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

	src := &Source{Name: filepath.Base(dir), Scale: 1, Smooth: true}
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
