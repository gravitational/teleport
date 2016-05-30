/*
Copyright 2015 Gravitational, Inc.

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

package auth

import (
	"io"
	"net"
	"sync"
	"time"

	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

// Implements a fake "socket" (net.Listener interface) on top of exisitng ssh.Channel
type fakeSocket struct {
	closed      chan int
	connections chan net.Conn
	closeOnce   sync.Once
}

// FakeSSHConnection implements net.Conn interface on top of the ssh.Cnahhel
// object. This allows us to run non-SSH servers (like HTTP) on top of an
// existing SSH connection
type FakeSSHConnection struct {
	remoteAddr net.Addr
	sshChan    ssh.Channel
	closeOnce  sync.Once
	closed     chan int
}

func (conn *FakeSSHConnection) Read(b []byte) (n int, err error) {
	return conn.sshChan.Read(b)
}

func (conn *FakeSSHConnection) Write(b []byte) (n int, err error) {
	return conn.sshChan.Write(b)
}

func (conn *FakeSSHConnection) Close() error {
	// broadcast the closing signal to all waiting parties
	conn.closeOnce.Do(func() {
		close(conn.closed)
	})
	return trace.Wrap(conn.sshChan.Close())
}

func (conn *FakeSSHConnection) RemoteAddr() net.Addr {
	return conn.remoteAddr
}

func (conn *FakeSSHConnection) LocalAddr() net.Addr {
	return &utils.NetAddr{AddrNetwork: "tcp", Addr: "socket.over.ssh"}
}

// SetDeadline is needed to implement net.Conn interface
func (conn *FakeSSHConnection) SetDeadline(t time.Time) error {
	return nil
}

// SetReadDeadline is needed to implement net.Conn interface
func (conn *FakeSSHConnection) SetReadDeadline(t time.Time) error {
	return nil
}

// SetWriteDeadline is needed to implement net.Conn interface
func (conn *FakeSSHConnection) SetWriteDeadline(t time.Time) error {
	return nil
}

// Accept waits for new connections to arrive (via CreateBridge) and returns them to
// the blocked http.Serve()
func (socket *fakeSocket) Accept() (c net.Conn, err error) {
	select {
	case newConnection := <-socket.connections:
		return newConnection, nil
	case <-socket.closed:
		return nil, io.EOF
	}
}

// Close closes the listener.
// Any blocked Accept operations will be unblocked and return errors.
func (socket *fakeSocket) Close() error {
	socket.closeOnce.Do(func() {
		// broadcast that listener has closed to all listening parties
		close(socket.closed)
	})
	return nil
}

// Addr returns the listener's network address.
func (socket *fakeSocket) Addr() net.Addr {
	return &utils.NetAddr{AddrNetwork: "tcp", Addr: "socket.over.ssh"}
}
