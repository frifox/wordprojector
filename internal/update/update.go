// Package update looks for new releases of the program on GitHub, downloads one and puts it in the place of the
// running program (a macOS app bundle, or a plain executable on Windows and Linux). The running copy is not stopped:
// the new one is used the next time the program starts, or when Restart is called.
package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
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

// DefaultRepo is the GitHub repository whose releases are used (owner/name).
const DefaultRepo = "frifox/wordprojector"

// Release is one release of the program.
type Release struct {
	Tag        string `json:"tag"`
	Name       string `json:"name"`
	Published  string `json:"published"` // YYYY-MM-DD
	Notes      string `json:"notes"`
	Prerelease bool   `json:"prerelease"`
	URL        string `json:"url"`
	// Installable is true when the release has a build for this computer.
	Installable bool `json:"installable"`

	asset, sums *asset
}

type asset struct {
	Name string
	URL  string
	Size int64
}

// Updater finds and installs releases.
type Updater struct {
	// Repo is "owner/name" on GitHub; API is the base address of its API (changed in tests).
	Repo, API string
	// Web is the address of the repository's web pages (the release feed is read from there when the API will not
	// answer).
	Web string
	// Current is the version of the running program (a date like "2026.10.22", or "dev" for an unnumbered build).
	Current string
	Client  *http.Client
	// Exe is the running executable; GOOS and GOARCH say which build of a release is wanted. They are the real ones
	// unless a test sets them.
	Exe, GOOS, GOARCH string

	AllowTemp bool // for tests, which run from a temporary folder
}

// New returns an Updater for the running program. WORDPROJECTOR_UPDATE_REPO and WORDPROJECTOR_UPDATE_API (for
// testing against another server) can replace the repository and the address of the API.
func New(current string) *Updater {
	u := &Updater{Repo: DefaultRepo, Current: current, Client: &http.Client{Timeout: 30 * time.Minute}, GOOS: runtime.GOOS, GOARCH: runtime.GOARCH}
	if v := os.Getenv("WORDPROJECTOR_UPDATE_REPO"); v != "" {
		u.Repo = v
	}
	u.API = "https://api.github.com/repos/" + u.Repo
	u.Web = "https://github.com/" + u.Repo
	if v := os.Getenv("WORDPROJECTOR_UPDATE_WEB"); v != "" {
		u.Web = strings.TrimRight(v, "/")
	}
	if v := os.Getenv("WORDPROJECTOR_UPDATE_API"); v != "" {
		u.API = strings.TrimRight(v, "/")
	}
	if exe, err := os.Executable(); err == nil {
		if real, err := filepath.EvalSymlinks(exe); err == nil {
			exe = real
		}
		u.Exe = exe
	}
	return u
}

// suffixes of the files of a release, by system
func (u *Updater) suffix() string {
	switch u.GOOS + "/" + u.GOARCH {
	case "darwin/arm64":
		return "-macos-arm64.zip"
	case "darwin/amd64":
		return "-macos-x64.zip"
	case "windows/amd64":
		return "-windows-x64.zip"
	case "windows/arm64":
		return "-windows-arm64.zip"
	case "linux/amd64":
		return "-linux-x64.tar.gz"
	}
	return ""
}

type apiRelease struct {
	Tag        string `json:"tag_name"`
	Name       string `json:"name"`
	Body       string `json:"body"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
	Published  string `json:"published_at"`
	HTMLURL    string `json:"html_url"`
	Assets     []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
		Size int64  `json:"size"`
	} `json:"assets"`
}

// List returns the releases, newest first.
func (u *Updater) List(ctx context.Context) ([]Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.API+"/releases?per_page=50", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "WordProjector/"+u.Current)
	resp, err := u.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		// without a login GitHub allows 60 questions an hour for each internet address; the feed of releases on the web
		// page has no such limit
		list, ferr := u.listFromFeed(ctx)
		if ferr == nil {
			return list, nil
		}
		return nil, fmt.Errorf("GitHub's limit on questions was reached%s; try again later", resetNote(resp))
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub answered %s", resp.Status)
	}
	var in []apiRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(&in); err != nil {
		return nil, fmt.Errorf("unreadable answer from GitHub: %w", err)
	}
	suf := u.suffix()
	out := []Release{}
	for _, r := range in {
		if r.Draft || r.Tag == "" {
			continue
		}
		rel := Release{Tag: r.Tag, Name: r.Name, Notes: r.Body, Prerelease: r.Prerelease, URL: r.HTMLURL}
		if len(r.Published) >= 10 {
			rel.Published = r.Published[:10]
		}
		for _, a := range r.Assets {
			a := asset{Name: a.Name, URL: a.URL, Size: a.Size}
			switch {
			case suf != "" && strings.HasSuffix(a.Name, suf):
				rel.asset = &a
			case a.Name == "checksums.txt" || a.Name == "SHA256SUMS":
				rel.sums = &a
			}
		}
		rel.Installable = rel.asset != nil
		out = append(out, rel)
	}
	return out, nil
}

// ---- versions ---------------------------------------------------------------------------------------------------

// Releases are tagged with the day they were made, YYYY.MM.DD, and ".n" for the n-th one made the same day
// ("2026.10.22", "2026.10.22.2"). A build made from files after a tag has more after it ("2026.10.22-3-gabc"); that is
// ignored here.
var verRe = regexp.MustCompile(`^(\d{4})\.(\d{2})\.(\d{2})(?:\.(\d+))?`)

func parse(v string) (n [4]int, ok bool) {
	m := verRe.FindStringSubmatch(strings.TrimSpace(v))
	if m == nil {
		return n, false
	}
	for i := 0; i < 4; i++ {
		n[i], _ = strconv.Atoi(m[i+1])
	}
	return n, true
}

// Compare returns <0, 0 or >0 when version a is older than, the same as or newer than b. Versions that are not
// dates ("dev") cannot be compared: ok is false then.
func Compare(a, b string) (c int, ok bool) {
	x, ok1 := parse(a)
	y, ok2 := parse(b)
	if !ok1 || !ok2 {
		return 0, false
	}
	for i := 0; i < 4; i++ {
		if x[i] != y[i] {
			if x[i] < y[i] {
				return -1, true
			}
			return 1, true
		}
	}
	return 0, true
}

// Same reports whether two tags name the same version.
func Same(a, b string) bool {
	c, ok := Compare(a, b)
	if !ok {
		return a == b
	}
	return c == 0
}

// Latest is the newest release that is not a pre-release and has a build for this computer.
func Latest(list []Release) *Release {
	for i := range list {
		if !list[i].Prerelease && list[i].Installable {
			return &list[i]
		}
	}
	return nil
}

// Newer reports whether release r is newer than the running program.
func (u *Updater) Newer(r *Release) bool {
	if r == nil {
		return false
	}
	c, ok := Compare(r.Tag, u.Current)
	return ok && c > 0
}

// Supported says whether this running program can replace itself, and if not, why.
func (u *Updater) Supported() (bool, string) {
	if u.suffix() == "" {
		return false, "There are no downloads for " + u.GOOS + "/" + u.GOARCH + "."
	}
	if _, ok := parse(u.Current); !ok {
		return false, "This is a development build without a version number."
	}
	if u.Exe == "" {
		return false, "The place of the running program is not known."
	}
	if !u.AllowTemp && (strings.Contains(u.Exe, string(filepath.Separator)+"go-build") || strings.HasPrefix(u.Exe, os.TempDir()+string(filepath.Separator))) {
		return false, "This copy is run from a temporary folder (a development build)."
	}
	return true, ""
}

// ---- download ---------------------------------------------------------------------------------------------------

func (u *Updater) get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "WordProjector/"+u.Current)
	resp, err := u.Client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		resp.Body.Close()
		return nil, fmt.Errorf("download of %s failed: %s", url, resp.Status)
	}
	return resp, nil
}

// expectedSum finds the SHA-256 listed for a file in the checksum file of the release ("" when it is not listed).
func (u *Updater) expectedSum(ctx context.Context, rel Release) (string, error) {
	if rel.sums == nil {
		return "", nil
	}
	resp, err := u.get(ctx, rel.sums.URL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		f := strings.Fields(line)
		if len(f) >= 2 && strings.TrimPrefix(f[len(f)-1], "*") == rel.asset.Name {
			return strings.ToLower(f[0]), nil
		}
	}
	return "", fmt.Errorf("%s is not in the checksum file of the release", rel.asset.Name)
}

// fetch downloads the build of a release into dir and checks it against the checksum file of the release.
func (u *Updater) fetch(ctx context.Context, rel Release, dir string, progress func(string)) (string, error) {
	want, err := u.expectedSum(ctx, rel)
	if err != nil {
		return "", err
	}
	resp, err := u.get(ctx, rel.asset.URL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	path := filepath.Join(dir, rel.asset.Name)
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	pr := &progressReader{r: resp.Body, total: resp.ContentLength, fn: progress}
	_, err = io.Copy(io.MultiWriter(f, h), pr)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", err
	}
	if got := hex.EncodeToString(h.Sum(nil)); want != "" && got != want {
		return "", fmt.Errorf("the download is damaged (checksum %s, expected %s)", got[:12], want[:12])
	}
	return path, nil
}

type progressReader struct {
	r     io.Reader
	total int64
	n     int64
	last  time.Time
	fn    func(string)
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.n += int64(n)
	if p.fn != nil && time.Since(p.last) > 300*time.Millisecond {
		p.last = time.Now()
		if p.total > 0 {
			p.fn(fmt.Sprintf("Downloading %d%%", p.n*100/p.total))
		} else {
			p.fn(fmt.Sprintf("Downloading %d MB", p.n>>20))
		}
	}
	return n, err
}

// resetNote says when GitHub will answer again, when it said so (" (in about 12 minutes)").
func resetNote(resp *http.Response) string {
	if v := resp.Header.Get("X-RateLimit-Reset"); v != "" {
		if sec, err := strconv.ParseInt(v, 10, 64); err == nil {
			if d := time.Until(time.Unix(sec, 0)); d > 0 {
				return fmt.Sprintf(" (in about %d minutes)", int(d.Minutes())+1)
			}
		}
	}
	return ""
}

type feed struct {
	Entries []struct {
		Title   string `xml:"title"`
		Updated string `xml:"updated"`
		Link    struct {
			Href string `xml:"href,attr"`
		} `xml:"link"`
		Content string `xml:"content"`
	} `xml:"entry"`
}

var tagText = regexp.MustCompile(`<[^>]*>`)

// listFromFeed reads the latest releases from the Atom feed of the repository. The feed holds no list of files: the
// files of a release are named by rule (and a release that has none fails to download, which is then reported).
func (u *Updater) listFromFeed(ctx context.Context) ([]Release, error) {
	resp, err := u.get(ctx, u.Web+"/releases.atom")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var f feed
	if err := xml.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&f); err != nil {
		return nil, err
	}
	suf := u.suffix()
	out := []Release{}
	for _, e := range f.Entries {
		tag := path.Base(e.Link.Href)
		if tag == "" || tag == "." || tag == "/" {
			continue
		}
		rel := Release{Tag: tag, Name: strings.TrimSpace(e.Title), URL: e.Link.Href}
		if len(e.Updated) >= 10 {
			rel.Published = e.Updated[:10]
		}
		rel.Notes = strings.TrimSpace(html.UnescapeString(tagText.ReplaceAllString(e.Content, " ")))
		if suf != "" {
			base := u.Web + "/releases/download/" + tag + "/"
			rel.asset = &asset{Name: "WordProjector-" + tag + suf, URL: base + "WordProjector-" + tag + suf}
			rel.sums = &asset{Name: "checksums.txt", URL: base + "checksums.txt"}
			rel.Installable = true
		}
		out = append(out, rel)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("the feed lists no releases")
	}
	return out, nil
}
