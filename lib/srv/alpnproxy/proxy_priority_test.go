/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package alpnproxy

import (
	"context"
	"crypto/tls"
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

// TestProxyDefaultALPNOrderPrioritizesHTTP1 guards the existing
// behaviour established by https://github.com/gravitational/teleport/pull/17886.
// A client that offers both h2 and http/1.1 must negotiate http/1.1 when
// the proxy runs with default settings, because Go's net/http does not yet
// implement WebSockets over HTTP/2 and Chrome had crbug 1379017.
func TestProxyDefaultALPNOrderPrioritizesHTTP1(t *testing.T) {
	t.Parallel()
	suite := NewSuite(t)

	noopHandler := func(ctx context.Context, conn net.Conn) error {
		<-ctx.Done()
		return nil
	}
	suite.router.Add(HandlerDecs{
		MatchFunc: MatchByProtocol(common.ProtocolHTTP),
		Handler:   noopHandler,
	})
	suite.router.Add(HandlerDecs{
		MatchFunc: MatchByProtocol(common.ProtocolHTTP2),
		Handler:   noopHandler,
	})
	suite.Start(t)

	conn, err := tls.Dial("tcp", suite.GetServerAddress(), &tls.Config{
		NextProtos: []string{
			string(common.ProtocolHTTP2),
			string(common.ProtocolHTTP),
		},
		ServerName: "localhost",
		RootCAs:    suite.GetCertPool(),
	})
	require.NoError(t, err)
	defer conn.Close()

	require.Equal(t, string(common.ProtocolHTTP),
		conn.ConnectionState().NegotiatedProtocol,
		"server must prefer http/1.1 over h2 by default to preserve WebSocket support")
}

// TestProxyPrioritizeHTTP2 verifies the opt-in flag puts h2 ahead of
// http/1.1 in the proxy's advertised ALPN list. The flag exists for
// customers like https://github.com/gravitational/teleport/issues/64717
// who want HTTP/2 multiplexing on apps that do not use WebSockets.
func TestProxyPrioritizeHTTP2(t *testing.T) {
	t.Parallel()
	suite := NewSuite(t)

	noopHandler := func(ctx context.Context, conn net.Conn) error {
		<-ctx.Done()
		return nil
	}
	suite.router.Add(HandlerDecs{
		MatchFunc: MatchByProtocol(common.ProtocolHTTP),
		Handler:   noopHandler,
	})
	suite.router.Add(HandlerDecs{
		MatchFunc: MatchByProtocol(common.ProtocolHTTP2),
		Handler:   noopHandler,
	})

	// Build the proxy with PrioritizeHTTP2 enabled.
	serverCert := mustGenCertSignedWithCA(t, suite.ca)
	tlsConfig := &tls.Config{
		NextProtos: common.ProtocolsToString(common.SupportedProtocols),
		ClientAuth: tls.VerifyClientCertIfGiven,
		ClientCAs:  suite.GetCertPool(),
		Certificates: []tls.Certificate{
			serverCert,
		},
	}

	proxyConfig := ProxyConfig{
		Listener:          suite.serverListener,
		WebTLSConfig:      tlsConfig,
		Router:            suite.router,
		Log:               logtest.NewLogger(),
		AccessPoint:       suite.accessPoint,
		IdentityTLSConfig: tlsConfig,
		ClusterName:       "root",
		PrioritizeHTTP2:   true,
	}
	svr, err := New(proxyConfig)
	require.NoError(t, err)
	svr.cfg.IdentityTLSConfig.GetConfigForClient = nil

	go func() {
		_ = svr.Serve(context.Background())
	}()
	t.Cleanup(func() { _ = svr.Close() })

	conn, err := tls.Dial("tcp", suite.GetServerAddress(), &tls.Config{
		NextProtos: []string{
			string(common.ProtocolHTTP2),
			string(common.ProtocolHTTP),
		},
		ServerName: "localhost",
		RootCAs:    suite.GetCertPool(),
	})
	require.NoError(t, err)
	defer conn.Close()

	require.Equal(t, string(common.ProtocolHTTP2),
		conn.ConnectionState().NegotiatedProtocol,
		"server must prefer h2 when PrioritizeHTTP2 is enabled")
}
