package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestParseVersion(t *testing.T) {
	cases := []struct {
		in     string
		want   Version
		wantOK bool
	}{
		{"v0.3.0", Version{0, 3, 0, false}, true},
		{"v1.12.7", Version{1, 12, 7, false}, true},
		// git describe, for a build made after the tag.
		{"v0.3.0-2-g73777df", Version{0, 3, 0, true}, true},
		{"v0.3.0-dirty", Version{0, 3, 0, true}, true},
		{"v0.3.0-2-g73777df-dirty", Version{0, 3, 0, true}, true},
		{"dev", Version{}, false},
		{"0.3.0", Version{}, false},
		{"v0.3", Version{}, false},
		{"", Version{}, false},
		{"73777df", Version{}, false},
	}
	for _, c := range cases {
		got, ok := ParseVersion(c.in)
		if ok != c.wantOK || got != c.want {
			t.Errorf("ParseVersion(%q) = %+v, %v; want %+v, %v", c.in, got, ok, c.want, c.wantOK)
		}
	}
}

// A string comparison puts v0.10.0 before v0.9.0.
func TestLessComparesNumbersNotStrings(t *testing.T) {
	order := []string{"v0.1.0", "v0.2.0", "v0.2.1", "v0.9.0", "v0.10.0", "v1.0.0"}
	for i := 0; i+1 < len(order); i++ {
		a, _ := ParseVersion(order[i])
		b, _ := ParseVersion(order[i+1])
		if !a.Less(b) || b.Less(a) {
			t.Errorf("%s should come before %s", order[i], order[i+1])
		}
	}
	same, _ := ParseVersion("v0.3.0")
	if same.Less(same) {
		t.Error("a version is less than itself")
	}
}

func TestStatus(t *testing.T) {
	cases := []struct {
		name            string
		current, latest string
		goos, goarch    string
		available, inst bool
		reason          string
	}{
		{"a newer release", "v0.3.0", "v0.3.1", "darwin", "arm64", true, true, ""},
		{"already the latest", "v0.3.1", "v0.3.1", "darwin", "arm64", false, false, ""},
		{"ahead of the latest", "v0.4.0", "v0.3.1", "darwin", "arm64", false, false, ""},
		// Built from a checkout after the latest tag: nothing newer to get.
		{"source build at the latest", "v0.3.1-2-gabc1234", "v0.3.1", "darwin", "arm64", false, false, ""},
		// Built from an older checkout: newer exists, but git is how to get it.
		{"source build behind", "v0.3.0-2-gabc1234", "v0.3.1", "darwin", "arm64", true, false, ReasonSource},
		{"no version at all", "dev", "v0.3.1", "darwin", "arm64", false, false, ReasonDev},
		{"a machine with no release", "v0.3.0", "v0.3.1", "linux", "arm64", true, false, ReasonPlatform},
		{"windows on arm", "v0.3.0", "v0.3.1", "windows", "arm64", true, true, ""},
	}
	for _, c := range cases {
		u := &Updater{Current: c.current, GOOS: c.goos, GOARCH: c.goarch}
		st := u.status(release{TagName: c.latest, HTMLURL: "https://github.com/x/y/releases/tag/" + c.latest})
		if st.Available != c.available || st.Installable != c.inst || st.Reason != c.reason {
			t.Errorf("%s: available=%v installable=%v reason=%q; want %v, %v, %q",
				c.name, st.Available, st.Installable, st.Reason, c.available, c.inst, c.reason)
		}
		if st.Current != c.current || st.Latest != c.latest {
			t.Errorf("%s: reports %s → %s, want %s → %s", c.name, st.Current, st.Latest, c.current, c.latest)
		}
	}
}

// These are the names the release workflow publishes. If that changes, this
// must change with it, or every update fails to find its archive.
func TestAssetNameMatchesTheReleaseWorkflow(t *testing.T) {
	cases := []struct{ goos, goarch, want string }{
		{"darwin", "arm64", "gumpet_v0.3.1_darwin_universal.tar.gz"},
		{"darwin", "amd64", "gumpet_v0.3.1_darwin_universal.tar.gz"},
		{"linux", "amd64", "gumpet_v0.3.1_linux_amd64.tar.gz"},
		{"windows", "amd64", "gumpet_v0.3.1_windows_amd64.zip"},
		{"windows", "arm64", "gumpet_v0.3.1_windows_arm64.zip"},
	}
	for _, c := range cases {
		got, err := AssetName("v0.3.1", c.goos, c.goarch)
		if err != nil || got != c.want {
			t.Errorf("AssetName(%s/%s) = %q, %v; want %q", c.goos, c.goarch, got, err, c.want)
		}
	}
	if _, err := AssetName("v0.3.1", "linux", "arm64"); err == nil {
		t.Error("linux/arm64 has no release, but got an asset name")
	}

	// And against the workflow itself, so a rename there fails here.
	workflow, err := os.ReadFile("../../.github/workflows/release.yml")
	if err != nil {
		t.Fatalf("read the release workflow: %v", err)
	}
	for _, part := range []string{"darwin_universal.tar.gz", "gumpet_${GITHUB_REF_NAME}_${{ matrix.name }}", "SHA256SUMS",
		"windows_amd64", "windows_arm64", "linux_amd64", SumsBundle} {
		if !strings.Contains(string(workflow), part) {
			t.Errorf("the release workflow no longer mentions %q, which this package relies on", part)
		}
	}
}

// releaseKey stands in for the real release key, whose private half lives only
// in the release workflow's secrets.
var releaseKey = mustKey()

func mustKey() *ecdsa.PrivateKey {
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}
	return k
}

// bundleFor is what `cosign sign-blob --key --bundle` writes for sums, cut
// down to the fields a bundle with a messageSignature always has.
func bundleFor(t *testing.T, key *ecdsa.PrivateKey, sums []byte) []byte {
	t.Helper()
	digest := sha256.Sum256(sums)
	sig, err := ecdsa.SignASN1(rand.Reader, key, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(map[string]any{
		"mediaType": "application/vnd.dev.sigstore.bundle.v0.3+json",
		"verificationMaterial": map[string]any{
			"publicKey": map[string]any{"hint": "ignored"},
		},
		"messageSignature": map[string]any{
			"messageDigest": map[string]any{"algorithm": "SHA2_256", "digest": digest[:]},
			"signature":     sig,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// fakeGitHub is a GitHub that serves one release.
type fakeGitHub struct {
	*httptest.Server
	mu       sync.Mutex
	requests []string
	tag      string
	files    map[string][]byte // name → bytes, SHA256SUMS included
}

// newFakeGitHub serves files as the release's assets. A SHA256SUMS among them
// is signed with releaseKey, as the workflow would, unless files already says
// what SumsBundle is — nil meaning the release has none.
func newFakeGitHub(t *testing.T, tag string, files map[string][]byte) *fakeGitHub {
	t.Helper()
	if b, ok := files[SumsBundle]; ok && b == nil {
		delete(files, SumsBundle)
	} else if s, ok := files["SHA256SUMS"]; ok && !hasBundle(files) {
		files[SumsBundle] = bundleFor(t, releaseKey, s)
	}
	f := &fakeGitHub{tag: tag, files: files}
	f.Server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.requests = append(f.requests, r.URL.Path)
		f.mu.Unlock()

		if r.URL.Path == "/repos/kaakaa/gumpet/releases/latest" {
			rel := release{TagName: f.tag, HTMLURL: "https://github.com/kaakaa/gumpet/releases/tag/" + f.tag}
			for name := range f.files {
				rel.Assets = append(rel.Assets, asset{Name: name, URL: f.URL + "/dl/" + name})
			}
			_ = json.NewEncoder(w).Encode(rel)
			return
		}
		if body, ok := f.files[strings.TrimPrefix(r.URL.Path, "/dl/")]; ok {
			_, _ = w.Write(body)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(f.Close)
	return f
}

func hasBundle(files map[string][]byte) bool {
	_, ok := files[SumsBundle]
	return ok
}

func (f *fakeGitHub) downloaded(name string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, p := range f.requests {
		if p == "/dl/"+name {
			return true
		}
	}
	return false
}

func (f *fakeGitHub) updater(current, exe, goos string) *Updater {
	u := New(current, exe)
	u.API = f.URL + "/repos/kaakaa/gumpet"
	u.Client = httpsOnly(f.Client())
	u.GOOS, u.GOARCH = goos, "amd64"
	u.Key = &releaseKey.PublicKey
	return u
}

func tarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		// The release workflow tars with -C out ., which names every entry ./x.
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

func zipped(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(body))
	}
	zw.Close()
	return buf.Bytes()
}

func sums(entries map[string][]byte) []byte {
	var b strings.Builder
	for name, body := range entries {
		sum := sha256.Sum256(body)
		fmt.Fprintf(&b, "%s  %s\n", hex.EncodeToString(sum[:]), name)
	}
	return []byte(b.String())
}

// installed makes a directory holding an "old" gumpet and gumpetctl.
func installed(t *testing.T, names ...string) (dir string) {
	t.Helper()
	dir = t.TempDir()
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("old "+n), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func read(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	return string(b)
}

func leftovers(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			out = append(out, e.Name())
		}
	}
	return out
}

func TestApplyReplacesGumpetAndGumpetctl(t *testing.T) {
	archive := tarGz(t, map[string]string{
		"./gumpet": "new gumpet", "./gumpetctl": "new gumpetctl", "./README.md": "readme",
	})
	name := "gumpet_v0.3.1_darwin_universal.tar.gz"
	gh := newFakeGitHub(t, "v0.3.1", map[string][]byte{name: archive, "SHA256SUMS": sums(map[string][]byte{name: archive})})
	dir := installed(t, "gumpet", "gumpetctl")

	st, err := gh.updater("v0.3.0", filepath.Join(dir, "gumpet"), "darwin").Apply(context.Background())
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if st.Latest != "v0.3.1" {
		t.Errorf("installed %s, want v0.3.1", st.Latest)
	}
	for name, want := range map[string]string{"gumpet": "new gumpet", "gumpetctl": "new gumpetctl"} {
		p := filepath.Join(dir, name)
		if got := read(t, p); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
		if st, _ := os.Stat(p); st.Mode().Perm()&0o111 == 0 {
			t.Errorf("%s is not executable (%v); it would install and then refuse to run", name, st.Mode())
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "README.md")); err == nil {
		t.Error("the README was unpacked; only the executables should be")
	}
	if l := leftovers(t, dir); len(l) != 0 {
		t.Errorf("left behind %v", l)
	}
}

// A gumpet installed on its own gets no gumpetctl put beside it.
func TestApplyDoesNotAddAGumpetctlThatWasNotThere(t *testing.T) {
	archive := tarGz(t, map[string]string{"./gumpet": "new gumpet", "./gumpetctl": "new gumpetctl"})
	name := "gumpet_v0.3.1_linux_amd64.tar.gz"
	gh := newFakeGitHub(t, "v0.3.1", map[string][]byte{name: archive, "SHA256SUMS": sums(map[string][]byte{name: archive})})
	dir := installed(t, "gumpet")

	if _, err := gh.updater("v0.3.0", filepath.Join(dir, "gumpet"), "linux").Apply(context.Background()); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "gumpetctl")); err == nil {
		t.Error("a gumpetctl appeared where there was none")
	}
}

// Windows cannot overwrite a running executable, so the old one is renamed
// out of the way, and cleared up on the next start.
func TestApplyOnWindowsMovesTheOldOneAside(t *testing.T) {
	archive := zipped(t, map[string]string{"gumpet.exe": "new gumpet", "gumpetctl.exe": "new gumpetctl"})
	name := "gumpet_v0.3.1_windows_amd64.zip"
	gh := newFakeGitHub(t, "v0.3.1", map[string][]byte{name: archive, "SHA256SUMS": sums(map[string][]byte{name: archive})})
	dir := installed(t, "gumpet.exe", "gumpetctl.exe")
	exe := filepath.Join(dir, "gumpet.exe")

	if _, err := gh.updater("v0.3.0", exe, "windows").Apply(context.Background()); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got := read(t, exe); got != "new gumpet" {
		t.Errorf("gumpet.exe = %q, want the new one", got)
	}
	if got := read(t, exe+".old"); got != "old gumpet.exe" {
		t.Errorf("gumpet.exe.old = %q, want the old one kept aside", got)
	}

	Cleanup(exe)
	for _, n := range []string{"gumpet.exe.old", "gumpetctl.exe.old"} {
		if _, err := os.Stat(filepath.Join(dir, n)); err == nil {
			t.Errorf("Cleanup left %s", n)
		}
	}
}

// The checksum is the only thing between a cut-off download and an executable
// that will not run. A mismatch must replace nothing.
func TestApplyReplacesNothingWhenTheChecksumIsWrong(t *testing.T) {
	archive := tarGz(t, map[string]string{"./gumpet": "new gumpet"})
	name := "gumpet_v0.3.1_darwin_universal.tar.gz"
	wrong := sums(map[string][]byte{name: []byte("something else")})
	gh := newFakeGitHub(t, "v0.3.1", map[string][]byte{name: archive, "SHA256SUMS": wrong})
	dir := installed(t, "gumpet")

	_, err := gh.updater("v0.3.0", filepath.Join(dir, "gumpet"), "darwin").Apply(context.Background())
	if err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("err = %v, want a checksum mismatch", err)
	}
	if got := read(t, filepath.Join(dir, "gumpet")); got != "old gumpet" {
		t.Errorf("gumpet = %q after a failed update, want it untouched", got)
	}
	if l := leftovers(t, dir); len(l) != 0 {
		t.Errorf("left behind %v", l)
	}
}

func TestApplyRefusesARelease(t *testing.T) {
	archive := tarGz(t, map[string]string{"./gumpet": "new gumpet"})
	name := "gumpet_v0.3.1_darwin_universal.tar.gz"
	cases := []struct {
		name  string
		files map[string][]byte
		want  string
	}{
		{"with no checksums", map[string][]byte{name: archive}, "SHA256SUMS"},
		{"whose checksums leave it out", map[string][]byte{name: archive, "SHA256SUMS": sums(map[string][]byte{"other": archive})}, "does not list"},
		{"with no archive for this machine", map[string][]byte{"SHA256SUMS": sums(map[string][]byte{name: archive})}, "has no"},
	}
	for _, c := range cases {
		gh := newFakeGitHub(t, "v0.3.1", c.files)
		dir := installed(t, "gumpet")
		_, err := gh.updater("v0.3.0", filepath.Join(dir, "gumpet"), "darwin").Apply(context.Background())
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want one mentioning %q", c.name, err, c.want)
		}
		if got := read(t, filepath.Join(dir, "gumpet")); got != "old gumpet" {
			t.Errorf("%s: gumpet was replaced", c.name)
		}
	}
}

// Finding out the folder is read-only after a hundred-megabyte download would
// waste it; this has to be known first.
func TestApplyChecksTheFolderBeforeDownloading(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can write anywhere")
	}
	archive := tarGz(t, map[string]string{"./gumpet": "new gumpet"})
	name := "gumpet_v0.3.1_darwin_universal.tar.gz"
	gh := newFakeGitHub(t, "v0.3.1", map[string][]byte{name: archive, "SHA256SUMS": sums(map[string][]byte{name: archive})})
	dir := installed(t, "gumpet")
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	_, err := gh.updater("v0.3.0", filepath.Join(dir, "gumpet"), "darwin").Apply(context.Background())
	if err == nil || !strings.Contains(err.Error(), "cannot replace") {
		t.Fatalf("err = %v, want one saying the folder cannot be written", err)
	}
	if gh.downloaded(name) {
		t.Error("the archive was downloaded before finding the folder was read-only")
	}
}

// A build from source is updated with git. Nothing is downloaded for it.
func TestApplyLeavesASourceBuildAlone(t *testing.T) {
	archive := tarGz(t, map[string]string{"./gumpet": "new gumpet"})
	name := "gumpet_v0.3.1_darwin_universal.tar.gz"
	gh := newFakeGitHub(t, "v0.3.1", map[string][]byte{name: archive, "SHA256SUMS": sums(map[string][]byte{name: archive})})
	dir := installed(t, "gumpet")

	_, err := gh.updater("v0.3.0-2-g73777df", filepath.Join(dir, "gumpet"), "darwin").Apply(context.Background())
	if err == nil {
		t.Fatal("a source build was replaced by a release")
	}
	if gh.downloaded(name) || gh.downloaded("SHA256SUMS") {
		t.Error("files were downloaded for a build that was never going to take them")
	}
	if got := read(t, filepath.Join(dir, "gumpet")); got != "old gumpet" {
		t.Error("gumpet was replaced")
	}
}

func TestApplyWhenAlreadyUpToDate(t *testing.T) {
	gh := newFakeGitHub(t, "v0.3.1", map[string][]byte{})
	dir := installed(t, "gumpet")
	_, err := gh.updater("v0.3.1", filepath.Join(dir, "gumpet"), "darwin").Apply(context.Background())
	if !errors.Is(err, ErrUpToDate) {
		t.Errorf("err = %v, want ErrUpToDate", err)
	}
}

// gumpet installed through a link — as a package manager might — has the file
// it points at replaced, and the link left alone.
func TestApplyReplacesWhatALinkPointsAt(t *testing.T) {
	archive := tarGz(t, map[string]string{"./gumpet": "new gumpet"})
	name := "gumpet_v0.3.1_darwin_universal.tar.gz"
	gh := newFakeGitHub(t, "v0.3.1", map[string][]byte{name: archive, "SHA256SUMS": sums(map[string][]byte{name: archive})})
	dir := installed(t, "gumpet")
	links := t.TempDir()
	link := filepath.Join(links, "gumpet")
	if err := os.Symlink(filepath.Join(dir, "gumpet"), link); err != nil {
		t.Skip("cannot make links here:", err)
	}

	if _, err := gh.updater("v0.3.0", link, "darwin").Apply(context.Background()); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got := read(t, filepath.Join(dir, "gumpet")); got != "new gumpet" {
		t.Errorf("the linked-to file = %q, want the new one", got)
	}
	if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Error("the link was replaced by a file")
	}
}

// Only the two executables come out, only from the top of the archive, and
// only as plain files.
func TestExtractTakesOnlyTheExecutables(t *testing.T) {
	good := tarGz(t, map[string]string{"./gumpet": "g", "./gumpetctl": "c", "./README.md": "r", "sub/gumpet": "wrong", "../gumpet": "escape"})
	files, err := extract("x.tar.gz", good, "darwin")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 || string(files["gumpet"]) != "g" || string(files["gumpetctl"]) != "c" {
		t.Errorf("extracted %v, want exactly gumpet and gumpetctl from the top", keys(files))
	}

	if _, err := extract("x.tar.gz", tarGz(t, map[string]string{"./README.md": "r"}), "darwin"); err == nil {
		t.Error("an archive with no gumpet in it was accepted")
	}

	// A link named gumpet could point anywhere.
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	_ = tw.WriteHeader(&tar.Header{Name: "./gumpet", Typeflag: tar.TypeSymlink, Linkname: "/bin/sh"})
	tw.Close()
	gz.Close()
	if _, err := extract("x.tar.gz", buf.Bytes(), "darwin"); err == nil {
		t.Error("a link named gumpet was accepted as gumpet")
	}
}

func keys(m map[string][]byte) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

// sha256sum writes "*name" for binary mode; both forms must be found.
func TestChecksumForReadsSha256sumOutput(t *testing.T) {
	sum := strings.Repeat("ab", 32)
	listing := []byte(sum + "  plain.tar.gz\n" + sum + " *binary.zip\n")
	for _, name := range []string{"plain.tar.gz", "binary.zip"} {
		if got, err := checksumFor(listing, name); err != nil || got != sum {
			t.Errorf("checksumFor(%s) = %q, %v", name, got, err)
		}
	}
	if _, err := checksumFor(listing, "missing.zip"); err == nil {
		t.Error("found a checksum for a file that is not listed")
	}
}

// An executable must not come from plain http, however it was pointed there.
func TestDownloadRefusesPlainHTTP(t *testing.T) {
	u := New("v0.3.0", "")
	if _, err := u.download(context.Background(), "http://example.com/gumpet.tar.gz", 10); err == nil {
		t.Error("downloaded over plain http")
	}

	// Nor by a redirect.
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer plain.Close()
	tls := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, plain.URL+"/gumpet.tar.gz", http.StatusFound)
	}))
	defer tls.Close()
	u.Client = httpsOnly(tls.Client())
	if _, err := u.download(context.Background(), tls.URL+"/gumpet.tar.gz", 10); err == nil {
		t.Error("followed a redirect to plain http")
	}
}

func TestCheckSaysWhenThereAreNoReleases(t *testing.T) {
	gh := httptest.NewTLSServer(http.NotFoundHandler())
	defer gh.Close()
	u := New("v0.3.0", "")
	u.API, u.Client = gh.URL, gh.Client()
	if _, err := u.Check(context.Background()); err == nil || !strings.Contains(err.Error(), "no releases") {
		t.Errorf("err = %v, want one saying there are no releases", err)
	}
}

// A release is installed only on the release key's signature. Each of these
// is what an attacker able to publish a release, but not holding the key,
// could put up — and each must replace nothing.
func TestApplyRefusesAReleaseNotSignedByTheReleaseKey(t *testing.T) {
	archive := tarGz(t, map[string]string{"./gumpet": "new gumpet"})
	name := "gumpet_v0.3.1_darwin_universal.tar.gz"
	good := sums(map[string][]byte{name: archive})
	other := sums(map[string][]byte{name: archive, "extra": []byte("x")})

	cases := []struct {
		name   string
		bundle []byte
		key    *ecdsa.PublicKey
		want   string
	}{
		{"with no signature", nil, &releaseKey.PublicKey, "no signature"},
		{"signed by some other key", bundleFor(t, mustKey(), good), &releaseKey.PublicKey, "not signed by"},
		{"whose signature is for other checksums", bundleFor(t, releaseKey, other), &releaseKey.PublicKey, "different SHA256SUMS"},
		{"whose signature is not a bundle", []byte("not json"), &releaseKey.PublicKey, "read the signature"},
		{"whose bundle holds no signature", []byte(`{"mediaType":"x"}`), &releaseKey.PublicKey, "no signature"},
		{"to a build with no key", bundleFor(t, releaseKey, good), nil, "no release key"},
	}
	for _, c := range cases {
		gh := newFakeGitHub(t, "v0.3.1", map[string][]byte{name: archive, "SHA256SUMS": good, SumsBundle: c.bundle})
		dir := installed(t, "gumpet")
		u := gh.updater("v0.3.0", filepath.Join(dir, "gumpet"), "darwin")
		u.Key = c.key
		_, err := u.Apply(context.Background())
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want one mentioning %q", c.name, err, c.want)
		}
		if got := read(t, filepath.Join(dir, "gumpet")); got != "old gumpet" {
			t.Errorf("%s: gumpet was replaced", c.name)
		}
		if gh.downloaded(name) {
			t.Errorf("%s: the archive was downloaded before the signature was checked", c.name)
		}
	}
}

// A signature over the digest alone, with no digest noted beside it, is still
// a signature: cosign has written bundles both ways.
func TestVerifySumsNeedsOnlyTheSignature(t *testing.T) {
	s := []byte("abc  gumpet.tar.gz\n")
	digest := sha256.Sum256(s)
	sig, _ := ecdsa.SignASN1(rand.Reader, releaseKey, digest[:])
	b, _ := json.Marshal(map[string]any{"messageSignature": map[string]any{"signature": sig}})
	if err := verifySums(&releaseKey.PublicKey, s, b); err != nil {
		t.Errorf("a bare signature was refused: %v", err)
	}
}

// The key compiled in is the one releases are checked against. If it does not
// parse, every update fails — so it must not ship that way.
func TestTheEmbeddedReleaseKeyIsUsable(t *testing.T) {
	if ReleaseKey() == nil {
		t.Fatalf("cosign.pub is not an ECDSA P-256 public key:\n%s", releaseKeyPEM)
	}
	if New("v0.1.0", "gumpet").Key == nil {
		t.Error("New does not use the release key")
	}
}

func TestParseKeyRefusesWhatIsNotACosignKey(t *testing.T) {
	p384, _ := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	der, _ := x509.MarshalPKIXPublicKey(&p384.PublicKey)
	cases := map[string][]byte{
		"not PEM":        []byte("hello"),
		"a private key":  pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: []byte{1}}),
		"a key on P-384": pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}),
		"a PEM of junk":  pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: []byte{1, 2, 3}}),
	}
	for name, data := range cases {
		if _, err := parseKey(data); err == nil {
			t.Errorf("%s was accepted as a release key", name)
		}
	}
}
