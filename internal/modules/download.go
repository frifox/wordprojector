package modules

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Module servers of the original project.
const (
	BibleListURL    = "http://softprojector.org/bibles/bible.xml"
	SongbookListURL = "http://softprojector.org/songbooks/songbooks.xml"
	ThemeListURL    = "http://softprojector.org/themes/themes.xml"
)

// Sources of module lists the user can pick (Books > Import from Cloud). A source has the same layout as the original
// server: /bibles/bible.xml, /songbooks/songbooks.xml and /themes/themes.xml.
const SoftProjectorURL = "http://softprojector.org"

var listPaths = map[string]string{"bible": "/bibles/bible.xml", "songbook": "/songbooks/songbooks.xml", "theme": "/themes/themes.xml"}

// ListURL is the module list address of a kind (bible, songbook, theme) on a source: "softprojector"
// or "custom", whose address is a site (the usual list paths are added) or the address of a list itself (.xml).
func ListURL(kind, source, custom string) (string, error) {
	path, ok := listPaths[kind]
	if !ok {
		return "", fmt.Errorf("unknown module kind")
	}
	base := ""
	switch source {
	case "", "softprojector":
		base = SoftProjectorURL
	case "custom":
		base = strings.TrimSpace(custom)
		if !strings.Contains(base, "://") {
			base = "http://" + base
		}
		u, err := url.Parse(base)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
			return "", fmt.Errorf("enter a web address, like http://example.org")
		}
		if strings.HasSuffix(strings.ToLower(u.Path), ".xml") {
			return base, nil
		}
	default:
		return "", fmt.Errorf("unknown source")
	}
	return strings.TrimRight(base, "/") + path, nil
}

// Module is one downloadable item from a module list.
type Module struct {
	Name string `json:"name" xml:"name"`
	Link string `json:"link" xml:"link"`
	Size int64  `json:"size" xml:"size"`
}

type moduleList struct {
	XMLName xml.Name `xml:"Modules"`
	Modules []Module `xml:"Module"`
}

// UserAgent is sent to the module servers (they reject the default Go agent).
var UserAgent = "WordProjector/1.0 (+https://github.com/wordprojector)"

var client = &http.Client{Timeout: 10 * time.Minute}

// FetchList downloads and parses a module list.
func FetchList(ctx context.Context, listURL string) ([]Module, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, listURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("server returned %s for %s", resp.Status, listURL)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	var ml moduleList
	if err := xml.Unmarshal(data, &ml); err != nil {
		return nil, fmt.Errorf("invalid module list: %w", err)
	}
	for i := range ml.Modules {
		ml.Modules[i].Name = strings.TrimSpace(ml.Modules[i].Name)
		ml.Modules[i].Link = strings.TrimSpace(ml.Modules[i].Link)
	}
	return ml.Modules, nil
}

var unsafeChars = regexp.MustCompile(`[^\p{L}\p{N}._ -]+`)

// SaveFileName computes the destination path for a module (port of getSaveFileName).
func SaveFileName(dir, link string) string {
	u, err := url.Parse(link)
	base := "download"
	if err == nil {
		if b := path.Base(u.Path); b != "" && b != "/" && b != "." {
			if ub, err := url.PathUnescape(b); err == nil {
				base = ub
			} else {
				base = b
			}
		}
	}
	base = unsafeChars.ReplaceAllString(base, "_")
	p := filepath.Join(dir, base)
	if _, err := os.Stat(p); err == nil {
		if err := os.Remove(p); err != nil {
			ext := filepath.Ext(base)
			stem := strings.TrimSuffix(base, ext)
			for i := 1; ; i++ {
				p = filepath.Join(dir, fmt.Sprintf("%s_%d%s", stem, i, ext))
				if _, err := os.Stat(p); err != nil {
					break
				}
			}
		}
	}
	return p
}

// Download fetches link into dest, reporting progress (bytes).
func Download(ctx context.Context, link, dest string, size int64, progress Progress) error {
	if progress == nil {
		progress = noProgress
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err != nil {
		return err
	}
	// The servers host files with spaces in names; make sure they are escaped.
	req.URL.RawPath = ""
	req.Header.Set("User-Agent", UserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("server returned %s", resp.Status)
	}
	if resp.ContentLength > 0 {
		size = resp.ContentLength
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	buf := make([]byte, 64<<10)
	var done int64
	start := time.Now()
	last := time.Now()
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				return werr
			}
			done += int64(n)
			if time.Since(last) > 200*time.Millisecond {
				last = time.Now()
				progress(int(done), int(size), speed(done, time.Since(start)))
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	progress(int(done), int(size), speed(done, time.Since(start)))
	return nil
}

func speed(bytes int64, d time.Duration) string {
	if d <= 0 {
		return ""
	}
	s := float64(bytes) / d.Seconds()
	switch {
	case s < 1024:
		return fmt.Sprintf("%.0f bytes/sec", s)
	case s < 1024*1024:
		return fmt.Sprintf("%.1f kB/s", s/1024)
	default:
		return fmt.Sprintf("%.1f MB/s", s/1024/1024)
	}
}
