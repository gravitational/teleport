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
// package auth implements certificate signing authority and access control server
// Authority server is composed of several parts:
//
// * Authority server itself that implements signing and acl logic
// * HTTP server wrapper for authority server
// * HTTP client wrapper
//

package utils

import (
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"
	"io"
	"net"
	"sync"
	"sync/atomic"
)

// Memory listener implements net.Listener using net.Conn
type MemoryListener struct {
	connections chan net.Conn
	state       chan int
	closed      uint32
}

func NewMemoryListener() *MemoryListener {
	ml := &MemoryListener{}
	ml.connections = make(chan net.Conn)
	ml.state = make(chan int)
	return ml
}

func (ml *MemoryListener) Accept() (net.Conn, error) {
	select {
	case newConnection := <-ml.connections:
		return newConnection, nil
	case <-ml.state:
		return nil, io.EOF
	}
}

func (ml *MemoryListener) Close() error {
	if atomic.CompareAndSwapUint32(&ml.closed, 0, 1) {
		close(ml.state)
	}
	return nil
}

func (ml *MemoryListener) Handle(conn net.Conn) error {
	select {
	case <-ml.state:
		return trace.Errorf("MemoryListener is closed")
	default:
	}

	local, remote := net.Pipe()

	wg := &sync.WaitGroup{}
	wg.Add(2)
	go func() {
		defer wg.Done()
		defer local.Close()
		io.Copy(local, conn)
	}()
	go func() {
		defer wg.Done()
		defer conn.Close()
		io.Copy(conn, local)
	}()
	ml.connections <- remote
	wg.Wait()

	return nil
}

func (ml *MemoryListener) Addr() net.Addr {
	addr := NetAddr{
		AddrNetwork: "tcp",
		Addr:        "memoryListenet",
	}
	return &addr
}
