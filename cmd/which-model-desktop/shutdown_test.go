package main

import (
	"sync"
	"testing"
	"time"
)

func TestCancelTrayTimersIdempotent(t *testing.T) {
	fired := make(chan struct{}, 1)
	trayMu.Lock()
	trayFallbackTimer = time.AfterFunc(25*time.Millisecond, func() { fired <- struct{}{} })
	trayMu.Unlock()

	cancelTrayTimers()
	cancelTrayTimers()

	trayMu.Lock()
	pending := trayFallbackTimer
	trayMu.Unlock()
	if pending != nil {
		t.Fatal("cancelTrayTimers left the fallback timer tracked")
	}
	select {
	case <-fired:
		t.Fatal("cancelled tray fallback timer fired")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestCancelPopoverTimersIdempotent(t *testing.T) {
	fired := make(chan struct{}, 1)
	popoverMu.Lock()
	popoverFocusTimer = time.AfterFunc(25*time.Millisecond, func() { fired <- struct{}{} })
	popoverMu.Unlock()

	cancelPopoverTimers()
	cancelPopoverTimers()

	popoverMu.Lock()
	pending := popoverFocusTimer
	popoverMu.Unlock()
	if pending != nil {
		t.Fatal("cancelPopoverTimers left the focus timer tracked")
	}
	select {
	case <-fired:
		t.Fatal("cancelled popover focus timer fired")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestHidePopoverCancelsFocusTimer(t *testing.T) {
	fired := make(chan struct{}, 1)
	popoverMu.Lock()
	popoverFocusTimer = time.AfterFunc(25*time.Millisecond, func() { fired <- struct{}{} })
	popoverMu.Unlock()

	hidePopover(nil)

	popoverMu.Lock()
	pending := popoverFocusTimer
	popoverMu.Unlock()
	if pending != nil {
		t.Fatal("hidePopover left the focus timer tracked")
	}
	select {
	case <-fired:
		t.Fatal("focus timer fired after hidePopover")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestEmitBridgeCloseIdempotent(t *testing.T) {
	bridge := newEmitBridge(nil)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			bridge.Close()
		}()
	}
	wg.Wait()

	select {
	case <-bridge.closed:
	default:
		t.Fatal("emitBridge.Close did not close the bridge")
	}
}
