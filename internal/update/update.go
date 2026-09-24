// Package update replaces a released gumpet with a newer release.
//
// It only ever acts when asked. gumpet's promise is that it makes no outgoing
// connections unless told to, and checking for a new version is one; so there
// is no timer here, only functions a person's click ends up calling.
//
// Everything but the final restart is ordinary code over bytes and files —
// which release is newer, which archive is this machine's, whether its
// checksum matches, what to take out of it, how to put it in place — so it is
// tested against a pretend GitHub rather than the real one.
package update

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// DefaultAPI is where releases are looked up.
const DefaultAPI = "https://api.github.com/repos/kaakaa/gumpet"

// MaxDownload caps an archive. The releases are a few tens of megabytes; a
// response far larger than that is not one of them.
const MaxDownload = 128 << 20

// maxSums caps the checksum file, which is a few lines.
const maxSums = 64 << 10

// Reasons a newer release cannot be installed, as codes rather than sentences
// so the settings page can say them in its own language.
const (
	// ReasonSource is a build from a git checkout, versioned by git describe.
	// Overwriting something a person built themselves with a download is not
	// what they asked for; they update it with git.
	ReasonSource = "source-build"
	// ReasonDev is a build with no version stamped in at all.
	ReasonDev = "dev-build"
	// ReasonPlatform is a machine no release is built for.
	ReasonPlatform = "no-build-for-platform"
)

// ErrUpToDate is returned by Apply when there is nothing newer.
var ErrUpToDate = errors.New("already up to date")

// Version is a release version, or a version git describe made from one.
type Version struct {
	Major, Minor, Patch int
	// Source is set for anything after the tag: "v0.3.0-2-g73777df" is two
	// commits past v0.3.0, and "v0.3.0-dirty" is v0.3.0 with changes. Either
	// is somebody's own build, not the release.
	Source bool
}

var versionPattern = regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)(-.+)?$`)

// ParseVersion reads a version as the Makefile and the release workflow stamp
// it. Anything else, "dev" included, is not a version.
func ParseVersion(s string) (Version, bool) {
	m := versionPattern.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return Version{}, false
	}
	var v Version
	v.Major, _ = strconv.Atoi(m[1])
	v.Minor, _ = strconv.Atoi(m[2])
	v.Patch, _ = strconv.Atoi(m[3])
	v.Source = m[4] != ""
	return v, true
}

// Less reports whether v is an earlier release than o. Numbers are compared as
// numbers: v0.10.0 is later than v0.9.0, which a string comparison gets wrong.
func (v Version) Less(o Version) bool {
	if v.Major != o.Major {
		return v.Major < o.Major
	}
	if v.Minor != o.Minor {
		return v.Minor < o.Minor
	}
	return v.Patch < o.Patch
}

// Status is what a check found.
type Status struct {
	Current string `json:"current"`
	Latest  string `json:"latest"`
	// URL is the release's page, for what changed.
	URL string `json:"url"`
	// Available is whether Latest is newer than what is running.
	Available bool `json:"available"`
	// Installable is whether this copy can replace itself with it.
	Installable bool `json:"installable"`
	// Reason is one of the Reason codes when a newer release cannot be
	// installed.
	Reason string `json:"reason,omitempty"`
}

// Updater checks for, and installs, newer releases.
type Updater struct {
	// Current is the running version.
	Current string
	// Exe is the running gumpet, as os.Executable reports it.
	Exe string
	// API is the repository's API root, [DefaultAPI] outside tests.
	API          string
	Client       *http.Client
	GOOS, GOARCH string
}

// New returns an Updater for the running binary.
func New(current, exe string) *Updater {
	return &Updater{
		Current: current,
		Exe:     exe,
		API:     DefaultAPI,
		Client:  httpsOnly(&http.Client{Timeout: 2 * time.Minute}),
		GOOS:    runtime.GOOS,
		GOARCH:  runtime.GOARCH,
	}
}

// httpsOnly refuses to follow a redirect off https. GitHub hands downloads
// over to another host by redirect; that is fine, but a redirect to plain
// http is not somewhere an executable should come from.
func httpsOnly(c *http.Client) *http.Client {
	c.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if req.URL.Scheme != "https" {
			return fmt.Errorf("refusing to follow a redirect to %s", req.URL.Redacted())
		}
		if len(via) >= 10 {
			return errors.New("too many redirects")
		}
		return nil
	}
	return c
}

type release struct {
	TagName string  `json:"tag_name"`
	HTMLURL string  `json:"html_url"`
	Assets  []asset `json:"assets"`
}

type asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

// Version is the running version.
func (u *Updater) Version() string { return u.Current }

// Check asks GitHub for the latest release and compares it with this one.
func (u *Updater) Check(ctx context.Context) (Status, error) {
	st, _, err := u.check(ctx)
	return st, err
}

func (u *Updater) check(ctx context.Context) (Status, release, error) {
	rel, err := u.latest(ctx)
	if err != nil {
		return Status{}, release{}, err
	}
	return u.status(rel), rel, nil
}

func (u *Updater) latest(ctx context.Context) (release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.API+"/releases/latest", nil)
	if err != nil {
		return release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "gumpet/"+u.Current)

	res, err := u.Client.Do(req)
	if err != nil {
		return release{}, fmt.Errorf("ask GitHub for the latest release: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return release{}, errors.New("there are no releases yet")
	}
	if res.StatusCode != http.StatusOK {
		return release{}, fmt.Errorf("GitHub answered %s", res.Status)
	}
	var rel release
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&rel); err != nil {
		return release{}, fmt.Errorf("read the latest release: %w", err)
	}
	if _, ok := ParseVersion(rel.TagName); !ok {
		return release{}, fmt.Errorf("the latest release is tagged %q, which is not a version", rel.TagName)
	}
	return rel, nil
}

// status compares the running version with rel.
func (u *Updater) status(rel release) Status {
	st := Status{Current: u.Current, Latest: rel.TagName, URL: rel.HTMLURL}
	latest, _ := ParseVersion(rel.TagName)

	cur, ok := ParseVersion(u.Current)
	if !ok {
		// Without a version there is nothing to compare, so no claim is made
		// either way; the page says what the latest is and leaves it there.
		st.Reason = ReasonDev
		return st
	}
	st.Available = cur.Less(latest)
	if !st.Available {
		return st
	}
	switch {
	case cur.Source:
		st.Reason = ReasonSource
	case !hasBuild(u.GOOS, u.GOARCH):
		st.Reason = ReasonPlatform
	default:
		st.Installable = true
	}
	return st
}

// AssetName is the archive a release publishes for goos/goarch. It follows
// the release workflow's naming, and must change with it.
func AssetName(version, goos, goarch string) (string, error) {
	switch {
	case goos == "darwin":
		// One universal binary serves both architectures.
		return "gumpet_" + version + "_darwin_universal.tar.gz", nil
	case goos == "linux" && goarch == "amd64":
		return "gumpet_" + version + "_linux_amd64.tar.gz", nil
	case goos == "windows" && (goarch == "amd64" || goarch == "arm64"):
		return "gumpet_" + version + "_windows_" + goarch + ".zip", nil
	}
	return "", fmt.Errorf("no release is built for %s/%s", goos, goarch)
}

func hasBuild(goos, goarch string) bool {
	_, err := AssetName("v0.0.0", goos, goarch)
	return err == nil
}

// Apply installs the latest release over the running one. It checks again
// rather than trusting an earlier answer: whatever the page last showed may be
// out of date, and this is the step that writes files.
//
// Nothing is replaced unless the whole archive arrived and matched its
// checksum. On success the new files are in place and the caller restarts.
func (u *Updater) Apply(ctx context.Context) (Status, error) {
	st, rel, err := u.check(ctx)
	if err != nil {
		return st, err
	}
	if !st.Available {
		return st, ErrUpToDate
	}
	if !st.Installable {
		return st, fmt.Errorf("%s cannot be installed over this copy (%s)", st.Latest, st.Reason)
	}

	exe, err := filepath.EvalSymlinks(u.Exe)
	if err != nil {
		return st, fmt.Errorf("find the running gumpet: %w", err)
	}
	dir := filepath.Dir(exe)
	// Before downloading anything: finding out afterwards that the folder is
	// not ours to write would waste the download and say so too late.
	if err := writable(dir); err != nil {
		return st, fmt.Errorf("cannot replace gumpet in %s: %w", dir, err)
	}

	name, err := AssetName(rel.TagName, u.GOOS, u.GOARCH)
	if err != nil {
		return st, err
	}
	archiveURL, sumsURL := "", ""
	for _, a := range rel.Assets {
		switch a.Name {
		case name:
			archiveURL = a.URL
		case "SHA256SUMS":
			sumsURL = a.URL
		}
	}
	if archiveURL == "" {
		return st, fmt.Errorf("%s has no %s", rel.TagName, name)
	}
	if sumsURL == "" {
		// Without a checksum there is no telling a cut-off download from a
		// whole one, and a cut-off executable is the one thing this must not
		// install.
		return st, fmt.Errorf("%s has no SHA256SUMS to check it against", rel.TagName)
	}

	sums, err := u.download(ctx, sumsURL, maxSums)
	if err != nil {
		return st, err
	}
	want, err := checksumFor(sums, name)
	if err != nil {
		return st, err
	}
	archive, err := u.download(ctx, archiveURL, MaxDownload)
	if err != nil {
		return st, err
	}
	if got := sha256.Sum256(archive); hex.EncodeToString(got[:]) != want {
		return st, fmt.Errorf("%s does not match its checksum; nothing was replaced", name)
	}

	files, err := extract(name, archive, u.GOOS)
	if err != nil {
		return st, err
	}
	if err := install(exe, files, u.GOOS); err != nil {
		return st, err
	}
	return st, nil
}

func (u *Updater) download(ctx context.Context, url string, limit int64) ([]byte, error) {
	if !strings.HasPrefix(url, "https://") {
		return nil, fmt.Errorf("refusing to download from %s", url)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "gumpet/"+u.Current)
	res, err := u.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", path.Base(url), err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: %s", path.Base(url), res.Status)
	}
	// One byte over the limit is enough to know it is too big.
	body, err := io.ReadAll(io.LimitReader(res.Body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", path.Base(url), err)
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("download %s: larger than %d bytes", path.Base(url), limit)
	}
	return body, nil
}

// checksumFor finds name's line in a sha256sum listing.
func checksumFor(sums []byte, name string) (string, error) {
	sc := bufio.NewScanner(bytes.NewReader(sums))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		// sha256sum marks binary mode with a leading '*' on the name.
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == name {
			sum := strings.ToLower(fields[0])
			if len(sum) != sha256.Size*2 {
				return "", fmt.Errorf("SHA256SUMS has a malformed line for %s", name)
			}
			return sum, nil
		}
	}
	return "", fmt.Errorf("SHA256SUMS does not list %s", name)
}

// wanted are the files taken out of an archive. Everything else in it — the
// README, the licences — is for someone unpacking it by hand.
func wanted(goos string) (gumpet, gumpetctl string) {
	if goos == "windows" {
		return "gumpet.exe", "gumpetctl.exe"
	}
	return "gumpet", "gumpetctl"
}

// extract takes the executables out of an archive. A name is only accepted at
// the top level and only as a plain file: an archive has no business putting
// anything in a subdirectory, a parent directory or behind a link, and this
// never writes where the name says in any case.
func extract(name string, data []byte, goos string) (map[string][]byte, error) {
	main, ctl := wanted(goos)
	out := map[string][]byte{}
	take := func(entry string, regular bool, r io.Reader) error {
		clean := path.Clean(strings.TrimPrefix(entry, "./"))
		if clean != main && clean != ctl {
			return nil
		}
		if !regular {
			return fmt.Errorf("%s in the archive is not a plain file", clean)
		}
		body, err := io.ReadAll(io.LimitReader(r, MaxDownload+1))
		if err != nil {
			return err
		}
		if len(body) > MaxDownload {
			return fmt.Errorf("%s in the archive is too large", clean)
		}
		out[clean] = body
		return nil
	}

	switch {
	case strings.HasSuffix(name, ".tar.gz"):
		gz, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", name, err)
		}
		tr := tar.NewReader(gz)
		for {
			h, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("read %s: %w", name, err)
			}
			if err := take(h.Name, h.Typeflag == tar.TypeReg, tr); err != nil {
				return nil, err
			}
		}
	case strings.HasSuffix(name, ".zip"):
		zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", name, err)
		}
		for _, f := range zr.File {
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("read %s: %w", name, err)
			}
			err = take(f.Name, f.Mode().IsRegular(), rc)
			rc.Close()
			if err != nil {
				return nil, err
			}
		}
	default:
		return nil, fmt.Errorf("do not know how to open %s", name)
	}

	if _, ok := out[main]; !ok {
		return nil, fmt.Errorf("%s has no %s in it", name, main)
	}
	return out, nil
}

// writable proves dir can take a new file, by making one.
func writable(dir string) error {
	f, err := os.CreateTemp(dir, ".gumpet-write-test-*")
	if err != nil {
		return err
	}
	name := f.Name()
	f.Close()
	return os.Remove(name)
}

// install puts files in place of exe and, if it sits beside it, gumpetctl.
//
// Everything is written out in full before anything is replaced, so a failure
// part-way leaves the old gumpet exactly as it was. The replacing is a rename
// in the same directory, which is atomic where it matters.
//
// Windows will not let a running executable be overwritten, but it will let
// one be renamed; so there the old one steps aside as .old first, and is
// cleared away the next time gumpet starts. See [Cleanup].
func install(exe string, files map[string][]byte, goos string) error {
	main, ctl := wanted(goos)
	dir := filepath.Dir(exe)

	type swap struct{ tmp, target string }
	var swaps []swap
	cleanup := func() {
		for _, s := range swaps {
			os.Remove(s.tmp)
		}
	}

	targets := map[string]string{main: exe}
	// gumpetctl is replaced only where there is one to replace: installing a
	// file the person did not put there is not updating.
	if ctlPath := filepath.Join(dir, ctl); fileExists(ctlPath) {
		if _, ok := files[ctl]; ok {
			targets[ctl] = ctlPath
		}
	}

	for _, name := range []string{main, ctl} {
		target, ok := targets[name]
		if !ok {
			continue
		}
		tmp := filepath.Join(dir, "."+name+".update")
		if err := os.WriteFile(tmp, files[name], 0o755); err != nil {
			cleanup()
			return fmt.Errorf("write the new %s: %w", name, err)
		}
		// WriteFile's mode is filtered by the umask; an executable that came
		// out 0644 would install fine and then refuse to run.
		if err := os.Chmod(tmp, 0o755); err != nil {
			cleanup()
			return err
		}
		swaps = append(swaps, swap{tmp: tmp, target: target})
	}

	for _, s := range swaps {
		if goos == "windows" {
			old := s.target + ".old"
			os.Remove(old)
			if err := os.Rename(s.target, old); err != nil {
				cleanup()
				return fmt.Errorf("move the old %s aside: %w", filepath.Base(s.target), err)
			}
		}
		if err := os.Rename(s.tmp, s.target); err != nil {
			cleanup()
			return fmt.Errorf("put the new %s in place: %w", filepath.Base(s.target), err)
		}
	}
	return nil
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}

// Cleanup removes what an update on Windows had to leave behind: the old
// executables, which could not be deleted while they were still running.
// Elsewhere there is never anything to remove, and it does nothing.
func Cleanup(exe string) {
	exe, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return
	}
	dir := filepath.Dir(exe)
	for _, name := range []string{filepath.Base(exe), "gumpetctl.exe"} {
		os.Remove(filepath.Join(dir, name+".old"))
	}
}
