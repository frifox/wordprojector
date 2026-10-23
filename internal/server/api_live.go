package server

import (
	"net/http"
	"strconv"
	"time"

	"wordprojector/internal/announce"
	"wordprojector/internal/live"
	"wordprojector/internal/media"
	"wordprojector/internal/schedule"
	"wordprojector/internal/slides"
	"wordprojector/internal/songs"
)

func (s *Server) liveRoutes() {
	m := s.mux
	e := s.Engine
	m.HandleFunc("POST /api/live/show-bible", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Book      int   `json:"book"`
			Chapter   int   `json:"chapter"`
			Selection []int `json:"selection"`
		}
		if err := readJSON(r, &in); err != nil {
			writeErr(w, 400, err)
			return
		}
		if err := e.ShowBible(in.Book, in.Chapter, in.Selection); err != nil {
			writeErr(w, 400, err)
			return
		}
		ok(w)
	})
	m.HandleFunc("POST /api/live/show-bible-ref", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			VerseIDs string `json:"verseIds"`
		}
		if err := readJSON(r, &in); err != nil {
			writeErr(w, 400, err)
			return
		}
		if err := e.ShowBibleRef(in.VerseIDs); err != nil {
			writeErr(w, 400, err)
			return
		}
		ok(w)
	})
	m.HandleFunc("POST /api/live/show-song", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			ID   int64       `json:"id"`
			Song *songs.Song `json:"song"`
			Row  int         `json:"row"`
		}
		if err := readJSON(r, &in); err != nil {
			writeErr(w, 400, err)
			return
		}
		if in.Song != nil {
			e.ShowSong(*in.Song, in.Row, "")
		} else {
			sg, err := e.Songs.Song(in.ID)
			if err != nil {
				writeErr(w, 404, err)
				return
			}
			e.ShowSong(sg, in.Row, "")
		}
		ok(w)
	})
	m.HandleFunc("POST /api/live/show-announcement", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			ID           int64                  `json:"id"`
			Announcement *announce.Announcement `json:"announcement"`
			Row          int                    `json:"row"`
		}
		if err := readJSON(r, &in); err != nil {
			writeErr(w, 400, err)
			return
		}
		if in.Announcement != nil {
			e.ShowAnnouncement(*in.Announcement, in.Row)
		} else {
			a, err := e.Ann.Get(in.ID)
			if err != nil {
				writeErr(w, 404, err)
				return
			}
			e.ShowAnnouncement(a, in.Row)
		}
		ok(w)
	})
	m.HandleFunc("POST /api/live/show-slideshow", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			ID        int64                  `json:"id"`
			Slideshow *slides.ShowWithSlides `json:"slideshow"`
			Row       int                    `json:"row"`
		}
		if err := readJSON(r, &in); err != nil {
			writeErr(w, 400, err)
			return
		}
		if in.Slideshow != nil {
			e.ShowSlideshow(*in.Slideshow, in.Row)
		} else {
			ss, err := e.Slides.Get(in.ID)
			if err != nil {
				writeErr(w, 404, err)
				return
			}
			e.ShowSlideshow(ss, in.Row)
		}
		ok(w)
	})
	m.HandleFunc("POST /api/live/show-media", func(w http.ResponseWriter, r *http.Request) {
		var in media.Item
		if err := readJSON(r, &in); err != nil {
			writeErr(w, 400, err)
			return
		}
		e.ShowVideo(in)
		ok(w)
	})
	m.HandleFunc("POST /api/live/show-schedule", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Index int `json:"index"`
		}
		if err := readJSON(r, &in); err != nil {
			writeErr(w, 400, err)
			return
		}
		it, found := e.Schedule.Get(in.Index)
		if !found {
			writeErr(w, 404, fmtErr("no such schedule item"))
			return
		}
		switch it.Type {
		case "bible":
			if it.Bible != nil {
				if err := e.ShowBibleRef(it.Bible.VerseIDs); err != nil {
					writeErr(w, 400, err)
					return
				}
			}
		case "song":
			if it.Song != nil {
				key := ""
				if it.Song.HasBackground && it.Song.ID > 0 {
					if _, has := e.Schedule.Blob("song:" + strconv.FormatInt(it.Song.ID, 10) + ":" + it.Song.BackgroundName); has {
						key = "song:" + strconv.FormatInt(it.Song.ID, 10) + ":" + it.Song.BackgroundName
					}
				}
				e.ShowSong(*it.Song, 0, key)
			}
		case "slideshow":
			if it.Slideshow != nil {
				e.ShowSlideshow(*it.Slideshow, 0)
			}
		case "media":
			if it.Media != nil {
				e.ShowVideo(*it.Media)
			}
		case "announce":
			if it.Announce != nil {
				e.ShowAnnouncement(*it.Announce, 0)
			}
		}
		ok(w)
	})
	m.HandleFunc("POST /api/live/index", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Index int  `json:"index"`
			Force bool `json:"force"`
		}
		_ = readJSON(r, &in)
		e.SetIndex(in.Index, in.Force)
		ok(w)
	})
	m.HandleFunc("POST /api/live/selection", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Selection []int `json:"selection"`
			Project   bool  `json:"project"`
		}
		_ = readJSON(r, &in)
		e.SetSelection(in.Selection, in.Project)
		ok(w)
	})
	m.HandleFunc("POST /api/live/preview-selection", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Selection []int `json:"selection"`
		}
		_ = readJSON(r, &in)
		writeJSON(w, e.PreviewSelection(in.Selection))
	})
	m.HandleFunc("POST /api/live/test-displays", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Slots []int `json:"slots"`
		}
		_ = readJSON(r, &in)
		e.TestDisplays(in.Slots, 3*time.Second)
		ok(w)
	})
	m.HandleFunc("POST /api/live/multiverse", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			On bool `json:"on"`
		}
		_ = readJSON(r, &in)
		e.SetMultiVerse(in.On)
		ok(w)
	})
	m.HandleFunc("POST /api/live/next", func(w http.ResponseWriter, r *http.Request) { e.Next(); ok(w) })
	m.HandleFunc("POST /api/live/prev", func(w http.ResponseWriter, r *http.Request) { e.Prev(); ok(w) })
	m.HandleFunc("POST /api/live/show", func(w http.ResponseWriter, r *http.Request) { e.Show(); ok(w) })
	m.HandleFunc("POST /api/live/hide", func(w http.ResponseWriter, r *http.Request) { e.Hide(); ok(w) })
	m.HandleFunc("POST /api/live/clear", func(w http.ResponseWriter, r *http.Request) { e.Clear(); ok(w) })
	m.HandleFunc("POST /api/live/escape", func(w http.ResponseWriter, r *http.Request) { e.Escape(); ok(w) })
	m.HandleFunc("POST /api/live/preview-slide", func(w http.ResponseWriter, r *http.Request) {
		var sl live.Slide
		if err := readJSON(r, &sl); err != nil {
			writeErr(w, 400, err)
			return
		}
		e.SetPreviewSlide(sl)
		ok(w)
	})
	m.HandleFunc("DELETE /api/live/preview-slide", func(w http.ResponseWriter, r *http.Request) { e.ClearPreviewSlide(); ok(w) })
	m.HandleFunc("POST /api/live/stop", func(w http.ResponseWriter, r *http.Request) { e.Stop(); ok(w) })
	m.HandleFunc("POST /api/live/pause", func(w http.ResponseWriter, r *http.Request) { e.Pause(); ok(w) })
	m.HandleFunc("POST /api/live/display", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			On bool `json:"on"`
		}
		_ = readJSON(r, &in)
		e.SetDisplayOn(in.On)
		ok(w)
	})
	m.HandleFunc("POST /api/live/video", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Cmd   string  `json:"cmd"`
			Value float64 `json:"value"`
		}
		_ = readJSON(r, &in)
		e.VideoCommand(in.Cmd, in.Value)
		ok(w)
	})

	// schedule
	m.HandleFunc("GET /api/schedule", func(w http.ResponseWriter, r *http.Request) {
		items, file, saved := e.Schedule.Snapshot()
		writeJSON(w, map[string]any{"items": items, "file": file, "saved": saved})
	})
	m.HandleFunc("POST /api/schedule/add", func(w http.ResponseWriter, r *http.Request) {
		var it schedule.Item
		if err := readJSON(r, &it); err != nil {
			writeErr(w, 400, err)
			return
		}
		// resolve full records when only ids are given
		switch it.Type {
		case "song":
			if it.Song != nil && it.Song.ID > 0 && it.Song.Text == "" {
				if sg, err := e.Songs.Song(it.Song.ID); err == nil {
					it.Song = &sg
				}
			}
		case "slideshow":
			if it.Slideshow != nil && it.Slideshow.ID > 0 && len(it.Slideshow.Slides) == 0 {
				if ss, err := e.Slides.Get(it.Slideshow.ID); err == nil {
					it.Slideshow = &ss
				}
			}
		case "announce":
			if it.Announce != nil && it.Announce.ID > 0 && it.Announce.Text == "" {
				if a, err := e.Ann.Get(it.Announce.ID); err == nil {
					it.Announce = &a
				}
			}
		}
		if it.Name == "" {
			it.Name = schedule.Label(it)
		}
		e.Schedule.Add(it)
		s.broadcastSchedule()
		ok(w)
	})
	m.HandleFunc("POST /api/schedule/remove", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Index int `json:"index"`
		}
		_ = readJSON(r, &in)
		e.Schedule.Remove(in.Index)
		s.broadcastSchedule()
		ok(w)
	})
	m.HandleFunc("POST /api/schedule/move", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			From, To int
		}
		_ = readJSON(r, &in)
		e.Schedule.Move(in.From, in.To)
		s.broadcastSchedule()
		ok(w)
	})
	m.HandleFunc("POST /api/schedule/clear", func(w http.ResponseWriter, r *http.Request) {
		e.Schedule.Clear()
		s.broadcastSchedule()
		ok(w)
	})
	m.HandleFunc("POST /api/schedule/new", func(w http.ResponseWriter, r *http.Request) {
		e.Schedule.NewFile()
		s.broadcastSchedule()
		ok(w)
	})
	m.HandleFunc("POST /api/schedule/close", func(w http.ResponseWriter, r *http.Request) {
		e.Schedule.Close()
		s.broadcastSchedule()
		ok(w)
	})
	m.HandleFunc("POST /api/schedule/open", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Path string `json:"path"`
		}
		_ = readJSON(r, &in)
		if in.Path == "" {
			paths, err := s.Desktop.OpenFiles("Open SoftProjector schedule:", []desktopFilter{{Name: "SoftProjector schedule file (*.spsc)", Pattern: "*.spsc"}}, false)
			if err != nil || len(paths) == 0 {
				writeJSON(w, map[string]any{"cancelled": true})
				return
			}
			in.Path = paths[0]
		}
		if err := e.Schedule.Open(in.Path); err != nil {
			writeErr(w, 400, err)
			return
		}
		s.broadcastSchedule()
		ok(w)
	})
	m.HandleFunc("POST /api/schedule/save", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Path string `json:"path"`
			As   bool   `json:"as"`
		}
		_ = readJSON(r, &in)
		_, file, _ := e.Schedule.Snapshot()
		path := in.Path
		if path == "" {
			if in.As || file == "" || file == "untitled.spsc" {
				if !s.Desktop.Available() {
					path = s.exportPath("schedule.spsc")
				} else {
					p, err := s.Desktop.SaveFile("Save SoftProjector schedule as:", "untitled.spsc", []desktopFilter{{Name: "SoftProjector schedule file (*.spsc)", Pattern: "*.spsc"}})
					if err != nil || p == "" {
						writeJSON(w, map[string]any{"cancelled": true})
						return
					}
					path = p
				}
			} else {
				path = file
			}
		}
		if !hasSuffixFold(path, ".spsc") {
			path += ".spsc"
		}
		if err := e.Schedule.Save(path, blobber{e}); err != nil {
			writeErr(w, 500, err)
			return
		}
		s.broadcastSchedule()
		resp := map[string]any{"ok": true, "path": path}
		if !s.Desktop.Available() {
			resp["download"] = "/api/download?path=" + urlQuery(path)
		}
		writeJSON(w, resp)
	})
}

type blobber struct{ e *live.Engine }

func (b blobber) SongBackground(id int64) []byte {
	bg, _ := b.e.SongBG(id)
	return bg
}

func (b blobber) SlideImages(id int64) (full, small, prev []byte) {
	return b.e.SlideImgs(id)
}
