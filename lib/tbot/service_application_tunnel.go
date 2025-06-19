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
	"cmp"
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"

	"github.com/gravitational/trace"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	"github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/tbot/client"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/utils"
)

// ApplicationTunnelService is a service that listens on a socket and forwards
// traffic to an application registered in Teleport Application Access. It is
// an authenticating tunnel and will automatically issue and renew certificates
// as needed.
type ApplicationTunnelService struct {
	botCfg             *config.BotConfig
	cfg                *config.ApplicationTunnelService
	proxyPingCache     *proxyPingCache
	log                *slog.Logger
	botClient          *apiclient.Client
	getBotIdentity     getBotIdentityFn
	botIdentityReadyCh <-chan struct{}
	identityGenerator  *identity.Generator
	clientBuilder      *client.Builder
}

func (s *ApplicationTunnelService) Run(ctx context.Context) error {
	ctx, span := tracer.Start(ctx, "ApplicationTunnelService/Run")
	defer span.End()

	l := s.cfg.Listener
	if l == nil {
		s.log.DebugContext(ctx, "Opening listener for application tunnel", "listen", s.cfg.Listen)
		var err error
		l, err = createListener(ctx, s.log, s.cfg.Listen)
		if err != nil {
			return trace.Wrap(err, "opening listener")
		}
		defer func() {
			if err := l.Close(); err != nil && !utils.IsUseOfClosedNetworkError(err) {
				s.log.ErrorContext(ctx, "Failed to close listener", "error", err)
			}
		}()
	}

	lpCfg, err := s.buildLocalProxyConfig(ctx)
	if err != nil {
		return trace.Wrap(err, "building local proxy config")
	}
	lpCfg.Listener = l

	lp, err := alpnproxy.NewLocalProxy(lpCfg)
	if err != nil {
		return trace.Wrap(err, "creating local proxy")
	}
	defer func() {
		if err := lp.Close(); err != nil {
			s.log.ErrorContext(ctx, "Failed to close local proxy", "error", err)
		}
	}()
	// Closed further down.

	// lp.Start will block and continues to block until lp.Close() is called.
	// Despite taking a context, it will not exit until the first connection is
	// made after the context is canceled.
	var errCh = make(chan error, 1)
	go func() {
		errCh <- lp.Start(ctx)
	}()
	s.log.InfoContext(ctx, "Listening for connections.", "address", l.Addr().String())

	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		return trace.Wrap(err, "local proxy failed")
	}
}

func alpnProtocolForApp(app types.Application) common.Protocol {
	if app.IsTCP() {
		return common.ProtocolTCP
	}
	return common.ProtocolHTTP
}

// buildLocalProxyConfig initializes the service, fetching any initial information and setting
// up the localproxy.
func (s *ApplicationTunnelService) buildLocalProxyConfig(ctx context.Context) (lpCfg alpnproxy.LocalProxyConfig, err error) {
	ctx, span := tracer.Start(ctx, "ApplicationTunnelService/buildLocalProxyConfig")
	defer span.End()

	if s.botIdentityReadyCh != nil {
		select {
		case <-s.botIdentityReadyCh:
		default:
			s.log.InfoContext(ctx, "Waiting for internal bot identity to be renewed before running")
			select {
			case <-s.botIdentityReadyCh:
			case <-ctx.Done():
				return alpnproxy.LocalProxyConfig{}, ctx.Err()
			}
		}
	}

	proxyPing, err := s.proxyPingCache.ping(ctx)
	if err != nil {
		return alpnproxy.LocalProxyConfig{}, trace.Wrap(err, "pinging proxy")
	}
	proxyAddr, err := proxyPing.proxyWebAddr()
	if err != nil {
		return alpnproxy.LocalProxyConfig{}, trace.Wrap(err, "determining proxy web addr")
	}

	s.log.DebugContext(ctx, "Issuing initial certificate for local proxy.")
	appCert, app, err := s.issueCert(ctx)
	if err != nil {
		return alpnproxy.LocalProxyConfig{}, trace.Wrap(err)
	}
	s.log.DebugContext(ctx, "Issued initial certificate for local proxy.")

	middleware := alpnProxyMiddleware{
		onNewConnection: func(ctx context.Context, lp *alpnproxy.LocalProxy) error {
			ctx, span := tracer.Start(ctx, "ApplicationTunnelService/OnNewConnection")
			defer span.End()

			if err := lp.CheckCertExpiry(ctx); err != nil {
				s.log.InfoContext(ctx, "Certificate for tunnel needs reissuing.", "reason", err.Error())
				cert, _, err := s.issueCert(ctx)
				if err != nil {
					return trace.Wrap(err, "issuing cert")
				}
				lp.SetCert(*cert)
			}
			return nil
		},
	}

	lpConfig := alpnproxy.LocalProxyConfig{
		Middleware: middleware,

		RemoteProxyAddr:    proxyAddr,
		ParentContext:      ctx,
		Protocols:          []common.Protocol{alpnProtocolForApp(app)},
		Cert:               *appCert,
		InsecureSkipVerify: s.botCfg.Insecure,
	}
	if apiclient.IsALPNConnUpgradeRequired(
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

func (s *ApplicationTunnelService) issueCert(
	ctx context.Context,
) (*tls.Certificate, types.Application, error) {
	ctx, span := tracer.Start(ctx, "ApplicationTunnelService/issueCert")
	defer span.End()

	// Right now we have to redetermine the route to app each time as the
	// session ID may need to change. Once v17 hits, this will be automagically
	// calculated by the auth server on cert generation, and we can fetch the
	// routeToApp once.
	effectiveLifetime := cmp.Or(s.cfg.CredentialLifetime, s.botCfg.CredentialLifetime)
	impersonatedIdentity, err := s.identityGenerator.GenerateFacade(ctx, identity.GenerateParams{
		Roles:           s.cfg.Roles,
		TTL:             effectiveLifetime.TTL,
		RenewalInterval: effectiveLifetime.RenewalInterval,
		Logger:          s.log,
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	impersonatedClient, err := s.clientBuilder.Build(ctx, impersonatedIdentity)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	defer func() {
		if err := impersonatedClient.Close(); err != nil {
			s.log.ErrorContext(ctx, "Failed to close impersonated client.", "error", err)
		}
	}()
	route, app, err := getRouteToApp(ctx, s.getBotIdentity(), impersonatedClient, s.cfg.AppName)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	s.log.DebugContext(ctx, "Requesting issuance of certificate for tunnel proxy.")
	routedIdent, err := s.identityGenerator.Generate(ctx, identity.GenerateParams{
		Roles:           s.cfg.Roles,
		TTL:             effectiveLifetime.TTL,
		RenewalInterval: effectiveLifetime.RenewalInterval,
		Options:         []identity.GenerateOption{identity.WithRouteToApp(route)},
		Logger:          s.log,
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	s.log.InfoContext(ctx, "Certificate issued for tunnel proxy.")

	return routedIdent.TLSCert, app, nil
}

// String returns a human-readable string that can uniquely identify the
// service.
func (s *ApplicationTunnelService) String() string {
	return fmt.Sprintf("%s:%s:%s", config.ApplicationTunnelServiceType, s.cfg.Listen, s.cfg.AppName)
}
