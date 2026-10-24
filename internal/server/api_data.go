package server

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"io/fs"
	"strings"
	"time"

	"wordprojector/internal/announce"
	"wordprojector/internal/bible"
	"wordprojector/internal/desktop"
	"wordprojector/internal/fonts"
	"wordprojector/internal/media"
	"wordprojector/internal/modules"
	"wordprojector/internal/settings"
	"wordprojector/internal/slides"
	"wordprojector/internal/songs"
	"wordprojector/internal/theme"
)

func (s *Server) routes() {
	m := s.mux
	m.HandleFunc("GET /api/ws", s.handleWS)
	m.HandleFunc("GET /api/bootstrap", s.bootstrap)
	// a second instance asks this to learn that the port is held by a Word Projector it can connect to
	m.HandleFunc("GET /api/ping", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"app": "WordProjector", "version": Version})
	})
	m.HandleFunc("GET /api/remote/info", s.remoteInfo)
	m.HandleFunc("GET /api/state", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, s.Engine.State()) })
	m.HandleFunc("GET /api/fonts", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, fonts.Families()) })
	m.HandleFunc("GET /api/screens", func(w http.ResponseWriter, r *http.Request) {
		sc := s.Desktop.Screens()
		if sc == nil {
			sc = []desktop.Screen{}
		}
		writeJSON(w, sc)
	})
	m.HandleFunc("GET /api/categories", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, songs.Categories) })
	m.HandleFunc("POST /api/app/quit", func(w http.ResponseWriter, r *http.Request) {
		ok(w)
		go func() { time.Sleep(200 * time.Millisecond); s.Desktop.Quit() }()
	})

	// songbooks & songs
	m.HandleFunc("GET /api/songbooks", func(w http.ResponseWriter, r *http.Request) { s.list(w, s.Engine.Songs.Songbooks) })
	m.HandleFunc("POST /api/songbooks", func(w http.ResponseWriter, r *http.Request) {
		var sb songs.Songbook
		if err := readJSON(r, &sb); err != nil {
			writeErr(w, 400, err)
			return
		}
		id, err := s.Engine.Songs.AddSongbook(sb.Name, sb.Info)
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		s.notify("songbooks")
		writeJSON(w, map[string]int64{"id": id})
	})
	m.HandleFunc("PUT /api/songbooks/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, _ := pathID(r, "id")
		var sb songs.Songbook
		if err := readJSON(r, &sb); err != nil {
			writeErr(w, 400, err)
			return
		}
		if err := s.Engine.Songs.UpdateSongbook(id, sb.Name, sb.Info); err != nil {
			writeErr(w, 500, err)
			return
		}
		s.notify("songbooks")
		ok(w)
	})
	m.HandleFunc("DELETE /api/songbooks/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, _ := pathID(r, "id")
		if err := s.Engine.Songs.DeleteSongbook(id); err != nil {
			writeErr(w, 500, err)
			return
		}
		s.notify("songbooks")
		ok(w)
	})
	m.HandleFunc("GET /api/songs", func(w http.ResponseWriter, r *http.Request) { s.list(w, s.Engine.Songs.Songs) })
	m.HandleFunc("GET /api/songs/counts", func(w http.ResponseWriter, r *http.Request) { s.list(w, s.Engine.Songs.Counts) })
	m.HandleFunc("POST /api/songs/counts/reset", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			ID int64 `json:"id"`
		}
		_ = readJSON(r, &in)
		var err error
		if in.ID > 0 {
			err = s.Engine.Songs.ResetCount(in.ID)
		} else {
			err = s.Engine.Songs.ResetCounts()
		}
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		ok(w)
	})
	m.HandleFunc("GET /api/songs/next-number", func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.ParseInt(r.URL.Query().Get("songbook"), 10, 64)
		n, err := s.Engine.Songs.NextNumber(id)
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, map[string]int{"number": n})
	})
	m.HandleFunc("POST /api/songs/search", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Text string `json:"text"`
			Type int    `json:"type"`
		}
		if err := readJSON(r, &in); err != nil {
			writeErr(w, 400, err)
			return
		}
		res, err := s.Engine.Songs.Search(in.Text, songs.SearchType(in.Type))
		if err != nil {
			writeErr(w, 400, err)
			return
		}
		writeJSON(w, res)
	})
	m.HandleFunc("POST /api/songs/match", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			SongbookName string `json:"songbookName"`
			Number       int    `json:"number"`
			Title        string `json:"title"`
		}
		if err := readJSON(r, &in); err != nil {
			writeErr(w, 400, err)
			return
		}
		id, found := s.Engine.Songs.FindMatching(in.SongbookName, in.Number, in.Title)
		writeJSON(w, map[string]any{"id": id, "found": found})
	})
	m.HandleFunc("GET /api/songs/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, _ := pathID(r, "id")
		sg, err := s.Engine.Songs.Song(id)
		if err != nil {
			writeErr(w, 404, err)
			return
		}
		writeJSON(w, sg)
	})
	m.HandleFunc("GET /api/songs/{id}/stanzas", func(w http.ResponseWriter, r *http.Request) {
		id, _ := pathID(r, "id")
		sg, err := s.Engine.Songs.Song(id)
		if err != nil {
			writeErr(w, 404, err)
			return
		}
		writeJSON(w, songs.StanzaList(sg.Text))
	})
	m.HandleFunc("POST /api/songs/stanzas", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Text string `json:"text"`
		}
		_ = readJSON(r, &in)
		list := songs.StanzaList(in.Text)
		if list == nil {
			list = []string{} // an empty song is an empty list, not null
		}
		writeJSON(w, list)
	})
	m.HandleFunc("POST /api/songs", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Song       songs.Song `json:"song"`
			Background string     `json:"background"` // path of a new background image
		}
		if err := readJSON(r, &in); err != nil {
			writeErr(w, 400, err)
			return
		}
		bg, err := loadBackground(in.Background)
		if err != nil {
			writeErr(w, 400, err)
			return
		}
		id, err := s.Engine.Songs.Insert(in.Song, bg)
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		s.notify("songs")
		writeJSON(w, map[string]int64{"id": id})
	})
	m.HandleFunc("PUT /api/songs/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, _ := pathID(r, "id")
		var in struct {
			Song       songs.Song `json:"song"`
			Background string     `json:"background"`
			ClearBackground bool  `json:"clearBackground"`
		}
		if err := readJSON(r, &in); err != nil {
			writeErr(w, 400, err)
			return
		}
		in.Song.ID = id
		keep := in.Background == "" && !in.ClearBackground
		bg, err := loadBackground(in.Background)
		if err != nil {
			writeErr(w, 400, err)
			return
		}
		if err := s.Engine.Songs.Update(in.Song, bg, keep); err != nil {
			writeErr(w, 500, err)
			return
		}
		s.notify("songs")
		s.Engine.BumpBackgrounds()
		ok(w)
	})
	m.HandleFunc("DELETE /api/songs/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, _ := pathID(r, "id")
		if err := s.Engine.Songs.Delete(id); err != nil {
			writeErr(w, 500, err)
			return
		}
		s.notify("songs")
		ok(w)
	})
	m.HandleFunc("GET /api/bg/song/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, _ := pathID(r, "id")
		b, err := s.Engine.Songs.Background(id)
		if err != nil || len(b) == 0 {
			http.NotFound(w, r)
			return
		}
		serveImage(w, r, b)
	})

	// bibles
	m.HandleFunc("GET /api/bibles", func(w http.ResponseWriter, r *http.Request) { s.list(w, s.Engine.Bibles.List) })
	m.HandleFunc("PUT /api/bibles/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, _ := pathID(r, "id")
		var in bible.Info
		if err := readJSON(r, &in); err != nil {
			writeErr(w, 400, err)
			return
		}
		in.ID = id
		if err := s.Engine.Bibles.UpdateInfo(in); err != nil {
			writeErr(w, 500, err)
			return
		}
		s.notify("bibles")
		ok(w)
	})
	m.HandleFunc("DELETE /api/bibles/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, _ := pathID(r, "id")
		if err := s.Engine.Bibles.Delete(id); err != nil {
			writeErr(w, 500, err)
			return
		}
		s.Engine.Refresh()
		s.notify("bibles")
		s.notify("settings")
		ok(w)
	})
	m.HandleFunc("GET /api/bibles/{id}/books", func(w http.ResponseWriter, r *http.Request) {
		id, _ := pathID(r, "id")
		books, err := s.Engine.Bibles.Books(id)
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, books)
	})
	m.HandleFunc("GET /api/bibles/{id}/chapter", func(w http.ResponseWriter, r *http.Request) {
		id, _ := pathID(r, "id")
		book, _ := strconv.Atoi(r.URL.Query().Get("book"))
		chapter, _ := strconv.Atoi(r.URL.Query().Get("chapter"))
		verses, err := s.Engine.Bibles.Chapter(id, book, chapter)
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, verses)
	})
	m.HandleFunc("GET /api/bibles/{id}/ref", func(w http.ResponseWriter, r *http.Request) {
		id, _ := pathID(r, "id")
		book, chapter, first, last, name, err := s.Engine.Bibles.Ref(id, r.URL.Query().Get("ids"))
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, map[string]any{"bookId": book, "bookName": name, "chapter": chapter, "first": first, "last": last})
	})
	m.HandleFunc("POST /api/bibles/{id}/search", func(w http.ResponseWriter, r *http.Request) {
		id, _ := pathID(r, "id")
		var in struct {
			Text     string `json:"text"`
			Exact    bool   `json:"exact"`
			FullWord bool   `json:"fullWord"`
			Range    int    `json:"range"`
			Book     int    `json:"book"`
			Chapter  int    `json:"chapter"`
		}
		if err := readJSON(r, &in); err != nil {
			writeErr(w, 400, err)
			return
		}
		book, chapter := 0, 0
		if in.Range >= 1 {
			book = in.Book
		}
		if in.Range >= 2 {
			chapter = in.Chapter
		}
		hits, hl, err := s.Engine.Bibles.Search(id, in.Text, in.Exact, in.FullWord, book, chapter)
		if err != nil {
			writeErr(w, 400, err)
			return
		}
		writeJSON(w, map[string]any{"hits": hits, "highlight": hl})
	})
	m.HandleFunc("GET /api/bibles/{id}/chapter-text", func(w http.ResponseWriter, r *http.Request) {
		// for printing: book name + verses
		id, _ := pathID(r, "id")
		book, _ := strconv.Atoi(r.URL.Query().Get("book"))
		chapter, _ := strconv.Atoi(r.URL.Query().Get("chapter"))
		verses, err := s.Engine.Bibles.Chapter(id, book, chapter)
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		info, _ := s.Engine.Bibles.Get(id)
		books, _ := s.Engine.Bibles.Books(id)
		name := ""
		for _, b := range books {
			if b.ID == book {
				name = b.Name
			}
		}
		writeJSON(w, map[string]any{"bible": info.Name, "book": name, "chapter": chapter, "verses": verses})
	})

	// announcements
	m.HandleFunc("GET /api/announcements", func(w http.ResponseWriter, r *http.Request) { s.list(w, s.Engine.Ann.List) })
	m.HandleFunc("POST /api/announcements", func(w http.ResponseWriter, r *http.Request) {
		var a announce.Announcement
		if err := readJSON(r, &a); err != nil {
			writeErr(w, 400, err)
			return
		}
		id, err := s.Engine.Ann.Insert(a)
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		s.notify("announcements")
		writeJSON(w, map[string]int64{"id": id})
	})
	m.HandleFunc("PUT /api/announcements/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, _ := pathID(r, "id")
		var a announce.Announcement
		if err := readJSON(r, &a); err != nil {
			writeErr(w, 400, err)
			return
		}
		a.ID = id
		if err := s.Engine.Ann.Update(a); err != nil {
			writeErr(w, 500, err)
			return
		}
		s.notify("announcements")
		ok(w)
	})
	m.HandleFunc("DELETE /api/announcements/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, _ := pathID(r, "id")
		if err := s.Engine.Ann.Delete(id); err != nil {
			writeErr(w, 500, err)
			return
		}
		s.notify("announcements")
		ok(w)
	})
	m.HandleFunc("POST /api/announcements/slides", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Text string `json:"text"`
		}
		_ = readJSON(r, &in)
		writeJSON(w, announce.SlideList(in.Text))
	})

	// slide shows
	m.HandleFunc("GET /api/slideshows", func(w http.ResponseWriter, r *http.Request) { s.list(w, s.Engine.Slides.List) })
	m.HandleFunc("GET /api/slideshows/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, _ := pathID(r, "id")
		ss, err := s.Engine.Slides.Get(id)
		if err != nil {
			writeErr(w, 404, err)
			return
		}
		writeJSON(w, ss)
	})
	m.HandleFunc("POST /api/slideshows", func(w http.ResponseWriter, r *http.Request) {
		var ss slides.ShowWithSlides
		if err := readJSON(r, &ss); err != nil {
			writeErr(w, 400, err)
			return
		}
		id, err := s.Engine.Slides.Save(ss, s.Engine.TempImages())
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		s.notify("slideshows")
		writeJSON(w, map[string]int64{"id": id})
	})
	m.HandleFunc("DELETE /api/slideshows/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, _ := pathID(r, "id")
		if err := s.Engine.Slides.Delete(id); err != nil {
			writeErr(w, 500, err)
			return
		}
		s.notify("slideshows")
		ok(w)
	})
	m.HandleFunc("POST /api/slideshows/images", s.addImages)
	m.HandleFunc("POST /api/slideshows/folder-images", s.addFolderImages)
	m.HandleFunc("GET /api/slides/{id}/image", func(w http.ResponseWriter, r *http.Request) {
		id, _ := pathID(r, "id")
		b, err := s.Engine.Slides.Image(id, r.URL.Query().Get("size"))
		if err != nil || len(b) == 0 {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "private, max-age=3600")
		serveImage(w, r, b)
	})
	m.HandleFunc("GET /api/blob/{key}", func(w http.ResponseWriter, r *http.Request) {
		b, found := s.Engine.TempImage(r.PathValue("key"), r.URL.Query().Get("size"))
		if !found || len(b) == 0 {
			http.NotFound(w, r)
			return
		}
		serveImage(w, r, b)
	})

	// media
	m.HandleFunc("GET /api/media", func(w http.ResponseWriter, r *http.Request) { s.list(w, s.Engine.Media.List) })
	m.HandleFunc("POST /api/media", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Paths []string `json:"paths"`
		}
		if err := readJSON(r, &in); err != nil {
			writeErr(w, 400, err)
			return
		}
		var paths []string
		for _, p := range in.Paths {
			if media.IsSupported(p) {
				paths = append(paths, p)
			}
		}
		if err := s.Engine.Media.Add(paths); err != nil {
			writeErr(w, 500, err)
			return
		}
		s.notify("media")
		writeJSON(w, map[string]int{"added": len(paths)})
	})
	m.HandleFunc("DELETE /api/media", func(w http.ResponseWriter, r *http.Request) {
		if err := s.Engine.Media.Remove(r.URL.Query().Get("path")); err != nil {
			writeErr(w, 500, err)
			return
		}
		s.notify("media")
		ok(w)
	})
	m.HandleFunc("GET /api/media/file", func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Query().Get("path")
		if p == "" {
			http.NotFound(w, r)
			return
		}
		if _, err := os.Stat(p); err != nil {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, p)
	})
	m.HandleFunc("GET /api/file", func(w http.ResponseWriter, r *http.Request) {
		// serve an arbitrary local image/file (announcement backgrounds etc.)
		p := r.URL.Query().Get("path")
		if p == "" {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, p)
	})

	// themes & settings
	m.HandleFunc("GET /api/themes", func(w http.ResponseWriter, r *http.Request) { s.list(w, s.Engine.Themes.List) })
	m.HandleFunc("GET /api/themes/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, _ := pathID(r, "id")
		t, err := s.Engine.Themes.Load(id)
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, t)
	})
	m.HandleFunc("GET /api/themes/{id}/preview", func(w http.ResponseWriter, r *http.Request) {
		id, _ := pathID(r, "id")
		sl, err := s.Engine.PreviewTheme(id)
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, sl)
	})
	m.HandleFunc("POST /api/themes", func(w http.ResponseWriter, r *http.Request) {
		var in theme.Info
		if err := readJSON(r, &in); err != nil {
			writeErr(w, 400, err)
			return
		}
		in.Name = strings.TrimSpace(in.Name)
		if in.Name == "" || theme.IsBuiltinName(in.Name) {
			writeErr(w, 400, fmt.Errorf("please choose another name"))
			return
		}
		if list, err := s.Engine.Themes.List(); err == nil {
			for _, th := range list {
				if strings.EqualFold(th.Name, in.Name) {
					writeErr(w, 400, fmt.Errorf("a theme named \"%s\" exists already", th.Name))
					return
				}
			}
		}
		t := theme.Default()
		t.Name = in.Name
		id, err := s.Engine.Themes.Insert(t)
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		s.notify("themes")
		writeJSON(w, map[string]int64{"id": id})
	})
	m.HandleFunc("POST /api/themes/{id}/copy", func(w http.ResponseWriter, r *http.Request) {
		id, _ := pathID(r, "id")
		var in struct {
			Name string `json:"name"`
		}
		if err := readJSON(r, &in); err != nil {
			writeErr(w, 400, err)
			return
		}
		name := strings.TrimSpace(in.Name)
		if name == "" || theme.IsBuiltinName(name) {
			writeErr(w, 400, fmt.Errorf("please choose another name"))
			return
		}
		if list, err := s.Engine.Themes.List(); err == nil {
			for _, th := range list {
				if strings.EqualFold(th.Name, name) {
					writeErr(w, 400, fmt.Errorf("a theme named \"%s\" exists already", th.Name))
					return
				}
			}
		}
		newID, err := s.Engine.Themes.Duplicate(id, name)
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		s.notify("themes")
		writeJSON(w, map[string]int64{"id": newID})
	})
	m.HandleFunc("PUT /api/themes/{id}/info", func(w http.ResponseWriter, r *http.Request) {
		id, _ := pathID(r, "id")
		var in theme.Info
		if err := readJSON(r, &in); err != nil {
			writeErr(w, 400, err)
			return
		}
		in.Name = strings.TrimSpace(in.Name)
		if in.Name == "" || theme.IsBuiltinName(in.Name) {
			writeErr(w, 400, fmt.Errorf("please choose another name"))
			return
		}
		if list, err := s.Engine.Themes.List(); err == nil {
			for _, th := range list {
				if th.ID != id && strings.EqualFold(th.Name, in.Name) {
					writeErr(w, 400, fmt.Errorf("a theme named \"%s\" exists already", th.Name))
					return
				}
			}
		}
		if err := s.Engine.Themes.Rename(id, in.Name); err != nil {
			writeErr(w, 500, err)
			return
		}
		s.reloadTheme()
		s.notify("themes")
		ok(w)
	})
	m.HandleFunc("PUT /api/themes/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, _ := pathID(r, "id")
		var t theme.Theme
		if err := readJSON(r, &t); err != nil {
			writeErr(w, 400, err)
			return
		}
		t.ID = id
		if err := s.Engine.Themes.Update(t); err != nil {
			writeErr(w, 500, err)
			return
		}
		s.reloadTheme()
		s.notify("themes")
		ok(w)
	})
	m.HandleFunc("DELETE /api/themes/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, _ := pathID(r, "id")
		if th, err := s.Engine.Themes.Load(id); err == nil && th.Builtin {
			writeErr(w, 400, fmt.Errorf("%q is a built-in theme and can't be deleted", th.Name))
			return
		}
		if err := s.Engine.Themes.Delete(id); err != nil {
			writeErr(w, 500, err)
			return
		}
		s.reloadTheme()
		s.notify("themes")
		s.notify("settings")
		ok(w)
	})
	// the list of background images of a theme (per slide kind and display)
	imageList := func(w http.ResponseWriter, id int64, kind string, disp int) {
		list, err := s.Engine.Themes.Images(id, kind, disp)
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, list)
	}
	m.HandleFunc("GET /api/themes/{id}/images/{kind}/{disp}", func(w http.ResponseWriter, r *http.Request) {
		id, _ := pathID(r, "id")
		disp, _ := strconv.Atoi(r.PathValue("disp"))
		imageList(w, id, r.PathValue("kind"), disp)
	})
	m.HandleFunc("POST /api/themes/{id}/images/{kind}/{disp}", func(w http.ResponseWriter, r *http.Request) {
		id, _ := pathID(r, "id")
		disp, _ := strconv.Atoi(r.PathValue("disp"))
		var in struct {
			Paths []string `json:"paths"`
		}
		_ = readJSON(r, &in)
		for _, p := range in.Paths {
			b, err := loadBackground(p)
			if err != nil {
				writeErr(w, 400, err)
				return
			}
			if _, err := s.Engine.Themes.AddImage(id, r.PathValue("kind"), disp, filepath.Base(p), b); err != nil {
				writeErr(w, 500, err)
				return
			}
		}
		s.reloadTheme()
		imageList(w, id, r.PathValue("kind"), disp)
	})
	m.HandleFunc("POST /api/themes/{id}/images/{kind}/{disp}/{img}/select", func(w http.ResponseWriter, r *http.Request) {
		id, _ := pathID(r, "id")
		disp, _ := strconv.Atoi(r.PathValue("disp"))
		img, _ := pathID(r, "img")
		if err := s.Engine.Themes.SelectImage(id, r.PathValue("kind"), disp, img); err != nil {
			writeErr(w, 500, err)
			return
		}
		s.reloadTheme()
		imageList(w, id, r.PathValue("kind"), disp)
	})
	m.HandleFunc("DELETE /api/themes/{id}/images/{kind}/{disp}/{img}", func(w http.ResponseWriter, r *http.Request) {
		id, _ := pathID(r, "id")
		disp, _ := strconv.Atoi(r.PathValue("disp"))
		img, _ := pathID(r, "img")
		if err := s.Engine.Themes.RemoveImage(id, r.PathValue("kind"), disp, img); err != nil {
			writeErr(w, 500, err)
			return
		}
		s.reloadTheme()
		imageList(w, id, r.PathValue("kind"), disp)
	})
	m.HandleFunc("PUT /api/themes/{id}/images/{kind}/{disp}/{img}", func(w http.ResponseWriter, r *http.Request) {
		id, _ := pathID(r, "id")
		disp, _ := strconv.Atoi(r.PathValue("disp"))
		img, _ := pathID(r, "img")
		var in struct {
			Name string `json:"name"`
		}
		_ = readJSON(r, &in)
		if err := s.Engine.Themes.RenameImage(id, r.PathValue("kind"), disp, img, in.Name); err != nil {
			writeErr(w, 400, err)
			return
		}
		s.reloadTheme()
		imageList(w, id, r.PathValue("kind"), disp)
	})
	m.HandleFunc("GET /api/theme-images/{img}", func(w http.ResponseWriter, r *http.Request) {
		img, _ := pathID(r, "img")
		b, err := s.Engine.Themes.ImageData(img)
		if err != nil || len(b) == 0 {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "private, max-age=3600")
		serveImage(w, r, b)
	})
	m.HandleFunc("GET /api/bg/theme/{kind}/{disp}", func(w http.ResponseWriter, r *http.Request) {
		disp, _ := strconv.Atoi(r.PathValue("disp"))
		tid, _ := strconv.ParseInt(r.URL.Query().Get("theme"), 10, 64)
		if tid == 0 {
			tid = s.Engine.Theme.ID
		}
		b, _, err := s.Engine.Themes.Background(tid, r.PathValue("kind"), disp)
		if err != nil || len(b) == 0 {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "private, max-age=3600")
		serveImage(w, r, b)
	})
	m.HandleFunc("GET /api/settings", func(w http.ResponseWriter, r *http.Request) {
		s.Engine.Locked(func() { writeJSON(w, s.Engine.Settings) })
	})
	m.HandleFunc("PUT /api/settings", func(w http.ResponseWriter, r *http.Request) {
		var in settings.Settings
		if err := readJSON(r, &in); err != nil {
			writeErr(w, 400, err)
			return
		}
		var saveErr error
		s.Engine.Locked(func() {
			// the Settings window holds a copy from when it was opened: what other parts of the program save by themselves
			// (the layout, the language, the order of buttons, what Books last used) must not be put back to the old copy
			cur := s.Engine.Settings.App
			in.App.Window = cur.Window
			in.App.Splitters = cur.Splitters
			in.App.Language = cur.Language
			in.Main.UITranslation = s.Engine.Settings.Main.UITranslation
			in.App.SlideshowOrder = cur.SlideshowOrder
			in.App.BibleButtonOrder = cur.BibleButtonOrder
			in.App.ModuleSource = cur.ModuleSource
			in.App.ModuleURLs = cur.ModuleURLs
			s.Engine.Settings = in
			saveErr = settings.Save(s.Engine.DB, in)
		})
		if saveErr != nil {
			writeErr(w, 500, saveErr)
			return
		}
		s.reloadTheme()
		s.notify("settings")
		ok(w)
	})
	m.HandleFunc("PATCH /api/settings/app", func(w http.ResponseWriter, r *http.Request) {
		// lightweight UI prefs (splitters etc.) without a full round trip
		var in struct {
			Splitters map[string]float64 `json:"splitters"`
			Language  *string            `json:"language"`
			UITheme   *string            `json:"uiTheme"`
			LivePreview *bool            `json:"livePreview"`
			SlideshowOrder *[]int        `json:"slideshowOrder"`
			BibleButtonOrder *[]string   `json:"bibleButtonOrder"`
			ModuleSource *string         `json:"moduleSource"`
			ModuleURLs map[string]string `json:"moduleUrls"`
		}
		if err := readJSON(r, &in); err != nil {
			writeErr(w, 400, err)
			return
		}
		s.Engine.Locked(func() {
			for k, v := range in.Splitters {
				s.Engine.Settings.App.Splitters[k] = v
			}
			if in.Language != nil {
				s.Engine.Settings.App.Language = *in.Language
				s.Engine.Settings.Main.UITranslation = *in.Language
			}
			if in.UITheme != nil {
				s.Engine.Settings.App.UITheme = *in.UITheme
			}
			if in.LivePreview != nil {
				s.Engine.Settings.App.LivePreview = *in.LivePreview
			}
			if in.SlideshowOrder != nil {
				s.Engine.Settings.App.SlideshowOrder = *in.SlideshowOrder
			}
			if in.BibleButtonOrder != nil {
				s.Engine.Settings.App.BibleButtonOrder = *in.BibleButtonOrder
			}
			if in.ModuleSource != nil {
				s.Engine.Settings.App.ModuleSource = *in.ModuleSource
			}
			if len(in.ModuleURLs) > 0 {
				if s.Engine.Settings.App.ModuleURLs == nil {
					s.Engine.Settings.App.ModuleURLs = map[string]string{}
				}
				for k, v := range in.ModuleURLs {
					s.Engine.Settings.App.ModuleURLs[k] = v
				}
			}
			_ = settings.Save(s.Engine.DB, s.Engine.Settings)
		})
		ok(w)
	})

	m.HandleFunc("GET /api/download", s.download)
	m.HandleFunc("POST /api/upload", s.upload)

	s.liveRoutes()
	s.manageRoutes()
	m.Handle("/", s.serveStatic())
}

func (s *Server) list(w http.ResponseWriter, fn any) {
	var v any
	var err error
	switch f := fn.(type) {
	case func() ([]songs.Songbook, error):
		v, err = f()
	case func() ([]songs.Song, error):
		v, err = f()
	case func() ([]songs.CountRow, error):
		v, err = f()
	case func() ([]bible.Info, error):
		v, err = f()
	case func() ([]announce.Announcement, error):
		v, err = f()
	case func() ([]slides.Show, error):
		v, err = f()
	case func() ([]media.Item, error):
		v, err = f()
	case func() ([]theme.Info, error):
		v, err = f()
	default:
		err = errors.New("bad list function")
	}
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	writeJSON(w, v)
}

// reloadTheme reloads the current theme into the engine and recomputes slides.
func (s *Server) reloadTheme() {
	var t theme.Theme
	var err error
	s.Engine.Locked(func() {
		t, err = s.Engine.Themes.Load(s.Engine.Settings.General.CurrentThemeID)
		if err == nil {
			s.Engine.Theme = t
			if s.Engine.Settings.General.CurrentThemeID != t.ID {
				s.Engine.Settings.General.CurrentThemeID = t.ID
				_ = settings.Save(s.Engine.DB, s.Engine.Settings)
			}
		}
	})
	s.Engine.BumpBackgrounds()
}

func serveImage(w http.ResponseWriter, r *http.Request, b []byte) {
	ct := http.DetectContentType(b)
	w.Header().Set("Content-Type", ct)
	http.ServeContent(w, r, "image", time.Time{}, bytes.NewReader(b))
}

// loadBackground reads an image file and re-encodes it as JPEG (quality 90)
// the way the original stored backgrounds.
func loadBackground(path string) ([]byte, error) {
	if path == "" {
		return nil, nil
	}
	img, err := slides.Decode(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read image %s: %w", filepath.Base(path), err)
	}
	// Keep backgrounds at a sane size for the database.
	img = slides.Scale(img, 1920, 1920)
	return slides.EncodeJPEG(img), nil
}

func (s *Server) bootstrap(w http.ResponseWriter, r *http.Request) {
	var sets settings.Settings
	var th theme.Theme
	s.Engine.Locked(func() { sets = s.Engine.Settings; th = s.Engine.Theme })
	themes, _ := s.Engine.Themes.List()
	bibles, _ := s.Engine.Bibles.List()
	books, _ := s.Engine.Songs.Songbooks()
	sc := s.Desktop.Screens()
	if sc == nil {
		sc = nil
	}
	writeJSON(w, map[string]any{
		"version":  Version,
		"desktop":  s.Desktop.Available(),
		"dataDir":  s.DataDir,
		"settings": sets,
		"theme":    th,
		"themes":   themes,
		"bibles":   bibles,
		"songbooks": books,
		"screens":  sc,
		"state":    s.Engine.State(),
		"categories": songs.Categories,
		"operatorBible": s.Engine.OperatorBibleID(),
		"titles": map[string]any{"verse": verseTitles(), "refrain": refrainTitles(), "slide": slideTitles(), "announce": announce.TitleWords()},
	})
}

func verseTitles() []string   { v, _, _ := songs.TitlePrefixes(); return v }
func refrainTitles() []string { _, r, _ := songs.TitlePrefixes(); return r }
func slideTitles() []string   { _, _, s := songs.TitlePrefixes(); return s }

// addImages prepares images for a slide show from local paths and returns slides with temp keys.
func (s *Server) addImages(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Paths []string `json:"paths"`
		Link  bool     `json:"link"`
	}
	if err := readJSON(r, &in); err != nil {
		writeErr(w, 400, err)
		return
	}
	writeJSON(w, s.prepareImages(in.Paths, in.Link, s.progress(imagesJob)))
}

// imagesJob is the job whose progress is reported while the pictures of a folder are prepared.
const imagesJob = "folder-images"

var folderImageExt = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".bmp": true, ".webp": true, ".tif": true, ".tiff": true}

// addFolderImages prepares the images of a folder in name order, those of its subfolders too when asked.
func (s *Server) addFolderImages(w http.ResponseWriter, r *http.Request) {
	in := struct {
		Path      string `json:"path"`
		Recursive *bool  `json:"recursive"`
		Link      bool   `json:"link"` // the slides show the pictures where they lie, without a copy
	}{}
	if err := readJSON(r, &in); err != nil || in.Path == "" {
		writeErr(w, 400, fmt.Errorf("no folder given"))
		return
	}
	recursive := in.Recursive == nil || *in.Recursive // the way it always worked
	var paths []string
	err := filepath.WalkDir(in.Path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if p != in.Path && (!recursive || strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasPrefix(d.Name(), ".") && folderImageExt[strings.ToLower(filepath.Ext(p))] {
			paths = append(paths, p)
		}
		return nil
	})
	if err != nil {
		writeErr(w, 400, err)
		return
	}
	writeJSON(w, s.prepareImages(paths, in.Link, s.progress(imagesJob)))
}

// prepareImages makes slides of pictures. With link the slides will refer to the files instead of holding copies.
func (s *Server) prepareImages(paths []string, link bool, progress modules.Progress) []slides.Slide {
	var sets slides.Settings
	s.Engine.Locked(func() { sets = s.Engine.Settings.Slides })
	out := []slides.Slide{}
	for i, p := range paths {
		if progress != nil {
			progress(i, len(paths), filepath.Base(p))
		}
		img, err := slides.Decode(p)
		if err != nil {
			continue
		}
		prepared := slides.Prepare(img, sets)
		key := fmt.Sprintf("tmp%d_%d", time.Now().UnixNano(), i)
		s.Engine.AddTempImage(key, prepared)
		sl := slides.Slide{Name: filepath.Base(p), Path: p, Temp: key, Width: prepared.Width, Height: prepared.Height, Linked: link}
		if link { // the size of the picture itself: it is shown as it is
			sl.Width, sl.Height = img.Bounds().Dx(), img.Bounds().Dy()
		}
		out = append(out, sl)
	}
	if progress != nil {
		progress(len(paths), len(paths), "")
	}
	return out
}

func (s *Server) upload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(256 << 20); err != nil {
		writeErr(w, 400, err)
		return
	}
	dir := s.uploadsDir()
	var paths []string
	for _, fhs := range r.MultipartForm.File {
		for _, fh := range fhs {
			f, err := fh.Open()
			if err != nil {
				continue
			}
			name := filepath.Base(fh.Filename)
			name = strings.ReplaceAll(name, "..", "_")
			dst := filepath.Join(dir, name)
			out, err := os.Create(dst)
			if err != nil {
				f.Close()
				continue
			}
			buf := make([]byte, 1<<20)
			for {
				n, rerr := f.Read(buf)
				if n > 0 {
					out.Write(buf[:n])
				}
				if rerr != nil {
					break
				}
			}
			out.Close()
			f.Close()
			paths = append(paths, dst)
		}
	}
	if paths == nil {
		paths = []string{}
	}
	writeJSON(w, map[string]any{"paths": paths})
}

func (s *Server) download(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Query().Get("path")
	if p == "" || !strings.HasPrefix(filepath.Clean(p), s.exportsDir()) {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filepath.Base(p)+"\"")
	http.ServeFile(w, r, p)
}

// remoteInfo tells the Remote settings page where the backend listens now, and the address of every network interface of
// this computer (what other devices would use, with the port the page chooses, when public access is on).
func (s *Server) remoteInfo(w http.ResponseWriter, r *http.Request) {
	host, port, _ := net.SplitHostPort(s.addr)
	ips := []string{}
	addrs, _ := net.InterfaceAddrs()
	for _, a := range addrs {
		if n, ok := a.(*net.IPNet); ok && n.IP.To4() != nil && !n.IP.IsLoopback() && !n.IP.IsLinkLocalUnicast() {
			ips = append(ips, n.IP.String())
		}
	}
	urls := []string{}
	if ip := net.ParseIP(host); host == "" || (ip != nil && ip.IsUnspecified()) {
		for _, a := range ips {
			urls = append(urls, "http://"+net.JoinHostPort(a, port))
		}
	}
	writeJSON(w, map[string]any{"listen": s.addr, "urls": urls, "ips": ips})
}
