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

package utils

import (
	"io"
	"net"
	"time"
)

// PipeNetConn implemetns net.Conn from io.Reader,io.Writer and io.Closer
type PipeNetConn struct {
	r          io.Reader
	w          io.Writer
	c          io.Closer
	localAddr  net.Addr
	remoteAddr net.Addr
}

func NewPipeNetConn(r io.Reader, w io.Writer, c io.Closer,
	fakelocalAddr net.Addr, fakeRemoteAddr net.Addr) *PipeNetConn {
	nc := PipeNetConn{
		r:          r,
		w:          w,
		c:          c,
		localAddr:  fakelocalAddr,
		remoteAddr: fakeRemoteAddr,
	}
	return &nc
}

func (nc *PipeNetConn) Read(buf []byte) (n int, e error) {
	return nc.r.Read(buf)
}

func (nc *PipeNetConn) Write(buf []byte) (n int, e error) {
	return nc.w.Write(buf)
}

func (nc *PipeNetConn) Close() error {
	return nc.c.Close()
}

func (nc *PipeNetConn) LocalAddr() net.Addr {
	return nc.localAddr
}

func (nc *PipeNetConn) RemoteAddr() net.Addr {
	return nc.remoteAddr
}

func (nc *PipeNetConn) SetDeadline(t time.Time) error {
	return nil
}

func (nc *PipeNetConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (nc *PipeNetConn) SetWriteDeadline(t time.Time) error {
	return nil
}

func SplitReaders(r1 io.Reader, r2 io.Reader) io.Reader {
	reader, writer := io.Pipe()
	go io.Copy(writer, r1)
	go io.Copy(writer, r2)
	return reader
}
