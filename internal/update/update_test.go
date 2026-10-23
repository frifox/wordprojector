package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompare(t *testing.T) {
	cases := []struct {
		a, b string
		c    int
		ok   bool
	}{
		{"2026-10-22", "2026-10-22", 0, true}, {"2026-10-23", "2026-10-22", 1, true}, {"2026-10-22.2", "2026-10-22", 1, true},
		{"2026-10-22.10", "2026-10-22.9", 1, true}, {"2025-12-31", "2026-01-01", -1, true},
		{"2026-10-22-3-gabc-dirty", "2026-10-22", 0, true}, {"dev", "2026-10-22", 0, false}, {"abc1234", "2026-10-22", 0, false},
	}
	for _, c := range cases {
		got, ok := Compare(c.a, c.b)
		if got != c.c || ok != c.ok {
			t.Errorf("Compare(%q,%q) = %d,%v; want %d,%v", c.a, c.b, got, ok, c.c, c.ok)
		}
	}
}

func zipOf(files map[string]string) []byte {
	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	for name, body := range files {
		h := &zip.FileHeader{Name: name, Method: zip.Deflate}
		h.SetMode(0o755)
		w, _ := zw.CreateHeader(h)
		w.Write([]byte(body))
	}
	zw.Close()
	return b.Bytes()
}

func tgzOf(files map[string]string) []byte {
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg})
		tw.Write([]byte(body))
	}
	tw.Close()
	gz.Close()
	return b.Bytes()
}

// a fake GitHub: one release with a build file (and a checksum file)
func fakeGitHub(t *testing.T, assetName string, data []byte, goodSum bool) *httptest.Server {
	t.Helper()
	sum := sha256.Sum256(data)
	hexSum := hex.EncodeToString(sum[:])
	if !goodSum {
		hexSum = strings.Repeat("0", 64)
	}
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/releases":
			json.NewEncoder(w).Encode([]map[string]any{
				{"tag_name": "2026-02-01.2", "prerelease": true, "published_at": "2026-02-01T10:00:00Z"},
				{"tag_name": "2026-01-02", "name": "One point one", "body": "notes", "published_at": "2026-01-02T10:00:00Z", "html_url": "http://x/y",
					"assets": []map[string]any{
						{"name": assetName, "browser_download_url": srv.URL + "/dl/" + assetName, "size": len(data)},
						{"name": "checksums.txt", "browser_download_url": srv.URL + "/dl/checksums.txt", "size": 10},
					}},
				{"tag_name": "2025-12-01", "published_at": "2025-12-01T10:00:00Z"},
			})
		case "/dl/" + assetName:
			w.Write(data)
		case "/dl/checksums.txt":
			w.Write([]byte(hexSum + "  " + assetName + "\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newUpdater(srv *httptest.Server, goos, goarch, exe string) *Updater {
	return &Updater{Repo: "x/y", API: srv.URL, Current: "2025-12-01", Client: srv.Client(), GOOS: goos, GOARCH: goarch, Exe: exe, AllowTemp: true}
}

func listed(t *testing.T, u *Updater) (Release, []Release) {
	t.Helper()
	list, err := u.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	l := Latest(list)
	if l == nil || l.Tag != "2026-01-02" {
		t.Fatalf("latest = %+v", l)
	}
	if !u.Newer(l) {
		t.Fatal("2026-01-02 should be newer than 2025-12-01")
	}
	if list[0].Installable || list[2].Installable {
		t.Fatal("releases without a build must not be installable")
	}
	return *l, list
}

func TestInstallMacBundle(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "Word Projector.app", "Contents", "MacOS", "WordProjector")
	os.MkdirAll(filepath.Dir(exe), 0o755)
	os.WriteFile(exe, []byte("old"), 0o755)
	data := zipOf(map[string]string{"WordProjector.app/Contents/MacOS/WordProjector": "new", "WordProjector.app/Contents/Info.plist": "plist"})
	srv := fakeGitHub(t, "WordProjector-2026-01-02-macos-arm64.zip", data, true)
	u := newUpdater(srv, "darwin", "arm64", exe)
	rel, _ := listed(t, u)
	if err := u.Install(context.Background(), rel, nil); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(exe)
	if string(got) != "new" {
		t.Fatalf("executable = %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "Word Projector.app", "Contents", "Info.plist")); err != nil {
		t.Fatal("the rest of the bundle was not replaced")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Fatalf("left behind: %v", entries)
	}
}

func TestInstallLinux(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "WordProjector")
	os.WriteFile(exe, []byte("old"), 0o755)
	data := tgzOf(map[string]string{"WordProjector/WordProjector": "new", "WordProjector/LICENSE": "x"})
	srv := fakeGitHub(t, "WordProjector-2026-01-02-linux-x64.tar.gz", data, true)
	u := newUpdater(srv, "linux", "amd64", exe)
	rel, _ := listed(t, u)
	if err := u.Install(context.Background(), rel, nil); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(exe)
	fi, _ := os.Stat(exe)
	if string(got) != "new" || fi.Mode()&0o100 == 0 {
		t.Fatalf("executable = %q %v", got, fi.Mode())
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Fatalf("left behind: %v", entries)
	}
}

func TestInstallWindowsKeepsOldUntilCleanup(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "WordProjector.exe")
	os.WriteFile(exe, []byte("old"), 0o755)
	data := zipOf(map[string]string{"WordProjector.exe": "new", "LICENSE.txt": "x"})
	srv := fakeGitHub(t, "WordProjector-2026-01-02-windows-x64.zip", data, true)
	u := newUpdater(srv, "windows", "amd64", exe)
	rel, _ := listed(t, u)
	if err := u.Install(context.Background(), rel, nil); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(exe)
	old, _ := os.ReadFile(exe + ".old")
	if string(got) != "new" || string(old) != "old" {
		t.Fatalf("exe %q, old %q", got, old)
	}
	u.Cleanup()
	if _, err := os.Stat(exe + ".old"); err == nil {
		t.Fatal("the old copy was not removed")
	}
}

func TestBadChecksumLeavesProgramAlone(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "WordProjector")
	os.WriteFile(exe, []byte("old"), 0o755)
	data := tgzOf(map[string]string{"WordProjector/WordProjector": "new"})
	srv := fakeGitHub(t, "WordProjector-2026-01-02-linux-x64.tar.gz", data, false)
	u := newUpdater(srv, "linux", "amd64", exe)
	rel, _ := listed(t, u)
	if err := u.Install(context.Background(), rel, nil); err == nil || !strings.Contains(err.Error(), "damaged") {
		t.Fatalf("err = %v", err)
	}
	got, _ := os.ReadFile(exe)
	if string(got) != "old" {
		t.Fatal("the program was replaced by a damaged download")
	}
}

func TestZipSlipRefused(t *testing.T) {
	dir := t.TempDir()
	zp := filepath.Join(dir, "evil.zip")
	os.WriteFile(zp, zipOf(map[string]string{"../evil.txt": "x"}), 0o644)
	if err := extract(zp, filepath.Join(dir, "out")); err == nil {
		t.Fatal("a path leaving the folder was accepted")
	}
	if _, err := os.Stat(filepath.Join(dir, "evil.txt")); err == nil {
		t.Fatal("file written outside")
	}
}

func TestSupported(t *testing.T) {
	u := &Updater{Current: "dev", GOOS: "linux", GOARCH: "amd64", Exe: "/opt/wp/WordProjector"}
	if ok, _ := u.Supported(); ok {
		t.Fatal("a dev build must not update itself")
	}
	u.Current = "2025-12-01"
	if ok, why := u.Supported(); !ok {
		t.Fatal(why)
	}
	u.GOARCH = "riscv64"
	if ok, _ := u.Supported(); ok {
		t.Fatal("no build for riscv64")
	}
}
