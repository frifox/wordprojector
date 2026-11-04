package server

import (
	"errors"
	"io/fs"
	"log"
	"net/http"
	"os"
	"runtime"
	"time"

	"wordprojector/internal/install"
	"wordprojector/internal/update"
)

// the program is put in its place and, for the system's menus, its icon is written from the one the page uses
func init() {
	if b, err := fs.ReadFile(dist, "dist/icons/softprojector.png"); err == nil {
		install.Icon = b
	}
}

func (s *Server) installRoutes() {
	m := s.mux
	m.HandleFunc("GET /api/install", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, install.Status(UninstallMode))
	})
	m.HandleFunc("POST /api/install", func(w http.ResponseWriter, r *http.Request) {
		var in install.Options
		if err := readJSON(r, &in); err != nil {
			writeErr(w, 400, err)
			return
		}
		if s.Quit == nil {
			writeErr(w, 400, errors.New("this program cannot restart itself"))
			return
		}
		res, err := install.Install(in, Version)
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		if res.Warning != "" {
			log.Printf("install: %s", res.Warning)
		}
		log.Printf("installed to %s", res.Exe)
		writeJSON(w, res)
		// the program starts again from its new place
		go func() {
			time.Sleep(900 * time.Millisecond) // lets the answer reach the page first
			u := &update.Updater{Exe: res.Exe, GOOS: runtime.GOOS}
			var args []string
			if res.RemoveOld != "" {
				args = []string{install.RemoveOldFlag, res.RemoveOld}
			}
			if err := u.Restart(args, s.Quit); err != nil {
				log.Printf("restart: %v", err)
			}
		}()
	})
	m.HandleFunc("POST /api/uninstall", func(w http.ResponseWriter, r *http.Request) {
		var in install.UninstallOptions
		if err := readJSON(r, &in); err != nil {
			writeErr(w, 400, err)
			return
		}
		if s.Quit == nil {
			writeErr(w, 400, errors.New("this program cannot quit by itself"))
			return
		}
		if err := install.Uninstall(in); err != nil {
			writeErr(w, 500, err)
			return
		}
		log.Printf("uninstalling: %+v", in)
		ok(w)
		go func() {
			time.Sleep(600 * time.Millisecond)
			s.Quit()
			time.Sleep(3 * time.Second)
			os.Exit(0) // a quit that did not happen must not leave the files locked
		}()
	})
}
