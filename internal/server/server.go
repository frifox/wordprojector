// Package server exposes the HTTP/WebSocket API and serves the embedded UI.
package server

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"wordprojector/internal/appdata"
	"wordprojector/internal/desktop"
	"wordprojector/internal/live"
	"wordprojector/internal/modules"
)

//go:embed all:dist
var dist embed.FS

// Version is set from main.
var Version = "dev"

// Server bundles everything.
type Server struct {
	Engine  *live.Engine
	Desktop desktop.Desktop
	DataDir string
	Importer *modules.Importer

	mux  *http.ServeMux
	hub  *hub
	jobs *jobs
	addr string

	// Quit makes the program quit (set by main); the update uses it to restart.
	Quit func()
	upd  *updateState
}

// New builds the server.
func New(e *live.Engine, d desktop.Desktop, dataDir string) *Server {
	if d == nil {
		d = desktop.Noop{}
	}
	s := &Server{Engine: e, Desktop: d, DataDir: dataDir, Importer: &modules.Importer{DB: e.DB}}
	s.hub = newHub()
	s.jobs = &jobs{}
	s.upd = newUpdateState(Version)
	e.Broadcast = s.hub.broadcast
	s.mux = http.NewServeMux()
	s.routes()
	s.updateRoutes()
	return s
}

// Start listens on addr ("127.0.0.1:0" for a random port) and returns the URL.
func (s *Server) Start(addr string) (string, error) {
	if addr == "" {
		addr = "127.0.0.1:0"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return "", err
	}
	s.addr = ln.Addr().String()
	srv := &http.Server{Handler: s, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("http server: %v", err)
		}
	}()
	go s.backgroundUpdate()
	host := s.addr
	if strings.HasPrefix(host, "[::]") || strings.HasPrefix(host, "0.0.0.0") {
		_, port, _ := net.SplitHostPort(host)
		host = "127.0.0.1:" + port
	}
	return "http://" + host, nil
}

// Addr returns the bound address.
func (s *Server) Addr() string { return s.addr }

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// ---- helpers --------------------------------------------------------------------

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, err error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
}

func readJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 64<<20))
	return dec.Decode(v)
}

func pathID(r *http.Request, name string) (int64, error) {
	return strconv.ParseInt(r.PathValue(name), 10, 64)
}

func ok(w http.ResponseWriter) { writeJSON(w, map[string]bool{"ok": true}) }

// ---- websocket hub ----------------------------------------------------------------

type client struct {
	conn *websocket.Conn
	send chan []byte
	role string
	disp int
}

type hub struct {
	mu      sync.Mutex
	clients map[*client]bool
}

func newHub() *hub { return &hub{clients: map[*client]bool{}} }

func (h *hub) broadcast(msg any) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients {
		select {
		case c.send <- data:
		default:
			// slow client: drop oldest by closing
			close(c.send)
			delete(h.clients, c)
		}
	}
}

func (h *hub) add(c *client) {
	h.mu.Lock()
	h.clients[c] = true
	h.mu.Unlock()
}

func (h *hub) remove(c *client) {
	h.mu.Lock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.send)
	}
	h.mu.Unlock()
}

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	c := &client{conn: conn, send: make(chan []byte, 256)}
	s.hub.add(c)
	// initial state
	st, _ := json.Marshal(map[string]any{"type": "state", "state": s.Engine.State()})
	c.send <- st
	items, file, saved := s.Engine.Schedule.Snapshot()
	sc, _ := json.Marshal(map[string]any{"type": "schedule", "items": items, "file": file, "saved": saved})
	c.send <- sc
	go func() {
		defer conn.Close()
		for data := range c.send {
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		}
	}()
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			s.hub.remove(c)
			if c.role == "projector" {
				s.Engine.PageReady(c.disp, false)
			}
			return
		}
		var msg struct {
			Type     string  `json:"type"`
			Role     string  `json:"role"`
			Disp     int     `json:"disp"`
			Position float64 `json:"position"`
			Duration float64 `json:"duration"`
			Playing  bool    `json:"playing"`
			Ended    bool    `json:"ended"`
			Version  int64   `json:"version"`
		}
		if json.Unmarshal(data, &msg) != nil {
			continue
		}
		switch msg.Type {
		case "hello":
			c.role, c.disp = msg.Role, msg.Disp
		case "ready": // a display page has drawn its first state
			if c.role == "projector" {
				s.Engine.PageReady(c.disp, true)
			}
		case "settled": // ... and has finished drawing this state version
			if c.role == "projector" {
				s.Engine.PageSettled(c.disp, msg.Version)
			}
		case "videoStatus":
			if msg.Disp <= 1 {
				s.Engine.VideoStatus(msg.Position, msg.Duration, msg.Playing, msg.Ended)
			}
		case "next":
			s.Engine.Next()
		case "prev":
			s.Engine.Prev()
		case "exit":
			s.Engine.Escape()
		case "stop":
			s.Engine.Stop()
		}
	}
}

// notify tells clients that a data set changed.
func (s *Server) notify(what string) {
	s.hub.broadcast(map[string]any{"type": "data", "what": what})
}

func (s *Server) broadcastSchedule() {
	items, file, saved := s.Engine.Schedule.Snapshot()
	s.hub.broadcast(map[string]any{"type": "schedule", "items": items, "file": file, "saved": saved})
}

// ---- background jobs with progress --------------------------------------------------

type jobs struct {
	mu      sync.Mutex
	running bool
}

func (j *jobs) start() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.running {
		return false
	}
	j.running = true
	return true
}

func (j *jobs) done() {
	j.mu.Lock()
	j.running = false
	j.mu.Unlock()
}

func (s *Server) progress(job string) modules.Progress {
	last := time.Now()
	return func(cur, max int, msg string) {
		if time.Since(last) < 100*time.Millisecond && cur != max {
			return
		}
		last = time.Now()
		s.hub.broadcast(map[string]any{"type": "progress", "job": job, "cur": cur, "max": max, "msg": msg})
	}
}

func (s *Server) progressMsg(job, text string) {
	s.hub.broadcast(map[string]any{"type": "progress", "job": job, "text": text})
}

func (s *Server) progressDone(job string, err error) {
	m := map[string]any{"type": "progress", "job": job, "done": true}
	if err != nil {
		m["error"] = err.Error()
	}
	s.hub.broadcast(m)
}

// ---- static files -------------------------------------------------------------------

func (s *Server) serveStatic() http.Handler {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		return http.NotFoundHandler()
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" {
			p = "index.html"
		}
		if _, err := fs.Stat(sub, p); err != nil {
			// SPA fallback
			r.URL.Path = "/"
			w.Header().Set("Cache-Control", "no-store")
			fileServer.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(p, "assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-store")
		}
		fileServer.ServeHTTP(w, r)
	})
}

// uploadsDir returns the folder for files uploaded from a browser client.
func (s *Server) uploadsDir() string {
	d, _ := appdata.Sub("Uploads")
	if d == "" {
		d = filepath.Join(os.TempDir(), "wordprojector-uploads")
		_ = os.MkdirAll(d, 0o755)
	}
	return d
}

func (s *Server) exportsDir() string {
	d, _ := appdata.Sub("Exports")
	return d
}

// Shutdown is a no-op placeholder for symmetry.
func (s *Server) Shutdown(ctx context.Context) error { return nil }

func clientBool(v string) bool { return v == "1" || v == "true" }

func fmtErr(format string, a ...any) error { return fmt.Errorf(format, a...) }
