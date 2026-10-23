package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const stagingPrefix = ".wordprojector-update-"

// where the running program lives: a macOS app bundle, or a plain executable
type place struct {
	bundle string // /Applications/Word Projector.app (macOS app only)
	exe    string
}

func (u *Updater) place() place {
	exe := u.Exe
	if u.GOOS == "darwin" {
		if i := strings.Index(exe, ".app"+string(filepath.Separator)+"Contents"+string(filepath.Separator)+"MacOS"+string(filepath.Separator)); i >= 0 {
			return place{bundle: exe[:i+len(".app")], exe: exe}
		}
	}
	return place{exe: exe}
}

// Install downloads a release and puts it where the running program is. The running program keeps going; the new one
// is what starts next (see Restart). progress gets short status texts.
func (u *Updater) Install(ctx context.Context, rel Release, progress func(string)) error {
	if ok, why := u.Supported(); !ok {
		return fmt.Errorf("%s", why)
	}
	if !rel.Installable {
		return fmt.Errorf("release %s has no download for this computer", rel.Tag)
	}
	pl := u.place()
	parent := filepath.Dir(pl.exe)
	if pl.bundle != "" {
		parent = filepath.Dir(pl.bundle)
	}
	// work next to the program, on the same disk, so that the replacement is a rename
	staging, err := os.MkdirTemp(parent, stagingPrefix)
	if err != nil {
		return fmt.Errorf("cannot write to %s: %w", parent, err)
	}
	defer os.RemoveAll(staging)

	say := func(s string) {
		if progress != nil {
			progress(s)
		}
	}
	say("Downloading")
	archive, err := u.fetch(ctx, rel, staging, say)
	if err != nil {
		return err
	}
	say("Unpacking")
	root := filepath.Join(staging, "new")
	if err := extract(archive, root); err != nil {
		return err
	}
	_ = os.Remove(archive)

	say("Installing")
	if pl.bundle != "" {
		return replaceBundle(root, pl.bundle)
	}
	return replaceExecutable(root, pl.exe, u.GOOS)
}

// replaceBundle puts the app bundle found in dir in the place of the old one.
func replaceBundle(dir, bundle string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var fresh string
	for _, e := range entries {
		if e.IsDir() && strings.HasSuffix(e.Name(), ".app") {
			fresh = filepath.Join(dir, e.Name())
			break
		}
	}
	if fresh == "" {
		return fmt.Errorf("the download holds no app")
	}
	old := bundle + ".old"
	_ = os.RemoveAll(old)
	if err := os.Rename(bundle, old); err != nil {
		return fmt.Errorf("cannot replace %s: %w", bundle, err)
	}
	if err := os.Rename(fresh, bundle); err != nil {
		_ = os.Rename(old, bundle) // put the old one back
		return fmt.Errorf("cannot install into %s: %w", bundle, err)
	}
	_ = os.RemoveAll(old)
	return nil
}

// replaceExecutable puts the program file found in dir in the place of the running one.
func replaceExecutable(dir, exe, goos string) error {
	var fresh string
	want := strings.ToLower(filepath.Base(exe))
	_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || fresh != "" {
			return nil
		}
		n := strings.ToLower(d.Name())
		if n == want || n == "wordprojector" || n == "wordprojector.exe" {
			fresh = p
		}
		return nil
	})
	if fresh == "" {
		return fmt.Errorf("the download holds no program")
	}
	if goos != "windows" {
		if err := os.Chmod(fresh, 0o755); err != nil {
			return err
		}
		// next to the old one, so it can be renamed over it; a running program may be replaced like that
		next := exe + ".new"
		if err := copyFile(fresh, next, 0o755); err != nil {
			return err
		}
		if err := os.Rename(next, exe); err != nil {
			_ = os.Remove(next)
			return fmt.Errorf("cannot replace %s: %w", exe, err)
		}
		return nil
	}
	// a running program cannot be overwritten on Windows, but it can be renamed
	old := exe + ".old"
	_ = os.Remove(old)
	if err := os.Rename(exe, old); err != nil {
		return fmt.Errorf("cannot replace %s: %w", exe, err)
	}
	if err := copyFile(fresh, exe, 0o755); err != nil {
		_ = os.Remove(exe)
		_ = os.Rename(old, exe)
		return err
	}
	return nil
}

func copyFile(from, to string, mode os.FileMode) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(to, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// Cleanup removes what an earlier update left behind (the old copy of the program, unfinished downloads). Call it
// when the program starts.
func (u *Updater) Cleanup() {
	pl := u.place()
	parent := filepath.Dir(pl.exe)
	if pl.bundle != "" {
		parent = filepath.Dir(pl.bundle)
		_ = os.RemoveAll(pl.bundle + ".old")
	}
	_ = os.Remove(pl.exe + ".old")
	_ = os.Remove(pl.exe + ".new")
	if entries, err := os.ReadDir(parent); err == nil {
		for _, e := range entries {
			if e.IsDir() && strings.HasPrefix(e.Name(), stagingPrefix) {
				_ = os.RemoveAll(filepath.Join(parent, e.Name()))
			}
		}
	}
}

// ---- unpacking --------------------------------------------------------------------------------------------------

// inside reports whether the path (taken from an archive) stays under dir.
func inside(dir, path string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func extract(archive, dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if strings.HasSuffix(archive, ".zip") {
		return unzip(archive, dir)
	}
	if strings.HasSuffix(archive, ".tar.gz") {
		return untar(archive, dir)
	}
	return fmt.Errorf("unknown kind of download: %s", filepath.Base(archive))
}

func unzip(archive, dir string) error {
	zr, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer zr.Close()
	for _, f := range zr.File {
		target := filepath.Join(dir, f.Name)
		if !inside(dir, target) {
			return fmt.Errorf("the download holds a file outside its folder: %s", f.Name)
		}
		mode := f.Mode()
		switch {
		case f.FileInfo().IsDir():
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case mode&os.ModeSymlink != 0:
			rc, err := f.Open()
			if err != nil {
				return err
			}
			b, err := io.ReadAll(io.LimitReader(rc, 4096))
			rc.Close()
			if err != nil {
				return err
			}
			if err := symlink(dir, target, string(b)); err != nil {
				return err
			}
		default:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			rc, err := f.Open()
			if err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode.Perm()|0o600)
			if err != nil {
				rc.Close()
				return err
			}
			_, err = io.Copy(out, rc)
			rc.Close()
			if cerr := out.Close(); err == nil {
				err = cerr
			}
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func untar(archive, dir string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target := filepath.Join(dir, h.Name)
		if !inside(dir, target) {
			return fmt.Errorf("the download holds a file outside its folder: %s", h.Name)
		}
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if err := symlink(dir, target, h.Linkname); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(h.Mode).Perm()|0o600)
			if err != nil {
				return err
			}
			_, err = io.Copy(out, tr)
			if cerr := out.Close(); err == nil {
				err = cerr
			}
			if err != nil {
				return err
			}
		}
	}
}

// symlink makes a link from an archive, only when it points to something under dir.
func symlink(dir, target, to string) error {
	resolved := to
	if !filepath.IsAbs(to) {
		resolved = filepath.Join(filepath.Dir(target), to)
	}
	if !inside(dir, resolved) {
		return fmt.Errorf("the download holds a link that leaves its folder: %s", to)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	return os.Symlink(to, target)
}

// ---- restart ----------------------------------------------------------------------------------------------------

// Restart starts the program again once this one has quit (quit is called to make it quit) and the new files are what
// starts. args are the arguments of the running program.
func (u *Updater) Restart(args []string, quit func()) error {
	pl := u.place()
	var cmd *exec.Cmd
	switch {
	case pl.bundle != "":
		cmd = exec.Command("sh", "-c", `sleep 1.5; exec open -n "$0"`, pl.bundle)
	case u.GOOS == "windows":
		cmd = exec.Command("cmd", "/C", "ping -n 3 127.0.0.1 >nul & start \"\" \""+pl.exe+"\"")
	default:
		cmd = exec.Command("sh", append([]string{"-c", `sleep 1.5; exec "$0" "$@"`, pl.exe}, args...)...)
	}
	detach(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	quit()
	return nil
}
