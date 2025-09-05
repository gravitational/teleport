package application

import (
	"cmp"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"go.opentelemetry.io/otel"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/lib/defaults"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/connection"
	"github.com/gravitational/teleport/lib/tbot/client"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/tbot/internal"
	"github.com/gravitational/teleport/lib/tbot/readyz"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

var applicationProxyTracer = otel.Tracer("github.com/gravitational/teleport/lib/tbot/services/applicationproxy")

var filteredUpstreamHeaders = map[string]struct{}{
	"Host":            {},
	"Accept-Encoding": {},
}

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
		return err
	}

	// Retrieve the Proxy Address to use for the Application Request
	proxyAddr, err := proxyPing.ProxyWebAddr()
	if err != nil {
		return err
	}
	s.proxyAddr = proxyAddr

	// Build the proxy url to use in the http Client.
	proxyUrl, err := url.Parse("https://" + proxyAddr)
	if err != nil {
		return err
	}
	s.proxyUrl = proxyUrl

	// This router expects the requests to come in via the style "GET <fqdn>:<port>/"
	// It doesn't really consider the CONNECT method, but it should work nonetheless.
	proxyHttpServer := http.Server{
		Addr:              s.cfg.Listen,
		ReadTimeout:       apidefaults.DefaultIOTimeout,
		ReadHeaderTimeout: defaults.ReadHeadersTimeout,
		WriteTimeout:      apidefaults.DefaultIOTimeout,
		IdleTimeout:       apidefaults.DefaultIdleTimeout,
	}
	proxyHttpServer.Handler = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		err := s.handleProxyRequest(w, req)
		if err != nil {
			s.handleProxyError(w, err)
			ctx.Done()
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

	ctx := req.Context()

	var appCert *tls.Certificate
	var err error
	var cached bool

	if s.cfg.CertificateCaching {
		cached = true
		appCert, err = utils.FnCacheGet(ctx, s.cache, appName, func(ctx context.Context) (*tls.Certificate, error) {
			cached = false
			s.log.InfoContext(ctx, fmt.Sprintf("(Re)issuing application Certificate for %s\n", appName))
			cert, _, err := s.issueCert(ctx, appName)
			return cert, err
		})
	} else {
		appCert, _, err = s.issueCert(ctx, appName)
	}

	if err != nil {
		return err
	}

	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				Certificates:       []tls.Certificate{*appCert},
				InsecureSkipVerify: s.botClient.Config().InsecureSkipVerify,
			},
		},
	}

	// Build the Application Request
	upstreamRequest := http.Request{
		Proto:  "https",
		Method: req.Method,
		Body:   req.Body,
		Host:   s.proxyAddr,
		URL: &url.URL{
			Scheme:      "https",
			Host:        s.proxyAddr,
			Path:        req.URL.Path,
			RawQuery:    req.URL.RawQuery,
			RawFragment: req.URL.RawFragment,
		},
		Header: http.Header{},
	}

	// Transfer all request headers
	for header, values := range req.Header {
		if _, excluded := filteredUpstreamHeaders[http.CanonicalHeaderKey(header)]; excluded {
			continue // Skip excluded headers
		}
		for _, value := range values {
			upstreamRequest.Header.Add(header, value)
		}
	}

	// Execute the Application Request
	result, err := httpClient.Do(&upstreamRequest)
	if err != nil {
		return err
	}

	// Transfer all response headers
	for key, values := range result.Header {
		for _, value := range values {
			w.Header().Set(key, value)
		}
	}

	// Add extra Teleport header
	w.Header().Set("X-Teleport-Application", appName)
	w.Header().Set("X-Teleport-Application-Cached", strconv.FormatBool(cached))

	// Write the StatusCode
	w.WriteHeader(result.StatusCode)

	// Write the Body
	_, bodyCopyError := io.Copy(w, result.Body)
	if bodyCopyError != nil {
		return bodyCopyError
	}

	return nil
}
