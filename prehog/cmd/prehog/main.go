package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	stdlog "log" //nolint:depguard
	"net"
	"net/http"
	"net/http/pprof"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	grpcreflect "github.com/bufbuild/connect-grpcreflect-go"
	"github.com/mattn/go-isatty"
	"github.com/pires/go-proxyproto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	prehogv1c "github.com/gravitational/prehog/gen/proto/prehog/v1alpha/prehogv1alphaconnect"
	"github.com/gravitational/prehog/lib/antihog"
	"github.com/gravitational/prehog/lib/authn"
	client "github.com/gravitational/prehog/lib/posthog"
	"github.com/gravitational/prehog/lib/prehog"
)

type config struct {
	listenAddr    string
	certFile      string
	keyFile       string
	proxyProtocol bool

	diagAddr  string
	diagDebug bool

	posthogURL     *url.URL
	apiKey         string
	clientCertFile string
	clientKeyFile  string

	antihogAutocapture bool
}

func main() {
	if isatty.IsTerminal(os.Stderr.Fd()) {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	} else {
		zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	}

	stdlog.SetFlags(0)
	stdlog.SetOutput(log.Logger)

	var cfg config
	flag.StringVar(&cfg.listenAddr, "listen-addr", ":8443", "listen address for the API server")
	flag.StringVar(&cfg.certFile, "cert", "", "path to listener cert for the API server (required)")
	flag.StringVar(&cfg.keyFile, "key", "", "path to listener key for the API server (required)")
	flag.BoolVar(&cfg.proxyProtocol, "proxy-protocol", false, "parse proxy protocol headers on incoming connections")

	flag.StringVar(&cfg.diagAddr, "diag-addr", "127.0.0.1:3000", "listen address for the diagnostics server")
	flag.BoolVar(&cfg.diagDebug, "diag-debug", false, "enable /debug/pprof/ endpoints")

	posthogURL := flag.String("posthog-url", "", "PostHog URL (required)")
	flag.StringVar(&cfg.apiKey, "api-key", "", "PostHog API key (required, default $PREHOG_API_KEY)")
	flag.StringVar(&cfg.clientCertFile, "client-cert", "", "path to client cert for PostHog (optional)")
	flag.StringVar(&cfg.clientKeyFile, "client-key", "", "path to client key for PostHog (optional)")

	flag.BoolVar(&cfg.antihogAutocapture, "autocapture", false, "enable autocapture in posthog-js")

	flag.Parse()

	if cfg.certFile == "" {
		fmt.Fprintln(os.Stderr, "missing cert")
		flag.Usage()
		os.Exit(2)
	}
	if cfg.keyFile == "" {
		fmt.Fprintln(os.Stderr, "missing key")
		flag.Usage()
		os.Exit(2)
	}
	if *posthogURL == "" {
		fmt.Fprintln(os.Stderr, "missing posthog-url")
		flag.Usage()
		os.Exit(2)
	}
	if u, err := url.Parse(*posthogURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		fmt.Fprintln(os.Stderr, "invalid posthog-url")
		flag.Usage()
		os.Exit(2)
	} else {
		cfg.posthogURL = u
	}
	if cfg.apiKey == "" {
		cfg.apiKey = os.Getenv("PREHOG_API_KEY")
	}
	if cfg.apiKey == "" {
		fmt.Fprintln(os.Stderr, "missing api-key")
		flag.Usage()
		os.Exit(2)
	}
	if cfg.clientCertFile != "" && cfg.clientKeyFile == "" {
		fmt.Fprintln(os.Stderr, "client-key must be set if client-cert is set")
		flag.Usage()
		os.Exit(2)
	}

	if err := run(cfg); err != nil {
		log.Fatal().Err(err).Send()
	}
}

func run(cfg config) error {
	listenerCert, err := tls.LoadX509KeyPair(cfg.certFile, cfg.keyFile)
	if err != nil {
		return fmt.Errorf("failed to load cert: %w", err)
	}

	var clientCert *tls.Certificate
	if cfg.clientCertFile != "" {
		cc, err := tls.LoadX509KeyPair(cfg.clientCertFile, cfg.clientKeyFile)
		if err != nil {
			return fmt.Errorf("failed to load client cert: %w", err)
		}
		clientCert = &cc
	}

	go runDiagServer(cfg.diagAddr, cfg.diagDebug)

	posthogClient := client.NewClient(cfg.posthogURL, cfg.apiKey, clientCert)
	serviceHandler := prehog.NewHandler(posthogClient)

	mux := http.NewServeMux()

	pat, h := prehogv1c.NewTeleportReportingServiceHandler(serviceHandler)
	mux.Handle(pat, authn.RequireLicense(h))

	pat, h = prehogv1c.NewSalesReportingServiceHandler(serviceHandler)
	mux.Handle(pat, authn.RequireCA(h))

	antihogHandler := antihog.NewHandler(posthogClient, cfg.antihogAutocapture)
	antihogHandler.Install(mux.HandleFunc)

	reflector := grpcreflect.NewStaticReflector(
		prehogv1c.TeleportReportingServiceName,
		prehogv1c.SalesReportingServiceName,
	)
	mux.Handle(grpcreflect.NewHandlerV1(reflector))
	mux.Handle(grpcreflect.NewHandlerV1Alpha(reflector))

	mux.HandleFunc("/livez", func(http.ResponseWriter, *http.Request) {})

	h = mux
	h = promhttp.InstrumentHandlerDuration(prehog.ApiRequestsDuration, h)
	h = promhttp.InstrumentHandlerCounter(prehog.ApiRequestsTotal, h)
	h = promhttp.InstrumentHandlerInFlight(prehog.ApiRequestsInFlight, h)

	srv := &http.Server{
		Handler: h,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{listenerCert},
			ClientAuth:   tls.VerifyClientCertIfGiven,
			ClientCAs:    authn.KnownLicenseCAs,
			MinVersion:   tls.VersionTLS12,
		},
		ConnContext: authn.ConnContext(authn.KnownLicenseCAs),

		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: time.Second,
		WriteTimeout:      10 * time.Second,

		ErrorLog: httpServerLogger("api"),
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	log.Err(posthogClient.Check(ctx)).Msg("PostHog connection check")

	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", cfg.listenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	if cfg.proxyProtocol {
		log.Info().Msg("Enabling proxy protocol on api listener")
		listener = &proxyproto.Listener{Listener: listener}
	}

	go func() {
		defer cancel()
		log.Info().Msg("Serving api")
		err := srv.ServeTLS(listener, "", "")
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		log.Err(err).Msg("Done serving api")
	}()

	<-ctx.Done()
	cancel()

	log.Info().Msg("Shutting down api server")
	err = srv.Shutdown(context.Background())
	log.Err(err).Msg("Shut down api server")

	return nil
}

func httpServerLogger(name string) *stdlog.Logger {
	return stdlog.New(
		log.With().
			Str("server", name).
			CallerWithSkipFrameCount(5). // by manual inspection of net/http in go 1.19
			Logger(),
		"", 0)
}

func runDiagServer(addr string, enableDebug bool) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/livez", func(http.ResponseWriter, *http.Request) {})
	if enableDebug {
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
		log.Warn().Msg("Enabled /debug/pprof/ endpoints on diag")
	}

	diagSrv := &http.Server{
		Addr:        addr,
		Handler:     mux,
		ReadTimeout: 5 * time.Second,
		ErrorLog:    httpServerLogger("diag"),
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Err(err).Msg("Failed to listen for diag")
		return
	}
	listener = &proxyproto.Listener{Listener: listener}
	log.Info().Msg("Serving diag")
	log.Err(diagSrv.Serve(listener)).Msg("Done serving diag")
}
