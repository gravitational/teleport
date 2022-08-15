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
	"crypto/tls"
	"net"
	"time"

	"github.com/gravitational/teleport/api/client/proxy"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/constants"
	tracessh "github.com/gravitational/teleport/api/observability/tracing/ssh"
	"github.com/gravitational/teleport/api/utils/sshutils"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

// ContextDialer represents network dialer interface that uses context
type ContextDialer interface {
	// DialContext is a function that dials the specified address
	DialContext(ctx context.Context, network, addr string) (net.Conn, error)
}

// ContextDialerFunc is a function wrapper that implements the ContextDialer interface.
type ContextDialerFunc func(ctx context.Context, network, addr string) (net.Conn, error)

// DialContext is a function that dials to the specified address
func (f ContextDialerFunc) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	return f(ctx, network, addr)
}

// newDirectDialer makes a new dialer to connect directly to an Auth server.
func newDirectDialer(keepAlivePeriod, dialTimeout time.Duration) ContextDialer {
	return &net.Dialer{
		Timeout:   dialTimeout,
		KeepAlive: keepAlivePeriod,
	}
}

// DialerConfig is the config used for NewDialer.
type DialerConfig struct {
	// KeepAlivePeriod defines period between keep alives.
	KeepAlivePeriod time.Duration
	// DialTimeout defines how long to attempt dialing before timing out.
	DialTimeout time.Duration
	// ALPNConnUpgradeRequired specifies if ALPN connection upgrade is required.
	ALPNConnUpgradeRequired bool
	// InsecureSkipTLSVerify specifies whether to skip certificate validation
	// for TLS connections.
	InsecureSkipTLSVerify bool
}

// NewDialer makes a new dialer that connects to an Auth server either directly
// or via an ALPN connection upgrade tunnel, and on top of that the dialer may
// also connect through an HTTP proxy depending on the environment.
func NewDialer(config DialerConfig) ContextDialer {
	return ContextDialerFunc(func(ctx context.Context, network, addr string) (net.Conn, error) {
		dialer := newDirectDialer(config.KeepAlivePeriod, config.DialTimeout)
		if config.ALPNConnUpgradeRequired {
			dialer = newALPNConnUpgradeDialer(config.KeepAlivePeriod, config.DialTimeout, config.InsecureSkipTLSVerify)
		}

		if proxyURL := proxy.GetProxyURL(addr); proxyURL != nil {
			return DialProxyWithDialer(ctx, proxyURL, addr, dialer)
		}
		return dialer.DialContext(ctx, network, addr)
	})
}

// NewProxyDialer makes a dialer to connect to an Auth server through the SSH reverse tunnel on the proxy.
// The dialer will ping the web client to discover the tunnel proxy address on each dial.
func NewProxyDialer(ssh ssh.ClientConfig, keepAlivePeriod, dialTimeout time.Duration, discoveryAddr string, insecure bool) ContextDialer {
	dialer := newTunnelDialer(ssh, keepAlivePeriod, dialTimeout)
	return ContextDialerFunc(func(ctx context.Context, network, _ string) (conn net.Conn, err error) {
		tunnelAddr, err := webclient.GetTunnelAddr(
			&webclient.Config{Context: ctx, ProxyAddr: discoveryAddr, Insecure: insecure})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		conn, err = dialer.DialContext(ctx, network, tunnelAddr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return conn, nil
	})
}

// newTunnelDialer makes a dialer to connect to an Auth server through the SSH reverse tunnel on the proxy.
func newTunnelDialer(ssh ssh.ClientConfig, keepAlivePeriod, dialTimeout time.Duration) ContextDialer {
	dialer := newDirectDialer(keepAlivePeriod, dialTimeout)
	return ContextDialerFunc(func(ctx context.Context, network, addr string) (conn net.Conn, err error) {
		conn, err = dialer.DialContext(ctx, network, addr)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		sconn, err := sshConnect(ctx, conn, ssh, dialTimeout, addr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return sconn, nil
	})
}

// newTLSRoutingTunnelDialer makes a reverse tunnel TLS Routing dialer to connect to an Auth server
// through the SSH reverse tunnel on the proxy.
func newTLSRoutingTunnelDialer(ssh ssh.ClientConfig, keepAlivePeriod, dialTimeout time.Duration, discoveryAddr string, insecure, alpnConnUpgradeRequired bool) ContextDialer {
	return ContextDialerFunc(func(ctx context.Context, network, addr string) (net.Conn, error) {
		tunnelAddr, err := webclient.GetTunnelAddr(
			&webclient.Config{Context: ctx, ProxyAddr: discoveryAddr, Insecure: insecure})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		host, _, err := webclient.ParseHostPort(tunnelAddr)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		tlsDialer := NewTLSRoutingDialer(TLSRoutingDialerConfig{
			KeepAlivePeriod:         keepAlivePeriod,
			DialTimeout:             dialTimeout,
			ALPNConnUpgradeRequired: alpnConnUpgradeRequired,
			TLSConfig: &tls.Config{
				NextProtos:         []string{constants.ALPNSNIProtocolReverseTunnel},
				InsecureSkipVerify: insecure,
				ServerName:         host,
			},
		})
		tlsConn, err := tlsDialer.DialContext(ctx, network, tunnelAddr)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		sconn, err := sshConnect(ctx, tlsConn, ssh, dialTimeout, tunnelAddr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return sconn, nil
	})
}

// sshConnect upgrades the underling connection to ssh and connects to the Auth service.
func sshConnect(ctx context.Context, conn net.Conn, ssh ssh.ClientConfig, dialTimeout time.Duration, addr string) (net.Conn, error) {
	ssh.Timeout = dialTimeout
	sconn, err := tracessh.NewClientConnWithDeadline(ctx, conn, addr, &ssh)
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
}

// TLSRoutingDialerConfig is the config for TLSRoutingDialer.
type TLSRoutingDialerConfig struct {
	// KeepAlivePeriod defines period between keep alives.
	KeepAlivePeriod time.Duration
	// DialTimeout defines how long to attempt dialing before timing out.
	DialTimeout time.Duration
	// TLSConfig is the TLS config used for the TLS connection.
	TLSConfig *tls.Config
	// ALPNConnUpgradeRequired specifies if ALPN connection upgrade is required.
	ALPNConnUpgradeRequired bool
}

// TLSRoutingDialer is a ContextDialer that dials a connection to the Proxy
// Service that has TLS routing enabled (aka single-port mode).
type TLSRoutingDialer struct {
	TLSRoutingDialerConfig
}

// TLSRoutingDialer creates a new TLSRoutingDialer.
func NewTLSRoutingDialer(cfg TLSRoutingDialerConfig) ContextDialer {
	return &TLSRoutingDialer{
		TLSRoutingDialerConfig: cfg,
	}
}

// DialContext implements ContextDialer.
func (d TLSRoutingDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	if d.TLSConfig == nil {
		return nil, trace.BadParameter("missing TLS config")
	}

	netDialer := newDirectDialer(d.KeepAlivePeriod, d.DialTimeout)
	if d.ALPNConnUpgradeRequired {
		netDialer = newALPNConnUpgradeDialer(d.KeepAlivePeriod, d.DialTimeout, d.TLSConfig.InsecureSkipVerify)
	}

	netConn, err := netDialer.DialContext(ctx, network, addr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tlsConn := tls.Client(netConn, d.TLSConfig)
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		defer tlsConn.Close()
		return nil, trace.Wrap(err)
	}

	return tlsConn, nil
}
