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

type dbHandler struct {
	cfg *dbHandlerConfig
	log *slog.Logger

	mu         sync.Mutex
	localProxy *alpnproxy.LocalProxy
}

type dbHandlerConfig struct {
	dbInfo     *vnetv1.DatabaseInfo
	dbProvider *dbProvider
	clock      clockwork.Clock
	// alwaysTrustRootClusterCA can be set in tests so that TLS dials to the
	// proxy always trust the root cluster CA rather than the system cert pool,
	// even when ALPN conn upgrades are not required.
	alwaysTrustRootClusterCA bool
	parentCtx                context.Context
}

func newDBHandler(cfg *dbHandlerConfig) *dbHandler {
	return &dbHandler{
		cfg: cfg,
		log: log.With(
			teleport.ComponentKey, teleport.Component("vnet", "db-handler"),
			"profile", cfg.dbInfo.GetDatabaseKey().GetProfile(),
			"leaf_cluster", cfg.dbInfo.GetDatabaseKey().GetLeafCluster(),
			"db_name", cfg.dbInfo.GetDatabaseKey().GetName(),
			"protocol", cfg.dbInfo.GetProtocol()),
	}
}

func (h *dbHandler) getOrInitializeLocalProxy(ctx context.Context) (*alpnproxy.LocalProxy, error) {
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
	middleware := &localProxyMiddleware{
		certChecker: certChecker,
		onNewConnection: func(ctx context.Context) error {
			return h.cfg.dbProvider.OnNewDBConnection(ctx, h.cfg.dbInfo.GetDatabaseKey())
		},
	}

	h.log.DebugContext(ctx, "Creating local proxy for database")
	lp, err := newLocalProxy(localProxyConfig{
		dialOptions:              h.cfg.dbInfo.GetDialOptions(),
		protocols:                []alpncommon.Protocol{alpnProtocol},
		parentContext:            h.cfg.parentCtx,
		middleware:               middleware,
		clock:                    h.cfg.clock,
		alwaysTrustRootClusterCA: h.cfg.alwaysTrustRootClusterCA,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	h.localProxy = lp
	return lp, nil
}

// handleTCPConnector handles an incoming TCP connection from VNet by passing it
// to the local ALPN proxy, which is configured with middleware to automatically
// handle certificate renewal and re-logins.
//
// localPort is part of the tcpHandler interface contract but is unused here
// the local ALPN proxy ignores it for database connections.
func (h *dbHandler) handleTCPConnector(ctx context.Context, _ uint16, connector func() (net.Conn, error)) error {
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
	})
}

func (i *dbCertIssuer) IssueCert(ctx context.Context) (tls.Certificate, error) {
	certVal, err, _ := i.group.Do("", func() (any, error) {
		return i.dbProvider.ReissueDBCert(ctx, i.dbInfo)
	})
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}
	cert, ok := certVal.(tls.Certificate)
	if !ok {
		return tls.Certificate{}, trace.BadParameter("singleflight returned unexpected type %T", certVal)
	}
	return cert, nil
}

// RouteToDatabase returns a proto.RouteToDatabase populated from dbInfo.
// Omitting the username is intentional: VNet only supports protocols where the
// db_service extracts the username from the wire protocol.
func RouteToDatabase(dbInfo *vnetv1.DatabaseInfo) *proto.RouteToDatabase {
	return &proto.RouteToDatabase{
		ServiceName: dbInfo.GetDatabaseKey().GetName(),
		Protocol:    dbInfo.GetProtocol(),
	}
}
