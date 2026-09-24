package lang

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// spoken are the packages whose words the pet says. Their source is read, not
// imported: internal/pet imports Ebitengine, and a test binary that did the
// same could not start without a screen.
var spoken = []string{"../pet", "../quiet"}

// saidInSource finds every string literal handed to a function called tr —
// g.tr, or a tr passed down to a helper. That is the convention the pet keeps
// to, and the only way this test can see what it says.
func saidInSource(t *testing.T) map[string][]string {
	t.Helper()
	said := map[string][]string{}
	fset := token.NewFileSet()
	for _, dir := range spoken {
		files, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, path := range files {
			if strings.HasSuffix(path, "_test.go") {
				continue
			}
			f, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				t.Fatalf("parse %s: %v", path, err)
			}
			ast.Inspect(f, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok || len(call.Args) == 0 {
					return true
				}
				var name string
				switch fn := call.Fun.(type) {
				case *ast.Ident:
					name = fn.Name
				case *ast.SelectorExpr:
					name = fn.Sel.Name
				}
				if name != "tr" {
					return true
				}
				lit, ok := call.Args[0].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					// A word worked out at run time cannot be checked, and
					// cannot be found in the dictionary reliably either.
					t.Errorf("%s: tr is given something other than a string literal", fset.Position(call.Pos()))
					return true
				}
				s, err := strconv.Unquote(lit.Value)
				if err != nil {
					t.Fatalf("%s: %v", fset.Position(lit.Pos()), err)
				}
				said[s] = append(said[s], fset.Position(lit.Pos()).String())
				return true
			})
		}
	}
	return said
}

// A word the pet says with no translation shows up in English in a Japanese
// menu, and nothing else would ever report it.
func TestEverythingThePetSaysIsTranslated(t *testing.T) {
	said := saidInSource(t)
	// A scanner that matched nothing would pass for the worst reason.
	if len(said) < 20 {
		t.Fatalf("found only %d words the pet says; the scanner is broken, not the pet", len(said))
	}
	var missing []string
	for en, where := range said {
		if _, ok := japanese[en]; !ok {
			missing = append(missing, strconv.Quote(en)+" at "+where[0])
		}
	}
	sort.Strings(missing)
	for _, m := range missing {
		t.Errorf("no Japanese for %s", m)
	}
}

// And the other way: a word reworded in the code leaves its old translation
// behind, doing nothing.
func TestTheDictionaryHoldsNothingThePetNoLongerSays(t *testing.T) {
	said := saidInSource(t)
	for en, ja := range japanese {
		if _, ok := said[en]; !ok {
			t.Errorf("the dictionary translates %q, which the pet no longer says", en)
		}
		if strings.TrimSpace(ja) == "" {
			t.Errorf("the translation of %q is empty", en)
		}
	}
}

func TestFromLocaleReadsEverySpelling(t *testing.T) {
	cases := []struct {
		in     string
		want   Lang
		wantOK bool
	}{
		{"ja_JP.UTF-8", Japanese, true}, // the environment
		{"ja-JP", Japanese, true},       // macOS and Windows
		{"ja", Japanese, true},
		{"JA_jp", Japanese, true},
		{"ja_JP@calendar=japanese", Japanese, true},
		{"en_US.UTF-8", English, true},
		{"en-JP", English, true},
		// A language with no dictionary is still a decision: English.
		{"fr_FR.UTF-8", English, true},
		// These say nothing, so detection has to look further.
		{"", "", false},
		{"C", "", false},
		{"C.UTF-8", "", false},
		{"POSIX", "", false},
	}
	for _, c := range cases {
		got, ok := FromLocale(c.in)
		if got != c.want || ok != c.wantOK {
			t.Errorf("FromLocale(%q) = %q, %v; want %q, %v", c.in, got, ok, c.want, c.wantOK)
		}
	}
}

// The environment is read first, in the order every Unix program reads it;
// the system is asked only when it says nothing — which is how a Mac app
// started from Finder finds itself.
func TestDetectPrefersTheEnvironmentAndFallsBackToTheSystem(t *testing.T) {
	system := func() string { return "ja-JP" }
	cases := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"LC_ALL wins", map[string]string{"LC_ALL": "en_US.UTF-8", "LANG": "ja_JP.UTF-8"}, "en_US.UTF-8"},
		{"then LC_MESSAGES", map[string]string{"LC_MESSAGES": "ja_JP.UTF-8", "LANG": "en_US.UTF-8"}, "ja_JP.UTF-8"},
		{"then LANG", map[string]string{"LANG": "en_US.UTF-8"}, "en_US.UTF-8"},
		{"started from Finder: nothing set", map[string]string{}, "ja-JP"},
		{"a C locale says nothing", map[string]string{"LANG": "C"}, "ja-JP"},
	}
	for _, c := range cases {
		got := detect(func(k string) string { return c.env[k] }, system)
		if got != c.want {
			t.Errorf("%s: detect = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestResolve(t *testing.T) {
	ja := func() string { return "ja_JP.UTF-8" }
	nothing := func() string { return "" }
	asked := false
	spy := func() string { asked = true; return "ja-JP" }

	cases := []struct {
		setting string
		locale  func() string
		want    Lang
	}{
		{"auto", ja, Japanese},
		{"auto", nothing, English},
		{"en", ja, English},
		{"ja", nothing, Japanese},
	}
	for _, c := range cases {
		if got := Resolve(c.setting, c.locale); got != c.want {
			t.Errorf("Resolve(%q) = %q, want %q", c.setting, got, c.want)
		}
	}
	// A fixed language must not go asking the system: on macOS that means
	// running a program.
	Resolve("en", spy)
	if asked {
		t.Error(`Resolve("en") asked the system for its locale`)
	}
}

func TestParseAppleLanguages(t *testing.T) {
	cases := []struct{ in, want string }{
		{"(\n    \"ja-JP\",\n    \"en-JP\"\n)\n", "ja-JP"},
		{"(\n    \"en-US\"\n)\n", "en-US"},
		{"(\n    en,\n    ja\n)\n", "en"},
		{"", ""},
		{"(\n)\n", ""},
	}
	for _, c := range cases {
		if got := parseAppleLanguages(c.in); got != c.want {
			t.Errorf("parseAppleLanguages(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFormatPutsArgumentsWhereTheTemplateSays(t *testing.T) {
	if got := Format("静かにしている間に {0} 件", 12); got != "静かにしている間に 12 件" {
		t.Errorf("Format = %q", got)
	}
	if got := Format("{1} before {0}", "a", "b"); got != "b before a" {
		t.Errorf("Format = %q", got)
	}
}

func TestTFallsBackToEnglish(t *testing.T) {
	if got := Japanese.T("a word nobody translated"); got != "a word nobody translated" {
		t.Errorf("T = %q, want the English back", got)
	}
	if got := English.T("Quit"); got != "Quit" {
		t.Errorf("English T = %q", got)
	}
}

func TestValid(t *testing.T) {
	for _, s := range []string{"auto", "en", "ja"} {
		if !Valid(s) {
			t.Errorf("Valid(%q) = false", s)
		}
	}
	for _, s := range []string{"", "fr", "EN", "japanese"} {
		if Valid(s) {
			t.Errorf("Valid(%q) = true", s)
		}
	}
	// Keep the setting's values and the dictionary's languages in step.
	if _, err := os.Stat("japanese.go"); err != nil {
		t.Error("ja is a setting but there is no dictionary for it")
	}
}
