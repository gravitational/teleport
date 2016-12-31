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
// for it, allowing to implement "disconnect after XX of idle time" policy
//
// Usage example:
// tc := utils.ObeyIdleTimeout(conn, time.Second * 30, "ssh connection")
// io.Copy(tc, xxx)
//
type TimeoutConn struct {
	net.Conn
	TimeoutDuration time.Duration

	// Name is only useful for debugging/logging, it's a convenient
	// way to tag every idle connection
	OwnerName string
}

// ObeyIdleTimeout wraps an existing network connection with timeout-obeying
// Write() and Read() - it will drop the connection after 'timeout' on idle
//
// Example:
// ObeyIdletimeout(conn, time.Second * 60, "api server").
func ObeyIdleTimeout(conn net.Conn, timeout time.Duration, ownerName string) net.Conn {
	return &TimeoutConn{
		Conn:            conn,
		TimeoutDuration: timeout,
		OwnerName:       ownerName,
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
