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

package alpnproxy

import (
	"context"
	"net"

	"github.com/gravitational/trace"
)

// ListenerMuxWrapper wraps the net.Listener and multiplex incoming connection from serviceListener and connection
// injected by HandleConnection handler.
type ListenerMuxWrapper struct {
	// net.Listener is the main service listener that is being wrapped.
	net.Listener
	// alpnListener is the ALPN service listener.
	alpnListener net.Listener
	connC        chan net.Conn
	errC         chan error
	close        chan struct{}
}

// NewMuxListenerWrapper creates a new instance of ListenerMuxWrapper
func NewMuxListenerWrapper(serviceListener, alpnListener net.Listener) *ListenerMuxWrapper {
	listener := &ListenerMuxWrapper{
		alpnListener: alpnListener,
		Listener:     serviceListener,
		connC:        make(chan net.Conn),
		errC:         make(chan error),
		close:        make(chan struct{}),
	}
	go listener.startAcceptingConnectionServiceListener()
	return listener
}

// HandleConnection allows injecting connection to the listener.
func (l *ListenerMuxWrapper) HandleConnection(ctx context.Context, conn net.Conn) error {
	select {
	case <-l.close:
		return trace.ConnectionProblem(nil, "listener is closed")
	case <-ctx.Done():
		return ctx.Err()
	case l.connC <- conn:
		return nil
	}
}

// Addr returns address of the listeners. If both serviceListener and alpnListener listeners were provided.
// function will return address obtained from the alpnListener listener.
func (l *ListenerMuxWrapper) Addr() net.Addr {
	if l.alpnListener != nil {
		return l.alpnListener.Addr()
	}
	return l.Listener.Addr()
}

// Accept waits for the next injected by HandleConnection or received from serviceListener and returns it.
func (l *ListenerMuxWrapper) Accept() (net.Conn, error) {
	select {
	case <-l.close:
		return nil, trace.ConnectionProblem(nil, "listener is closed")
	case err := <-l.errC:
		return nil, trace.Wrap(err)
	case conn := <-l.connC:
		return conn, nil
	}
}

func (l *ListenerMuxWrapper) startAcceptingConnectionServiceListener() {
	if l.Listener == nil {
		return
	}
	for {
		conn, err := l.Listener.Accept()
		if err != nil {
			l.errC <- err
			return
		}
		select {
		case l.connC <- conn:
		case <-l.close:
			return

		}
	}
}

// Close the ListenerMuxWrapper.
func (l *ListenerMuxWrapper) Close() error {
	var errs []error
	if l.Listener != nil {
		if err := l.Listener.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if l.alpnListener != nil {
		if err := l.alpnListener.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	// Close channel only once.
	select {
	case <-l.close:
	default:
		close(l.close)
	}
	return trace.NewAggregate(errs...)
}
