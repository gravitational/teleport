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

func makefakeSocket() fakeSocket {
	return fakeSocket{
		closed:      make(chan int),
		connections: make(chan net.Conn),
	}
}

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
