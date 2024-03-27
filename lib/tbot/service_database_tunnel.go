/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package tbot

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/url"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	"github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tlsca"
)

// DatabaseTunnelService is a service that listens on a local port and forwards
// connections to a remote database service. It is an authenticating tunnel and
// will automatically issue and renew certificates as needed.
type DatabaseTunnelService struct {
	botCfg         *config.BotConfig
	cfg            *config.DatabaseTunnelService
	log            logrus.FieldLogger
	resolver       reversetunnelclient.Resolver
	botClient      *auth.Client
	getBotIdentity getBotIdentityFn

	// routeToDatabase is the cached route to database parameters. We determine
	// this once and then reuse it to reduce latency associated with issuing
	// the certificates on connection. Set by buildLocalProxyConfig.
	routeToDatabase proto.RouteToDatabase
	// roles is the list of roles to request for the impersonated teleport
	// identity. This is the configured value, or if unconfigured, all the
	// roles the bot has. Set by buildLocalProxyConfig.
	roles []string
}

var _ alpnproxy.LocalProxyMiddleware = (*DatabaseTunnelService)(nil)

// buildLocalProxyConfig initializes the service, fetching any initial information and setting
// up the localproxy.
func (s *DatabaseTunnelService) buildLocalProxyConfig(ctx context.Context) (lpCfg alpnproxy.LocalProxyConfig, err error) {
	ctx, span := tracer.Start(ctx, "DatabaseTunnelService/buildLocalProxyConfig")
	defer span.End()

	// Determine the roles to use for the impersonated db access user. We fall
	// back to all the roles the bot has if none are configured.
	s.roles = s.cfg.Roles
	if len(s.roles) == 0 {
		s.roles, err = fetchDefaultRoles(ctx, s.botClient, s.getBotIdentity())
		if err != nil {
			return alpnproxy.LocalProxyConfig{}, trace.Wrap(err, "fetching default roles")
		}
	}

	// Fetch information about the database and then issue the initial
	// certificate. We issue the initial certificate to allow us to fail faster.
	// We cache the routeToDatabase as these will not change during the lifetime
	// of the service and this reduces the time needed to issue a new
	// certificate.
	// TODO: Use impersonated identity to fetch route to catch permissions
	// issues sooner.
	s.routeToDatabase, err = getRouteToDatabase(
		ctx, s.log, s.botClient, s.cfg.Service, s.cfg.Username, s.cfg.Database,
	)
	if err != nil {
		return alpnproxy.LocalProxyConfig{}, trace.Wrap(err)
	}
	dbCert, err := s.issueCert(ctx)
	if err != nil {
		return alpnproxy.LocalProxyConfig{}, trace.Wrap(err)
	}

	proxyAddr := "leaf.tele.ottr.sh:443"

	alpnProtocol, err := common.ToALPNProtocol(s.routeToDatabase.Protocol)
	if err != nil {
		return alpnproxy.LocalProxyConfig{}, trace.Wrap(err)

	}
	lpConfig := alpnproxy.LocalProxyConfig{
		// Pass ourselves in as the middleware which is called on connection to
		// issue certificates. See OnNewConnection.
		Middleware: s,

		RemoteProxyAddr: proxyAddr,
		ParentContext:   ctx,
		Protocols:       []common.Protocol{alpnProtocol},
		Certs:           []tls.Certificate{*dbCert},
	}
	if client.IsALPNConnUpgradeRequired(
		ctx,
		proxyAddr,
		s.botCfg.Insecure,
	) {
		lpConfig.ALPNConnUpgradeRequired = true
		// If ALPN Conn Upgrade will be used, we need to set the cluster CAs
		// to validate the Proxy's auth issued host cert.
		lpConfig.RootCAs = s.getBotIdentity().TLSCAPool
	}

	return lpConfig, nil
}

func (s *DatabaseTunnelService) Run(ctx context.Context) error {
	ctx, span := tracer.Start(ctx, "DatabaseTunnelService/Run")
	defer span.End()

	listenUrl, err := url.Parse(s.cfg.Listen)
	if err != nil {
		return trace.Wrap(err, "parsing listen url")
	}

	lpCfg, err := s.buildLocalProxyConfig(ctx)
	if err != nil {
		return trace.Wrap(err, "building local proxy config")
	}

	l, err := net.Listen("tcp", listenUrl.Host)
	if err != nil {
		return trace.Wrap(err, "opening listener")
	}
	defer func() {
		if err := l.Close(); err != nil {
			s.log.WithError(err).Error("Failed to close listener")
		}
	}()

	lp, err := alpnproxy.NewLocalProxy(lpCfg)
	if err != nil {
		return trace.Wrap(err, "creating local proxy")
	}
	defer func() {
		if err := lp.Close(); err != nil {
			s.log.WithError(err).Error("Failed to close local proxy")
		}
	}()

	return trace.Wrap(lp.Start(ctx))
}

func (s *DatabaseTunnelService) issueCert(ctx context.Context) (*tls.Certificate, error) {
	ctx, span := tracer.Start(ctx, "DatabaseTunnelService/issueCert")
	defer span.End()

	s.log.Debug("Issuing certificate for database tunnel.")
	ident, err := generateIdentity(
		ctx,
		s.botClient,
		s.getBotIdentity(),
		s.roles,
		s.botCfg.CertificateTTL,
		func(req *proto.UserCertsRequest) {
			req.RouteToDatabase = s.routeToDatabase
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s.log.Info("Certificate issued for database tunnel.")

	return ident.TLSCert, nil
}

// OnNewConnection is called by the localproxy when a new connection is made.
// Implements the LocalProxyMiddleware interface.
func (s *DatabaseTunnelService) OnNewConnection(
	ctx context.Context, lp *alpnproxy.LocalProxy, _ net.Conn,
) error {
	ctx, span := tracer.Start(ctx, "DatabaseTunnelService/OnNewConnection")
	defer span.End()

	// Check if the certificate needs reissuing, if so, reissue.
	if err := lp.CheckDBCerts(tlsca.RouteToDatabase{
		ServiceName: s.routeToDatabase.ServiceName,
		Protocol:    s.routeToDatabase.Protocol,
		Database:    s.routeToDatabase.Database,
		Username:    s.routeToDatabase.Username,
	}); err != nil {
		s.log.WithField("reason", err.Error()).Info("Certificate for tunnel needs reissuing.")
		cert, err := s.issueCert(ctx)
		if err != nil {
			return trace.Wrap(err, "issuing cert")
		}
		lp.SetCerts([]tls.Certificate{*cert})
	}
	return nil
}

// OnStart is called by the localproxy is started.
// Required to implement the LocalProxyMiddleware interface.
func (s *DatabaseTunnelService) OnStart(_ context.Context, _ *alpnproxy.LocalProxy) error {
	// Nothing to do here. We already inject an initial cert.
	return nil
}

// String returns a human-readable string that can uniquely identify the
// service.
func (s *DatabaseTunnelService) String() string {
	return fmt.Sprintf("%s:%s", config.DatabaseTunnelServiceType, s.cfg.Listen)
}
