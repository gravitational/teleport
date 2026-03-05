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
	"crypto/tls"
	"crypto/x509"
	"log/slog"
	"net"
	"sync"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/sync/singleflight"

	"github.com/gravitational/teleport"
	vnetv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/v1"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
)

type dbTunnelHandler struct {
	cfg *dbTunnelHandlerConfig
	log *slog.Logger

	// mu guards localProxy.
	mu         sync.Mutex
	localProxy *alpnproxy.LocalProxy
}

type dbTunnelHandlerConfig struct {
	matchedDB                *vnetv1.MatchedDatabase
	dbProvider               *dbProvider
	clock                    clockwork.Clock
	alwaysTrustRootClusterCA bool
}

func newDBTunnelHandler(cfg *dbTunnelHandlerConfig) *dbTunnelHandler {
	return &dbTunnelHandler{
		cfg: cfg,
		log: log.With(
			teleport.ComponentKey, teleport.Component("vnet", "db-tunnel-handler"),
			"profile", cfg.matchedDB.GetProfile(),
			"db_service", cfg.matchedDB.GetDbServiceName(),
			"db_user", cfg.matchedDB.GetDbUser(),
		),
	}
}

func (h *dbTunnelHandler) handleTCPConnector(ctx context.Context, localPort uint16, connector func() (net.Conn, error)) error {
	lp, err := h.getOrInitLocalProxy(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(lp.HandleTCPConnector(ctx, connector), "handling DB tunnel TCP connector")
}

// getOrInitLocalProxy lazily initializes the local proxy on the first
// connection. We must defer this because we need the database protocol
// to configure the correct ALPN protocol.
func (h *dbTunnelHandler) getOrInitLocalProxy(ctx context.Context) (*alpnproxy.LocalProxy, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.localProxy != nil {
		return h.localProxy, nil
	}

	dbKey := &vnetv1.DBKey{
		Profile:       h.cfg.matchedDB.GetProfile(),
		LeafCluster:   h.cfg.matchedDB.GetLeafCluster(),
		DbServiceName: h.cfg.matchedDB.GetDbServiceName(),
		DbUser:        h.cfg.matchedDB.GetDbUser(),
		DbProtocol:    h.cfg.matchedDB.GetDbProtocol(),
	}

	// Use the protocol from the matched database if available (populated at
	// DNS resolution time by embedded VNet). Otherwise issue an initial cert
	// to discover the protocol, which is the case for the split-process VNet.
	dbProtocol := h.cfg.matchedDB.GetDbProtocol()
	if dbProtocol == "" {
		h.log.DebugContext(ctx, "Issuing initial DB cert to determine protocol")
		var err error
		_, dbProtocol, err = h.cfg.dbProvider.ReissueDBCert(ctx, dbKey, "")
		if err != nil {
			return nil, trace.Wrap(err, "issuing initial DB cert")
		}
	}
	alpnProtocol, err := alpncommon.ToALPNProtocol(dbProtocol)
	if err != nil {
		return nil, trace.Wrap(err, "unsupported database protocol %q", dbProtocol)
	}
	h.log.DebugContext(ctx, "Creating DB tunnel local proxy", "alpn_protocol", alpnProtocol)

	certIssuer := &dbCertIssuer{
		dbProvider: h.cfg.dbProvider,
		dbKey:      dbKey,
	}
	certChecker := client.NewCertChecker(certIssuer, h.cfg.clock)

	dialOptions := h.cfg.matchedDB.GetDialOptions()
	localProxyConfig := alpnproxy.LocalProxyConfig{
		RemoteProxyAddr:         dialOptions.GetWebProxyAddr(),
		Protocols:               []alpncommon.Protocol{alpnProtocol},
		ParentContext:           ctx,
		SNI:                     dialOptions.GetSni(),
		ALPNConnUpgradeRequired: dialOptions.GetAlpnConnUpgradeRequired(),
		Middleware:              certChecker,
		InsecureSkipVerify:      dialOptions.GetInsecureSkipVerify(),
		Clock:                   h.cfg.clock,
	}
	if dialOptions.GetAlpnConnUpgradeRequired() || h.cfg.alwaysTrustRootClusterCA {
		certPoolPEM := dialOptions.GetRootClusterCaCertPool()
		if len(certPoolPEM) == 0 {
			return nil, trace.BadParameter("ALPN conn upgrade required but no root CA cert pool provided")
		}
		caPool := x509.NewCertPool()
		if !caPool.AppendCertsFromPEM(certPoolPEM) {
			return nil, trace.Errorf("failed to parse root cluster CA certs")
		}
		localProxyConfig.RootCAs = caPool
	}
	lp, err := alpnproxy.NewLocalProxy(localProxyConfig)
	if err != nil {
		return nil, trace.Wrap(err, "creating local proxy for DB tunnel")
	}
	h.localProxy = lp
	return lp, nil
}

// dbCertIssuer implements [client.CertIssuer] for database tunnel certificates.
type dbCertIssuer struct {
	dbProvider *dbProvider
	dbKey      *vnetv1.DBKey
	group      singleflight.Group
}

func (i *dbCertIssuer) CheckCert(cert *x509.Certificate) error {
	// Basic expiry check is handled by the CertChecker wrapper.
	return nil
}

func (i *dbCertIssuer) IssueCert(ctx context.Context) (tls.Certificate, error) {
	cert, _, err := i.dbProvider.ReissueDBCert(ctx, i.dbKey, "")
	return cert, trace.Wrap(err)
}
