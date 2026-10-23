//go:build headless

package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	o := parseFlags()
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
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	<-ch
}
