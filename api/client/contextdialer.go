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
	"github.com/gravitational/teleport/api/sshutils"

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
func NewDialer(keepAliveInterval, dialTimeout time.Duration) ContextDialer {
	return &net.Dialer{
		Timeout:   dialTimeout,
		KeepAlive: keepAliveInterval,
	}
}

// NewAddrsDialer makes a new dialer from a list of addresses.
func NewAddrsDialer(addrs []string, keepAliveInterval, dialTimeout time.Duration) (ContextDialer, error) {
	if len(addrs) == 0 {
		return nil, trace.BadParameter("no addresses to dial")
	}
	dialer := NewDialer(keepAliveInterval, dialTimeout)
	return ContextDialerFunc(func(ctx context.Context, network, _ string) (conn net.Conn, err error) {
		for _, addr := range addrs {
			conn, err = dialer.DialContext(ctx, network, addr)
			if err == nil {
				return conn, nil
			}
		}
		// not wrapping on purpose to preserve the original error
		return nil, err
	}), nil
}

// NewTunnelDialer make a new ssh tunnel dialer
func NewTunnelDialer(ssh ssh.ClientConfig, keepAliveInterval, dialTimeout time.Duration) ContextDialer {
	dialer := NewDialer(keepAliveInterval, dialTimeout)
	return ContextDialerFunc(func(ctx context.Context, network, addr string) (conn net.Conn, err error) {
		conn, err = dialer.DialContext(ctx, network, addr)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		sconn, err := sshutils.NewClientConnWithDeadline(conn, addr, &ssh)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// Build a net.Conn over the tunnel. Make this an exclusive connection:
		// close the net.Conn as well as the channel upon close.
		conn, _, err = sshutils.ConnectProxyTransport(sconn.Conn, &sshutils.DialReq{
			Address: constants.RemoteAuthServer,
		}, true)
		if err != nil {
			err2 := sconn.Close()
			return nil, trace.NewAggregate(err, err2)
		}
		return conn, nil
	})
}
