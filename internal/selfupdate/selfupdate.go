// Package selfupdate replaces the running lantern binary with a GitHub release:
// the same download, checksum, and install steps install.sh takes, minus its
// "no sha256 tool" escape hatch — verification here is mandatory.
package selfupdate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	repo         = "kjrocker/pglantern-cli"
	maxArchive   = 100 << 20
	maxChecksums = 1 << 20
)

// Updater fetches releases and installs them. Every field is overridable so
// tests can point it at an httptest server.
type Updater struct {
	APIBase      string // GitHub API repo root; /releases/latest is appended
	DownloadBase string // release download root; /<tag>/<asset> is appended
	HTTP         *http.Client
	OS, Arch     string
	UserAgent    string
}

// New returns an Updater for the published GitHub releases, identifying itself
// as the given lantern version.
func New(version string) *Updater {
	return &Updater{
		APIBase:      "https://api.github.com/repos/" + repo,
		DownloadBase: "https://github.com/" + repo + "/releases/download",
		HTTP:         &http.Client{Timeout: 60 * time.Second},
		OS:           runtime.GOOS,
		Arch:         runtime.GOARCH,
		UserAgent:    "lantern/" + version,
	}
}

type statusError struct {
	url    string
	status int
}

func (e *statusError) Error() string {
	return fmt.Sprintf("GET %s: %d %s", e.url, e.status, http.StatusText(e.status))
}

func (u *Updater) get(ctx context.Context, url string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", u.UserAgent)
	resp, err := u.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, &statusError{url: url, status: resp.StatusCode}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("GET %s: response larger than %d bytes", url, limit)
	}
	return body, nil
}

// Latest returns the tag of the newest published release, e.g. "v0.9.0".
func (u *Updater) Latest(ctx context.Context) (string, error) {
	body, err := u.get(ctx, u.APIBase+"/releases/latest", maxChecksums)
	if err != nil {
		var se *statusError
		if errors.As(err, &se) {
			switch se.status {
			case http.StatusForbidden, http.StatusTooManyRequests:
				return "", errors.New("GitHub rate limit hit looking up the latest release; retry later or pass --to vX.Y.Z")
			case http.StatusNotFound:
				return "", fmt.Errorf("no releases found for %s", repo)
			}
		}
		return "", fmt.Errorf("looking up the latest release: %w", err)
	}
	var rel struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(body, &rel); err != nil || rel.TagName == "" {
		return "", errors.New("could not determine the latest release; pass --to vX.Y.Z")
	}
	return rel.TagName, nil
}

// Install downloads release tag for this platform, verifies it against the
// release's checksums.txt, and atomically replaces exe with it. Progress lines
// go to log. On any failure exe is left untouched.
func (u *Updater) Install(ctx context.Context, tag, exe string, log io.Writer) error {
	if !supported(u.OS, u.Arch) {
		return fmt.Errorf("no prebuilt lantern binaries for %s/%s; build from source: https://github.com/%s#install", u.OS, u.Arch, repo)
	}
	version := strings.TrimPrefix(tag, "v")
	archive := fmt.Sprintf("lantern_%s_%s_%s.tar.gz", version, u.OS, u.Arch)
	base := u.DownloadBase + "/" + tag + "/"

	fmt.Fprintf(log, "downloading lantern %s (%s/%s)\n", tag, u.OS, u.Arch)
	data, err := u.get(ctx, base+archive, maxArchive)
	if err != nil {
		return assetError(tag, archive, err)
	}
	sums, err := u.get(ctx, base+"checksums.txt", maxChecksums)
	if err != nil {
		return assetError(tag, "checksums.txt", err)
	}
	if err := verify(data, sums, archive); err != nil {
		return err
	}
	fmt.Fprintln(log, "checksum ok")

	bin, err := extract(data)
	if err != nil {
		return err
	}
	return replace(ctx, exe, bin, version)
}

func supported(goos, goarch string) bool {
	return (goos == "linux" || goos == "darwin") && (goarch == "amd64" || goarch == "arm64")
}

func assetError(tag, name string, err error) error {
	var se *statusError
	if errors.As(err, &se) && se.status == http.StatusNotFound {
		return fmt.Errorf("release %s has no %s (does that release exist?)", tag, name)
	}
	return fmt.Errorf("downloading %s: %w", name, err)
}

// verify checks data against its line in a goreleaser checksums.txt
// ("<sha256>  <file>"). An archive missing from the file is as fatal as a
// mismatch.
func verify(data, sums []byte, archive string) error {
	got := sha256.Sum256(data)
	for _, line := range strings.Split(string(sums), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || strings.TrimPrefix(fields[1], "*") != archive {
			continue
		}
		if !strings.EqualFold(fields[0], hex.EncodeToString(got[:])) {
			return fmt.Errorf("checksum verification failed for %s", archive)
		}
		return nil
	}
	return fmt.Errorf("%s is not listed in checksums.txt; refusing to install it unverified", archive)
}

// extract pulls the lantern binary from the root of a release tarball.
func extract(data []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("reading archive: %w", err)
	}
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil, errors.New("archive did not contain a lantern binary")
		}
		if err != nil {
			return nil, fmt.Errorf("reading archive: %w", err)
		}
		if hdr.Typeflag == tar.TypeReg && path.Clean(hdr.Name) == "lantern" {
			return io.ReadAll(io.LimitReader(tr, maxArchive))
		}
	}
}

// replace writes bin next to exe, proves it runs and reports the expected
// version, then renames it over exe — atomic on one filesystem, and safe while
// exe is the running process.
func replace(ctx context.Context, exe string, bin []byte, version string) (err error) {
	dir := filepath.Dir(exe)
	tmp, err := os.CreateTemp(dir, ".lantern-upgrade-*")
	if err != nil {
		return writeError(dir, err)
	}
	name := tmp.Name()
	defer func() {
		if err != nil {
			os.Remove(name)
		}
	}()

	_, err = tmp.Write(bin)
	if err == nil {
		err = tmp.Sync()
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Chmod(name, 0o755)
	}
	if err != nil {
		return writeError(dir, err)
	}

	out, runErr := exec.CommandContext(ctx, name, "--version").CombinedOutput()
	if runErr != nil {
		return fmt.Errorf("downloaded binary would not run: %v", runErr)
	}
	if fields := strings.Fields(string(out)); len(fields) == 0 || fields[len(fields)-1] != version {
		return fmt.Errorf("downloaded binary reports %q, expected version %s", strings.TrimSpace(string(out)), version)
	}

	if err = os.Rename(name, exe); err != nil {
		return writeError(dir, err)
	}
	return nil
}

func writeError(dir string, err error) error {
	if errors.Is(err, fs.ErrPermission) {
		return fmt.Errorf("cannot write to %s: re-run with sudo, or reinstall into a writable directory with install.sh --bin-dir", dir)
	}
	return fmt.Errorf("installing into %s: %w", dir, err)
}

// Compare orders two release versions ("v0.9.0" or "0.9.0"): -1 if a < b, 0 if
// equal, 1 if a > b. Anything but MAJOR.MINOR.PATCH is an error.
func Compare(a, b string) (int, error) {
	pa, err := parse(a)
	if err != nil {
		return 0, err
	}
	pb, err := parse(b)
	if err != nil {
		return 0, err
	}
	for i := range 3 {
		if pa[i] < pb[i] {
			return -1, nil
		}
		if pa[i] > pb[i] {
			return 1, nil
		}
	}
	return 0, nil
}

func parse(v string) ([3]int, error) {
	var out [3]int
	parts := strings.Split(strings.TrimPrefix(v, "v"), ".")
	if len(parts) != 3 {
		return out, fmt.Errorf("not a release version: %q", v)
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return out, fmt.Errorf("not a release version: %q", v)
		}
		out[i] = n
	}
	return out, nil
}
