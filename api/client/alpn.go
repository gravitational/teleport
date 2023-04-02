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
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/utils/pingconn"
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
}

// ALPNDialer is a ContextDialer that dials a connection to the Proxy Service
// with ALPN and SNI configured in the provided TLSConfig. An ALPN connection
// upgrade is also performed at the initial connection, if an upgrade is
// required. If the negotiated protocol is a Ping protocol, it will return the
// de-multiplexed connection without the Ping.
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
func (d *ALPNDialer) shouldUpdateServerName() bool {
	return d.cfg.TLSConfig.ServerName == ""
}
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

	if IsALPNPingProtocol(tlsConn.ConnectionState().NegotiatedProtocol) {
		logrus.Debugf("Using ping connection for protocol %v.", tlsConn.ConnectionState().NegotiatedProtocol)
		return pingconn.New(tlsConn), nil
	}
	return tlsConn, nil
}

// DialALPN a helper to dial using an ALPNDialer.
func DialALPN(ctx context.Context, addr string, cfg ALPNDialerConfig) (net.Conn, error) {
	conn, err := NewALPNDialer(cfg).DialContext(ctx, "tcp", addr)
	return conn, trace.Wrap(err)
}

// ALPNSNIProtocolPingSuffix receives an ALPN protocol and returns it with the
// Ping protocol suffix.
func ALPNProtocolWithPing(protocol string) string {
	return protocol + constants.ALPNSNIProtocolPingSuffix
}

// IsALPNPingProtocol checks if the provided protocol is suffixed with Ping.
func IsALPNPingProtocol(protocol string) bool {
	return strings.HasSuffix(protocol, constants.ALPNSNIProtocolPingSuffix)
}
