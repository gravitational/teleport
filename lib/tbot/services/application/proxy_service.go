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
	"crypto/sha256"
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
	httpClient          *http.Client
	proxyAddr           string
	proxyPort           string
	alpnUpgradeRequired bool
}

// upstreamDialTarget binds a transport connection-pool identity to the exact
// client certificate that must authenticate connections in that pool.
// Both are cached together in FnCache and treated as immutable after creation.
type upstreamDialTarget struct {
	certificate *tls.Certificate
	poolAddr    string
}

type upstreamDialTargetContextKey struct{}

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

	fnCache, err := utils.NewFnCache(utils.FnCacheConfig{
		Context: ctx,
		TTL:     s.effectiveLifetime.RenewalInterval,
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
	_, s.proxyPort, err = net.SplitHostPort(proxyAddr)
	if err != nil {
		return trace.Wrap(err, "determining proxy port")
	}

	// Check if ALPN upgrade will be required to pass client certificate to the
	// proxy.
	s.alpnUpgradeRequired, err = s.alpnUpgradeCache.IsUpgradeRequired(
		ctx, proxyAddr, s.connCfg.Insecure,
	)
	if err != nil {
		return trace.Wrap(err, "testing if ALPN connection upgrade is required")
	}

	// Share one transport across applications so MaxIdleConns bounds the total
	// idle connections, including pools for previously issued certificates.
	// Transport also owns idle eviction, timeouts, and synchronization.
	transport := &http.Transport{
		DialTLSContext:        s.dialUpstream,
		DisableKeepAlives:     s.cfg.HTTP.UpstreamDisableKeepAlives,
		IdleConnTimeout:       s.cfg.HTTP.UpstreamIdleConnTimeout,
		MaxIdleConns:          s.cfg.HTTP.UpstreamMaxIdleConns,
		ResponseHeaderTimeout: s.cfg.HTTP.UpstreamResponseHeaderTimeout,
	}
	defer transport.CloseIdleConnections()
	s.httpClient = &http.Client{
		Timeout:   s.cfg.HTTP.UpstreamRequestTimeout,
		Transport: transport,
		// Per RFC 9110, our proxy is an intermediary and redirects should be
		// handled by the user agent, so forward them on to the client rather than
		// handling them on their behalf.
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
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

func (s *ProxyService) newUpstreamDialTarget(host string, certificate *tls.Certificate) (*upstreamDialTarget, error) {
	if certificate == nil || len(certificate.Certificate) == 0 {
		return nil, trace.Errorf("application certificate has no certificate chain")
	}

	// Transport pools HTTPS connections by origin, without considering the
	// client certificate. Every app dials the same Teleport Proxy, so using its
	// address as the origin would allow connections to cross app identities.
	// Hash the app name and full certificate chain to give each identity its
	// own pool. A renewed chain selects a new pool; existing connections in the
	// old pool are left to the transport's idle timeout and global idle limit.
	hash := sha256.New()
	_, _ = hash.Write([]byte(host))
	// Separate the variable-length app name from the DER certificate chain.
	_, _ = hash.Write([]byte{0})
	for _, certificateDER := range certificate.Certificate {
		_, _ = hash.Write(certificateDER)
	}
	digest := hash.Sum(nil)

	return &upstreamDialTarget{
		certificate: certificate,
		poolAddr: net.JoinHostPort(
			// A 32-byte digest becomes 64 hex characters. Split it into two
			// 32-character labels to stay within DNS's 63-character label limit,
			// even though this synthetic hostname must never reach DNS.
			// .invalid is a reserved domain suffix specifically intended for
			// deliberately invalid names (RFC 2606). It makes the synthetic authority
			// visibly artificial and avoids collisions with real public domains.
			// Go’s transport gives it no special pooling behavior. Our dialer must
			// still ensure it is never resolved or dialed.
			fmt.Sprintf("%x.%x.invalid", digest[:16], digest[16:]),
			// Include the discovered port explicitly: omitting it would make
			// Transport supply HTTPS's default :443, even for another proxy port.
			s.proxyPort,
		),
	}, nil
}

// dialUpstream establishes an application-authenticated connection to the
// Teleport Proxy. addr is an opaque connection-pool identity and is never
// resolved or dialed directly.
func (s *ProxyService) dialUpstream(ctx context.Context, network, addr string) (net.Conn, error) {
	target, ok := ctx.Value(upstreamDialTargetContextKey{}).(*upstreamDialTarget)
	if !ok {
		return nil, trace.Errorf("application dial target missing from context")
	}
	if addr != target.poolAddr {
		return nil, trace.Errorf("unexpected application connection pool address %q", addr)
	}

	// Always pass the real address to the ALPN dialer. Besides the network
	// destination, it determines TLS ServerName when no explicit name is set.
	return apiclient.NewALPNDialer(apiclient.ALPNDialerConfig{
		DialTimeout:             s.cfg.HTTP.DialTimeout,
		ALPNConnUpgradeRequired: s.alpnUpgradeRequired,
		TLSConfig: &tls.Config{
			Certificates:       []tls.Certificate{*target.certificate},
			InsecureSkipVerify: s.botClient.Config().InsecureSkipVerify,
			// Keep HTTP/1.1, whose pools are bounded by Transport.MaxIdleConns.
			NextProtos: []string{string(common.ProtocolHTTP)},
		},
		GetClusterCAs: func(context.Context) (*x509.CertPool, error) {
			return s.getBotIdentity().TLSCAPool, nil
		},
	}).DialContext(ctx, network, s.proxyAddr)
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
	target, err := utils.FnCacheGet(
		ctx,
		s.cache,
		req.Host,
		func(ctx context.Context) (*upstreamDialTarget, error) {
			s.log.InfoContext(
				ctx, "Issuing app cert",
				"app", req.Host,
			)
			certificate, _, err := s.issueCert(ctx, req.Host)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return s.newUpstreamDialTarget(req.Host, certificate)
		})
	if err != nil {
		return trace.Wrap(err, "fetching application certificate")
	}

	// DialTLSContext receives only a context, network, and address. The address
	// identifies the pool but does not contain its certificate, and looking the
	// certificate up again could race with renewal. Transport retains request
	// context values in the dial context, including when it detaches a
	// speculative dial from request cancellation, so this immutable value binds
	// the selected pool and certificate without another cache.
	upstreamCtx := context.WithValue(req.Context(), upstreamDialTargetContextKey{}, target)
	// Build the Application Request
	upstreamReq := req.Clone(upstreamCtx)
	// URL.Host is an opaque origin used only as the transport's connection-pool
	// identity. The dialer validates it and connects to the real proxy address.
	upstreamReq.URL.Host = target.poolAddr
	// Request.Host controls the HTTP Host header independently of URL.Host,
	// so the synthetic pool identity is not sent as the proxy's Host header.
	upstreamReq.Host = s.proxyAddr
	upstreamReq.URL.Scheme = "https"
	// RequestURI must be empty when making client requests.
	upstreamReq.RequestURI = ""
	// TODO: Are there any headers we should override, add, or remove on the
	// upstream request?

	// Execute the upstream request
	resp, err := s.httpClient.Do(upstreamReq)
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
