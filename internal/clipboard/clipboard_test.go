package clipboard

import (
	"bytes"
	"errors"
	"reflect"
	"testing"
)

func env(vars map[string]string) func(string) string {
	return func(k string) string { return vars[k] }
}

func installed(names ...string) func(string) (string, error) {
	return func(name string) (string, error) {
		for _, n := range names {
			if n == name {
				return "/usr/bin/" + name, nil
			}
		}
		return "", errors.New("not found")
	}
}

func TestPickUsesWhatEachDesktopShipsWith(t *testing.T) {
	cases := []struct {
		name      string
		goos      string
		vars      map[string]string
		installed []string
		want      string
	}{
		{"macOS", "darwin", nil, []string{"pbcopy"}, "pbcopy"},
		{"Windows", "windows", nil, []string{"clip"}, "clip"},
		{"X with xclip", "linux", nil, []string{"xclip", "wl-copy"}, "xclip"},
		{"X with only xsel", "linux", nil, []string{"xsel"}, "xsel"},
		// Under Wayland, xclip only reaches XWayland windows, so wl-copy is
		// preferred when it is there...
		{"Wayland", "linux", map[string]string{"WAYLAND_DISPLAY": "wayland-0"}, []string{"xclip", "wl-copy"}, "wl-copy"},
		// ...and xclip is still better than nothing when it is not.
		{"Wayland without wl-copy", "linux", map[string]string{"WAYLAND_DISPLAY": "wayland-0"}, []string{"xclip"}, "xclip"},
		{"X with only wl-copy", "linux", nil, []string{"wl-copy"}, "wl-copy"},
		{"BSD falls in with Linux", "freebsd", nil, []string{"xsel"}, "xsel"},
	}
	for _, c := range cases {
		got, err := pick(c.goos, env(c.vars), installed(c.installed...))
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if got.name != c.want {
			t.Errorf("%s: picked %s, want %s", c.name, got.name, c.want)
		}
	}
}

// A Linux desktop with no clipboard tool is common, and must come back as an
// error the pet can log rather than anything that stops it.
func TestPickSaysSoWhenNothingIsInstalled(t *testing.T) {
	_, err := pick("linux", env(nil), installed())
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("err = %v, want ErrUnavailable", err)
	}
	for _, name := range []string{"xclip", "xsel", "wl-copy"} {
		if !bytes.Contains([]byte(err.Error()), []byte(name)) {
			t.Errorf("error %q does not say it looked for %s", err, name)
		}
	}
}

func TestEncodePassesUTF8ThroughEverywhereButWindows(t *testing.T) {
	text := "ビルド失敗\n/tmp/x.log"
	if got := encode(command{name: "pbcopy"}, text); string(got) != text {
		t.Errorf("pbcopy gets %q, want the text unchanged", got)
	}
}

// clip.exe reads UTF-8 in the console's code page, so Japanese would arrive as
// mojibake. UTF-16 with a byte-order mark is what it reads as Unicode.
func TestEncodeGivesClipUTF16WithCRLF(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []byte
	}{
		{"ASCII", "a\nb", []byte{0xff, 0xfe, 'a', 0, '\r', 0, '\n', 0, 'b', 0}},
		{"already CRLF", "a\r\nb", []byte{0xff, 0xfe, 'a', 0, '\r', 0, '\n', 0, 'b', 0}},
		// あ is U+3042.
		{"Japanese", "あ", []byte{0xff, 0xfe, 0x42, 0x30}},
		// 😀 is outside the BMP and needs a surrogate pair: D83D DE00.
		{"surrogate pair", "😀", []byte{0xff, 0xfe, 0x3d, 0xd8, 0x00, 0xde}},
	}
	for _, c := range cases {
		got := encode(command{name: "clip", windows: true}, c.in)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: encode = % x, want % x", c.name, got, c.want)
		}
	}
}

// pbcopy reads its input in the locale's encoding, and a Mac app started from
// Finder has no locale at all: LANG is set by terminals, not by launchd. In
// that state Japanese is silently dropped and the clipboard comes out empty.
// Found by running it from a shell with no LANG, which is the same situation.
func TestPbcopyIsToldTheTextIsUTF8(t *testing.T) {
	c, err := pick("darwin", env(nil), installed("pbcopy"))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range c.env {
		if e == "LC_ALL=en_US.UTF-8" {
			found = true
		}
	}
	if !found {
		t.Errorf("pbcopy runs with env %q; without a UTF-8 locale it drops non-ASCII text", c.env)
	}
}
