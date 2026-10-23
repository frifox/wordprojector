package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"os/exec"
	"runtime"
	"strings"

	"wordprojector/internal/update"
)

// projectURL is the project page on GitHub: the one place the program sends people to for help.
const projectURL = "https://github.com/" + update.DefaultRepo

// openInBrowser opens a page of the project in the computer's own browser (the program's window cannot browse).
func openInBrowser(url string) error {
	if url != projectURL && !strings.HasPrefix(url, projectURL+"/") {
		return errors.New("only pages of the project can be opened")
	}
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}

func (s *Server) openRoutes() {
	s.mux.HandleFunc("POST /api/open-url", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			URL string `json:"url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			writeErr(w, 400, err)
			return
		}
		if err := openInBrowser(in.URL); err != nil {
			writeErr(w, 400, err)
			return
		}
		ok(w)
	})
}
