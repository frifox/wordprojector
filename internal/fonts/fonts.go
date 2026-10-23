// Package fonts enumerates font families installed on the system by reading
// the name tables of TrueType/OpenType files in the standard font folders.
package fonts

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"golang.org/x/image/font/sfnt"
)

var (
	once  sync.Once
	cache []string
)

func dirs() []string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "darwin":
		return []string{"/System/Library/Fonts", "/System/Library/Fonts/Supplemental", "/Library/Fonts", filepath.Join(home, "Library", "Fonts")}
	case "windows":
		win := os.Getenv("WINDIR")
		if win == "" {
			win = `C:\Windows`
		}
		return []string{filepath.Join(win, "Fonts"), filepath.Join(os.Getenv("LOCALAPPDATA"), "Microsoft", "Windows", "Fonts")}
	default:
		return []string{"/usr/share/fonts", "/usr/local/share/fonts", filepath.Join(home, ".fonts"), filepath.Join(home, ".local", "share", "fonts")}
	}
}

// Families returns the sorted list of installed font family names (cached).
func Families() []string {
	once.Do(func() { cache = scan() })
	return cache
}

func scan() []string {
	set := map[string]bool{}
	var buf sfnt.Buffer
	for _, d := range dirs() {
		_ = filepath.WalkDir(d, func(path string, de os.DirEntry, err error) error {
			if err != nil || de.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			if ext != ".ttf" && ext != ".otf" && ext != ".ttc" {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			if ext == ".ttc" {
				c, err := sfnt.ParseCollection(data)
				if err != nil {
					return nil
				}
				for i := 0; i < c.NumFonts(); i++ {
					f, err := c.Font(i)
					if err != nil {
						continue
					}
					if n, err := f.Name(&buf, sfnt.NameIDFamily); err == nil && n != "" {
						set[n] = true
					}
				}
				return nil
			}
			f, err := sfnt.Parse(data)
			if err != nil {
				return nil
			}
			if n, err := f.Name(&buf, sfnt.NameIDFamily); err == nil && n != "" {
				set[n] = true
			}
			return nil
		})
	}
	// Always offer the classic families even when they are not detected.
	for _, n := range []string{"Arial", "Helvetica", "Times New Roman", "Verdana", "Georgia", "Tahoma", "Courier New"} {
		set[n] = true
	}
	out := make([]string, 0, len(set))
	for n := range set {
		if strings.HasPrefix(n, ".") {
			continue
		}
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
