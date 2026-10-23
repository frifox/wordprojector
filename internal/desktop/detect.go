package desktop

import (
	"context"
	"encoding/json"
	"os/exec"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DetectScreens asks the operating system for the connected displays, their size and their arrangement.
// It is used when the server runs without the native shell (browser mode) and works without cgo:
// macOS through JavaScript for Automation (NSScreen), Windows through PowerShell (System.Windows.Forms)
// and Linux through xrandr. It returns nil when the information can't be had.
func DetectScreens() []Screen {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	var list []Screen
	switch runtime.GOOS {
	case "darwin":
		list = detectDarwin(ctx)
	case "windows":
		list = detectWindows(ctx)
	case "linux":
		list = detectLinux(ctx)
	}
	// same order as the desktop shell: main screen first, then left to right, then top to bottom
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].IsPrimary != list[j].IsPrimary {
			return list[i].IsPrimary
		}
		if list[i].X != list[j].X {
			return list[i].X < list[j].X
		}
		return list[i].Y < list[j].Y
	})
	for i := range list {
		list[i].Index = i
	}
	return list
}

const darwinScript = `ObjC.import('AppKit');
var s = $.NSScreen.screens, n = s.count, out = [];
var mainH = s.objectAtIndex(0).frame.size.height;
for (var i = 0; i < n; i++) {
  var sc = s.objectAtIndex(i), f = sc.frame, name = '';
  try { name = ObjC.unwrap(sc.localizedName); } catch (e) {}
  out.push({name: name, x: f.origin.x, y: mainH - (f.origin.y + f.size.height), width: f.size.width, height: f.size.height, isPrimary: i === 0});
}
JSON.stringify(out);`

func detectDarwin(ctx context.Context) []Screen {
	out, err := exec.CommandContext(ctx, "osascript", "-l", "JavaScript", "-e", darwinScript).Output()
	if err != nil {
		return nil
	}
	return parseScreensJSON(out)
}

const windowsScript = `Add-Type -AssemblyName System.Windows.Forms; ` +
	`ConvertTo-Json -Compress @([System.Windows.Forms.Screen]::AllScreens | ForEach-Object { ` +
	`@{ name = $_.DeviceName; x = $_.Bounds.X; y = $_.Bounds.Y; width = $_.Bounds.Width; height = $_.Bounds.Height; isPrimary = $_.Primary } })`

func detectWindows(ctx context.Context) []Screen {
	out, err := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", windowsScript).Output()
	if err != nil {
		return nil
	}
	return parseScreensJSON(out)
}

var xrandrLine = regexp.MustCompile(`^(\S+) connected( primary)? (\d+)x(\d+)\+(-?\d+)\+(-?\d+)`)

func detectLinux(ctx context.Context) []Screen {
	out, err := exec.CommandContext(ctx, "xrandr", "--query").Output()
	if err != nil {
		return nil
	}
	var list []Screen
	for _, line := range strings.Split(string(out), "\n") {
		m := xrandrLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		w, _ := strconv.Atoi(m[3])
		h, _ := strconv.Atoi(m[4])
		x, _ := strconv.Atoi(m[5])
		y, _ := strconv.Atoi(m[6])
		list = append(list, Screen{Name: m[1], X: x, Y: y, Width: w, Height: h, IsPrimary: m[2] != ""})
	}
	if len(list) > 0 {
		anyPrimary := false
		for _, s := range list {
			anyPrimary = anyPrimary || s.IsPrimary
		}
		if !anyPrimary {
			list[0].IsPrimary = true
		}
	}
	return list
}

// parseScreensJSON reads a JSON array (or a single object, which PowerShell may produce) of screens.
func parseScreensJSON(b []byte) []Screen {
	type raw struct {
		Name      string  `json:"name"`
		X         float64 `json:"x"`
		Y         float64 `json:"y"`
		Width     float64 `json:"width"`
		Height    float64 `json:"height"`
		IsPrimary bool    `json:"isPrimary"`
	}
	var many []raw
	if err := json.Unmarshal(b, &many); err != nil {
		var one raw
		if json.Unmarshal(b, &one) != nil {
			return nil
		}
		many = []raw{one}
	}
	var out []Screen
	for _, r := range many {
		if r.Width <= 0 || r.Height <= 0 {
			continue
		}
		out = append(out, Screen{Name: r.Name, X: int(r.X), Y: int(r.Y), Width: int(r.Width), Height: int(r.Height), IsPrimary: r.IsPrimary})
	}
	return out
}

// detector caches DetectScreens for a few seconds (it starts a process).
type detector struct {
	mu   sync.Mutex
	at   time.Time
	list []Screen
}

var cached detector

// CachedScreens is DetectScreens with a short cache.
func CachedScreens() []Screen {
	cached.mu.Lock()
	defer cached.mu.Unlock()
	if time.Since(cached.at) > 3*time.Second {
		cached.list = DetectScreens()
		cached.at = time.Now()
	}
	return append([]Screen(nil), cached.list...)
}
