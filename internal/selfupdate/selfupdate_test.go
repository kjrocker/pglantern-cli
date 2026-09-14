package selfupdate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testArchive = "lantern_0.10.0_linux_amd64.tar.gz"

// fakeBin stands in for a release binary: it answers --version like cobra does.
func fakeBin(version string) []byte {
	return []byte("#!/bin/sh\necho \"lantern version " + version + "\"\n")
}

func buildArchive(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	hdr := &tar.Header{Name: name, Mode: 0o755, Size: int64(len(content)), Typeflag: tar.TypeReg}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func sumLine(data []byte, name string) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]) + "  " + name + "\n"
}

func updaterFor(t *testing.T, h http.Handler) *Updater {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &Updater{
		APIBase:      srv.URL + "/api",
		DownloadBase: srv.URL + "/dl",
		HTTP:         srv.Client(),
		OS:           "linux",
		Arch:         "amd64",
		UserAgent:    "lantern/test",
	}
}

// releaseServer serves v0.10.0's archive and, unless checksums is empty, its
// checksums.txt.
func releaseServer(t *testing.T, archive []byte, checksums string) *Updater {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/dl/v0.10.0/"+testArchive, func(w http.ResponseWriter, r *http.Request) {
		w.Write(archive)
	})
	if checksums != "" {
		mux.HandleFunc("/dl/v0.10.0/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, checksums)
		})
	}
	return updaterFor(t, mux)
}

// installedExe creates a stand-in for the running binary under a non-default
// name, as install.sh --bin-name would leave it.
func installedExe(t *testing.T) (dir, exe string) {
	t.Helper()
	dir = t.TempDir()
	exe = filepath.Join(dir, "pglantern")
	if err := os.WriteFile(exe, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir, exe
}

func assertOnlyFile(t *testing.T, dir, exe string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s = %q, want %q", exe, got, want)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("dir has %d entries, want only %s (temp file left behind?)", len(entries), filepath.Base(exe))
	}
}

func TestLatest(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		body    string
		want    string
		wantErr string
	}{
		{name: "ok", status: 200, body: `{"tag_name":"v0.10.0"}`, want: "v0.10.0"},
		{name: "rate limited", status: 403, wantErr: "rate limit"},
		{name: "no releases", status: 404, wantErr: "no releases found"},
		{name: "no tag", status: 200, body: `{}`, wantErr: "could not determine"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u := updaterFor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/releases/latest" {
					t.Errorf("requested %s", r.URL.Path)
				}
				w.WriteHeader(tc.status)
				io.WriteString(w, tc.body)
			}))
			got, err := u.Latest(context.Background())
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("Latest = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestInstallReplacesExecutable(t *testing.T) {
	bin := fakeBin("0.10.0")
	archive := buildArchive(t, "lantern", bin)
	u := releaseServer(t, archive, sumLine([]byte("other"), "lantern_0.10.0_darwin_arm64.tar.gz")+sumLine(archive, testArchive))
	dir, exe := installedExe(t)

	var log bytes.Buffer
	if err := u.Install(context.Background(), "v0.10.0", exe, &log); err != nil {
		t.Fatalf("Install: %v\nlog:\n%s", err, log.String())
	}

	assertOnlyFile(t, dir, exe, bin)
	info, err := os.Stat(exe)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Errorf("mode = %v, want 0755", info.Mode().Perm())
	}
	if !strings.Contains(log.String(), "checksum ok") {
		t.Errorf("log missing checksum ok:\n%s", log.String())
	}
}

func TestInstallFailuresLeaveExecutableUntouched(t *testing.T) {
	good := buildArchive(t, "lantern", fakeBin("0.10.0"))
	cases := []struct {
		name      string
		archive   []byte
		checksums func(archive []byte) string
		os        string
		wantErr   string
	}{
		{
			name:      "checksum mismatch",
			archive:   good,
			checksums: func([]byte) string { return strings.Repeat("0", 64) + "  " + testArchive + "\n" },
			wantErr:   "checksum verification failed",
		},
		{
			name:      "archive not in checksums",
			archive:   good,
			checksums: func(a []byte) string { return sumLine(a, "lantern_0.10.0_darwin_arm64.tar.gz") },
			wantErr:   "not listed in checksums.txt",
		},
		{
			name:      "no checksums file",
			archive:   good,
			checksums: func([]byte) string { return "" },
			wantErr:   "has no checksums.txt",
		},
		{
			name:      "no lantern in archive",
			archive:   buildArchive(t, "README.md", []byte("hi")),
			checksums: func(a []byte) string { return sumLine(a, testArchive) },
			wantErr:   "did not contain a lantern binary",
		},
		{
			name:      "binary reports wrong version",
			archive:   buildArchive(t, "lantern", fakeBin("0.9.0")),
			checksums: func(a []byte) string { return sumLine(a, testArchive) },
			wantErr:   "expected version 0.10.0",
		},
		{
			name:      "unsupported platform",
			archive:   good,
			checksums: func(a []byte) string { return sumLine(a, testArchive) },
			os:        "windows",
			wantErr:   "no prebuilt lantern binaries for windows/amd64",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u := releaseServer(t, tc.archive, tc.checksums(tc.archive))
			if tc.os != "" {
				u.OS = tc.os
			}
			dir, exe := installedExe(t)

			err := u.Install(context.Background(), "v0.10.0", exe, io.Discard)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want containing %q", err, tc.wantErr)
			}
			assertOnlyFile(t, dir, exe, []byte("old"))
		})
	}
}

func TestInstallUnwritableDirSuggestsSudo(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	archive := buildArchive(t, "lantern", fakeBin("0.10.0"))
	u := releaseServer(t, archive, sumLine(archive, testArchive))
	dir, exe := installedExe(t)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	err := u.Install(context.Background(), "v0.10.0", exe, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "sudo") {
		t.Fatalf("err = %v, want a sudo hint", err)
	}
}

func TestCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"v0.9.0", "0.10.0", -1},
		{"0.10.0", "v0.9.0", 1},
		{"v1.2.3", "1.2.3", 0},
		{"1.0.0", "0.99.99", 1},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%s_vs_%s", tc.a, tc.b), func(t *testing.T) {
			got, err := Compare(tc.a, tc.b)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("Compare(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
			}
		})
	}

	for _, bad := range []string{"dev", "0.9", "0.9.0-SNAPSHOT-abc", "v1.x.0", ""} {
		if _, err := Compare(bad, "0.9.0"); err == nil {
			t.Errorf("Compare(%q) accepted a non-release version", bad)
		}
	}
}
