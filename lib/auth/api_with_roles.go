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

package auth

import (
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/recorder"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"

	"golang.org/x/crypto/ssh"
)

type APIWithRoles struct {
	config      APIConfig
	listeners   map[teleport.Role]*fakeSocket
	servers     map[teleport.Role]*APIServer
	askedToStop bool
}

// APIConfig is a configuration file
type APIConfig struct {
	AuthServer        *AuthServer
	EventLog          events.Log
	SessionService    session.Service
	Recorder          recorder.Recorder
	Roles             []teleport.Role
	PermissionChecker PermissionChecker
}

func NewAPIWithRoles(config APIConfig) *APIWithRoles {
	api := APIWithRoles{}
	api.listeners = make(map[teleport.Role]*fakeSocket)
	api.servers = make(map[teleport.Role]*APIServer)
	api.config = config

	for _, role := range config.Roles {
		a := AuthWithRoles{
			authServer:  config.AuthServer,
			elog:        config.EventLog,
			sessions:    config.SessionService,
			recorder:    config.Recorder,
			permChecker: config.PermissionChecker,
			role:        role,
		}
		api.servers[role] = NewAPIServer(&a)
		api.listeners[role] = makefakeSocket()
	}
	return &api
}

func (api *APIWithRoles) Serve() {
	wg := sync.WaitGroup{}
	for role := range api.listeners {
		wg.Add(1)
		go func(listener net.Listener, handler http.Handler) {
			defer wg.Done()
			if err := http.Serve(listener, handler); (err != nil) && (err != io.EOF) {
				if !api.askedToStop {
					log.Errorf(err.Error())
				}
			}
		}(api.listeners[role], api.servers[role])
	}
	wg.Wait()
	log.Infof("[AUTH] API exited")
}

func (api *APIWithRoles) Close() {
	api.askedToStop = true
	var err error

	for _, listener := range api.listeners {
		listener.Close()
	}
	if api.config.EventLog != nil {
		if err = api.config.EventLog.Close(); err != nil {
			log.Error(err)
		}
	}
	if api.config.Recorder != nil {
		if err = api.config.Recorder.Close(); err != nil {
			log.Error(err)
		}
	}
}

// HandleNewChannel is called when a new SSH channel (SSH connection) wants to communicate via HTTP API
// to one of the API servers
func (api *APIWithRoles) HandleNewChannel(remoteAddr net.Addr, channel ssh.Channel, role teleport.Role) error {
	// find a listener mapped to the requested role:
	listener, ok := api.listeners[role]
	if !ok {
		channel.Close()
		return trace.Errorf("no such role '%v'", role)
	}
	// create a bridge between the incoming SSH channel to the HTTP-based API server
	return listener.CreateBridge(remoteAddr, channel)
}

// Implements a fake "socket" (net.Listener interface) on top of exisitng ssh.Channel
type fakeSocket struct {
	closed      chan int
	connections chan net.Conn
	closeOnce   sync.Once
}

func makefakeSocket() *fakeSocket {
	return &fakeSocket{
		closed:      make(chan int),
		connections: make(chan net.Conn),
	}
}

type FakeSSHConnection struct {
	remoteAddr net.Addr
	sshChan    ssh.Channel
	closeOnce  sync.Once
	closed     chan int
}

func (conn *FakeSSHConnection) Read(b []byte) (n int, err error) {
	return conn.sshChan.Read(b)
}

func (conn *FakeSSHConnection) Write(b []byte) (n int, err error) {
	return conn.sshChan.Write(b)
}

func (conn *FakeSSHConnection) Close() error {
	// broadcast the closing signal to all waiting parties
	conn.closeOnce.Do(func() {
		close(conn.closed)
	})
	return trace.Wrap(conn.sshChan.Close())
}

func (conn *FakeSSHConnection) RemoteAddr() net.Addr {
	return conn.remoteAddr
}

func (conn *FakeSSHConnection) LocalAddr() net.Addr {
	return &utils.NetAddr{AddrNetwork: "tcp", Addr: "socket.over.ssh"}
}

// SetDeadline is needed to implement net.Conn interface
func (conn *FakeSSHConnection) SetDeadline(t time.Time) error {
	return nil
}

// SetReadDeadline is needed to implement net.Conn interface
func (conn *FakeSSHConnection) SetReadDeadline(t time.Time) error {
	return nil
}

// SetWriteDeadline is needed to implement net.Conn interface
func (conn *FakeSSHConnection) SetWriteDeadline(t time.Time) error {
	return nil
}

// CreateBridge takes an incoming SSH connection and creates an SSH-to-HTTP "bridge connection"
// and waits for that connection to be closed either by the client or by the server
func (socket *fakeSocket) CreateBridge(remoteAddr net.Addr, sshChan ssh.Channel) error {
	if sshChan == nil {
		return trace.Wrap(teleport.BadParameter("sshChan", "supply ssh channel"))
	}
	// wrap sshChan into a 'fake connection' which allows us to
	//   a) preserve the original address of the connected client
	//   b) sit and wait until client closes the ssh channel, so we'll close this fake socket
	connection := &FakeSSHConnection{
		remoteAddr: remoteAddr,
		sshChan:    sshChan,
		closed:     make(chan int),
	}
	select {
	// Accept() will unblock this select
	case socket.connections <- connection:
	}
	// wait for the connection to close:
	select {
	case <-connection.closed:
	}
	return nil
}

// Accept waits for new connections to arrive (via CreateBridge) and returns them to
// the blocked http.Serve()
func (socket *fakeSocket) Accept() (c net.Conn, err error) {
	select {
	case newConnection := <-socket.connections:
		return newConnection, nil
	case <-socket.closed:
		return nil, io.EOF
	}
}

// Close closes the listener.
// Any blocked Accept operations will be unblocked and return errors.
func (socket *fakeSocket) Close() error {
	socket.closeOnce.Do(func() {
		// broadcast that listener has closed to all listening parties
		close(socket.closed)
	})
	return nil
}

// Addr returns the listener's network address.
func (socket *fakeSocket) Addr() net.Addr {
	return &utils.NetAddr{AddrNetwork: "tcp", Addr: "socket.over.ssh"}
}
