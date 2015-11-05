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
