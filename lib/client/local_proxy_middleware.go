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

package client

import (
	"context"
	"crypto/tls"
	"net"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// DBCertChecker is a middleware that ensures that the local proxy has valid TLS database certs.
type DBCertChecker struct {
	// tc is a TeleportClient used to reissue certificates when necessary.
	tc *TeleportClient
	// dbRoute contains database routing information.
	dbRoute tlsca.RouteToDatabase
	// Clock specifies the time provider. Will be used to override the time anchor
	// for TLS certificate verification.
	// Defaults to real clock if unspecified
	clock clockwork.Clock
}

func NewDBCertChecker(tc *TeleportClient, dbRoute tlsca.RouteToDatabase, clock clockwork.Clock) *DBCertChecker {
	if clock == nil {
		clock = clockwork.NewRealClock()
	}
	return &DBCertChecker{
		tc:      tc,
		dbRoute: dbRoute,
		clock:   clock,
	}
}

var _ alpnproxy.LocalProxyMiddleware = (*DBCertChecker)(nil)

// OnNewConnection is a callback triggered when a new downstream connection is
// accepted by the local proxy.
func (c *DBCertChecker) OnNewConnection(ctx context.Context, lp *alpnproxy.LocalProxy, conn net.Conn) error {
	return trace.Wrap(c.ensureValidCerts(ctx, lp))
}

// OnStart is a callback triggered when the local proxy starts.
func (c *DBCertChecker) OnStart(ctx context.Context, lp *alpnproxy.LocalProxy) error {
	return trace.Wrap(c.ensureValidCerts(ctx, lp))
}

// ensureValidCerts ensures that the local proxy is configured with valid certs.
func (c *DBCertChecker) ensureValidCerts(ctx context.Context, lp *alpnproxy.LocalProxy) error {
	if err := lp.CheckDBCerts(c.dbRoute); err != nil {
		log.WithError(err).Debug("local proxy tunnel certificates need to be reissued")
	} else {
		return nil
	}
	return trace.Wrap(c.renewCerts(ctx, lp))
}

// renewCerts attempts to renew the database certs for the local proxy.
func (c *DBCertChecker) renewCerts(ctx context.Context, lp *alpnproxy.LocalProxy) error {
	var accessRequests []string
	if profile, err := c.tc.ProfileStatus(); err != nil {
		log.WithError(err).Warn("unable to load profile, requesting database certs without access requests")
	} else {
		accessRequests = profile.ActiveRequests.AccessRequests
	}

	var key *Key
	if err := RetryWithRelogin(ctx, c.tc, func() error {
		newKey, err := c.tc.IssueUserCertsWithMFA(ctx, ReissueParams{
			RouteToCluster: c.tc.SiteName,
			RouteToDatabase: proto.RouteToDatabase{
				ServiceName: c.dbRoute.ServiceName,
				Protocol:    c.dbRoute.Protocol,
				Username:    c.dbRoute.Username,
				Database:    c.dbRoute.Database,
			},
			AccessRequests: accessRequests,
			RequesterName:  proto.UserCertsRequest_TSH_DB_LOCAL_PROXY_TUNNEL,
		}, mfa.WithPromptReasonSessionMFA("database", c.dbRoute.ServiceName))
		key = newKey
		return trace.Wrap(err)
	}); err != nil {
		return trace.Wrap(err)
	}

	dbCert, ok := key.DBTLSCerts[c.dbRoute.ServiceName]
	if !ok {
		return trace.NotFound("database '%v' TLS cert missing", c.dbRoute.ServiceName)
	}
	tlsCert, err := keys.X509KeyPair(dbCert, key.PrivateKeyPEM())
	if err != nil {
		return trace.Wrap(err)
	}
	leaf, err := utils.TLSCertLeaf(tlsCert)
	if err != nil {
		return trace.Wrap(err)
	}
	certTTL := leaf.NotAfter.Sub(c.clock.Now()).Round(time.Minute)
	log.Debugf("Database certificate renewed: valid until %s [valid for %v]",
		leaf.NotAfter.Format(time.RFC3339), certTTL)
	// reduce per-handshake processing by setting the parsed leaf.
	tlsCert.Leaf = leaf
	lp.SetCerts([]tls.Certificate{tlsCert})
	return nil
}
