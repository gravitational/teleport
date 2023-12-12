// Teleport
// Copyright (C) 2023  Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package multiplexer

import (
	"bufio"
	"context"
	"net"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
)

type connectionLimiter interface {
	AcquireConnection(string) error
	ReleaseConnection(string)
}

func RunSingleplexer[B ~string | ~[]byte](ctx context.Context,
	listener net.Listener,
	handleConn func(net.Conn),
	earlyData B,
	getCA CertAuthorityGetter, clusterName string,
	limiter connectionLimiter,
) {
	listenCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		// unblock the Accept by closing the listener when the context is done
		<-listenCtx.Done()
		_ = listener.Close()
	}()

	for {
		c, err := listener.Accept()
		if err == nil {
			go handleSingleplexedConn(ctx, c, handleConn, earlyData, getCA, clusterName, limiter)
			continue
		}
		if listenCtx.Err() != nil || utils.IsUseOfClosedNetworkError(err) {
			break
		}
		backoff := 5 * time.Second
		if tErr, ok := err.(interface{ Temporary() bool }); ok && tErr.Temporary() {
			backoff = 100 * time.Millisecond
		}
		select {
		case <-listenCtx.Done():
			break
		case <-time.After(backoff):
		}
	}
}

func handleSingleplexedConn[B ~string | ~[]byte](ctx context.Context,
	c net.Conn,
	handleConn func(net.Conn),
	earlyData B,
	getCA CertAuthorityGetter, clusterName string,
	limiter connectionLimiter,
) {
	defer func() {
		if c != nil {
			c.Close()
		}
	}()

	// copied from [multiplexer.Mux.Serve()]
	if t, ok := c.(*net.TCPConn); ok {
		_ = t.SetKeepAlive(true)
		_ = t.SetKeepAlivePeriod(3 * time.Minute)
	}

	_ = c.SetDeadline(time.Now().Add(defaults.ReadHeadersTimeout))

	// XXX: this makes the same assumption regarding the availability of a small
	// write buffer that [ssh.NewServerConn] makes. It's not great as it limits
	// the use of synchronous connections like [net.Pipe], but doing it in
	// parallel makes the code quite a bit more complicated.
	if len(earlyData) > 0 {
		if _, err := c.Write([]byte(earlyData)); err != nil {
			return
		}
	}

	reader := bufio.NewReader(c)
	isProxyV2, err := readerHasPrefix(reader, ProxyV2Prefix)
	if err != nil {
		// errors on Peek(), almost surely I/O
		return
	}

	var remoteAddr net.Addr
	var limiterToken string

	if isProxyV2 {
		proxyline, err := ReadProxyLineV2(reader)
		if err != nil {
			// mostly I/O errors
			return
		}
		if proxyline == nil {
			// we shouldn't honor LOCAL proxylines
			return
		}
		if err := proxyline.VerifySignature(ctx,
			getCA, clusterName,
			clockwork.NewRealClock(),
		); err != nil {
			// bad signature
			return
		}
		remoteAddr = &proxyline.Source
		limiterToken = proxyline.Source.IP.String()
	} else if r := c.RemoteAddr(); r != nil {
		limiterToken = r.String()
		if host, _, err := utils.SplitHostPort(c.RemoteAddr().String()); err == nil {
			limiterToken = host
		}
	}

	if limiter != nil {
		if err := limiter.AcquireConnection(limiterToken); err != nil {
			return
		}
		defer limiter.ReleaseConnection(limiterToken)
	}

	_ = c.SetDeadline(time.Time{})

	wrapped := &singleplexedConn{
		Conn:       c,
		remoteAddr: remoteAddr,
		reader:     reader,
		skip:       len(earlyData),
	}

	// handing the connection over, disable the defer
	c = nil

	handleConn(wrapped)
}

type singleplexedConn struct {
	net.Conn

	remoteAddr net.Addr

	readMu sync.Mutex
	reader *bufio.Reader

	writeMu sync.Mutex
	skip    int
}

// Close implements [io.Closer] and [net.Conn].
func (c *singleplexedConn) Close() error {
	err := trace.Wrap(c.Conn.Close())

	c.readMu.Lock()
	defer c.readMu.Unlock()
	_, _ = c.reader.Discard(c.reader.Buffered())

	return err
}

// Read implements [io.Reader] and [net.Conn].
func (c *singleplexedConn) Read(b []byte) (int, error) {
	c.readMu.Lock()
	defer c.readMu.Unlock()

	return c.reader.Read(b)
}

// Write implements [io.Writer] and [net.Conn].
func (c *singleplexedConn) Write(b []byte) (int, error) {
	c.writeMu.Lock()
	if c.skip < 1 {
		c.writeMu.Unlock()
		return c.Conn.Write(b)
	}
	defer c.writeMu.Unlock()

	if len(b) <= c.skip {
		// check if the connection is open and not past the write deadline
		_, err := c.Conn.Write(nil)
		if err != nil {
			return 0, trace.Wrap(err)
		}
		c.skip -= len(b)
		return len(b), nil
	}

	b = b[c.skip:]
	n, err := c.Conn.Write(b)
	if n > 0 {
		n += c.skip
		c.skip = 0
	}
	return n, trace.Wrap(err)
}

// RemoteAddr implements [net.Conn].
func (c *singleplexedConn) RemoteAddr() net.Addr {
	if c.remoteAddr != nil {
		return c.remoteAddr
	}
	return c.Conn.RemoteAddr()
}

func readerHasPrefix[B ~[]byte | ~string](r *bufio.Reader, prefix B) (bool, error) {
	for i, b := range []byte(prefix) {
		buf, err := r.Peek(i + 1)
		if err != nil {
			return false, trace.Wrap(err)
		}
		if buf[i] != b {
			return false, nil
		}
	}
	return true, nil
}
