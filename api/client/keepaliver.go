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

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
)

// NewKeepAliver returns a new instance of keep aliver.
// It is the caller's responsibility to invoke Close on the
// returned value to release the keepAliver resources.
func (c *Client) NewKeepAliver(ctx context.Context) (types.KeepAliver, error) {
	cancelCtx, cancel := context.WithCancel(ctx)
	stream, err := c.grpc.SendKeepAlives(cancelCtx)
	if err != nil {
		cancel()
		return nil, trace.Wrap(err)
	}
	k := &streamKeepAliver{
		stream:      stream,
		ctx:         cancelCtx,
		cancel:      cancel,
		keepAlivesC: make(chan types.KeepAlive),
	}
	go k.forwardKeepAlives()
	go k.recv()
	return k, nil
}

type streamKeepAliver struct {
	mu          sync.RWMutex
	stream      proto.AuthService_SendKeepAlivesClient
	ctx         context.Context
	cancel      context.CancelFunc
	keepAlivesC chan types.KeepAlive
	err         error
}

// KeepAlives returns the streamKeepAliver's channel of KeepAlives
func (k *streamKeepAliver) KeepAlives() chan<- types.KeepAlive {
	return k.keepAlivesC
}

func (k *streamKeepAliver) forwardKeepAlives() {
	for {
		select {
		case <-k.ctx.Done():
			return
		case keepAlive := <-k.keepAlivesC:
			err := k.stream.Send(&keepAlive)
			if err != nil {
				k.closeWithError(trace.Wrap(err))
				return
			}
		}
	}
}

// Error returns the streamKeepAliver's error after closing
func (k *streamKeepAliver) Error() error {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.err
}

// Done returns a channel that closes once the streamKeepAliver is Closed
func (k *streamKeepAliver) Done() <-chan struct{} {
	return k.ctx.Done()
}

// recv is necessary to receive errors from the
// server, otherwise no errors will be propagated
func (k *streamKeepAliver) recv() {
	err := k.stream.RecvMsg(&emptypb.Empty{})
	k.closeWithError(trace.Wrap(err))
}

func (k *streamKeepAliver) closeWithError(err error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.Close()
	k.err = err
}

// Close the streamKeepAliver
func (k *streamKeepAliver) Close() error {
	k.cancel()
	return nil
}
