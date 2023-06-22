/*
Copyright 2015-2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package web implements web proxy handler that provides
// web interface to view and connect to teleport nodes
package web

import (
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/oxy/ratelimit"
	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/julienschmidt/httprouter"
	lemma_secret "github.com/mailgun/lemma/secret"
	"github.com/sashabaranov/go-openai"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	oteltrace "go.opentelemetry.io/otel/trace"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"golang.org/x/crypto/ssh"
	"golang.org/x/exp/slices"
	"golang.org/x/mod/semver"
	"golang.org/x/time/rate"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/gravitational/teleport"
	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	apitracing "github.com/gravitational/teleport/api/observability/tracing"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/installers"
	"github.com/gravitational/teleport/api/utils/keys"
	apisshutils "github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	dtconfig "github.com/gravitational/teleport/lib/devicetrust/config"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/httplib/csrf"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/multiplexer"
	"github.com/gravitational/teleport/lib/plugin"
	"github.com/gravitational/teleport/lib/proxy"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/secret"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/web/app"
	"github.com/gravitational/teleport/lib/web/ui"
)

const (
	// SSOLoginFailureMessage is a generic error message to avoid disclosing sensitive SSO failure messages.
	SSOLoginFailureMessage = "Failed to login. Please check Teleport's log for more details."

	// assistantTokensPerHour defines how many assistant rate limiter tokens are replenished every hour.
	assistantTokensPerHour = 140
	// assistantLimiterRate is the rate (in tokens per second)
	// at which tokens for the assistant rate limiter are replenished
	assistantLimiterRate = rate.Limit(assistantTokensPerHour / float64(time.Hour/time.Second))
	// assistantLimiterCapacity is the total capacity of the token bucket for the assistant rate limiter.
	// The bucket starts full, prefilled for a week.
	assistantLimiterCapacity = assistantTokensPerHour * 24 * 7
)

// healthCheckAppServerFunc defines a function used to perform a health check
// to AppServer that can handle application requests (based on cluster name and
// public address).
type healthCheckAppServerFunc func(ctx context.Context, publicAddr string, clusterName string) error

// Handler is HTTP web proxy handler
type Handler struct {
	log logrus.FieldLogger

	sync.Mutex
	httprouter.Router
	cfg                     Config
	auth                    *sessionCache
	sessionStreamPollPeriod time.Duration
	clock                   clockwork.Clock
	limiter                 *limiter.RateLimiter
	// assistantLimiter limits the amount of tokens that can be consumed
	// by OpenAI API calls when using a shared key.
	// golang.org/x/time/rate is used, as the oxy ratelimiter
	// is quite tightly tied to individual http.Requests,
	// and instead we want to consume arbitrary amounts of tokens.
	assistantLimiter     *rate.Limiter
	healthCheckAppServer healthCheckAppServerFunc
	// sshPort specifies the SSH proxy port extracted
	// from configuration
	sshPort string

	// userConns tracks amount of current active connections with user certificates.
	userConns atomic.Int32

	// ClusterFeatures contain flags for supported and unsupported features.
	ClusterFeatures proto.Features
}

// HandlerOption is a functional argument - an option that can be passed
// to NewHandler function
type HandlerOption func(h *Handler) error

// SetSessionStreamPollPeriod sets polling period for session streams
func SetSessionStreamPollPeriod(period time.Duration) HandlerOption {
	return func(h *Handler) error {
		if period < 0 {
			return trace.BadParameter("period should be non zero")
		}
		h.sessionStreamPollPeriod = period
		return nil
	}
}

// SetClock sets the clock on a handler
func SetClock(clock clockwork.Clock) HandlerOption {
	return func(h *Handler) error {
		h.clock = clock
		return nil
	}
}

type proxySettingsGetter interface {
	GetProxySettings(ctx context.Context) (*webclient.ProxySettings, error)
}

// Config represents web handler configuration parameters
type Config struct {
	// PluginRegistry handles plugin registration
	PluginRegistry plugin.Registry
	// Proxy is a reverse tunnel proxy that handles connections
	// to local cluster or remote clusters using unified interface
	Proxy reversetunnel.Tunnel
	// AuthServers is a list of auth servers this proxy talks to
	AuthServers utils.NetAddr
	// DomainName is a domain name served by web handler
	DomainName string
	// ProxyClient is a client that authenticated as proxy
	ProxyClient auth.ClientI
	// ProxySSHAddr points to the SSH address of the proxy
	ProxySSHAddr utils.NetAddr
	// ProxyKubeAddr points to the Kube address of the proxy
	ProxyKubeAddr utils.NetAddr
	// ProxyWebAddr points to the web (HTTPS) address of the proxy
	ProxyWebAddr utils.NetAddr
	// ProxyPublicAddr contains web proxy public addresses.
	ProxyPublicAddrs []utils.NetAddr

	// CipherSuites is the list of cipher suites Teleport suppports.
	CipherSuites []uint16

	// FIPS mode means Teleport started in a FedRAMP/FIPS 140-2 compliant
	// configuration.
	FIPS bool

	// AccessPoint holds a cache to the Auth Server.
	AccessPoint auth.ProxyAccessPoint

	// Emitter is event emitter
	Emitter events.StreamEmitter

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
	ProxySettings proxySettingsGetter

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
	SessionControl *srv.SessionController

	// PROXYSigner is used to sign PROXY header and securely propagate client IP information
	PROXYSigner multiplexer.PROXYHeaderSigner

	// TracerProvider generates tracers to create spans with
	TracerProvider oteltrace.TracerProvider

	// HealthCheckAppServer is a function that checks if the proxy can handle
	// application requests.
	HealthCheckAppServer healthCheckAppServerFunc

	// UI provides config options for the web UI
	UI webclient.UIConfig

	// OpenAIConfig provides config options for the OpenAI integration.
	OpenAIConfig *openai.ClientConfig
}

type APIHandler struct {
	handler *Handler

	// appHandler is a http.Handler to forward requests to applications.
	appHandler *app.Handler
}

// ConnectionHandler defines a function for serving incoming connections.
type ConnectionHandler func(ctx context.Context, conn net.Conn) error

// Check if this request should be forwarded to an application handler to
// be handled by the UI and handle the request appropriately.
func (h *APIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// If the request is either to the fragment authentication endpoint or if the
	// request has a session cookie or a client cert, forward to
	// application handlers. If the request is requesting a
	// FQDN that is not of the proxy, redirect to application launcher.

	if h.appHandler != nil && (app.HasFragment(r) || app.HasSession(r) || app.HasClientCert(r)) {
		h.appHandler.ServeHTTP(w, r)
		return
	}

	// Only try to redirect if the handler is serving the full Web API.
	if !h.handler.cfg.MinimalReverseTunnelRoutesOnly {
		if redir, ok := app.HasName(r, h.handler.cfg.ProxyPublicAddrs); ok {
			http.Redirect(w, r, redir, http.StatusFound)
			return
		}
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
	const apiPrefix = "/" + teleport.WebAPIVersion
	cfg.ProxyClient = auth.WithGithubConnectorConversions(cfg.ProxyClient)
	h := &Handler{
		cfg:                  cfg,
		log:                  newPackageLogger(),
		clock:                clockwork.NewRealClock(),
		ClusterFeatures:      cfg.ClusterFeatures,
		healthCheckAppServer: cfg.HealthCheckAppServer,
	}

	// Check for self-hosted vs Cloud.
	// TODO(justinas): this needs to be modified when we allow user-supplied API keys in Cloud
	if cfg.ClusterFeatures.GetCloud() {
		h.assistantLimiter = rate.NewLimiter(assistantLimiterRate, assistantLimiterCapacity)
	} else {
		// Set up a limiter with "infinite limit", the "burst" parameter is ignored
		h.assistantLimiter = rate.NewLimiter(rate.Inf, 0)
	}

	// for properly handling url-encoded parameter values.
	h.UseRawPath = true

	for _, o := range opts {
		if err := o(h); err != nil {
			return nil, trace.Wrap(err)
		}
	}

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
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	h.auth = sessionCache
	sshPortValue := strconv.Itoa(defaults.SSHProxyListenPort)
	if cfg.ProxySSHAddr.String() != "" {
		_, sshPort, err := net.SplitHostPort(cfg.ProxySSHAddr.String())
		if err != nil {
			h.log.WithError(err).Warnf("Invalid SSH proxy address %q, will use default port %v.",
				cfg.ProxySSHAddr.String(), defaults.SSHProxyListenPort)

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
				Period:  defaults.LimiterPasswordlessPeriod,
				Average: defaults.LimiterPasswordlessAverage,
				Burst:   defaults.LimiterPasswordlessBurst,
			},
		},
		MaxConnections:   defaults.LimiterMaxConnections,
		MaxNumberOfUsers: defaults.LimiterMaxConcurrentUsers,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.MinimalReverseTunnelRoutesOnly {
		h.bindMinimalEndpoints()
	} else {
		h.bindDefaultEndpoints()
	}

	// if Web UI is enabled, check the assets dir:
	var indexPage *template.Template
	if cfg.StaticFS != nil {
		index, err := cfg.StaticFS.Open("/index.html")
		if err != nil {
			h.log.WithError(err).Error("Failed to open index file.")
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

		h.Handle("GET", "/web/config.js", httplib.MakeHandler(h.getWebConfig))
	}

	routingHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// ensure security headers are set for all responses
		httplib.SetDefaultSecurityHeaders(w.Header())

		// request is going to the API?
		if strings.HasPrefix(r.URL.Path, apiPrefix) {
			http.StripPrefix(apiPrefix, h).ServeHTTP(w, r)
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
			fs := http.FileServer(cfg.StaticFS)

			fs = makeGzipHandler(fs)
			fs = makeCacheHandler(fs)

			http.StripPrefix("/web", fs).ServeHTTP(w, r)
		} else if strings.HasPrefix(r.URL.Path, "/web/") || r.URL.Path == "/web" {
			csrfToken, err := csrf.AddCSRFProtection(w, r)
			if err != nil {
				h.log.WithError(err).Warn("Failed to generate CSRF token.")
			}

			session, err := h.authenticateWebSession(w, r)
			if err != nil {
				h.log.Debugf("Could not authenticate: %v", err)
			}
			session.XCSRF = csrfToken

			httplib.SetNoCacheHeaders(w.Header())

			// app access needs to make a CORS fetch request, so we only set the default CSP on that page
			if strings.HasPrefix(r.URL.Path, "/web/launch/") {
				parts := strings.Split(r.URL.Path, "/")
				// grab the FQDN from the URL to allow in the connect-src CSP
				applicationURL := "https://" + parts[3] + ":*"

				httplib.SetAppLaunchContentSecurityPolicy(w.Header(), applicationURL)
			} else {
				httplib.SetIndexContentSecurityPolicy(w.Header())
			}
			if err := indexPage.Execute(w, session); err != nil {
				h.log.WithError(err).Error("Failed to execute index page template.")
			}
		} else {
			http.NotFound(w, r)
		}
	})

	h.NotFound = routingHandler

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
			Clock:            h.clock,
			AuthClient:       cfg.ProxyClient,
			AccessPoint:      cfg.AccessPoint,
			ProxyClient:      cfg.Proxy,
			CipherSuites:     cfg.CipherSuites,
			ProxyPublicAddrs: cfg.ProxyPublicAddrs,
			WebPublicAddr:    resp.SSH.PublicAddr,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if h.healthCheckAppServer == nil {
			h.healthCheckAppServer = appHandler.HealthCheckAppServer
		}
	}

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
	ctx, err := h.AuthenticateRequest(w, r, false)
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
	h.GET("/webapi/find", httplib.MakeHandler(h.find))
	// Issue host credentials.
	h.POST("/webapi/host/credentials", httplib.MakeHandler(h.hostCredentials))
}

// bindDefaultEndpoints binds the default endpoints for the web API.
func (h *Handler) bindDefaultEndpoints() {
	h.bindMinimalEndpoints()

	// ping endpoint is used to check if the server is up. the /webapi/ping
	// endpoint returns the default authentication method and configuration that
	// the server supports. the /webapi/ping/:connector endpoint can be used to
	// query the authentication configuration for a specific connector.
	h.GET("/webapi/ping", httplib.MakeHandler(h.ping))
	h.GET("/webapi/ping/:connector", httplib.MakeHandler(h.pingWithConnector))

	// Unauthenticated access to JWT public keys.
	h.GET("/.well-known/jwks.json", httplib.MakeHandler(h.jwks))

	// Unauthenticated access to the message of the day
	h.GET("/webapi/motd", httplib.MakeHandler(h.motd))

	// Unauthenticated access to retrieving the script used to install
	// Teleport
	h.GET("/webapi/scripts/installer/:name", httplib.MakeHandler(h.installer))

	// desktop access configuration scripts
	h.GET("/webapi/scripts/desktop-access/install-ad-ds.ps1", httplib.MakeHandler(h.desktopAccessScriptInstallADDSHandle))
	h.GET("/webapi/scripts/desktop-access/install-ad-cs.ps1", httplib.MakeHandler(h.desktopAccessScriptInstallADCSHandle))
	h.GET("/webapi/scripts/desktop-access/configure/:token/configure-ad.ps1", httplib.MakeHandler(h.desktopAccessScriptConfigureHandle))

	// Forwards traces to the configured upstream collector
	h.POST("/webapi/traces", h.WithAuth(h.traces))

	// App sessions
	h.POST("/webapi/sessions/app", h.WithAuth(h.createAppSession))

	// DELETE IN 13, deprecated/unused web sessions routes (avatus)
	// https://github.com/gravitational/teleport/pull/19892
	h.POST("/webapi/sessions", httplib.WithCSRFProtection(h.WithLimiterHandlerFunc(h.createWebSession)))
	h.DELETE("/webapi/sessions", h.WithAuth(h.deleteWebSession))
	h.POST("/webapi/sessions/renew", h.WithAuth(h.renewWebSession))

	// Web sessions
	h.POST("/webapi/sessions/web", httplib.WithCSRFProtection(h.WithLimiterHandlerFunc(h.createWebSession)))
	h.DELETE("/webapi/sessions/web", h.WithAuth(h.deleteWebSession))
	h.POST("/webapi/sessions/web/renew", h.WithAuth(h.renewWebSession))
	h.POST("/webapi/users", h.WithAuth(h.createUserHandle))
	h.PUT("/webapi/users", h.WithAuth(h.updateUserHandle))
	h.GET("/webapi/users", h.WithAuth(h.getUsersHandle))
	h.DELETE("/webapi/users/:username", h.WithAuth(h.deleteUserHandle))

	// We have an overlap route here, please see godoc of handleGetUserOrResetToken
	// h.GET("/webapi/users/:username", h.WithAuth(h.getUserHandle))
	// h.GET("/webapi/users/password/token/:token", httplib.MakeHandler(h.getResetPasswordTokenHandle))
	h.GET("/webapi/users/*wildcard", h.handleGetUserOrResetToken)

	h.PUT("/webapi/users/password/token", httplib.WithCSRFProtection(h.changeUserAuthentication))
	h.PUT("/webapi/users/password", h.WithAuth(h.changePassword))
	h.POST("/webapi/users/password/token", h.WithAuth(h.createResetPasswordToken))
	h.POST("/webapi/users/privilege/token", h.WithAuth(h.createPrivilegeTokenHandle))

	// Issues SSH temp certificates based on 2FA access creds
	h.POST("/webapi/ssh/certs", h.WithLimiter(h.createSSHCert))

	// list available sites
	h.GET("/webapi/sites", h.WithAuth(h.getClusters))

	// Site specific API

	// get namespaces
	h.GET("/webapi/sites/:site/namespaces", h.WithClusterAuth(h.getSiteNamespaces))

	// get nodes
	h.GET("/webapi/sites/:site/nodes", h.WithClusterAuth(h.clusterNodesGet))

	// Get applications.
	h.GET("/webapi/sites/:site/apps", h.WithClusterAuth(h.clusterAppsGet))

	// get login alerts
	h.GET("/webapi/sites/:site/alerts", h.WithClusterAuth(h.clusterLoginAlertsGet))

	// active sessions handlers
	h.GET("/webapi/sites/:site/connect", h.WithClusterAuth(h.siteNodeConnect))                     // connect to an active session (via websocket)
	h.GET("/webapi/sites/:site/sessions", h.WithClusterAuth(h.clusterActiveAndPendingSessionsGet)) // get list of active and pending sessions

	// Audit events handlers.
	h.GET("/webapi/sites/:site/events/search", h.WithClusterAuth(h.clusterSearchEvents))                 // search site events
	h.GET("/webapi/sites/:site/events/search/sessions", h.WithClusterAuth(h.clusterSearchSessionEvents)) // search site session events
	h.GET("/webapi/sites/:site/sessions/:sid/events", h.WithClusterAuth(h.siteSessionEventsGet))         // get recorded session's timing information (from events)
	h.GET("/webapi/sites/:site/sessions/:sid/stream", h.siteSessionStreamGet)                            // get recorded session's bytes (from events)

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

	// token generation
	h.POST("/webapi/token", h.WithAuth(h.createTokenHandle))

	// join scripts
	h.GET("/scripts/:token/install-node.sh", httplib.MakeHandler(h.getNodeJoinScriptHandle))
	h.GET("/scripts/:token/install-app.sh", httplib.MakeHandler(h.getAppJoinScriptHandle))
	h.GET("/scripts/:token/install-database.sh", httplib.MakeHandler(h.getDatabaseJoinScriptHandle))
	// web context
	h.GET("/webapi/sites/:site/context", h.WithClusterAuth(h.getUserContext))
	h.GET("/webapi/sites/:site/resources/check", h.WithClusterAuth(h.checkAccessToRegisteredResource))

	// Database access handlers.
	h.GET("/webapi/sites/:site/databases", h.WithClusterAuth(h.clusterDatabasesGet))
	h.POST("/webapi/sites/:site/databases", h.WithClusterAuth(h.handleDatabaseCreate))
	h.PUT("/webapi/sites/:site/databases/:database", h.WithClusterAuth(h.handleDatabaseUpdate))
	h.GET("/webapi/sites/:site/databases/:database", h.WithClusterAuth(h.clusterDatabaseGet))
	h.GET("/webapi/sites/:site/databases/:database/iam/policy", h.WithClusterAuth(h.handleDatabaseGetIAMPolicy))

	// DatabaseService handlers
	h.GET("/webapi/sites/:site/databaseservices", h.WithClusterAuth(h.clusterDatabaseServicesList))

	// Kube access handlers.
	h.GET("/webapi/sites/:site/kubernetes", h.WithClusterAuth(h.clusterKubesGet))
	h.GET("/webapi/sites/:site/pods", h.WithClusterAuth(h.clusterKubePodsGet))

	// Github connector handlers
	h.GET("/webapi/github/login/web", h.WithRedirect(h.githubLoginWeb))
	h.GET("/webapi/github/callback", h.WithMetaRedirect(h.githubCallback))
	h.POST("/webapi/github/login/console", h.WithLimiter(h.githubLoginConsole))

	// MFA public endpoints.
	h.POST("/webapi/sites/:site/mfa/required", h.WithClusterAuth(h.isMFARequired))
	h.POST("/webapi/mfa/login/begin", h.WithLimiter(h.mfaLoginBegin))
	h.POST("/webapi/mfa/login/finish", httplib.MakeHandler(h.mfaLoginFinish))
	h.POST("/webapi/mfa/login/finishsession", httplib.MakeHandler(h.mfaLoginFinishSession))
	h.DELETE("/webapi/mfa/token/:token/devices/:devicename", httplib.MakeHandler(h.deleteMFADeviceWithTokenHandle))
	h.GET("/webapi/mfa/token/:token/devices", httplib.MakeHandler(h.getMFADevicesWithTokenHandle))
	h.POST("/webapi/mfa/token/:token/authenticatechallenge", httplib.MakeHandler(h.createAuthenticateChallengeWithTokenHandle))
	h.POST("/webapi/mfa/token/:token/registerchallenge", httplib.MakeHandler(h.createRegisterChallengeWithTokenHandle))

	// MFA private endpoints.
	h.GET("/webapi/mfa/devices", h.WithAuth(h.getMFADevicesHandle))
	h.POST("/webapi/mfa/authenticatechallenge", h.WithAuth(h.createAuthenticateChallengeHandle))
	h.POST("/webapi/mfa/devices", h.WithAuth(h.addMFADeviceHandle))
	h.POST("/webapi/mfa/authenticatechallenge/password", h.WithAuth(h.createAuthenticateChallengeWithPassword))

	// trusted clusters
	h.POST("/webapi/trustedclusters/validate", httplib.MakeHandler(h.validateTrustedCluster))

	// User Status (used by client to check if user session is valid)
	h.GET("/webapi/user/status", h.WithAuth(h.getUserStatus))

	h.GET("/webapi/roles", h.WithAuth(h.getRolesHandle))
	h.POST("/webapi/roles", h.WithAuth(h.upsertRoleHandle))
	h.PUT("/webapi/roles/:name", h.WithAuth(h.upsertRoleHandle))
	h.DELETE("/webapi/roles/:name", h.WithAuth(h.deleteRole))

	h.GET("/webapi/github", h.WithAuth(h.getGithubConnectorsHandle))
	h.POST("/webapi/github", h.WithAuth(h.upsertGithubConnectorHandle))
	h.PUT("/webapi/github/:name", h.WithAuth(h.upsertGithubConnectorHandle))
	h.DELETE("/webapi/github/:name", h.WithAuth(h.deleteGithubConnector))

	h.GET("/webapi/trustedcluster", h.WithAuth(h.getTrustedClustersHandle))
	h.POST("/webapi/trustedcluster", h.WithAuth(h.upsertTrustedClusterHandle))
	h.PUT("/webapi/trustedcluster/:name", h.WithAuth(h.upsertTrustedClusterHandle))
	h.DELETE("/webapi/trustedcluster/:name", h.WithAuth(h.deleteTrustedCluster))

	h.GET("/webapi/apps/:fqdnHint", h.WithAuth(h.getAppFQDN))
	h.GET("/webapi/apps/:fqdnHint/:clusterName/:publicAddr", h.WithAuth(h.getAppFQDN))

	// Desktop access endpoints.
	h.GET("/webapi/sites/:site/desktops", h.WithClusterAuth(h.clusterDesktopsGet))
	h.GET("/webapi/sites/:site/desktopservices", h.WithClusterAuth(h.clusterDesktopServicesGet))
	h.GET("/webapi/sites/:site/desktops/:desktopName", h.WithClusterAuth(h.getDesktopHandle))
	// GET /webapi/sites/:site/desktops/:desktopName/connect?access_token=<bearer_token>&username=<username>&width=<width>&height=<height>
	h.GET("/webapi/sites/:site/desktops/:desktopName/connect", h.WithClusterAuth(h.desktopConnectHandle))
	// GET /webapi/sites/:site/desktopplayback/:sid?access_token=<bearer_token>
	h.GET("/webapi/sites/:site/desktopplayback/:sid", h.WithClusterAuth(h.desktopPlaybackHandle))
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
	h.DELETE("/webapi/sites/:site/integrations/:name", h.WithClusterAuth(h.integrationsDelete))

	// AWS OIDC Integration Actions
	h.POST("/webapi/sites/:site/integrations/aws-oidc/:name/databases", h.WithClusterAuth(h.awsOIDCListDatabases))

	// AWS OIDC Integration specific endpoints:
	// Unauthenticated access to OpenID Configuration - used for AWS OIDC IdP integration
	h.GET("/.well-known/openid-configuration", h.WithLimiter(h.openidConfiguration))
	h.GET(OIDCJWKWURI, h.WithLimiter(h.jwksOIDC))
	h.GET("/webapi/thumbprint", h.WithLimiter(h.thumbprint))

	// Connection upgrades.
	h.GET("/webapi/connectionupgrade", httplib.MakeHandler(h.connectionUpgrade))

	// create user events.
	h.POST("/webapi/precapture", h.WithLimiter(h.createPreUserEventHandle))
	// create authenticated user events.
	h.POST("/webapi/capture", h.WithAuth(h.createUserEventHandle))

	h.GET("/webapi/headless/:headless_authentication_id", h.WithAuth(h.getHeadless))
	h.PUT("/webapi/headless/:headless_authentication_id", h.WithAuth(h.putHeadlessState))

	h.GET("/webapi/sites/:site/user-groups", h.WithClusterAuth(h.getUserGroups))

	// WebSocket endpoint for the chat conversation
	h.GET("/webapi/sites/:site/assistant", h.WithClusterAuth(h.assistant))

	// Sets the title for the conversation.
	h.POST("/webapi/assistant/conversations/:conversation_id/title", h.WithAuth(h.setAssistantTitle))
	h.POST("/webapi/assistant/title/summary", h.WithAuth(h.generateAssistantTitle))

	// Creates a new conversation - the conversation ID is returned in the response.
	h.POST("/webapi/assistant/conversations", h.WithAuth(h.createAssistantConversation))

	// Returns all conversations for the given user.
	h.GET("/webapi/assistant/conversations", h.WithAuth(h.getAssistantConversations))

	// Returns all messages in the given conversation.
	h.GET("/webapi/assistant/conversations/:conversation_id", h.WithAuth(h.getAssistantConversationByID))

	// Allows executing an arbitrary command on multiple nodes.
	h.GET("/webapi/command/:site/execute", h.WithClusterAuth(h.executeCommand))
}

// GetProxyClient returns authenticated auth server client
func (h *Handler) GetProxyClient() auth.ClientI {
	return h.cfg.ProxyClient
}

// GetAccessPoint returns the caching access point.
func (h *Handler) GetAccessPoint() auth.ProxyAccessPoint {
	return h.cfg.AccessPoint
}

// Close closes associated session cache operations
func (h *Handler) Close() error {
	return h.auth.Close()
}

func (h *Handler) getUserStatus(w http.ResponseWriter, r *http.Request, _ httprouter.Params, c *SessionContext) (interface{}, error) {
	return OK(), nil
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
func (h *Handler) getUserContext(w http.ResponseWriter, r *http.Request, p httprouter.Params, c *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
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

	user, err := clt.GetUser(c.GetUser(), false)
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

	userContext, err := ui.NewUserContext(user, accessChecker.Roles(), h.ClusterFeatures, desktopRecordingEnabled)
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
	}

	userContext.Cluster, err = ui.GetClusterDetails(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	userContext.ConsumedAccessRequestID = c.cfg.Session.GetConsumedAccessRequestID()

	return userContext, nil
}

// PublicProxyAddr returns the publicly advertised proxy address
func (h *Handler) PublicProxyAddr() string {
	return h.cfg.PublicProxyAddr
}

func localSettings(cap types.AuthPreference) (webclient.AuthenticationSettings, error) {
	as := webclient.AuthenticationSettings{
		Type:                constants.Local,
		SecondFactor:        cap.GetSecondFactor(),
		PreferredLocalMFA:   cap.GetPreferredLocalMFA(),
		AllowPasswordless:   cap.GetAllowPasswordless(),
		AllowHeadless:       cap.GetAllowHeadless(),
		Local:               &webclient.LocalSettings{},
		PrivateKeyPolicy:    cap.GetPrivateKeyPolicy(),
		DeviceTrustDisabled: deviceTrustDisabled(cap),
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
		log.WithError(err).Warnf("Error reading U2F settings")
	}

	// Webauthn settings.
	switch webConfig, err := cap.GetWebauthn(); {
	case err == nil:
		as.Webauthn = &webclient.Webauthn{
			RPID: webConfig.RPID,
		}
	case !trace.IsNotFound(err):
		log.WithError(err).Warnf("Error reading WebAuthn settings")
	}

	return as, nil
}

func oidcSettings(connector types.OIDCConnector, cap types.AuthPreference) webclient.AuthenticationSettings {
	return webclient.AuthenticationSettings{
		Type: constants.OIDC,
		OIDC: &webclient.OIDCSettings{
			Name:    connector.GetName(),
			Display: connector.GetDisplay(),
		},
		// Local fallback / MFA.
		SecondFactor:        cap.GetSecondFactor(),
		PreferredLocalMFA:   cap.GetPreferredLocalMFA(),
		PrivateKeyPolicy:    cap.GetPrivateKeyPolicy(),
		DeviceTrustDisabled: deviceTrustDisabled(cap),
	}
}

func samlSettings(connector types.SAMLConnector, cap types.AuthPreference) webclient.AuthenticationSettings {
	return webclient.AuthenticationSettings{
		Type: constants.SAML,
		SAML: &webclient.SAMLSettings{
			Name:    connector.GetName(),
			Display: connector.GetDisplay(),
		},
		// Local fallback / MFA.
		SecondFactor:        cap.GetSecondFactor(),
		PreferredLocalMFA:   cap.GetPreferredLocalMFA(),
		PrivateKeyPolicy:    cap.GetPrivateKeyPolicy(),
		DeviceTrustDisabled: deviceTrustDisabled(cap),
	}
}

func githubSettings(connector types.GithubConnector, cap types.AuthPreference) webclient.AuthenticationSettings {
	return webclient.AuthenticationSettings{
		Type: constants.Github,
		Github: &webclient.GithubSettings{
			Name:    connector.GetName(),
			Display: connector.GetDisplay(),
		},
		// Local fallback / MFA.
		SecondFactor:        cap.GetSecondFactor(),
		PreferredLocalMFA:   cap.GetPreferredLocalMFA(),
		PrivateKeyPolicy:    cap.GetPrivateKeyPolicy(),
		DeviceTrustDisabled: deviceTrustDisabled(cap),
	}
}

// deviceTrustDisabled is used to set its namesake field in
// [webclient.PingResponse.Auth].
func deviceTrustDisabled(cap types.AuthPreference) bool {
	return dtconfig.GetEffectiveMode(cap.GetDeviceTrust()) == constants.DeviceTrustModeOff
}

func getAuthSettings(ctx context.Context, authClient auth.ClientI) (webclient.AuthenticationSettings, error) {
	authPreference, err := authClient.GetAuthPreference(ctx)
	if err != nil {
		return webclient.AuthenticationSettings{}, trace.Wrap(err)
	}

	var as webclient.AuthenticationSettings

	switch authPreference.GetType() {
	case constants.Local:
		as, err = localSettings(authPreference)
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

	return as, nil
}

// traces forwards spans from the web ui to the upstream collector configured for the proxy. If tracing is
// disabled then the forwarding is a noop.
func (h *Handler) traces(w http.ResponseWriter, r *http.Request, _ httprouter.Params, _ *SessionContext) (interface{}, error) {
	body, err := io.ReadAll(io.LimitReader(r.Body, teleport.MaxHTTPRequestSize))
	if err != nil {
		h.log.WithError(err).Error("Failed to read traces request")
		w.WriteHeader(http.StatusBadRequest)
		return nil, nil
	}

	if err := r.Body.Close(); err != nil {
		h.log.WithError(err).Warn("Failed to close traces request body")
	}

	var data tracepb.TracesData
	if err := protojson.Unmarshal(body, &data); err != nil {
		h.log.WithError(err).Error("Failed to unmarshal traces request")
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
		if err := h.cfg.TraceClient.UploadTraces(r.Context(), data.ResourceSpans); err != nil {
			h.log.WithError(err).Error("Failed to upload traces")
		}
	}()

	w.WriteHeader(http.StatusOK)
	return nil, nil
}

func (h *Handler) ping(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var err error
	authSettings, err := getAuthSettings(r.Context(), h.cfg.ProxyClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	proxyConfig, err := h.cfg.ProxySettings.GetProxySettings(r.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO(jakule): This part should be removed once the plugin support is added to OSS.
	if proxyConfig.AssistEnabled {
		// TODO(jakule): Currently assist is disabled when per-session MFA is enabled as this part is not implemented.
		authPreference, err := h.cfg.ProxyClient.GetAuthPreference(r.Context())
		if err != nil {
			return webclient.AuthenticationSettings{}, trace.Wrap(err)
		}
		mfaRequired := authPreference.GetRequireMFAType() != types.RequireMFAType_OFF
		enabled, err := h.cfg.ProxyClient.IsAssistEnabled(r.Context())
		if err != nil {
			return webclient.AuthenticationSettings{}, trace.Wrap(err)
		}

		// disable if per-session MFA is enabled and it's ok by the auth
		proxyConfig.AssistEnabled = enabled.Enabled && !mfaRequired
	}

	pr, err := h.cfg.ProxyClient.Ping(r.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return webclient.PingResponse{
		Auth:              authSettings,
		Proxy:             *proxyConfig,
		ServerVersion:     teleport.Version,
		MinClientVersion:  teleport.MinClientVersion,
		ClusterName:       h.auth.clusterName,
		AutomaticUpgrades: pr.ServerFeatures.GetAutomaticUpgrades(),
	}, nil
}

func (h *Handler) find(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	proxyConfig, err := h.cfg.ProxySettings.GetProxySettings(r.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return webclient.PingResponse{
		Proxy:            *proxyConfig,
		ServerVersion:    teleport.Version,
		MinClientVersion: teleport.MinClientVersion,
		ClusterName:      h.auth.clusterName,
	}, nil
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
		Proxy:         *proxyConfig,
		ServerVersion: teleport.Version,
		ClusterName:   h.auth.clusterName,
	}

	hasMessageOfTheDay := cap.GetMessageOfTheDay() != ""
	if slices.Contains(constants.SystemConnectors, connectorName) {
		response.Auth, err = localSettings(cap)
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
		h.log.WithError(err).Error("Cannot retrieve OIDC connectors.")
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
		h.log.WithError(err).Error("Cannot retrieve SAML connectors.")
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
		h.log.WithError(err).Error("Cannot retrieve GitHub connectors.")
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
		h.log.WithError(err).Error("Cannot retrieve AuthPreferences.")
		authSettings = webclient.WebConfigAuthSettings{
			Providers:        authProviders,
			SecondFactor:     constants.SecondFactorOff,
			LocalAuthEnabled: true,
			AuthType:         constants.Local,
		}
	} else {
		authType := cap.GetType()
		var localConnectorName string

		if authType == constants.Local {
			localConnectorName = cap.GetConnectorName()
		}

		authSettings = webclient.WebConfigAuthSettings{
			Providers:          authProviders,
			SecondFactor:       cap.GetSecondFactor(),
			LocalAuthEnabled:   cap.GetAllowLocalAuth(),
			AllowPasswordless:  cap.GetAllowPasswordless(),
			AuthType:           authType,
			PreferredLocalMFA:  cap.GetPreferredLocalMFA(),
			LocalConnectorName: localConnectorName,
			PrivateKeyPolicy:   cap.GetPrivateKeyPolicy(),
			MOTD:               cap.GetMessageOfTheDay(),
		}
	}

	clusterFeatures := h.ClusterFeatures
	// ping server to get cluster features since h.ClusterFeatures may be stale
	pingResponse, err := h.GetProxyClient().Ping(r.Context())
	if err != nil {
		h.log.WithError(err).Warn("Cannot retrieve cluster features, client may receive stale features")
	} else {
		clusterFeatures = *pingResponse.ServerFeatures
	}

	// get tunnel address to display on cloud instances
	tunnelPublicAddr := ""
	assistEnabled := false // TODO(jakule) remove when plugins are implemented
	proxyConfig, err := h.cfg.ProxySettings.GetProxySettings(r.Context())
	if err != nil {
		h.log.WithError(err).Warn("Cannot retrieve ProxySettings, tunnel address won't be set in Web UI.")
	} else {
		if clusterFeatures.GetCloud() {
			tunnelPublicAddr = proxyConfig.SSH.TunnelPublicAddr
		}
		// TODO(jakule): This part should be removed once the plugin support is added to OSS.
		if proxyConfig.AssistEnabled {
			enabled, err := h.cfg.ProxyClient.IsAssistEnabled(r.Context())
			if err != nil {
				return nil, trace.Wrap(err)
			}

			// disable if auth doesn't support assist
			assistEnabled = enabled.Enabled
		}
	}

	// disable joining sessions if proxy session recording is enabled
	canJoinSessions := true
	recCfg, err := h.cfg.ProxyClient.GetSessionRecordingConfig(r.Context())
	if err != nil {
		h.log.WithError(err).Error("Cannot retrieve SessionRecordingConfig.")
	} else {
		canJoinSessions = !services.IsRecordAtProxy(recCfg.GetMode())
	}

	webCfg := webclient.WebConfig{
		Auth:                 authSettings,
		CanJoinSessions:      canJoinSessions,
		IsCloud:              clusterFeatures.GetCloud(),
		TunnelPublicAddress:  tunnelPublicAddr,
		RecoveryCodesEnabled: clusterFeatures.GetRecoveryCodes(),
		UI:                   h.getUIConfig(r.Context()),
		IsDashboard:          isDashboard(clusterFeatures),
		IsUsageBasedBilling:  clusterFeatures.GetIsUsageBased(),
		AutomaticUpgrades:    clusterFeatures.GetAutomaticUpgrades(),
		AssistEnabled:        assistEnabled,
	}

	resource, err := h.cfg.ProxyClient.GetClusterName()
	if err != nil {
		h.log.WithError(err).Warn("Failed to query cluster name.")
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

type JWKSResponse struct {
	// Keys is a list of public keys in JWK format.
	Keys []jwt.JWK `json:"keys"`
}

// getUiConfig will first try to get an ui_config set in the cache and then
// return what was set by the file config. Returns nil if neither are set which
// is fine, as the web UI can set its own defaults.
func (h *Handler) getUIConfig(ctx context.Context) webclient.UIConfig {
	// In v13+ this will be called from the proxy local cache on `AccessPoint.GetUIConfig`.
	// https://github.com/gravitational/teleport/pull/22097#discussion_r1117445223
	if uiConfig, err := h.cfg.ProxyClient.GetUIConfig(ctx); err == nil && uiConfig != nil {
		return webclient.UIConfig{
			ScrollbackLines: int(uiConfig.GetScrollbackLines()),
		}
	}
	return h.cfg.UI
}

// jwks returns all public keys used to sign JWT tokens for this cluster.
func (h *Handler) jwks(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	clusterName, err := h.cfg.ProxyClient.GetDomainName(r.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Fetch the JWT public keys only.
	ca, err := h.cfg.ProxyClient.GetCertAuthority(r.Context(), types.CertAuthID{
		Type:       types.JWTSigner,
		DomainName: clusterName,
	}, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pairs := ca.GetTrustedJWTKeyPairs()

	// Create response and allocate space for the keys.
	var resp JWKSResponse
	resp.Keys = make([]jwt.JWK, 0, len(pairs))

	// Loop over and all add public keys in JWK format.
	for _, pair := range pairs {
		jwk, err := jwt.MarshalJWK(pair.PublicKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		resp.Keys = append(resp.Keys, jwk)
	}
	return &resp, nil
}

func (h *Handler) motd(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	authPrefs, err := h.cfg.ProxyClient.GetAuthPreference(r.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return webclient.MotD{Text: authPrefs.GetMessageOfTheDay()}, nil
}

func (h *Handler) githubLoginWeb(w http.ResponseWriter, r *http.Request, p httprouter.Params) string {
	logger := h.log.WithField("auth", "github")
	logger.Debug("Web login start.")

	req, err := ParseSSORequestParams(r)
	if err != nil {
		logger.WithError(err).Error("Failed to extract SSO parameters from request.")
		return client.LoginFailedRedirectURL
	}

	remoteAddr, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		logger.WithError(err).Error("Failed to parse request remote address.")
		return client.LoginFailedRedirectURL
	}

	response, err := h.cfg.ProxyClient.CreateGithubAuthRequest(r.Context(), types.GithubAuthRequest{
		CSRFToken:         req.CSRFToken,
		ConnectorID:       req.ConnectorID,
		CreateWebSession:  true,
		ClientRedirectURL: req.ClientRedirectURL,
		ClientLoginIP:     remoteAddr,
	})
	if err != nil {
		logger.WithError(err).Error("Error creating auth request.")
		return client.LoginFailedRedirectURL

	}

	return response.RedirectURL
}

func (h *Handler) githubLoginConsole(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	logger := h.log.WithField("auth", "github")
	logger.Debug("Console login start.")

	req := new(client.SSOLoginConsoleReq)
	if err := httplib.ReadJSON(r, req); err != nil {
		logger.WithError(err).Error("Error reading json.")
		return nil, trace.AccessDenied(SSOLoginFailureMessage)
	}

	if err := req.CheckAndSetDefaults(); err != nil {
		logger.WithError(err).Error("Missing request parameters.")
		return nil, trace.AccessDenied(SSOLoginFailureMessage)
	}

	remoteAddr, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		logger.WithError(err).Error("Failed to parse request remote address.")
		return nil, trace.AccessDenied(SSOLoginFailureMessage)
	}

	response, err := h.cfg.ProxyClient.CreateGithubAuthRequest(r.Context(), types.GithubAuthRequest{
		ConnectorID:          req.ConnectorID,
		PublicKey:            req.PublicKey,
		CertTTL:              req.CertTTL,
		ClientRedirectURL:    req.RedirectURL,
		Compatibility:        req.Compatibility,
		RouteToCluster:       req.RouteToCluster,
		KubernetesCluster:    req.KubernetesCluster,
		AttestationStatement: req.AttestationStatement.ToProto(),
		ClientLoginIP:        remoteAddr,
	})
	if err != nil {
		logger.WithError(err).Error("Failed to create GitHub auth request.")
		return nil, trace.AccessDenied(SSOLoginFailureMessage)
	}

	return &client.SSOLoginConsoleResponse{
		RedirectURL: response.RedirectURL,
	}, nil
}

func (h *Handler) githubCallback(w http.ResponseWriter, r *http.Request, p httprouter.Params) string {
	logger := h.log.WithField("auth", "github")
	logger.Debugf("Callback start: %v.", r.URL.Query())

	response, err := h.cfg.ProxyClient.ValidateGithubAuthCallback(r.Context(), r.URL.Query())
	if err != nil {
		logger.WithError(err).Error("Error while processing callback.")

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
		logger.Infof("Redirecting to web browser.")

		res := &SSOCallbackResponse{
			CSRFToken:         response.Req.CSRFToken,
			Username:          response.Username,
			SessionName:       response.Session.GetName(),
			ClientRedirectURL: response.Req.ClientRedirectURL,
		}

		if err := SSOSetWebSessionAndRedirectURL(w, r, res, true); err != nil {
			logger.WithError(err).Error("Error setting web session.")
			return client.LoginFailedRedirectURL
		}

		return res.ClientRedirectURL
	}

	logger.Infof("Callback is redirecting to console login.")
	if len(response.Req.PublicKey) == 0 {
		logger.Error("Not a web or console login request.")
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
		logger.WithError(err).Error("Error constructing ssh response")
		return client.LoginFailedRedirectURL
	}

	return redirectURL.String()
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
	teleportPackage := teleport.ComponentTeleport
	if modules.GetModules().BuildType() == modules.BuildEnterprise || feats.Cloud {
		teleportPackage = fmt.Sprintf("%s-%s", teleport.ComponentTeleport, modules.BuildEnterprise)
	}

	// By default, it uses the stable/v<majorVersion> channel.
	repoChannel := fmt.Sprintf("stable/%s", version)

	// However, when Automatic Upgrades are on and cluster is part of cloud,
	// it will use the stable/cloud channel.
	if feats.Cloud && feats.AutomaticUpgrades {
		repoChannel = stableCloudChannelRepo
	}

	tmpl := installers.Template{
		PublicProxyAddr: h.PublicProxyAddr(),
		MajorVersion:    version,
		TeleportPackage: teleportPackage,
		RepoChannel:     repoChannel,
		// teleport-upgrade is not available for v12.
		// Hard coding the following value to false for compatibility purposes
		// This ensures the Installer scripts and exposed variables stay the same as v13+.
		// If users install v13 and then downgrade to v12, the templating won't break.
		AutomaticUpgrades: "false",
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
}

// ConstructSSHResponse creates a special SSH response for SSH login method
// that encodes everything using the client's secret key
func ConstructSSHResponse(response AuthParams) (*url.URL, error) {
	u, err := url.Parse(response.ClientRedirectURL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	consoleResponse := auth.SSHLoginResponse{
		Username:    response.Username,
		Cert:        response.Cert,
		TLSCert:     response.TLSCert,
		HostSigners: auth.AuthoritiesToTrustedCerts(response.HostSigners),
	}
	out, err := json.Marshal(consoleResponse)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Extract secret out of the request. Look for both "secret" which is the
	// old format and "secret_key" which is the new fomat. If this is not done,
	// then users would have to update their callback URL in their identity
	// provider.
	values := u.Query()
	secretV1 := values.Get("secret")
	secretV2 := values.Get("secret_key")
	values.Set("secret", "")
	values.Set("secret_key", "")

	var ciphertext []byte

	switch {
	// AES-GCM based symmetric cipher.
	case secretV2 != "":
		key, err := secret.ParseKey([]byte(secretV2))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		ciphertext, err = key.Seal(out)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	// NaCl based symmetric cipher (legacy).
	case secretV1 != "":
		// If FIPS mode was requested, make sure older clients that use NaCl get rejected.
		if response.FIPS {
			return nil, trace.BadParameter("non-FIPS compliant encryption: NaCl, check " +
				"that tsh release was downloaded from a Teleport account https://teleport.sh")
		}

		secretKeyBytes, err := lemma_secret.EncodedStringToKey(secretV1)
		if err != nil {
			return nil, trace.BadParameter("bad secret")
		}
		encryptor, err := lemma_secret.New(&lemma_secret.Config{KeyBytes: secretKeyBytes})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		sealedBytes, err := encryptor.Seal(out)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		ciphertext, err = json.Marshal(sealedBytes)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	default:
		return nil, trace.BadParameter("missing secret")
	}

	// Place ciphertext into the response body.
	values.Set("response", string(ciphertext))

	u.RawQuery = values.Encode()
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
	// SessionExpires is when this session expires.
	SessionExpires time.Time `json:"sessionExpires,omitempty"`
	// SessionInactiveTimeoutMS specifies how long in milliseconds
	// a user WebUI session can be left idle before being logged out
	// by the server. A zero value means there is no idle timeout set.
	SessionInactiveTimeoutMS int `json:"sessionInactiveTimeout"`
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

	return &CreateSessionResponse{
		TokenType:                roundtrip.AuthBearer,
		Token:                    token.GetName(),
		TokenExpiresIn:           int(token.Expiry().Sub(sctx.cfg.Parent.clock.Now()) / time.Second),
		SessionInactiveTimeoutMS: int(sctx.cfg.Session.GetIdleTimeout().Milliseconds()),
	}, nil
}

// createWebSession creates a new web session based on user, pass and 2nd factor token
//
// POST /v1/webapi/sessions/web
//
// {"user": "alex", "pass": "abc123", "second_factor_token": "token", "second_factor_type": "totp"}
//
// # Response
//
// {"type": "bearer", "token": "bearer token", "user": {"name": "alex", "allowed_logins": ["admin", "bob"]}, "expires_in": 20}
func (h *Handler) createWebSession(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req *CreateSessionReq
	if err := httplib.ReadJSON(r, &req); err != nil {
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

	switch cap.GetSecondFactor() {
	case constants.SecondFactorOff:
		webSession, err = h.auth.AuthWithoutOTP(r.Context(), req.User, req.Pass, clientMeta)
	case constants.SecondFactorOTP, constants.SecondFactorOn:
		webSession, err = h.auth.AuthWithOTP(r.Context(), req.User, req.Pass, req.SecondFactorToken, clientMeta)
	case constants.SecondFactorOptional:
		if req.SecondFactorToken == "" {
			webSession, err = h.auth.AuthWithoutOTP(r.Context(), req.User, req.Pass, clientMeta)
		} else {
			webSession, err = h.auth.AuthWithOTP(r.Context(), req.User, req.Pass, req.SecondFactorToken, clientMeta)
		}
	default:
		return nil, trace.AccessDenied("unknown second factor type: %q", cap.GetSecondFactor())
	}
	if err != nil {
		h.log.WithError(err).Warnf("Access attempt denied for user %q.", req.User)
		// Since checking for private key policy meant that they passed authn,
		// return policy error as is to help direct user.
		if keys.IsPrivateKeyPolicyError(err) {
			return nil, trace.Wrap(err)
		}
		// Obscure all other errors.
		return nil, trace.AccessDenied("invalid credentials")
	}

	// Block and wait a few seconds for the session that was created to show up
	// in the cache. If this request is not blocked here, it can get stuck in a
	// racy session creation loop.
	err = h.waitForWebSession(r.Context(), types.GetWebSessionRequest{
		User:      req.User,
		SessionID: webSession.GetName(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := SetSessionCookie(w, req.User, webSession.GetName()); err != nil {
		return nil, trace.Wrap(err)
	}

	ctx, err := h.auth.newSessionContext(r.Context(), req.User, webSession.GetName())
	if err != nil {
		h.log.WithError(err).Warnf("Access attempt denied for user %q.", req.User)
		return nil, trace.AccessDenied("need auth")
	}

	res, err := newSessionResponse(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	res.SessionExpires = webSession.GetExpiryTime()

	return res, nil
}

func clientMetaFromReq(r *http.Request) *auth.ForwardedClientMetadata {
	// multiplexer handles extracting real client IP using PROXY protocol where
	// available, so we can omit checking X-Forwarded-For.
	return &auth.ForwardedClientMetadata{
		UserAgent:  r.UserAgent(),
		RemoteAddr: r.RemoteAddr,
	}
}

// deleteWebSession is called to sign out user
//
// DELETE /v1/webapi/sessions/:sid
//
// Response:
//
// {"message": "ok"}
func (h *Handler) deleteWebSession(w http.ResponseWriter, r *http.Request, _ httprouter.Params, ctx *SessionContext) (interface{}, error) {
	err := h.logout(r.Context(), w, ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return OK(), nil
}

func (h *Handler) logout(ctx context.Context, w http.ResponseWriter, sctx *SessionContext) error {
	if err := sctx.Invalidate(ctx); err != nil {
		return trace.Wrap(err)
	}
	ClearSession(w)

	return nil
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
	if err := httplib.ReadJSON(r, &req); err != nil {
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
	if err := SetSessionCookie(w, newSession.GetUser(), newSession.GetName()); err != nil {
		return nil, trace.Wrap(err)
	}

	res, err := newSessionResponse(newContext)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	res.SessionExpires = newSession.GetExpiryTime()

	return res, nil
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
	WebauthnCreationResponse *wanlib.CredentialCreationResponse `json:"webauthnCreationResponse"`
}

func (h *Handler) changeUserAuthentication(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req changeUserAuthenticationRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
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
				Webauthn: wanlib.CredentialCreationResponseToProto(req.WebauthnCreationResponse),
			},
		}
	case req.SecondFactorToken != "":
		protoReq.NewMFARegisterResponse = &proto.MFARegisterResponse{Response: &proto.MFARegisterResponse_TOTP{
			TOTP: &proto.TOTPRegisterResponse{Code: req.SecondFactorToken},
		}}
	}

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
	ctx, err := h.auth.newSessionContext(r.Context(), sess.GetUser(), sess.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = h.trySettingConnectorNameToPasswordless(r.Context(), ctx, req)
	if err != nil {
		h.log.WithError(err).Error("Failed to set passwordless as connector name.")
	}

	if err := SetSessionCookie(w, sess.GetUser(), sess.GetName()); err != nil {
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

// trySettingConnectorNameToPasswordless sets cluster_auth_preference connectorName to `passwordless` when the first cloud user chooses passwordless as the authentication method.
// This simplifies UX for cloud users, as they will not need to select a passwordless connector when logging in.
func (h *Handler) trySettingConnectorNameToPasswordless(ctx context.Context, sessCtx *SessionContext, req changeUserAuthenticationRequest) error {
	// We use the presence of a WebAuthn response, along with the absence of a
	// password, as a proxy to determine that a passwordless registration took
	// place, as it is not possible to infer that just from the WebAuthn response.
	isPasswordlessRegistration := req.WebauthnCreationResponse != nil && len(req.Password) == 0
	if !isPasswordlessRegistration {
		return nil
	}

	if !h.ClusterFeatures.GetCloud() {
		return nil
	}

	authPreference, err := sessCtx.cfg.RootClient.GetAuthPreference(ctx)
	if err != nil {
		return nil
	}

	if connector := authPreference.GetConnectorName(); connector != "" && connector != constants.LocalConnector {
		return nil
	}

	users, err := h.cfg.ProxyClient.GetUsers(false)
	if err != nil {
		return trace.Wrap(err)
	}

	if len(users) != 1 {
		return nil
	}

	authPreference.SetConnectorName(constants.PasswordlessConnector)

	err = sessCtx.cfg.RootClient.SetAuthPreference(ctx, authPreference)
	return trace.Wrap(err)
}

// createResetPasswordToken allows a UI user to reset a user's password.
// This handler is also required for after creating new users.
func (h *Handler) createResetPasswordToken(w http.ResponseWriter, r *http.Request, _ httprouter.Params, ctx *SessionContext) (interface{}, error) {
	var req auth.CreateUserTokenRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	token, err := clt.CreateResetPasswordToken(r.Context(),
		auth.CreateUserTokenRequest{
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
	result, err := h.getResetPasswordToken(context.TODO(), p.ByName("token"))
	if err != nil {
		h.log.WithError(err).Warn("Failed to fetch a reset password token.")
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
// {"user": "alex", "pass": "abc123"}
// {"passwordless": true}
//
// Successful response:
//
// {"webauthn_challenge": {...}, "totp_challenge": true}
// {"webauthn_challenge": {...}} // passwordless
func (h *Handler) mfaLoginBegin(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req *client.MFAChallengeRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	mfaReq := &proto.CreateAuthenticateChallengeRequest{}
	if req.Passwordless {
		mfaReq.Request = &proto.CreateAuthenticateChallengeRequest_Passwordless{
			Passwordless: &proto.Passwordless{},
		}
	} else {
		mfaReq.Request = &proto.CreateAuthenticateChallengeRequest_UserCredentials{
			UserCredentials: &proto.UserCredentials{
				Username: req.User,
				Password: []byte(req.Pass),
			},
		}
	}

	mfaChallenge, err := h.auth.proxyClient.CreateAuthenticateChallenge(r.Context(), mfaReq)
	if err != nil {
		return nil, trace.AccessDenied("invalid credentials")
	}
	return client.MakeAuthenticateChallenge(mfaChallenge), nil
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
	if err := httplib.ReadJSON(r, &req); err != nil {
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
	if err != nil {
		// Since checking for private key policy meant that they passed authn,
		// return policy error as is to help direct user.
		if keys.IsPrivateKeyPolicyError(err) {
			return nil, trace.Wrap(err)
		}
		// Obscure all other errors.
		return nil, trace.AccessDenied("invalid credentials")
	}

	// Fetch user from session, user is empty for passwordless requests.
	user := session.GetUser()
	if err := SetSessionCookie(w, user, session.GetName()); err != nil {
		return nil, trace.Wrap(err)
	}

	ctx, err := h.auth.newSessionContext(r.Context(), user, session.GetName())
	if err != nil {
		return nil, trace.AccessDenied("need auth")
	}

	return newSessionResponse(ctx)
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
	remoteClusters, err := clt.GetRemoteClusters()
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
func (h *Handler) getSiteNamespaces(w http.ResponseWriter, r *http.Request, _ httprouter.Params, c *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	clt, err := site.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	namespaces, err := clt.GetNamespaces()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return getSiteNamespacesResponse{
		Namespaces: namespaces,
	}, nil
}

// clusterNodesGet returns a list of nodes for a given cluster site.
func (h *Handler) clusterNodesGet(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	// Get a client to the Auth Server with the logged in user's identity. The
	// identity of the logged in user is used to fetch the list of nodes.
	clt, err := sctx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := listResources(clt, r, types.KindNode)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	servers, err := types.ResourcesWithLabels(resp.Resources).AsServers()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessChecker, err := sctx.GetUserAccessChecker()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	uiServers, err := ui.MakeServers(site.GetName(), servers, accessChecker.Roles())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return listResourcesGetResponse{
		Items:      uiServers,
		StartKey:   resp.NextKey,
		TotalCount: resp.TotalCount,
	}, nil
}

type getLoginAlertsResponse struct {
	Alerts []types.ClusterAlert `json:"alerts"`
}

// clusterLoginAlertsGet returns a list of on-login alerts for the user.
func (h *Handler) clusterLoginAlertsGet(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
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

// createIdentityContext creates a srv.IdentityContext from the ssh cert of the user
// stored within the SessionContext.
func createIdentityContext(login string, sessionCtx *SessionContext) (srv.IdentityContext, error) {
	accessChecker, err := sessionCtx.GetUserAccessChecker()
	if err != nil {
		return srv.IdentityContext{}, trace.Wrap(err)
	}

	sshCert, err := sessionCtx.GetSSHCertificate()
	if err != nil {
		return srv.IdentityContext{}, trace.Wrap(err)
	}

	unmappedRoles, err := services.ExtractRolesFromCert(sshCert)
	if err != nil {
		return srv.IdentityContext{}, trace.Wrap(err)
	}

	accessRequestIDs, err := srv.ParseAccessRequestIDs(sshCert.Extensions[teleport.CertExtensionTeleportActiveRequests])
	if err != nil {
		return srv.IdentityContext{}, trace.Wrap(err)
	}

	return srv.IdentityContext{
		AccessChecker:  accessChecker,
		TeleportUser:   sessionCtx.GetUser(),
		Login:          login,
		Certificate:    sshCert,
		UnmappedRoles:  unmappedRoles,
		ActiveRequests: accessRequestIDs,
		Impersonator:   sshCert.Extensions[teleport.CertExtensionImpersonator],
	}, nil
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
	site reversetunnel.RemoteSite,
) (interface{}, error) {
	q := r.URL.Query()
	params := q.Get("params")
	if params == "" {
		return nil, trace.BadParameter("missing params")
	}
	var req *TerminalRequest
	if err := json.Unmarshal([]byte(params), &req); err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := sessionCtx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	identity, err := createIdentityContext(req.Login, sessionCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ctx, err := h.cfg.SessionControl.AcquireSessionContext(r.Context(), identity, h.cfg.ProxyWebAddr.Addr, r.RemoteAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var sessionData session.Session
	var displayLogin string

	clusterName := site.GetName()

	if req.SessionID.IsZero() {
		// An existing session ID was not provided so we need to create a new one.
		sessionData, err = h.generateSession(ctx, clt, req, clusterName, sessionCtx)
		if err != nil {
			h.log.WithError(err).Debug("Unable to generate new ssh session.")
			return nil, trace.Wrap(err)
		}
	} else {
		sessionData, displayLogin, err = h.fetchExistingSession(ctx, clt, req, clusterName)
		if err != nil {
			return nil, trace.Wrap(err)
		}
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

	h.log.Debugf("New terminal request for server=%s, login=%s, sid=%s, websid=%s.",
		req.Server, req.Login, sessionData.ID, sessionCtx.GetSessionID())

	keepAliveInterval := req.KeepAliveInterval
	// Try to use the keep alive interval from the request.
	// When it's not set or below a second, use the cluster's keep alive interval.
	if keepAliveInterval < time.Second {
		authAccessPoint, err := site.CachingAccessPoint()
		if err != nil {
			h.log.Debugf("Unable to get auth access point: %v", err)
			return nil, trace.Wrap(err)
		}

		netConfig, err := authAccessPoint.GetClusterNetworkingConfig(ctx)
		if err != nil {
			h.log.WithError(err).Debug("Unable to fetch cluster networking config.")
			return nil, trace.Wrap(err)
		}

		keepAliveInterval = netConfig.GetKeepAliveInterval()
	}

	terminalConfig := TerminalHandlerConfig{
		Term:               req.Term,
		SessionCtx:         sessionCtx,
		AuthProvider:       clt,
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
	}

	term, err := NewTerminal(ctx, terminalConfig)
	if err != nil {
		h.log.WithError(err).Error("Unable to create terminal.")
		return nil, trace.Wrap(err)
	}

	h.userConns.Add(1)
	defer h.userConns.Add(-1)

	// start the websocket session with a web-based terminal:
	h.log.Infof("Getting terminal to %#v.", req)
	httplib.MakeTracingHandler(term, teleport.ComponentProxy).ServeHTTP(w, r)

	return nil, nil
}

func (h *Handler) generateSession(ctx context.Context, clt auth.ClientI, req *TerminalRequest, clusterName string, scx *SessionContext) (session.Session, error) {
	owner := scx.cfg.User
	h.log.Infof("Generating new session for %s\n", clusterName)

	host, err := findByHost(ctx, clt, req.Server)
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
		ServerID:       host.id,
		ClusterName:    clusterName,
		ServerHostname: host.hostName,
		ServerHostPort: host.port,
		Moderated:      accessEvaluator.IsModerated(),
		ID:             session.NewID(),
		Created:        time.Now().UTC(),
		LastActive:     time.Now().UTC(),
		Namespace:      apidefaults.Namespace,
		Owner:          owner,
	}, nil
}

func (h *Handler) generateCommandSession(host *hostInfo, login, clusterName, owner string) (session.Session, error) {
	h.log.Infof("Generating new session for %s in %s\n", host.hostName, clusterName)

	return session.Session{
		Login:          login,
		ServerID:       host.id,
		ClusterName:    clusterName,
		ServerHostname: host.hostName,
		ServerHostPort: host.port,
		ID:             session.NewID(),
		Created:        time.Now().UTC(),
		LastActive:     time.Now().UTC(),
		Namespace:      apidefaults.Namespace,
		Owner:          owner,
	}, nil
}

// hostInfo is a helper struct used to store host information.
type hostInfo struct {
	id       string
	hostName string
	port     int
}

// findByQuery returns all hosts matching the given query/predicate.
// The query is a predicate expression that can be used to filter hosts.
func findByQuery(ctx context.Context, clt auth.ClientI, query string) ([]hostInfo, error) {
	resources, err := apiclient.GetResourcesWithFilters(ctx, clt, proto.ListResourcesRequest{
		ResourceType:        types.KindNode,
		Namespace:           apidefaults.Namespace,
		PredicateExpression: query,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	hosts := make([]hostInfo, 0, len(resources))
	for _, resource := range resources {
		server, ok := resource.(types.Server)
		if !ok {
			return nil, trace.BadParameter("expected types.Server, got: %T", resource)
		}

		h := hostInfo{
			hostName: server.GetHostname(),
			id:       server.GetName(),
			port:     defaultPort,
		}
		hosts = append(hosts, h)
	}

	return hosts, nil
}

// findByHost return a host matching by the host name.
func findByHost(ctx context.Context, clt auth.ClientI, serverName string) (*hostInfo, error) {
	var host hostInfo

	if _, err := uuid.Parse(serverName); err != nil {
		// The requested server is either a hostname or an address. Get all
		// servers that may fuzzily match by populating SearchKeywords
		resources, err := apiclient.GetResourcesWithFilters(ctx, clt, proto.ListResourcesRequest{
			ResourceType:   types.KindNode,
			Namespace:      apidefaults.Namespace,
			SearchKeywords: []string{serverName},
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if len(resources) == 0 {
			// If we didn't find the resource set host and port,
			// so we can try direct dial.
			host.hostName, host.port, err = serverHostPort(serverName)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			host.id = host.hostName
		}

		matches := 0
		for _, resource := range resources {
			server, ok := resource.(types.Server)
			if !ok {
				return nil, trace.BadParameter("expected types.Server, got: %T", resource)
			}

			// match by hostname
			if server.GetHostname() == serverName {
				if matches > 0 {
					matches++
					continue
				}

				host.hostName = server.GetHostname()
				host.id = server.GetName()
				host.port = 0

				matches++
				continue
			}

			// exact match by address
			if server.GetAddr() == serverName {
				if matches > 0 {
					matches++
					continue
				}

				host.hostName = serverName
				host.id = server.GetName()
				host.port = 0

				matches++
				continue
			}
		}

		// there was either at least one partial match or multiple
		// exact matches on the server. connect with the resolved
		// host and port of the requested server.
		if matches > 1 || host.hostName == "" && host.id == "" {
			host.hostName, host.port, err = serverHostPort(serverName)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			host.id = serverName
		}
	} else {
		// Even though the UUID was provided and can be dialed directly, the UI
		// requires the hostname to populate the title of the session window.
		// Looking the node up directly by UUID is the most efficient we can be until
		// the UI is modified to remember the hostname when the connect button is
		// used to establish a session.
		server, err := clt.GetNode(ctx, apidefaults.Namespace, serverName)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		host.hostName = server.GetHostname()
		host.port = 0
		host.id = serverName
	}
	return &host, nil
}

// fetchExistingSession fetches an active or pending SSH session by the SessionID passed in the TerminalRequest.
func (h *Handler) fetchExistingSession(ctx context.Context, clt auth.ClientI, req *TerminalRequest, siteName string) (session.Session, string, error) {
	sessionID, err := session.ParseID(req.SessionID.String())
	if err != nil {
		return session.Session{}, "", trace.Wrap(err)
	}
	h.log.Infof("Attempting to join existing session: %s\n", sessionID)

	tracker, err := clt.GetSessionTracker(ctx, string(*sessionID))
	if err != nil {
		return session.Session{}, "", trace.Wrap(err)
	}

	if tracker.GetSessionKind() != types.SSHSessionKind || tracker.GetState() == types.SessionState_SessionStateTerminated {
		return session.Session{}, "", trace.NotFound("SSH session %v not found", sessionID)
	}

	sessionData := trackerToLegacySession(tracker, siteName)
	// When joining an existing session use the specially handled
	// `SSHSessionJoinPrincipal` login instead of the provided login so that
	// users are able to join sessions without having permissions to create
	// new ones themselves for auditing purposes. Otherwise, the user would
	// fail the SSH lib username validation step.
	sessionData.Login = teleport.SSHSessionJoinPrincipal
	// Using the Login above will then display `-teleport-internal-join` as the
	// username that the user is logging in as, so we need to instead show the
	// session username in the UI.
	displayLogin := tracker.GetLogin()

	return sessionData, displayLogin, nil
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
	}
}

// clusterActiveAndPendingSessionsGet gets the list of active and pending sessions for a site.
//
// GET /v1/webapi/sites/:site/sessions
func (h *Handler) clusterActiveAndPendingSessionsGet(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
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

const maxStreamBytes = 5 * 1024 * 1024

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
func (h *Handler) clusterSearchEvents(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	values := r.URL.Query()

	var eventTypes []string
	if include := values.Get("include"); include != "" {
		eventTypes = strings.Split(include, ",")
	}

	searchEvents := func(clt auth.ClientI, from, to time.Time, limit int, order types.EventOrder, startKey string) ([]apievents.AuditEvent, string, error) {
		return clt.SearchEvents(from, to, apidefaults.Namespace, eventTypes, limit, order, startKey)
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
func (h *Handler) clusterSearchSessionEvents(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	searchSessionEvents := func(clt auth.ClientI, from, to time.Time, limit int, order types.EventOrder, startKey string) ([]apievents.AuditEvent, string, error) {
		return clt.SearchSessionEvents(from, to, limit, order, startKey, nil, "")
	}
	return clusterEventsList(r.Context(), sctx, site, r.URL.Query(), searchSessionEvents)
}

// clusterEventsList returns a list of audit events obtained using the provided
// searchEvents method.
func clusterEventsList(ctx context.Context, sctx *SessionContext, site reversetunnel.RemoteSite, values url.Values, searchEvents func(clt auth.ClientI, from, to time.Time, limit int, order types.EventOrder, startKey string) ([]apievents.AuditEvent, string, error)) (interface{}, error) {
	from, err := queryTime(values, "from", time.Now().UTC().AddDate(0, -1, 0))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	to, err := queryTime(values, "to", time.Now().UTC())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	limit, err := queryLimit(values, "limit", defaults.EventsIterationLimit)
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

// queryLimit returns the limit parameter with the specified name from the
// query string.
//
// If there's no such parameter, specified default limit is returned.
func queryLimit(query url.Values, name string, def int) (int, error) {
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
func queryLimitAsInt32(query url.Values, name string, def int32) (int32, error) {
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

// siteSessionStreamGet returns a byte array from a session's stream
//
// GET /v1/webapi/sites/:site/namespaces/:namespace/sessions/:sid/stream?query
//
// Query parameters:
//
//	"offset"   : bytes from the beginning
//	"bytes"    : number of bytes to read (it won't return more than 512Kb)
//
// Unlike other request handlers, this one does not return JSON.
// It returns the binary stream unencoded, directly in the respose body,
// with Content-Type of application/octet-stream, gzipped with up to 95%
// compression ratio.
func (h *Handler) siteSessionStreamGet(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	httplib.SetNoCacheHeaders(w.Header())

	onError := func(err error) {
		h.log.WithError(err).Debug("Unable to retrieve session chunk.")
		http.Error(w, err.Error(), trace.ErrorToCode(err))
	}

	// authenticate first
	sctx, site, err := h.authenticateRequestWithCluster(w, r, p)
	if err != nil {
		onError(trace.Wrap(err))
		return
	}

	// get the session
	sid, err := session.ParseID(p.ByName("sid"))
	if err != nil {
		onError(trace.Wrap(err))
		return
	}
	clt, err := sctx.GetUserClient(r.Context(), site)
	if err != nil {
		onError(trace.Wrap(err))
		return
	}

	// look at 'offset' parameter
	query := r.URL.Query()
	offset, _ := strconv.Atoi(query.Get("offset"))
	if err != nil {
		onError(trace.Wrap(err))
		return
	}
	max, err := strconv.Atoi(query.Get("bytes"))
	if err != nil || max <= 0 {
		max = maxStreamBytes
	}
	if max > maxStreamBytes {
		max = maxStreamBytes
	}

	// call the site API to get the chunk:
	bytes, err := clt.GetSessionChunk(apidefaults.Namespace, *sid, offset, max)
	if err != nil {
		onError(trace.Wrap(err))
		return
	}
	// see if we can gzip it:
	var writer io.Writer = w
	for _, acceptedEnc := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		if strings.TrimSpace(acceptedEnc) == "gzip" {
			gzipper := gzip.NewWriter(w)
			writer = gzipper
			defer gzipper.Close()
			w.Header().Set("Content-Encoding", "gzip")
		}
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	_, err = writer.Write(bytes)
	if err != nil {
		onError(trace.Wrap(err))
		return
	}
}

type eventsListGetResponse struct {
	// Events is list of events retrieved.
	Events []events.EventFields `json:"events"`
	// StartKey is the position to resume search events.
	StartKey string `json:"startKey"`
}

// siteSessionEventsGet gets the site session by id
//
// GET /v1/webapi/sites/:site/namespaces/:namespace/sessions/:sid/events?after=N
//
// Query:
//
//	"after" : cursor value of an event to return "newer than" events
//	          good for repeated polling
//
// Response body (each event is an arbitrary JSON structure)
//
// {"events": [{...}, {...}, ...}
func (h *Handler) siteSessionEventsGet(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	sessionID, err := session.ParseID(p.ByName("sid"))
	if err != nil {
		return nil, trace.BadParameter("invalid session ID %q", p.ByName("sid"))
	}

	clt, err := sctx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	afterN, err := strconv.Atoi(r.URL.Query().Get("after"))
	if err != nil {
		afterN = 0
	}

	e, err := clt.GetSessionEvents(apidefaults.Namespace, *sessionID, afterN, true)
	if err != nil {
		h.log.WithError(err).Debugf("Unable to find events for session %v.", sessionID)
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("unable to find events for session %q", sessionID)
		}

		return nil, trace.Wrap(err)
	}
	return eventsListGetResponse{Events: e}, nil
}

// hostCredentials sends a registration token and metadata to the Auth Server
// and gets back SSH and TLS certificates.
func (h *Handler) hostCredentials(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req types.RegisterUsingTokenRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	authClient := h.cfg.ProxyClient
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
func (h *Handler) createSSHCert(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req client.CreateSSHCertReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	authClient := h.cfg.ProxyClient

	cap, err := authClient.GetAuthPreference(r.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authSSHUserReq := auth.AuthenticateSSHRequest{
		AuthenticateUserRequest: auth.AuthenticateUserRequest{
			Username:       req.User,
			PublicKey:      req.PubKey,
			ClientMetadata: clientMetaFromReq(r),
		},
		CompatibilityMode:    req.Compatibility,
		TTL:                  req.TTL,
		RouteToCluster:       req.RouteToCluster,
		KubernetesCluster:    req.KubernetesCluster,
		AttestationStatement: req.AttestationStatement,
	}

	if req.HeadlessAuthenticationID != "" {
		// We need to use the default callback timeout rather than the standard client timeout.
		// However, authClient is shared across all Proxy->Auth requests, so we need to create
		// a new client to avoid applying the callback timeout to other concurrent requests. To
		// this end, we create a clone of the HTTP Client with the desired timeout instead.
		httpClient, err := authClient.CloneHTTPClient(
			auth.ClientParamTimeout(defaults.CallbackTimeout),
			auth.ClientParamResponseHeaderTimeout(defaults.CallbackTimeout),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		authSSHUserReq.AuthenticateUserRequest.HeadlessAuthenticationID = req.HeadlessAuthenticationID
		loginResp, err := httpClient.AuthenticateSSHUser(r.Context(), authSSHUserReq)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return loginResp, nil
	}

	switch cap.GetSecondFactor() {
	case constants.SecondFactorOff:
		authSSHUserReq.AuthenticateUserRequest.Pass = &auth.PassCreds{
			Password: []byte(req.Password),
		}
	case constants.SecondFactorOTP, constants.SecondFactorOn, constants.SecondFactorOptional:
		authSSHUserReq.AuthenticateUserRequest.OTP = &auth.OTPCreds{
			Password: []byte(req.Password),
			Token:    req.OTPToken,
		}
	default:
		return nil, trace.AccessDenied("unsupported second factor type: %q", cap.GetSecondFactor())
	}

	loginResp, err := authClient.AuthenticateSSHUser(r.Context(), authSSHUserReq)
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
	var validateRequestRaw auth.ValidateTrustedClusterRequestRaw
	if err := httplib.ReadJSON(r, &validateRequestRaw); err != nil {
		return nil, trace.Wrap(err)
	}

	validateRequest, err := validateRequestRaw.ToNative()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	validateResponse, err := h.auth.ValidateTrustedCluster(r.Context(), validateRequest)
	if err != nil {
		h.log.WithError(err).Error("Failed validating trusted cluster")
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
type ClusterHandler func(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error)

// WithClusterAuth wraps a ClusterHandler to ensure that a request is authenticated to this proxy
// (the same as WithAuth), as well as to grab the remoteSite (which can represent this local cluster
// or a remote trusted cluster) as specified by the ":site" url parameter.
func (h *Handler) WithClusterAuth(fn ClusterHandler) httprouter.Handle {
	return httplib.MakeHandler(func(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
		sctx, site, err := h.authenticateRequestWithCluster(w, r, p)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return fn(w, r, p, sctx, site)
	})
}

// authenticateRequestWithCluster ensures that a request is authenticated
// to this proxy, returning the *SessionContext (same as AuthenticateRequest),
// and also grabs the remoteSite (which can represent this local cluster or a
// remote trusted cluster) as specified by the ":site" url parameter.
func (h *Handler) authenticateRequestWithCluster(w http.ResponseWriter, r *http.Request, p httprouter.Params) (*SessionContext, reversetunnel.RemoteSite, error) {
	sctx, err := h.AuthenticateRequest(w, r, true)
	if err != nil {
		h.log.WithError(err).Warn("Failed to authenticate.")
		return nil, nil, trace.Wrap(err)
	}

	site, err := h.getSiteByParams(sctx, p)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return sctx, site, nil
}

// getSiteByParams gets the remoteSite (which can represent this local cluster or a
// remote trusted cluster) as specified by the ":site" url parameter.
func (h *Handler) getSiteByParams(sctx *SessionContext, p httprouter.Params) (reversetunnel.RemoteSite, error) {
	clusterName := p.ByName("site")
	if clusterName == currentSiteShortcut {
		res, err := h.cfg.ProxyClient.GetClusterName()
		if err != nil {
			h.log.WithError(err).Warn("Failed to query cluster name.")
			return nil, trace.Wrap(err)
		}
		clusterName = res.GetClusterName()
	}

	site, err := h.getSiteByClusterName(sctx, clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return site, nil
}

func (h *Handler) getSiteByClusterName(ctx *SessionContext, clusterName string) (reversetunnel.RemoteSite, error) {
	proxy, err := h.ProxyWithRoles(ctx)
	if err != nil {
		h.log.WithError(err).Warn("Failed to get proxy with roles.")
		return nil, trace.Wrap(err)
	}

	site, err := proxy.GetSite(clusterName)
	if err != nil {
		h.log.WithError(err).WithField("cluster-name", clusterName).Warn("Failed to query site.")
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
	UserClientForCluster(ctx context.Context, clusterName string) (auth.ClientI, error)
}

type clusterClientProvider struct {
	h   *Handler
	ctx *SessionContext
}

// UserClientForCluster returns a client to the local or remote cluster
// identified by clusterName and is authenticated with the identity of the user.
func (r clusterClientProvider) UserClientForCluster(ctx context.Context, clusterName string) (auth.ClientI, error) {
	site, err := r.h.getSiteByClusterName(r.ctx, clusterName)
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
type ProvisionTokenHandler func(w http.ResponseWriter, r *http.Request, p httprouter.Params, site reversetunnel.RemoteSite, token types.ProvisionToken) (interface{}, error)

// WithProvisionTokenAuth ensures that request is authenticated with a provision token.
// Provision tokens, when used like this are invalidated as soon as used.
// Doesn't matter if the underlying response was a success or an error.
func (h *Handler) WithProvisionTokenAuth(fn ProvisionTokenHandler) httprouter.Handle {
	return httplib.MakeHandler(func(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
		ctx := r.Context()
		logger := h.log.WithField("request", fmt.Sprintf("%v %v", r.Method, r.URL.Path))

		creds, err := roundtrip.ParseAuthHeaders(r)
		if err != nil {
			logger.WithError(err).Warn("No auth headers.")
			return nil, trace.AccessDenied("need auth")
		}

		token, err := h.consumeTokenForAPICall(ctx, creds.Password)
		if err != nil {
			h.log.WithError(err).Warn("Failed to authenticate.")
			return nil, trace.AccessDenied("need auth")
		}

		site, err := h.cfg.Proxy.GetSite(h.auth.clusterName)
		if err != nil {
			h.log.WithError(err).WithField("cluster-name", h.auth.clusterName).Warn("Failed to query cluster.")
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
func (h *Handler) consumeTokenForAPICall(ctx context.Context, tokenName string) (types.ProvisionToken, error) {
	token, err := h.GetProxyClient().GetToken(ctx, tokenName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := h.GetProxyClient().DeleteToken(ctx, token.GetName()); err != nil {
		return nil, trace.Wrap(err)
	}

	return token, nil
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
			h.log.WithError(err).Warn("Failed to issue a redirect.")
		}
	}
}

// WithAuth ensures that a request is authenticated.
func (h *Handler) WithAuth(fn ContextHandler) httprouter.Handle {
	return httplib.MakeHandler(func(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
		sctx, err := h.AuthenticateRequest(w, r, true)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return fn(w, r, p, sctx)
	})
}

// WithAuthCookieAndCSRF ensures that a request is authenticated
// for plain old non-AJAX requests (does not check the Bearer header).
// It enforces CSRF checks (except for "safe" methods).
func (h *Handler) WithAuthCookieAndCSRF(fn ContextHandler) httprouter.Handle {
	f := func(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
		sctx, err := h.AuthenticateRequest(w, r, false)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return fn(w, r, p, sctx)
	}

	return httplib.WithCSRFProtection(f)
}

// WithLimiter adds IP-based rate limiting to fn.
func (h *Handler) WithLimiter(fn httplib.HandlerFunc) httprouter.Handle {
	return httplib.MakeHandler(func(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
		return h.WithLimiterHandlerFunc(fn)(w, r, p)
	})
}

// WithLimiterHandlerFunc adds IP-based rate limiting to a HandlerFunc. This
// should be used when you need to nest this inside another HandlerFunc.
func (h *Handler) WithLimiterHandlerFunc(fn httplib.HandlerFunc) httplib.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
		remote, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		err = h.limiter.RegisterRequest(remote, nil /* customRate */)
		// MaxRateError doesn't play well with errors.Is, hence the cast.
		if _, ok := err.(*ratelimit.MaxRateError); ok {
			return nil, trace.LimitExceeded(err.Error())
		}
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return fn(w, r, p)
	}
}

// AuthenticateRequest authenticates request using combination of a session cookie
// and bearer token
func (h *Handler) AuthenticateRequest(w http.ResponseWriter, r *http.Request, checkBearerToken bool) (*SessionContext, error) {
	const missingCookieMsg = "missing session cookie"
	logger := h.log.WithField("request", fmt.Sprintf("%v %v", r.Method, r.URL.Path))
	cookie, err := r.Cookie(CookieName)
	if err != nil || (cookie != nil && cookie.Value == "") {
		if err != nil {
			logger.Warn(err)
		}
		return nil, trace.AccessDenied(missingCookieMsg)
	}
	decodedCookie, err := DecodeCookie(cookie.Value)
	if err != nil {
		logger.WithError(err).Warn("Failed to decode cookie.")
		return nil, trace.AccessDenied("failed to decode cookie")
	}
	ctx, err := h.auth.getOrCreateSession(r.Context(), decodedCookie.User, decodedCookie.SID)
	if err != nil {
		logger.WithError(err).Warn("Invalid session.")
		ClearSession(w)
		return nil, trace.AccessDenied("need auth")
	}
	if checkBearerToken {
		creds, err := roundtrip.ParseAuthHeaders(r)
		if err != nil {
			logger.WithError(err).Warn("No auth headers.")
			return nil, trace.AccessDenied("need auth")
		}
		if err := ctx.validateBearerToken(r.Context(), creds.Password); err != nil {
			logger.WithError(err).Warn("Request failed: bad bearer token.")
			return nil, trace.AccessDenied("bad bearer token")
		}
	}
	return ctx, nil
}

// ProxyWithRoles returns a reverse tunnel proxy verifying the permissions
// of the given user.
func (h *Handler) ProxyWithRoles(ctx *SessionContext) (reversetunnel.Tunnel, error) {
	accessChecker, err := ctx.GetUserAccessChecker()
	if err != nil {
		h.log.WithError(err).Warn("Failed to get client roles.")
		return nil, trace.Wrap(err)
	}
	return reversetunnel.NewTunnelWithRoles(h.cfg.Proxy, accessChecker, h.cfg.AccessPoint), nil
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
	// CSRFToken is the token in the CSRF cookie header.
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
}

// SSOSetWebSessionAndRedirectURL validates the CSRF token in the response
// against that in the request, validates that the callback URL in the response
// can be parsed, and sets a session cookie with the username and session name
// from the response. On success, nil is returned. If the validation fails, an
// error is returned.
func SSOSetWebSessionAndRedirectURL(w http.ResponseWriter, r *http.Request, response *SSOCallbackResponse, verifyCSRF bool) error {
	if verifyCSRF {
		if err := csrf.VerifyToken(response.CSRFToken, r); err != nil {
			return trace.Wrap(err)
		}
	}

	if err := SetSessionCookie(w, response.Username, response.SessionName); err != nil {
		return trace.Wrap(err)
	}

	parsedURL, err := url.Parse(response.ClientRedirectURL)
	if err != nil {
		return trace.Wrap(err)
	}

	response.ClientRedirectURL = parsedURL.RequestURI()

	return nil
}

// authExportPublic returns the CA Certs that can be used to set up a chain of trust which includes the current Teleport Cluster
//
// GET /webapi/sites/:site/auth/export?type=<auth type>
// GET /webapi/auth/export?type=<auth type>
func (h *Handler) authExportPublic(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	authorities, err := client.ExportAuthorities(
		r.Context(),
		h.GetProxyClient(),
		client.ExportAuthoritiesRequest{
			AuthType: r.URL.Query().Get("type"),
		},
	)
	if err != nil {
		h.log.WithError(err).Debug("Failed to generate CA Certs.")
		http.Error(w, err.Error(), trace.ErrorToCode(err))
		return
	}

	reader := strings.NewReader(authorities)

	// ServeContent sets the correct headers: Content-Type, Content-Length and Accept-Ranges.
	// It also handles the Range negotiation
	http.ServeContent(w, r, "authorized_hosts.txt", time.Now(), reader)
}

// isDashboard returns a bool indicating if the cluster is a
// dashboard cluster.
// Dashboard is a cluster running on cloud infrastructure that
// isn't a Teleport Cloud cluster
func isDashboard(features proto.Features) bool {
	// TODO(matheus): for now, we assume dashboard based on
	// the presence of recovery codes, which are never enabled
	// in OSS or self-hosted Teleport.
	return !features.GetCloud() && features.GetRecoveryCodes()
}
