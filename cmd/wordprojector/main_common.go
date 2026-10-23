package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"wordprojector/internal/appdata"
	"wordprojector/internal/db"
	"wordprojector/internal/desktop"
	"wordprojector/internal/live"
	"wordprojector/internal/server"
	"wordprojector/internal/settings"
)

// version is injected at build time with -ldflags "-X main.version=...".
var version = "dev"

type runtimeOpts struct {
	dataDir  string
	listen   string
	headless bool
	noUI     bool
	openUI   bool
}

func parseFlags() runtimeOpts {
	var o runtimeOpts
	flag.StringVar(&o.dataDir, "data", "", "data directory (default: per-user application data)")
	flag.StringVar(&o.listen, "listen", "", "address to listen on, e.g. 127.0.0.1:8080 (default: random localhost port)")
	flag.BoolVar(&o.headless, "headless", false, "run only the web server without native windows")
	flag.BoolVar(&o.noUI, "no-ui", false, "start without the operator window (display windows still open); operate from a browser")
	flag.Parse()
	if v := os.Getenv("WORDPROJECTOR_LISTEN"); v != "" && o.listen == "" {
		o.listen = v
	}
	if v := os.Getenv("WORDSPROJECTOR_LISTEN"); v != "" && o.listen == "" { // the old name of the variable
		o.listen = v
	}
	return o
}

// boot opens the database and builds engine + server.
func boot(o runtimeOpts, d desktop.Desktop) (*server.Server, *live.Engine, string, error) {
	server.Version = version
	dir := o.dataDir
	if dir == "" {
		var err error
		dir, err = appdata.Dir()
		if err != nil {
			return nil, nil, "", err
		}
	} else {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, nil, "", err
		}
		appdata.SetDir(dir)
	}
	logFile, err := os.OpenFile(filepath.Join(dir, "wordprojector.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err == nil {
		log.SetOutput(logFile)
	}
	log.Printf("Word Projector %s starting, data dir %s", version, dir)
	database, err := db.Open(filepath.Join(dir, appdata.DBFileName))
	if err != nil {
		return nil, nil, "", fmt.Errorf("open database: %w", err)
	}
	engine, err := live.New(database)
	if err != nil {
		return nil, nil, "", err
	}
	// slide show pictures are copied to ~/Documents/WordProjector/SlideShows
	if pics, err := appdata.DocumentsDir("SlideShows"); err != nil {
		log.Printf("slide show folder: %v (pictures stay in the database)", err)
	} else {
		engine.Slides.Dir = pics
		if err := engine.Slides.Migrate(); err != nil {
			log.Printf("moving slide show pictures: %v", err)
		}
	}
	// background images of the themes (Settings > Background) go to ~/Documents/WordProjector/Backgrounds
	if bgs, err := appdata.DocumentsDir("Backgrounds"); err != nil {
		log.Printf("backgrounds folder: %v (images stay in the database)", err)
	} else {
		engine.Themes.Dir = bgs
		if err := engine.Themes.Migrate(); err != nil {
			log.Printf("moving background images: %v", err)
		}
	}
	srv := server.New(engine, d, dir)
	return srv, engine, dir, nil
}

// Choices when the port is taken.
const (
	choiceConnect = iota // use the Word Projector that is already listening
	choiceNext           // listen on the next free port
	choiceQuit
)

var errQuit = errors.New("quit")

// listenAddr is where the backend should listen: --listen / WORDPROJECTOR_LISTEN, else the Remote settings.
// explicit is true for the former (the user asked for exactly that, so a taken port is an error, not a question).
func listenAddr(o runtimeOpts, e *live.Engine) (addr string, explicit bool) {
	if o.listen != "" {
		return o.listen, true
	}
	var r settings.Remote
	e.Locked(func() { r = e.Settings.App.Remote })
	return r.ListenAddr(), false
}

func addrInUse(err error) bool {
	if errors.Is(err, syscall.EADDRINUSE) {
		return true
	}
	m := strings.ToLower(err.Error())
	return strings.Contains(m, "address already in use") || strings.Contains(m, "only one usage of each socket address")
}

// existingInstance describes the Word Projector listening on the port ("" if it is something else or nothing).
func existingInstance(port string) string {
	c := http.Client{Timeout: 2 * time.Second}
	resp, err := c.Get("http://127.0.0.1:" + port + "/api/ping")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	var in struct {
		App     string `json:"app"`
		Version string `json:"version"`
	}
	if json.NewDecoder(resp.Body).Decode(&in) != nil || in.App != "WordProjector" {
		return ""
	}
	return "Word Projector " + in.Version
}

// startServer binds the backend. When the wanted port is taken, ask decides: connect to the instance that holds it
// (other is true, nothing is bound here), bind the next free port, or quit.
func startServer(srv *server.Server, o runtimeOpts, e *live.Engine, ask func(port int, existing string) int) (url string, other bool, err error) {
	addr, explicit := listenAddr(o, e)
	url, err = srv.Start(addr)
	if err == nil || explicit || !addrInUse(err) {
		return url, false, err
	}
	host, portStr, serr := net.SplitHostPort(addr)
	port, _ := strconv.Atoi(portStr)
	if serr != nil || port == 0 {
		return "", false, err
	}
	existing := existingInstance(portStr)
	switch ask(port, existing) {
	case choiceConnect:
		if existing == "" || o.noUI { // without an operator window there is nothing to show of it
			return "", false, errQuit
		}
		log.Printf("port %d is taken by %s: connecting to it", port, existing)
		return "http://127.0.0.1:" + portStr, true, nil
	case choiceNext:
		for p := port + 1; p <= port+100 && p <= 65535; p++ {
			if url, err = srv.Start(net.JoinHostPort(host, strconv.Itoa(p))); err == nil {
				log.Printf("port %d is taken: listening on %d instead", port, p)
				return url, false, nil
			}
			if !addrInUse(err) {
				return "", false, err
			}
		}
		return "", false, fmt.Errorf("no free port found after %d", port)
	}
	return "", false, errQuit
}

// askOnTerminal is the "port is taken" question for the headless server.
func askOnTerminal(port int, existing string) int {
	fmt.Printf("Port %d is already in use.", port)
	opts := "[n] use the next available port, [q] quit"
	if existing != "" {
		fmt.Printf(" %s is already running there.", existing)
		opts = "[c] connect to it, " + opts
	}
	fmt.Printf("\n%s: ", opts)
	var in string
	fmt.Scanln(&in)
	switch strings.ToLower(strings.TrimSpace(in)) {
	case "c":
		return choiceConnect
	case "n":
		return choiceNext
	}
	return choiceQuit
}
