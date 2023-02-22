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

package peer

import (
	"net"
	"time"

	streamutils "github.com/gravitational/teleport/api/utils/grpc/stream"
)

// streamConn wraps [streamutils.ReadWriter] in a [net.Conn] interface.
type streamConn struct {
	*streamutils.ReadWriter

	src net.Addr
	dst net.Addr
}

// newStreamConn creates a new streamConn.
func newStreamConn(rw *streamutils.ReadWriter, src net.Addr, dst net.Addr) *streamConn {
	return &streamConn{
		ReadWriter: rw,
		src:        src,
		dst:        dst,
	}
}

// LocalAddr is the original source address of the client.
func (c *streamConn) LocalAddr() net.Addr {
	return c.src
}

// RemoteAddr is the address of the reverse tunnel node.
func (c *streamConn) RemoteAddr() net.Addr {
	return c.dst
}

func (c *streamConn) SetDeadline(t time.Time) error {
	return nil
}

func (c *streamConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (c *streamConn) SetWriteDeadline(t time.Time) error {
	return nil
}
