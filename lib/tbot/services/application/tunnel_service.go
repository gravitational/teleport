/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package application

import (
	"cmp"
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"time"

	"github.com/gravitational/trace"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	"github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/connection"
	"github.com/gravitational/teleport/lib/tbot/client"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/tbot/internal"
	"github.com/gravitational/teleport/lib/tbot/readyz"
	"github.com/gravitational/teleport/lib/utils"
)

func TunnelServiceBuilder(
	cfg *TunnelConfig,
	connCfg connection.Config,
	defaultCredentialLifetime bot.CredentialLifetime,
	leeway time.Duration,
) bot.ServiceBuilder {
	buildFn := func(deps bot.ServiceDependencies) (bot.Service, error) {
		if err := cfg.CheckAndSetDefaults(deps.Scoped); err != nil {
			return nil, trace.Wrap(err)
		}
		svc := &TunnelService{
			connCfg:                   connCfg,
			defaultCredentialLifetime: defaultCredentialLifetime,
			leeway:                    leeway,
			getBotIdentity:            deps.BotIdentity,
			botIdentityReadyCh:        deps.BotIdentityReadyCh,
			proxyPinger:               deps.ProxyPinger,
			botClient:                 deps.Client,
			cfg:                       cfg,
			identityGenerator:         deps.IdentityGenerator,
			clientBuilder:             deps.ClientBuilder,
			log:                       deps.Logger,
			statusReporter:            deps.GetStatusReporter(),
		}
		return svc, nil
	}
	return bot.NewServiceBuilder(TunnelServiceType, cfg.Name, buildFn)
}

// TunnelService is a service that listens on a socket and forwards
// traffic to an application registered in Teleport Application Access. It is
// an authenticating tunnel and will automatically issue and renew certificates
// as needed.
type TunnelService struct {
	connCfg                   connection.Config
	defaultCredentialLifetime bot.CredentialLifetime
	leeway                    time.Duration
	cfg                       *TunnelConfig
	proxyPinger               connection.ProxyPinger
	log                       *slog.Logger
	botClient                 *apiclient.Client
	getBotIdentity            func() *identity.Identity
	botIdentityReadyCh        <-chan struct{}
	statusReporter            readyz.Reporter
	identityGenerator         *identity.Generator
	clientBuilder             *client.Builder
}

func (s *TunnelService) Run(ctx context.Context) error {
	ctx, span := tracer.Start(ctx, "TunnelService/Run")
	defer span.End()

	l := s.cfg.Listener
	if l == nil {
		s.log.DebugContext(ctx, "Opening listener for application tunnel", "listen", s.cfg.Listen)
		var err error
		l, err = internal.CreateListener(ctx, s.log, s.cfg.Listen)
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

	s.statusReporter.Report(readyz.Healthy)

	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		s.statusReporter.ReportReason(readyz.Unhealthy, err.Error())
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
func (s *TunnelService) buildLocalProxyConfig(ctx context.Context) (lpCfg alpnproxy.LocalProxyConfig, err error) {
	ctx, span := tracer.Start(ctx, "TunnelService/buildLocalProxyConfig")
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

	proxyPing, err := s.proxyPinger.Ping(ctx)
	if err != nil {
		return alpnproxy.LocalProxyConfig{}, trace.Wrap(err, "pinging proxy")
	}
	proxyAddr, err := proxyPing.ProxyWebAddr()
	if err != nil {
		return alpnproxy.LocalProxyConfig{}, trace.Wrap(err, "determining proxy web addr")
	}

	s.log.DebugContext(ctx, "Issuing initial certificate for local proxy.")
	appCert, app, err := s.issueCert(ctx)
	if err != nil {
		return alpnproxy.LocalProxyConfig{}, trace.Wrap(err)
	}
	s.log.DebugContext(ctx, "Issued initial certificate for local proxy.")

	// Attempt to provide a sensible cap for leeway values based on the
	// configured and actual cert TTL to guard against rapid renewals. This
	// doesn't attempt to be perfect, and it's still possible to force a new
	// cert to be issued on every connection with the right set of unreasonable
	// values. However, we want it to be possible to specify  nearly-too-large
	// values for testing purposes (i.e. using negative leeway to simulate clock
	// drift) and do not want to be in the business of calculating leeway-leeway
	// so we'll keep the check somewhat simple.
	issuedCertLifetime := appCert.Leaf.NotAfter.Sub(appCert.Leaf.NotBefore)
	effectiveLifetime := cmp.Or(s.cfg.CredentialLifetime, s.defaultCredentialLifetime)
	effectiveTTL := min(issuedCertLifetime, effectiveLifetime.TTL)

	leeway := s.leeway
	if leeway >= effectiveTTL {
		s.log.WarnContext(ctx,
			"leeway is greater than the credential lifetime and will be "+
				"ignored, be aware of potential failures due to clock drift",
			"configured_ttl", effectiveLifetime.TTL,
			"configured_leeway", leeway,
			"issued_cert_ttl", issuedCertLifetime,
		)
		leeway = 0
	}

	middleware := internal.ALPNProxyMiddleware{
		OnNewConnectionFunc: func(ctx context.Context, lp *alpnproxy.LocalProxy) error {
			ctx, span := tracer.Start(ctx, "TunnelService/OnNewConnection")
			defer span.End()

			if err := lp.CheckCertExpiryWithLeeway(ctx, leeway); err != nil {
				s.log.InfoContext(ctx, "Certificate for tunnel needs reissuing.",
					"reason", err.Error(),
					"leeway", leeway,
				)
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
		InsecureSkipVerify: s.connCfg.Insecure,
		Clock:              s.cfg.clock,
	}
	if apiclient.IsALPNConnUpgradeRequired(
		ctx,
		proxyAddr,
		s.connCfg.Insecure,
	) {
		lpConfig.ALPNConnUpgradeRequired = true
		// If ALPN Conn Upgrade will be used, we need to set the cluster CAs
		// to validate the Proxy's auth issued host cert.
		lpConfig.RootCAs = s.getBotIdentity().TLSCAPool
	}

	return lpConfig, nil
}

func (s *TunnelService) issueCert(
	ctx context.Context,
) (*tls.Certificate, types.Application, error) {
	ctx, span := tracer.Start(ctx, "TunnelService/issueCert")
	defer span.End()

	// Right now we have to redetermine the route to app each time as the
	// session ID may need to change. Once v17 hits, this will be automagically
	// calculated by the auth server on cert generation, and we can fetch the
	// routeToApp once.
	effectiveLifetime := cmp.Or(s.cfg.CredentialLifetime, s.defaultCredentialLifetime)
	identityOpts := []identity.GenerateOption{
		identity.WithRoles(s.cfg.Roles),
		identity.WithLifetime(effectiveLifetime.TTL, effectiveLifetime.RenewalInterval),
		identity.WithLogger(s.log),
	}
	impersonatedIdentity, err := s.identityGenerator.GenerateFacade(ctx, identityOpts...)
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
	routedIdent, err := s.identityGenerator.Generate(ctx, append(identityOpts, identity.WithRouteToApp(route))...)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	s.log.InfoContext(ctx, "Certificate issued for tunnel proxy.")

	// In tests, notify the test that a cert has been issued.
	if s.cfg.certIssuedHook != nil {
		s.cfg.certIssuedHook()
	}

	// The leaf isn't appended by the stdlib, so add it here so we can inspect
	// the TTL downstream.
	cert := routedIdent.TLSCert
	cert.Leaf = routedIdent.X509Cert

	return cert, app, nil
}

// String returns a human-readable string that can uniquely identify the
// service.
func (s *TunnelService) String() string {
	return cmp.Or(
		s.cfg.Name,
		fmt.Sprintf("%s:%s:%s", TunnelServiceType, s.cfg.Listen, s.cfg.AppName),
	)
}
