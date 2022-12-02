//go:build !windows
// +build !windows

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

package utils

import (
	"net"
	"os"
	"syscall"

	"github.com/gravitational/trace"
)

// DualPipeNetConn creates a pipe to connect a client and a server. The
// two net.Conn instances are wrapped in an PipeNetConn which holds the source and
// destination addresses.
//
// The pipe is constructed from a syscall.Socketpair instead of a net.Pipe because
// the synchronous nature of net.Pipe causes it to deadlock when attempting to perform
// TLS or SSH handshakes.
func DualPipeNetConn(srcAddr net.Addr, dstAddr net.Addr) (net.Conn, net.Conn, error) {
	fds, err := syscall.Socketpair(syscall.AF_LOCAL, syscall.SOCK_STREAM, 0)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	f1 := os.NewFile(uintptr(fds[0]), srcAddr.String())
	defer f1.Close()

	f2 := os.NewFile(uintptr(fds[1]), dstAddr.String())
	defer f2.Close()

	client, err := net.FileConn(f1)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	server, err := net.FileConn(f2)
	if err != nil {
		return nil, nil, trace.NewAggregate(err, client.Close())
	}

	serverConn := NewPipeNetConn(server, server, server, dstAddr, srcAddr)
	clientConn := NewPipeNetConn(client, client, client, srcAddr, dstAddr)

	return serverConn, clientConn, nil
}
