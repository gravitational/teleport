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

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// LocalProxyMiddleware provides callback functions for LocalProxy.
type LocalProxyMiddleware interface {
	// OnNewConnection is a callback triggered when a new downstream connection is
	// accepted by the local proxy.
	OnNewConnection(ctx context.Context, lp *LocalProxy, conn net.Conn) error
	// OnStart is a callback triggered when the local proxy starts.
	OnStart(ctx context.Context, lp *LocalProxy) error
}

// DBCertChecker is a middleware that ensures that the local proxy has valid TLS database certs.
type DBCertChecker struct {
	// tc is a TeleportClient used to reissue certificates when necessary.
	tc *client.TeleportClient
	// dbRoute contains database routing information.
	dbRoute tlsca.RouteToDatabase
	// Clock specifies the time provider. Will be used to override the time anchor
	// for TLS certificate verification.
	// Defaults to real clock if unspecified
	clock clockwork.Clock
}

func NewDBCertChecker(tc *client.TeleportClient, dbRoute tlsca.RouteToDatabase, clock clockwork.Clock) (LocalProxyMiddleware, error) {
	if err := dbRoute.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	if clock == nil {
		clock = clockwork.NewRealClock()
	}
	return &DBCertChecker{
		tc:      tc,
		dbRoute: dbRoute,
		clock:   clock,
	}, nil
}

var _ LocalProxyMiddleware = (*DBCertChecker)(nil)

// OnNewConnection is a callback triggered when a new downstream connection is
// accepted by the local proxy.
func (c *DBCertChecker) OnNewConnection(ctx context.Context, lp *LocalProxy, conn net.Conn) error {
	return trace.Wrap(c.checkCerts(ctx, lp))
}

// OnStart is a callback triggered when the local proxy starts.
func (c *DBCertChecker) OnStart(ctx context.Context, lp *LocalProxy) error {
	return trace.Wrap(c.checkCerts(ctx, lp))
}

// needCertRenewal checks if the local proxy TLS certs are configured and not expired.
func (c *DBCertChecker) needCertRenewal(lp *LocalProxy) (bool, error) {
	certs := lp.GetCerts()
	if len(certs) == 0 {
		log.Debug("local proxy has no TLS certificates configured, need cert renewal")
		return true, nil
	}
	err := utils.VerifyTLSCertificateExpiry(certs[0], c.clock)
	if err != nil {
		log.WithError(err).Debug("need cert renewal")
		return true, nil
	}
	return false, trace.Wrap(err)
}

// checkCerts checks if the local proxy requires database TLS cert renewal.
func (c *DBCertChecker) checkCerts(ctx context.Context, lp *LocalProxy) error {
	log.Debug("checking local proxy database certs")
	if needDBLogin, err := c.needCertRenewal(lp); err != nil {
		return trace.Wrap(err)
	} else if !needDBLogin {
		return nil
	}

	var accessRequests []string
	if profile, err := client.StatusCurrent(c.tc.HomePath, c.tc.WebProxyAddr, ""); err != nil {
		log.WithError(err).Warn("unable to load profile, requesting database certs without access requests")
	} else {
		accessRequests = profile.ActiveRequests.AccessRequests
	}
	var key *client.Key
	if err := client.RetryWithRelogin(ctx, c.tc, func() error {
		newKey, err := c.tc.IssueUserCertsWithMFA(ctx, client.ReissueParams{
			RouteToCluster: c.tc.SiteName,
			RouteToDatabase: proto.RouteToDatabase{
				ServiceName: c.dbRoute.ServiceName,
				Protocol:    c.dbRoute.Protocol,
				Username:    c.dbRoute.Username,
				Database:    c.dbRoute.Database,
			},
			AccessRequests: accessRequests,
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
	lp.SetCerts([]tls.Certificate{tlsCert})
	return nil
}
