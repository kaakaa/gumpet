package assets

import (
	"io/fs"
	"os"
	"strings"
	"testing"
)

// Every pet directory has to carry a NOTICE, and it has to make it into the
// binary. Nothing else would notice if one were missing: the pet would load
// and walk, and the release's THIRD_PARTY_NOTICES, which collects
// assets/*/NOTICE, would simply leave it out — artwork shipped with no
// attribution. The embed list is written out by hand, so a new directory left
// off it is caught here too.
func TestEveryPetCarriesItsNotice(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	pets := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pets++
		dir := e.Name()
		if _, err := os.Stat(dir + "/NOTICE"); err != nil {
			t.Errorf("%s has no NOTICE saying where its artwork came from", dir)
			continue
		}
		notice, err := fs.ReadFile(Pets, dir+"/NOTICE")
		if err != nil {
			t.Errorf("%s/NOTICE is not embedded; add %s/* to the go:embed line for Pets", dir, dir)
			continue
		}
		// A source to check and a licence to check it against are the two
		// things a NOTICE is for.
		text := string(notice)
		if !strings.Contains(text, "http") {
			t.Errorf("%s/NOTICE names no source to check the artwork against", dir)
		}
		if !strings.Contains(strings.ToLower(text), "licen") && !strings.Contains(text, "CC0") {
			t.Errorf("%s/NOTICE does not say what licence the artwork is under", dir)
		}
	}
	if pets == 0 {
		t.Fatal("found no pet directories; the test is looking in the wrong place")
	}
}
