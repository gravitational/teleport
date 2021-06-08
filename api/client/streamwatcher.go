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
	stream, err := c.grpc.WatchEvents(cancelCtx, &protoWatch, c.callOpts...)
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
		out, err := eventFromGRPC(*event)
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

// eventFromGRPC converts an proto.Event to a types.Event
func eventFromGRPC(in proto.Event) (*types.Event, error) {
	eventType, err := eventTypeFromGRPC(in.Type)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out := types.Event{
		Type: eventType,
	}
	if eventType == types.OpInit {
		return &out, nil
	}
	if r := in.GetResourceHeader(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetCertAuthority(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetStaticTokens(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetProvisionToken(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetClusterName(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetClusterConfig(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetUser(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetRole(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetNamespace(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetServer(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetReverseTunnel(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetTunnelConnection(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetAccessRequest(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetAppSession(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetWebSession(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetWebToken(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetRemoteCluster(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetDatabaseServer(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetClusterNetworkingConfig(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetSessionRecordingConfig(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetAuthPreference(); r != nil {
		out.Resource = r
		return &out, nil
	} else {
		return nil, trace.BadParameter("received unsupported resource %T", in.Resource)
	}
}

func eventTypeFromGRPC(in proto.Operation) (types.OpType, error) {
	switch in {
	case proto.Operation_INIT:
		return types.OpInit, nil
	case proto.Operation_PUT:
		return types.OpPut, nil
	case proto.Operation_DELETE:
		return types.OpDelete, nil
	default:
		return types.OpInvalid, trace.BadParameter("unsupported operation type: %v", in)
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
