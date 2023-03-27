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

package alpnproxy

import (
	"context"
	"crypto/tls"
	"net"
	"time"

	"github.com/gravitational/trace"

	apiclient "github.com/gravitational/teleport/api/client"
)

// ContextDialer represents network dialer interface that uses context
type ContextDialer interface {
	// DialContext is a function that dials the specified address
	DialContext(ctx context.Context, network, addr string) (net.Conn, error)
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

// DialContext implements ContextDialer.
func (d ALPNDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	if d.cfg.TLSConfig == nil {
		return nil, trace.BadParameter("missing TLS config")
	}

	dialer := apiclient.NewDialer(ctx, d.cfg.DialTimeout, d.cfg.DialTimeout, apiclient.WithTLSConfig(d.cfg.TLSConfig))
	if d.cfg.ALPNConnUpgradeRequired {
		dialer = newALPNConnUpgradeDialer(dialer, &tls.Config{
			InsecureSkipVerify: d.cfg.TLSConfig.InsecureSkipVerify,
		})
	}

	conn, err := dialer.DialContext(ctx, network, addr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tlsConn := tls.Client(conn, d.cfg.TLSConfig)
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
