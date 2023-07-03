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
	"fmt"
	"net"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/client/proto"
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

	hint := fmt.Sprintf("MFA is required to access database %q", c.dbRoute.ServiceName)
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
		}, func(opts *PromptMFAChallengeOpts) {
			opts.HintBeforePrompt = hint
		})
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
