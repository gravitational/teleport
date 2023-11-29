/*
Copyright 2023 Gravitational, Inc.

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

package tcpip

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/interval"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// DialThroughForwarder dials a remote tcp address via a local forwarder.
func DialThroughForwarder(sock string, addr string) (net.Conn, error) {
	conn, err := net.Dial("unix", sock)
	if err != nil {
		return nil, trace.Errorf("failed to dial forwarder socket: %v", err)
	}

	req := ForwardRequest{
		DestinationAddress: addr,
	}

	reqBytes, err := utils.FastMarshal(&req)
	if err != nil {
		conn.Close()
		return nil, trace.Errorf("failed to marshal forward request: %v", err)
	}

	if err := WriteLengthPrefixedMessage(conn, reqBytes); err != nil {
		conn.Close()
		return nil, trace.Errorf("failed to send forward request: %v", err)
	}

	rspBytes, err := ReadLengthPrefixedMessage(conn)
	if err != nil {
		conn.Close()
		return nil, trace.Errorf("failed to read forward response: %v", err)
	}

	var rsp ForwardResponse
	if err := utils.FastUnmarshal(rspBytes, &rsp); err != nil {
		conn.Close()
		return nil, trace.Errorf("failed to unmarshal forward response: %v", err)
	}

	if !rsp.Success {
		conn.Close()
		return nil, trace.Errorf("failed to dial forwarding target: %q", err)
	}

	// connection is now ready to forward
	return conn, nil
}

// SocketKeepaliver runs periodic keepalive operations against a forwarder socket.
type SocketKeepaliver struct {
	socket    string
	interval  time.Duration
	done      chan struct{}
	closeOnce sync.Once
}

// NewSocketKeepaliver sets up a keepaliver for a socket. The returned keepaliver does nothing
// until Run is called.
func NewSocketKeepaliver(socket string, interval time.Duration) *SocketKeepaliver {
	return &SocketKeepaliver{
		socket:   socket,
		interval: interval,
		done:     make(chan struct{}),
	}
}

// Run runs the keepalive process until closed.
func (s *SocketKeepaliver) Run() error {
	// iotimeout is used for all io related to pinging sockets to determine
	// if they are healthy. 9s is arbitrary, but reasonably generous as we
	// expect local socket io to be very fast and the ping operation is trivial.
	const iotimeout = 9 * time.Second

	checkInterval := interval.New(interval.Config{
		Duration:      s.interval,
		FirstDuration: utils.FullJitter(min(time.Second*30, s.interval)),
		Jitter:        retryutils.NewSeventhJitter(),
	})
	defer checkInterval.Stop()

	for {
		select {
		case <-checkInterval.Next():
			if _, err := os.Stat(s.socket); os.IsNotExist(err) {
				return fmt.Errorf("socket %q removed while keepaliver was active", s.socket)
			}
			if err := keepaliveSocket(s.socket, iotimeout); err != nil {
				// error may be transient, so we just log it and carry on
				logrus.Warnf("Failed to keepalive socket %q: %v", s.socket, err)
			}
		case <-s.done:
			return nil
		}
	}
}

// Close terminates the keepalive process.
func (s *SocketKeepaliver) Close() error {
	s.closeOnce.Do(func() {
		close(s.done)
	})
	return nil
}

// SocketCleaner runs periodic cleanup operations against a forwarding socket directory. Note that
// this heler will close *any* socket in the target directory that has the suffix '.sock' but does
// not successfully handle the ping protocol used in this package. Sockets that implement protocols
// other than those defined in this package shouldn't be placed in a directory watched by a cleaner.
type SocketCleaner struct {
	dir       string
	interval  time.Duration
	done      chan struct{}
	closeOnce sync.Once
}

// NewSocketCleaner sets up a socket cleaner for the given directory. The returned cleaner
// does nothing until Run is called.
func NewSocketCleaner(dir string, interval time.Duration) *SocketCleaner {
	return &SocketCleaner{
		dir:      dir,
		interval: interval,
		done:     make(chan struct{}),
	}
}

// Run runs the cleanup process until closed.
func (s *SocketCleaner) Run() error {
	// iotimeout is used for all io related to pinging sockets to determine
	// if they are healthy. 9s is arbitrary, but reasonably generous as we
	// expect local socket io to be very fast and the ping operation is trivial.
	const iotimeout = 9 * time.Second

	checkInterval := interval.New(interval.Config{
		Duration:      s.interval,
		FirstDuration: utils.FullJitter(min(time.Second*30, s.interval)),
		Jitter:        retryutils.NewSeventhJitter(),
	})
	defer checkInterval.Stop()

	// state tracks unhealthy sockets for future cleanup
	var state map[string]struct{}

	for {
		select {
		case <-checkInterval.Next():
			if _, err := os.Stat(s.dir); os.IsNotExist(err) {
				// socket dir is created lazily
				continue
			}
			var err error
			state, err = cleanSocks(s.dir, state, iotimeout)
			if err != nil {
				logrus.Warnf("Unexpected error during cleanup of socket dir %q: %v", s.dir, err)
			}
		case <-s.done:
			return nil
		}
	}
}

// Close terminates the cleanup process.
func (s *SocketCleaner) Close() error {
	s.closeOnce.Do(func() {
		close(s.done)
	})
	return nil
}

// ForwardRequest is the payload sent from parent to child when starting a new forwarding connection.
type ForwardRequest struct {
	// DestinationAddress is the target address to dial to.
	DestinationAddress string `json:"dst_addr"`
}

// ForwardResponse indicates success/failure in dialing the target address.
type ForwardResponse struct {
	// Success is true if the target address was successfully dialed.
	Success bool `json:"success"`
	// Error is the error string associated with a failed dial.
	Error string `json:"error"`
}

// Forwarder implements the core logic of the forwarding child process. It is designed to
// sit behind a socket, serving tcpip forward requests passed to it from the parent.
type Forwarder struct {
	listener          net.Listener
	inactivityTimeout time.Duration
	lastActive        atomic.Pointer[time.Time]
	inflight          atomic.Int64
	closeOnce         sync.Once
	done              chan struct{}
}

// NewForwarder builds a forwarder. The passed in file descriptor must point to a valid
// listener (presumably passed in from the parent process). The inactivityTimeout determines
// roughly how long the forwarder should continue listening for incoming connections after
// the last time it was active.
func NewForwarder(fd *os.File, inactivityTimeout time.Duration) (*Forwarder, error) {
	listener, err := ListenerFromFD(fd)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Forwarder{
		listener:          listener,
		inactivityTimeout: inactivityTimeout,
		done:              make(chan struct{}),
	}, nil
}

// Run runs forwarding operations until the listener is closed.
func (f *Forwarder) Run() error {
	defer f.Close()
	if f.inactivityTimeout != 0 {
		go f.enforceInactivityTimeout()
	}

	return f.forward()
}

// Close halts the forwarder.
func (f *Forwarder) Close() error {
	f.closeOnce.Do(func() {
		close(f.done)
		f.listener.Close()
	})
	return nil
}

func (f *Forwarder) enforceInactivityTimeout() {
	defer f.Close()
	for {
		select {
		case <-time.After(f.inactivityTimeout / 3):
			if f.inflight.Load() != 0 {
				// currently active
				continue
			}

			lastActive, ok := f.getLastActive()
			if !ok {
				// no requests have come through yet, so treat
				// this as the start time of inactivity.
				f.updateLastActive()
				continue
			}

			if elapsed := time.Since(lastActive); elapsed >= f.inactivityTimeout {
				logrus.Warnf("Closing forwarder due to inactivity. (elapsed: %v)", elapsed)
				return
			}

		case <-f.done:
			return
		}
	}
}

func (f *Forwarder) forward() error {
	for {
		conn, err := f.listener.Accept()
		if err != nil {
			select {
			case <-f.done:
				// closed explicitly
				return nil
			default:
				// closing due to error
				return trace.Wrap(err)
			}
		}

		go f.forwardConn(conn)
	}
}

func (f *Forwarder) getLastActive() (t time.Time, ok bool) {
	l := f.lastActive.Load()
	if l == nil {
		return time.Time{}, false
	}
	return *l, !l.IsZero()
}

func (f *Forwarder) updateLastActive() {
	now := time.Now()
	f.lastActive.Store(&now)
}

func (f *Forwarder) forwardConn(conn net.Conn) {
	defer conn.Close()

	f.inflight.Add(1)
	defer f.inflight.Add(-1)

	// read the forward request
	reqBytes, err := ReadLengthPrefixedMessage(conn)
	if err != nil {
		if errors.Is(err, io.EOF) {
			// opening and then immediately closing a connection
			// is how we do health-checks on sockets, so its not
			// a true error.
			return
		}
		logrus.Warnf("Failed to read forward request: %v", err)
		return
	}

	if bytes.HasPrefix(reqBytes, controlMsgPrefix) {
		// we support a few very basic control messages
		// for health check and keepalive.

		var rsp []byte
		switch {
		case bytes.Equal(reqBytes, pingMsg):
			// nothing to do, just a healthcheck
			rsp = pongMsg
		case bytes.Equal(reqBytes, keepaliveMsg):
			// keepalive messages update our 'last active' time as if
			// they were real forwards (indicates that parent is healthy
			// and wants the forwarder to persist).
			f.updateLastActive()
			rsp = keepaliveAck
		default:
			logrus.Warnf("Unknown control msg %q (this is a bug).", reqBytes)
			rsp = unknownMsg
		}

		if err := WriteLengthPrefixedMessage(conn, rsp); err != nil {
			logrus.Warnf("Failed to respond to control message %q: %v", reqBytes, err)
		}
		return
	}

	var req ForwardRequest
	if err := utils.FastUnmarshal(reqBytes, &req); err != nil {
		logrus.Warnf("Failed to decode forward request: %v", err)
		return
	}

	if req.DestinationAddress == "" {
		logrus.Warn("Invalid forward request (missing destination address).")
		return
	}

	// treat any non-empty forward request as activity, even if the address
	// doesn't end up being dialable.
	defer f.updateLastActive()

	// dial the destination and build response message
	var rsp ForwardResponse
	fconn, err := net.Dial("tcp", req.DestinationAddress)
	if err != nil {
		rsp.Success = false
		rsp.Error = err.Error()
	} else {
		rsp.Success = true
	}

	rspBytes, err := utils.FastMarshal(&rsp)
	if err != nil {
		logrus.Warnf("Failed to encode forward response: %v", err)
		return
	}

	if err := WriteLengthPrefixedMessage(conn, rspBytes); err != nil {
		if errors.Is(err, io.EOF) {
			return
		}
		logrus.Warnf("Failed to write forward response: %v", err)
		return
	}

	if !rsp.Success {
		// dial failed and failure has been successfully propagated to
		// parent. no more work to do.
		return
	}

	if err := utils.ProxyConn(context.Background(), conn, fconn); err != nil {
		if errors.Is(err, io.EOF) {
			return
		}

		logrus.Warnf("Failure during conn forwarding: %v", err)
		return
	}
}
