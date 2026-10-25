package live

import (
	"testing"
	"time"
)

func TestWaitPageEvents(t *testing.T) {
	e := &Engine{}
	if e.WaitPageReady(1, 20*time.Millisecond) {
		t.Fatal("ready before the page said so")
	}
	go func() {
		time.Sleep(10 * time.Millisecond)
		e.PageReady(1, true)
	}()
	start := time.Now()
	if !e.WaitPageReady(1, 2*time.Second) {
		t.Fatal("did not wake up when the page was ready")
	}
	if time.Since(start) > 500*time.Millisecond {
		t.Fatal("waited for the time out instead of the event")
	}
}

func TestWaitPageVersions(t *testing.T) {
	e := &Engine{}
	e.PageApplied(2, 5)
	if !e.WaitPageApplied(2, 5, time.Millisecond) || !e.WaitPageApplied(2, 4, time.Millisecond) {
		t.Fatal("applied 5 covers 4 and 5")
	}
	if e.WaitPageApplied(2, 6, 10*time.Millisecond) {
		t.Fatal("6 was not applied")
	}
	go func() {
		time.Sleep(5 * time.Millisecond)
		e.PageSettled(2, 7)
	}()
	if !e.WaitPageSettled(2, 7, 2*time.Second) {
		t.Fatal("settled 7 not seen")
	}
	if !e.WaitPageApplied(2, 7, time.Millisecond) {
		t.Fatal("a settled version is applied too")
	}
	e.PageReady(2, false)
	if e.SettledVersion(2) != 0 {
		t.Fatal("a page that is gone has settled nothing")
	}
}
