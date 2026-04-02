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
	"github.com/gravitational/teleport/api/client/proto"
	vnetv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/v1"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/tlsca"
)

type tcpDBHandler struct {
	cfg *tcpDBHandlerConfig
	log *slog.Logger

	mu         sync.Mutex
	localProxy *alpnproxy.LocalProxy
}

type tcpDBHandlerConfig struct {
	dbInfo     *vnetv1.DatabaseInfo
	dbProvider *dbProvider
	clock      clockwork.Clock
	// alwaysTrustRootClusterCA can be set in tests so that TLS dials to the
	// proxy always trust the root cluster CA rather than the system cert pool,
	// even when ALPN conn upgrades are not required.
	alwaysTrustRootClusterCA bool
}

func newTCPDBHandler(cfg *tcpDBHandlerConfig) *tcpDBHandler {
	return &tcpDBHandler{
		cfg: cfg,
		log: log.With(
			teleport.ComponentKey, teleport.Component("vnet", "tcp-db-handler"),
			"profile", cfg.dbInfo.GetDatabaseKey().GetProfile(),
			"leaf_cluster", cfg.dbInfo.GetDatabaseKey().GetLeafCluster(),
			"db_name", cfg.dbInfo.GetDatabaseKey().GetName(),
			"db_user", cfg.dbInfo.GetUsername(),
			"protocol", cfg.dbInfo.GetProtocol()),
	}
}

func (h *tcpDBHandler) getOrInitializeLocalProxy(ctx context.Context) (*alpnproxy.LocalProxy, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.localProxy != nil {
		return h.localProxy, nil
	}

	alpnProtocol, err := alpncommon.ToALPNProtocol(h.cfg.dbInfo.GetProtocol())
	if err != nil {
		return nil, trace.Wrap(err, "mapping database protocol %q to ALPN protocol", h.cfg.dbInfo.GetProtocol())
	}

	dbCertIssuer := &dbCertIssuer{
		dbProvider: h.cfg.dbProvider,
		dbInfo:     h.cfg.dbInfo,
	}
	certChecker := client.NewCertChecker(dbCertIssuer, h.cfg.clock)
	middleware := &dbLocalProxyMiddleware{
		certChecker: certChecker,
		dbProvider:  h.cfg.dbProvider,
		dbKey:       h.cfg.dbInfo.GetDatabaseKey(),
	}

	dialOptions := h.cfg.dbInfo.GetDialOptions()
	localProxyConfig := alpnproxy.LocalProxyConfig{
		RemoteProxyAddr:         dialOptions.GetWebProxyAddr(),
		Protocols:               []alpncommon.Protocol{alpnProtocol},
		ParentContext:           ctx,
		SNI:                     dialOptions.GetSni(),
		ALPNConnUpgradeRequired: dialOptions.GetAlpnConnUpgradeRequired(),
		Middleware:              middleware,
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

	h.log.DebugContext(ctx, "Creating local proxy for database")
	lp, err := alpnproxy.NewLocalProxy(localProxyConfig)
	if err != nil {
		return nil, trace.Wrap(err, "creating local proxy")
	}
	h.localProxy = lp
	return lp, nil
}

// handleTCPConnector handles an incoming TCP connection from VNet by passing it
// to the local ALPN proxy, which is configured with middleware to automatically
// handle certificate renewal and re-logins.
func (h *tcpDBHandler) handleTCPConnector(ctx context.Context, localPort uint16, connector func() (net.Conn, error)) error {
	lp, err := h.getOrInitializeLocalProxy(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(lp.HandleTCPConnector(ctx, connector), "handling TCP connector")
}

// dbCertIssuer implements [client.CertIssuer] for VNet database access.
type dbCertIssuer struct {
	dbProvider *dbProvider
	dbInfo     *vnetv1.DatabaseInfo
	group      singleflight.Group
}

func (i *dbCertIssuer) CheckCert(cert *x509.Certificate) error {
	return alpnproxy.CheckDBCertSubject(cert, tlsca.RouteToDatabase{
		ServiceName: i.dbInfo.GetDatabaseKey().GetName(),
		Protocol:    i.dbInfo.GetProtocol(),
		Username:    i.dbInfo.GetUsername(),
	})
}

func (i *dbCertIssuer) IssueCert(ctx context.Context) (tls.Certificate, error) {
	cert, err, _ := i.group.Do("", func() (any, error) {
		return i.dbProvider.ReissueDBCert(ctx, i.dbInfo)
	})
	return cert.(tls.Certificate), trace.Wrap(err)
}

// dbLocalProxyMiddleware wraps around [client.CertChecker] and additionally
// calls OnNewDBConnection on [dbProvider] for observability.
type dbLocalProxyMiddleware struct {
	dbKey       *vnetv1.DatabaseKey
	certChecker *client.CertChecker
	dbProvider  *dbProvider
}

func (m *dbLocalProxyMiddleware) OnNewConnection(ctx context.Context, lp *alpnproxy.LocalProxy) error {
	err := m.certChecker.OnNewConnection(ctx, lp)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(m.dbProvider.OnNewDBConnection(ctx, m.dbKey))
}

func (m *dbLocalProxyMiddleware) OnStart(ctx context.Context, lp *alpnproxy.LocalProxy) error {
	return trace.Wrap(m.certChecker.OnStart(ctx, lp))
}

// RouteToDatabase returns a proto.RouteToDatabase populated from dbInfo.
func RouteToDatabase(dbInfo *vnetv1.DatabaseInfo) *proto.RouteToDatabase {
	return &proto.RouteToDatabase{
		ServiceName: dbInfo.GetDatabaseKey().GetName(),
		Protocol:    dbInfo.GetProtocol(),
		Username:    dbInfo.GetUsername(),
	}
}
