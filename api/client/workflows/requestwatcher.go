/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package workflows

import (
	"context"
	"sync"

	"github.com/gravitational/teleport/api/types"

	"github.com/gravitational/trace"
)

// RequestWatcher is a thin wrapper around types.Watcher
// used for watching access request events.
type RequestWatcher struct {
	types.Watcher
	initC   chan struct{}
	eventsC chan RequestEvent
	errMux  sync.Mutex
	err     error
}

// RequestEvent is an access request event.
type RequestEvent struct {
	// Type is the operation type of the event.
	// Possible values are types.OpPut and types.OpDelete
	Type types.OpType
	// Request is the payload of the event.
	Request types.AccessRequest
}

// NewRequestWatcher creates a new RequestWatcher.
func NewRequestWatcher(ctx context.Context, clt Client, filter types.AccessRequestFilter) (*RequestWatcher, error) {
	eventWatcher, err := clt.NewWatcher(ctx,
		types.Watch{
			Kinds: []types.WatchKind{
				{
					Kind:   types.KindAccessRequest,
					Filter: filter.IntoMap(),
				},
			},
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	w := &RequestWatcher{
		Watcher: eventWatcher,
		initC:   make(chan struct{}),
		eventsC: make(chan RequestEvent),
	}
	go w.receiveEvents()
	return w, nil
}

// receiveEvents receives events from the client stream and sends
// the associated RequestEvents to the requestWatcher's eventC channel.
func (w *RequestWatcher) receiveEvents() {
	for event := range w.Watcher.Events() {
		switch event.Type {
		case types.OpPut, types.OpDelete:
		case types.OpInit:
			close(w.initC)
			continue
		default:
			w.closeWithError(trace.Errorf("unexpected event op type %s", event.Type))
			return
		}

		req, ok := event.Resource.(types.AccessRequest)
		if !ok {
			w.closeWithError(trace.Errorf("unexpected resource type %T", event.Resource))
			return
		}

		select {
		case w.eventsC <- RequestEvent{Type: event.Type, Request: req}:
		case <-w.Done():
			return
		}
	}
}

// WaitInit waits for the Init operation to complete or
// returns an error if the watcher or context closes first.
func (w *RequestWatcher) WaitInit(ctx context.Context) error {
	select {
	case <-w.initC:
		return nil
	case <-w.Done():
		return w.Error()
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Events returns a channel of RequestEvents.
func (w *RequestWatcher) Events() <-chan RequestEvent {
	return w.eventsC
}

// Error returns the last error of the RequestWatcher.
func (w *RequestWatcher) Error() error {
	w.errMux.Lock()
	defer w.errMux.Unlock()
	if w.err == nil {
		return w.Watcher.Error()
	}
	return w.err
}

// setError closes the Watcher and sets the RequestWatcher error.
func (w *RequestWatcher) closeWithError(err error) {
	w.errMux.Lock()
	defer w.errMux.Unlock()
	w.Close()
	w.err = err
}
