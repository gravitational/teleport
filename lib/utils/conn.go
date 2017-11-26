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
