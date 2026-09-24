// Package lang decides which language the pet speaks in, and says the pet's
// own words in it: its menu, and what it composes itself.
//
// It never touches what the pet was sent. A message, a heading or a headline
// is shown as it arrived; only the words gumpet wrote are translated.
//
// Like the settings page, the English is the key. Code says tr("Quit"), the
// English reads correctly with no dictionary at all, and a word missing from
// the dictionary shows up in English rather than as a key. A test reads the
// pet's source and fails if it says something the dictionary does not know.
package lang

import (
	"fmt"
	"strconv"
	"strings"
)

// Lang is a language the pet can speak.
type Lang string

const (
	English  Lang = "en"
	Japanese Lang = "ja"
)

// Auto is the setting that follows the system.
const Auto = "auto"

// Settings are the values the language setting accepts.
var Settings = []string{Auto, string(English), string(Japanese)}

// Valid reports whether s is one of [Settings].
func Valid(s string) bool {
	for _, v := range Settings {
		if s == v {
			return true
		}
	}
	return false
}

// Resolve turns the setting into a language. For "auto", locale is the
// system's locale — see [Detect] — and is only read then.
func Resolve(setting string, locale func() string) Lang {
	switch setting {
	case string(English):
		return English
	case string(Japanese):
		return Japanese
	}
	if l, ok := FromLocale(locale()); ok {
		return l
	}
	return English
}

// FromLocale reads a locale however the system spells it: "ja_JP.UTF-8" from
// the environment, "ja-JP" from macOS and Windows, or plain "ja". The second
// result is false when the locale says nothing — empty, or the "C" and
// "POSIX" locales, which are what a system with no preference reports — so
// that the caller can look somewhere else.
func FromLocale(tag string) (Lang, bool) {
	tag = strings.TrimSpace(tag)
	if tag == "" || tag == "C" || tag == "POSIX" || strings.HasPrefix(tag, "C.") {
		return "", false
	}
	primary := strings.ToLower(strings.FieldsFunc(tag, func(r rune) bool {
		return r == '_' || r == '-' || r == '.' || r == '@'
	})[0])
	if primary == "ja" {
		return Japanese, true
	}
	// Any other language has no dictionary yet, and English is the one every
	// word already exists in.
	return English, true
}

// T says en in l. A word with no translation comes back as it is, in English.
func (l Lang) T(en string) string {
	if l == Japanese {
		if s, ok := japanese[en]; ok {
			return s
		}
	}
	return en
}

// Format fills {0}, {1}... in a translated template. Word order differs
// between languages, so where a number goes is up to the template, not to the
// code that has the number.
func Format(tmpl string, args ...any) string {
	for i, a := range args {
		tmpl = strings.ReplaceAll(tmpl, "{"+strconv.Itoa(i)+"}", fmt.Sprint(a))
	}
	return tmpl
}
