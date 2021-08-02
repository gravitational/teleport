/*
Copyright 2020-2021 Gravitational, Inc.

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
	alpnListener net.Listener
	net.Listener
	connC chan net.Conn
	errC  chan error
	close chan struct{}
}

// NewMuxListenerWrapper creates a new instance of ListenerMuxWrapper
func NewMuxListenerWrapper(serviceListener net.Listener, alpnListener net.Listener) *ListenerMuxWrapper {
	return &ListenerMuxWrapper{
		alpnListener: alpnListener,
		Listener:     serviceListener,
		connC:        make(chan net.Conn),
		errC:         make(chan error),
		close:        make(chan struct{}),
	}
}

// HandleConnection allows to injection connection to the listener.
func (l *ListenerMuxWrapper) HandleConnection(ctx context.Context, conn net.Conn) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case l.connC <- conn:
		return nil
	}
}

func (l *ListenerMuxWrapper) Addr() net.Addr {
	if l.alpnListener != nil {
		return l.alpnListener.Addr()
	}
	return l.Listener.Addr()
}

func (l *ListenerMuxWrapper) Accept() (net.Conn, error) {
	go l.startAcceptingConnectionServiceListener()
	select {
	case <-l.close:
		return nil, trace.ConnectionProblem(nil, "listener is closed")
	case err := <-l.errC:
		return nil, err
	case conn := <-l.connC:
		return conn, nil
	}
}

func (l *ListenerMuxWrapper) startAcceptingConnectionServiceListener() {
	if l.Listener == nil {
		return
	}
	for {
		select {
		case <-l.close:
			return
		default:
		}
		conn, err := l.Listener.Accept()
		if err != nil {
			l.errC <- err
			return
		}
		l.connC <- conn
	}
}

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
