package utils

import (
	"net"
	"time"
)

// TimeoutConn wraps an existing net.Conn and adds read/write timeouts
// for it, allowing to safely pass it into io.Copy()
//
// Usage example:
// tc := utils.ObeyTimeouts(conn, time.Second * 30, "ssh connection")
// io.Copy(tc, xxx)
//
type TimeoutConn struct {
	net.Conn
	TimeoutDuration time.Duration

	// Name is only useful for debugging/logging, it's a convenient
	// way to "name" every active connection
	Name string
}

func ObeyTimeouts(conn net.Conn, timeout time.Duration, name string) net.Conn {
	return &TimeoutConn{
		Conn:            conn,
		TimeoutDuration: timeout,
		Name:            name,
	}
}

func (tc *TimeoutConn) Read(p []byte) (n int, err error) {
	err = tc.Conn.SetReadDeadline(time.Now().Add(tc.TimeoutDuration))
	if err != nil {
		return 0, err
	}
	return tc.Conn.Read(p)
}

func (tc *TimeoutConn) Write(p []byte) (n int, err error) {
	err = tc.Conn.SetWriteDeadline(time.Now().Add(tc.TimeoutDuration))
	if err != nil {
		return 0, err
	}
	return tc.Conn.Write(p)
}
