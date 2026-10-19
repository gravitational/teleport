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
	"crypto/x509"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"

	"github.com/gravitational/trace"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/connection"
	"github.com/gravitational/teleport/lib/tbot/client"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/tbot/internal"
	"github.com/gravitational/teleport/lib/tbot/readyz"
	"github.com/gravitational/teleport/lib/utils"
)

// ProxyServiceBuilder returns the service builder used to construct the
// ProxyService during bot startup.
func ProxyServiceBuilder(
	cfg *ProxyServiceConfig,
	connCfg connection.Config,
	defaultCredentialLifetime bot.CredentialLifetime,
	alpnUpgradeCache *internal.ALPNUpgradeCache,
) bot.ServiceBuilder {
	buildFn := func(deps bot.ServiceDependencies) (bot.Service, error) {
		if err := cfg.CheckAndSetDefaults(deps.Scoped); err != nil {
			return nil, trace.Wrap(err)
		}
		svc := &ProxyService{
			connCfg:            connCfg,
			effectiveLifetime:  cmp.Or(cfg.CredentialLifetime, defaultCredentialLifetime),
			getBotIdentity:     deps.BotIdentity,
			botIdentityReadyCh: deps.BotIdentityReadyCh,
			proxyPinger:        deps.ProxyPinger,
			botClient:          deps.Client,
			cfg:                cfg,
			identityGenerator:  deps.IdentityGenerator,
			clientBuilder:      deps.ClientBuilder,
			scoped:             deps.Scoped,
			alpnUpgradeCache:   alpnUpgradeCache,
			log:                deps.Logger,
			statusReporter:     deps.GetStatusReporter(),
		}
		return svc, nil
	}
	return bot.NewServiceBuilder(ProxyServiceType, cfg.Name, buildFn)
}

// ProxyService presents a http_proxy compatible proxy on a listener which will
// forward traffic to applications through Teleport. Unlike the TunnelService
// it is protocol aware and does not support TCP applications.
type ProxyService struct {
	connCfg            connection.Config
	effectiveLifetime  bot.CredentialLifetime
	cfg                *ProxyServiceConfig
	proxyPinger        connection.ProxyPinger
	log                *slog.Logger
	botClient          *apiclient.Client
	getBotIdentity     func() *identity.Identity
	botIdentityReadyCh <-chan struct{}
	statusReporter     readyz.Reporter
	identityGenerator  *identity.Generator
	clientBuilder      *client.Builder
	scoped             bool
	alpnUpgradeCache   *internal.ALPNUpgradeCache

	cache               *utils.FnCache
	proxyAddr           string
	alpnUpgradeRequired bool
}

// Run runs the service until the context is closed or an error occurs.
func (s *ProxyService) Run(ctx context.Context) error {
	ctx, span := tracer.Start(ctx, "ProxyService/Run")
	defer span.End()

	l := s.cfg.Listener
	if l == nil {
		s.log.DebugContext(
			ctx, "Opening listener for application proxy",
			"listen", s.cfg.Listen,
		)
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

	// Initialize the fnCache
	fnCache, err := utils.NewFnCache(utils.FnCacheConfig{
		TTL: s.effectiveLifetime.RenewalInterval,
		// Otherwise a failed issuance is served to every request for that
		// app until the TTL expires.
		ReloadOnErr: true,
	})
	if err != nil {
		return trace.Wrap(err, "initializing cache")
	}
	s.cache = fnCache

	// Ping the Teleport Proxy
	proxyPing, err := s.proxyPinger.Ping(ctx)
	if err != nil {
		return trace.Wrap(err, "pinging upstream proxy")
	}

	// Retrieve the Proxy Address to use for the upstream request
	proxyAddr, err := proxyPing.ProxyWebAddr()
	if err != nil {
		return trace.Wrap(err, "determining proxy address from ping response")
	}
	s.proxyAddr = proxyAddr

	// Check if ALPN upgrade will be required to pass client certificate to the
	// proxy.
	s.alpnUpgradeRequired, err = s.alpnUpgradeCache.IsUpgradeRequired(
		ctx, proxyAddr, s.connCfg.Insecure,
	)
	if err != nil {
		return trace.Wrap(err, "testing if ALPN connection upgrade is required")
	}

	httpSrv := http.Server{
		ReadTimeout:       *s.cfg.HTTP.ReadTimeout,
		ReadHeaderTimeout: s.cfg.HTTP.ReadHeaderTimeout,
		WriteTimeout:      *s.cfg.HTTP.WriteTimeout,
		IdleTimeout:       s.cfg.HTTP.IdleTimeout,
		BaseContext: func(net.Listener) context.Context {
			// Use the main context which controls the service being stopped as
			// the base context for all incoming requests.
			return ctx
		},
		Handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			err := s.handleProxyRequest(w, req)
			if err != nil {
				trace.WriteError(w, err)
				s.log.ErrorContext(
					req.Context(), "Encountered an error while proxying request",
					"error", err,
				)
				return
			}
		}),
	}
	s.log.InfoContext(ctx, "Finished initializing")

	errCh := make(chan error, 1)
	go func() {
		s.log.DebugContext(ctx, "Starting proxy request handler goroutine")
		errCh <- httpSrv.Serve(l)
	}()
	s.log.InfoContext(
		ctx, "Listening for proxy connections",
		"address", l.Addr().String(),
	)
	s.statusReporter.Report(readyz.Healthy)

	select {
	case <-ctx.Done():
		return trace.Wrap(httpSrv.Close(), "closing http server")
	case err := <-errCh:
		s.statusReporter.ReportReason(readyz.Unhealthy, err.Error())
		return trace.Wrap(err, "local proxy failed")
	}
}

// String returns a human friendly representation of the service.
func (s *ProxyService) String() string {
	return cmp.Or(
		s.cfg.Name,
		fmt.Sprintf("%s:%s", ProxyServiceType, s.cfg.Listen),
	)
}

// issueCert issues a role-impersonated app-routed X509 certificate
func (s *ProxyService) issueCert(
	ctx context.Context,
	host string,
) (*tls.Certificate, types.Application, error) {
	ctx, span := tracer.Start(ctx, "ProxyService/issueCert")
	defer span.End()

	var (
		routedIdent *identity.Identity
		app         types.Application
		err         error
	)
	if s.scoped {
		routedIdent, app, err = s.generateScopedIdentity(ctx, host)
	} else {
		routedIdent, app, err = s.generateUnscopedIdentity(ctx, host)
	}
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return routedIdent.TLSCert, app, nil
}

// generateUnscopedIdentity generates a role-impersonated identity, creates a
// client from it, and looks up the application by name the request host is
// treated as the application name.
func (s *ProxyService) generateUnscopedIdentity(ctx context.Context, appName string) (*identity.Identity, types.Application, error) {
	// TODO(noah): At a later date, we should consider running a background
	// goroutine that maintains a role-impersonated client. We should probably
	// make this a generic helper since we have a few services that do this now.
	// This will avoid the need to create and destroy a client for each new
	// upstream.

	// Right now we have to redetermine the route to app each time as the
	// session ID may need to change. Once v17 hits, this will be automagically
	// calculated by the auth server on cert generation, and we can fetch the
	// routeToApp once.
	identityOpts := []identity.GenerateOption{
		identity.WithLifetime(s.effectiveLifetime.TTL, s.effectiveLifetime.RenewalInterval),
		identity.WithLogger(s.log),
	}
	if s.cfg.DelegationSessionID != "" {
		identityOpts = append(identityOpts, identity.WithDelegation(s.cfg.DelegationSessionID))
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
			s.log.ErrorContext(
				ctx, "Failed to close impersonated client",
				"error", err,
			)
		}
	}()

	// TODO(noah): Now that app session ids are no longer being retrieved,
	// we can begin to cache the getApp rather than regenerating this
	// on each renew in the ProxyService
	app, err := getAppLegacy(ctx, impersonatedClient, appName)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	routeToApp := routeForApp(app, s.getBotIdentity().ClusterName)

	routedIdent, err := s.identityGenerator.Generate(
		ctx, append(identityOpts, identity.WithRouteToApp(routeToApp))...,
	)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return routedIdent, app, nil
}

// generateScopedIdentity resolves the scoped application identified by host and
// generates an identity routed to it. Since application names are only unique
// within a scope, host must be the scope-qualified hash used as the first label
// of the application's public address. The scoped bot client resolves that
// label to an application the bot is permitted to access.
func (s *ProxyService) generateScopedIdentity(ctx context.Context, host string) (*identity.Identity, types.Application, error) {
	app, err := getAppByPublicAddrPrefix(ctx, s.botClient, host)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	routeToApp := routeForApp(app, s.getBotIdentity().ClusterName)

	s.log.DebugContext(ctx, "Requesting issuance of certificate for tunnel proxy.")
	routedIdent, err := s.identityGenerator.Generate(ctx,
		identity.WithLifetime(s.effectiveLifetime.TTL, s.effectiveLifetime.RenewalInterval),
		identity.WithLogger(s.log),
		identity.WithRouteToApp(routeToApp),
	)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return routedIdent, app, nil
}

// handleProxyRequest handles incoming HTTP requests to the server.
//
// This roughly implements HTTP proxying as defined within RFC2616.
// On receiving a HTTP request, we determine the upstream target application
// from the Host header, and issue a certificate for that application. The proxy
// then makes the upstream request to the Teleport Proxy for that application.
//
// It does not support HTTP "tunneling" as defined by RFC2616 via the CONNECT
// method. CONNECT requests are rejected with a 501 Not Implemented response.
//
// We currently do not account for HTTP/2 support because we have an unencrypted
// listener. By default, http.Server will only support HTTP/2 if TLS is enabled.
// At a later date, we could add support for h2c with prior-knowledge, but this
// would introduce significant additional complexity to this proxy - so we shall
// defer until there is a demonstrated need.
func (s *ProxyService) handleProxyRequest(w http.ResponseWriter, req *http.Request) error {
	ctx := req.Context()
	s.log.DebugContext(
		ctx, "Received request to proxy",
		"url", req.URL.String(),
		"host", req.Host,
		"method", req.Method,
		"remote_addr", req.RemoteAddr,
	)

	// Pre-emptively block CONNECT requests as we do not currently support them
	// but, proxying them forward as normal requests would make it difficult for
	// us to introduce CONNECT support later without breaking compat.
	if req.Method == http.MethodConnect {
		return trace.NotImplemented("proxy does not support CONNECT method")
	}

	// net/http implements RFC7230 5.3 correctly, and will prefer the host
	// specified within an absolute-form request-target over the Host header
	// when setting req.Host - which means we can safely use req.Host here.
	var appCert *tls.Certificate
	var err error
	appCert, err = utils.FnCacheGet(
		ctx,
		s.cache,
		req.Host,
		func(ctx context.Context) (*tls.Certificate, error) {
			s.log.InfoContext(
				ctx, "Issuing app cert",
				"app", req.Host,
			)
			cert, _, err := s.issueCert(ctx, req.Host)
			return cert, err
		})
	if err != nil {
		return trace.Wrap(err, "fetching certificate")
	}

	transport := &http.Transport{
		// The transport is discarded after this request, so a kept-alive
		// connection would otherwise linger until the proxy closes it.
		// This eliminates the need to defer a call to transport.CloseIdleConns()
		DisableKeepAlives: true,
		// The ALPN dialer completes the TLS handshake, so it is wired in as
		// DialTLSContext rather than DialContext.
		DialTLSContext: apiclient.NewALPNDialer(apiclient.ALPNDialerConfig{
			DialTimeout:             s.cfg.HTTP.DialTimeout,
			ALPNConnUpgradeRequired: s.alpnUpgradeRequired,
			TLSConfig: &tls.Config{
				Certificates:       []tls.Certificate{*appCert},
				InsecureSkipVerify: s.botClient.Config().InsecureSkipVerify,
				NextProtos:         []string{string(common.ProtocolHTTP)},
			},
			GetClusterCAs: func(context.Context) (*x509.CertPool, error) {
				return s.getBotIdentity().TLSCAPool, nil
			},
		}).DialContext,
		ResponseHeaderTimeout: s.cfg.HTTP.UpstreamResponseHeaderTimeout,
	}

	// TODO(noah): We could cache the httpClient for each upstream, this
	// would potentially allow performance improvements by caching connections.
	// The transport settings would need to be reconsidered, i.e DisableKeepAlives.
	httpClient := &http.Client{
		Timeout:   s.cfg.HTTP.UpstreamRequestTimeout,
		Transport: transport,
		// Per RFC 9110, our proxy is an intermediary and redirects should be
		// handled by the user agent, so forward them on to the client rather than
		// handling them on their behalf.
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Build the Application Request
	upstreamReq := req.Clone(req.Context())
	// For now, we redirect all requests to the proxy web's address. However,
	// it would be a potential improvement for us to switch to fetching the
	// public address from the App resource. This would reduce the chance of
	// breakage if the Proxy is eventually modified to consider SNI. The problem
	// with doing this is that the public address does not include a port, which
	// we could guess from the Proxy's public web address.
	upstreamReq.Host = s.proxyAddr
	upstreamReq.URL.Host = s.proxyAddr
	upstreamReq.URL.Scheme = "https"
	// RequestURI must be empty when making client requests.
	upstreamReq.RequestURI = ""
	// TODO: Are there any headers we should override, add, or remove on the
	// upstream request?

	// Execute the upstream request
	resp, err := httpClient.Do(upstreamReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Prepare and send the response back to the client
	// Copy headers first.
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Set(key, value)
		}
	}
	// Send status code and then copy the body.
	w.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(w, resp.Body); err != nil {
		return trace.Wrap(err, "copying response body")
	}

	return nil
}
