package live

import (
	"sync"
	"time"
)

// pageStates is what the display pages (the windows' web pages) have reported. A window is shown only when its page
// has the state that asked for it, and hidden only after its page has drawn the final state (no old picture left behind
// for the next show). Waiting for that is event driven: every report wakes the waiters.
type pageStates struct {
	mu      sync.Mutex
	ready   map[int]bool
	applied map[int]int64 // newest state version the page has put into its document (needs no painting)
	settled map[int]int64 // newest state version the page has drawn, and finished fading to
	changed chan struct{} // closed (and replaced) whenever something above changes
}

func (p *pageStates) init() {
	if p.ready == nil {
		p.ready, p.applied, p.settled, p.changed = map[int]bool{}, map[int]int64{}, map[int]int64{}, make(chan struct{})
	}
}

// notify wakes everyone waiting. The caller holds the lock.
func (p *pageStates) notify() {
	close(p.changed)
	p.changed = make(chan struct{})
}

// wait blocks until done reports true, a page reports something that makes it true, or the time is up (that is a
// safety net for a page that never answers, not a delay: it is not waited for when the page does answer).
func (p *pageStates) wait(max time.Duration, done func() bool) bool {
	timeout := time.NewTimer(max)
	defer timeout.Stop()
	for {
		p.mu.Lock()
		p.init()
		if done() {
			p.mu.Unlock()
			return true
		}
		ch := p.changed
		p.mu.Unlock()
		select {
		case <-ch:
		case <-timeout.C:
			return false
		}
	}
}

// PageReady records that the page of display disp has drawn its first state; false forgets it (the page is gone).
func (e *Engine) PageReady(disp int, ready bool) {
	p := &e.pages
	p.mu.Lock()
	defer p.mu.Unlock()
	p.init()
	p.ready[disp] = ready
	if !ready {
		delete(p.applied, disp)
		delete(p.settled, disp)
	}
	p.notify()
}

// IsPageReady reports whether the page of display disp has drawn its first state.
func (e *Engine) IsPageReady(disp int) bool {
	p := &e.pages
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.ready[disp]
}

// PageApplied records the newest state version the page of display disp has put into its document. That happens
// also while the window is hidden, when nothing is drawn.
func (e *Engine) PageApplied(disp int, version int64) {
	p := &e.pages
	p.mu.Lock()
	defer p.mu.Unlock()
	p.init()
	if version > p.applied[disp] {
		p.applied[disp] = version
		p.notify()
	}
}

// PageSettled records the newest state version the page of display disp has finished drawing.
func (e *Engine) PageSettled(disp int, version int64) {
	p := &e.pages
	p.mu.Lock()
	defer p.mu.Unlock()
	p.init()
	if version > p.settled[disp] {
		p.settled[disp] = version
		p.notify()
	}
	if version > p.applied[disp] {
		p.applied[disp] = version
	}
}

// SettledVersion is the newest state version the page of display disp has finished drawing.
func (e *Engine) SettledVersion(disp int) int64 {
	p := &e.pages
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.settled[disp]
}

// WaitPageReady waits until the page of display disp has drawn its first state. False when max passed first.
func (e *Engine) WaitPageReady(disp int, max time.Duration) bool {
	return e.pages.wait(max, func() bool { return e.pages.ready[disp] })
}

// WaitPageApplied waits until the page of display disp has the state of that version in its document.
func (e *Engine) WaitPageApplied(disp int, version int64, max time.Duration) bool {
	return e.pages.wait(max, func() bool { return e.pages.applied[disp] >= version })
}

// WaitPageSettled waits until the page of display disp has drawn the state of that version and finished fading to it.
func (e *Engine) WaitPageSettled(disp int, version int64, max time.Duration) bool {
	return e.pages.wait(max, func() bool { return e.pages.settled[disp] >= version })
}
