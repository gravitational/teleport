package grpcclient

import (
	"context"

	"github.com/gravitational/teleport/lib/backend"
)

type watcher struct {
	events chan backend.Event
	done   chan struct{}
	cancel context.CancelFunc
}

var _ backend.Watcher = &watcher{}

func (w watcher) Close() error {
	w.cancel()
	return nil
}

func (w watcher) Done() <-chan struct{} {
	return w.done
}

func (w watcher) Events() <-chan backend.Event {
	return w.events
}

// shutdown is called if there's an error and we want to
// kill off the watcher
func (w watcher) shutdown() {
	close(w.done)
}
