/*
Copyright 2025 Gravitational, Inc.

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
package websocketupgradeproto

import (
	"cmp"
	"context"
	"crypto/tls"
	"log/slog"
	"net"
	"net/url"

	"github.com/gobwas/ws"
	"github.com/gravitational/trace"
)

// WebSocketALPNClientConnConfig holds the configuration for creating a
// WebSocket ALPN client connection.
// It includes the URL to connect to, a dialer function, TLS configuration,
// supported protocols, and a logger.
type WebSocketALPNClientConnConfig struct {
	// URL is the WebSocket server URL to connect to.
	// It should include the scheme (ws or wss) and the path.
	// Example: "wss://example.com/ws"
	URL *url.URL
	// Dialer is a function that dials the network and address.
	Dialer func(context.Context, string, string) (net.Conn, error)
	// TLSConfig is the TLS configuration to use for secure connections.
	TLSConfig *tls.Config
	// Protocols is a list of protocols to negotiate during the WebSocket handshake.
	Protocols []string
	// Logger is an optional logger for logging events.
	Logger *slog.Logger
}

// NewWebSocketALPNClientConn creates a new WebsocketUpgradeConn for the client side.
// It dials the WebSocket server using the provided URL and TLS configuration.
func NewWebSocketALPNClientConn(ctx context.Context, cfg WebSocketALPNClientConnConfig) (*Conn, error) {
	dialer := ws.Dialer{
		Protocols: cfg.Protocols,
		NetDial: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return cfg.Dialer(ctx, network, addr)
		},
		TLSConfig: cloneTLSConfigAndSetServerName(cfg.TLSConfig.Clone(), cfg.URL.Hostname()),
	}

	uCopy := *cfg.URL
	switch uCopy.Scheme {
	case "http":
		uCopy.Scheme = "ws"
	case "https":
		uCopy.Scheme = "wss"
	}
	conn, _, hs, err := dialer.Dial(ctx, uCopy.String())
	if err != nil {
		return nil, trace.Wrap(err, "failed to dial WebSocket")
	}

	if hs.Protocol == "" {
		_ = conn.Close()
		return nil, trace.BadParameter("WebSocket handshake failed: no protocol specified")
	}

	return newWebsocketUpgradeConn(newWebsocketUpgradeConnConfig{
		ctx:      ctx,
		conn:     conn,
		logger:   cmp.Or(cfg.Logger, slog.Default()),
		hs:       hs,
		connType: clientConnection,
	}), nil
}

func cloneTLSConfigAndSetServerName(tlsConfig *tls.Config, serverName string) *tls.Config {
	tlsConfig = tlsConfig.Clone()
	if tlsConfig == nil {
		tlsConfig = &tls.Config{}
	}
	if tlsConfig.ServerName == "" {
		tlsConfig.ServerName = serverName
	}
	return tlsConfig
}
