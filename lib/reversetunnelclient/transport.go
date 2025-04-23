/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package reversetunnelclient

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/sshutils"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/utils/proxy"
)

// NewTunnelAuthDialer creates a new instance of TunnelAuthDialer
func NewTunnelAuthDialer(config TunnelAuthDialerConfig) (*TunnelAuthDialer, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &TunnelAuthDialer{
		TunnelAuthDialerConfig: config,
	}, nil
}

// TunnelAuthDialerConfig specifies TunnelAuthDialer configuration.
type TunnelAuthDialerConfig struct {
	// Resolver retrieves the address of the proxy
	Resolver Resolver
	// ClientConfig is SSH tunnel client config
	ClientConfig *ssh.ClientConfig
	// Log is used for logging.
	Log *slog.Logger
	// InsecureSkipTLSVerify is whether to skip certificate validation.
	InsecureSkipTLSVerify bool
	// GetClusterCAs contains cluster CAs.
	GetClusterCAs client.GetClusterCAsFunc
}

func (c *TunnelAuthDialerConfig) CheckAndSetDefaults() error {
	switch {
	case c.Resolver == nil:
		return trace.BadParameter("missing tunnel address resolver")
	case c.GetClusterCAs == nil:
		return trace.BadParameter("missing cluster CA getter")
	case c.Log == nil:
		return trace.BadParameter("missing log")
	}
	return nil
}

// TunnelAuthDialer connects to the Auth Server through the reverse tunnel.
type TunnelAuthDialer struct {
	// TunnelAuthDialerConfig is the TunnelAuthDialer configuration.
	TunnelAuthDialerConfig
}

// DialContext dials auth server via SSH tunnel
func (t *TunnelAuthDialer) DialContext(ctx context.Context, _, _ string) (net.Conn, error) {
	// Connect to the reverse tunnel server.
	opts := []proxy.DialerOptionFunc{
		proxy.WithInsecureSkipTLSVerify(t.InsecureSkipTLSVerify),
	}

	addr, mode, err := t.Resolver(ctx)
	if err != nil {
		t.Log.ErrorContext(
			ctx, "Failed to resolve tunnel address",
			"error", err,
		)
		return nil, trace.Wrap(err)
	}

	if mode == types.ProxyListenerMode_Multiplex {
		opts = append(opts, proxy.WithALPNDialer(client.ALPNDialerConfig{
			TLSConfig: &tls.Config{
				NextProtos: []string{
					string(alpncommon.ProtocolReverseTunnelV2),
					string(alpncommon.ProtocolReverseTunnel),
				},
				InsecureSkipVerify: t.InsecureSkipTLSVerify,
			},
			DialTimeout:             t.ClientConfig.Timeout,
			ALPNConnUpgradeRequired: client.IsALPNConnUpgradeRequired(ctx, addr.Addr, t.InsecureSkipTLSVerify),
			GetClusterCAs:           t.GetClusterCAs,
		}))
	}

	dialer := proxy.DialerFromEnvironment(addr.Addr, opts...)
	sconn, err := dialer.Dial(ctx, addr.AddrNetwork, addr.Addr, t.ClientConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Build a net.Conn over the tunnel. Make this an exclusive connection:
	// close the net.Conn as well as the channel upon close.
	conn, _, err := sshutils.ConnectProxyTransport(
		sconn.Conn,
		&sshutils.DialReq{
			Address: RemoteAuthServer,
		},
		true,
	)
	if err != nil {
		return nil, trace.NewAggregate(err, sconn.Close())
	}
	return conn, nil
}
