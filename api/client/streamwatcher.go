/*
Copyright 2020 Gravitational, Inc.

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

package client

import (
	"context"
	"sync"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"

	"github.com/gravitational/trace"
	"github.com/gravitational/trace/trail"
)

// NewWatcher returns a new streamWatcher
func (c *Client) NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error) {
	cancelCtx, cancel := context.WithCancel(ctx)
	var protoWatch proto.Watch
	for _, kind := range watch.Kinds {
		protoWatch.Kinds = append(protoWatch.Kinds, proto.FromWatchKind(kind))
	}
	stream, err := c.grpc.WatchEvents(cancelCtx, &protoWatch)
	if err != nil {
		cancel()
		return nil, trail.FromGRPC(err)
	}
	w := &streamWatcher{
		stream:  stream,
		ctx:     cancelCtx,
		cancel:  cancel,
		eventsC: make(chan types.Event),
	}
	go w.receiveEvents()
	return w, nil
}

type streamWatcher struct {
	mu      sync.RWMutex
	stream  proto.AuthService_WatchEventsClient
	ctx     context.Context
	cancel  context.CancelFunc
	eventsC chan types.Event
	err     error
}

// Error returns the streamWatcher's error
func (w *streamWatcher) Error() error {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.err == nil {
		return trace.Wrap(w.ctx.Err())
	}
	return w.err
}

func (w *streamWatcher) closeWithError(err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.Close()
	w.err = err
}

// Events returns the streamWatcher's events channel
func (w *streamWatcher) Events() <-chan types.Event {
	return w.eventsC
}

func (w *streamWatcher) receiveEvents() {
	for {
		event, err := w.stream.Recv()
		if err != nil {
			w.closeWithError(trail.FromGRPC(err))
			return
		}
		out, err := EventFromGRPC(*event)
		if err != nil {
			w.closeWithError(trail.FromGRPC(err))
			return
		}
		select {
		case w.eventsC <- *out:
		case <-w.Done():
			return
		}
	}
}

// Done returns a channel that closes once the streamWatcher is Closed
func (w *streamWatcher) Done() <-chan struct{} {
	return w.ctx.Done()
}

// Close the streamWatcher
func (w *streamWatcher) Close() error {
	w.cancel()
	return nil
}
