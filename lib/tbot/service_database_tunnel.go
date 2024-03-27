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
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	"github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tlsca"
)

// DatabaseTunnelService
type DatabaseTunnelService struct {
	svcIdentity *config.UnstableClientCredentialOutput
	botCfg      *config.BotConfig
	cfg         *config.DatabaseTunnelService
	log         logrus.FieldLogger
	botClient   *auth.Client
	resolver    reversetunnelclient.Resolver

	dbRoute tlsca.RouteToDatabase

	// client holds the impersonated client for the service
	client *auth.Client
}

// OnNewConnection is called by the localproxy when a new connection is made.
// Implements the LocalProxyMiddleware interface.
func (s *DatabaseTunnelService) OnNewConnection(ctx context.Context, lp *alpnproxy.LocalProxy, conn net.Conn) error {
	// Check if the certificate needs reissuing, if so, reissue.
	if err := lp.CheckDBCerts(s.dbRoute); err != nil {
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

func (s *DatabaseTunnelService) issueCert(ctx context.Context) (*tls.Certificate, error) {
	//TODO implement me
	panic("implement me")
}

var _ alpnproxy.LocalProxyMiddleware = (*DatabaseTunnelService)(nil)

// setup initializes the service
func (s *DatabaseTunnelService) setup(ctx context.Context) (*alpnproxy.LocalProxy, error) {
	ctx, span := tracer.Start(ctx, "DatabaseTunnelService/setup")
	defer span.End()

	listenUrl, err := url.Parse(s.cfg.Listen)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO(noah): To support Oracle we need to offer a TLS listener instead
	l, err := net.Listen("tcp", listenUrl.Host)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer func() {
		if err := l.Close(); err != nil {
			s.log.WithError(err).Error("failed to close listener")
		}
	}()

	// TODO: Fetch!
	s.dbRoute = tlsca.RouteToDatabase{}
	protocol := ""
	proxyAddr := ""
	cert := &tls.Certificate{}

	alpnProtocol, err := common.ToALPNProtocol(protocol)
	if err != nil {
		return nil, trace.Wrap(err)

	}

	lpConfig := alpnproxy.LocalProxyConfig{
		Listener:        l,
		RemoteProxyAddr: proxyAddr,
		ParentContext:   ctx,
		Middleware:      s,
		Protocols:       []common.Protocol{alpnProtocol},
		Certs:           []tls.Certificate{*cert}, // TODO
	}

	if client.IsALPNConnUpgradeRequired(
		ctx,
		proxyAddr,
		s.botCfg.Insecure,
	) {
		lpConfig.ALPNConnUpgradeRequired = true
		// If ALPN Conn Upgrade will be used, we need to set the cluster CAs
		// to validate the Proxy's auth issued host cert.
		lpConfig.RootCAs = nil // TODO
	}
	lp, err := alpnproxy.NewLocalProxy(lpConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return lp, nil
}

func (s *DatabaseTunnelService) Run(ctx context.Context) error {
	ctx, span := tracer.Start(ctx, "DatabaseTunnelService/Run")
	defer span.End()

	lp, err := s.setup(ctx)
	if err != nil {
		return trace.Wrap(err)

	}

	return trace.Wrap(lp.Start(ctx))
}

// String returns a human-readable string that can uniquely identify the
// service.
func (s *DatabaseTunnelService) String() string {
	return fmt.Sprintf("%s:%s", config.DatabaseTunnelServiceType, s.cfg.Listen)
}
