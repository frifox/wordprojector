package server

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"wordprojector/internal/appdata"
	"wordprojector/internal/desktop"
	"wordprojector/internal/modules"
	"wordprojector/internal/slides"
)

type desktopFilter = desktop.Filter

func hasSuffixFold(s, suffix string) bool { return strings.HasSuffix(strings.ToLower(s), suffix) }

func urlQuery(s string) string { return url.QueryEscape(s) }

// exportPath returns a path in the Exports folder for browser mode exports.
func (s *Server) exportPath(name string) string {
	return filepath.Join(s.exportsDir(), name)
}

func safeName(s string) string {
	s = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`<>:"/\|?*`, r) {
			return '_'
		}
		return r
	}, s)
	s = strings.TrimSpace(s)
	if s == "" {
		s = "export"
	}
	return s
}

func (s *Server) manageRoutes() {
	m := s.mux
	m.HandleFunc("POST /api/dialog/open", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Title    string           `json:"title"`
			Filters  []desktop.Filter `json:"filters"`
			Multiple bool             `json:"multiple"`
		}
		_ = readJSON(r, &in)
		if !s.Desktop.Available() {
			writeErr(w, 501, desktop.ErrNoDesktop)
			return
		}
		paths, err := s.Desktop.OpenFiles(in.Title, in.Filters, in.Multiple)
		if err != nil {
			writeJSON(w, map[string]any{"paths": []string{}, "cancelled": true})
			return
		}
		if paths == nil {
			paths = []string{}
		}
		writeJSON(w, map[string]any{"paths": paths, "cancelled": len(paths) == 0})
	})
	m.HandleFunc("POST /api/dialog/folder", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Title string `json:"title"`
		}
		_ = readJSON(r, &in)
		if !s.Desktop.Available() {
			writeErr(w, 501, desktop.ErrNoDesktop)
			return
		}
		p, err := s.Desktop.OpenFolder(in.Title)
		if err != nil || p == "" {
			writeJSON(w, map[string]any{"path": "", "cancelled": true})
			return
		}
		writeJSON(w, map[string]any{"path": p, "name": filepath.Base(p)})
	})
	m.HandleFunc("POST /api/dialog/save", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Title    string           `json:"title"`
			Filename string           `json:"filename"`
			Filters  []desktop.Filter `json:"filters"`
		}
		_ = readJSON(r, &in)
		if !s.Desktop.Available() {
			writeErr(w, 501, desktop.ErrNoDesktop)
			return
		}
		p, err := s.Desktop.SaveFile(in.Title, in.Filename, in.Filters)
		if err != nil || p == "" {
			writeJSON(w, map[string]any{"path": "", "cancelled": true})
			return
		}
		writeJSON(w, map[string]any{"path": p})
	})

	// import
	importHandler := func(kind string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			var in struct {
				Paths []string `json:"paths"`
			}
			_ = readJSON(r, &in)
			if len(in.Paths) == 0 {
				writeErr(w, 400, fmt.Errorf("no file given"))
				return
			}
			if !s.jobs.start() {
				writeErr(w, 409, fmt.Errorf("another import is running"))
				return
			}
			go func() {
				defer s.jobs.done()
				job := "import-" + kind
				var lastErr error
				for _, p := range in.Paths {
					s.progressMsg(job, "Importing: "+p)
					var err error
					switch kind {
					case "songbook":
						_, err = s.Importer.ImportSongbook(p, s.progress(job))
					case "bible":
						_, err = s.Importer.ImportBible(p, s.progress(job))
					case "theme":
						_, err = s.Importer.ImportTheme(p, s.progress(job))
					}
					if err != nil {
						lastErr = err
						s.progressMsg(job, "Error: "+err.Error())
					}
				}
				s.afterImport(kind)
				s.progressDone(job, lastErr)
			}()
			writeJSON(w, map[string]any{"started": true, "job": "import-" + kind})
		}
	}
	m.HandleFunc("POST /api/import/songbook", importHandler("songbook"))
	m.HandleFunc("POST /api/import/bible", importHandler("bible"))
	m.HandleFunc("POST /api/import/theme", importHandler("theme"))

	// export
	m.HandleFunc("POST /api/export/{kind}", func(w http.ResponseWriter, r *http.Request) {
		kind := r.PathValue("kind")
		var in struct {
			ID   int64   `json:"id"`
			IDs  []int64 `json:"ids"`
			Path string  `json:"path"`
			Name string  `json:"name"`
		}
		_ = readJSON(r, &in)
		ext := map[string]string{"songbook": ".sps", "bible": ".spb", "theme": ".spt"}[kind]
		if ext == "" {
			writeErr(w, 400, fmt.Errorf("unknown export kind"))
			return
		}
		path := in.Path
		if path == "" {
			def := safeName(in.Name) + ext
			if s.Desktop.Available() {
				p, err := s.Desktop.SaveFile("Export as:", def, []desktop.Filter{{Name: "SoftProjector file (*" + ext + ")", Pattern: "*" + ext}})
				if err != nil || p == "" {
					writeJSON(w, map[string]any{"cancelled": true})
					return
				}
				path = p
			} else {
				path = s.exportPath(def)
			}
		}
		if !hasSuffixFold(path, ext) {
			path += ext
		}
		var err error
		var title string
		switch kind {
		case "songbook":
			title, err = s.Importer.ExportSongbook(path, in.ID, nil)
		case "bible":
			title, err = s.Importer.ExportBible(path, in.ID, nil)
		case "theme":
			err = s.Importer.ExportTheme(path, in.IDs, nil)
		}
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		resp := map[string]any{"ok": true, "path": path, "title": title}
		if !s.Desktop.Available() {
			resp["download"] = "/api/download?path=" + url.QueryEscape(path)
		}
		writeJSON(w, resp)
	})

	// a theme as a zip file (theme.json and backgrounds/): see internal/theme/pack.go
	m.HandleFunc("POST /api/theme-pack/export", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			ID   int64  `json:"id"`
			Path string `json:"path"`
		}
		_ = readJSON(r, &in)
		data, name, err := s.Engine.Themes.Export(in.ID)
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		path := in.Path
		if path == "" {
			def := safeName(name) + ".zip"
			if s.Desktop.Available() {
				p, err := s.Desktop.SaveFile("Export theme as:", def, []desktop.Filter{{Name: "Theme file (*.zip)", Pattern: "*.zip"}})
				if err != nil || p == "" {
					writeJSON(w, map[string]any{"cancelled": true})
					return
				}
				path = p
			} else {
				path = s.exportPath(def)
			}
		}
		if !hasSuffixFold(path, ".zip") {
			path += ".zip"
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err == nil {
			err = os.WriteFile(path, data, 0o644)
		} else {
			writeErr(w, 500, err)
			return
		}
		resp := map[string]any{"ok": true, "path": path, "name": name}
		if !s.Desktop.Available() {
			resp["download"] = "/api/download?path=" + url.QueryEscape(path)
		}
		writeJSON(w, resp)
	})
	m.HandleFunc("POST /api/theme-pack/import", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Path string `json:"path"`
		}
		_ = readJSON(r, &in)
		if in.Path == "" {
			writeErr(w, 400, fmt.Errorf("no file given"))
			return
		}
		data, err := os.ReadFile(in.Path)
		if err != nil {
			writeErr(w, 400, err)
			return
		}
		id, name, err := s.Engine.Themes.Import(data, func(b []byte) ([]byte, error) {
			img, err := slides.DecodeBytes(b)
			if err != nil {
				return nil, err
			}
			return slides.EncodeJPEG(slides.Scale(img, 1920, 1920)), nil
		})
		if err != nil {
			writeErr(w, 400, err)
			return
		}
		s.notify("themes")
		writeJSON(w, map[string]any{"id": id, "name": name})
	})

	// module download
	m.HandleFunc("GET /api/modules/list", func(w http.ResponseWriter, r *http.Request) {
		u, err := modules.ListURL(r.URL.Query().Get("kind"), r.URL.Query().Get("source"), r.URL.Query().Get("url"))
		if err != nil {
			writeErr(w, 400, err)
			return
		}
		list, err := modules.FetchList(r.Context(), u)
		if err != nil {
			writeErr(w, 502, err)
			return
		}
		writeJSON(w, list)
	})
	m.HandleFunc("POST /api/modules/download", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Kind    string           `json:"kind"`
			Modules []modules.Module `json:"modules"`
		}
		if err := readJSON(r, &in); err != nil {
			writeErr(w, 400, err)
			return
		}
		sub := map[string]string{"bible": "BibleModules", "songbook": "SongModules", "theme": "ThemeModules"}[in.Kind]
		if sub == "" {
			writeErr(w, 400, fmt.Errorf("unknown module kind"))
			return
		}
		if !s.jobs.start() {
			writeErr(w, 409, fmt.Errorf("another download is running"))
			return
		}
		dir, _ := appdata.Sub(sub)
		go func() {
			defer s.jobs.done()
			job := "download-" + in.Kind
			var lastErr error
			total := len(in.Modules) * 2
			step := 0
			for _, mod := range in.Modules {
				dest := modules.SaveFileName(dir, mod.Link)
				s.progressMsg(job, fmt.Sprintf("\nDownloading: %s\nFrom: %s", mod.Name, mod.Link))
				err := modules.Download(context.Background(), mod.Link, dest, mod.Size, s.progress(job))
				step++
				s.hub.broadcast(map[string]any{"type": "progress", "job": job, "total": step, "totalMax": total})
				if err != nil {
					lastErr = err
					s.progressMsg(job, "Download Error: "+err.Error())
					os.Remove(dest)
					step++
					continue
				}
				s.progressMsg(job, "Saved to: "+dest)
				s.progressMsg(job, "\nImporting: "+dest)
				switch in.Kind {
				case "bible":
					_, err = s.Importer.ImportBible(dest, s.progress(job))
				case "songbook":
					_, err = s.Importer.ImportSongbook(dest, s.progress(job))
				case "theme":
					_, err = s.Importer.ImportTheme(dest, s.progress(job))
				}
				step++
				s.hub.broadcast(map[string]any{"type": "progress", "job": job, "total": step, "totalMax": total})
				if err != nil {
					lastErr = err
					s.progressMsg(job, "Import Error: "+err.Error())
				}
			}
			s.afterImport(in.Kind)
			s.progressDone(job, lastErr)
		}()
		writeJSON(w, map[string]any{"started": true, "job": "download-" + in.Kind})
	})
}

func (s *Server) afterImport(kind string) {
	switch kind {
	case "bible":
		s.Engine.Bibles.Invalidate()
		s.Engine.Refresh()
		s.notify("bibles")
		s.notify("settings")
	case "songbook":
		s.notify("songbooks")
		s.notify("songs")
	case "theme":
		s.reloadTheme()
		s.notify("themes")
	}
}
