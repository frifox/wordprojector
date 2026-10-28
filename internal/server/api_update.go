package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"wordprojector/internal/update"
)

// updateState is what the program knows about releases: the list from GitHub, when it was fetched, and whether a newer
// copy is already installed (it starts after a restart).
type updateState struct {
	u *update.Updater

	mu         sync.Mutex
	releases   []update.Release
	checkedAt  time.Time
	checkErr   string
	installing string // the tag being installed ("" when idle)
	progress   string
	pending    string // the tag installed, waiting for a restart
}

func newUpdateState(current, commit string) *updateState {
	u := update.New(current)
	u.Commit = commit
	return &updateState{u: u}
}

type updateStatus struct {
	Current     string           `json:"current"`
	Commit      string           `json:"commit,omitempty"` // the commit the running program was built from
	Supported   bool             `json:"supported"`
	Reason      string           `json:"reason,omitempty"`
	Repo        string           `json:"repo"`
	Releases    []update.Release `json:"releases"`
	Latest      string           `json:"latest"`
	Newer       bool             `json:"newer"` // the latest release is newer than the running program
	CheckedAt   string           `json:"checkedAt"`
	Error       string           `json:"error,omitempty"`
	Installing  string           `json:"installing,omitempty"`
	Progress    string           `json:"progress,omitempty"`
	Pending     string           `json:"pending,omitempty"`
	CanRestart  bool             `json:"canRestart"`
}

func (s *Server) updateStatus() updateStatus {
	st := s.upd
	st.mu.Lock()
	defer st.mu.Unlock()
	ok, why := st.u.Supported()
	out := updateStatus{Current: st.u.Current, Commit: st.u.Commit, Supported: ok, Reason: why, Repo: st.u.Repo, Releases: st.releases, Error: st.checkErr,
		Installing: st.installing, Progress: st.progress, Pending: st.pending, CanRestart: ok && s.Quit != nil}
	if out.Releases == nil {
		out.Releases = []update.Release{}
	}
	if !st.checkedAt.IsZero() {
		out.CheckedAt = st.checkedAt.Format(time.RFC3339)
	}
	if l := update.Latest(st.releases); l != nil {
		out.Latest = l.Tag
		out.Newer = st.u.Newer(l) && st.pending == "" // once it is installed there is nothing new to install
	}
	return out
}

// checkUpdates fetches the list of releases.
func (s *Server) checkUpdates(ctx context.Context) {
	st := s.upd
	list, err := st.u.List(ctx)
	st.mu.Lock()
	st.checkedAt = time.Now()
	if err != nil {
		st.checkErr = err.Error()
	} else {
		st.checkErr = ""
		st.releases = list
	}
	st.mu.Unlock()
	s.notify("update")
}

// installUpdate puts a release in the place of the running program.
func (s *Server) installUpdate(ctx context.Context, tag string) error {
	st := s.upd
	st.mu.Lock()
	if st.installing != "" {
		st.mu.Unlock()
		return fmt.Errorf("an update is being installed already")
	}
	var rel *update.Release
	for i := range st.releases {
		if st.releases[i].Tag == tag {
			rel = &st.releases[i]
		}
	}
	if rel == nil {
		st.mu.Unlock()
		return fmt.Errorf("release %s is not in the list", tag)
	}
	r := *rel
	st.installing, st.progress = tag, ""
	st.mu.Unlock()
	s.notify("update")

	err := st.u.Install(ctx, r, func(p string) {
		st.mu.Lock()
		st.progress = p
		st.mu.Unlock()
		s.notify("update")
	})
	st.mu.Lock()
	st.installing, st.progress = "", ""
	if err == nil {
		st.pending = tag
	}
	st.mu.Unlock()
	s.notify("update")
	if err != nil {
		log.Printf("update to %s: %v", tag, err)
	} else {
		log.Printf("update to %s installed; it starts the next time the program is started", tag)
	}
	return err
}

// restartForUpdate starts the program again and quits this copy.
func (s *Server) restartForUpdate() error {
	if s.Quit == nil {
		return fmt.Errorf("this program cannot restart itself")
	}
	return s.upd.u.Restart(os.Args[1:], s.Quit)
}

// backgroundUpdate looks for a newer release a few seconds after the start (unless that is turned off) and, when asked
// to, installs it.
func (s *Server) backgroundUpdate() {
	s.upd.u.Cleanup()
	time.Sleep(8 * time.Second)
	var skip, auto bool
	s.Engine.Locked(func() { skip, auto = s.Engine.Settings.App.SkipUpdateCheck, s.Engine.Settings.App.AutoInstallUpdates })
	if skip {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	s.checkUpdates(ctx)
	cancel()
	st := s.updateStatus()
	if !auto || !st.Newer || !st.Supported {
		return
	}
	ctx, cancel = context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	if err := s.installUpdate(ctx, st.Latest); err != nil {
		log.Printf("automatic update: %v", err)
	}
}

func (s *Server) updateRoutes() {
	m := s.mux
	m.HandleFunc("GET /api/update", func(w http.ResponseWriter, r *http.Request) {
		s.upd.mu.Lock()
		never := s.upd.checkedAt.IsZero()
		s.upd.mu.Unlock()
		if r.URL.Query().Get("refresh") == "1" || never && r.URL.Query().Get("check") == "1" {
			ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
			s.checkUpdates(ctx)
			cancel()
		}
		writeJSON(w, s.updateStatus())
	})
	m.HandleFunc("POST /api/update/install", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Tag     string `json:"tag"`
			Restart bool   `json:"restart"`
		}
		if err := readJSON(r, &in); err != nil {
			writeErr(w, 400, err)
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
		defer cancel()
		if err := s.installUpdate(ctx, in.Tag); err != nil {
			writeErr(w, 500, err)
			return
		}
		if in.Restart {
			go func() {
				time.Sleep(600 * time.Millisecond) // lets the answer reach the page first
				if err := s.restartForUpdate(); err != nil {
					log.Printf("restart: %v", err)
				}
			}()
		}
		ok(w)
	})
	m.HandleFunc("POST /api/update/restart", func(w http.ResponseWriter, r *http.Request) {
		go func() {
			time.Sleep(600 * time.Millisecond)
			if err := s.restartForUpdate(); err != nil {
				log.Printf("restart: %v", err)
			}
		}()
		ok(w)
	})
}
