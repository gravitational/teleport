// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package vnet

import (
	"context"
	"crypto/x509"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	vnetv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/v1"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
)

// localProxyConfig holds the parameters needed to create a LocalProxy for VNet.
type localProxyConfig struct {
	dialOptions              *vnetv1.DialOptions
	protocols                []alpncommon.Protocol
	parentContext            context.Context
	middleware               alpnproxy.LocalProxyMiddleware
	clock                    clockwork.Clock
	alwaysTrustRootClusterCA bool
}

// newLocalProxy creates a new [alpnproxy.LocalProxy] configured for VNet
// use. It handles the common setup of dial options, ALPN protocols, and the
// optional RootCAs configuration for ALPN connection upgrades.
func newLocalProxy(cfg localProxyConfig) (*alpnproxy.LocalProxy, error) {
	dialOptions := cfg.dialOptions
	proxyConfig := alpnproxy.LocalProxyConfig{
		RemoteProxyAddr:         dialOptions.GetWebProxyAddr(),
		Protocols:               cfg.protocols,
		ParentContext:           cfg.parentContext,
		SNI:                     dialOptions.GetSni(),
		ALPNConnUpgradeRequired: dialOptions.GetAlpnConnUpgradeRequired(),
		Middleware:              cfg.middleware,
		InsecureSkipVerify:      dialOptions.GetInsecureSkipVerify(),
		Clock:                   cfg.clock,
	}
	if dialOptions.GetAlpnConnUpgradeRequired() || cfg.alwaysTrustRootClusterCA {
		certPoolPEM := dialOptions.GetRootClusterCaCertPool()
		if len(certPoolPEM) == 0 {
			return nil, trace.BadParameter("ALPN conn upgrade required but no root CA cert pool provided")
		}
		caPool := x509.NewCertPool()
		if !caPool.AppendCertsFromPEM(certPoolPEM) {
			return nil, trace.Errorf("failed to parse root cluster CA certs")
		}
		proxyConfig.RootCAs = caPool
	}
	lp, err := alpnproxy.NewLocalProxy(proxyConfig)
	if err != nil {
		return nil, trace.Wrap(err, "creating local proxy")
	}
	return lp, nil
}

// localProxyMiddleware is a shared [alpnproxy.LocalProxyMiddleware]
// implementation. It delegates cert checking to a [client.CertChecker]
// and calls a connection callback for observability on each new connection.
type localProxyMiddleware struct {
	certChecker     *client.CertChecker
	onNewConnection func(ctx context.Context) error
}

func (m *localProxyMiddleware) OnNewConnection(ctx context.Context, lp *alpnproxy.LocalProxy) error {
	if err := m.certChecker.OnNewConnection(ctx, lp); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(m.onNewConnection(ctx))
}

func (m *localProxyMiddleware) OnStart(ctx context.Context, lp *alpnproxy.LocalProxy) error {
	return trace.Wrap(m.certChecker.OnStart(ctx, lp))
}
