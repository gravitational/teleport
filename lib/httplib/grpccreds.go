/*
Copyright 2020 Gravitational, Inc.

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

package httplib

import (
	"context"
	"crypto/tls"
	"net"
	"syscall"

	"google.golang.org/grpc/credentials"

	"github.com/gravitational/trace"
)

// TLSCreds is the credentials required for authenticating a connection using TLS.
type TLSCreds struct {
	// TLS configuration
	Config *tls.Config
}

// Info returns protocol info
func (c TLSCreds) Info() credentials.ProtocolInfo {
	return credentials.ProtocolInfo{
		SecurityProtocol: "tls",
		SecurityVersion:  "1.2",
		ServerName:       c.Config.ServerName,
	}
}

// ClientHandshake callback is called to perform client handshake on the tls conn
func (c *TLSCreds) ClientHandshake(ctx context.Context, authority string, rawConn net.Conn) (_ net.Conn, _ credentials.AuthInfo, err error) {
	return nil, nil, trace.NotImplemented("client handshakes are not supported")
}

// ServerHandshake callback is called to perform server TLS handshake
// this wrapper makes sure that the connection is already tls and
// handshake has been performed
func (c *TLSCreds) ServerHandshake(rawConn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	tlsConn, ok := rawConn.(*tls.Conn)
	if !ok {
		return nil, nil, trace.BadParameter("expected TLS connection")
	}
	return WrapSyscallConn(rawConn, tlsConn), credentials.TLSInfo{State: tlsConn.ConnectionState()}, nil
}

// Clone clones transport credentials
func (c *TLSCreds) Clone() credentials.TransportCredentials {
	return &TLSCreds{
		Config: c.Config.Clone(),
	}
}

// OverrideServerName overrides server name in the TLS config
func (c *TLSCreds) OverrideServerName(serverNameOverride string) error {
	c.Config.ServerName = serverNameOverride
	return nil
}

type sysConn = syscall.Conn

// syscallConn keeps reference of rawConn to support syscall.Conn for channelz.
// SyscallConn() (the method in interface syscall.Conn) is explicitly
// implemented on this type,
//
// Interface syscall.Conn is implemented by most net.Conn implementations (e.g.
// TCPConn, UnixConn), but is not part of net.Conn interface. So wrapper conns
// that embed net.Conn don't implement syscall.Conn. (Side note: tls.Conn
// doesn't embed net.Conn, so even if syscall.Conn is part of net.Conn, it won't
// help here).
type syscallConn struct {
	net.Conn
	// sysConn is a type alias of syscall.Conn. It's necessary because the name
	// `Conn` collides with `net.Conn`.
	sysConn
}

// WrapSyscallConn tries to wrap rawConn and newConn into a net.Conn that
// implements syscall.Conn. rawConn will be used to support syscall, and newConn
// will be used for read/write.
//
// This function returns newConn if rawConn doesn't implement syscall.Conn.
func WrapSyscallConn(rawConn, newConn net.Conn) net.Conn {
	sysConn, ok := rawConn.(syscall.Conn)
	if !ok {
		return newConn
	}
	return &syscallConn{
		Conn:    newConn,
		sysConn: sysConn,
	}
}
