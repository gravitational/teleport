/*
Copyright 2017 Gravitational, Inc.

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

package utils

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"sync/atomic"

	"github.com/gravitational/trace"
)

// NewCloserConn returns new connection wrapper that
// when closed will also close passed closers
func NewCloserConn(conn net.Conn, closers ...io.Closer) *CloserConn {
	return &CloserConn{
		Conn:    conn,
		closers: closers,
	}
}

// CloserConn wraps connection and attaches additional closers to it
type CloserConn struct {
	net.Conn
	closers []io.Closer
}

// AddCloser adds any closer in ctx that will be called
// whenever server closes session channel
func (c *CloserConn) AddCloser(closer io.Closer) {
	c.closers = append(c.closers, closer)
}

// Close closes connection and all closers
func (c *CloserConn) Close() error {
	var errors []error
	for _, closer := range c.closers {
		errors = append(errors, closer.Close())
	}
	errors = append(errors, c.Conn.Close())
	return trace.NewAggregate(errors...)
}

// Roundtrip is a single connection simplistic HTTP client
// that allows us to bypass a connection pool to test load balancing
// used in tests, as it only supports GET request on /
func Roundtrip(addr string) (string, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	return RoundtripWithConn(conn)
}

// RoundtripWithConn uses HTTP GET on the existing connection,
// used in tests as it only performs GET request on /
func RoundtripWithConn(conn net.Conn) (string, error) {
	_, err := fmt.Fprintf(conn, "GET / HTTP/1.1\r\nHost: 127.0.0.1\r\n\r\n")
	if err != nil {
		return "", err
	}

	re, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		return "", err
	}
	defer re.Body.Close()
	out, err := ioutil.ReadAll(re.Body)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// Stater is extension interface of the net.Conn for implementations that
// track connection statistics.
type Stater interface {
	// Stat returns TX, RX data.
	Stat() (uint64, uint64)
}

// TrackingConn is a net.Conn that keeps track of how much data was transmitted
// (TX) and received (RX) over the net.Conn. A maximum of about 18446
// petabytes can be kept track of for TX and RX before it rolls over.
// See https://golang.org/ref/spec#Numeric_types for more details.
type TrackingConn struct {
	// net.Conn is the underlying net.Conn.
	net.Conn

	// txBytes keeps track of how many bytes were transmitted.
	txBytes uint64

	// rxBytes keeps track of how many bytes were received.
	rxBytes uint64
}

// NewTrackingConn returns a net.Conn that can keep track of how much data was
// transmitted over it.
func NewTrackingConn(conn net.Conn) *TrackingConn {
	return &TrackingConn{
		Conn: conn,
	}
}

// Stat returns the transmitted (TX) and received (RX) bytes over the net.Conn.
func (s *TrackingConn) Stat() (uint64, uint64) {
	return atomic.LoadUint64(&s.txBytes), atomic.LoadUint64(&s.rxBytes)
}

func (s *TrackingConn) Read(b []byte) (n int, err error) {
	n, err = s.Conn.Read(b)
	atomic.AddUint64(&s.rxBytes, uint64(n))
	return n, trace.Wrap(err)
}

func (s *TrackingConn) Write(b []byte) (n int, err error) {
	n, err = s.Conn.Write(b)
	atomic.AddUint64(&s.txBytes, uint64(n))
	return n, trace.Wrap(err)
}
