/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

// Package web implements web proxy handler that provides
// web interface to view and connect to teleport nodes
package web

import (
	"cmp"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	gogoproto "github.com/gogo/protobuf/proto"
	"github.com/google/safetext/shsprintf"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/julienschmidt/httprouter"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	oteltrace "go.opentelemetry.io/otel/trace"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"golang.org/x/crypto/ssh"
	"golang.org/x/mod/semver"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	notificationsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/notifications/v1"
	"github.com/gravitational/teleport/api/mfa"
	apitracing "github.com/gravitational/teleport/api/observability/tracing"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/installers"
	"github.com/gravitational/teleport/api/utils/keys"
	apisshutils "github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/automaticupgrades"
	"github.com/gravitational/teleport/lib/client"
	dbrepl "github.com/gravitational/teleport/lib/client/db/repl"
	"github.com/gravitational/teleport/lib/client/sso"
	"github.com/gravitational/teleport/lib/defaults"
	dtconfig "github.com/gravitational/teleport/lib/devicetrust/config"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/httplib/csrf"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/multiplexer"
	"github.com/gravitational/teleport/lib/observability/tracing"
	"github.com/gravitational/teleport/lib/player"
	"github.com/gravitational/teleport/lib/plugin"
	"github.com/gravitational/teleport/lib/proxy"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/secret"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/readonly"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	"github.com/gravitational/teleport/lib/web/app"
	websession "github.com/gravitational/teleport/lib/web/session"
	"github.com/gravitational/teleport/lib/web/terminal"
	"github.com/gravitational/teleport/lib/web/ui"
)

const (
	// SSOLoginFailureMessage is a generic error message to avoid disclosing sensitive SSO failure messages.
	SSOLoginFailureMessage = "Failed to login. Please check Teleport's log for more details."

	// SSOLoginFailureInvalidRedirect is a slightly specific error message for
	// SSO failures related to the use of an invalid or disallowed login
	// callback URL in tsh login.
	SSOLoginFailureInvalidRedirect = "Failed to login due to a disallowed callback URL. Please check Teleport's log for more details."

	// webUIFlowLabelKey is a label that may be added to resources
	// created via the web UI, indicating which flow the resource was created on.
	// This label is used for enhancing UX in the web app, by showing icons related,
	// to the workflow it was added, or providing unique features to those resources.
	// Example values:
	// - github-actions-ssh: indicates that the resource was added via the Bot GitHub Actions SSH flow
	webUIFlowLabelKey = "teleport.internal/ui-flow"
	// IncludedResourceModeAll describes that only requestable resources should be returned.
	IncludedResourceModeRequestable = "requestable"
	// IncludedResourceModeAll describes that all resources, requestable and available, should be returned.
	IncludedResourceModeAll = "all"
	// DefaultFeatureWatchInterval is the default time in which the feature watcher
	// should ping the auth server to check for updated features
	DefaultFeatureWatchInterval = time.Minute * 5
	// findEndpointCacheTTL is the cache TTL for the find endpoint generic answer.
	// This cache is here to protect against accidental or intentional DDoS, the TTL must be low to quickly reflect
	// cluster configuration changes.
	findEndpointCacheTTL = 10 * time.Second
	// DefaultAgentUpdateJitterSeconds is the default jitter agents should wait before updating.
	DefaultAgentUpdateJitterSeconds = 60
)

// healthCheckAppServerFunc defines a function used to perform a health check
// to AppServer that can handle application requests (based on cluster name and
// public address).
type healthCheckAppServerFunc func(ctx context.Context, publicAddr string, clusterName string) error

// Handler is HTTP web proxy handler
type Handler struct {
	logger *slog.Logger

	sync.Mutex
	httprouter.Router
	cfg                  Config
	auth                 *sessionCache
	clock                clockwork.Clock
	limiter              *limiter.RateLimiter
	highLimiter          *limiter.RateLimiter
	healthCheckAppServer healthCheckAppServerFunc
	// sshPort specifies the SSH proxy port extracted
	// from configuration
	sshPort string

	// userConns tracks amount of current active connections with user certificates.
	userConns atomic.Int32

	// clusterFeatures contain flags for supported and unsupported features.
	clusterFeatures proto.Features

	// nodeWatcher is a services.NodeWatcher used by Assist to lookup nodes from
	// the proxy's cache and get nodes in real time.
	nodeWatcher *services.GenericWatcher[types.Server, readonly.Server]

	// tracer is used to create spans.
	tracer oteltrace.Tracer

	// findEndpointCache is used to cache the find endpoint answer. As this endpoint is unprotected and has high
	// rate-limits, each call must cause minimal work. The cached answer can be modulated after, for example if the
	// caller specified its Automatic Updates UUID or group.
	findEndpointCache *utils.FnCache

	// clusterMaintenanceConfig is used to cache the cluster maintenance config from the AUth Service.
	clusterMaintenanceConfigCache *utils.FnCache
}

// HandlerOption is a functional argument - an option that can be passed
// to NewHandler function
type HandlerOption func(h *Handler) error

// SetClock sets the clock on a handler
func SetClock(clock clockwork.Clock) HandlerOption {
	return func(h *Handler) error {
		h.clock = clock
		return nil
	}
}

type ProxySettingsGetter interface {
	GetProxySettings(ctx context.Context) (*webclient.ProxySettings, error)
}

// PresenceChecker is a function that executes an MFA prompt to enforce
// that a user is present.
type PresenceChecker = func(ctx context.Context, term io.Writer, maintainer client.PresenceMaintainer, sessionID string, mfaCeremony *mfa.Ceremony, opts ...client.PresenceOption) error

// Config represents web handler configuration parameters
type Config struct {
	// PluginRegistry handles plugin registration
	PluginRegistry plugin.Registry
	// Proxy is a reverse tunnel proxy that handles connections
	// to local cluster or remote clusters using unified interface
	Proxy reversetunnelclient.Tunnel
	// AuthServers is a list of auth servers this proxy talks to
	AuthServers utils.NetAddr
	// ProxyClient is a client that authenticated as proxy
	ProxyClient authclient.ClientI
	// ProxySSHAddr points to the SSH address of the proxy
	ProxySSHAddr utils.NetAddr
	// ProxyKubeAddr points to the Kube address of the proxy
	ProxyKubeAddr utils.NetAddr
	// ProxyWebAddr points to the web (HTTPS) address of the proxy
	ProxyWebAddr utils.NetAddr
	// ProxyPublicAddr contains web proxy public addresses.
	ProxyPublicAddrs []utils.NetAddr
	// GetProxyClientCertificate returns the proxy client certificate.
	GetProxyClientCertificate func() (*tls.Certificate, error)
	// CipherSuites is the list of cipher suites Teleport suppports.
	CipherSuites []uint16

	// FIPS mode means Teleport started in a FedRAMP/FIPS 140-2 compliant
	// configuration.
	FIPS bool

	// AccessPoint holds a cache to the Auth Server.
	AccessPoint authclient.ProxyAccessPoint

	// Emitter is event emitter
	Emitter apievents.Emitter

	// HostUUID is the UUID of this process.
	HostUUID string

	// Context is used to signal process exit.
	Context context.Context

	// StaticFS optionally specifies the HTTP file system to use.
	// Enables web UI if set.
	StaticFS http.FileSystem

	// CachedSessionLingeringThreshold specifies the time the session will linger
	// in the cache before getting purged after it has expired.
	// Defaults to cachedSessionLingeringThreshold if unspecified.
	CachedSessionLingeringThreshold *time.Duration

	// ClusterFeatures contains flags for supported/unsupported features.
	ClusterFeatures proto.Features

	// ProxySettings allows fetching the current proxy settings.
	ProxySettings ProxySettingsGetter

	// MinimalReverseTunnelRoutesOnly mode handles only the endpoints required for
	// a reverse tunnel agent to establish a connection.
	MinimalReverseTunnelRoutesOnly bool

	// PublicProxyAddr is used to template the public proxy address
	// into the installer script responses
	PublicProxyAddr string

	// ALPNHandler is the ALPN connection handler for handling upgraded ALPN
	// connection through a HTTP upgrade call.
	ALPNHandler ConnectionHandler

	// TraceClient is used to forward spans to the upstream collector for the UI
	TraceClient otlptrace.Client

	// Router is used to route ssh sessions to hosts
	Router *proxy.Router

	// SessionControl is used to determine if users are
	// allowed to spawn new sessions
	SessionControl SessionController

	// PROXYSigner is used to sign PROXY header and securely propagate client IP information
	PROXYSigner multiplexer.PROXYHeaderSigner

	// TracerProvider generates tracers to create spans with
	TracerProvider oteltrace.TracerProvider

	// HealthCheckAppServer is a function that checks if the proxy can handle
	// application requests.
	HealthCheckAppServer healthCheckAppServerFunc

	// UI provides config options for the web UI
	UI webclient.UIConfig

	// NodeWatcher is a services.NodeWatcher used by Assist to lookup nodes from
	// the proxy's cache and get nodes in real time.
	NodeWatcher *services.GenericWatcher[types.Server, readonly.Server]

	// PresenceChecker periodically runs the mfa ceremony for moderated
	// sessions.
	PresenceChecker PresenceChecker

	// AccessGraphAddr is the address of the Access Graph service GRPC API
	AccessGraphAddr utils.NetAddr

	// AutomaticUpgradesChannels is a map of all version channels used by the
	// proxy built-in version server to retrieve target versions. This is part
	// of the automatic upgrades.
	AutomaticUpgradesChannels automaticupgrades.Channels

	// IntegrationAppHandler handles App Access requests which use an Integration.
	IntegrationAppHandler app.ServerHandler

	// FeatureWatchInterval is the interval between pings to the auth server
	// to fetch new cluster features
	FeatureWatchInterval time.Duration

	// DatabaseREPLRegistry is used for retrieving database REPL.
	DatabaseREPLRegistry dbrepl.REPLRegistry
}

// SetDefaults ensures proper default values are set if
// not provided.
func (c *Config) SetDefaults() {
	c.ProxyClient = auth.WithGithubConnectorConversions(c.ProxyClient)

	if c.TracerProvider == nil {
		c.TracerProvider = tracing.NoopProvider()
	}

	if c.PresenceChecker == nil {
		c.PresenceChecker = client.RunPresenceTask
	}

	c.FeatureWatchInterval = cmp.Or(c.FeatureWatchInterval, DefaultFeatureWatchInterval)
}

type APIHandler struct {
	handler *Handler

	// appHandler is a http.Handler to forward requests to applications.
	appHandler *app.Handler
}

// ConnectionHandler defines a function for serving incoming connections.
type ConnectionHandler func(ctx context.Context, conn net.Conn) error

func (h *APIHandler) handlePreflight(w http.ResponseWriter, r *http.Request) {
	raddr, err := utils.ParseAddr(r.Host)
	if err != nil {
		return
	}
	publicAddr := raddr.Host()

	servers, err := app.Match(r.Context(), h.handler.cfg.AccessPoint, app.MatchPublicAddr(publicAddr))
	if err != nil {
		h.handler.logger.InfoContext(r.Context(), "failed to match application with public addr", "public_addr", publicAddr)
		return
	}

	if len(servers) == 0 {
		h.handler.logger.InfoContext(r.Context(), "failed to match application with public addr", "public_addr", publicAddr)
		return
	}

	foundApp := servers[0].GetApp()
	corsPolicy := foundApp.GetCORS()
	if corsPolicy == nil {
		return
	}

	origin := r.Header.Get("Origin")
	// The Access-Control-Allow-Origin can only include one origin or a wildcard. However,
	// any request which includes credentials _must_ return an origin and not a wildcard.
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/CORS#sect2
	if slices.Contains(corsPolicy.AllowedOrigins, "*") || slices.Contains(corsPolicy.AllowedOrigins, origin) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
	} else {
		return
	}

	if len(corsPolicy.AllowedMethods) > 0 {
		w.Header().Set("Access-Control-Allow-Methods", strings.Join(corsPolicy.AllowedMethods, ","))
	}

	// This is a list of headers that are allowed in the spec. Wildcards are allowed.
	// Note: "Authorization" headers must be explicitly listed and cannot be wildcarded
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Access-Control-Allow-Headers#sect2
	if len(corsPolicy.AllowedHeaders) > 0 {
		w.Header().Set("Access-Control-Allow-Headers", strings.Join(corsPolicy.AllowedHeaders, ","))
	}

	if len(corsPolicy.ExposedHeaders) > 0 {
		w.Header().Set("Access-Control-Expose-Headers", strings.Join(corsPolicy.ExposedHeaders, ","))
	}

	// The only valid value for this header is "true", so we will only set it if configured to true
	if corsPolicy.AllowCredentials {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}

	// This will allow preflight responses to be cached for the specified duration
	if corsPolicy.MaxAge > 0 {
		w.Header().Set("Access-Control-Max-Age", fmt.Sprintf("%d", corsPolicy.MaxAge))
	}

	w.WriteHeader(http.StatusOK)
}

// Check if this request should be forwarded to an application handler to
// be handled by the UI and handle the request appropriately.
func (h *APIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// If the request is either to the fragment authentication endpoint or if the
	// request has a session cookie or a client cert, forward to
	// application handlers. If the request is requesting a
	// FQDN that is not of the proxy, redirect to application launcher.
	if h.appHandler != nil && (app.HasFragment(r) || app.HasSessionCookie(r) || app.HasClientCert(r)) {
		h.appHandler.ServeHTTP(w, r)
		return
	}

	// if the request is for an app, passthrough OPTIONS requests to the app handler
	redir, ok := app.HasName(r, h.handler.cfg.ProxyPublicAddrs)
	if ok && r.Method == http.MethodOptions {
		h.handlePreflight(w, r)
		return
	}
	// Only try to redirect if the handler is serving the full Web API.
	if !h.handler.cfg.MinimalReverseTunnelRoutesOnly && ok {
		http.Redirect(w, r, redir, http.StatusFound)
		return
	}

	// Serve the Web UI.
	h.handler.ServeHTTP(w, r)
}

// HandleConnection handles connections from plain TCP applications.
func (h *APIHandler) HandleConnection(ctx context.Context, conn net.Conn) error {
	return h.appHandler.HandleConnection(ctx, conn)
}

func (h *APIHandler) Close() error {
	return h.handler.Close()
}

// NewHandler returns a new instance of web proxy handler
func NewHandler(cfg Config, opts ...HandlerOption) (*APIHandler, error) {
	cfg.SetDefaults()

	h := &Handler{
		cfg:                  cfg,
		logger:               slog.Default().With(teleport.ComponentKey, teleport.ComponentWeb),
		clock:                clockwork.NewRealClock(),
		clusterFeatures:      cfg.ClusterFeatures,
		healthCheckAppServer: cfg.HealthCheckAppServer,
		tracer:               cfg.TracerProvider.Tracer(teleport.ComponentWeb),
	}

	if automaticUpgrades(cfg.ClusterFeatures) && h.cfg.AutomaticUpgradesChannels == nil {
		h.cfg.AutomaticUpgradesChannels = automaticupgrades.Channels{}
	}

	// for properly handling url-encoded parameter values.
	h.UseRawPath = true

	for _, o := range opts {
		if err := o(h); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// We create the cache after applying the options to make sure we use the fake clock if it was passed.
	findCache, err := utils.NewFnCache(utils.FnCacheConfig{
		TTL:         findEndpointCacheTTL,
		Clock:       h.clock,
		Context:     cfg.Context,
		ReloadOnErr: false,
	})
	if err != nil {
		return nil, trace.Wrap(err, "creating /find cache")
	}
	h.findEndpointCache = findCache

	// We create the cache after applying the options to make sure we use the fake clock if it was passed.
	cmcCache, err := utils.NewFnCache(utils.FnCacheConfig{
		TTL:         findEndpointCacheTTL,
		Clock:       h.clock,
		Context:     cfg.Context,
		ReloadOnErr: false,
	})
	if err != nil {
		return nil, trace.Wrap(err, "creating /find cache")
	}
	h.clusterMaintenanceConfigCache = cmcCache

	sessionLingeringThreshold := cachedSessionLingeringThreshold
	if cfg.CachedSessionLingeringThreshold != nil {
		sessionLingeringThreshold = *cfg.CachedSessionLingeringThreshold
	}

	sessionCache, err := newSessionCache(h.cfg.Context, sessionCacheOptions{
		proxyClient:               cfg.ProxyClient,
		accessPoint:               cfg.AccessPoint,
		servers:                   []utils.NetAddr{cfg.AuthServers},
		cipherSuites:              cfg.CipherSuites,
		clock:                     h.clock,
		sessionLingeringThreshold: sessionLingeringThreshold,
		proxySigner:               cfg.PROXYSigner,
		logger:                    h.logger,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	h.auth = sessionCache
	sshPortValue := strconv.Itoa(defaults.SSHProxyListenPort)
	if cfg.ProxySSHAddr.String() != "" {
		_, sshPort, err := net.SplitHostPort(cfg.ProxySSHAddr.String())
		if err != nil {
			h.logger.WarnContext(h.cfg.Context, "Invalid SSH proxy address, will use default port",
				"error", err,
				"ssh_proxy_addr", logutils.StringerAttr(&cfg.ProxySSHAddr),
				"default_port", defaults.SSHProxyListenPort,
			)
		} else {
			sshPortValue = sshPort
		}
	}

	h.sshPort = sshPortValue

	// rateLimiter is used to limit unauthenticated challenge generation for
	// passwordless and for unauthenticated metrics.
	h.limiter, err = limiter.NewRateLimiter(limiter.Config{
		Rates: []limiter.Rate{
			{
				Period:  defaults.LimiterPeriod,
				Average: defaults.LimiterAverage,
				Burst:   defaults.LimiterBurst,
			},
		},
		MaxConnections: defaults.LimiterMaxConnections,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// highLimiter is used for endpoints which are only CPU constrained and require high request rates
	h.highLimiter, err = limiter.NewRateLimiter(limiter.Config{
		Rates: []limiter.Rate{
			{
				Period:  defaults.LimiterHighPeriod,
				Average: defaults.LimiterHighAverage,
				Burst:   defaults.LimiterHighBurst,
			},
		},
		MaxConnections: defaults.LimiterMaxConnections,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.MinimalReverseTunnelRoutesOnly {
		h.bindMinimalEndpoints()
	} else {
		h.bindDefaultEndpoints()
	}

	// serve the web UI from the embedded filesystem
	var indexPage *template.Template
	// we will set our etag based on the teleport version and
	// the webasset app hash if available. The version only will not
	// suffice as it can cause incorrect caching for local development.

	// The hash of the webasset app.js is used to ensure that builds at
	// different times or different OSes will be the same and not cause
	// cache invalidation for production users. For example, using a timestamp
	// at build time would cause different OS builds to be different, and timestamps
	// at process start would mean multiple proxies would serving different etags)
	etag := fmt.Sprintf("W/%q", teleport.Version)
	if cfg.StaticFS != nil {
		index, err := cfg.StaticFS.Open("/index.html")
		if err != nil {
			h.logger.ErrorContext(h.cfg.Context, "Failed to open index file", "error", err)
			return nil, trace.Wrap(err)
		}
		defer index.Close()
		indexContent, err := io.ReadAll(index)
		if err != nil {
			return nil, trace.ConvertSystemError(err)
		}
		indexPage, err = template.New("index").Parse(string(indexContent))
		if err != nil {
			return nil, trace.BadParameter("failed parsing index.html template: %v", err)
		}

		h.Handle("GET", "/robots.txt", httplib.MakeHandler(serveRobotsTxt))

		etagFromAppHash, err := readEtagFromAppHash(cfg.StaticFS)
		if err != nil {
			h.logger.ErrorContext(h.cfg.Context, "Could not read apphash from embedded webassets. Using version only as ETag for Web UI assets", "error", err)
		} else {
			etag = etagFromAppHash
		}
	}

	// This endpoint is used both by Web UI and Connect.
	h.Handle("GET", "/web/config.js", h.WithUnauthenticatedLimiter(h.getWebConfig))

	if cfg.NodeWatcher != nil {
		h.nodeWatcher = cfg.NodeWatcher
	}

	const v1Prefix = "/v1"
	notFoundRoutingHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Request is going to the API?
		// If no routes were matched, it could be because it's a path with `v1` prefix
		// (eg: the Teleport web app will call "most" endpoints with v1 prefixed).
		//
		// `v1` paths are not defined with `v1` prefix. If the path turns out to be prefixed
		// with `v1`, it will be stripped and served again. Historically, that's how it started
		// and should be kept that way to prevent breakage.
		//
		// v2+ prefixes will be expected by both caller and definition and will not be stripped.
		if strings.HasPrefix(r.URL.Path, v1Prefix) {
			pathParts := strings.Split(r.URL.Path, "/")
			if len(pathParts) > 2 {
				// check against known second part of path to ensure we
				// aren't allowing paths like /v1/v2/webapi
				// part[0] is empty space from leading slash "/"
				// part[1] is the prefix "v1"
				switch pathParts[2] {
				case "webapi", "enterprise", "scripts", ".well-known", "workload-identity", "web":
					http.StripPrefix(v1Prefix, h).ServeHTTP(w, r)
					return
				}
			}
			httplib.RouteNotFoundResponse(r.Context(), w, teleport.Version)
			return
		}

		// request is going to the web UI
		if cfg.StaticFS == nil {
			w.WriteHeader(http.StatusNotImplemented)
			return
		}

		// redirect to "/web" when someone hits "/"
		if r.URL.Path == "/" {
			app.SetRedirectPageHeaders(w.Header(), "")
			http.Redirect(w, r, "/web", http.StatusFound)
			return
		}

		// serve Web UI:
		if strings.HasPrefix(r.URL.Path, "/web/app") {

			// Check if the incoming request wants to check the version
			// and if the version has not changed, return a Not Modified response
			if match := r.Header.Get("If-None-Match"); match == etag {
				w.WriteHeader(http.StatusNotModified)
				return
			}

			fs := http.FileServer(cfg.StaticFS)

			fs = makeGzipHandler(fs)
			fs = makeCacheHandler(fs, etag)

			http.StripPrefix("/web", fs).ServeHTTP(w, r)
		} else if strings.HasPrefix(r.URL.Path, "/web/") || r.URL.Path == "/web" {
			csrfToken, err := csrf.AddCSRFProtection(w, r)
			if err != nil {
				h.logger.WarnContext(r.Context(), "Failed to generate CSRF token", "error", err)
			}

			// Ignore errors here, as unauthenticated requests for index.html are common - the user might
			// not have logged in yet, or their session may have expired.
			// The web app will show them the login page in this case.
			session, _ := h.authenticateWebSession(w, r)
			session.XCSRF = csrfToken

			httplib.SetNoCacheHeaders(w.Header())
			httplib.SetIndexContentSecurityPolicy(w.Header(), cfg.ClusterFeatures, r.URL.Path)

			if err := indexPage.Execute(w, session); err != nil {
				h.logger.ErrorContext(r.Context(), "Failed to execute index page template", "error", err)
			}
		} else {
			httplib.RouteNotFoundResponse(r.Context(), w, teleport.Version)
			return
		}
	})

	h.NotFound = notFoundRoutingHandler

	if cfg.PluginRegistry != nil {
		if err := cfg.PluginRegistry.RegisterProxyWebHandlers(h); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	resp, err := h.cfg.ProxySettings.GetProxySettings(cfg.Context)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create application specific handler. This handler handles sessions and
	// forwarding for application access.
	var appHandler *app.Handler
	if !cfg.MinimalReverseTunnelRoutesOnly {
		appHandler, err = app.NewHandler(cfg.Context, &app.HandlerConfig{
			Clock:                 h.clock,
			AuthClient:            cfg.ProxyClient,
			AccessPoint:           cfg.AccessPoint,
			ProxyClient:           cfg.Proxy,
			CipherSuites:          cfg.CipherSuites,
			ProxyPublicAddrs:      cfg.ProxyPublicAddrs,
			WebPublicAddr:         resp.SSH.PublicAddr,
			IntegrationAppHandler: cfg.IntegrationAppHandler,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if h.healthCheckAppServer == nil {
			h.healthCheckAppServer = appHandler.HealthCheckAppServer
		}
	}

	go h.startFeatureWatcher()

	return &APIHandler{
		handler:    h,
		appHandler: appHandler,
	}, nil
}

type webSession struct {
	Session string
	XCSRF   string
}

func (h *Handler) authenticateWebSession(w http.ResponseWriter, r *http.Request) (webSession, error) {
	ctx, err := h.AuthenticateRequest(w, r, false /* validate bearer token */)
	if err != nil {
		return webSession{}, trace.Wrap(err)
	}
	resp, err := newSessionResponse(ctx)
	if err != nil {
		return webSession{}, trace.Wrap(err)
	}
	out, err := json.Marshal(resp)
	if err != nil {
		return webSession{}, trace.Wrap(err)
	}
	return webSession{
		Session: base64.StdEncoding.EncodeToString(out),
	}, nil
}

// bindMinimalEndpoints binds only the endpoints required for a reverse tunnel
// agent to establish a connection.
func (h *Handler) bindMinimalEndpoints() {
	// find is like ping, but is faster because it is optimized for servers
	// and does not fetch the data that servers don't need, e.g.
	// OIDC connectors and auth preferences
	// Note that find is a unique endpoint that requires high request rates
	// sometimes through NATs and thus should not be rate limited by IP.
	h.GET("/webapi/find", httplib.MakeHandler(h.find))
	// Issue host credentials.
	h.POST("/webapi/host/credentials", h.WithUnauthenticatedHighLimiter(h.hostCredentials))
}

// bindDefaultEndpoints binds the default endpoints for the web API.
func (h *Handler) bindDefaultEndpoints() {
	h.bindMinimalEndpoints()

	// ping endpoint is used to check if the server is up. the /webapi/ping
	// endpoint returns the default authentication method and configuration that
	// the server supports. the /webapi/ping/:connector endpoint can be used to
	// query the authentication configuration for a specific connector.
	h.GET("/webapi/ping", httplib.MakeHandler(h.ping))
	h.GET("/webapi/ping/:connector", h.WithUnauthenticatedHighLimiter(h.pingWithConnector))

	// Unauthenticated access to JWT public keys.
	h.GET("/.well-known/jwks.json", h.WithUnauthenticatedHighLimiter(h.wellKnownJWKS))

	// Unauthenticated access to the message of the day
	h.GET("/webapi/motd", h.WithHighLimiter(h.motd))

	// Unauthenticated access to retrieving the script used to install Teleport
	h.GET("/webapi/scripts/installer/:name", h.WithLimiter(h.installer))

	// Forwards traces to the configured upstream collector
	h.POST("/webapi/traces", h.WithAuth(h.traces))

	// App sessions
	h.POST("/webapi/sessions/app", h.WithAuth(h.createAppSession))

	// Web sessions
	h.POST("/webapi/sessions/web", h.WithLimiter(h.createWebSession))
	h.DELETE("/webapi/sessions/web", h.WithAuth(h.deleteWebSession))
	h.POST("/webapi/sessions/web/renew", h.WithAuth(h.renewWebSession))

	// Secure (device-bound) sessions
	h.POST("/webapi/securesession/startsession", h.startSecureSession)

	h.POST("/webapi/users", h.WithAuth(h.createUserHandle))
	h.PUT("/webapi/users", h.WithAuth(h.updateUserHandle))
	h.GET("/webapi/users", h.WithAuth(h.getUsersHandle))
	h.DELETE("/webapi/users/:username", h.WithAuth(h.deleteUserHandle))

	// We have an overlap route here, please see godoc of handleGetUserOrResetToken
	// h.GET("/webapi/users/:username", h.WithAuth(h.getUserHandle))
	// h.GET("/webapi/users/password/token/:token", h.WithLimiter(h.getResetPasswordTokenHandle))
	h.GET("/webapi/users/*wildcard", h.handleGetUserOrResetToken)

	h.PUT("/webapi/users/password/token", h.WithLimiter(h.changeUserAuthentication))
	h.PUT("/webapi/users/password", h.WithAuth(h.changePassword))
	h.POST("/webapi/users/password/token", h.WithAuth(h.createResetPasswordToken))
	h.POST("/webapi/users/privilege/token", h.WithAuth(h.createPrivilegeTokenHandle))

	// Issues SSH temp certificates based on 2FA access creds
	// TODO(Joerger): DELETE IN v18.0.0, deprecated in favor of mfa login endpoints.
	h.POST("/webapi/ssh/certs", h.WithUnauthenticatedLimiter(h.createSSHCert))

	h.POST("/webapi/headless/login", h.WithUnauthenticatedLimiter(h.headlessLogin))

	// list available sites
	h.GET("/webapi/sites", h.WithAuth(h.getClusters))

	// Site specific API

	// get site info
	h.GET("/webapi/sites/:site/info", h.WithClusterAuth(h.getClusterInfo))

	// get namespaces
	h.GET("/webapi/sites/:site/namespaces", h.WithClusterAuth(h.getSiteNamespaces))

	// get unified resources
	h.GET("/webapi/sites/:site/resources", h.WithClusterAuth(h.clusterUnifiedResourcesGet))

	// get nodes
	h.GET("/webapi/sites/:site/nodes", h.WithClusterAuth(h.clusterNodesGet))
	h.POST("/webapi/sites/:site/nodes", h.WithClusterAuth(h.handleNodeCreate))

	// Get applications.
	h.GET("/webapi/sites/:site/apps", h.WithClusterAuth(h.clusterAppsGet))

	// get login alerts
	h.GET("/webapi/sites/:site/alerts", h.WithClusterAuth(h.clusterLoginAlertsGet))

	// lock interactions
	h.GET("/webapi/sites/:site/locks", h.WithClusterAuth(h.getClusterLocks))
	h.PUT("/webapi/sites/:site/locks", h.WithClusterAuth(h.createClusterLock))
	h.DELETE("/webapi/sites/:site/locks/:uuid", h.WithClusterAuth(h.deleteClusterLock))

	// active sessions handlers
	h.GET("/webapi/sites/:site/connect/ws", h.WithClusterAuthWebSocket(h.siteNodeConnect))         // connect to an active session (via websocket, with auth over websocket)
	h.GET("/webapi/sites/:site/sessions", h.WithClusterAuth(h.clusterActiveAndPendingSessionsGet)) // get list of active and pending sessions

	h.GET("/webapi/sites/:site/kube/exec/ws", h.WithClusterAuthWebSocket(h.podConnect)) // connect to a pod with exec (via websocket, with auth over websocket)
	h.GET("/webapi/sites/:site/db/exec/ws", h.WithClusterAuthWebSocket(h.dbConnect))

	// Audit events handlers.
	h.GET("/webapi/sites/:site/events/search", h.WithClusterAuth(h.clusterSearchEvents))                 // search site events
	h.GET("/webapi/sites/:site/events/search/sessions", h.WithClusterAuth(h.clusterSearchSessionEvents)) // search site session events
	h.GET("/webapi/sites/:site/ttyplayback/:sid", h.WithClusterAuth(h.ttyPlaybackHandle))
	h.GET("/webapi/sites/:site/sessionlength/:sid", h.WithClusterAuth(h.sessionLengthHandle))

	// scp file transfer
	h.GET("/webapi/sites/:site/nodes/:server/:login/scp", h.WithClusterAuth(h.transferFile))
	h.POST("/webapi/sites/:site/nodes/:server/:login/scp", h.WithClusterAuth(h.transferFile))

	// Sign required files to set up mTLS using the db format.
	h.POST("/webapi/sites/:site/sign/db", h.WithProvisionTokenAuth(h.signDatabaseCertificate))

	// Returns the CA Certs
	// Deprecated, use the `webapi/auth/export` endpoint.
	// Returning other clusters (trusted cluster) CA certs would leak wether the TrustedCluster exists or not.
	// Given that this is a public/unauthorized endpoint, we should refrain from exposing that kind of information.
	h.GET("/webapi/sites/:site/auth/export", h.authExportPublic)
	h.GET("/webapi/auth/export", h.authExportPublic)

	// join token handlers
	h.PUT("/webapi/tokens/yaml", h.WithAuth(h.updateTokenYAML))
	// used for creating a new token
	h.POST("/webapi/tokens", h.WithAuth(h.upsertTokenHandle))
	// used for updating a token
	h.PUT("/webapi/tokens", h.WithAuth(h.upsertTokenHandle))
	// TODO(kimlisa): DELETE IN 19.0 - Replaced by /v2/webapi/token endpoint
	// MUST delete with related code found in web/packages/teleport/src/services/joinToken/joinToken.ts(fetchJoinToken)
	h.POST("/webapi/token", h.WithAuth(h.createTokenForDiscoveryHandle))
	// used for creating tokens used during guided discover flows
	// v2 endpoint processes "suggestedLabels" field
	h.POST("/v2/webapi/token", h.WithAuth(h.createTokenForDiscoveryHandle))
	h.GET("/webapi/tokens", h.WithAuth(h.getTokens))
	h.DELETE("/webapi/tokens", h.WithAuth(h.deleteToken))

	// install script, the ':token' wildcard is a hack to make the router happy and support
	// the token-less route "/scripts/install.sh".
	// h.installScriptHandle Will reject any unknown sub-route.
	h.GET("/scripts/:token", h.WithHighLimiter(h.installScriptHandle))

	// join scripts
	h.GET("/scripts/:token/install-node.sh", h.WithLimiter(h.getNodeJoinScriptHandle))
	h.GET("/scripts/:token/install-app.sh", h.WithLimiter(h.getAppJoinScriptHandle))
	h.GET("/scripts/:token/install-database.sh", h.WithLimiter(h.getDatabaseJoinScriptHandle))

	// Discovery installation script requires a query param to define the DiscoveryGroup:
	// ?discoveryGroup=<group name>
	h.GET("/scripts/:token/install-discovery.sh", h.WithLimiter(h.getDiscoveryJoinScriptHandle))

	// web context
	h.GET("/webapi/sites/:site/context", h.WithClusterAuth(h.getUserContext))
	// Deprecated: Use `/webapi/sites/:site/resources` instead.
	// TODO(kiosion): DELETE in 18.0
	h.GET("/webapi/sites/:site/resources/check", h.WithClusterAuth(h.checkAccessToRegisteredResource))

	// Database access handlers.
	h.GET("/webapi/sites/:site/databases", h.WithClusterAuth(h.clusterDatabasesGet))
	h.POST("/webapi/sites/:site/databases", h.WithClusterAuth(h.handleDatabaseCreateOrOverwrite))
	h.PUT("/webapi/sites/:site/databases/:database", h.WithClusterAuth(h.handleDatabasePartialUpdate))
	h.GET("/webapi/sites/:site/databases/:database", h.WithClusterAuth(h.clusterDatabaseGet))
	h.GET("/webapi/sites/:site/databases/:database/iam/policy", h.WithClusterAuth(h.handleDatabaseGetIAMPolicy))
	h.GET("/webapi/scripts/databases/configure/sqlserver/:token/configure-ad.ps1", httplib.MakeHandler(h.sqlServerConfigureADScriptHandle))

	// DatabaseService handlers
	h.GET("/webapi/sites/:site/databaseservices", h.WithClusterAuth(h.clusterDatabaseServicesList))

	// Kube access handlers.
	h.GET("/webapi/sites/:site/kubernetes", h.WithClusterAuth(h.clusterKubesGet))
	h.GET("/webapi/sites/:site/kubernetes/resources", h.WithClusterAuth(h.clusterKubeResourcesGet))

	// Github connector handlers
	h.GET("/webapi/github/login/web", h.WithRedirect(h.githubLoginWeb))
	h.GET("/webapi/github/callback", h.WithMetaRedirect(h.githubCallback))
	h.POST("/webapi/github/login/console", h.WithLimiter(h.githubLoginConsole))

	// MFA public endpoints.
	h.POST("/webapi/sites/:site/mfa/required", h.WithClusterAuth(h.isMFARequired))
	h.POST("/webapi/mfa/login/begin", h.WithLimiter(h.mfaLoginBegin))
	h.POST("/webapi/mfa/login/finish", h.WithLimiter(h.mfaLoginFinish))
	h.POST("/webapi/mfa/login/finishsession", h.WithLimiter(h.mfaLoginFinishSession))
	h.DELETE("/webapi/mfa/token/:token/devices/:devicename", h.WithLimiter(h.deleteMFADeviceWithTokenHandle))
	h.GET("/webapi/mfa/token/:token/devices", h.WithLimiter(h.getMFADevicesWithTokenHandle))
	h.POST("/webapi/mfa/token/:token/authenticatechallenge", h.WithLimiter(h.createAuthenticateChallengeWithTokenHandle))
	h.POST("/webapi/mfa/token/:token/registerchallenge", h.WithLimiter(h.createRegisterChallengeWithTokenHandle))

	// MFA private endpoints.
	h.GET("/webapi/mfa/devices", h.WithAuth(h.getMFADevicesHandle))
	h.POST("/webapi/mfa/authenticatechallenge", h.WithAuth(h.createAuthenticateChallengeHandle))
	// TODO(Joerger) v19.0.0: currently unused, WebUI can use these in v19 without backwards compatibility concerns.
	h.DELETE("/webapi/mfa/devices", h.WithAuth(h.deleteMFADeviceHandle))
	h.POST("/webapi/mfa/registerchallenge", h.WithAuth(h.createRegisterChallengeHandle))

	h.POST("/webapi/mfa/devices", h.WithAuth(h.addMFADeviceHandle))
	// DEPRECATED in favor of mfa/authenticatechallenge.
	// TODO(bl-nero): DELETE IN 17.0.0
	h.POST("/webapi/mfa/authenticatechallenge/password", h.WithAuth(h.createAuthenticateChallengeWithPassword))

	// Device Trust.
	// Do not enforce bearer token for /webconfirm, it is called from outside the
	// Web UI.
	h.GET("/webapi/devices/webconfirm", h.WithSession(h.deviceWebConfirm))

	// trusted clusters
	h.POST("/webapi/trustedclusters/validate", h.WithUnauthenticatedLimiter(h.validateTrustedCluster))

	// User Status (used by client to check if user session is valid)
	h.GET("/webapi/user/status", h.WithAuth(h.getUserStatus))

	h.GET("/webapi/roles", h.WithAuth(h.listRolesHandle))
	h.POST("/webapi/roles", h.WithAuth(h.createRoleHandle))
	h.PUT("/webapi/roles/:name", h.WithAuth(h.updateRoleHandle))
	h.DELETE("/webapi/roles/:name", h.WithAuth(h.deleteRole))
	h.GET("/webapi/presetroles", h.WithUnauthenticatedHighLimiter(h.getPresetRoles))

	h.GET("/webapi/github", h.WithAuth(h.getGithubConnectorsHandle))
	h.POST("/webapi/github", h.WithAuth(h.createGithubConnectorHandle))
	// The extra "connector" in the path is to avoid a wildcard conflict with the github handlers used
	// during the login flow ("github/login/web" and "github/callback").
	h.GET("/webapi/github/connector/:name", h.WithAuth(h.getGithubConnectorHandle))
	h.PUT("/webapi/github/:name", h.WithAuth(h.updateGithubConnectorHandle))
	h.DELETE("/webapi/github/:name", h.WithAuth(h.deleteGithubConnector))

	// Sets the default connector in the auth preference.
	h.PUT("/webapi/authconnector/default", h.WithAuth(h.setDefaultConnectorHandle))

	h.GET("/webapi/trustedcluster", h.WithAuth(h.getTrustedClustersHandle))
	h.POST("/webapi/trustedcluster", h.WithAuth(h.upsertTrustedClusterHandle))
	h.PUT("/webapi/trustedcluster/:name", h.WithAuth(h.upsertTrustedClusterHandle))
	h.DELETE("/webapi/trustedcluster/:name", h.WithAuth(h.deleteTrustedCluster))

	h.GET("/webapi/apps/:fqdnHint", h.WithAuth(h.getAppDetails))
	h.GET("/webapi/apps/:fqdnHint/:clusterName/:publicAddr", h.WithAuth(h.getAppDetails))

	h.POST("/webapi/yaml/parse/:kind", h.WithAuth(h.yamlParse))
	h.POST("/webapi/yaml/stringify/:kind", h.WithAuth(h.yamlStringify))

	// Desktop access endpoints.
	h.GET("/webapi/sites/:site/desktops", h.WithClusterAuth(h.clusterDesktopsGet))
	h.GET("/webapi/sites/:site/desktopservices", h.WithClusterAuth(h.clusterDesktopServicesGet))
	h.GET("/webapi/sites/:site/desktops/:desktopName", h.WithClusterAuth(h.getDesktopHandle))
	// GET /webapi/sites/:site/desktops/:desktopName/connect?username=<username>&width=<width>&height=<height>
	h.GET("/webapi/sites/:site/desktops/:desktopName/connect/ws", h.WithClusterAuthWebSocket(h.desktopConnectHandle))
	// GET /webapi/sites/:site/desktopplayback/:sid/ws
	h.GET("/webapi/sites/:site/desktopplayback/:sid/ws", h.WithClusterAuthWebSocket(h.desktopPlaybackHandle))
	h.GET("/webapi/sites/:site/desktops/:desktopName/active", h.WithClusterAuth(h.desktopIsActive))

	// GET a Connection Diagnostics by its name
	h.GET("/webapi/sites/:site/diagnostics/connections/:connectionid", h.WithClusterAuth(h.getConnectionDiagnostic))
	// Diagnose a Connection
	h.POST("/webapi/sites/:site/diagnostics/connections", h.WithClusterAuth(h.diagnoseConnection))

	// Integrations CRUD
	h.GET("/webapi/sites/:site/integrations", h.WithClusterAuth(h.integrationsList))
	h.POST("/webapi/sites/:site/integrations", h.WithClusterAuth(h.integrationsCreate))
	h.GET("/webapi/sites/:site/integrations/:name", h.WithClusterAuth(h.integrationsGet))
	h.PUT("/webapi/sites/:site/integrations/:name", h.WithClusterAuth(h.integrationsUpdate))
	h.GET("/webapi/sites/:site/integrations/:name/stats", h.WithClusterAuth(h.integrationStats))
	h.GET("/webapi/sites/:site/integrations/:name/discoveryrules", h.WithClusterAuth(h.integrationDiscoveryRules))
	h.GET("/webapi/sites/:site/integrations/:name/ca", h.WithClusterAuth(h.integrationsExportCA))
	// TODO(kimlisa): DELETE IN 19.0 - Replaced by /v2 equivalent endpoint
	h.DELETE("/webapi/sites/:site/integrations/:name_or_subkind", h.WithClusterAuth(h.integrationsDelete))
	h.DELETE("/v2/webapi/sites/:site/integrations/:name_or_subkind", h.WithClusterAuth(h.integrationsDelete))

	// GET the Microsoft Teams plugin app.zip file.
	h.GET("/webapi/sites/:site/plugins/:plugin/files/msteams_app.zip", h.WithClusterAuth(h.integrationsMsTeamsAppZipGet))

	// AWS OIDC Integration Actions
	h.GET("/webapi/scripts/integrations/configure/awsoidc-idp.sh", h.WithLimiter(h.awsOIDCConfigureIdP))
	h.POST("/webapi/sites/:site/integrations/aws-oidc/:name/ping", h.WithClusterAuth(h.awsOIDCPing))
	h.POST("/webapi/sites/:site/integrations/aws-oidc/:name/databases", h.WithClusterAuth(h.awsOIDCListDatabases))
	h.GET("/webapi/scripts/integrations/configure/listdatabases-iam.sh", h.WithLimiter(h.awsOIDCConfigureListDatabasesIAM))
	h.POST("/webapi/sites/:site/integrations/aws-oidc/:name/deployservice", h.WithClusterAuth(h.awsOIDCDeployService))
	h.POST("/webapi/sites/:site/integrations/aws-oidc/:name/deploydatabaseservices", h.WithClusterAuth(h.awsOIDCDeployDatabaseServices))
	h.POST("/webapi/sites/:site/integrations/aws-oidc/:name/listdeployeddatabaseservices", h.WithClusterAuth(h.awsOIDCListDeployedDatabaseService))
	h.GET("/webapi/scripts/integrations/configure/deployservice-iam.sh", h.WithLimiter(h.awsOIDCConfigureDeployServiceIAM))
	h.POST("/webapi/sites/:site/integrations/aws-oidc/:name/eksclusters", h.WithClusterAuth(h.awsOIDCListEKSClusters))
	// TODO(kimlisa): DELETE IN 19.0 - replaced by /v2/webapi/sites/:site/integrations/aws-oidc/:name/enrolleksclusters
	// MUST delete with related code found in web/packages/teleport/src/services/integrations/integrations.ts(enrollEksClusters)
	h.POST("/webapi/sites/:site/integrations/aws-oidc/:name/enrolleksclusters", h.WithClusterAuth(h.awsOIDCEnrollEKSClusters))
	// v2 endpoint introduces "extraLabels" field.
	h.POST("/v2/webapi/sites/:site/integrations/aws-oidc/:name/enrolleksclusters", h.WithClusterAuth(h.awsOIDCEnrollEKSClusters))
	h.POST("/webapi/sites/:site/integrations/aws-oidc/:name/securitygroups", h.WithClusterAuth(h.awsOIDCListSecurityGroups))
	h.POST("/webapi/sites/:site/integrations/aws-oidc/:name/databasevpcs", h.WithClusterAuth(h.awsOIDCListDatabaseVPCs))
	h.POST("/webapi/sites/:site/integrations/aws-oidc/:name/subnets", h.WithClusterAuth(h.awsOIDCListSubnets))
	h.POST("/webapi/sites/:site/integrations/aws-oidc/:name/requireddatabasesvpcs", h.WithClusterAuth(h.awsOIDCRequiredDatabasesVPCS))
	h.GET("/webapi/scripts/integrations/configure/eks-iam.sh", h.WithLimiter(h.awsOIDCConfigureEKSIAM))
	h.GET("/webapi/scripts/integrations/configure/access-graph-cloud-sync-iam.sh", h.WithLimiter(h.accessGraphCloudSyncOIDC))
	h.GET("/webapi/scripts/integrations/configure/aws-app-access-iam.sh", h.WithLimiter(h.awsOIDCConfigureAWSAppAccessIAM))
	// TODO(kimlisa): DELETE IN 19.0 - Replaced by /v2 equivalent endpoint
	h.POST("/webapi/sites/:site/integrations/aws-oidc/:name/aws-app-access", h.WithClusterAuth(h.awsOIDCCreateAWSAppAccess))
	// v2 endpoint introduces "labels" field
	// MUST delete with related code found in web/packages/teleport/src/services/integrations/integrations.ts(createAwsAppAccess)
	h.POST("/v2/webapi/sites/:site/integrations/aws-oidc/:name/aws-app-access", h.WithClusterAuth(h.awsOIDCCreateAWSAppAccess))
	// The Integration DELETE endpoint already sets the expected named param after `/integrations/`
	// It must be re-used here, otherwise the router will not start.
	// See https://github.com/julienschmidt/httprouter/issues/364
	h.DELETE("/webapi/sites/:site/integrations/:name_or_subkind/aws-app-access/:name", h.WithClusterAuth(h.awsOIDCDeleteAWSAppAccess))
	h.GET("/webapi/scripts/integrations/configure/ec2-ssm-iam.sh", h.WithLimiter(h.awsOIDCConfigureEC2SSMIAM))

	// SAML IDP integration endpoints
	h.GET("/webapi/scripts/integrations/configure/gcp-workforce-saml.sh", h.WithLimiter(h.gcpWorkforceConfigScript))

	// Okta integration endpoints.
	h.GET(OktaJWKSWellknownURI, h.WithLimiter(h.jwksOkta))

	// Azure OIDC integration endpoints
	h.GET("/webapi/scripts/integrations/configure/azureoidc.sh", h.WithLimiter(h.azureOIDCConfigure))

	// OIDC Integration specific endpoints:
	// Unauthenticated access to OpenID Configuration - used for AWS OIDC IdP integration
	h.GET("/.well-known/openid-configuration", h.WithLimiter(h.openidConfiguration))
	h.GET(OIDCJWKWURI, h.WithLimiter(h.jwksOIDC))
	h.GET("/webapi/thumbprint", h.WithLimiter(h.thumbprint))

	// SPIFFE Federation Trust Bundle
	h.GET("/webapi/spiffe/bundle.json", h.WithLimiter(h.getSPIFFEBundle))
	h.GET("/workload-identity/jwt-jwks.json", h.WithLimiter(h.getSPIFFEJWKS))
	h.GET("/workload-identity/.well-known/openid-configuration", h.WithLimiter(h.getSPIFFEOIDCDiscoveryDocument))

	// DiscoveryConfig CRUD
	h.GET("/webapi/sites/:site/discoveryconfig", h.WithClusterAuth(h.discoveryconfigList))
	h.POST("/webapi/sites/:site/discoveryconfig", h.WithClusterAuth(h.discoveryconfigCreate))
	h.GET("/webapi/sites/:site/discoveryconfig/:name", h.WithClusterAuth(h.discoveryconfigGet))
	h.PUT("/webapi/sites/:site/discoveryconfig/:name", h.WithClusterAuth(h.discoveryconfigUpdate))
	h.DELETE("/webapi/sites/:site/discoveryconfig/:name", h.WithClusterAuth(h.discoveryconfigDelete))

	// User Tasks CRUD
	// Listing Tasks by Integration: GET /webapi/sites/:site/usertask?integration=<integration-name>
	h.GET("/webapi/sites/:site/usertask", h.WithClusterAuth(h.userTaskListByIntegration))
	h.GET("/webapi/sites/:site/usertask/:name", h.WithClusterAuth(h.userTaskGet))
	h.PUT("/webapi/sites/:site/usertask/:name/state", h.WithClusterAuth(h.userTaskStateUpdate))

	// Connection upgrades.
	h.GET("/webapi/connectionupgrade", httplib.MakeHandler(h.connectionUpgrade))

	// create user events.
	h.POST("/webapi/precapture", h.WithUnauthenticatedLimiter(h.createPreUserEventHandle))
	// create authenticated user events.
	h.POST("/webapi/capture", h.WithAuth(h.createUserEventHandle))

	h.GET("/webapi/headless/:headless_authentication_id", h.WithAuth(h.getHeadless))
	h.PUT("/webapi/headless/:headless_authentication_id", h.WithAuth(h.putHeadlessState))

	h.GET("/webapi/sites/:site/user-groups", h.WithClusterAuth(h.getUserGroups))

	// Fetches the user's preferences
	h.GET("/webapi/user/preferences", h.WithAuth(h.getUserPreferences))

	// Updates the user's preferences
	h.PUT("/webapi/user/preferences", h.WithAuth(h.updateUserPreferences))

	// Fetches the user's cluster preferences.
	h.GET("/webapi/user/preferences/:site", h.WithClusterAuth(h.getUserClusterPreferences))

	// Updates the user's cluster preferences.
	h.PUT("/webapi/user/preferences/:site", h.WithClusterAuth(h.updateUserClusterPreferences))

	// Returns logins included in the Connect My Computer role of the user.
	// Returns an empty list of logins if the user does not have a Connect My Computer role assigned.
	h.GET("/webapi/connectmycomputer/logins", h.WithAuth(h.connectMyComputerLoginsList))

	// Implements the agent version server.
	// Channel can contain "/", hence the use of a catch-all parameter
	h.GET("/webapi/automaticupgrades/channel/*request", h.WithUnauthenticatedHighLimiter(h.automaticUpgrades109))

	// GET Machine ID bot by name
	h.GET("/webapi/sites/:site/machine-id/bot/:name", h.WithClusterAuth(h.getBot))
	// GET Machine ID bots
	h.GET("/webapi/sites/:site/machine-id/bot", h.WithClusterAuth(h.listBots))
	// Create Machine ID bots
	h.POST("/webapi/sites/:site/machine-id/bot", h.WithClusterAuth(h.createBot))
	// Create bot join tokens
	h.POST("/webapi/sites/:site/machine-id/token", h.WithClusterAuth(h.createBotJoinToken))
	// PUT Machine ID bot by name
	h.PUT("/webapi/sites/:site/machine-id/bot/:name", h.WithClusterAuth(h.updateBot))
	// Delete Machine ID bot
	h.DELETE("/webapi/sites/:site/machine-id/bot/:name", h.WithClusterAuth(h.deleteBot))

	// GET a paginated list of notifications for a user
	h.GET("/webapi/sites/:site/notifications", h.WithClusterAuth(h.notificationsGet))
	// Upsert the timestamp of the latest notification that the user has seen.
	h.PUT("/webapi/sites/:site/lastseennotification", h.WithClusterAuth(h.notificationsUpsertLastSeenTimestamp))
	// Upsert a notification state when to mark a notification as read or hide it.
	h.PUT("/webapi/sites/:site/notificationstate", h.WithClusterAuth(h.notificationsUpsertNotificationState))

	// Git servers
	h.PUT("/webapi/sites/:site/gitservers", h.WithClusterAuth(h.gitServerCreateOrUpsert))
	h.GET("/webapi/sites/:site/gitservers/:name", h.WithClusterAuth(h.gitServerGet))
	h.DELETE("/webapi/sites/:site/gitservers/:name", h.WithClusterAuth(h.gitServerDelete))
}

// GetProxyClient returns authenticated auth server client
func (h *Handler) GetProxyClient() authclient.ClientI {
	return h.cfg.ProxyClient
}

// GetProxyClientCertificate returns the proxy client certificate.
func (h *Handler) GetProxyClientCertificate() (*tls.Certificate, error) {
	if h.cfg.GetProxyClientCertificate == nil {
		return nil, trace.BadParameter("GetProxyClientCertificate is not set")
	}
	tlsCert, err := h.cfg.GetProxyClientCertificate()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return tlsCert, nil
}

// GetAccessPoint returns the caching access point.
func (h *Handler) GetAccessPoint() authclient.ProxyAccessPoint {
	return h.cfg.AccessPoint
}

// Close closes associated session cache operations
func (h *Handler) Close() error {
	return h.auth.Close()
}

type userStatusResponse struct {
	RequiresDeviceTrust types.TrustedDeviceRequirement `json:"requiresDeviceTrust,omitempty"`
	HasDeviceExtensions bool                           `json:"hasDeviceExtensions,omitempty"`
	Message             string                         `json:"message"` // Always set to "ok"
}

func (h *Handler) getUserStatus(w http.ResponseWriter, r *http.Request, _ httprouter.Params, c *SessionContext) (interface{}, error) {
	return userStatusResponse{
		RequiresDeviceTrust: c.cfg.Session.GetTrustedDeviceRequirement(),
		HasDeviceExtensions: c.cfg.Session.GetHasDeviceExtensions(),
		Message:             "ok",
	}, nil
}

// handleGetUserOrResetToken has two handlers:
// - read user
// - return reset password token
// It has two because the expected route for reading a user overlaps with an already existing one
// Using `GET /webapi/users/:username` invalidates the `GET /webapi/users/password/token/:token` route
// An alternative would be using the resource's singular name `GET /webapi/user/:username` but it invalidates the `GET /webapi/user/status` route
// So, instead we'll use `GET /webapi/users/*wildcard`, parse the path/params and call the appropriate handler
func (h *Handler) handleGetUserOrResetToken(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	// do we have multiple path fields or just one
	relativePath := p.ByName("wildcard")
	relativePath = strings.TrimPrefix(relativePath, "/") // relativePath might start with "/", removing it helps reasoning
	pathFields := strings.Split(relativePath, "/")

	params := httprouter.Params{}

	handleFunc := httplib.MakeHandler(func(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
		http.NotFound(w, r)
		return nil, nil
	})

	// having one means we have an username
	if len(pathFields) == 1 {
		params = httprouter.Params{httprouter.Param{
			Key:   "username",
			Value: pathFields[0],
		}}

		handleFunc = h.WithAuth(h.getUserHandle)
	}

	// if we have exactly 3 and they look like /password/token/:token
	if len(pathFields) == 3 && pathFields[0] == "password" && pathFields[1] == "token" && pathFields[2] != "" {
		params = httprouter.Params{httprouter.Param{
			Key:   "token",
			Value: pathFields[2],
		}}

		handleFunc = httplib.MakeHandler(h.getResetPasswordTokenHandle)
	}

	handleFunc(w, r, params)
}

// getUserContext returns user context
//
// GET /webapi/sites/:site/context
func (h *Handler) getUserContext(w http.ResponseWriter, r *http.Request, p httprouter.Params, c *SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
	cn, err := h.cfg.AccessPoint.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if cn.GetClusterName() != site.GetName() {
		return nil, trace.BadParameter("endpoint only implemented for root cluster")
	}
	accessChecker, err := c.GetUserAccessChecker()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := c.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	user, err := clt.GetUser(r.Context(), c.GetUser(), false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// The following section is similar to
	// https://github.com/gravitational/teleport/blob/ea810d30d99f26e58a190edc5facfbe0c09ea5e5/lib/srv/desktop/windows_server.go#L757-L769
	recConfig, err := c.cfg.UnsafeCachedAuthClient.GetSessionRecordingConfig(r.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	desktopRecordingEnabled := recConfig.GetMode() != types.RecordOff

	features := h.GetClusterFeatures()
	entitlement := modules.GetProtoEntitlement(&features, entitlements.AccessMonitoring)
	// ensure entitlement is set & feature is configured
	accessMonitoringEnabled := entitlement.Enabled && features.GetAccessMonitoringConfigured()

	userContext, err := ui.NewUserContext(user, accessChecker.Roles(), features, desktopRecordingEnabled, accessMonitoringEnabled)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	res, err := clt.GetAccessCapabilities(r.Context(), types.AccessCapabilitiesRequest{
		RequestableRoles:   true,
		SuggestedReviewers: true,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	userContext.AccessCapabilities = ui.AccessCapabilities{
		RequestableRoles:   res.RequestableRoles,
		SuggestedReviewers: res.SuggestedReviewers,
		RequireReason:      res.RequireReason,
	}

	userContext.AllowedSearchAsRoles = accessChecker.GetAllowedSearchAsRoles()

	userContext.Cluster, err = ui.GetClusterDetails(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pingResp, err := clt.Ping(r.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if pingResp.LicenseExpiry != nil && !pingResp.LicenseExpiry.IsZero() {
		userContext.Cluster.LicenseExpiry = pingResp.LicenseExpiry
	}

	userContext.ConsumedAccessRequestID = c.cfg.Session.GetConsumedAccessRequestID()

	return userContext, nil
}

// PublicProxyAddr returns the publicly advertised proxy address
func (h *Handler) PublicProxyAddr() string {
	return h.cfg.PublicProxyAddr
}

// AccessGraphAddr returns the TAG API address
func (h *Handler) AccessGraphAddr() utils.NetAddr {
	return h.cfg.AccessGraphAddr
}

func localSettings(ctx context.Context, cap types.AuthPreference, logger *slog.Logger) (webclient.AuthenticationSettings, error) {
	as := webclient.AuthenticationSettings{
		Type:                    constants.Local,
		SecondFactor:            types.LegacySecondFactorFromSecondFactors(cap.GetSecondFactors()),
		PreferredLocalMFA:       cap.GetPreferredLocalMFA(),
		AllowPasswordless:       cap.GetAllowPasswordless(),
		AllowHeadless:           cap.GetAllowHeadless(),
		Local:                   &webclient.LocalSettings{},
		PrivateKeyPolicy:        cap.GetPrivateKeyPolicy(),
		PIVSlot:                 cap.GetPIVSlot(),
		DeviceTrust:             deviceTrustSettings(cap),
		SignatureAlgorithmSuite: cap.GetSignatureAlgorithmSuite(),
	}

	// Only copy the connector name if it's truly local and not a local fallback.
	if cap.GetType() == constants.Local {
		as.Local.Name = cap.GetConnectorName()
	}

	// U2F settings.
	switch u2f, err := cap.GetU2F(); {
	case err == nil:
		as.U2F = &webclient.U2FSettings{AppID: u2f.AppID}
	case !trace.IsNotFound(err):
		logger.WarnContext(ctx, "Error reading U2F settings", "error", err)
	}

	// Webauthn settings.
	switch webConfig, err := cap.GetWebauthn(); {
	case err == nil:
		as.Webauthn = &webclient.Webauthn{
			RPID: webConfig.RPID,
		}
	case !trace.IsNotFound(err):
		logger.WarnContext(ctx, "Error reading WebAuthn settings", "error", err)
	}

	return as, nil
}

func oidcSettings(connector types.OIDCConnector, cap types.AuthPreference) webclient.AuthenticationSettings {
	return webclient.AuthenticationSettings{
		Type: constants.OIDC,
		OIDC: &webclient.OIDCSettings{
			Name:      connector.GetName(),
			Display:   connector.GetDisplay(),
			IssuerURL: connector.GetIssuerURL(),
		},
		// Local fallback / MFA.
		SecondFactor:            types.LegacySecondFactorFromSecondFactors(cap.GetSecondFactors()),
		PreferredLocalMFA:       cap.GetPreferredLocalMFA(),
		PrivateKeyPolicy:        cap.GetPrivateKeyPolicy(),
		PIVSlot:                 cap.GetPIVSlot(),
		DeviceTrust:             deviceTrustSettings(cap),
		SignatureAlgorithmSuite: cap.GetSignatureAlgorithmSuite(),
	}
}

func samlSettings(connector types.SAMLConnector, cap types.AuthPreference) webclient.AuthenticationSettings {
	return webclient.AuthenticationSettings{
		Type: constants.SAML,
		SAML: &webclient.SAMLSettings{
			Name:                connector.GetName(),
			Display:             connector.GetDisplay(),
			SingleLogoutEnabled: connector.GetSingleLogoutURL() != "",
			// Note that we get the connector's primary SSO field, not the MFA SSO field.
			// These two values are often unique, but should have the same host prefix
			// (e.g. https://dev-813354.oktapreview.com) in reasonable, functional setups.
			SSO: connector.GetSSO(),
		},
		// Local fallback / MFA.
		SecondFactor:            types.LegacySecondFactorFromSecondFactors(cap.GetSecondFactors()),
		PreferredLocalMFA:       cap.GetPreferredLocalMFA(),
		PrivateKeyPolicy:        cap.GetPrivateKeyPolicy(),
		PIVSlot:                 cap.GetPIVSlot(),
		DeviceTrust:             deviceTrustSettings(cap),
		SignatureAlgorithmSuite: cap.GetSignatureAlgorithmSuite(),
	}
}

func githubSettings(connector types.GithubConnector, cap types.AuthPreference) webclient.AuthenticationSettings {
	return webclient.AuthenticationSettings{
		Type: constants.Github,
		Github: &webclient.GithubSettings{
			Name:        connector.GetName(),
			Display:     connector.GetDisplay(),
			EndpointURL: connector.GetEndpointURL(),
		},
		// Local fallback / MFA.
		SecondFactor:            types.LegacySecondFactorFromSecondFactors(cap.GetSecondFactors()),
		PreferredLocalMFA:       cap.GetPreferredLocalMFA(),
		PrivateKeyPolicy:        cap.GetPrivateKeyPolicy(),
		PIVSlot:                 cap.GetPIVSlot(),
		DeviceTrust:             deviceTrustSettings(cap),
		SignatureAlgorithmSuite: cap.GetSignatureAlgorithmSuite(),
	}
}

func deviceTrustSettings(cap types.AuthPreference) webclient.DeviceTrustSettings {
	dt := cap.GetDeviceTrust()
	return webclient.DeviceTrustSettings{
		Disabled:   deviceTrustDisabled(cap),
		AutoEnroll: dt != nil && dt.AutoEnroll,
	}
}

// deviceTrustDisabled is used to set its namesake field in
// [webclient.PingResponse.Auth].
func deviceTrustDisabled(cap types.AuthPreference) bool {
	return dtconfig.GetEffectiveMode(cap.GetDeviceTrust()) == constants.DeviceTrustModeOff
}

func getAuthSettings(ctx context.Context, authClient authclient.ClientI, logger *slog.Logger) (webclient.AuthenticationSettings, error) {
	authPreference, err := authClient.GetAuthPreference(ctx)
	if err != nil {
		return webclient.AuthenticationSettings{}, trace.Wrap(err)
	}

	var as webclient.AuthenticationSettings

	switch authPreference.GetType() {
	case constants.Local:
		as, err = localSettings(ctx, authPreference, logger)
		if err != nil {
			return webclient.AuthenticationSettings{}, trace.Wrap(err)
		}
	case constants.OIDC:
		if authPreference.GetConnectorName() != "" {
			oidcConnector, err := authClient.GetOIDCConnector(ctx, authPreference.GetConnectorName(), false)
			if err != nil {
				return webclient.AuthenticationSettings{}, trace.Wrap(err)
			}

			as = oidcSettings(oidcConnector, authPreference)
		} else {
			oidcConnectors, err := authClient.GetOIDCConnectors(ctx, false)
			if err != nil {
				return webclient.AuthenticationSettings{}, trace.Wrap(err)
			}
			if len(oidcConnectors) == 0 {
				return webclient.AuthenticationSettings{}, trace.BadParameter("no oidc connectors found")
			}

			as = oidcSettings(oidcConnectors[0], authPreference)
		}
	case constants.SAML:
		if authPreference.GetConnectorName() != "" {
			samlConnector, err := authClient.GetSAMLConnector(ctx, authPreference.GetConnectorName(), false)
			if err != nil {
				return webclient.AuthenticationSettings{}, trace.Wrap(err)
			}

			as = samlSettings(samlConnector, authPreference)
		} else {
			samlConnectors, err := authClient.GetSAMLConnectors(ctx, false)
			if err != nil {
				return webclient.AuthenticationSettings{}, trace.Wrap(err)
			}
			if len(samlConnectors) == 0 {
				return webclient.AuthenticationSettings{}, trace.BadParameter("no saml connectors found")
			}

			as = samlSettings(samlConnectors[0], authPreference)
		}
	case constants.Github:
		if authPreference.GetConnectorName() != "" {
			githubConnector, err := authClient.GetGithubConnector(ctx, authPreference.GetConnectorName(), false)
			if err != nil {
				return webclient.AuthenticationSettings{}, trace.Wrap(err)
			}
			as = githubSettings(githubConnector, authPreference)
		} else {
			githubConnectors, err := authClient.GetGithubConnectors(ctx, false)
			if err != nil {
				return webclient.AuthenticationSettings{}, trace.Wrap(err)
			}
			if len(githubConnectors) == 0 {
				return webclient.AuthenticationSettings{}, trace.BadParameter("no github connectors found")
			}
			as = githubSettings(githubConnectors[0], authPreference)
		}
	default:
		return webclient.AuthenticationSettings{}, trace.BadParameter("unknown type %v", authPreference.GetType())
	}

	as.HasMessageOfTheDay = authPreference.GetMessageOfTheDay() != ""
	pingResp, err := authClient.Ping(ctx)
	if err != nil {
		return webclient.AuthenticationSettings{}, trace.Wrap(err)
	}
	as.LoadAllCAs = pingResp.LoadAllCAs
	as.DefaultSessionTTL = authPreference.GetDefaultSessionTTL()

	return as, nil
}

// traces forwards spans from the web ui to the upstream collector configured for the proxy. If tracing is
// disabled then the forwarding is a noop.
func (h *Handler) traces(w http.ResponseWriter, r *http.Request, _ httprouter.Params, _ *SessionContext) (interface{}, error) {
	body, err := utils.ReadAtMost(r.Body, teleport.MaxHTTPResponseSize)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "Failed to read traces request", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		return nil, nil
	}

	if err := r.Body.Close(); err != nil {
		h.logger.WarnContext(r.Context(), "Failed to close traces request body", "error", err)
	}

	var data tracepb.TracesData
	if err := protojson.Unmarshal(body, &data); err != nil {
		h.logger.ErrorContext(r.Context(), "Failed to unmarshal traces request", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		return nil, nil
	}

	if len(data.ResourceSpans) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return nil, nil
	}

	// Unmarshalling of TraceId, SpanId, and ParentSpanId might all yield incorrect values. The raw values from
	// OpenTelemetry-js are hex encoded, but the unmarshal call above will decode them as base64.
	// In order to ensure the ids are in the right format and won't be rejected by the upstream collector
	// we attempt to convert them back into the base64 and then hex decode them.
	for _, resourceSpan := range data.ResourceSpans {
		for _, scopeSpan := range resourceSpan.ScopeSpans {
			for _, span := range scopeSpan.Spans {

				// attempt to convert the trace id to the right format
				if tid, err := oteltrace.TraceIDFromHex(base64.StdEncoding.EncodeToString(span.TraceId)); err == nil {
					span.TraceId = tid[:]
				}

				// attempt to convert the span id to the right format
				if sid, err := oteltrace.SpanIDFromHex(base64.StdEncoding.EncodeToString(span.SpanId)); err == nil {
					span.SpanId = sid[:]
				}

				// attempt to convert the parent span id to the right format
				if len(span.ParentSpanId) > 0 {
					if psid, err := oteltrace.SpanIDFromHex(base64.StdEncoding.EncodeToString(span.ParentSpanId)); err == nil {
						span.ParentSpanId = psid[:]
					}
				}
			}
		}
	}

	go func() {
		// Because the uploading happens in a goroutine we cannot use the request scoped context
		// since it will more than likely get canceled prior to the traces being uploaded. Use
		// a background context with a lenient timeout to allow for a large number of spans to complete.
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := h.cfg.TraceClient.UploadTraces(ctx, data.ResourceSpans); err != nil {
			h.logger.ErrorContext(ctx, "Failed to upload traces", "error", err)
		}
	}()

	w.WriteHeader(http.StatusOK)
	return nil, nil
}

func (h *Handler) ping(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var err error
	authSettings, err := getAuthSettings(r.Context(), h.cfg.ProxyClient, h.logger)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	proxyConfig, err := h.cfg.ProxySettings.GetProxySettings(r.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pr, err := h.cfg.ProxyClient.Ping(r.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	group := r.URL.Query().Get(webclient.AgentUpdateGroupParameter)

	return webclient.PingResponse{
		Auth:              authSettings,
		Proxy:             *proxyConfig,
		ServerVersion:     teleport.Version,
		MinClientVersion:  teleport.MinClientVersion,
		ClusterName:       h.auth.clusterName,
		AutomaticUpgrades: pr.ServerFeatures.GetAutomaticUpgrades(),
		AutoUpdate:        h.automaticUpdateSettings184(r.Context(), group, "" /* updater UUID */),
		Edition:           modules.GetModules().BuildType(),
		FIPS:              modules.IsBoringBinary(),
	}, nil
}

func (h *Handler) find(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	group := r.URL.Query().Get(webclient.AgentUpdateGroupParameter)
	cacheKey := "find"
	if group != "" {
		cacheKey += "-" + group
	}

	// cache the generic answer to avoid doing work for each request
	resp, err := utils.FnCacheGet[*webclient.PingResponse](r.Context(), h.findEndpointCache, cacheKey, func(ctx context.Context) (*webclient.PingResponse, error) {
		proxyConfig, err := h.cfg.ProxySettings.GetProxySettings(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		authPref, err := h.cfg.AccessPoint.GetAuthPreference(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return &webclient.PingResponse{
			Proxy:            *proxyConfig,
			Auth:             webclient.AuthenticationSettings{SignatureAlgorithmSuite: authPref.GetSignatureAlgorithmSuite()},
			ServerVersion:    teleport.Version,
			MinClientVersion: teleport.MinClientVersion,
			ClusterName:      h.auth.clusterName,
			Edition:          modules.GetModules().BuildType(),
			FIPS:             modules.IsBoringBinary(),
			AutoUpdate:       h.automaticUpdateSettings184(ctx, group, "" /* updater UUID */),
		}, nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

func (h *Handler) pingWithConnector(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	authClient := h.cfg.ProxyClient
	connectorName := p.ByName("connector")

	cap, err := authClient.GetAuthPreference(r.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pingResp, err := authClient.Ping(r.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	loadAllCAs := pingResp.LoadAllCAs

	proxyConfig, err := h.cfg.ProxySettings.GetProxySettings(r.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	response := &webclient.PingResponse{
		Proxy:            *proxyConfig,
		ServerVersion:    teleport.Version,
		MinClientVersion: teleport.MinClientVersion,
		ClusterName:      h.auth.clusterName,
	}

	hasMessageOfTheDay := cap.GetMessageOfTheDay() != ""
	if slices.Contains(constants.SystemConnectors, connectorName) {
		response.Auth, err = localSettings(r.Context(), cap, h.logger)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		response.Auth.HasMessageOfTheDay = hasMessageOfTheDay
		response.Auth.LoadAllCAs = loadAllCAs
		response.Auth.Local.Name = connectorName // echo connector queried by caller
		return response, nil
	}

	// collectorNames stores a list of the registered collector names so that
	// in the event that no connector has matched, the list can be returned.
	var collectorNames []string

	// first look for a oidc connector with that name
	oidcConnectors, err := authClient.GetOIDCConnectors(r.Context(), false)
	if err == nil {
		for index, value := range oidcConnectors {
			collectorNames = append(collectorNames, value.GetMetadata().Name)
			if value.GetMetadata().Name == connectorName {
				response.Auth = oidcSettings(oidcConnectors[index], cap)
				response.Auth.HasMessageOfTheDay = hasMessageOfTheDay
				response.Auth.LoadAllCAs = loadAllCAs
				return response, nil
			}
		}
	}

	// if no oidc connector was found, look for a saml connector
	samlConnectors, err := authClient.GetSAMLConnectors(r.Context(), false)
	if err == nil {
		for index, value := range samlConnectors {
			collectorNames = append(collectorNames, value.GetMetadata().Name)
			if value.GetMetadata().Name == connectorName {
				response.Auth = samlSettings(samlConnectors[index], cap)
				response.Auth.HasMessageOfTheDay = hasMessageOfTheDay
				response.Auth.LoadAllCAs = loadAllCAs
				return response, nil
			}
		}
	}

	// look for github connector
	githubConnectors, err := authClient.GetGithubConnectors(r.Context(), false)
	if err == nil {
		for index, value := range githubConnectors {
			collectorNames = append(collectorNames, value.GetMetadata().Name)
			if value.GetMetadata().Name == connectorName {
				response.Auth = githubSettings(githubConnectors[index], cap)
				response.Auth.HasMessageOfTheDay = hasMessageOfTheDay
				response.Auth.LoadAllCAs = loadAllCAs
				return response, nil
			}
		}
	}

	return nil,
		trace.BadParameter(
			"invalid connector name: %v; valid options: %s",
			connectorName, strings.Join(collectorNames, ", "))
}

// getWebConfig returns configuration for the web application.
func (h *Handler) getWebConfig(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	httplib.SetWebConfigHeaders(w.Header())

	authProviders := []webclient.WebConfigAuthProvider{}

	// get all OIDC connectors
	oidcConnectors, err := h.cfg.ProxyClient.GetOIDCConnectors(r.Context(), false)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "Cannot retrieve OIDC connectors", "error", err)
	}
	for _, item := range oidcConnectors {
		authProviders = append(authProviders, webclient.WebConfigAuthProvider{
			Type:        webclient.WebConfigAuthProviderOIDCType,
			WebAPIURL:   webclient.WebConfigAuthProviderOIDCURL,
			Name:        item.GetName(),
			DisplayName: item.GetDisplay(),
		})
	}

	// get all SAML connectors
	samlConnectors, err := h.cfg.ProxyClient.GetSAMLConnectors(r.Context(), false)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "Cannot retrieve SAML connectors", "error", err)
	}
	for _, item := range samlConnectors {
		authProviders = append(authProviders, webclient.WebConfigAuthProvider{
			Type:        webclient.WebConfigAuthProviderSAMLType,
			WebAPIURL:   webclient.WebConfigAuthProviderSAMLURL,
			Name:        item.GetName(),
			DisplayName: item.GetDisplay(),
		})
	}

	// get all Github connectors
	githubConnectors, err := h.cfg.ProxyClient.GetGithubConnectors(r.Context(), false)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "Cannot retrieve GitHub connectors", "error", err)
	}
	for _, item := range githubConnectors {
		authProviders = append(authProviders, webclient.WebConfigAuthProvider{
			Type:        webclient.WebConfigAuthProviderGitHubType,
			WebAPIURL:   webclient.WebConfigAuthProviderGitHubURL,
			Name:        item.GetName(),
			DisplayName: item.GetDisplay(),
		})
	}

	// get auth type & second factor type
	var authSettings webclient.WebConfigAuthSettings
	if cap, err := h.cfg.ProxyClient.GetAuthPreference(r.Context()); err != nil {
		h.logger.ErrorContext(r.Context(), "Cannot retrieve AuthPreferences", "error", err)
		authSettings = webclient.WebConfigAuthSettings{
			Providers:        authProviders,
			SecondFactor:     constants.SecondFactorOff,
			LocalAuthEnabled: true,
			AuthType:         constants.Local,
		}
	} else {
		authType := cap.GetType()
		var localConnectorName string
		var defaultConnectorName string

		if authType == constants.Local {
			localConnectorName = cap.GetConnectorName()
		} else {
			defaultConnectorName = cap.GetConnectorName()
		}

		authSettings = webclient.WebConfigAuthSettings{
			Providers:            authProviders,
			SecondFactor:         types.LegacySecondFactorFromSecondFactors(cap.GetSecondFactors()),
			LocalAuthEnabled:     cap.GetAllowLocalAuth(),
			AllowPasswordless:    cap.GetAllowPasswordless(),
			AuthType:             authType,
			DefaultConnectorName: defaultConnectorName,
			PreferredLocalMFA:    cap.GetPreferredLocalMFA(),
			LocalConnectorName:   localConnectorName,
			PrivateKeyPolicy:     cap.GetPrivateKeyPolicy(),
			MOTD:                 cap.GetMessageOfTheDay(),
		}
	}

	clusterFeatures := h.GetClusterFeatures()

	// get tunnel address to display on cloud instances
	tunnelPublicAddr := ""
	proxyConfig, err := h.cfg.ProxySettings.GetProxySettings(r.Context())
	if err != nil {
		h.logger.WarnContext(r.Context(), "Cannot retrieve ProxySettings, tunnel address won't be set in Web UI", "error", err)
	} else {
		if clusterFeatures.GetCloud() {
			tunnelPublicAddr = proxyConfig.SSH.TunnelPublicAddr
		}
	}

	// disable joining sessions if proxy session recording is enabled
	canJoinSessions := true
	recCfg, err := h.cfg.ProxyClient.GetSessionRecordingConfig(r.Context())
	if err != nil {
		h.logger.ErrorContext(r.Context(), "Cannot retrieve SessionRecordingConfig", "error", err)
	} else {
		canJoinSessions = !services.IsRecordAtProxy(recCfg.GetMode())
	}

	automaticUpgradesEnabled := clusterFeatures.GetAutomaticUpgrades()
	var automaticUpgradesTargetVersion string
	if automaticUpgradesEnabled {
		const group, updaterUUID = "", ""
		agentVersion, err := h.autoUpdateAgentVersion(r.Context(), group, updaterUUID)
		if err != nil {
			h.logger.ErrorContext(r.Context(), "Cannot read autoupdate target version", "error", err)
		} else {
			// agentVersion doesn't have the leading "v" which is expected here.
			automaticUpgradesTargetVersion = fmt.Sprintf("v%s", agentVersion)
		}
	}

	disableRoleVisualizer, _ := strconv.ParseBool(os.Getenv("TELEPORT_UNSTABLE_DISABLE_ROLE_VISUALIZER"))

	webCfg := webclient.WebConfig{
		Edition:                        modules.GetModules().BuildType(),
		Auth:                           authSettings,
		CanJoinSessions:                canJoinSessions,
		IsCloud:                        clusterFeatures.GetCloud(),
		TunnelPublicAddress:            tunnelPublicAddr,
		RecoveryCodesEnabled:           clusterFeatures.GetRecoveryCodes(),
		UI:                             h.getUIConfig(r.Context()),
		IsPolicyRoleVisualizerEnabled:  !disableRoleVisualizer,
		IsDashboard:                    services.IsDashboard(clusterFeatures),
		IsTeam:                         false,
		IsUsageBasedBilling:            clusterFeatures.GetIsUsageBased(),
		AutomaticUpgrades:              automaticUpgradesEnabled,
		AutomaticUpgradesTargetVersion: automaticUpgradesTargetVersion,
		CustomTheme:                    clusterFeatures.GetCustomTheme(),
		Questionnaire:                  clusterFeatures.GetQuestionnaire(),
		IsStripeManaged:                clusterFeatures.GetIsStripeManaged(),
		PremiumSupport:                 clusterFeatures.GetSupportType() == proto.SupportType_SUPPORT_TYPE_PREMIUM,
		PlayableDatabaseProtocols:      player.SupportedDatabaseProtocols,
		// Entitlements are reset/overridden in setEntitlementsWithLegacyLogic until setEntitlementsWithLegacyLogic is removed in v18
		Entitlements: GetWebCfgEntitlements(clusterFeatures.GetEntitlements()),
	}

	// Set entitlements with backwards field compatibility
	setEntitlementsWithLegacyLogic(&webCfg, clusterFeatures)

	resource, err := h.cfg.ProxyClient.GetClusterName()
	if err != nil {
		h.logger.WarnContext(r.Context(), "Failed to query cluster name", "error", err)
	} else {
		webCfg.ProxyClusterName = resource.GetClusterName()
	}

	out, err := json.Marshal(webCfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	fmt.Fprintf(w, "var GRV_CONFIG = %v;", string(out))
	return nil, nil
}

// setEntitlementsWithLegacyLogic ensures entitlements on webCfg are backwards compatible
// If Entitlements are present, will set the legacy fields equal to the equivalent entitlement value
// i.e. webCfg.IsIGSEnabled = clusterFeatures.Entitlements[entitlements.Identity].Enabled
// && webCfg.Entitlements[entitlements.Identity] = clusterFeatures.Entitlements[entitlements.Identity].Enabled
// If Entitlements are not present, will set the legacy fields AND the entitlement equal to the legacy feature
// i.e. webCfg.IsIGSEnabled = clusterFeatures.GetIdentityGovernance()
// && webCfg.Entitlements[entitlements.Identity] = clusterFeatures.GetIdentityGovernance()
// todo (michellescripts) remove in v18; & inline entitlement logic above
func setEntitlementsWithLegacyLogic(webCfg *webclient.WebConfig, clusterFeatures proto.Features) {
	// if Entitlements are not present, GetWebCfgEntitlements will return a map of entitlement to {enabled:false}
	// if Entitlements are present, GetWebCfgEntitlements will populate the fields appropriately
	webCfg.Entitlements = GetWebCfgEntitlements(clusterFeatures.GetEntitlements())

	if ent := clusterFeatures.GetEntitlements(); len(ent) > 0 {
		// webCfg.Entitlements: No update as they are set above
		// webCfg.<legacy fields>: set equal to entitlement value
		webCfg.AccessRequests = modules.GetProtoEntitlement(&clusterFeatures, entitlements.AccessRequests).Enabled
		webCfg.ExternalAuditStorage = modules.GetProtoEntitlement(&clusterFeatures, entitlements.ExternalAuditStorage).Enabled
		webCfg.HideInaccessibleFeatures = modules.GetProtoEntitlement(&clusterFeatures, entitlements.FeatureHiding).Enabled
		webCfg.IsIGSEnabled = modules.GetProtoEntitlement(&clusterFeatures, entitlements.Identity).Enabled
		webCfg.IsPolicyEnabled = modules.GetProtoEntitlement(&clusterFeatures, entitlements.Policy).Enabled
		webCfg.JoinActiveSessions = modules.GetProtoEntitlement(&clusterFeatures, entitlements.JoinActiveSessions).Enabled
		webCfg.MobileDeviceManagement = modules.GetProtoEntitlement(&clusterFeatures, entitlements.MobileDeviceManagement).Enabled
		webCfg.OIDC = modules.GetProtoEntitlement(&clusterFeatures, entitlements.OIDC).Enabled
		webCfg.SAML = modules.GetProtoEntitlement(&clusterFeatures, entitlements.SAML).Enabled
		webCfg.TrustedDevices = modules.GetProtoEntitlement(&clusterFeatures, entitlements.DeviceTrust).Enabled
		webCfg.FeatureLimits = webclient.FeatureLimits{
			AccessListCreateLimit:               int(modules.GetProtoEntitlement(&clusterFeatures, entitlements.AccessLists).Limit),
			AccessMonitoringMaxReportRangeLimit: int(modules.GetProtoEntitlement(&clusterFeatures, entitlements.AccessMonitoring).Limit),
			AccessRequestMonthlyRequestLimit:    int(modules.GetProtoEntitlement(&clusterFeatures, entitlements.AccessRequests).Limit),
		}

	} else {
		// webCfg.Entitlements: All records are {enabled: false}; update to equal legacy feature value
		webCfg.Entitlements[string(entitlements.ExternalAuditStorage)] = webclient.EntitlementInfo{Enabled: clusterFeatures.GetExternalAuditStorage()}
		webCfg.Entitlements[string(entitlements.FeatureHiding)] = webclient.EntitlementInfo{Enabled: clusterFeatures.GetFeatureHiding()}
		webCfg.Entitlements[string(entitlements.Identity)] = webclient.EntitlementInfo{Enabled: clusterFeatures.GetIdentityGovernance()}
		webCfg.Entitlements[string(entitlements.JoinActiveSessions)] = webclient.EntitlementInfo{Enabled: clusterFeatures.GetJoinActiveSessions()}
		webCfg.Entitlements[string(entitlements.MobileDeviceManagement)] = webclient.EntitlementInfo{Enabled: clusterFeatures.GetMobileDeviceManagement()}
		webCfg.Entitlements[string(entitlements.OIDC)] = webclient.EntitlementInfo{Enabled: clusterFeatures.GetOIDC()}
		webCfg.Entitlements[string(entitlements.Policy)] = webclient.EntitlementInfo{Enabled: clusterFeatures.GetPolicy() != nil && clusterFeatures.GetPolicy().Enabled}
		webCfg.Entitlements[string(entitlements.SAML)] = webclient.EntitlementInfo{Enabled: clusterFeatures.GetSAML()}
		// set default Identity fields to legacy feature value
		webCfg.Entitlements[string(entitlements.AccessLists)] = webclient.EntitlementInfo{Enabled: true, Limit: clusterFeatures.GetAccessList().GetCreateLimit()}
		webCfg.Entitlements[string(entitlements.AccessMonitoring)] = webclient.EntitlementInfo{Enabled: clusterFeatures.GetAccessMonitoring().GetEnabled(), Limit: clusterFeatures.GetAccessMonitoring().GetMaxReportRangeLimit()}
		webCfg.Entitlements[string(entitlements.AccessRequests)] = webclient.EntitlementInfo{Enabled: clusterFeatures.GetAccessRequests().GetMonthlyRequestLimit() > 0, Limit: clusterFeatures.GetAccessRequests().GetMonthlyRequestLimit()}
		webCfg.Entitlements[string(entitlements.DeviceTrust)] = webclient.EntitlementInfo{Enabled: clusterFeatures.GetDeviceTrust().GetEnabled(), Limit: clusterFeatures.GetDeviceTrust().GetDevicesUsageLimit()}
		// override Identity Package features if Identity is enabled: set true and clear limit
		if clusterFeatures.GetIdentityGovernance() {
			webCfg.Entitlements[string(entitlements.AccessLists)] = webclient.EntitlementInfo{Enabled: true}
			webCfg.Entitlements[string(entitlements.AccessMonitoring)] = webclient.EntitlementInfo{Enabled: true}
			webCfg.Entitlements[string(entitlements.AccessRequests)] = webclient.EntitlementInfo{Enabled: true}
			webCfg.Entitlements[string(entitlements.DeviceTrust)] = webclient.EntitlementInfo{Enabled: true}
			webCfg.Entitlements[string(entitlements.OktaSCIM)] = webclient.EntitlementInfo{Enabled: true}
			webCfg.Entitlements[string(entitlements.OktaUserSync)] = webclient.EntitlementInfo{Enabled: true}
			webCfg.Entitlements[string(entitlements.SessionLocks)] = webclient.EntitlementInfo{Enabled: true}
		}

		// webCfg.<legacy fields>: set equal to legacy feature value
		webCfg.AccessRequests = clusterFeatures.GetAccessRequests().GetMonthlyRequestLimit() > 0
		webCfg.ExternalAuditStorage = clusterFeatures.GetExternalAuditStorage()
		webCfg.HideInaccessibleFeatures = clusterFeatures.GetFeatureHiding()
		webCfg.IsIGSEnabled = clusterFeatures.GetIdentityGovernance()
		webCfg.IsPolicyEnabled = clusterFeatures.GetPolicy() != nil && clusterFeatures.GetPolicy().Enabled
		webCfg.JoinActiveSessions = clusterFeatures.GetJoinActiveSessions()
		webCfg.MobileDeviceManagement = clusterFeatures.GetMobileDeviceManagement()
		webCfg.OIDC = clusterFeatures.GetOIDC()
		webCfg.SAML = clusterFeatures.GetSAML()
		webCfg.TrustedDevices = clusterFeatures.GetDeviceTrust().GetEnabled()
		webCfg.FeatureLimits = webclient.FeatureLimits{
			AccessListCreateLimit:               int(clusterFeatures.GetAccessList().GetCreateLimit()),
			AccessMonitoringMaxReportRangeLimit: int(clusterFeatures.GetAccessMonitoring().GetMaxReportRangeLimit()),
			AccessRequestMonthlyRequestLimit:    int(clusterFeatures.GetAccessRequests().GetMonthlyRequestLimit()),
		}
	}
}

// GetWebCfgEntitlements takes a cloud entitlement set and returns a modules Entitlement set
func GetWebCfgEntitlements(protoEntitlements map[string]*proto.EntitlementInfo) map[string]webclient.EntitlementInfo {
	all := entitlements.AllEntitlements
	result := make(map[string]webclient.EntitlementInfo, len(all))

	for _, e := range all {
		al, ok := protoEntitlements[string(e)]
		if !ok {
			result[string(e)] = webclient.EntitlementInfo{}
			continue
		}

		result[string(e)] = webclient.EntitlementInfo{
			Enabled: al.Enabled,
			Limit:   al.Limit,
		}
	}

	return result
}

type JWKSResponse struct {
	// Keys is a list of public keys in JWK format.
	Keys []jwt.JWK `json:"keys"`
}

// getUiConfig will first try to get an ui_config set in the cache and then
// return what was set by the file config. Returns nil if neither are set which
// is fine, as the web UI can set its own defaults.
func (h *Handler) getUIConfig(ctx context.Context) webclient.UIConfig {
	if uiConfig, err := h.cfg.AccessPoint.GetUIConfig(ctx); err == nil && uiConfig != nil {
		return webclient.UIConfig{
			ScrollbackLines: int(uiConfig.GetScrollbackLines()),
			ShowResources:   uiConfig.GetShowResources(),
		}
	}
	return h.cfg.UI
}

// jwks returns all public keys used to sign JWT tokens for this cluster.
func (h *Handler) wellKnownJWKS(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	return h.jwks(r.Context(), types.JWTSigner, true)
}

func (h *Handler) motd(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	authPrefs, err := h.cfg.ProxyClient.GetAuthPreference(r.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return webclient.MotD{Text: authPrefs.GetMessageOfTheDay()}, nil
}

func (h *Handler) githubLoginWeb(w http.ResponseWriter, r *http.Request, p httprouter.Params) string {
	logger := h.logger.With("auth", "github")
	logger.DebugContext(r.Context(), "Web login start")

	req, err := ParseSSORequestParams(r)
	if err != nil {
		logger.ErrorContext(r.Context(), "Failed to extract SSO parameters from request", "error", err)
		return client.LoginFailedRedirectURL
	}

	remoteAddr, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		logger.ErrorContext(r.Context(), "Failed to parse request remote address", "error", err)
		return client.LoginFailedRedirectURL
	}

	response, err := h.cfg.ProxyClient.CreateGithubAuthRequest(r.Context(), types.GithubAuthRequest{
		CSRFToken:         req.CSRFToken,
		ConnectorID:       req.ConnectorID,
		CreateWebSession:  true,
		ClientRedirectURL: req.ClientRedirectURL,
		ClientLoginIP:     remoteAddr,
		ClientUserAgent:   r.UserAgent(),
	})
	if err != nil {
		logger.ErrorContext(r.Context(), "Error creating auth request", "error", err)
		return client.LoginFailedRedirectURL
	}

	return response.RedirectURL
}

func (h *Handler) githubLoginConsole(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	logger := h.logger.With("auth", "github")
	logger.DebugContext(r.Context(), "Console login start")

	req := new(client.SSOLoginConsoleReq)
	if err := httplib.ReadResourceJSON(r, req); err != nil {
		logger.ErrorContext(r.Context(), "Error reading json", "error", err)
		return nil, trace.AccessDenied("%s", SSOLoginFailureMessage)
	}

	if err := req.CheckAndSetDefaults(); err != nil {
		logger.ErrorContext(r.Context(), "Missing request parameters", "error", err)
		return nil, trace.AccessDenied("%s", SSOLoginFailureMessage)
	}

	remoteAddr, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		logger.ErrorContext(r.Context(), "Failed to parse request remote address", "error", err)
		return nil, trace.AccessDenied("%s", SSOLoginFailureMessage)
	}

	response, err := h.cfg.ProxyClient.CreateGithubAuthRequest(r.Context(), types.GithubAuthRequest{
		ConnectorID:             req.ConnectorID,
		SshPublicKey:            req.SSHPubKey,
		TlsPublicKey:            req.TLSPubKey,
		SshAttestationStatement: req.SSHAttestationStatement.ToProto(),
		TlsAttestationStatement: req.TLSAttestationStatement.ToProto(),
		CertTTL:                 req.CertTTL,
		ClientRedirectURL:       req.RedirectURL,
		Compatibility:           req.Compatibility,
		RouteToCluster:          req.RouteToCluster,
		KubernetesCluster:       req.KubernetesCluster,
		ClientLoginIP:           remoteAddr,
	})
	if err != nil {
		logger.ErrorContext(r.Context(), "Failed to create GitHub auth request", "error", err)
		if strings.Contains(err.Error(), auth.InvalidClientRedirectErrorMessage) {
			return nil, trace.AccessDenied("%s", SSOLoginFailureInvalidRedirect)
		}
		return nil, trace.AccessDenied("%s", SSOLoginFailureMessage)
	}

	return &client.SSOLoginConsoleResponse{
		RedirectURL: response.RedirectURL,
	}, nil
}

func (h *Handler) githubCallback(w http.ResponseWriter, r *http.Request, p httprouter.Params) string {
	logger := h.logger.With("auth", "github")
	logger.DebugContext(r.Context(), "Callback start", "query", r.URL.Query())

	response, err := h.cfg.ProxyClient.ValidateGithubAuthCallback(r.Context(), r.URL.Query())
	if err != nil {
		logger.ErrorContext(r.Context(), "Error while processing callback", "error", err)

		// try to find the auth request, which bears the original client redirect URL.
		// if found, use it to terminate the flow.
		//
		// this improves the UX by terminating the failed SSO flow immediately, rather than hoping for a timeout.
		if requestID := r.URL.Query().Get("state"); requestID != "" {
			if request, errGet := h.cfg.ProxyClient.GetGithubAuthRequest(r.Context(), requestID); errGet == nil && !request.CreateWebSession {
				if redURL, errEnc := RedirectURLWithError(request.ClientRedirectURL, err); errEnc == nil {
					return redURL.String()
				}
			}
		}
		if errors.Is(err, auth.ErrGithubNoTeams) {
			return client.LoginFailedUnauthorizedRedirectURL
		}

		return client.LoginFailedBadCallbackRedirectURL
	}

	// if we created web session, set session cookie and redirect to original url
	if response.Req.CreateWebSession {
		logger.InfoContext(r.Context(), "Redirecting to web browser")

		res := &SSOCallbackResponse{
			CSRFToken:         response.Req.CSRFToken,
			Username:          response.Username,
			SessionName:       response.Session.GetName(),
			ClientRedirectURL: response.Req.ClientRedirectURL,
		}

		if err := SSOSetWebSessionAndRedirectURL(w, r, res, true); err != nil {
			logger.ErrorContext(r.Context(), "Error setting web session.", "error", err)
			return client.LoginFailedRedirectURL
		}

		if dwt := response.Session.GetDeviceWebToken(); dwt != nil {
			logger.DebugContext(r.Context(), "GitHub WebSession created with device web token")
			// if a device web token is present, we must send the user to the device authorize page
			// to upgrade the session.
			redirectPath, err := BuildDeviceWebRedirectPath(dwt, res.ClientRedirectURL)
			if err != nil {
				logger.DebugContext(r.Context(), "Invalid device web token", "error", err)
			}
			return redirectPath
		}
		return res.ClientRedirectURL
	}

	logger.InfoContext(r.Context(), "Callback is redirecting to console login")
	if len(response.Req.SSHPubKey)+len(response.Req.TLSPubKey) == 0 {
		logger.ErrorContext(r.Context(), "Not a web or console login request")
		return client.LoginFailedRedirectURL
	}

	redirectURL, err := ConstructSSHResponse(AuthParams{
		ClientRedirectURL: response.Req.ClientRedirectURL,
		Username:          response.Username,
		Identity:          response.Identity,
		Session:           response.Session,
		Cert:              response.Cert,
		TLSCert:           response.TLSCert,
		HostSigners:       response.HostSigners,
		FIPS:              h.cfg.FIPS,
	})
	if err != nil {
		logger.ErrorContext(r.Context(), "Error constructing ssh response", "error", err)
		return client.LoginFailedRedirectURL
	}

	return redirectURL.String()
}

// BuildDeviceWebRedirectPath constructs the redirect path for device web authorization.
// It takes a DeviceWebToken and an optional client redirect URL as input.
// The function formats a redirect path with the device ID and token from the provided DeviceWebToken.
// If the clientRedirectURL is provided, it's appended to the redirect path
// as a query parameter named "redirect_uri".
// Will always at least return "/web" path.
func BuildDeviceWebRedirectPath(dwt *types.DeviceWebToken, clientRedirectURL string) (string, error) {
	const basePath = "/web"

	if dwt == nil {
		return basePath, trace.BadParameter("DeviceWebToken cannot be nil")
	}
	if dwt.Id == "" || dwt.Token == "" {
		return basePath, trace.BadParameter("DeviceWebToken ID and Token cannot be empty")
	}
	redirectPath := fmt.Sprintf("/web/device/authorize/%s/%s", dwt.Id, dwt.Token)
	if clientRedirectURL != "" {
		redirectPath = fmt.Sprintf("%s?redirect_uri=%s", redirectPath, clientRedirectURL)
	}
	return redirectPath, nil
}

func (h *Handler) installer(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	httplib.SetScriptHeaders(w.Header())

	installerName := p.ByName("name")
	installer, err := h.auth.proxyClient.GetInstaller(r.Context(), installerName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ping, err := h.auth.Ping(r.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// semver parsing requires a 'v' at the beginning of the version string.
	version := semver.Major("v" + ping.ServerVersion)
	instTmpl, err := template.New("").Parse(installer.GetScript())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	feats := modules.GetModules().Features()
	teleportPackage := types.PackageNameOSS
	if modules.GetModules().BuildType() == modules.BuildEnterprise || feats.Cloud {
		teleportPackage = types.PackageNameEnt
		if h.cfg.FIPS {
			teleportPackage = types.PackageNameEntFIPS
		}
	}

	// By default, it uses the stable/v<majorVersion> channel.
	repoChannel := fmt.Sprintf("stable/%s", version)

	// If the updater must be installed, then change the repo to stable/cloud
	// It must also install the version specified in
	// https://updates.releases.teleport.dev/v1/stable/cloud/version
	installUpdater := automaticUpgrades(*ping.ServerFeatures)
	if installUpdater {
		repoChannel = automaticupgrades.DefaultCloudChannelName
	}
	azureClientID := r.URL.Query().Get("azure-client-id")

	tmpl := installers.Template{
		PublicProxyAddr:   h.PublicProxyAddr(),
		MajorVersion:      shsprintf.EscapeDefaultContext(version),
		TeleportPackage:   teleportPackage,
		RepoChannel:       shsprintf.EscapeDefaultContext(repoChannel),
		AutomaticUpgrades: strconv.FormatBool(installUpdater),
		AzureClientID:     shsprintf.EscapeDefaultContext(azureClientID),
	}
	err = instTmpl.Execute(w, tmpl)
	return nil, trace.Wrap(err)
}

// AuthParams are used to construct redirect URL containing auth
// information back to tsh login
type AuthParams struct {
	// Username is authenticated teleport username
	Username string
	// Identity contains validated OIDC identity
	Identity types.ExternalIdentity
	// Web session will be generated by auth server if requested in OIDCAuthRequest
	Session types.WebSession
	// Cert will be generated by certificate authority
	Cert []byte
	// TLSCert is PEM encoded TLS certificate
	TLSCert []byte
	// HostSigners is a list of signing host public keys
	// trusted by proxy, used in console login
	HostSigners []types.CertAuthority
	// ClientRedirectURL is a URL to redirect client to
	ClientRedirectURL string
	// FIPS mode means Teleport started in a FedRAMP/FIPS 140-2 compliant
	// configuration.
	FIPS bool
	// MFAToken is an SSO MFA token.
	MFAToken string
}

// ConstructSSHResponse creates a special SSH response for SSH login method
// that encodes everything using the client's secret key
func ConstructSSHResponse(response AuthParams) (*url.URL, error) {
	u, err := url.Parse(response.ClientRedirectURL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	consoleResponse := authclient.SSHLoginResponse{
		Username:    response.Username,
		Cert:        response.Cert,
		TLSCert:     response.TLSCert,
		HostSigners: authclient.AuthoritiesToTrustedCerts(response.HostSigners),
		MFAToken:    response.MFAToken,
	}
	out, err := json.Marshal(consoleResponse)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Extract secret out of the request.
	secretKey := u.Query().Get("secret_key")

	// We don't use a secret key for WebUI SSO MFA redirects. The request ID itself is
	// kept a secret on the front end to minimize the risk of a phishing attack.
	if secretKey == "" && u.Path == sso.WebMFARedirect && response.MFAToken != "" {
		q := u.Query()
		q.Add("response", string(out))
		u.RawQuery = q.Encode()
		return u, nil
	}

	if secretKey == "" {
		return nil, trace.BadParameter("missing secret_key")
	}

	var ciphertext []byte

	// AES-GCM based symmetric cipher.
	key, err := secret.ParseKey([]byte(secretKey))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ciphertext, err = key.Seal(out)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Place ciphertext into the redirect URL.
	u.RawQuery = url.Values{"response": {string(ciphertext)}}.Encode()
	return u, nil
}

// RedirectURLWithError adds an err query parameter to the given redirect URL with the
// given errReply message and returns the new URL. If the given URL cannot be parsed,
// an error is returned with a nil URL. It is used to return an error back to the
// original URL in an SSO callback when validation fails.
func RedirectURLWithError(clientRedirectURL string, errReply error) (*url.URL, error) {
	u, err := url.Parse(clientRedirectURL)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	values := u.Query()
	values.Set("err", errReply.Error())

	u.RawQuery = values.Encode()
	return u, nil
}

// CreateSessionReq is a request to create session from username, password and
// second factor token.
type CreateSessionReq struct {
	// User is the Teleport username.
	User string `json:"user"`
	// Pass is the password.
	Pass string `json:"pass"`
	// SecondFactorToken is the OTP.
	SecondFactorToken string `json:"second_factor_token"`
}

// String returns text description of this response
func (r *CreateSessionResponse) String() string {
	return fmt.Sprintf("WebSession(type=%v,token=%v,expires=%vs)",
		r.TokenType, r.Token, r.TokenExpiresIn)
}

// CreateSessionResponse returns OAuth compabible data about
// access token: https://tools.ietf.org/html/rfc6749
type CreateSessionResponse struct {
	// TokenType is token type (bearer)
	TokenType string `json:"type"`
	// Token value
	Token string `json:"token"`
	// TokenExpiresIn sets seconds before this token is not valid
	TokenExpiresIn int `json:"expires_in"`
	// SessionExpiresIn is the seconds before the session itself expires.
	SessionExpiresIn int `json:"sessionExpiresIn,omitempty"`
	// SessionExpires is when this session expires.
	SessionExpires time.Time `json:"sessionExpires,omitempty"`
	// SessionInactiveTimeoutMS specifies how long in milliseconds
	// a user WebUI session can be left idle before being logged out
	// by the server. A zero value means there is no idle timeout set.
	SessionInactiveTimeoutMS int `json:"sessionInactiveTimeout"`
	// DeviceWebToken is the token used to perform on-behalf-of device
	// authentication.
	// If not nil it should be forwarded to Connect for the device authentication
	// ceremony.
	DeviceWebToken *types.DeviceWebToken `json:"deviceWebToken,omitempty"`
	// TrustedDeviceRequirement calculated for the web session.
	// Follows [types.TrustedDeviceRequirement].
	TrustedDeviceRequirement int32 `json:"trustedDeviceRequirement,omitempty"`
}

func newSessionResponse(sctx *SessionContext) (*CreateSessionResponse, error) {
	accessChecker, err := sctx.GetUserAccessChecker()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	_, err = accessChecker.CheckLoginDuration(0)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	token, err := sctx.getToken()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	now := sctx.cfg.Parent.clock.Now()
	sessionExpiryTime := sctx.cfg.Session.GetExpiryTime()

	return &CreateSessionResponse{
		TokenType:                roundtrip.AuthBearer,
		Token:                    token.GetName(),
		TokenExpiresIn:           int(token.Expiry().Sub(now) / time.Second),
		SessionExpiresIn:         int(sessionExpiryTime.Sub(now) / time.Second),
		SessionExpires:           sessionExpiryTime,
		SessionInactiveTimeoutMS: int(sctx.cfg.Session.GetIdleTimeout().Milliseconds()),
		DeviceWebToken:           sctx.cfg.Session.GetDeviceWebToken(),
		TrustedDeviceRequirement: int32(sctx.cfg.Session.GetTrustedDeviceRequirement()),
	}, nil
}

// createWebSession creates a new web session based on user, pass and 2nd factor token
//
// POST /v1/webapi/sessions/web
//
// {"user": "alex", "pass": "abcdef123456", "second_factor_token": "token", "second_factor_type": "totp"}
//
// # Response
//
// {"type": "bearer", "token": "bearer token", "user": {"name": "alex", "allowed_logins": ["admin", "bob"]}, "expires_in": 20}
func (h *Handler) createWebSession(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req *CreateSessionReq
	if err := httplib.ReadResourceJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	// get cluster preferences to see if we should login
	// with password or password+otp
	authClient := h.cfg.ProxyClient
	cap, err := authClient.GetAuthPreference(r.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clientMeta := clientMetaFromReq(r)

	var webSession types.WebSession
	switch {
	case !cap.IsSecondFactorEnforced():
		webSession, err = h.auth.AuthWithoutOTP(r.Context(), req.User, req.Pass, clientMeta)
	case req.SecondFactorToken == "" && !cap.IsSecondFactorEnforced():
		webSession, err = h.auth.AuthWithoutOTP(r.Context(), req.User, req.Pass, clientMeta)
	case cap.IsSecondFactorTOTPAllowed():
		webSession, err = h.auth.AuthWithOTP(r.Context(), req.User, req.Pass, req.SecondFactorToken, clientMeta)
	default:
		return nil, trace.AccessDenied("direct login with password+otp not supported by this cluster")
	}
	if err != nil {
		h.logger.WarnContext(r.Context(), "Access attempt denied for user", "user", req.User, "error", err)
		// Since checking for private key policy meant that they passed authn,
		// return policy error as is to help direct user.
		if keys.IsPrivateKeyPolicyError(err) {
			return nil, trace.Wrap(err)
		}
		// Obscure all other errors.
		return nil, trace.AccessDenied("invalid credentials")
	}

	if err := websession.SetCookie(w, req.User, webSession.GetName()); err != nil {
		return nil, trace.Wrap(err)
	}

	ctx, err := h.auth.newSessionContextFromSession(r.Context(), webSession)
	if err != nil {
		h.logger.WarnContext(r.Context(), "Access attempt denied for user", "user", req.User, "error", err)
		return nil, trace.AccessDenied("need auth")
	}

	res, err := newSessionResponse(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	res.SessionExpires = webSession.GetExpiryTime()

	// request that the browser starts a new device-bound session
	// TODO(zmb3): support both RS256 and ES256
	// TODO(zmb3): determine whether we should pass the authorization attribute
	h.logger.InfoContext(r.Context(), "requesting dbsc flow")
	w.Header().Add(websession.SecureSessionRegistrationHeader,
		fmt.Sprintf(`(RS256);challenge="nonce";path="/webapi/securesession/startsession";authorization=%v`, res.Token))
	//	(ES256 RS256); path="StartSession"; challenge="DBSC-challenge5"; authorization="auth-code-123"

	return res, nil
}

// startSecureSession is the endpoint that browsers use to initiate the secure (device bound)
// session flow.
func (h *Handler) startSecureSession(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	h.logger.InfoContext(r.Context(), "starting DBSC flow")
	registrationJWT := r.Header.Get("Sec-Session-Response")
	if registrationJWT == "" {
		http.Error(w, "missing registration JWT", http.StatusBadRequest)
		return
	}

	h.logger.InfoContext(r.Context(), "got registration JWT", "jwt", registrationJWT)
	w.WriteHeader(http.StatusOK)
}

func clientMetaFromReq(r *http.Request) *authclient.ForwardedClientMetadata {
	return &authclient.ForwardedClientMetadata{
		UserAgent:  r.UserAgent(),
		RemoteAddr: r.RemoteAddr,
	}
}

// deleteWebSession is called to sign out user from web, app and SAML IdP session.
//
// DELETE /v1/webapi/sessions/:sid
//
// Response:
//
// {"message": "ok"}
func (h *Handler) deleteWebSession(w http.ResponseWriter, r *http.Request, _ httprouter.Params, ctx *SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		h.logger.WarnContext(r.Context(), "Failed to retrieve user client, SAML single logout will be skipped for user",
			"user", ctx.GetUser(),
			"error", err,
		)
	}

	var user types.User
	// Only run this if we successfully retrieved the client.
	if err == nil {
		user, err = clt.GetUser(r.Context(), ctx.GetUser(), false)
		if err != nil {
			h.logger.WarnContext(r.Context(), "Failed to retrieve user during logout, SAML single logout will be skipped for user",
				"user", ctx.GetUser(),
				"error", err,
			)
		}
	}

	if err := h.logout(r.Context(), w, ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	// If the user has SAML SLO (single logout) configured, return a redirect link to the SLO URL.
	if user != nil && len(user.GetSAMLIdentities()) > 0 && user.GetSAMLIdentities()[0].SAMLSingleLogoutURL != "" {
		// The WebUI will redirect the user to this URL to initiate the SAML SLO on the IdP side. This is safe because this URL
		// is hard-coded in the auth connector and can't be modified by the end user. Additionally, the user's Teleport session has already
		// been invalidated by this point so there is nothing to hijack.
		return map[string]interface{}{"samlSloUrl": user.GetSAMLIdentities()[0].SAMLSingleLogoutURL}, nil
	}

	return OK(), nil
}

func (h *Handler) logout(ctx context.Context, w http.ResponseWriter, sctx *SessionContext) error {
	if err := sctx.Invalidate(ctx); err != nil {
		h.logger.WarnContext(ctx, "Failed to invalidate sessions",
			"user", sctx.GetUser(),
			"error", err,
		)
	}

	if err := h.auth.releaseResources(ctx, sctx.GetUser(), sctx.GetSessionID()); err != nil {
		h.logger.DebugContext(ctx, "sessionCache: Failed to release web session",
			"session_id", sctx.GetSessionID(),
			"error", err,
		)
	}
	clearSessionCookies(w)

	return nil
}

// clearSessionCookies clears Web UI session and SAML session cookie.
func clearSessionCookies(w http.ResponseWriter) {
	// Clear Web UI session cookie
	websession.ClearCookie(w)
}

type renewSessionRequest struct {
	// AccessRequestID is the id of an approved access request.
	AccessRequestID string `json:"requestId"`
	// Switchback indicates switching back to default roles when creating new session.
	Switchback bool `json:"switchback"`
	// ReloadUser is a flag to indicate if user needs to be refetched from the backend
	// to apply new user changes e.g. user traits were updated.
	ReloadUser bool `json:"reloadUser"`
}

// renewWebSession updates this existing session with a new session.
//
// Depending on request fields sent in for extension, the new session creation can vary depending on:
//   - AccessRequestID (opt): appends roles approved from access request to currently assigned roles or,
//   - Switchback (opt): roles stacked with assuming approved access requests, will revert to user's default roles
//   - ReloadUser (opt): similar to default but updates user related data (e.g login traits) by retrieving it from the backend
//   - default (none set): create new session with currently assigned roles
func (h *Handler) renewWebSession(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *SessionContext) (interface{}, error) {
	req := renewSessionRequest{}
	if err := httplib.ReadResourceJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	if req.AccessRequestID != "" && req.Switchback || req.AccessRequestID != "" && req.ReloadUser || req.Switchback && req.ReloadUser {
		return nil, trace.BadParameter("failed to renew session: only one field can be set")
	}

	newSession, err := ctx.extendWebSession(r.Context(), req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	newContext, err := h.auth.newSessionContextFromSession(r.Context(), newSession)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := websession.SetCookie(w, newSession.GetUser(), newSession.GetName()); err != nil {
		return nil, trace.Wrap(err)
	}

	res, err := newSessionResponse(newContext)
	return res, trace.Wrap(err)
}

type changeUserAuthenticationRequest struct {
	// SecondFactorToken is the TOTP code.
	SecondFactorToken string `json:"second_factor_token"`
	// TokenID is the ID of a reset or invite token.
	TokenID string `json:"token"`
	// DeviceName is the name of new mfa or passwordless device.
	DeviceName string `json:"deviceName"`
	// Password is user password string converted to bytes.
	Password []byte `json:"password"`
	// WebauthnCreationResponse is the signed credential creation response.
	WebauthnCreationResponse *wantypes.CredentialCreationResponse `json:"webauthnCreationResponse"`
}

func (h *Handler) changeUserAuthentication(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req changeUserAuthenticationRequest
	if err := httplib.ReadResourceJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	protoReq := &proto.ChangeUserAuthenticationRequest{
		TokenID:       req.TokenID,
		NewPassword:   req.Password,
		NewDeviceName: req.DeviceName,
	}
	switch {
	case req.WebauthnCreationResponse != nil:
		protoReq.NewMFARegisterResponse = &proto.MFARegisterResponse{
			Response: &proto.MFARegisterResponse_Webauthn{
				Webauthn: wantypes.CredentialCreationResponseToProto(req.WebauthnCreationResponse),
			},
		}
	case req.SecondFactorToken != "":
		protoReq.NewMFARegisterResponse = &proto.MFARegisterResponse{Response: &proto.MFARegisterResponse_TOTP{
			TOTP: &proto.TOTPRegisterResponse{Code: req.SecondFactorToken},
		}}
	}

	remoteAddr, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	protoReq.LoginIP = remoteAddr

	res, err := h.auth.proxyClient.ChangeUserAuthentication(r.Context(), protoReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if res.PrivateKeyPolicyEnabled {
		if res.GetRecovery() == nil {
			return &ui.ChangedUserAuthn{
				PrivateKeyPolicyEnabled: res.PrivateKeyPolicyEnabled,
			}, nil
		}
		return &ui.ChangedUserAuthn{
			Recovery: ui.RecoveryCodes{
				Codes:   res.GetRecovery().GetCodes(),
				Created: &res.GetRecovery().Created,
			},
			PrivateKeyPolicyEnabled: res.PrivateKeyPolicyEnabled,
		}, nil
	}

	sess := res.WebSession
	ctx, err := h.auth.newSessionContextFromSession(r.Context(), sess)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := websession.SetCookie(w, sess.GetUser(), sess.GetName()); err != nil {
		return nil, trace.Wrap(err)
	}

	// Checks for at least one valid login.
	if _, err := newSessionResponse(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	if res.GetRecovery() == nil {
		return &ui.ChangedUserAuthn{}, nil
	}

	return &ui.ChangedUserAuthn{
		Recovery: ui.RecoveryCodes{
			Codes:   res.GetRecovery().GetCodes(),
			Created: &res.GetRecovery().Created,
		},
	}, nil
}

// createResetPasswordToken allows a UI user to reset a user's password.
// This handler is also required for after creating new users.
func (h *Handler) createResetPasswordToken(w http.ResponseWriter, r *http.Request, _ httprouter.Params, ctx *SessionContext) (interface{}, error) {
	var req authclient.CreateUserTokenRequest
	if err := httplib.ReadResourceJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	token, err := clt.CreateResetPasswordToken(r.Context(),
		authclient.CreateUserTokenRequest{
			Name: req.Name,
			Type: req.Type,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.ResetPasswordToken{
		TokenID: token.GetName(),
		Expiry:  token.Expiry(),
		User:    token.GetUser(),
	}, nil
}

func (h *Handler) getResetPasswordTokenHandle(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	result, err := h.getResetPasswordToken(r.Context(), p.ByName("token"))
	if err != nil {
		h.logger.WarnContext(r.Context(), "Failed to fetch a reset password token", "error", err)
		// We hide the error from the remote user to avoid giving any hints.
		return nil, trace.AccessDenied("bad or expired token")
	}

	return result, nil
}

func (h *Handler) getResetPasswordToken(ctx context.Context, tokenID string) (interface{}, error) {
	token, err := h.auth.proxyClient.GetResetPasswordToken(ctx, tokenID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// CreateRegisterChallenge rotates TOTP secrets for a given tokenID.
	// It is required to get called every time a user fetches 2nd-factor secrets during registration attempt.
	// This ensures that an attacker that gains the ResetPasswordToken link can not view it,
	// extract the OTP key from the QR code, then allow the user to signup with
	// the same OTP token.
	res, err := h.auth.proxyClient.CreateRegisterChallenge(ctx, &proto.CreateRegisterChallengeRequest{
		TokenID:    tokenID,
		DeviceType: proto.DeviceType_DEVICE_TYPE_TOTP,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.ResetPasswordToken{
		TokenID: token.GetName(),
		User:    token.GetUser(),
		QRCode:  res.GetTOTP().GetQRCode(),
	}, nil
}

// mfaLoginBegin is the first step in the MFA authentication ceremony, which
// may be completed either via mfaLoginFinish (SSH) or mfaLoginFinishSession
// (Web).
//
// POST /webapi/mfa/login/begin
//
// {"user": "alex", "pass": "abcdef123456"}
// {"passwordless": true}
//
// Successful response:
//
// {"webauthn_challenge": {...}, "totp_challenge": true}
// {"webauthn_challenge": {...}} // passwordless
func (h *Handler) mfaLoginBegin(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req *client.MFAChallengeRequest
	if err := httplib.ReadResourceJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	mfaReq := &proto.CreateAuthenticateChallengeRequest{}
	if req.Passwordless {
		mfaReq.Request = &proto.CreateAuthenticateChallengeRequest_Passwordless{
			Passwordless: &proto.Passwordless{},
		}

		mfaReq.ChallengeExtensions = &mfav1.ChallengeExtensions{
			Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_PASSWORDLESS_LOGIN,
		}
	} else {
		mfaReq.Request = &proto.CreateAuthenticateChallengeRequest_UserCredentials{
			UserCredentials: &proto.UserCredentials{
				Username: req.User,
				Password: []byte(req.Pass),
			},
		}

		mfaReq.ChallengeExtensions = &mfav1.ChallengeExtensions{
			Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
		}
	}

	mfaChallenge, err := h.auth.proxyClient.CreateAuthenticateChallenge(r.Context(), mfaReq)
	if err != nil {
		// Do not obfuscate config-related errors.
		if errors.Is(err, types.ErrPasswordlessRequiresWebauthn) || errors.Is(err, types.ErrPasswordlessDisabledBySettings) {
			return nil, trace.Wrap(err)
		}
		return nil, trace.AccessDenied("invalid credentials")
	}

	return makeAuthenticateChallenge(mfaChallenge, "" /*channelID*/), nil
}

// mfaLoginFinish completes the MFA login ceremony, returning a new SSH
// certificate if successful.
//
// POST /v1/mfa/login/finish
//
// { "user": "bob", "password": "pass", "pub_key": "key to sign", "ttl": 1000000000 }                   # password-only
// { "user": "bob", "webauthn_challenge_response": {...}, "pub_key": "key to sign", "ttl": 1000000000 } # mfa
//
// # Success response
//
// { "cert": "base64 encoded signed cert", "host_signers": [{"domain_name": "example.com", "checking_keys": ["base64 encoded public signing key"]}] }
func (h *Handler) mfaLoginFinish(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req *client.AuthenticateSSHUserRequest
	if err := httplib.ReadResourceJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	clientMeta := clientMetaFromReq(r)
	cert, err := h.auth.AuthenticateSSHUser(r.Context(), *req, clientMeta)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return cert, nil
}

// mfaLoginFinishSession completes the MFA login ceremony, returning a new web
// session if successful.
//
// POST /webapi/mfa/login/finishsession
//
// {"user": "alex", "webauthn_challenge_response": {...}}
//
// Successful response:
//
// {"type": "bearer", "token": "bearer token", "user": {"name": "alex", "allowed_logins": ["admin", "bob"]}, "expires_in": 20}
func (h *Handler) mfaLoginFinishSession(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	req := &client.AuthenticateWebUserRequest{}
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	clientMeta := clientMetaFromReq(r)
	session, err := h.auth.AuthenticateWebUser(r.Context(), req, clientMeta)
	switch {
	// Since checking for private key policy meant that they passed authn,
	// return policy error as is to help direct user.
	case keys.IsPrivateKeyPolicyError(err):
		return nil, trace.Wrap(err)

	// Return a friendlier error if an SSO user tried to do passwordless.
	case errors.Is(err, types.ErrPassswordlessLoginBySSOUser):
		return nil, trace.Wrap(err)

	// Obscure all other errors.
	case err != nil:
		// log the actual error.
		h.logger.WarnContext(r.Context(), "Login attempt denied for user", "user", req.User, "error", err)
		return nil, trace.AccessDenied("invalid credentials")
	}

	// Fetch user from session, user is empty for passwordless requests.
	user := session.GetUser()
	if err := websession.SetCookie(w, user, session.GetName()); err != nil {
		return nil, trace.Wrap(err)
	}

	ctx, err := h.auth.newSessionContextFromSession(r.Context(), session)
	if err != nil {
		return nil, trace.AccessDenied("need auth")
	}

	res, err := newSessionResponse(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// request that the browser starts a new device-bound session
	// TODO(zmb3): support both RS256 and ES256
	// TODO(zmb3): determine whether we should pas the authorization attribute
	h.logger.InfoContext(r.Context(), "requesting dbsc flow")
	w.Header().Add(websession.SecureSessionRegistrationHeader,
		`(RS256 ES256);challenge="kbvy6czlka";path="StartSession"`)
	//fmt.Sprintf(`(ES256);challenge="nonce";path="/webapi/securesession/startsession";authorization=%v`, res.Token))

	return res, nil
}

// getClusters returns a list of cluster and its data.
//
// GET /v1/webapi/sites
//
// Successful response:
//
// {"sites": {"name": "localhost", "last_connected": "RFC3339 time", "status": "active"}}
func (h *Handler) getClusters(w http.ResponseWriter, r *http.Request, p httprouter.Params, c *SessionContext) (interface{}, error) {
	// Get a client to the Auth Server with the logged in users identity. The
	// identity of the logged in user is used to fetch the list of nodes.
	clt, err := c.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	remoteClusters, err := clt.GetRemoteClusters(r.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clusterName, err := clt.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	rc, err := types.NewRemoteCluster(clusterName.GetClusterName())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	rc.SetLastHeartbeat(time.Now().UTC())
	rc.SetConnectionStatus(teleport.RemoteClusterStatusOnline)
	clusters := make([]types.RemoteCluster, 0, len(remoteClusters)+1)
	clusters = append(clusters, rc)
	clusters = append(clusters, remoteClusters...)
	out, err := ui.NewClustersFromRemote(clusters)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil
}

type getClusterInfoResponse struct {
	ui.Cluster
	IsCloud bool `json:"isCloud"`
}

// getClusterInfo returns the information about the cluster in the :site param
func (h *Handler) getClusterInfo(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (interface{}, error) {
	ctx := r.Context()
	clusterDetails, err := ui.GetClusterDetails(ctx, site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := sctx.GetUserClient(ctx, site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pingResp, err := clt.Ping(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return getClusterInfoResponse{
		Cluster: *clusterDetails,
		IsCloud: pingResp.GetServerFeatures().Cloud,
	}, nil
}

type getSiteNamespacesResponse struct {
	Namespaces []types.Namespace `json:"namespaces"`
}

// getSiteNamespaces returns a list of namespaces for a given site
//
// GET /v1/webapi/sites/:site/namespaces
//
// Successful response:
//
// {"namespaces": [{..namespace resource...}]}
func (h *Handler) getSiteNamespaces(w http.ResponseWriter, r *http.Request, _ httprouter.Params, c *SessionContext, site reversetunnelclient.RemoteSite) (interface{}, error) {
	return getSiteNamespacesResponse{
		Namespaces: []types.Namespace{types.DefaultNamespace()},
	}, nil
}

func makeUnifiedResourceRequest(r *http.Request) (*proto.ListUnifiedResourcesRequest, error) {
	values := r.URL.Query()

	limit, err := QueryLimitAsInt32(values, "limit", defaults.MaxIterationLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sortBy := types.GetSortByFromString(values.Get("sort"))

	var kinds []string
	for _, kind := range values["kinds"] {
		if kind != "" {
			kinds = append(kinds, kind)
		}
	}

	// set default kinds to be requested if none exist in the request
	if len(kinds) == 0 {
		kinds = []string{
			types.KindApp,
			types.KindDatabase,
			types.KindNode,
			types.KindWindowsDesktop,
			types.KindKubernetesCluster,
			types.KindSAMLIdPServiceProvider,
			types.KindGitServer,
		}
	}

	startKey := values.Get("startKey")
	includeRequestable := values.Get("includedResourceMode") == IncludedResourceModeAll
	useSearchAsRoles := values.Get("searchAsRoles") == "yes" || values.Get("includedResourceMode") == IncludedResourceModeRequestable

	return &proto.ListUnifiedResourcesRequest{
		Kinds:               kinds,
		Limit:               limit,
		StartKey:            startKey,
		SortBy:              sortBy,
		PinnedOnly:          values.Get("pinnedOnly") == "true",
		PredicateExpression: values.Get("query"),
		SearchKeywords:      client.ParseSearchKeywords(values.Get("search"), ' '),
		UseSearchAsRoles:    useSearchAsRoles,
		IncludeLogins:       true,
		IncludeRequestable:  includeRequestable,
	}, nil
}

type loginGetter interface {
	GetAllowedLoginsForResource(resource services.AccessCheckable) ([]string, error)
}

// calculateSSHLogins returns the subset of the allowedLogins that exist in
// the principals of the identity. This is required because SSH authorization
// only allows using a login that exists in the certificates valid principals.
// When connecting to servers in a leaf cluster, the root certificate is used,
// so we need to ensure that we only present the allowed logins that would
// result in a successful connection, if any exists.
func calculateSSHLogins(identity *tlsca.Identity, allowedLogins []string) ([]string, error) {
	allowed := make(map[string]struct{})
	for _, login := range allowedLogins {
		allowed[login] = struct{}{}
	}

	var logins []string
	for _, local := range identity.Principals {
		if _, ok := allowed[local]; ok {
			logins = append(logins, local)
		}
	}

	slices.Sort(logins)
	return logins, nil
}

// calculateAppLogins determines the app logins allowed for the provided
// resource.
//
// TODO(gabrielcorado): DELETE IN V18.0.0
// This is here for backward compatibility in case the auth server
// does not support enriched resources yet.
func calculateAppLogins(loginGetter loginGetter, r types.AppServer, allowedLogins []string) ([]string, error) {
	if len(allowedLogins) > 0 {
		return allowedLogins, nil
	}

	logins, err := loginGetter.GetAllowedLoginsForResource(r.GetApp())
	return logins, trace.Wrap(err)
}

// getUserGroupLookup is a generator to retrieve UserGroupLookup on first call and return it again in subsequent calls.
// If we encounter an error, we log it once and return an empty UserGroupLookup for the current and subsequent calls.
// The returned function is not thread safe.
func (h *Handler) getUserGroupLookup(ctx context.Context, clt apiclient.GetResourcesClient) func() map[string]types.UserGroup {
	userGroupLookup := make(map[string]types.UserGroup)
	var gotUserGroupLookup bool
	return func() map[string]types.UserGroup {
		if gotUserGroupLookup {
			return userGroupLookup
		}

		userGroups, err := apiclient.GetAllResources[types.UserGroup](ctx, clt, &proto.ListResourcesRequest{
			ResourceType:     types.KindUserGroup,
			Namespace:        apidefaults.Namespace,
			UseSearchAsRoles: true,
		})
		if err != nil {
			h.logger.InfoContext(ctx, "Unable to fetch user groups while listing applications, unable to display associated user groups", "error", err)
		}

		for _, userGroup := range userGroups {
			userGroupLookup[userGroup.GetName()] = userGroup
		}

		gotUserGroupLookup = true
		return userGroupLookup
	}
}

// clusterUnifiedResourcesGet returns a list of resources for a given cluster site. This includes all resources available to be displayed in the web ui
// such as Nodes, Apps, Desktops, etc etc
func (h *Handler) clusterUnifiedResourcesGet(w http.ResponseWriter, request *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (interface{}, error) {
	clt, err := sctx.GetUserClient(request.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	identity, err := sctx.GetIdentity()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	req, err := makeUnifiedResourceRequest(request)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	page, next, err := apiclient.GetUnifiedResourcePage(request.Context(), clt, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessChecker, err := sctx.GetUserAccessChecker()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	getUserGroupLookup := h.getUserGroupLookup(request.Context(), clt)

	unifiedResources := make([]any, 0, len(page))
	for _, enriched := range page {
		switch r := enriched.ResourceWithLabels.(type) {
		case types.Server:
			switch enriched.GetKind() {
			case types.KindNode:
				logins, err := calculateSSHLogins(identity, enriched.Logins)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				unifiedResources = append(unifiedResources, ui.MakeServer(site.GetName(), r, logins, enriched.RequiresRequest))
			case types.KindGitServer:
				unifiedResources = append(unifiedResources, ui.MakeGitServer(site.GetName(), r, enriched.RequiresRequest))
			}
		case types.DatabaseServer:
			db := ui.MakeDatabase(r.GetDatabase(), accessChecker, h.cfg.DatabaseREPLRegistry, enriched.RequiresRequest)
			unifiedResources = append(unifiedResources, db)
		case types.AppServer:
			allowedAWSRoles, err := calculateAppLogins(accessChecker, r, enriched.Logins)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			allowedAWSRolesLookup := map[string][]string{
				r.GetApp().GetName(): allowedAWSRoles,
			}

			app := ui.MakeApp(r.GetApp(), ui.MakeAppsConfig{
				LocalClusterName:      h.auth.clusterName,
				LocalProxyDNSName:     h.proxyDNSName(),
				AppClusterName:        site.GetName(),
				AllowedAWSRolesLookup: allowedAWSRolesLookup,
				UserGroupLookup:       getUserGroupLookup(),
				Logger:                h.logger,
				RequiresRequest:       enriched.RequiresRequest,
			})
			unifiedResources = append(unifiedResources, app)
		case types.AppServerOrSAMLIdPServiceProvider:
			//nolint:staticcheck // SA1019. TODO(sshah) DELETE IN 17.0
			if r.IsAppServer() {
				allowedAWSRoles, err := accessChecker.GetAllowedLoginsForResource(r.GetAppServer().GetApp())
				if err != nil {
					return nil, trace.Wrap(err)
				}
				allowedAWSRolesLookup := map[string][]string{
					r.GetAppServer().GetApp().GetName(): allowedAWSRoles,
				}
				app := ui.MakeApp(r.GetAppServer().GetApp(), ui.MakeAppsConfig{
					LocalClusterName:      h.auth.clusterName,
					LocalProxyDNSName:     h.proxyDNSName(),
					AppClusterName:        site.GetName(),
					AllowedAWSRolesLookup: allowedAWSRolesLookup,
					UserGroupLookup:       getUserGroupLookup(),
					Logger:                h.logger,
					RequiresRequest:       enriched.RequiresRequest,
				})
				unifiedResources = append(unifiedResources, app)
			} else {
				app := ui.MakeAppTypeFromSAMLApp(r.GetSAMLIdPServiceProvider(), ui.MakeAppsConfig{
					LocalClusterName:  h.auth.clusterName,
					LocalProxyDNSName: h.proxyDNSName(),
					AppClusterName:    site.GetName(),
					RequiresRequest:   enriched.RequiresRequest,
				})
				unifiedResources = append(unifiedResources, app)
			}
		case types.SAMLIdPServiceProvider:
			// SAMLIdPServiceProvider resources are shown as
			// "apps" in the UI.
			app := ui.MakeAppTypeFromSAMLApp(r, ui.MakeAppsConfig{
				LocalClusterName:  h.auth.clusterName,
				LocalProxyDNSName: h.proxyDNSName(),
				AppClusterName:    site.GetName(),
				RequiresRequest:   enriched.RequiresRequest,
			})
			unifiedResources = append(unifiedResources, app)
		case types.WindowsDesktop:
			unifiedResources = append(unifiedResources, ui.MakeDesktop(r, enriched.Logins, enriched.RequiresRequest))
		case types.KubeCluster:
			kube := ui.MakeKubeCluster(r, accessChecker, enriched.RequiresRequest)
			unifiedResources = append(unifiedResources, kube)
		case types.KubeServer:
			kube := ui.MakeKubeCluster(r.GetCluster(), accessChecker, enriched.RequiresRequest)
			unifiedResources = append(unifiedResources, kube)
		default:
			return nil, trace.Errorf("UI Resource has unknown type: %T", enriched)
		}
	}

	resp := listResourcesGetResponse{
		Items:    unifiedResources,
		StartKey: next,
	}

	return resp, nil
}

// clusterNodesGet returns a list of nodes for a given cluster site.
func (h *Handler) clusterNodesGet(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (interface{}, error) {
	// Get a client to the Auth Server with the logged in user's identity. The
	// identity of the logged in user is used to fetch the list of nodes.
	clt, err := sctx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	req, err := convertListResourcesRequest(r, types.KindNode)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req.IncludeLogins = true

	page, err := apiclient.GetEnrichedResourcePage(r.Context(), clt, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	identity, err := sctx.GetIdentity()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	uiServers := make([]ui.Server, 0, len(page.Resources))
	for _, resource := range page.Resources {
		server, ok := resource.ResourceWithLabels.(types.Server)
		if !ok {
			continue
		}

		logins, err := calculateSSHLogins(identity, resource.Logins)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		uiServers = append(uiServers, ui.MakeServer(site.GetName(), server, logins, false /* requiresRequest */))
	}

	return listResourcesGetResponse{
		Items:      uiServers,
		StartKey:   page.NextKey,
		TotalCount: page.Total,
	}, nil
}

// iso8601MilliFormat is the time format of dates returned from the frontend using Date().
const iso8601MilliFormat = "2006-01-02T15:04:05.000Z0700"

// notificationsGet returns a paginated list of notifications for a user.
func (h *Handler) notificationsGet(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (interface{}, error) {
	clt, err := sctx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	values := r.URL.Query()

	limit, err := QueryLimitAsInt32(values, "limit", defaults.MaxIterationLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	startKey := values.Get("startKey")

	response, err := clt.NotificationServiceClient().ListNotifications(r.Context(), &notificationsv1.ListNotificationsRequest{
		PageSize:  limit,
		PageToken: startKey,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var uiNotifications []ui.Notification

	for _, notification := range response.Notifications {
		uiNotif := ui.MakeNotification(notification)
		uiNotifications = append(uiNotifications, uiNotif)
	}

	return GetNotificationsResponse{
		Notifications:            uiNotifications,
		NextKey:                  response.NextPageToken,
		UserLastSeenNotification: response.UserLastSeenNotificationTimestamp.AsTime().Format(iso8601MilliFormat),
	}, nil
}

type GetNotificationsResponse struct {
	Notifications            []ui.Notification `json:"notifications"`
	NextKey                  string            `json:"nextKey"`
	UserLastSeenNotification string            `json:"userLastSeenNotification"`
}

// notificationsUpsertLastSeenTimestamp upserts a user's last seen notification timestamp.
func (h *Handler) notificationsUpsertLastSeenTimestamp(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (interface{}, error) {
	clt, err := sctx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var req *UpsertUserLastSeenNotificationRequest
	if err := httplib.ReadResourceJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	lastSeenTime, err := time.Parse(iso8601MilliFormat, req.Time)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := clt.NotificationServiceClient().UpsertUserLastSeenNotification(r.Context(), &notificationsv1.UpsertUserLastSeenNotificationRequest{
		Username: sctx.GetUser(),
		UserLastSeenNotification: &notificationsv1.UserLastSeenNotification{
			Status: &notificationsv1.UserLastSeenNotificationStatus{
				LastSeenTime: timestamppb.New(lastSeenTime),
			},
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &UpsertUserLastSeenNotificationRequest{
		Time: resp.Status.LastSeenTime.AsTime().Format(iso8601MilliFormat),
	}, nil
}

type UpsertUserLastSeenNotificationRequest struct {
	Time string `json:"time"`
}

// notificationsUpsertNotificationState upserts a user notification state.
func (h *Handler) notificationsUpsertNotificationState(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (interface{}, error) {
	clt, err := sctx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var req *upsertUserNotificationStateRequest
	if err := httplib.ReadResourceJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := clt.NotificationServiceClient().UpsertUserNotificationState(r.Context(), &notificationsv1.UpsertUserNotificationStateRequest{
		Username: sctx.GetUser(),
		UserNotificationState: &notificationsv1.UserNotificationState{
			Spec: &notificationsv1.UserNotificationStateSpec{
				NotificationId: req.NotificationId,
			},
			Status: &notificationsv1.UserNotificationStateStatus{
				NotificationState: req.NotificationState,
			},
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &upsertUserNotificationStateRequest{
		NotificationId:    resp.GetSpec().GetNotificationId(),
		NotificationState: resp.GetStatus().GetNotificationState(),
	}, nil
}

type upsertUserNotificationStateRequest struct {
	NotificationId    string                            `json:"notificationId"`
	NotificationState notificationsv1.NotificationState `json:"notificationState"`
}

type getLoginAlertsResponse struct {
	Alerts []types.ClusterAlert `json:"alerts"`
}

// clusterLoginAlertsGet returns a list of on-login alerts for the user.
func (h *Handler) clusterLoginAlertsGet(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (interface{}, error) {
	// Get a client to the Auth Server with the logged in user's identity. The
	// identity of the logged in user is used to fetch the list of alerts.
	clt, err := sctx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	alerts, err := clt.GetClusterAlerts(h.cfg.Context, types.GetClusterAlertsRequest{
		Labels: map[string]string{
			types.AlertOnLogin: "yes",
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return getLoginAlertsResponse{
		Alerts: alerts,
	}, nil
}

func (h *Handler) getClusterLocks(
	w http.ResponseWriter,
	r *http.Request,
	p httprouter.Params,
	sessionCtx *SessionContext,
	site reversetunnelclient.RemoteSite,
) (interface{}, error) {
	ctx := r.Context()
	clt, err := sessionCtx.GetUserClient(ctx, site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	locks, err := clt.GetLocks(ctx, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.MakeLocks(locks), nil
}

type createLockReq struct {
	Targets types.LockTarget `json:"targets"`
	Message string           `json:"message"`
	TTL     string           `json:"ttl"`
}

func (h *Handler) createClusterLock(
	w http.ResponseWriter,
	r *http.Request,
	p httprouter.Params,
	sessionCtx *SessionContext,
	site reversetunnelclient.RemoteSite,
) (interface{}, error) {
	var req *createLockReq
	if err := httplib.ReadResourceJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	ctx := r.Context()
	clt, err := sessionCtx.GetUserClient(ctx, site)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var ttlDuration time.Duration
	if req.TTL != "" {
		ttlDuration, err = time.ParseDuration(req.TTL)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	var expires *time.Time
	if ttlDuration != 0 {
		t := time.Now().UTC().Add(ttlDuration)
		expires = &t
	}

	lock, err := types.NewLock(uuid.New().String(), types.LockSpecV2{
		Target:  req.Targets,
		Message: req.Message,
		Expires: expires,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = clt.UpsertLock(ctx, lock)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.MakeLock(lock), nil
}

func (h *Handler) deleteClusterLock(
	w http.ResponseWriter,
	r *http.Request,
	p httprouter.Params,
	sessionCtx *SessionContext,
	site reversetunnelclient.RemoteSite,
) (interface{}, error) {
	ctx := r.Context()
	clt, err := sessionCtx.GetUserClient(ctx, site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = clt.DeleteLock(ctx, p.ByName("uuid"))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return OK(), nil
}

// SessionController restricts creation of sessions based on
// cluster session control configuration(locks, connection limits, etc).
type SessionController interface {
	// AcquireSessionContext attempts to create a context for the session. If the session is
	// not allowed due to session control an error is returned. The returned
	// context is scoped to the session and will be canceled in the event that session
	// controls terminate the session early.
	AcquireSessionContext(ctx context.Context, sctx *SessionContext, login, localAddr, remoteAddr string) (context.Context, error)
}

// SessionControllerFunc type is an adapter to allow the use of
// ordinary functions a [SessionController]. If f is a function
// with the appropriate signature, SessionControllerFunc(f) is a
// SessionController that calls f.
type SessionControllerFunc func(ctx context.Context, sctx *SessionContext, login, localAddr, remoteAddr string) (context.Context, error)

// AcquireSessionContext calls f(ctx, sctx, localAddr, remoteAddr).
func (f SessionControllerFunc) AcquireSessionContext(ctx context.Context, sctx *SessionContext, login, localAddr, remoteAddr string) (context.Context, error) {
	ctx, err := f(ctx, sctx, login, localAddr, remoteAddr)
	return ctx, trace.Wrap(err)
}

// siteNodeConnect connect to the site node
//
// GET /v1/webapi/sites/:site/namespaces/:namespace/connect?access_token=bearer_token&params=<urlencoded json-structure>
//
// Due to the nature of websocket we can't POST parameters as is, so we have
// to add query parameters. The params query parameter is a URL-encoded JSON structure:
//
// {"server_id": "uuid", "login": "admin", "term": {"h": 120, "w": 100}, "sid": "123"}
//
// Successful response is a websocket stream that allows read write to the server
func (h *Handler) siteNodeConnect(
	w http.ResponseWriter,
	r *http.Request,
	p httprouter.Params,
	sessionCtx *SessionContext,
	site reversetunnelclient.RemoteSite,
	ws *websocket.Conn,
) (interface{}, error) {
	q := r.URL.Query()
	params := q.Get("params")
	if params == "" {
		return nil, trace.BadParameter("missing params")
	}
	var req TerminalRequest
	if err := json.Unmarshal([]byte(params), &req); err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := sessionCtx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ctx, err := h.cfg.SessionControl.AcquireSessionContext(r.Context(), sessionCtx, req.Login, h.cfg.ProxyWebAddr.Addr, r.RemoteAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var (
		sessionData  session.Session
		displayLogin string
		tracker      types.SessionTracker
	)

	clusterName := site.GetName()
	if req.SessionID.IsZero() {
		// An existing session ID was not provided so we need to create a new one.
		sessionData, err = h.generateSession(r.Context(), &req, clusterName, sessionCtx)
		if err != nil {
			h.logger.DebugContext(r.Context(), "Unable to generate new ssh session", "error", err)
			return nil, trace.Wrap(err)
		}
	} else {
		sessionData, tracker, err = h.fetchExistingSession(ctx, clt, &req, clusterName)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		displayLogin = tracker.GetLogin()
	}

	// If the participantMode is not specified, and the user is the one who created the session,
	// they should be in Peer mode. If not, default to Observer mode.
	if req.ParticipantMode == "" {
		if sessionData.Owner == sessionCtx.cfg.User {
			req.ParticipantMode = types.SessionPeerMode
		} else {
			req.ParticipantMode = types.SessionObserverMode
		}
	}

	h.logger.DebugContext(r.Context(), "New terminal request",
		"server", req.Server,
		"login", req.Login,
		"sid", sessionData.ID,
		"websid", sessionCtx.GetSessionID(),
	)

	authAccessPoint, err := site.CachingAccessPoint()
	if err != nil {
		h.logger.DebugContext(r.Context(), "Unable to get auth access point", "error", err)
		return nil, trace.Wrap(err)
	}

	dialTimeout := apidefaults.DefaultIOTimeout
	keepAliveInterval := apidefaults.KeepAliveInterval()
	if netConfig, err := authAccessPoint.GetClusterNetworkingConfig(ctx); err != nil {
		h.logger.DebugContext(r.Context(), "Unable to fetch cluster networking config", "error", err)
	} else {
		dialTimeout = netConfig.GetSSHDialTimeout()
		keepAliveInterval = netConfig.GetKeepAliveInterval()
	}

	// Try to use the keep alive interval from the request.
	// When it's not set or below a second, use the cluster's keep alive interval.
	if req.KeepAliveInterval >= time.Second {
		keepAliveInterval = req.KeepAliveInterval
	}

	nw, err := site.NodeWatcher()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	term, err := NewTerminal(ctx, TerminalHandlerConfig{
		Logger:             h.logger,
		Term:               req.Term,
		SessionCtx:         sessionCtx,
		UserAuthClient:     clt,
		LocalAccessPoint:   h.auth.accessPoint,
		DisplayLogin:       displayLogin,
		SessionData:        sessionData,
		KeepAliveInterval:  keepAliveInterval,
		ProxyHostPort:      h.ProxyHostPort(),
		ProxyPublicAddr:    h.PublicProxyAddr(),
		InteractiveCommand: req.InteractiveCommand,
		Router:             h.cfg.Router,
		TracerProvider:     h.cfg.TracerProvider,
		ParticipantMode:    req.ParticipantMode,
		PROXYSigner:        h.cfg.PROXYSigner,
		Tracker:            tracker,
		PresenceChecker:    h.cfg.PresenceChecker,
		WebsocketConn:      ws,
		SSHDialTimeout:     dialTimeout,
		HostNameResolver: func(serverID string) (string, error) {
			matches, err := nw.CurrentResourcesWithFilter(r.Context(), func(n readonly.Server) bool {
				return n.GetName() == serverID
			})
			if err != nil {
				return "", trace.Wrap(err)
			}

			if len(matches) != 1 {
				return "", trace.NotFound("unable to resolve hostname for server %s", serverID)
			}

			return matches[0].GetHostname(), nil
		},
	})
	if err != nil {
		h.logger.ErrorContext(r.Context(), "Unable to create terminal", "error", err)
		return nil, trace.Wrap(err)
	}

	h.userConns.Add(1)
	defer h.userConns.Add(-1)

	// start the websocket session with a web-based terminal:
	httplib.MakeTracingHandler(term, teleport.ComponentProxy).ServeHTTP(w, r)

	return nil, nil
}

func (h *Handler) setDefaultConnectorHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *SessionContext) (interface{}, error) {
	var req ui.SetDefaultAuthConnectorRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	authPref, err := clt.GetAuthPreference(r.Context())
	if err != nil {
		return nil, trace.Wrap(err, "failed to get auth preference")
	}

	authPref.SetConnectorName(req.Name)
	authPref.SetType(req.Type)

	_, err = clt.UpsertAuthPreference(r.Context(), authPref)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return OK(), nil
}

type podConnectParams struct {
	// Term is the initial PTY size.
	Term session.TerminalParams `json:"term"`
}

func (h *Handler) podConnect(
	w http.ResponseWriter,
	r *http.Request,
	p httprouter.Params,
	sctx *SessionContext,
	site reversetunnelclient.RemoteSite,
	ws *websocket.Conn,
) (interface{}, error) {
	q := r.URL.Query()
	if q.Get("params") == "" {
		return nil, trace.BadParameter("missing params")
	}
	var params podConnectParams
	if err := json.Unmarshal([]byte(q.Get("params")), &params); err != nil {
		return nil, trace.Wrap(err)
	}

	execReq, err := readPodExecRequestFromWS(ws)
	if err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) || terminal.IsOKWebsocketCloseError(trace.Unwrap(err)) {
			return nil, nil
		}
		var netError net.Error
		if errors.As(trace.Unwrap(err), &netError) && netError.Timeout() {
			return nil, trace.BadParameter("timed out waiting for pod exec request data on websocket connection")
		}

		return nil, trace.Wrap(err)
	}
	execReq.Term = params.Term

	if err := execReq.Validate(); err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := sctx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clusterName := site.GetName()

	accessChecker, err := sctx.GetUserAccessChecker()
	if err != nil {
		return session.Session{}, trace.Wrap(err)
	}
	policySets := accessChecker.SessionPolicySets()
	accessEvaluator := auth.NewSessionAccessEvaluator(policySets, types.KubernetesSessionKind, sctx.GetUser())

	sess := session.Session{
		Kind:                  types.KubernetesSessionKind,
		Login:                 "root",
		ClusterName:           clusterName,
		KubernetesClusterName: execReq.KubeCluster,
		Moderated:             accessEvaluator.IsModerated(),
		ID:                    session.NewID(),
		Created:               h.clock.Now().UTC(),
		LastActive:            h.clock.Now().UTC(),
		Namespace:             apidefaults.Namespace,
		Owner:                 sctx.GetUser(),
		Command:               execReq.Command,
	}

	h.logger.DebugContext(r.Context(), "New kube exec request",
		"namespace", execReq.Namespace,
		"pod", execReq.Pod,
		"container", execReq.Container,
		"sid", sess.ID,
		"websid", sctx.GetSessionID(),
	)

	authAccessPoint, err := site.CachingAccessPoint()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	netConfig, err := authAccessPoint.GetClusterNetworkingConfig(r.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	keepAliveInterval := netConfig.GetKeepAliveInterval()

	serverAddr, tlsServerName, err := h.getKubeExecClusterData(netConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	hostCA, err := h.auth.accessPoint.GetCertAuthority(r.Context(), types.CertAuthID{
		Type:       types.HostCA,
		DomainName: h.auth.clusterName,
	}, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ph := podHandler{
		req:                 execReq,
		sess:                sess,
		sctx:                sctx,
		teleportCluster:     site.GetName(),
		ws:                  ws,
		keepAliveInterval:   keepAliveInterval,
		logger:              h.logger.With(teleport.ComponentKey, "pod"),
		userClient:          clt,
		localCA:             hostCA,
		configServerAddr:    serverAddr,
		configTLSServerName: tlsServerName,
	}

	ph.ServeHTTP(w, r)
	return nil, nil
}

// KubeExecDataWaitTimeout is how long server would wait for user to send pod exec data (namespace, pod name etc)
// on websocket connection, after user initiated the exec into pod flow.
const KubeExecDataWaitTimeout = defaults.HeadlessLoginTimeout

func readPodExecRequestFromWS(ws *websocket.Conn) (*PodExecRequest, error) {
	err := ws.SetReadDeadline(time.Now().Add(KubeExecDataWaitTimeout))
	if err != nil {
		return nil, trace.Wrap(err, "failed to set read deadline for websocket connection")
	}

	messageType, bytes, err := ws.ReadMessage()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := ws.SetReadDeadline(time.Time{}); err != nil {
		return nil, trace.Wrap(err, "failed to set read deadline for websocket connection")
	}

	if messageType != websocket.BinaryMessage {
		return nil, trace.BadParameter("Expected binary message of type websocket.BinaryMessage, got %v", messageType)
	}

	var envelope terminal.Envelope
	if err := gogoproto.Unmarshal(bytes, &envelope); err != nil {
		return nil, trace.BadParameter("Failed to parse envelope: %v", err)
	}

	var req PodExecRequest
	if err := json.Unmarshal([]byte(envelope.Payload), &req); err != nil {
		return nil, trace.Wrap(err)
	}

	return &req, nil
}

func (h *Handler) getKubeExecClusterData(netConfig types.ClusterNetworkingConfig) (string, string, error) {
	if netConfig.GetProxyListenerMode() == types.ProxyListenerMode_Separate {
		return "https://" + h.kubeProxyHostPort(), "", nil
	}

	proxyAddr := createHostPort(h.cfg.ProxyWebAddr, defaults.HTTPListenPort)
	host, port, err := utils.SplitHostPort(proxyAddr)
	if err != nil {
		return "", "", trace.Wrap(err, "failed to split proxy address %q", proxyAddr)
	}

	tlsServerName := client.GetKubeTLSServerName(host)

	return "https://" + net.JoinHostPort(host, port), tlsServerName, nil
}

func (h *Handler) generateSession(ctx context.Context, req *TerminalRequest, clusterName string, scx *SessionContext) (session.Session, error) {
	owner := scx.cfg.User
	h.logger.InfoContext(ctx, "Generating new session", "cluster", clusterName)

	host, port, err := serverHostPort(req.Server)
	if err != nil {
		return session.Session{}, trace.Wrap(err)
	}

	accessChecker, err := scx.GetUserAccessChecker()
	if err != nil {
		return session.Session{}, trace.Wrap(err)
	}
	policySets := accessChecker.SessionPolicySets()
	accessEvaluator := auth.NewSessionAccessEvaluator(policySets, types.SSHSessionKind, owner)

	return session.Session{
		Kind:           types.SSHSessionKind,
		Login:          req.Login,
		ServerID:       host,
		ClusterName:    clusterName,
		ServerHostname: host,
		ServerHostPort: port,
		Moderated:      accessEvaluator.IsModerated(),
		ID:             session.NewID(),
		Created:        time.Now().UTC(),
		LastActive:     time.Now().UTC(),
		Namespace:      apidefaults.Namespace,
		Owner:          owner,
	}, nil
}

// fetchExistingSession fetches an active or pending SSH session by the SessionID passed in the TerminalRequest.
func (h *Handler) fetchExistingSession(ctx context.Context, clt authclient.ClientI, req *TerminalRequest, siteName string) (session.Session, types.SessionTracker, error) {
	sessionID, err := session.ParseID(req.SessionID.String())
	if err != nil {
		return session.Session{}, nil, trace.Wrap(err)
	}
	h.logger.InfoContext(ctx, "Attempting to join existing session", "session_id", sessionID)

	tracker, err := clt.GetSessionTracker(ctx, string(*sessionID))
	if err != nil {
		return session.Session{}, nil, trace.Wrap(err)
	}

	if tracker.GetSessionKind() != types.SSHSessionKind || tracker.GetState() == types.SessionState_SessionStateTerminated {
		return session.Session{}, nil, trace.NotFound("SSH session %v not found", sessionID)
	}

	sessionData := trackerToLegacySession(tracker, siteName)
	// When joining an existing session use the specially handled
	// `SSHSessionJoinPrincipal` login instead of the provided login so that
	// users are able to join sessions without having permissions to create
	// new ones themselves for auditing purposes. Otherwise, the user would
	// fail the SSH lib username validation step.
	sessionData.Login = teleport.SSHSessionJoinPrincipal

	return sessionData, tracker, nil
}

type siteSessionGenerateResponse struct {
	Session session.Session `json:"session"`
}

type siteSessionsGetResponse struct {
	Sessions []siteSessionsGetResponseSession `json:"sessions"`
}

type siteSessionsGetResponseSession struct {
	session.Session
	ParticipantModes []types.SessionParticipantMode `json:"participantModes"`
}

func trackerToLegacySession(tracker types.SessionTracker, clusterName string) session.Session {
	participants := tracker.GetParticipants()
	parties := make([]session.Party, 0, len(participants))

	for _, participant := range participants {
		parties = append(parties, session.Party{
			ID:         session.ID(participant.ID),
			User:       participant.User,
			ServerID:   tracker.GetAddress(),
			LastActive: participant.LastActive,
			// note: we don't populate the RemoteAddr field since it isn't used and we don't have an equivalent value
		})
	}
	accessEvaluator := auth.NewSessionAccessEvaluator(tracker.GetHostPolicySets(), types.SSHSessionKind, tracker.GetHostUser())

	return session.Session{
		Kind:      tracker.GetSessionKind(),
		ID:        session.ID(tracker.GetSessionID()),
		Namespace: apidefaults.Namespace,
		Parties:   parties,
		TerminalParams: session.TerminalParams{
			W: teleport.DefaultTerminalWidth,
			H: teleport.DefaultTerminalHeight,
		},
		Login:                 tracker.GetLogin(),
		Created:               tracker.GetCreated(),
		LastActive:            tracker.GetLastActive(),
		ServerID:              tracker.GetAddress(),
		ServerHostname:        tracker.GetHostname(),
		ServerAddr:            tracker.GetAddress(),
		ClusterName:           clusterName,
		KubernetesClusterName: tracker.GetKubeCluster(),
		DesktopName:           tracker.GetDesktopName(),
		AppName:               tracker.GetAppName(),
		Moderated:             accessEvaluator.IsModerated(),
		DatabaseName:          tracker.GetDatabaseName(),
		Owner:                 tracker.GetHostUser(),
		Command:               strings.Join(tracker.GetCommand(), " "),
	}
}

// clusterActiveAndPendingSessionsGet gets the list of active and pending sessions for a site.
//
// GET /v1/webapi/sites/:site/sessions
func (h *Handler) clusterActiveAndPendingSessionsGet(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (interface{}, error) {
	clt, err := sctx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	trackers, err := clt.GetActiveSessionTrackers(r.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	userRoles, err := clt.GetCurrentUserRoles(r.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessContext := auth.SessionAccessContext{
		Username: sctx.GetUser(),
		Roles:    userRoles,
	}

	sessions := make([]siteSessionsGetResponseSession, 0, len(trackers))
	for _, tracker := range trackers {
		if tracker.GetState() != types.SessionState_SessionStateTerminated {
			session := trackerToLegacySession(tracker, p.ByName("site"))
			// Get the participant modes available to the user from their roles.
			accessEvaluator := auth.NewSessionAccessEvaluator(tracker.GetHostPolicySets(), types.SSHSessionKind, tracker.GetHostUser())
			participantModes := accessEvaluator.CanJoin(accessContext)

			sessions = append(sessions, siteSessionsGetResponseSession{Session: session, ParticipantModes: participantModes})
		}
	}

	return siteSessionsGetResponse{Sessions: sessions}, nil
}

func toFieldsSlice(rawEvents []apievents.AuditEvent) ([]events.EventFields, error) {
	el := make([]events.EventFields, 0, len(rawEvents))
	for _, event := range rawEvents {
		els, err := events.ToEventFields(event)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		el = append(el, els)
	}

	return el, nil
}

// clusterSearchEvents returns all audit log events matching the provided criteria
//
// GET /v1/webapi/sites/:site/events/search
//
// Query parameters:
//
//	"from"    : date range from, encoded as RFC3339
//	"to"      : date range to, encoded as RFC3339
//	"limit"   : optional maximum number of events to return on each fetch
//	"startKey": resume events search from the last event received,
//	            empty string means start search from beginning
//	"include" : optional comma-separated list of event names to return e.g.
//	            include=session.start,session.end, all are returned if empty
//	"order":    optional ordering of events. Can be either "asc" or "desc"
//	            for ascending and descending respectively.
//	            If no order is provided it defaults to descending.
func (h *Handler) clusterSearchEvents(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (interface{}, error) {
	values := r.URL.Query()

	var eventTypes []string
	if include := values.Get("include"); include != "" {
		eventTypes = strings.Split(include, ",")
	}

	searchEvents := func(clt authclient.ClientI, from, to time.Time, limit int, order types.EventOrder, startKey string) ([]apievents.AuditEvent, string, error) {
		return clt.SearchEvents(r.Context(), events.SearchEventsRequest{
			From:       from,
			To:         to,
			EventTypes: eventTypes,
			Limit:      limit,
			Order:      order,
			StartKey:   startKey,
		})
	}
	return clusterEventsList(r.Context(), sctx, site, r.URL.Query(), searchEvents)
}

// clusterSearchSessionEvents returns session events matching the criteria.
//
// GET /v1/webapi/sites/:site/sessions/search
//
// Query parameters:
//
//	"from"    : date range from, encoded as RFC3339
//	"to"      : date range to, encoded as RFC3339
//	"limit"   : optional maximum number of events to return on each fetch
//	"startKey": resume events search from the last event received,
//	            empty string means start search from beginning
//	"order":    optional ordering of events. Can be either "asc" or "desc"
//	            for ascending and descending respectively.
//	            If no order is provided it defaults to descending.
func (h *Handler) clusterSearchSessionEvents(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (interface{}, error) {
	searchSessionEvents := func(clt authclient.ClientI, from, to time.Time, limit int, order types.EventOrder, startKey string) ([]apievents.AuditEvent, string, error) {
		return clt.SearchSessionEvents(r.Context(), events.SearchSessionEventsRequest{
			From:     from,
			To:       to,
			Limit:    limit,
			Order:    order,
			StartKey: startKey,
		})
	}
	return clusterEventsList(r.Context(), sctx, site, r.URL.Query(), searchSessionEvents)
}

// clusterEventsList returns a list of audit events obtained using the provided
// searchEvents method.
func clusterEventsList(ctx context.Context, sctx *SessionContext, site reversetunnelclient.RemoteSite, values url.Values, searchEvents func(clt authclient.ClientI, from, to time.Time, limit int, order types.EventOrder, startKey string) ([]apievents.AuditEvent, string, error)) (interface{}, error) {
	from, err := queryTime(values, "from", time.Now().UTC().AddDate(0, -1, 0))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	to, err := queryTime(values, "to", time.Now().UTC())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	limit, err := QueryLimit(values, "limit", defaults.EventsIterationLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	order, err := queryOrder(values, "order", types.EventOrderDescending)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	startKey := values.Get("startKey")

	clt, err := sctx.GetUserClient(ctx, site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rawEvents, lastKey, err := searchEvents(clt, from, to, limit, order, startKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	el, err := toFieldsSlice(rawEvents)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return eventsListGetResponse{Events: el, StartKey: lastKey}, nil
}

// queryTime parses the query string parameter with the specified name as a
// RFC3339 time and returns it.
//
// If there's no such parameter, specified default value is returned.
func queryTime(query url.Values, name string, def time.Time) (time.Time, error) {
	str := query.Get(name)
	if str == "" {
		return def, nil
	}
	parsed, err := time.Parse(time.RFC3339, str)
	if err != nil {
		return time.Time{}, trace.BadParameter("failed to parse %v as RFC3339 time: %v", name, str)
	}
	return parsed, nil
}

// QueryLimit returns the limit parameter with the specified name from the
// query string.
//
// If there's no such parameter, specified default limit is returned.
func QueryLimit(query url.Values, name string, def int) (int, error) {
	str := query.Get(name)
	if str == "" {
		return def, nil
	}
	limit, err := strconv.Atoi(str)
	if err != nil {
		return 0, trace.BadParameter("failed to parse %v as limit: %v", name, str)
	}
	return limit, nil
}

// queryLimitAsInt32 returns the limit parameter with the specified name from the
// query string. Similar to function 'queryLimit' except it returns as type int32.
//
// If there's no such parameter, specified default limit is returned.
func QueryLimitAsInt32(query url.Values, name string, def int32) (int32, error) {
	str := query.Get(name)
	if str == "" {
		return def, nil
	}
	limit, err := strconv.ParseInt(str, 10, 32)
	if err != nil {
		return 0, trace.BadParameter("failed to parse %v as limit: %v", name, str)
	}
	return int32(limit), nil
}

// queryOrder returns the order parameter with the specified name from the
// query string or a default if the parameter is not provided.
func queryOrder(query url.Values, name string, def types.EventOrder) (types.EventOrder, error) {
	value := query.Get(name)
	switch value {
	case "desc":
		return types.EventOrderDescending, nil
	case "asc":
		return types.EventOrderAscending, nil
	case "":
		return def, nil
	default:
		return types.EventOrderAscending, trace.BadParameter("parameter %v is not a valid ordering", value)
	}
}

type eventsListGetResponse struct {
	// Events is list of events retrieved.
	Events []events.EventFields `json:"events"`
	// StartKey is the position to resume search events.
	StartKey string `json:"startKey"`
}

// hostCredentials sends a registration token and metadata to the Auth Server
// and gets back SSH and TLS certificates.
func (h *Handler) hostCredentials(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req types.RegisterUsingTokenRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	authClient := h.cfg.ProxyClient
	remoteAddr, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req.RemoteAddr = remoteAddr
	certs, err := authClient.RegisterUsingToken(r.Context(), &req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return certs, nil
}

// createSSHCert is a web call that generates new SSH certificate based
// on user's name, password, 2nd factor token and public key user wishes to sign
//
// POST /v1/webapi/ssh/certs
//
// { "user": "bob", "password": "pass", "otp_token": "tok", "pub_key": "key to sign", "ttl": 1000000000 }
//
// # Success response
//
// { "cert": "base64 encoded signed cert", "host_signers": [{"domain_name": "example.com", "checking_keys": ["base64 encoded public signing key"]}] }
//
// TODO(Joerger): DELETE IN v18.0.0, deprecated in favor of mfa login endpoints.
func (h *Handler) createSSHCert(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req client.CreateSSHCertReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	authClient := h.cfg.ProxyClient

	cap, err := authClient.GetAuthPreference(r.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authSSHUserReq := authclient.AuthenticateSSHRequest{
		AuthenticateUserRequest: authclient.AuthenticateUserRequest{
			Username:       req.User,
			SSHPublicKey:   req.SSHPubKey,
			TLSPublicKey:   req.TLSPubKey,
			ClientMetadata: clientMetaFromReq(r),
		},
		CompatibilityMode:       req.Compatibility,
		TTL:                     req.TTL,
		RouteToCluster:          req.RouteToCluster,
		KubernetesCluster:       req.KubernetesCluster,
		SSHAttestationStatement: req.SSHAttestationStatement,
		TLSAttestationStatement: req.TLSAttestationStatement,
	}

	if req.HeadlessAuthenticationID != "" {
		// We need to use the default callback timeout rather than the standard client timeout.
		// However, authClient is shared across all Proxy->Auth requests, so we need to create
		// a new client to avoid applying the callback timeout to other concurrent requests. To
		// this end, we create a clone of the HTTP Client with the desired timeout instead.
		httpClient, err := authClient.CloneHTTPClient(
			authclient.ClientParamTimeout(defaults.HeadlessLoginTimeout),
			authclient.ClientParamResponseHeaderTimeout(defaults.HeadlessLoginTimeout),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// HTTP server has shorter WriteTimeout than is needed, so we override WriteDeadline of the connection.
		if conn, err := authz.ConnFromContext(r.Context()); err == nil {
			if err := conn.SetWriteDeadline(h.clock.Now().Add(defaults.HeadlessLoginTimeout)); err != nil {
				return nil, trace.Wrap(err)
			}
		}

		authSSHUserReq.AuthenticateUserRequest.HeadlessAuthenticationID = req.HeadlessAuthenticationID
		loginResp, err := httpClient.AuthenticateSSHUser(r.Context(), authSSHUserReq)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return loginResp, nil
	}

	switch {
	case !cap.IsSecondFactorEnforced():
		authSSHUserReq.AuthenticateUserRequest.Pass = &authclient.PassCreds{
			Password: []byte(req.Password),
		}
	case cap.IsSecondFactorTOTPAllowed():
		authSSHUserReq.AuthenticateUserRequest.OTP = &authclient.OTPCreds{
			Password: []byte(req.Password),
			Token:    req.OTPToken,
		}
	default:
		return nil, trace.AccessDenied("direct login with password+otp not supported by this cluster")
	}

	loginResp, err := authClient.AuthenticateSSHUser(r.Context(), authSSHUserReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return loginResp, nil
}

// headlessLogin is a web call that perform headless login based on a user's
// name, returning new login certs if successful.
//
// POST /v1/webapi/headless/login
//
// { "user": "bob", "pub_key": "key to sign", "ttl": 1000000000 }
//
// # Success response
//
// { "cert": "base64 encoded signed cert", "host_signers": [{"domain_name": "example.com", "checking_keys": ["base64 encoded public signing key"]}] }
func (h *Handler) headlessLogin(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req client.HeadlessLoginReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	authClient := h.cfg.ProxyClient

	authSSHUserReq := authclient.AuthenticateSSHRequest{
		AuthenticateUserRequest: authclient.AuthenticateUserRequest{
			Username:                 req.User,
			SSHPublicKey:             req.SSHPubKey,
			TLSPublicKey:             req.TLSPubKey,
			ClientMetadata:           clientMetaFromReq(r),
			HeadlessAuthenticationID: req.HeadlessAuthenticationID,
		},
		CompatibilityMode:       req.Compatibility,
		TTL:                     req.TTL,
		RouteToCluster:          req.RouteToCluster,
		KubernetesCluster:       req.KubernetesCluster,
		SSHAttestationStatement: req.SSHAttestationStatement,
		TLSAttestationStatement: req.TLSAttestationStatement,
	}

	// We need to use the default callback timeout rather than the standard client timeout.
	// However, authClient is shared across all Proxy->Auth requests, so we need to create
	// a new client to avoid applying the callback timeout to other concurrent requests. To
	// this end, we create a clone of the HTTP Client with the desired timeout instead.
	httpClient, err := authClient.CloneHTTPClient(
		authclient.ClientParamTimeout(defaults.HeadlessLoginTimeout),
		authclient.ClientParamResponseHeaderTimeout(defaults.HeadlessLoginTimeout),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// HTTP server has shorter WriteTimeout than is needed, so we override WriteDeadline of the connection.
	if conn, err := authz.ConnFromContext(r.Context()); err == nil {
		if err := conn.SetWriteDeadline(h.clock.Now().Add(defaults.HeadlessLoginTimeout)); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	loginResp, err := httpClient.AuthenticateSSHUser(r.Context(), authSSHUserReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return loginResp, nil
}

// validateTrustedCluster validates the token for a trusted cluster and returns it's own host and user certificate authority.
//
// POST /webapi/trustedclusters/validate
//
// * Request body:
//
//	{
//	    "token": "foo",
//	    "certificate_authorities": ["AQ==", "Ag=="]
//	}
//
// * Response:
//
//	{
//	    "certificate_authorities": ["AQ==", "Ag=="]
//	}
func (h *Handler) validateTrustedCluster(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var validateRequestRaw authclient.ValidateTrustedClusterRequestRaw
	if err := httplib.ReadJSON(r, &validateRequestRaw); err != nil {
		return nil, trace.Wrap(err)
	}

	validateRequest, err := validateRequestRaw.ToNative()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	validateResponse, err := h.auth.ValidateTrustedCluster(r.Context(), validateRequest)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "Failed validating trusted cluster", "error", err)
		if trace.IsAccessDenied(err) {
			return nil, trace.AccessDenied("access denied: the cluster token has been rejected")
		}
		return nil, trace.Wrap(err)
	}

	validateResponseRaw, err := validateResponse.ToRaw()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return validateResponseRaw, nil
}

func (h *Handler) String() string {
	return "multi site"
}

// currentSiteShortcut is a special shortcut that will return the first
// available site, is helpful when UI works in single site mode to reduce
// the amount of requests
const currentSiteShortcut = "-current-"

// ContextHandler is a handler called with the auth context, what means it is authenticated and ready to work
type ContextHandler func(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext) (interface{}, error)

// ClusterHandler is a authenticated handler that is called for some existing remote cluster
type ClusterHandler func(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (interface{}, error)

// ClusterWebsocketHandler is a authenticated websocket handler that is called for some existing remote cluster
type ClusterWebsocketHandler func(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite, ws *websocket.Conn) (interface{}, error)

// WithClusterAuth wraps a ClusterHandler to ensure that a request is authenticated to this proxy
// (the same as WithAuth), as well as to grab the remoteSite (which can represent this local cluster
// or a remote trusted cluster) as specified by the ":site" url parameter.
//
// WithClusterAuth also provides CSRF protection by requiring the bearer token to be present.
func (h *Handler) WithClusterAuth(fn ClusterHandler) httprouter.Handle {
	return httplib.MakeHandler(func(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
		sctx, site, err := h.authenticateRequestWithCluster(w, r, p)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return fn(w, r, p, sctx, site)
	})
}

func (h *Handler) writeErrToWebSocket(ctx context.Context, ws *websocket.Conn, err error) {
	if err == nil {
		return
	}
	errEnvelope := terminal.Envelope{
		Version: defaults.WebsocketVersion,
		Type:    defaults.WebsocketError,
		Payload: trace.UserMessage(err),
	}
	env, err := errEnvelope.Marshal()
	if err != nil {
		h.logger.ErrorContext(ctx, "error marshaling proto", "error", err)
		return
	}
	if err := ws.WriteMessage(websocket.BinaryMessage, env); err != nil {
		h.logger.ErrorContext(ctx, "error writing proto", "error", err)
		return
	}
}

// authnWsUpgrader is an upgrader that allows any origin to connect to the websocket.
// This makes our lives easier in our automated tests. While ordinarily this would be
// used to enforce the same-origin policy, we don't need to worry about that for authenticated
// websockets, which also require a valid bearer token sent over the websocket after upgrade.
// Therefore even if an attacker were to connect to the websocket and trick the browser into
// sending the session cookie, they would still fail to send the bearer token needed to authenticate.
var authnWsUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
	// We’re disabling the error handler here since error handling is managed within the handler itself.
	// Allowing the WS error handler to operate would result in it writing to the response writer,
	// which conflicts with our custom error handler and leads to unintended errors.
	Error: func(http.ResponseWriter, *http.Request, int, error) {},
}

// WithClusterAuthWebSocket wraps a ClusterWebsocketHandler to ensure that a request is authenticated
// to this proxy via websocket, as well as to grab the remoteSite (which can represent this local
// cluster or a remote trusted cluster) as specified by the ":site" url parameter.
func (h *Handler) WithClusterAuthWebSocket(fn ClusterWebsocketHandler) httprouter.Handle {
	return httplib.MakeHandler(func(w http.ResponseWriter, r *http.Request, p httprouter.Params) (any, error) {
		sctx, ws, site, err := h.authenticateWSRequestWithCluster(w, r, p)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// WS protocol requires the server send a close message
		// which should be done by downstream users
		defer ws.Close()
		if _, err := fn(w, r, p, sctx, site, ws); err != nil {
			h.writeErrToWebSocket(r.Context(), ws, err)
		}
		return nil, nil
	})
}

// authenticateWSRequestWithCluster ensures that a request is
// authenticated to this proxy via websocket, returning the
// *SessionContext (same as AuthenticateRequest), and also grabs the
// remoteSite (which can represent this local cluster or a remote
// trusted cluster) as specified by the ":site" url parameter.
func (h *Handler) authenticateWSRequestWithCluster(w http.ResponseWriter, r *http.Request, p httprouter.Params) (*SessionContext, *websocket.Conn, reversetunnelclient.RemoteSite, error) {
	sctx, ws, err := h.AuthenticateRequestWS(w, r)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}

	site, err := h.getSiteByParams(r.Context(), sctx, p)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}

	return sctx, ws, site, nil
}

// authenticateRequestWithCluster ensures that a request is authenticated
// to this proxy, returning the *SessionContext (same as AuthenticateRequest),
// and also grabs the remoteSite (which can represent this local cluster or a
// remote trusted cluster) as specified by the ":site" url parameter.
func (h *Handler) authenticateRequestWithCluster(w http.ResponseWriter, r *http.Request, p httprouter.Params) (*SessionContext, reversetunnelclient.RemoteSite, error) {
	sctx, err := h.AuthenticateRequest(w, r, true)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	site, err := h.getSiteByParams(r.Context(), sctx, p)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return sctx, site, nil
}

// getSiteByParams gets the remoteSite (which can represent this local cluster or a
// remote trusted cluster) as specified by the ":site" url parameter.
func (h *Handler) getSiteByParams(ctx context.Context, sctx *SessionContext, p httprouter.Params) (reversetunnelclient.RemoteSite, error) {
	clusterName := p.ByName("site")
	site, err := h.getSiteByClusterName(ctx, sctx, clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return site, nil
}

func (h *Handler) getSiteByClusterName(ctx context.Context, sctx *SessionContext, clusterName string) (reversetunnelclient.RemoteSite, error) {
	if clusterName == currentSiteShortcut {
		res, err := h.cfg.ProxyClient.GetClusterName()
		if err != nil {
			h.logger.WarnContext(ctx, "Failed to query cluster name", "error", err)
			return nil, trace.Wrap(err)
		}
		clusterName = res.GetClusterName()
	}

	proxy, err := h.ProxyWithRoles(ctx, sctx)
	if err != nil {
		h.logger.WarnContext(ctx, "Failed to get proxy with roles", "error", err)
		return nil, trace.Wrap(err)
	}

	site, err := proxy.GetSite(clusterName)
	if err != nil {
		h.logger.WarnContext(ctx, "Failed to query site", "error", err, "cluster", clusterName)
		return nil, trace.Wrap(err)
	}

	return site, nil
}

// ClusterClientProvider is an interface for a type which can provide
// authenticated clients to remote clusters.
type ClusterClientProvider interface {
	// UserClientForCluster returns a client to the local or remote cluster
	// identified by clusterName and is authenticated with the identity of the
	// user.
	UserClientForCluster(ctx context.Context, clusterName string) (authclient.ClientI, error)
}

type clusterClientProvider struct {
	h   *Handler
	ctx *SessionContext
}

// UserClientForCluster returns a client to the local or remote cluster
// identified by clusterName and is authenticated with the identity of the user.
func (r clusterClientProvider) UserClientForCluster(ctx context.Context, clusterName string) (authclient.ClientI, error) {
	site, err := r.h.getSiteByClusterName(ctx, r.ctx, clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := r.ctx.GetUserClient(ctx, site)
	return clt, trace.Wrap(err)
}

// ClusterClientHandler is an authenticated handler which can get a client for
// any remote cluster.
type ClusterClientHandler func(http.ResponseWriter, *http.Request, httprouter.Params, *SessionContext, ClusterClientProvider) (interface{}, error)

// WithClusterClientProvider wraps a ClusterClientHandler to ensure that a
// request is authenticated to this proxy (the same as WithAuth), and passes a
// ClusterClientProvider so that the handler can access remote clusters. Use
// this instead of WithClusterAuth when the remote cluster cannot be encoded in
// the path or multiple clusters may need to be accessed from a single handler.
func (h *Handler) WithClusterClientProvider(fn ClusterClientHandler) httprouter.Handle {
	return httplib.MakeHandler(func(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
		sctx, err := h.AuthenticateRequest(w, r, true)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		g := clusterClientProvider{
			h:   h,
			ctx: sctx,
		}
		return fn(w, r, p, sctx, g)
	})
}

// ProvisionTokenHandler is a authenticated handler that is called for some existing Token
type ProvisionTokenHandler func(w http.ResponseWriter, r *http.Request, p httprouter.Params, site reversetunnelclient.RemoteSite, token types.ProvisionToken) (interface{}, error)

// WithProvisionTokenAuth ensures that request is authenticated with a provision token.
// Provision tokens, when used like this are invalidated as soon as used.
// Doesn't matter if the underlying response was a success or an error.
func (h *Handler) WithProvisionTokenAuth(fn ProvisionTokenHandler) httprouter.Handle {
	return httplib.MakeHandler(func(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
		ctx := r.Context()

		creds, err := roundtrip.ParseAuthHeaders(r)
		if err != nil {
			return nil, trace.AccessDenied("need auth")
		}

		token, err := consumeTokenForAPICall(ctx, h.GetProxyClient(), creds.Password)
		if err != nil {
			return nil, trace.AccessDenied("need auth")
		}

		site, err := h.cfg.Proxy.GetSite(h.auth.clusterName)
		if err != nil {
			h.logger.WarnContext(ctx, "Failed to query cluster", "error", err, "cluster", h.auth.clusterName)
			return nil, trace.Wrap(err)
		}

		return fn(w, r, p, site, token)
	})
}

// consumeTokenForAPICall will fetch a token, check if the requireRole is present and then delete the token
// If any of those calls returns an error, this method also returns an error
//
// If multiple clients reach here at the same time, only one of them will be able to actually make the request.
// This is possible because the latest call - DeleteToken - returns an error if the resource doesn't exist
// This is currently true for all the backends as explained here
// https://github.com/gravitational/teleport/commit/24fcadc375d8359e80790b3ebeaa36bd8dd2822f
func consumeTokenForAPICall(ctx context.Context, proxyClient authclient.ClientI, tokenName string) (types.ProvisionToken, error) {
	token, err := proxyClient.GetToken(ctx, tokenName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if token.GetJoinMethod() != types.JoinMethodToken {
		return nil, trace.BadParameter("unexpected join method %q for token %q", token.GetJoinMethod(), token.GetSafeName())
	}

	if !checkTokenTTL(token) {
		return nil, trace.BadParameter("expired token %q", token.GetSafeName())
	}

	if err := proxyClient.DeleteToken(ctx, token.GetName()); err != nil {
		return nil, trace.Wrap(err)
	}

	return token, nil
}

// checkTokenTTL returns true if the token is still valid.
// This is similar to checkTokenTTL in auth.Server, but does not delete expired tokens.
func checkTokenTTL(tok types.ProvisionToken) bool {
	// Always accept tokens without an expiry configured.
	if tok.Expiry().IsZero() {
		return true
	}

	now := time.Now().UTC()

	return tok.Expiry().After(now)
}

type redirectHandlerFunc func(w http.ResponseWriter, r *http.Request, p httprouter.Params) (redirectURL string)

func isValidRedirectURL(redirectURL string) bool {
	u, err := url.ParseRequestURI(redirectURL)
	return err == nil && (!u.IsAbs() || (u.Scheme == "http" || u.Scheme == "https"))
}

// WithRedirect is a handler that redirects to the path specified in the returned value.
func (h *Handler) WithRedirect(fn redirectHandlerFunc) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		app.SetRedirectPageHeaders(w.Header(), "")

		redirectURL := fn(w, r, p)
		if !isValidRedirectURL(redirectURL) {
			redirectURL = client.LoginFailedRedirectURL
		}
		http.Redirect(w, r, redirectURL, http.StatusFound)
	}
}

// WithMetaRedirect is a handler that redirects to the path specified
// using HTML rather than HTTP. This is needed for redirects that can
// have a header size larger than 8kb, which some middlewares will drop.
// See https://github.com/gravitational/teleport/issues/7467.
func (h *Handler) WithMetaRedirect(fn redirectHandlerFunc) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		redirectURL := fn(w, r, p)
		if !isValidRedirectURL(redirectURL) {
			redirectURL = client.LoginFailedRedirectURL
		}
		err := app.MetaRedirect(w, redirectURL)
		if err != nil {
			h.logger.WarnContext(r.Context(), "Failed to issue a redirect", "error", err)
		}
	}
}

// WithAuth ensures that a request is authenticated.
// Authenticated requests require both a session cookie as well as a bearer token.
// WithAuth also provides CSRF protection by requiring the bearer token to be present.
func (h *Handler) WithAuth(fn ContextHandler) httprouter.Handle {
	return httplib.MakeHandler(func(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
		sctx, err := h.AuthenticateRequest(w, r, true /* check bearer token */)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return fn(w, r, p, sctx)
	})
}

// WithSession ensures that the request provides a session cookie.
// It does not check for a bearer token.
//
// WithSession does not provide CSRF protection, so it should only
// be used for non-state-changing requests or when other CSRF mitigations
// are applied.
func (h *Handler) WithSession(fn ContextHandler) httprouter.Handle {
	return httplib.MakeHandler(func(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
		sctx, err := h.AuthenticateRequest(w, r, false /* check bearer token */)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return fn(w, r, p, sctx)
	})
}

// WithUnauthenticatedLimiter adds a conditional IP-based rate limiting that will limit only unauthenticated requests.
// This is a good default to use as both Cluster and User auth are checked here, but `WithLimiter` can be used if
// you're certain that no authenticated requests will be made.
func (h *Handler) WithUnauthenticatedLimiter(fn httplib.HandlerFunc) httprouter.Handle {
	return h.unauthenticatedLimiterFunc(fn, h.WithLimiterHandlerFunc)
}

// WithUnauthenticatedHighLimiter adds a conditional IP-based rate limiting that will limit only unauthenticated
// requests.  This is similar to WithUnauthenticatedLimiter, however this one allows a much higher rate limit.
// This higher rate limit should only be used on endpoints which are only CPU constrained
// (no file or other resources used).
func (h *Handler) WithUnauthenticatedHighLimiter(fn httplib.HandlerFunc) httprouter.Handle {
	return h.unauthenticatedLimiterFunc(fn, h.WithHighLimiterHandlerFunc)
}

func (h *Handler) unauthenticatedLimiterFunc(fn httplib.HandlerFunc, rateFunc func(fn httplib.HandlerFunc) httplib.HandlerFunc) httprouter.Handle {
	return httplib.MakeHandler(func(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
		if _, _, err := h.authenticateRequestWithCluster(w, r, p); err != nil {
			// retry with user auth
			if _, err = h.AuthenticateRequest(w, r, true /* check token */); err != nil {
				// no auth passed, limit request
				return rateFunc(fn)(w, r, p)
			}
		}
		// auth passed, call directly
		return fn(w, r, p)
	})
}

// WithLimiter adds IP-based rate limiting to fn.
// Limits are applied to all requests, authenticated or not.
func (h *Handler) WithLimiter(fn httplib.HandlerFunc) httprouter.Handle {
	return httplib.MakeHandler(func(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
		return h.WithLimiterHandlerFunc(fn)(w, r, p)
	})
}

// WithHighLimiter adds high rate IP-based rate limiting to fn.
// This should only be used on functions which are CPU constrained, and don't use disk or other services.
// Limits are applied to all requests, authenticated or not.
func (h *Handler) WithHighLimiter(fn httplib.HandlerFunc) httprouter.Handle {
	return httplib.MakeHandler(func(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
		return h.WithHighLimiterHandlerFunc(fn)(w, r, p)
	})
}

// WithLimiterHandlerFunc adds IP-based rate limiting to a HandlerFunc. This
// should be used when you need to nest this inside another HandlerFunc.
func (h *Handler) WithLimiterHandlerFunc(fn httplib.HandlerFunc) httplib.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
		err := rateLimitRequest(r, h.limiter)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return fn(w, r, p)
	}
}

// WithHighLimiterHandlerFunc adds IP-based rate limiting to a HandlerFunc. This is similar to WithLimiterHandlerFunc
// but provides a higher rate limit.  This should only be used for requests which are only CPU bound (no disk or other
// resources used).
func (h *Handler) WithHighLimiterHandlerFunc(fn httplib.HandlerFunc) httplib.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
		err := rateLimitRequest(r, h.highLimiter)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return fn(w, r, p)
	}
}

func rateLimitRequest(r *http.Request, limiter *limiter.RateLimiter) error {
	remote, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(limiter.RegisterRequest(remote, nil /* customRate */))
}

func (h *Handler) validateCookie(w http.ResponseWriter, r *http.Request) (*SessionContext, error) {
	const missingCookieMsg = "missing session cookie"
	cookie, err := r.Cookie(websession.CookieName)
	if err != nil || (cookie != nil && cookie.Value == "") {
		return nil, trace.AccessDenied("%s", missingCookieMsg)
	}
	decodedCookie, err := websession.DecodeCookie(cookie.Value)
	if err != nil {
		return nil, trace.AccessDenied("failed to decode cookie")
	}
	sctx, err := h.auth.getOrCreateSession(r.Context(), decodedCookie.User, decodedCookie.SID)
	if err != nil {
		clearSessionCookies((w))
		return nil, trace.AccessDenied("need auth")
	}

	return sctx, nil
}

// AuthenticateRequest authenticates request using combination of a session cookie
// and bearer token
func (h *Handler) AuthenticateRequest(w http.ResponseWriter, r *http.Request, checkBearerToken bool) (*SessionContext, error) {
	sctx, err := h.validateCookie(w, r)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if checkBearerToken {
		creds, err := roundtrip.ParseAuthHeaders(r)
		if err != nil {
			return nil, trace.AccessDenied("need auth")
		}
		if err := sctx.validateBearerToken(r.Context(), creds.Password); err != nil {
			return nil, trace.AccessDenied("bad bearer token")
		}
	}

	if err := parseMFAResponseFromRequest(r); err != nil {
		return nil, trace.Wrap(err)
	}

	return sctx, nil
}

// parseMFAResponse checks for an MFA response in the request header.
// if found, the mfa response is added to the request context, where
// it can be recalled to augment client authentication further down
// the call stack.
func parseMFAResponseFromRequest(r *http.Request) error {
	ctx, err := contextWithMFAResponseFromRequestHeader(r.Context(), r.Header)
	if err != nil {
		return trace.Wrap(err)
	}

	// Update the request reference with a cloned request with the update ctx.
	*r = *r.WithContext(ctx)
	return nil
}

// contextWithMFAResponseFromRequestHeader attempts to parse an MFA response
// from the request header. If found, the MFA response is added to the given
// context and returned.
func contextWithMFAResponseFromRequestHeader(ctx context.Context, requestHeader http.Header) (context.Context, error) {
	if mfaResponseJSON := requestHeader.Get("Teleport-MFA-Response"); mfaResponseJSON != "" {
		mfaResp, err := client.ParseMFAChallengeResponse([]byte(mfaResponseJSON))
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return mfa.ContextWithMFAResponse(ctx, mfaResp), nil
	}

	return ctx, nil
}

type wsBearerToken struct {
	Token string `json:"token"`
}

type wsStatus struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// wsIODeadline is used to set a deadline for receiving a message from
// an authenticated websocket so unauthenticated sockets dont get left
// open.
const wsIODeadline = time.Second * 4

// AuthenticateRequest authenticates request using combination of a session cookie
// and bearer token retrieved from a websocket
func (h *Handler) AuthenticateRequestWS(w http.ResponseWriter, r *http.Request) (*SessionContext, *websocket.Conn, error) {
	sctx, err := h.validateCookie(w, r)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	ws, err := authnWsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, nil, trace.ConnectionProblem(err, "Error upgrading to websocket: %v", err)
	}
	if err := ws.SetReadDeadline(time.Now().Add(wsIODeadline)); err != nil {
		return nil, nil, trace.ConnectionProblem(err, "Error setting websocket read deadline: %v", err)
	}

	var t wsBearerToken
	if err := ws.ReadJSON(&t); err != nil {
		return nil, nil, trace.Wrap(err)
	}
	if err := sctx.validateBearerToken(r.Context(), t.Token); err != nil {
		writeErr := ws.WriteJSON(wsStatus{
			Type:    "create_session_response",
			Status:  "error",
			Message: "invalid token",
		})
		if writeErr != nil {
			h.logger.ErrorContext(r.Context(), "Error while writing invalid token error to websocket", "error", writeErr)
		}

		return nil, nil, trace.Wrap(err)
	}

	if err := ws.WriteJSON(wsStatus{
		Type:   "create_session_response",
		Status: "ok",
	}); err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// unset the deadline as downstream consumers should handle this themselves.
	if err := ws.SetReadDeadline(time.Time{}); err != nil {
		return nil, nil, trace.ConnectionProblem(err, "Error setting websocket read deadline: %v", err)
	}

	if err := parseMFAResponseFromRequest(r); err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return sctx, ws, nil
}

// ProxyWithRoles returns a reverse tunnel proxy verifying the permissions
// of the given user.
func (h *Handler) ProxyWithRoles(ctx context.Context, sctx *SessionContext) (reversetunnelclient.Tunnel, error) {
	accessChecker, err := sctx.GetUserAccessChecker()
	if err != nil {
		h.logger.WarnContext(ctx, "Failed to get client roles", "error", err)
		return nil, trace.Wrap(err)
	}

	cn, err := h.cfg.AccessPoint.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return reversetunnelclient.NewTunnelWithRoles(h.cfg.Proxy, cn.GetClusterName(), accessChecker, h.cfg.AccessPoint), nil
}

// ProxyHostPort returns the address of the proxy server using --proxy
// notation, i.e. "localhost:8030,8023"
func (h *Handler) ProxyHostPort() string {
	hp := createHostPort(h.cfg.ProxyWebAddr, defaults.HTTPListenPort)
	return fmt.Sprintf("%s,%s", hp, h.sshPort)
}

func (h *Handler) kubeProxyHostPort() string {
	return createHostPort(h.cfg.ProxyKubeAddr, defaults.KubeListenPort)
}

// Address can be set in the config with unspecified host, like
// 0.0.0.0:3080 or :443.
//
// In this case, the dial will succeed (dialing 0.0.0.0 is same a dialing
// localhost in Go) but the host certificate will not validate since
// 0.0.0.0 is never a valid principal (auth server explicitly removes it
// when issuing host certs).
//
// As such, replace 0.0.0.0 with localhost in this case: proxy listens on
// all interfaces and localhost is always included in the valid principal
// set.
func createHostPort(netAddr utils.NetAddr, port int) string {
	if netAddr.IsHostUnspecified() {
		return fmt.Sprintf("localhost:%v", netAddr.Port(port))
	}
	return netAddr.String()
}

func message(msg string) interface{} {
	return map[string]interface{}{"message": msg}
}

// OK is a response that indicates request was successful.
func OK() interface{} {
	return message("ok")
}

// makeTeleportClientConfig creates default teleport client configuration
// that is used to initiate an SSH terminal session or SCP file transfer
func makeTeleportClientConfig(ctx context.Context, sctx *SessionContext) (*client.Config, error) {
	agent, cert, err := sctx.GetAgent()
	if err != nil {
		return nil, trace.BadParameter("failed to get user credentials: %v", err)
	}

	signers, err := agent.Signers()
	if err != nil {
		return nil, trace.BadParameter("failed to get user credentials: %v", err)
	}

	tlsConfig, err := sctx.ClientTLSConfig(ctx)
	if err != nil {
		return nil, trace.BadParameter("failed to get client TLS config: %v", err)
	}

	callback, err := apisshutils.NewHostKeyCallback(
		apisshutils.HostKeyCallbackConfig{
			GetHostCheckers: sctx.getCheckers,
			Clock:           sctx.cfg.Parent.clock,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	proxyListenerMode, err := sctx.GetProxyListenerMode(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	config := &client.Config{
		Username:          sctx.GetUser(),
		Agent:             agent,
		NonInteractive:    true,
		TLS:               tlsConfig,
		AuthMethods:       []ssh.AuthMethod{ssh.PublicKeys(signers...)},
		ProxySSHPrincipal: cert.ValidPrincipals[0],
		HostKeyCallback:   callback,
		TLSRoutingEnabled: proxyListenerMode == types.ProxyListenerMode_Multiplex,
		Tracer:            apitracing.DefaultProvider().Tracer("webterminal"),
	}

	return config, nil
}

// SSORequestParams holds parameters parsed out of a HTTP request initiating an
// SSO login. See ParseSSORequestParams().
type SSORequestParams struct {
	// ClientRedirectURL is the URL specified in the query parameter
	// redirect_url, which will be unescaped here.
	ClientRedirectURL string
	// ConnectorID identifies the SSO connector to use to log in, from
	// the connector_id query parameter.
	ConnectorID string
	// CSRFToken is used to protect against login-CSRF in SSO flows.
	CSRFToken string
}

// ParseSSORequestParams extracts the SSO request parameters from an http.Request,
// returning them in an SSORequestParams struct. If any fields are not present,
// an error is returned.
func ParseSSORequestParams(r *http.Request) (*SSORequestParams, error) {
	// Manually grab the value from query param "redirect_url".
	//
	// The "redirect_url" param can contain its own query params such as in
	// "https://localhost/login?connector_id=github&redirect_url=https://localhost:8080/web/cluster/im-a-cluster-name/nodes?search=tunnel&sort=hostname:asc",
	// which would be incorrectly parsed with the standard method.
	// For example a call to r.URL.Query().Get("redirect_url") in the example above would return
	// "https://localhost:8080/web/cluster/im-a-cluster-name/nodes?search=tunnel",
	// as it would take the "&sort=hostname:asc" to be a separate query param.
	//
	// This logic assumes that anything coming after "redirect_url" is part of
	// the redirect URL.
	splittedRawQuery := strings.Split(r.URL.RawQuery, "&redirect_url=")
	var clientRedirectURL string
	if len(splittedRawQuery) > 1 {
		clientRedirectURL, _ = url.QueryUnescape(splittedRawQuery[1])
	}
	if clientRedirectURL == "" {
		return nil, trace.BadParameter("missing redirect_url query parameter")
	}

	query := r.URL.Query()
	connectorID := query.Get("connector_id")
	if connectorID == "" {
		return nil, trace.BadParameter("missing connector_id query parameter")
	}

	csrfToken, err := csrf.ExtractTokenFromCookie(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &SSORequestParams{
		ClientRedirectURL: clientRedirectURL,
		ConnectorID:       connectorID,
		CSRFToken:         csrfToken,
	}, nil
}

// SSOCallbackResponse holds the parameters for validating and executing an SSO
// callback URL. See SSOSetWebSessionAndRedirectURL().
type SSOCallbackResponse struct {
	// CSRFToken is the token provided in the originating SSO login request
	// to be validated against.
	CSRFToken string
	// Username is the authenticated teleport username of the user that has
	// logged in, provided by the SSO provider.
	Username string
	// SessionName is the name of the session generated by auth server if
	// requested in the SSO request.
	SessionName string
	// ClientRedirectURL is the URL to redirect back to on completion of
	// the SSO login process.
	ClientRedirectURL string
	// MFAToken is an SSO MFA token.
	MFAToken string
}

// SSOSetWebSessionAndRedirectURL validates the CSRF token in the response
// against that in the request, validates that the callback URL in the response
// can be parsed, and sets a session cookie with the username and session name
// from the response. On success, nil is returned. If the validation fails, an
// error is returned.
func SSOSetWebSessionAndRedirectURL(w http.ResponseWriter, r *http.Request, response *SSOCallbackResponse, verifyCSRF bool) error {
	if verifyCSRF {
		// Make sure that the CSRF token provided in this request matches
		// the token that was used to initiate this SSO attempt.
		//
		// This ensures that an attacker cannot perform a login CSRF attack
		// in order to get the victim to log in to the incorrect account.
		if err := csrf.VerifyToken(response.CSRFToken, r); err != nil {
			return trace.Wrap(err)
		}
	}

	if err := websession.SetCookie(w, response.Username, response.SessionName); err != nil {
		return trace.Wrap(err)
	}

	parsedRedirectURL, err := httplib.OriginLocalRedirectURI(response.ClientRedirectURL)
	if err != nil {
		return trace.Wrap(err)
	}
	response.ClientRedirectURL = parsedRedirectURL

	return nil
}

const robots = `User-agent: *
Disallow: /`

func serveRobotsTxt(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(robots))
	return nil, nil
}

func readEtagFromAppHash(fs http.FileSystem) (string, error) {
	hashFile, err := fs.Open("/apphash")
	if err != nil {
		return "", trace.Wrap(err)
	}
	defer hashFile.Close()

	appHash, err := io.ReadAll(hashFile)
	if err != nil {
		return "", trace.Wrap(err)
	}

	versionWithHash := fmt.Sprintf("%s-%s", teleport.Version, string(appHash))
	etag := fmt.Sprintf("%q", versionWithHash)

	return etag, nil
}
