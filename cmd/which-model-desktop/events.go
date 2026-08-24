// Events bridge (S00 SPEC §2.2, S02 SPEC §2.3). Bridge.Emit is the
// service.EmitFunc passed to service.New: it never blocks (drops with a log
// line when the 64-deep buffer is full) and forwards event names/payloads
// verbatim to the frontend via app.Event.Emit once the app exists. The bridge
// also invokes a host-side tap (the tray label refresh) on the drain goroutine
// before each emit, so host UI refreshes piggyback on the same single
// event-delivery goroutine.
package main

import (
	"log"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// emitMsg is one queued event crossing from a service write lock to the host.
type emitMsg struct {
	Name string
	Data any
}

// emitBufferCapacity is the buffered channel capacity (S02 SPEC §2.3).
const emitBufferCapacity = 64

// emitBridge is the S00 §2.2 events bridge. Emit never blocks.
type emitBridge struct {
	ch  chan emitMsg // capacity 64
	tap func(string) // host-side listener (tray label refresh); called on the drain goroutine

	app    *application.App
	closed chan struct{}
	once   sync.Once // guards SetApp starting the drain goroutine
}

// newEmitBridge returns a bridge that queues events until SetApp hands it the
// live app and starts the single drain goroutine. tap may be nil.
func newEmitBridge(tap func(name string)) *emitBridge {
	return &emitBridge{
		ch:     make(chan emitMsg, emitBufferCapacity),
		tap:    tap,
		closed: make(chan struct{}),
	}
}

// SetApp records the live app and starts the drain goroutine (once). Events
// queued before this call flush through in order. Calling SetApp more than
// once is a no-op.
func (b *emitBridge) SetApp(app *application.App) {
	b.once.Do(func() {
		b.app = app
		go b.drain()
	})
}

// Emit queues an event for the frontend. It never blocks: when the buffer is
// full the event is dropped and one line is logged (S02 SPEC §2.3).
func (b *emitBridge) Emit(name string, data any) {
	select {
	case b.ch <- emitMsg{Name: name, Data: data}:
	default:
		log.Printf("events: dropped %s (queue full)", name)
	}
}

// Close stops the drain goroutine. Idempotent.
func (b *emitBridge) Close() {
	select {
	case <-b.closed:
		return
	default:
		close(b.closed)
	}
}

// drain delivers queued events. It runs until Close. For each event it first
// invokes the host-side tap (tray label refresh), then forwards to the
// frontend via app.Event.Emit (the v3 beta.9 spelling of app.EmitEvent; S02
// SPEC §2.3 "verify exact method name at implementation").
func (b *emitBridge) drain() {
	for {
		select {
		case <-b.closed:
			return
		case eve := <-b.ch:
			if b.tap != nil {
				b.tap(eve.Name)
			}
			if b.app != nil {
				b.app.Event.Emit(eve.Name, eve.Data)
			}
		}
	}
}
