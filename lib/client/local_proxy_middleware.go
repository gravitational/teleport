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
	"crypto/x509"
	"net"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// CertChecker is a local proxy middleware that ensures certs are valid
// on start up and on each new connection.
type CertChecker struct {
	// certReissuer checks and reissues certs.
	certReissuer certReissuer
	// clock specifies the time provider. Will be used to override the time anchor
	// for TLS certificate verification. Defaults to real clock if unspecified
	clock clockwork.Clock
}

var _ alpnproxy.LocalProxyMiddleware = (*CertChecker)(nil)

func newCertChecker(certIssuer certReissuer, clock clockwork.Clock) *CertChecker {
	if clock == nil {
		clock = clockwork.NewRealClock()
	}
	return &CertChecker{
		certReissuer: certIssuer,
		clock:        clock,
	}
}

// Create a new CertChecker for the given database.
func NewDBCertChecker(tc *TeleportClient, dbRoute tlsca.RouteToDatabase, clock clockwork.Clock) *CertChecker {
	return newCertChecker(&dbCertReissuer{
		tc:      tc,
		dbRoute: dbRoute,
	}, clock)
}

// Create a new CertChecker for the given app.
func NewAppCertChecker(tc *TeleportClient, appRoute proto.RouteToApp, clock clockwork.Clock) *CertChecker {
	return newCertChecker(&appCertReissuer{
		tc:       tc,
		appRoute: appRoute,
	}, clock)
}

// OnNewConnection is a callback triggered when a new downstream connection is
// accepted by the local proxy.
func (c *CertChecker) OnNewConnection(ctx context.Context, lp *alpnproxy.LocalProxy, conn net.Conn) error {
	return trace.Wrap(c.ensureValidCerts(ctx, lp))
}

// OnStart is a callback triggered when the local proxy starts.
func (c *CertChecker) OnStart(ctx context.Context, lp *alpnproxy.LocalProxy) error {
	return trace.Wrap(c.ensureValidCerts(ctx, lp))
}

// ensureValidCerts ensures that the local proxy is configured with valid certs.
func (c *CertChecker) ensureValidCerts(ctx context.Context, lp *alpnproxy.LocalProxy) error {
	if err := lp.CheckCert(c.certReissuer.checkCert); err != nil {
		log.WithError(err).Debug("local proxy tunnel certificates need to be reissued")
	} else {
		return nil
	}

	cert, err := c.certReissuer.reissueCert(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	// reduce per-handshake processing by setting the parsed leaf.
	if err := utils.InitCertLeaf(&cert); err != nil {
		return trace.Wrap(err)
	}

	certTTL := cert.Leaf.NotAfter.Sub(c.clock.Now()).Round(time.Minute)
	log.Debugf("Certificate renewed: valid until %s [valid for %v]", cert.Leaf.NotAfter.Format(time.RFC3339), certTTL)

	lp.SetCert(cert)
	return nil
}

// certReissuer checks and reissues certs.
type certReissuer interface {
	// checkCert checks that an existing certificate is valid.
	checkCert(cert *x509.Certificate) error
	// reissueCert reissues a tls certificate.
	reissueCert(ctx context.Context) (tls.Certificate, error)
}

type dbCertReissuer struct {
	// tc is a TeleportClient used to issue certificates when necessary.
	tc *TeleportClient
	// dbRoute contains database routing information.
	dbRoute tlsca.RouteToDatabase
}

func (c *dbCertReissuer) checkCert(cert *x509.Certificate) error {
	return alpnproxy.CheckDBCertSubject(cert, c.dbRoute)
}

func (c *dbCertReissuer) reissueCert(ctx context.Context) (tls.Certificate, error) {
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
		return tls.Certificate{}, trace.Wrap(err)
	}

	dbCert, err := key.DBTLSCert(c.dbRoute.ServiceName)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}
	return dbCert, nil
}

type appCertReissuer struct {
	// tc is a TeleportClient used to issue certificates when necessary.
	tc *TeleportClient
	// appRoute contains app routing information.
	appRoute proto.RouteToApp
}

func (c *appCertReissuer) checkCert(cert *x509.Certificate) error {
	// appCertIssuer does not perform any additional certificate checks.
	return nil
}

func (c *appCertReissuer) ReissueCert(ctx context.Context) (tls.Certificate, error) {
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
			RouteToApp:     c.appRoute,
			AccessRequests: accessRequests,
			RequesterName:  proto.UserCertsRequest_TSH_APP_LOCAL_PROXY,
		}, mfa.WithPromptReasonSessionMFA("application", c.appRoute.Name))
		key = newKey
		return trace.Wrap(err)
	}); err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	appCert, err := key.AppTLSCert(c.appRoute.Name)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}
	return appCert, nil
}
