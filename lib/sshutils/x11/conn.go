// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package x11

import (
	"io"
	"net"

	"github.com/gravitational/trace"
)

const (
	// x11MaxDisplays is the number of displays which the
	// server will support concurrent X11 forwarding for.
	x11MaxDisplays = 1000
)

// XServerConn is a connection to an XServer through a direct or forwarded connection.
// It can either be a tcp conn, unix conn, or an X11 channel.
type XServerConn interface {
	io.ReadWriteCloser
	// must implement CloseWrite to prevent read loop from halting pre-maturely
	CloseWrite() error
}

// XServerListener is a proxy XServer listener used to forward XServer requests.
// to an actualy XServer. The underlying listener may be a unix or tcp listener.
type XServerListener interface {
	// Accept waits for and returns the next connection to the listener.
	Accept() (XServerConn, error)

	// Close closes the listener.
	// Any blocked Accept operations will be unblocked and return errors.
	Close() error

	// Addr returns the listener's network address.
	Addr() net.Addr
}

type xserverUnixListener struct {
	*net.UnixListener
}

func (l *xserverUnixListener) Accept() (XServerConn, error) {
	return l.AcceptUnix()
}

type xserverTCPListener struct {
	*net.TCPListener
}

func (l *xserverTCPListener) Accept() (XServerConn, error) {
	return l.AcceptTCP()
}

// OpenNewXServerListener opens an XServerListener for the first available Display.
func OpenNewXServerListener(displayOffset int, screen uint32) (XServerListener, Display, error) {
	for displayNumber := displayOffset; displayNumber < displayOffset+x11MaxDisplays; displayNumber++ {
		display := Display{DisplayNumber: displayNumber}
		if l, err := display.Listen(); err == nil {
			return l, display, nil
		}
	}
	return nil, Display{}, trace.LimitExceeded("No more X11 sockets are available")
}
