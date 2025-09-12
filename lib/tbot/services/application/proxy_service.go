package application

import (
	"cmp"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"time"

	"go.opentelemetry.io/otel"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/lib/defaults"

	"github.com/gravitational/trace"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/connection"
	"github.com/gravitational/teleport/lib/tbot/client"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/tbot/internal"
	"github.com/gravitational/teleport/lib/tbot/readyz"
	"github.com/gravitational/teleport/lib/utils"
)

var applicationProxyTracer = otel.Tracer("github.com/gravitational/teleport/lib/tbot/services/applicationproxy")

func ProxyServiceBuilder(
	cfg *ProxyServiceConfig,
	connCfg connection.Config,
	defaultCredentialLifetime bot.CredentialLifetime,
) bot.ServiceBuilder {
	return func(deps bot.ServiceDependencies) (bot.Service, error) {
		if err := cfg.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		svc := &ProxyService{
			connCfg:                   connCfg,
			defaultCredentialLifetime: defaultCredentialLifetime,
			getBotIdentity:            deps.BotIdentity,
			botIdentityReadyCh:        deps.BotIdentityReadyCh,
			proxyPinger:               deps.ProxyPinger,
			botClient:                 deps.Client,
			cfg:                       cfg,
			identityGenerator:         deps.IdentityGenerator,
			clientBuilder:             deps.ClientBuilder,
		}

		svc.log = deps.LoggerForService(svc)
		svc.statusReporter = deps.StatusRegistry.AddService(svc.String())
		return svc, nil
	}
}

type ProxyService struct {
	connCfg                   connection.Config
	defaultCredentialLifetime bot.CredentialLifetime
	cfg                       *ProxyServiceConfig
	proxyPinger               connection.ProxyPinger
	log                       *slog.Logger
	botClient                 *apiclient.Client
	getBotIdentity            func() *identity.Identity
	botIdentityReadyCh        <-chan struct{}
	statusReporter            readyz.Reporter
	identityGenerator         *identity.Generator
	clientBuilder             *client.Builder

	cache     *utils.FnCache
	proxyAddr string
	proxyUrl  *url.URL
}

func (s *ProxyService) Run(ctx context.Context) error {
	ctx, span := applicationProxyTracer.Start(ctx, "ProxyService/Run")
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

	// Initialize the fnCache
	fnCache, err := utils.NewFnCache(utils.FnCacheConfig{
		// TODO: More appropriate cache time?
		TTL: 1 * time.Minute,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	s.cache = fnCache

	// lp.Start will block and continues to block until lp.Close() is called.
	// Despite taking a context, it will not exit until the first connection is
	// made after the context is canceled.
	var errCh = make(chan error, 1)
	go func() {
		s.log.DebugContext(ctx, "Starting proxy goroutine")
		errCh <- s.startProxy(ctx)
	}()
	s.log.InfoContext(ctx, "Listening for proxy connections.", "address", l.Addr().String())

	s.statusReporter.Report(readyz.Healthy)

	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		s.statusReporter.ReportReason(readyz.Unhealthy, err.Error())
		return trace.Wrap(err, "local proxy failed")
	}
}

func (s *ProxyService) String() string {
	return cmp.Or(
		s.cfg.Name,
		fmt.Sprintf("%s:%s", ProxyServiceType, s.cfg.Listen),
	)
}

func (s *ProxyService) issueCert(
	ctx context.Context,
	appName string,
) (*tls.Certificate, types.Application, error) {
	ctx, span := applicationProxyTracer.Start(ctx, "ProxyService/issueCert")
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
	route, app, err := getRouteToApp(ctx, s.getBotIdentity(), impersonatedClient, appName)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	s.log.DebugContext(ctx, "Requesting issuance of certificate for ProxyService proxy.")
	routedIdent, err := s.identityGenerator.Generate(ctx, append(identityOpts, identity.WithRouteToApp(route))...)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	s.log.InfoContext(ctx, "Certificate issued for ProxyService proxy.")

	return routedIdent.TLSCert, app, nil
}

func (s *ProxyService) startProxy(ctx context.Context) error {
	// Ping the Teleport Proxy
	proxyPing, err := s.proxyPinger.Ping(ctx)
	if err != nil {
		return trace.Wrap(err, "pinging upstream proxy")
	}

	// Retrieve the Proxy Address to use for the Application Request
	proxyAddr, err := proxyPing.ProxyWebAddr()
	if err != nil {
		return trace.Wrap(err, "determining proxy address from ping response")
	}
	s.proxyAddr = proxyAddr

	// Build the proxy url to use in the http Client.
	proxyUrl, err := url.Parse("https://" + proxyAddr)
	if err != nil {
		return err
	}
	s.proxyUrl = proxyUrl

	// TODO: Check if ALPN Upgrade is required - if so, implement it here or
	// throw an error for this MVP.

	// This router expects the requests to come in via the style "GET <fqdn>:<port>/"
	// It doesn't really consider the CONNECT method, but it should work nonetheless.
	proxyHttpServer := http.Server{
		Addr:              s.cfg.Listen,
		ReadTimeout:       apidefaults.DefaultIOTimeout,
		ReadHeaderTimeout: defaults.ReadHeadersTimeout,
		WriteTimeout:      apidefaults.DefaultIOTimeout,
		IdleTimeout:       apidefaults.DefaultIdleTimeout,
		BaseContext: func(net.Listener) context.Context {
			// Use the main context which controls the service being stopped as
			// the base context for all incoming requests.
			return ctx
		},
	}
	proxyHttpServer.Handler = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		err := s.handleProxyRequest(w, req)
		if err != nil {
			s.handleProxyError(w, err)
			return
		}
	})

	err = proxyHttpServer.ListenAndServe()
	if err != nil {
		return err
	}

	return nil
}

func (s *ProxyService) handleProxyError(w http.ResponseWriter, err error) {
	trace.WriteError(w, err)
	s.log.Error("Encountered an error while proxying request", "error", err)
}

func (s *ProxyService) handleProxyRequest(w http.ResponseWriter, req *http.Request) error {
	// Resolve Application Name via either URL or Host Header
	appName := cmp.Or(req.URL.Host, req.Header.Get("Host"))

	// Pre-emptively block CONNECT requests as we do not currently support them
	// but, proxying them forward as normal requests would make it difficult for
	// us to introduce CONNECT support later without breaking compat.
	if req.Method == http.MethodConnect {
		return trace.NotImplemented("Proxy does not support CONNECT method")
	}

	ctx := req.Context()

	var appCert *tls.Certificate
	var err error
	appCert, err = utils.FnCacheGet(ctx, s.cache, appName, func(ctx context.Context) (*tls.Certificate, error) {
		s.log.InfoContext(ctx, fmt.Sprintf("(Re)issuing application Certificate for %s\n", appName))
		cert, _, err := s.issueCert(ctx, appName)
		return cert, err
	})
	if err != nil {
		return err
	}

	// TODO(noah): We could cache the httpClient itself for each upstream, this
	// would potentially allow performance improvements by caching connections.
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				Certificates:       []tls.Certificate{*appCert},
				InsecureSkipVerify: s.botClient.Config().InsecureSkipVerify,
			},
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
		return err
	}

	return nil
}
