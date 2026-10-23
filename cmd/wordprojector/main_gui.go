//go:build !headless

package main

import (
	"fmt"
	"os"

	"wordprojector/internal/shell"
)

func main() {
	o := parseFlags()
	if o.headless {
		runHeadless(o)
		return
	}
	sh := shell.New("Word Projector", o.noUI)
	srv, engine, _, err := boot(o, sh)
	if err != nil {
		sh.Fatal("Word Projector could not start", err.Error())
		os.Exit(1)
	}
	sh.Configure(engine)
	srv.Quit = sh.Quit
	// the backend is bound once the event loop runs, so the "port is taken" question can be shown
	err = sh.Run(func() (string, bool, error) {
		return startServer(srv, o, engine, func(port int, existing string) int {
			switch sh.AskPortBusy(port, existing, !o.noUI) {
			case shell.PortConnect:
				return choiceConnect
			case shell.PortNext:
				return choiceNext
			}
			return choiceQuit
		})
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runHeadless(o runtimeOpts) {
	srv, engine, dir, err := boot(o, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	srv.Quit = func() { os.Exit(0) } // the update restarts the program through this
	url, other, err := startServer(srv, o, engine, askOnTerminal)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if other {
		fmt.Printf("Connected to the running Word Projector: open %s in a browser\n", url)
		return
	}
	fmt.Printf("Word Projector %s (headless)\nData: %s\nOpen %s in a browser; projector pages at %s/projector/1\n", version, dir, url, url)
	select {}
}
