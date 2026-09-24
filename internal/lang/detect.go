package lang

import (
	"os"
	"strings"
)

// Detect is the system's locale, found the way [detect] describes.
func Detect() string { return detect(os.Getenv, systemLocale) }

// detect looks for a locale in the environment first, the way every Unix
// program does, and then asks the system.
//
// The environment alone is not enough. A terminal sets LANG, but a Mac app
// started from Finder or at login has none, so on a Japanese Mac a gumpet
// started from the Dock would otherwise speak English. The same absence broke
// pbcopy's handling of Japanese in #36.
func detect(getenv func(string) string, system func() string) string {
	for _, name := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := getenv(name); v != "" {
			if _, ok := FromLocale(v); ok {
				return v
			}
		}
	}
	return system()
}

// parseAppleLanguages takes the first of the languages macOS lists in order
// of preference. `defaults read -g AppleLanguages` prints a property list:
//
//	(
//	    "ja-JP",
//	    "en-JP"
//	)
func parseAppleLanguages(out string) string {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimSuffix(line, ",")
		line = strings.Trim(line, `"`)
		if line == "" || line == "(" || line == ")" {
			continue
		}
		return line
	}
	return ""
}
