/*
Copyright 2022 Gravitational, Inc.

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
	"crypto/x509"
	"net"
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/constants"
)

// GetClusterCAsFunc is a function to fetch cluster CAs.
type GetClusterCAsFunc func(ctx context.Context) (*x509.CertPool, error)

// ClusterCAsFromCertPool returns a GetClusterCAsFunc with provided static cert
// pool.
func ClusterCAsFromCertPool(cas *x509.CertPool) GetClusterCAsFunc {
	return func(_ context.Context) (*x509.CertPool, error) {
		return cas, nil
	}
}

// ALPNDialerConfig is the config for ALPNDialer.
type ALPNDialerConfig struct {
	// KeepAlivePeriod defines period between keep alives.
	KeepAlivePeriod time.Duration
	// DialTimeout defines how long to attempt dialing before timing out.
	DialTimeout time.Duration
	// TLSConfig is the TLS config used for the TLS connection.
	TLSConfig *tls.Config
	// ALPNConnUpgradeRequired specifies if ALPN connection upgrade is required.
	ALPNConnUpgradeRequired bool
	// GetClusterCAs is an optional callback function to fetch cluster
	// CAs when connection upgrade is required. If not provided, it's assumed
	// the proper CAs are already present in TLSConfig.
	GetClusterCAs GetClusterCAsFunc
	// PROXYHeaderGetter is used if present to get signed PROXY headers to propagate client's IP.
	// Used by proxy's web server to make calls on behalf of connected clients.
	PROXYHeaderGetter PROXYHeaderGetter
}

// ALPNDialer is a ContextDialer that dials a connection to the Proxy Service
// with ALPN and SNI configured in the provided TLSConfig. An ALPN connection
// upgrade is also performed at the initial connection, if an upgrade is
// required.
type ALPNDialer struct {
	cfg ALPNDialerConfig
}

// NewALPNDialer creates a new ALPNDialer.
func NewALPNDialer(cfg ALPNDialerConfig) ContextDialer {
	return &ALPNDialer{
		cfg: cfg,
	}
}

func (d *ALPNDialer) shouldUpdateTLSConfig() bool {
	return d.shouldUpdateServerName() || d.shouldGetClusterCAs()
}

// shouldUpdateServerName returns true if ServerName is not in the provided TLS
// config. It will default to the host of the dialing address.
func (d *ALPNDialer) shouldUpdateServerName() bool {
	return d.cfg.TLSConfig.ServerName == ""
}

// shouldGetClusterCAs returns true if RootCAs of the provided TLS config needs
// to be set to the Teleport cluster CAs.
//
// When Teleport Proxy is behind a L7 load balancer, the load balancer
// usually terminates TLS with public certs, and the Proxy is usually in
// private subnets with self-signed web certs. During the connection
// upgrade flow for TLS Routing, instead of serving these self-signed web
// certs, the TLS Routing handler at the Proxy server will present the
// Cluster CAs so clients here can still verify the server.
func (d *ALPNDialer) shouldGetClusterCAs() bool {
	return d.cfg.ALPNConnUpgradeRequired && d.cfg.TLSConfig.RootCAs == nil && d.cfg.GetClusterCAs != nil
}

func (d *ALPNDialer) getTLSConfig(ctx context.Context, addr string) (*tls.Config, error) {
	if d.cfg.TLSConfig == nil {
		return nil, trace.BadParameter("missing TLS config")
	}
	if !d.shouldUpdateTLSConfig() {
		return d.cfg.TLSConfig, nil
	}

	var err error
	tlsConfig := d.cfg.TLSConfig.Clone()
	if d.shouldGetClusterCAs() {
		tlsConfig.RootCAs, err = d.cfg.GetClusterCAs(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if d.shouldUpdateServerName() {
		tlsConfig.ServerName, _, err = webclient.ParseHostPort(addr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return tlsConfig, nil
}

// DialContext implements ContextDialer.
func (d *ALPNDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	tlsConfig, err := d.getTLSConfig(ctx, addr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	dialer := NewDialer(ctx, d.cfg.DialTimeout, d.cfg.DialTimeout,
		WithInsecureSkipVerify(d.cfg.TLSConfig.InsecureSkipVerify),
		WithALPNConnUpgrade(d.cfg.ALPNConnUpgradeRequired),
		WithALPNConnUpgradePing(shouldALPNConnUpgradeWithPing(tlsConfig)),
		WithPROXYHeaderGetter(d.cfg.PROXYHeaderGetter),
	)

	conn, err := dialer.DialContext(ctx, network, addr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tlsConn := tls.Client(conn, tlsConfig)
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		defer tlsConn.Close()
		return nil, trace.Wrap(err)
	}
	return tlsConn, nil
}

// DialALPN a helper to dial using an ALPNDialer and returns a tls.Conn if
// successful.
func DialALPN(ctx context.Context, addr string, cfg ALPNDialerConfig) (*tls.Conn, error) {
	conn, err := NewALPNDialer(cfg).DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		return nil, trace.BadParameter("failed to convert to tls.Conn")
	}
	return tlsConn, nil
}

// IsALPNPingProtocol checks if the provided protocol is suffixed with Ping.
func IsALPNPingProtocol(protocol string) bool {
	return strings.HasSuffix(protocol, constants.ALPNSNIProtocolPingSuffix)
}

// shouldALPNConnUpgradeWithPing returns true if Ping wrapper is required
// during connection upgrade.
func shouldALPNConnUpgradeWithPing(config *tls.Config) bool {
	for _, proto := range config.NextProtos {
		switch proto {
		// Server usually sends SSH keepalives or HTTP2 pings every five
		// minutes for reverse tunnel and SSH connections. Load balancers
		// usually have a shorter idle timeout. Thus wrapping the connection
		// with Ping protocol at the connection upgrade layer to keepalive.
		case constants.ALPNSNIProtocolReverseTunnel,
			constants.ALPNSNIProtocolSSH:
			return true
		}
	}
	return false
}
