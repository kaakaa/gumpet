package server

import (
	"regexp"
	"strings"
	"testing"
)

// The settings page carries its English inside the nodes that show it, and a
// dictionary of Japanese beside them. Nothing but a person reading the whole
// page would notice a sentence that was reworded in one place and not the
// other, and nobody reads 900 lines of HTML looking for that. These tests do.
//
// They are the same bargain as TestRenderRoundTrips: the page and the
// dictionary are two halves of one thing, so a test holds them together.

// i18nOpenTag finds a tag marked for translation. data-i18n-placeholder and
// data-i18n-dyn are deliberately not matched: what follows data-i18n here must
// be either the end of the tag or another attribute.
var (
	i18nOpenTag     = regexp.MustCompile(`<([a-zA-Z0-9]+)\b[^>]*\bdata-i18n(?:\s[^>]*)?>`)
	i18nPlaceholder = regexp.MustCompile(`placeholder="([^"]*)"\s+data-i18n-placeholder`)
	jsString        = regexp.MustCompile(`"((?:[^"\\]|\\.)*)"`)
	// The script's own ways of naming a string to translate.
	jsKeyed = []*regexp.Regexp{
		regexp.MustCompile(`\bt\("((?:[^"\\]|\\.)*)"\)`),
		regexp.MustCompile(`\bstatus\("((?:[^"\\]|\\.)*)"\)`),
		regexp.MustCompile(`\bnote\("[a-z]+", "((?:[^"\\]|\\.)*)"`),
		regexp.MustCompile(`\bdyn\([^,]+, "((?:[^"\\]|\\.)*)"`),
	}
	dictEntry = regexp.MustCompile(`(?m)^\s*"((?:[^"\\]|\\.)*)":\s*"((?:[^"\\]|\\.)*)",?\s*$`)
	spaces    = regexp.MustCompile(`\s+`)
)

// settingsPage returns the settings page's markup and its script, which live
// in two files.
func settingsPage(t *testing.T) (markup, script string) {
	t.Helper()
	html, err := ui.ReadFile("ui/settings.html")
	if err != nil {
		t.Fatalf("the settings page is missing from this build: %v", err)
	}
	js, err := ui.ReadFile("ui/settings.js")
	if err != nil {
		t.Fatalf("the settings page's script is missing from this build: %v", err)
	}
	return string(html), string(js)
}

// split returns the page's markup, its script with the dictionary cut out, and
// the dictionary on its own. The dictionary is separated because its own keys
// are string literals too, and a stale entry would otherwise be found by the
// search meant to prove nothing still says it.
func split(t *testing.T, markup, script string) (string, string, string) {
	t.Helper()
	from := strings.Index(script, "var STRINGS = {")
	if from < 0 {
		t.Fatal("the settings page has no STRINGS dictionary")
	}
	rest := script[from:]
	to := strings.Index(rest, "\n  };")
	if to < 0 {
		t.Fatal("the STRINGS dictionary does not end where this test expects")
	}
	return markup, script[:from] + rest[to:], rest[:to]
}

// pageEnglish is every string the page shows that is marked for translation.
func pageEnglish(t *testing.T, markup, script string) []string {
	t.Helper()
	var out []string

	for _, m := range i18nOpenTag.FindAllStringSubmatchIndex(markup, -1) {
		name := markup[m[2]:m[3]]
		rest := markup[m[1]:]
		end := strings.Index(rest, "</"+name+">")
		if end < 0 {
			t.Fatalf("a <%s> marked data-i18n is never closed", name)
		}
		inner := rest[:end]
		// The scanner stops at the first closing tag, so a nested one of the
		// same name would silently cut the string in half.
		if strings.Contains(inner, "<"+name) {
			t.Fatalf("a <%s> marked data-i18n contains another <%s>; "+
				"mark the inner one instead", name, name)
		}
		out = append(out, collapse(inner))
	}

	for _, m := range i18nPlaceholder.FindAllStringSubmatch(markup, -1) {
		out = append(out, m[1])
	}
	for _, re := range jsKeyed {
		for _, m := range re.FindAllStringSubmatch(script, -1) {
			out = append(out, unescape(m[1]))
		}
	}
	return out
}

func collapse(s string) string { return strings.TrimSpace(spaces.ReplaceAllString(s, " ")) }

// unescape undoes the two escapes the dictionary actually uses. A JavaScript
// string can hold more than this, but a translation that needs them is a
// translation that wants rewording.
func unescape(s string) string {
	return strings.NewReplacer(`\"`, `"`, `\\`, `\`).Replace(s)
}

func japanese(t *testing.T, dict string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, m := range dictEntry.FindAllStringSubmatch(dict, -1) {
		out[unescape(m[1])] = unescape(m[2])
	}
	if len(out) == 0 {
		t.Fatal("no entries were read out of the dictionary")
	}
	return out
}

// A sentence added to the page without a translation shows up in English for
// everyone reading in Japanese, and nothing else reports it.
func TestEveryStringOnTheSettingsPageIsTranslated(t *testing.T) {
	html, js := settingsPage(t)
	markup, script, dict := split(t, html, js)
	ja := japanese(t, dict)

	english := pageEnglish(t, markup, script)
	// A scanner that quietly matched nothing would pass this test for the
	// worst possible reason.
	if len(english) < 80 {
		t.Fatalf("only %d translatable strings were found on a page that has "+
			"far more; the scanner is broken, not the page", len(english))
	}

	for _, en := range english {
		if _, ok := ja[en]; !ok {
			t.Errorf("no Japanese for %q", en)
		}
	}
}

// The other direction: an English sentence reworded in the page leaves its old
// self in the dictionary, where it does nothing and looks like a translation.
func TestTheDictionaryHoldsNothingThePageNoLongerSays(t *testing.T) {
	html, js := settingsPage(t)
	markup, script, dict := split(t, html, js)
	ja := japanese(t, dict)

	said := map[string]bool{}
	for _, en := range pageEnglish(t, markup, script) {
		said[en] = true
	}
	// Anything else the script mentions counts too, so that a string used in a
	// way this test does not know about is not reported as stale.
	for _, m := range jsString.FindAllStringSubmatch(script, -1) {
		said[unescape(m[1])] = true
	}

	for en, jp := range ja {
		if !said[en] {
			t.Errorf("the dictionary translates %q, which the page no longer says", en)
		}
		if strings.TrimSpace(jp) == "" {
			t.Errorf("the translation of %q is empty", en)
		}
	}
}
