package application

import (
	"cmp"
	"context"
	"crypto/tls"
	"fmt"
	"go.opentelemetry.io/otel"
	"io"
	"log/slog"
	"net/http"
	"slices"

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

	// lp.Start will block and continues to block until lp.Close() is called.
	// Despite taking a context, it will not exit until the first connection is
	// made after the context is canceled.
	var errCh = make(chan error, 1)
	go func() {
		s.log.DebugContext(ctx, "Starting proxy goroutine")
		errCh <- s.startProxy()
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
		fmt.Sprintf("%s:%s:%s", ProxyServiceType, s.cfg.Listen, s.cfg.Applications),
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

func (s *ProxyService) startProxy() error {
	router := http.NewServeMux()
	// This router expects the requests to come in via the style "GET <fqdn>:<port>/"
	// It doesn't really consider the CONNECT method, but it should work nonetheless.
	router.HandleFunc("/", s.handleProxyRequest)

	err := http.ListenAndServe(s.cfg.Listen, router)
	if err != nil {
		return err
	}

	return nil
}

func (s *ProxyService) handleProxyRequest(w http.ResponseWriter, req *http.Request) {
	// TODO: Better exception handling here is needed. We might have to consider the service unhealthy if requests fail.

	// Resolve Application Name via either URL or Host Header
	appName := cmp.Or(req.URL.Host, req.Header.Get("Host"), req.Header.Get("host"))

	// Validate against Application whitelist (if there is any)
	if s.cfg.Applications != nil && !slices.Contains(s.cfg.Applications, appName) {
		http.Error(w, "invalid application", http.StatusUnauthorized)
		return
	}

	ctx := req.Context()

	// TODO: We could cache these for their lifetime in a 'registry'
	appCert, _, err := s.issueCert(ctx, appName)

	if err != nil {
		http.Error(w, "An internal error occurred", http.StatusInternalServerError)
		s.log.ErrorContext(ctx, trace.Wrap(err, "Error getting application certificate").Error())
	}

	// TODO: We could cache this object in memory for future requests and just break it down when the certificates expire.
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				Certificates:       []tls.Certificate{*appCert},
				InsecureSkipVerify: true,
			},
		},
	}

	// Ping the Teleport Proxy
	// TODO: The ping increases the latency, we could probably avoid this.
	proxyPing, err := s.proxyPinger.Ping(ctx)
	if err != nil {
		s.log.ErrorContext(ctx, trace.Wrap(err, "error pinging proxy").Error())
	}

	// Retrieve the Proxy Address to use for the Application Request
	proxyAddr, err := proxyPing.ProxyWebAddr()

	// Execute the Application Request
	result, err := httpClient.Get("https://" + proxyAddr)
	if err != nil {
		fmt.Printf("Error getting proxy response: %v\n", err)
	}

	// Transfer all headers
	for key, values := range result.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Add extra Teleport header
	w.Header().Add("X-Teleport-Application", appName)

	// Write the StatusCode
	w.WriteHeader(result.StatusCode)

	// Write the Body
	_, bodyCopyError := io.Copy(w, result.Body)
	if bodyCopyError != nil {
		return
	}
}
