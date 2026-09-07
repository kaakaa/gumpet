// Package fontfile finds a font on this machine for gumpet to draw with.
//
// Bundling a Japanese font would add several megabytes to a binary that is
// otherwise mostly artwork, and every desktop gumpet runs on already has one.
// Looking for it is only a matter of knowing where each system keeps it.
package fontfile

import (
	"fmt"
	"os"
	"runtime"
)

// SystemCandidates lists the fonts to try on this machine, best first. They are
// all faces that cover Japanese as well as Latin, because a pet that renders
// half its messages as empty boxes is worse than a blocky one.
func SystemCandidates() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"/System/Library/Fonts/ヒラギノ角ゴシック W3.ttc",
			"/System/Library/Fonts/ヒラギノ角ゴシック W4.ttc",
			"/System/Library/Fonts/Hiragino Sans GB.ttc",
			"/Library/Fonts/Arial Unicode.ttf",
			"/System/Library/Fonts/Supplemental/Arial Unicode.ttf",
		}
	case "windows":
		return []string{
			`C:\Windows\Fonts\YuGothM.ttc`,
			`C:\Windows\Fonts\YuGothR.ttc`,
			`C:\Windows\Fonts\meiryo.ttc`,
			`C:\Windows\Fonts\msgothic.ttc`,
			`C:\Windows\Fonts\segoeui.ttf`,
		}
	default:
		return []string{
			"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
			"/usr/share/fonts/truetype/noto/NotoSansCJK-Regular.ttc",
			"/usr/share/fonts/opentype/noto/NotoSansCJKjp-Regular.otf",
			"/usr/share/fonts/truetype/fonts-japanese-gothic.ttf",
			"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		}
	}
}

// Resolve picks the font file to draw with.
//
// A configured path is taken as given, and not finding it is an error worth
// reporting rather than papering over. Otherwise, when system is true, the
// first of [SystemCandidates] that exists wins. An empty return with no error
// means nothing was found and the caller should fall back to the bundled
// bitmap font.
func Resolve(configured string, system bool) (string, error) {
	if configured != "" {
		if _, err := os.Stat(configured); err != nil {
			return "", fmt.Errorf("font %q: %w", configured, err)
		}
		return configured, nil
	}
	if !system {
		return "", nil
	}
	return FirstExisting(SystemCandidates()), nil
}

// FirstExisting returns the first path that is a readable file, or "".
func FirstExisting(paths []string) string {
	for _, path := range paths {
		if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
			return path
		}
	}
	return ""
}
