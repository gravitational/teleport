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

package client

import (
	"context"
	"net"
	"time"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/utils/sshutils"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

// ContextDialer represents network dialer interface that uses context
type ContextDialer interface {
	// DialContext is a function that dials the specified address
	DialContext(ctx context.Context, network, addr string) (net.Conn, error)
}

// ContextDialerFunc is a function wrapper that implements the ContextDialer interface
type ContextDialerFunc func(ctx context.Context, network, addr string) (net.Conn, error)

// DialContext is a function that dials to the specified address
func (f ContextDialerFunc) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	return f(ctx, network, addr)
}

// NewDialer makes a new dialer.
func NewDialer(keepAlivePeriod, dialTimeout time.Duration) ContextDialer {
	return &net.Dialer{
		Timeout:   dialTimeout,
		KeepAlive: keepAlivePeriod,
	}
}

// NewTunnelDialer make a new ssh tunnel dialer.
func NewTunnelDialer(ssh ssh.ClientConfig, keepAlivePeriod, dialTimeout time.Duration) ContextDialer {
	dialer := NewDialer(keepAlivePeriod, dialTimeout)
	return ContextDialerFunc(func(ctx context.Context, network, addr string) (conn net.Conn, err error) {
		conn, err = dialer.DialContext(ctx, network, addr)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		ssh.Timeout = dialTimeout
		sconn, err := sshutils.NewClientConnWithDeadline(conn, addr, &ssh)
		if err != nil {
			return nil, trace.NewAggregate(err, conn.Close())
		}

		// Build a net.Conn over the tunnel. Make this an exclusive connection:
		// close the net.Conn as well as the channel upon close.
		conn, _, err = sshutils.ConnectProxyTransport(sconn.Conn, &sshutils.DialReq{
			Address: constants.RemoteAuthServer,
		}, true)
		if err != nil {
			return nil, trace.NewAggregate(err, sconn.Close())
		}
		return conn, nil
	})
}
