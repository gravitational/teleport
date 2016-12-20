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

// ObeyTimeouts wraps an existing network connection with timeout-obeying
// Write() and Read()
func ObeyTimeouts(conn net.Conn, timeout time.Duration, name string) net.Conn {
	return &TimeoutConn{
		Conn:            conn,
		TimeoutDuration: timeout,
		Name:            name,
	}
}

func (tc *TimeoutConn) Read(p []byte) (n int, err error) {
	// note: checking for errors here does not buy anything: some net.Conn interface
	// 	     implementations (sshConn, pipe) simply return "not supported" error
	tc.Conn.SetReadDeadline(time.Now().Add(tc.TimeoutDuration))
	return tc.Conn.Read(p)
}

func (tc *TimeoutConn) Write(p []byte) (n int, err error) {
	// note: checking for errors here does not buy anything: some net.Conn interface
	// 	     implementations (sshConn, pipe) simply return "not supported" error
	tc.Conn.SetWriteDeadline(time.Now().Add(tc.TimeoutDuration))
	return tc.Conn.Write(p)
}
