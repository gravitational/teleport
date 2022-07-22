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
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	apiutils "github.com/gravitational/teleport/api/utils"
	apisshutils "github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/httplib/csrf"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/observability/tracing"
	"github.com/gravitational/teleport/lib/plugin"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/secret"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/web/app"
	"github.com/gravitational/teleport/lib/web/ui"

	"github.com/gravitational/oxy/ratelimit"
	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"

	"github.com/jonboulle/clockwork"
	"github.com/julienschmidt/httprouter"
	lemma_secret "github.com/mailgun/lemma/secret"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

const (
	// ssoLoginConsoleErr is a generic error message to hide revealing sso login failure msgs.
	ssoLoginConsoleErr = "Failed to login. Please check Teleport's log for more details."
	metaRedirectHTML   = `
<!DOCTYPE html>
<html lang="en">
	<head>
		<title>Teleport Redirection Service</title>
		<meta http-equiv="cache-control" content="no-cache"/>
		<meta http-equiv="refresh" content="0;URL='{{.}}'" />
	</head>
	<body></body>
</html>
`
)

var metaRedirectTemplate = template.Must(template.New("meta-redirect").Parse(metaRedirectHTML))

// Handler is HTTP web proxy handler
type Handler struct {
	log logrus.FieldLogger

	sync.Mutex
	httprouter.Router
	cfg                     Config
	auth                    *sessionCache
	sessionStreamPollPeriod time.Duration
	clock                   clockwork.Clock
	// sshPort specifies the SSH proxy port extracted
	// from configuration
	sshPort string

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

	// cachedSessionLingeringThreshold specifies the time the session will linger
	// in the cache before getting purged after it has expired.
	// Defaults to cachedSessionLingeringThreshold if unspecified.
	cachedSessionLingeringThreshold *time.Duration

	// ClusterFeatures contains flags for supported/unsupported features.
	ClusterFeatures proto.Features

	// ProxySettings allows fetching the current proxy settings.
	ProxySettings proxySettingsGetter
	TenantUrl     string
}

type APIHandler struct {
	handler *Handler

	// appHandler is a http.Handler to forward requests to applications.
	appHandler *app.Handler
}

// Check if this request should be forwarded to an application handler to
// be handled by the UI and handle the request appropriately.
func (h *APIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// If the request is either to the fragment authentication endpoint or if the
	// request is already authenticated (has a session cookie), forward to
	// application handlers. If the request is unauthenticated and requesting a
	// FQDN that is not of the proxy, redirect to application launcher.
	if app.HasFragment(r) || app.HasSession(r) || app.HasClientCert(r) {
		h.appHandler.ServeHTTP(w, r)
		return
	}
	if redir, ok := app.HasName(r, h.handler.cfg.ProxyPublicAddrs); ok {
		http.Redirect(w, r, redir, http.StatusFound)
		return
	}
	// Serve the Web UI.
	h.handler.ServeHTTP(w, r)
}

func (h *APIHandler) Close() error {
	return h.handler.Close()
}

// NewHandler returns a new instance of web proxy handler
func NewHandler(cfg Config, opts ...HandlerOption) (*APIHandler, error) {
	const apiPrefix = "/" + teleport.WebAPIVersion
	h := &Handler{
		cfg:             cfg,
		log:             newPackageLogger(),
		clock:           clockwork.NewRealClock(),
		ClusterFeatures: cfg.ClusterFeatures,
	}

	// for properly handling url-encoded parameter values.
	h.UseRawPath = true

	for _, o := range opts {
		if err := o(h); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	sessionLingeringThreshold := cachedSessionLingeringThreshold
	if cfg.cachedSessionLingeringThreshold != nil {
		sessionLingeringThreshold = *cfg.cachedSessionLingeringThreshold
	}

	auth, err := newSessionCache(sessionCacheOptions{
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
	h.auth = auth

	_, sshPort, err := net.SplitHostPort(cfg.ProxySSHAddr.String())
	if err != nil {
		h.log.WithError(err).Warnf("Invalid SSH proxy address %q, will use default port %v.",
			cfg.ProxySSHAddr.String(), defaults.SSHProxyListenPort)
		sshPort = strconv.Itoa(defaults.SSHProxyListenPort)
	}
	h.sshPort = sshPort

	// challengeLimiter is used to limit unauthenticated challenge generation for
	// passwordless.
	challengeLimiter, err := limiter.NewRateLimiter(limiter.Config{
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

	// ping endpoint is used to check if the server is up. the /webapi/ping
	// endpoint returns the default authentication method and configuration that
	// the server supports. the /webapi/ping/:connector endpoint can be used to
	// query the authentication configuration for a specific connector.
	h.GET("/webapi/ping", httplib.MakeHandler(h.ping))
	h.GET("/webapi/ping/:connector", httplib.MakeHandler(h.pingWithConnector))
	// find is like ping, but is faster because it is optimized for servers
	// and does not fetch the data that servers don't need, e.g.
	// OIDC connectors and auth preferences
	h.GET("/webapi/find", httplib.MakeHandler(h.find))

	// Unauthenticated access to JWT public keys.
	h.GET("/.well-known/jwks.json", httplib.MakeHandler(h.jwks))

	// Unauthenticated access to the message of the day
	h.GET("/webapi/motd", httplib.MakeHandler(h.motd))

	// DELETE IN: 5.1.0
	//
	// Migrated this endpoint to /webapi/sessions/web below.
	h.POST("/webapi/sessions", httplib.WithCSRFProtection(h.createWebSession))

	// Web sessions
	h.POST("/webapi/sessions/web", httplib.WithCSRFProtection(h.createWebSession))
	h.POST("/webapi/sessions/app", h.WithAuth(h.createAppSession))
	h.DELETE("/webapi/sessions", h.WithAuth(h.deleteSession))
	h.POST("/webapi/sessions/renew", h.WithAuth(h.renewSession))
	// idemeum session
	h.POST("/webapi/sessions/idemeum", httplib.MakeHandler(h.createSessionForIdemeumServiceToken))
	h.POST("/webapi/users", h.WithAuth(h.createUserHandle))
	h.PUT("/webapi/users", h.WithAuth(h.updateUserHandle))
	h.GET("/webapi/users", h.WithAuth(h.getUsersHandle))
	h.DELETE("/webapi/users/:username", h.WithAuth(h.deleteUserHandle))

	// We have an overlap route here, please see godoc of handleGetUserOrResetToken
	//h.GET("/webapi/users/:username", h.WithAuth(h.getUserHandle))
	//h.GET("/webapi/users/password/token/:token", httplib.MakeHandler(h.getResetPasswordTokenHandle))
	h.GET("/webapi/users/*wildcard", h.handleGetUserOrResetToken)

	h.PUT("/webapi/users/password/token", httplib.WithCSRFProtection(h.changeUserAuthentication))
	h.PUT("/webapi/users/password", h.WithAuth(h.changePassword))
	h.POST("/webapi/users/password/token", h.WithAuth(h.createResetPasswordToken))
	h.POST("/webapi/users/privilege/token", h.WithAuth(h.createPrivilegeTokenHandle))

	// Issues SSH temp certificates based on 2FA access creds
	h.POST("/webapi/ssh/certs", httplib.MakeHandler(h.createSSHCert))

	// list available sites
	h.GET("/webapi/sites", h.WithAuth(h.getClusters))

	// Site specific API

	// get namespaces
	h.GET("/webapi/sites/:site/namespaces", h.WithClusterAuth(h.getSiteNamespaces))

	// get nodes
	h.GET("/webapi/sites/:site/nodes", h.WithClusterAuth(h.clusterNodesGet))

	// Get applications.
	h.GET("/webapi/sites/:site/apps", h.WithClusterAuth(h.clusterAppsGet))

	// active sessions handlers
	h.GET("/webapi/sites/:site/connect", h.WithClusterAuth(h.siteNodeConnect))       // connect to an active session (via websocket)
	h.GET("/webapi/sites/:site/sessions", h.WithClusterAuth(h.siteSessionsGet))      // get active list of sessions
	h.POST("/webapi/sites/:site/sessions", h.WithClusterAuth(h.siteSessionGenerate)) // create active session metadata
	h.GET("/webapi/sites/:site/sessions/:sid", h.WithClusterAuth(h.siteSessionGet))  // get active session metadata

	// Audit events handlers.
	h.GET("/webapi/sites/:site/events/search", h.WithClusterAuth(h.clusterSearchEvents))                 // search site events
	h.GET("/webapi/sites/:site/events/search/sessions", h.WithClusterAuth(h.clusterSearchSessionEvents)) // search site session events
	h.GET("/webapi/sites/:site/sessions/:sid/events", h.WithClusterAuth(h.siteSessionEventsGet))         // get recorded session's timing information (from events)
	h.GET("/webapi/sites/:site/sessions/:sid/stream", h.siteSessionStreamGet)                            // get recorded session's bytes (from events)

	// scp file transfer
	h.GET("/webapi/sites/:site/nodes/:server/:login/scp", h.WithClusterAuth(h.transferFile))
	h.POST("/webapi/sites/:site/nodes/:server/:login/scp", h.WithClusterAuth(h.transferFile))

	// token generation
	h.POST("/webapi/token", h.WithAuth(h.createTokenHandle))

	// add Node token generation
	// DELETE IN 11.0. Deprecated, use /webapi/token for generating tokens of any role.
	h.POST("/webapi/nodes/token", h.WithAuth(h.createNodeTokenHandle))
	// join scripts
	h.GET("/scripts/:token/install-node.sh", httplib.MakeHandler(h.getNodeJoinScriptHandle))
	h.GET("/scripts/:token/install-app.sh", httplib.MakeHandler(h.getAppJoinScriptHandle))
	// web context
	h.GET("/webapi/sites/:site/context", h.WithClusterAuth(h.getUserContext))

	// Database access handlers.
	h.GET("/webapi/sites/:site/databases", h.WithClusterAuth(h.clusterDatabasesGet))

	// Kube access handlers.
	h.GET("/webapi/sites/:site/kubernetes", h.WithClusterAuth(h.clusterKubesGet))

	// OIDC related callback handlers
	h.GET("/webapi/oidc/login/web", h.WithRedirect(h.oidcLoginWeb))
	h.GET("/webapi/oidc/callback", h.WithMetaRedirect(h.oidcCallback))
	h.POST("/webapi/oidc/login/console", httplib.MakeHandler(h.oidcLoginConsole))

	// SAML 2.0 handlers
	h.POST("/webapi/saml/acs", h.WithRedirect(h.samlACS))
	h.GET("/webapi/saml/sso", h.WithMetaRedirect(h.samlSSO))
	h.POST("/webapi/saml/login/console", httplib.MakeHandler(h.samlSSOConsole))

	// Github connector handlers
	h.GET("/webapi/github/login/web", h.WithRedirect(h.githubLoginWeb))
	h.GET("/webapi/github/callback", h.WithMetaRedirect(h.githubCallback))
	h.POST("/webapi/github/login/console", httplib.MakeHandler(h.githubLoginConsole))

	// MFA public endpoints.
	h.POST("/webapi/mfa/login/begin", h.withLimiter(challengeLimiter, h.mfaLoginBegin))
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

	// Issue host credentials.
	h.POST("/webapi/host/credentials", httplib.MakeHandler(h.hostCredentials))

	h.GET("/webapi/roles", h.WithAuth(h.getRolesHandle))
	h.PUT("/webapi/roles", h.WithAuth(h.upsertRoleHandle))
	h.POST("/webapi/roles", h.WithAuth(h.upsertRoleHandle))
	h.DELETE("/webapi/roles/:name", h.WithAuth(h.deleteRole))

	h.GET("/webapi/github", h.WithAuth(h.getGithubConnectorsHandle))
	h.PUT("/webapi/github", h.WithAuth(h.upsertGithubConnectorHandle))
	h.POST("/webapi/github", h.WithAuth(h.upsertGithubConnectorHandle))
	h.DELETE("/webapi/github/:name", h.WithAuth(h.deleteGithubConnector))

	h.GET("/webapi/trustedcluster", h.WithAuth(h.getTrustedClustersHandle))
	h.PUT("/webapi/trustedcluster", h.WithAuth(h.upsertTrustedClusterHandle))
	h.POST("/webapi/trustedcluster", h.WithAuth(h.upsertTrustedClusterHandle))
	h.DELETE("/webapi/trustedcluster/:name", h.WithAuth(h.deleteTrustedCluster))

	h.GET("/webapi/apps/:fqdnHint", h.WithAuth(h.getAppFQDN))
	h.GET("/webapi/apps/:fqdnHint/:clusterName/:publicAddr", h.WithAuth(h.getAppFQDN))

	// Desktop access endpoints.
	h.GET("/webapi/sites/:site/desktops", h.WithClusterAuth(h.clusterDesktopsGet))
	h.GET("/webapi/sites/:site/desktops/:desktopName", h.WithClusterAuth(h.getDesktopHandle))
	// GET /webapi/sites/:site/desktops/:desktopName/connect?access_token=<bearer_token>&username=<username>&width=<width>&height=<height>
	h.GET("/webapi/sites/:site/desktops/:desktopName/connect", h.WithClusterAuth(h.desktopConnectHandle))
	// GET /webapi/sites/:site/desktopplayback/:sid?access_token=<bearer_token>
	h.GET("/webapi/sites/:site/desktopplayback/:sid", h.WithAuth(h.desktopPlaybackHandle))

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
			http.Redirect(w, r, "/web", http.StatusFound)
			return
		}

		// serve Web UI:
		if strings.HasPrefix(r.URL.Path, "/web/app") {
			httplib.SetStaticFileHeaders(w.Header())
			http.StripPrefix("/web", makeGzipHandler(http.FileServer(cfg.StaticFS))).ServeHTTP(w, r)
		} else if strings.HasPrefix(r.URL.Path, "/web/") || r.URL.Path == "/web" {
			csrfToken, err := csrf.AddCSRFProtection(w, r)
			if err != nil {
				h.log.WithError(err).Warn("Failed to generate CSRF token.")
			}

			session := struct {
				Session string
				XCSRF   string
			}{
				XCSRF: csrfToken,
			}

			ctx, err := h.AuthenticateRequest(w, r, false)
			if err == nil {
				resp, err := newSessionResponse(ctx)
				if err == nil {
					out, err := json.Marshal(resp)
					if err == nil {
						session.Session = base64.StdEncoding.EncodeToString(out)
					}
				} else {
					h.log.WithError(err).Debug("Could not authenticate.")
				}
			}
			httplib.SetIndexHTMLHeaders(w.Header())
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
	appHandler, err := app.NewHandler(cfg.Context, &app.HandlerConfig{
		Clock:         h.clock,
		AuthClient:    cfg.ProxyClient,
		AccessPoint:   cfg.AccessPoint,
		ProxyClient:   cfg.Proxy,
		CipherSuites:  cfg.CipherSuites,
		WebPublicAddr: resp.SSH.PublicAddr,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &APIHandler{
		handler:    h,
		appHandler: appHandler,
	}, nil
}

// GetProxyClient returns authenticated auth server client
func (h *Handler) GetProxyClient() auth.ClientI {
	return h.cfg.ProxyClient
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
//
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
	recConfig, err := c.unsafeCachedAuthClient.GetSessionRecordingConfig(r.Context())
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

	return userContext, nil
}

func localSettings(cap types.AuthPreference) (webclient.AuthenticationSettings, error) {
	as := webclient.AuthenticationSettings{
		Type:              constants.Local,
		SecondFactor:      cap.GetSecondFactor(),
		PreferredLocalMFA: cap.GetPreferredLocalMFA(),
		AllowPasswordless: cap.GetAllowPasswordless(),
		Local:             &webclient.LocalSettings{},
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
		SecondFactor:      cap.GetSecondFactor(),
		PreferredLocalMFA: cap.GetPreferredLocalMFA(),
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
		SecondFactor:      cap.GetSecondFactor(),
		PreferredLocalMFA: cap.GetPreferredLocalMFA(),
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
		SecondFactor:      cap.GetSecondFactor(),
		PreferredLocalMFA: cap.GetPreferredLocalMFA(),
	}
}

func getAuthSettings(ctx context.Context, authClient auth.ClientI) (webclient.AuthenticationSettings, error) {
	cap, err := authClient.GetAuthPreference(ctx)
	if err != nil {
		return webclient.AuthenticationSettings{}, trace.Wrap(err)
	}

	var as webclient.AuthenticationSettings

	switch cap.GetType() {
	case constants.Local:
		as, err = localSettings(cap)
		if err != nil {
			return webclient.AuthenticationSettings{}, trace.Wrap(err)
		}
	case constants.OIDC:
		if cap.GetConnectorName() != "" {
			oidcConnector, err := authClient.GetOIDCConnector(ctx, cap.GetConnectorName(), false)
			if err != nil {
				return webclient.AuthenticationSettings{}, trace.Wrap(err)
			}

			as = oidcSettings(oidcConnector, cap)
		} else {
			oidcConnectors, err := authClient.GetOIDCConnectors(ctx, false)
			if err != nil {
				return webclient.AuthenticationSettings{}, trace.Wrap(err)
			}
			if len(oidcConnectors) == 0 {
				return webclient.AuthenticationSettings{}, trace.BadParameter("no oidc connectors found")
			}

			as = oidcSettings(oidcConnectors[0], cap)
		}
	case constants.SAML:
		if cap.GetConnectorName() != "" {
			samlConnector, err := authClient.GetSAMLConnector(ctx, cap.GetConnectorName(), false)
			if err != nil {
				return webclient.AuthenticationSettings{}, trace.Wrap(err)
			}

			as = samlSettings(samlConnector, cap)
		} else {
			samlConnectors, err := authClient.GetSAMLConnectors(ctx, false)
			if err != nil {
				return webclient.AuthenticationSettings{}, trace.Wrap(err)
			}
			if len(samlConnectors) == 0 {
				return webclient.AuthenticationSettings{}, trace.BadParameter("no saml connectors found")
			}

			as = samlSettings(samlConnectors[0], cap)
		}
	case constants.Github:
		if cap.GetConnectorName() != "" {
			githubConnector, err := authClient.GetGithubConnector(ctx, cap.GetConnectorName(), false)
			if err != nil {
				return webclient.AuthenticationSettings{}, trace.Wrap(err)
			}
			as = githubSettings(githubConnector, cap)
		} else {
			githubConnectors, err := authClient.GetGithubConnectors(ctx, false)
			if err != nil {
				return webclient.AuthenticationSettings{}, trace.Wrap(err)
			}
			if len(githubConnectors) == 0 {
				return webclient.AuthenticationSettings{}, trace.BadParameter("no github connectors found")
			}
			as = githubSettings(githubConnectors[0], cap)
		}
	default:
		return webclient.AuthenticationSettings{}, trace.BadParameter("unknown type %v", cap.GetType())
	}

	as.HasMessageOfTheDay = cap.GetMessageOfTheDay() != ""

	return as, nil
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

	return webclient.PingResponse{
		Auth:             authSettings,
		Proxy:            *proxyConfig,
		ServerVersion:    teleport.Version,
		MinClientVersion: teleport.MinClientVersion,
		ClusterName:      h.auth.clusterName,
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
	if apiutils.SliceContainsStr(constants.SystemConnectors, connectorName) {
		response.Auth, err = localSettings(cap)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		response.Auth.HasMessageOfTheDay = hasMessageOfTheDay
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
		h.log.WithError(err).Error("Cannot retrieve Github connectors.")
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
		}
	}

	// get tunnel address to display on cloud instances
	tunnelPublicAddr := ""
	if h.ClusterFeatures.GetCloud() {
		proxyConfig, err := h.cfg.ProxySettings.GetProxySettings(r.Context())
		if err != nil {
			h.log.WithError(err).Warn("Cannot retrieve ProxySettings, tunnel address won't be set in Web UI.")
		} else {
			tunnelPublicAddr = proxyConfig.SSH.TunnelPublicAddr
		}
	}

	// disable joining sessions if proxy session recording is enabled
	canJoinSessions := true
	recCfg, err := h.cfg.ProxyClient.GetSessionRecordingConfig(r.Context())
	if err != nil {
		h.log.WithError(err).Error("Cannot retrieve SessionRecordingConfig.")
	} else {
		canJoinSessions = services.IsRecordAtProxy(recCfg.GetMode()) == false
	}

	webCfg := webclient.WebConfig{
		Auth:                authSettings,
		CanJoinSessions:     canJoinSessions,
		IsCloud:             h.ClusterFeatures.GetCloud(),
		TunnelPublicAddress: tunnelPublicAddr,
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

func (h *Handler) oidcLoginWeb(w http.ResponseWriter, r *http.Request, p httprouter.Params) string {
	logger := h.log.WithField("auth", "oidc")
	logger.Debug("Web login start.")

	req, err := parseSSORequestParams(r)
	if err != nil {
		logger.WithError(err).Error("Failed to extract SSO parameters from request.")
		return client.LoginFailedRedirectURL
	}

	response, err := h.cfg.ProxyClient.CreateOIDCAuthRequest(r.Context(), types.OIDCAuthRequest{
		CSRFToken:         req.csrfToken,
		ConnectorID:       req.connectorID,
		CreateWebSession:  true,
		ClientRedirectURL: req.clientRedirectURL,
		CheckUser:         true,
		ProxyAddress:      r.Host,
	})
	if err != nil {
		logger.WithError(err).Error("Error creating auth request.")
		return client.LoginFailedRedirectURL
	}

	return response.RedirectURL
}

func (h *Handler) githubLoginWeb(w http.ResponseWriter, r *http.Request, p httprouter.Params) string {
	logger := h.log.WithField("auth", "github")
	logger.Debug("Web login start.")

	req, err := parseSSORequestParams(r)
	if err != nil {
		logger.WithError(err).Error("Failed to extract SSO parameters from request.")
		return client.LoginFailedRedirectURL
	}

	response, err := h.cfg.ProxyClient.CreateGithubAuthRequest(r.Context(), types.GithubAuthRequest{
		CSRFToken:         req.csrfToken,
		ConnectorID:       req.connectorID,
		CreateWebSession:  true,
		ClientRedirectURL: req.clientRedirectURL,
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
		return nil, trace.AccessDenied(ssoLoginConsoleErr)
	}

	if err := req.CheckAndSetDefaults(); err != nil {
		logger.WithError(err).Error("Missing request parameters.")
		return nil, trace.AccessDenied(ssoLoginConsoleErr)
	}

	response, err := h.cfg.ProxyClient.CreateGithubAuthRequest(r.Context(), types.GithubAuthRequest{
		ConnectorID:       req.ConnectorID,
		PublicKey:         req.PublicKey,
		CertTTL:           req.CertTTL,
		ClientRedirectURL: req.RedirectURL,
		Compatibility:     req.Compatibility,
		RouteToCluster:    req.RouteToCluster,
		KubernetesCluster: req.KubernetesCluster,
	})
	if err != nil {
		logger.WithError(err).Error("Failed to create Github auth request.")
		return nil, trace.AccessDenied(ssoLoginConsoleErr)
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
				if redURL, errEnc := redirectURLWithError(request.ClientRedirectURL, err); errEnc == nil {
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

		res := &ssoCallbackResponse{
			csrfToken:         response.Req.CSRFToken,
			username:          response.Username,
			sessionName:       response.Session.GetName(),
			clientRedirectURL: response.Req.ClientRedirectURL,
		}

		if err := ssoSetWebSessionAndRedirectURL(w, r, res); err != nil {
			logger.WithError(err).Error("Error setting web session.")
			return client.LoginFailedRedirectURL
		}

		return res.clientRedirectURL
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

func (h *Handler) oidcLoginConsole(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	logger := h.log.WithField("auth", "oidc")
	logger.Debug("Console login start.")

	req := new(client.SSOLoginConsoleReq)
	if err := httplib.ReadJSON(r, req); err != nil {
		logger.WithError(err).Error("Error reading json.")
		return nil, trace.AccessDenied(ssoLoginConsoleErr)
	}

	if err := req.CheckAndSetDefaults(); err != nil {
		logger.WithError(err).Error("Missing request parameters.")
		return nil, trace.AccessDenied(ssoLoginConsoleErr)
	}

	response, err := h.cfg.ProxyClient.CreateOIDCAuthRequest(r.Context(), types.OIDCAuthRequest{
		ConnectorID:       req.ConnectorID,
		ClientRedirectURL: req.RedirectURL,
		PublicKey:         req.PublicKey,
		CertTTL:           req.CertTTL,
		CheckUser:         true,
		Compatibility:     req.Compatibility,
		RouteToCluster:    req.RouteToCluster,
		KubernetesCluster: req.KubernetesCluster,
		ProxyAddress:      r.Host,
	})
	if err != nil {
		logger.WithError(err).Error("Failed to create OIDC auth request.")
		return nil, trace.AccessDenied(ssoLoginConsoleErr)
	}

	return &client.SSOLoginConsoleResponse{
		RedirectURL: response.RedirectURL,
	}, nil
}

func (h *Handler) oidcCallback(w http.ResponseWriter, r *http.Request, p httprouter.Params) string {
	logger := h.log.WithField("auth", "oidc")
	logger.Debug("Callback start.")

	response, err := h.cfg.ProxyClient.ValidateOIDCAuthCallback(r.Context(), r.URL.Query())
	if err != nil {
		logger.WithError(err).Error("Error while processing callback.")

		// try to find the auth request, which bears the original client redirect URL.
		// if found, use it to terminate the flow.
		//
		// this improves the UX by terminating the failed SSO flow immediately, rather than hoping for a timeout.
		if requestID := r.URL.Query().Get("state"); requestID != "" {
			if request, errGet := h.cfg.ProxyClient.GetOIDCAuthRequest(r.Context(), requestID); errGet == nil && !request.CreateWebSession {
				if redURL, errEnc := redirectURLWithError(request.ClientRedirectURL, err); errEnc == nil {
					return redURL.String()
				}
			}
		}

		if errors.Is(err, auth.ErrOIDCNoRoles) {
			return client.LoginFailedUnauthorizedRedirectURL
		}

		return client.LoginFailedBadCallbackRedirectURL
	}

	// if we created web session, set session cookie and redirect to original url
	if response.Req.CreateWebSession {
		logger.Info("Redirecting to web browser.")

		res := &ssoCallbackResponse{
			csrfToken:         response.Req.CSRFToken,
			username:          response.Username,
			sessionName:       response.Session.GetName(),
			clientRedirectURL: response.Req.ClientRedirectURL,
		}

		if err := ssoSetWebSessionAndRedirectURL(w, r, res); err != nil {
			logger.WithError(err).Error("Error setting web session.")
			return client.LoginFailedRedirectURL
		}

		return res.clientRedirectURL
	}

	logger.Info("Callback redirecting to console login.")
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
	})
	if err != nil {
		logger.WithError(err).Error("Error constructing ssh response")
		return client.LoginFailedRedirectURL
	}

	return redirectURL.String()
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
				"that tsh release was downloaded from https://dashboard.gravitational.com")
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

func redirectURLWithError(clientRedirectURL string, errReply error) (*url.URL, error) {
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

func newSessionResponse(ctx *SessionContext) (*CreateSessionResponse, error) {
	accessChecker, err := ctx.GetUserAccessChecker()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	_, err = accessChecker.CheckLoginDuration(0)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	token, err := ctx.getToken()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &CreateSessionResponse{
		TokenType:                roundtrip.AuthBearer,
		Token:                    token.GetName(),
		TokenExpiresIn:           int(token.Expiry().Sub(ctx.parent.clock.Now()) / time.Second),
		SessionInactiveTimeoutMS: int(ctx.session.GetIdleTimeout().Milliseconds()),
	}, nil
}

// createWebSession creates a new web session based on user, pass and 2nd factor token
//
// POST /v1/webapi/sessions/web
//
// {"user": "alex", "pass": "abc123", "second_factor_token": "token", "second_factor_type": "totp"}
//
// Response
//
// {"type": "bearer", "token": "bearer token", "user": {"name": "alex", "allowed_logins": ["admin", "bob"]}, "expires_in": 20}
//
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
		webSession, err = h.auth.AuthWithoutOTP(req.User, req.Pass, clientMeta)
	case constants.SecondFactorOTP, constants.SecondFactorOn:
		webSession, err = h.auth.AuthWithOTP(
			req.User, req.Pass, req.SecondFactorToken, clientMeta,
		)
	case constants.SecondFactorOptional:
		if req.SecondFactorToken == "" {
			webSession, err = h.auth.AuthWithoutOTP(
				req.User, req.Pass, clientMeta,
			)
		} else {
			webSession, err = h.auth.AuthWithOTP(
				req.User, req.Pass, req.SecondFactorToken, clientMeta,
			)
		}
	default:
		return nil, trace.AccessDenied("unknown second factor type: %q", cap.GetSecondFactor())
	}
	if err != nil {
		h.log.WithError(err).Warnf("Access attempt denied for user %q.", req.User)
		return nil, trace.AccessDenied("bad auth credentials")
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

	ctx, err := h.auth.newSessionContext(req.User, webSession.GetName())
	if err != nil {
		h.log.WithError(err).Warnf("Access attempt denied for user %q.", req.User)
		return nil, trace.AccessDenied("need auth")
	}

	return newSessionResponse(ctx)
}

func clientMetaFromReq(r *http.Request) *auth.ForwardedClientMetadata {
	// multiplexer handles extracting real client IP using PROXY protocol where
	// available, so we can omit checking X-Forwarded-For.
	return &auth.ForwardedClientMetadata{
		UserAgent:  r.UserAgent(),
		RemoteAddr: r.RemoteAddr,
	}
}

// deleteSession is called to sign out user
//
// DELETE /v1/webapi/sessions/:sid
//
// Response:
//
// {"message": "ok"}
//
func (h *Handler) deleteSession(w http.ResponseWriter, r *http.Request, _ httprouter.Params, ctx *SessionContext) (interface{}, error) {
	err := h.logout(w, ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return OK(), nil
}

func (h *Handler) logout(w http.ResponseWriter, ctx *SessionContext) error {
	if err := ctx.Invalidate(); err != nil {
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
}

// renewSession updates this existing session with a new session.
//
// Depending on request fields sent in for extension, the new session creation can vary depending on:
//   - requestId (opt): appends roles approved from access request to currently assigned roles or,
//   - switchback (opt): roles stacked with assuming approved access requests, will revert to user's default roles
//   - default (none set): create new session with currently assigned roles
func (h *Handler) renewSession(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *SessionContext) (interface{}, error) {
	req := renewSessionRequest{}
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	if req.AccessRequestID != "" && req.Switchback {
		return nil, trace.BadParameter("Failed to renew session: fields 'AccessRequestID' and 'Switchback' cannot be both set")
	}

	newSession, err := ctx.extendWebSession(r.Context(), req.AccessRequestID, req.Switchback)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	newContext, err := h.auth.newSessionContextFromSession(newSession)
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

type IdemeumServiceTokenRequest struct {
	ServiceToken string `json:"serviceToken"`
}

func (h *Handler) createSessionForIdemeumServiceToken(w http.ResponseWriter, r *http.Request, params httprouter.Params) (interface{}, error) {
	if h.cfg.TenantUrl == "" {
		return nil, trace.BadParameter("Failed to issue session: Tenant Url not configured")
	}

	req := IdemeumServiceTokenRequest{}
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	if req.ServiceToken == "" {
		return nil, trace.BadParameter("Failed to issue session: Missing 'ServiceToken' field")
	}

	newSession, err := h.auth.ValidateServiceToken(r.Context(), req.ServiceToken, h.cfg.TenantUrl)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := SetSessionCookie(w, newSession.GetUser(), newSession.GetName()); err != nil {
		return nil, trace.Wrap(err)
	}

	ctx, err := h.auth.newSessionContext(newSession.GetUser(), newSession.GetName())
	if err != nil {
		return nil, trace.AccessDenied("need auth")
	}
	return newSessionResponse(ctx)
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

	sess := res.WebSession
	ctx, err := h.auth.newSessionContext(sess.GetUser(), sess.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := SetSessionCookie(w, sess.GetUser(), sess.GetName()); err != nil {
		return nil, trace.Wrap(err)
	}

	// Checks for at least one valid login.
	if _, err := newSessionResponse(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	if res.GetRecovery() == nil {
		return &ui.RecoveryCodes{}, nil
	}

	return &ui.RecoveryCodes{
		Codes:   res.Recovery.Codes,
		Created: &res.Recovery.Created,
	}, nil
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
		return nil, trace.AccessDenied("bad auth credentials")
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
// Success response
//
// { "cert": "base64 encoded signed cert", "host_signers": [{"domain_name": "example.com", "checking_keys": ["base64 encoded public signing key"]}] }
func (h *Handler) mfaLoginFinish(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req *client.AuthenticateSSHUserRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	clientMeta := clientMetaFromReq(r)
	cert, err := h.auth.AuthenticateSSHUser(*req, clientMeta)
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
	session, err := h.auth.AuthenticateWebUser(req, clientMeta)
	if err != nil {
		return nil, trace.AccessDenied("bad auth credentials")
	}

	// Fetch user from session, user is empty for passwordless requests.
	user := session.GetUser()
	if err := SetSessionCookie(w, user, session.GetName()); err != nil {
		return nil, trace.Wrap(err)
	}

	ctx, err := h.auth.newSessionContext(user, session.GetName())
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
//
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

/* getSiteNamespaces returns a list of namespaces for a given site

GET /v1/webapi/sites/:site/namespaces

Successful response:

{"namespaces": [{..namespace resource...}]}
*/
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
func (h *Handler) clusterNodesGet(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	// Get a client to the Auth Server with the logged in user's identity. The
	// identity of the logged in user is used to fetch the list of nodes.
	clt, err := ctx.GetUserClient(site)
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

	accessChecker, err := ctx.GetUserAccessChecker()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return listResourcesGetResponse{
		Items:      ui.MakeServers(site.GetName(), servers, accessChecker.Roles()),
		StartKey:   resp.NextKey,
		TotalCount: resp.TotalCount,
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
//
func (h *Handler) siteNodeConnect(
	w http.ResponseWriter,
	r *http.Request,
	p httprouter.Params,
	ctx *SessionContext,
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

	h.log.Debugf("New terminal request for ns=%s, server=%s, login=%s, sid=%s, websid=%s.",
		req.Namespace, req.Server, req.Login, req.SessionID, ctx.GetSessionID())

	authAccessPoint, err := site.CachingAccessPoint()
	if err != nil {
		h.log.WithError(err).Debug("Unable to get auth access point.")
		return nil, trace.Wrap(err)
	}

	netConfig, err := authAccessPoint.GetClusterNetworkingConfig(h.cfg.Context)
	if err != nil {
		h.log.WithError(err).Debug("Unable to fetch cluster networking config.")
		return nil, trace.Wrap(err)
	}

	req.KeepAliveInterval = netConfig.GetKeepAliveInterval()
	req.Namespace = apidefaults.Namespace
	req.ProxyHostPort = h.ProxyHostPort()
	req.Cluster = site.GetName()

	clt, err := ctx.GetUserClient(site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	term, err := NewTerminal(r.Context(), *req, clt, ctx)
	if err != nil {
		h.log.WithError(err).Error("Unable to create terminal.")
		return nil, trace.Wrap(err)
	}

	// start the websocket session with a web-based terminal:
	h.log.Infof("Getting terminal to %#v.", req)
	term.Serve(w, r)

	return nil, nil
}

type siteSessionGenerateReq struct {
	Session session.Session `json:"session"`
}

type siteSessionGenerateResponse struct {
	Session session.Session `json:"session"`
}

// siteSessionCreate generates a new site session that can be used by UI
// The ServerID from request can be in the form of hostname, uuid, or ip address.
func (h *Handler) siteSessionGenerate(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	clt, err := ctx.GetUserClient(site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var req *siteSessionGenerateReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	namespace := apidefaults.Namespace
	if req.Session.ServerID != "" {
		servers, err := clt.GetNodes(r.Context(), namespace)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		hostname, _, err := resolveServerHostPort(req.Session.ServerID, servers)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		req.Session.ServerHostname = hostname
	}

	req.Session.ID = session.NewID()
	req.Session.Created = time.Now().UTC()
	req.Session.LastActive = time.Now().UTC()
	req.Session.Namespace = namespace

	return siteSessionGenerateResponse{Session: req.Session}, nil
}

type siteSessionsGetResponse struct {
	Sessions []session.Session `json:"sessions"`
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
	}
}

// siteSessionsGet gets the list of site sessions filtered by creation time
// and whether they're active or not
//
// GET /v1/webapi/sites/:site/namespaces/:namespace/sessions
//
// Response body:
//
// {"sessions": [{"id": "sid", "terminal_params": {"w": 100, "h": 100}, "parties": [], "login": "bob"}, ...] }
func (h *Handler) siteSessionsGet(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	clt, err := ctx.GetUserClient(site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	trackers, err := clt.GetActiveSessionTrackers(r.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sessions := make([]session.Session, 0, len(trackers))
	for _, tracker := range trackers {
		// Only return ssh and k8s session type.
		isSSHOrK8s := tracker.GetSessionKind() == types.SSHSessionKind || tracker.GetSessionKind() == types.KubernetesSessionKind
		if isSSHOrK8s && tracker.GetState() != types.SessionState_SessionStateTerminated {
			sessions = append(sessions, trackerToLegacySession(tracker, p.ByName("site")))
		}
	}

	return siteSessionsGetResponse{Sessions: sessions}, nil
}

// siteSessionGet gets the list of site session by id
//
// GET /v1/webapi/sites/:site/namespaces/:namespace/sessions/:sid
//
// Response body:
//
// {"session": {"id": "sid", "terminal_params": {"w": 100, "h": 100}, "parties": [], "login": "bob"}}
//
func (h *Handler) siteSessionGet(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	sessionID, err := session.ParseID(p.ByName("sid"))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	h.log.Infof("web.getSession(%v)", sessionID)

	clt, err := ctx.GetUserClient(site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tracker, err := clt.GetSessionTracker(r.Context(), string(*sessionID))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if tracker.GetSessionKind() != types.SSHSessionKind || tracker.GetState() == types.SessionState_SessionStateTerminated {
		return nil, trace.NotFound("session %v not found", sessionID)
	}

	return trackerToLegacySession(tracker, site.GetName()), nil
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
//   "from"    : date range from, encoded as RFC3339
//   "to"      : date range to, encoded as RFC3339
//   "limit"   : optional maximum number of events to return on each fetch
//   "startKey": resume events search from the last event received,
//               empty string means start search from beginning
//   "include" : optional comma-separated list of event names to return e.g.
//               include=session.start,session.end, all are returned if empty
//   "order":    optional ordering of events. Can be either "asc" or "desc"
//               for ascending and descending respectively.
//               If no order is provided it defaults to descending.
func (h *Handler) clusterSearchEvents(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	values := r.URL.Query()

	var eventTypes []string
	if include := values.Get("include"); include != "" {
		eventTypes = strings.Split(include, ",")
	}

	searchEvents := func(clt auth.ClientI, from, to time.Time, limit int, order types.EventOrder, startKey string) ([]apievents.AuditEvent, string, error) {
		return clt.SearchEvents(from, to, apidefaults.Namespace, eventTypes, limit, order, startKey)
	}
	return clusterEventsList(ctx, site, r.URL.Query(), searchEvents)
}

// clusterSearchSessionEvents returns session events matching the criteria.
//
// GET /v1/webapi/sites/:site/sessions/search
//
// Query parameters:
//   "from"    : date range from, encoded as RFC3339
//   "to"      : date range to, encoded as RFC3339
//   "limit"   : optional maximum number of events to return on each fetch
//   "startKey": resume events search from the last event received,
//               empty string means start search from beginning
//   "order":    optional ordering of events. Can be either "asc" or "desc"
//               for ascending and descending respectively.
//               If no order is provided it defaults to descending.
func (h *Handler) clusterSearchSessionEvents(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	searchSessionEvents := func(clt auth.ClientI, from, to time.Time, limit int, order types.EventOrder, startKey string) ([]apievents.AuditEvent, string, error) {
		return clt.SearchSessionEvents(from, to, limit, order, startKey, nil, "")
	}
	return clusterEventsList(ctx, site, r.URL.Query(), searchSessionEvents)
}

// clusterEventsList returns a list of audit events obtained using the provided
// searchEvents method.
func clusterEventsList(ctx *SessionContext, site reversetunnel.RemoteSite, values url.Values, searchEvents func(clt auth.ClientI, from, to time.Time, limit int, order types.EventOrder, startKey string) ([]apievents.AuditEvent, string, error)) (interface{}, error) {
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

	clt, err := ctx.GetUserClient(site)
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
//   "offset"   : bytes from the beginning
//   "bytes"    : number of bytes to read (it won't return more than 512Kb)
//
// Unlike other request handlers, this one does not return JSON.
// It returns the binary stream unencoded, directly in the respose body,
// with Content-Type of application/octet-stream, gzipped with up to 95%
// compression ratio.
func (h *Handler) siteSessionStreamGet(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	httplib.SetNoCacheHeaders(w.Header())

	var site reversetunnel.RemoteSite
	onError := func(err error) {
		h.log.WithError(err).Debug("Unable to retrieve session chunk.")
		http.Error(w, err.Error(), trace.ErrorToCode(err))
	}

	// authenticate first:
	ctx, err := h.AuthenticateRequest(w, r, true)
	if err != nil {
		h.log.WithError(err).Warn("Failed to authenticate.")
		// clear session just in case if the authentication request is not valid
		ClearSession(w)
		onError(trace.Wrap(err))
		return
	}

	// get the site interface:
	siteName := p.ByName("site")
	if siteName == currentSiteShortcut {
		res, err := h.cfg.ProxyClient.GetClusterName()
		if err != nil {
			onError(trace.Wrap(err))
			return
		}
		siteName = res.GetClusterName()
	}
	proxy, err := h.ProxyWithRoles(ctx)
	if err != nil {
		onError(trace.Wrap(err))
		return
	}
	site, err = proxy.GetSite(siteName)
	if err != nil {
		onError(trace.Wrap(err))
		return
	}

	// get the session:
	sid, err := session.ParseID(p.ByName("sid"))
	if err != nil {
		onError(trace.Wrap(err))
		return
	}
	clt, err := ctx.GetUserClient(site)
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
//    "after" : cursor value of an event to return "newer than" events
//              good for repeated polling
//
// Response body (each event is an arbitrary JSON structure)
//
// {"events": [{...}, {...}, ...}
//
func (h *Handler) siteSessionEventsGet(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	sessionID, err := session.ParseID(p.ByName("sid"))
	if err != nil {
		return nil, trace.BadParameter("invalid session ID %q", p.ByName("sid"))
	}

	clt, err := ctx.GetUserClient(site)
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
// Success response
//
// { "cert": "base64 encoded signed cert", "host_signers": [{"domain_name": "example.com", "checking_keys": ["base64 encoded public signing key"]}] }
//
func (h *Handler) createSSHCert(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req *client.CreateSSHCertReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	authClient := h.cfg.ProxyClient
	cap, err := authClient.GetAuthPreference(r.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clientMeta := clientMetaFromReq(r)

	var cert *auth.SSHLoginResponse

	switch cap.GetSecondFactor() {
	case constants.SecondFactorOff:
		cert, err = h.auth.GetCertificateWithoutOTP(*req, clientMeta)
	case constants.SecondFactorOTP, constants.SecondFactorOn, constants.SecondFactorOptional:
		// convert legacy requests to new parameter here. remove once migration to TOTP is complete.
		if req.HOTPToken != "" {
			req.OTPToken = req.HOTPToken
		}
		cert, err = h.auth.GetCertificateWithOTP(*req, clientMeta)
	default:
		return nil, trace.AccessDenied("unknown second factor type: %q", cap.GetSecondFactor())
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return cert, nil
}

// validateTrustedCluster validates the token for a trusted cluster and returns it's own host and user certificate authority.
//
// POST /webapi/trustedclusters/validate
//
// * Request body:
//
// {
//     "token": "foo",
//     "certificate_authorities": ["AQ==", "Ag=="]
// }
//
// * Response:
//
// {
//     "certificate_authorities": ["AQ==", "Ag=="]
// }
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
type ClusterHandler func(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error)

// WithClusterAuth ensures that request is authenticated and is issued for existing cluster
func (h *Handler) WithClusterAuth(fn ClusterHandler) httprouter.Handle {
	return httplib.MakeHandler(func(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
		ctx, err := h.AuthenticateRequest(w, r, true)
		if err != nil {
			h.log.WithError(err).Warn("Failed to authenticate.")
			return nil, trace.Wrap(err)
		}

		clusterName := p.ByName("site")
		if clusterName == currentSiteShortcut {
			res, err := h.cfg.ProxyClient.GetClusterName()
			if err != nil {
				h.log.WithError(err).Warn("Failed to query cluster name.")
				return nil, trace.Wrap(err)
			}
			clusterName = res.GetClusterName()
		}

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

		return fn(w, r, p, ctx, site)
	})
}

type redirectHandlerFunc func(w http.ResponseWriter, r *http.Request, p httprouter.Params) (redirectURL string)

func isValidRedirectURL(redirectURL string) bool {
	u, err := url.ParseRequestURI(redirectURL)
	return err == nil && (!u.IsAbs() || (u.Scheme == "http" || u.Scheme == "https"))
}

// WithRedirect is a handler that redirects to the path specified in the returned value.
func (h *Handler) WithRedirect(fn redirectHandlerFunc) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		// ensure that neither proxies nor browsers cache http traffic
		httplib.SetNoCacheHeaders(w.Header())

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
		httplib.SetNoCacheHeaders(w.Header())
		app.SetRedirectPageHeaders(w.Header(), "")
		redirectURL := fn(w, r, p)
		if !isValidRedirectURL(redirectURL) {
			redirectURL = client.LoginFailedRedirectURL
		}
		err := metaRedirectTemplate.Execute(w, redirectURL)
		if err != nil {
			h.log.WithError(err).Warn("Failed to execute template.")
		}
	}
}

// WithAuth ensures that request is authenticated
func (h *Handler) WithAuth(fn ContextHandler) httprouter.Handle {
	return httplib.MakeHandler(func(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
		ctx, err := h.AuthenticateRequest(w, r, true)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return fn(w, r, p, ctx)
	})
}

// withLimiter adds IP-based rate limiting to fn.
func (h *Handler) withLimiter(l *limiter.RateLimiter, fn httplib.HandlerFunc) httprouter.Handle {
	return httplib.MakeHandler(func(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
		err := l.RegisterRequest(r.RemoteAddr, nil /* customRate */)
		// MaxRateError doesn't play well with errors.Is, hence the cast.
		if _, ok := err.(*ratelimit.MaxRateError); ok {
			return nil, trace.LimitExceeded(err.Error())
		}
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return fn(w, r, p)
	})
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
	ctx, err := h.auth.validateSession(r.Context(), decodedCookie.User, decodedCookie.SID)
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
	// Proxy web address can be set in the config with unspecified host, like
	// 0.0.0.0:3080 or :443.
	//
	// In this case, the dial will succeed (dialing 0.0.0.0 is same a dialing
	// localhost in Go) but the SSH host certificate will not validate since
	// 0.0.0.0 is never a valid principal (auth server explicitly removes it
	// when issuing host certs).
	//
	// As such, replace 0.0.0.0 with localhost in this case: proxy listens on
	// all interfaces and localhost is always included in the valid principal
	// set.
	if h.cfg.ProxyWebAddr.IsHostUnspecified() {
		return fmt.Sprintf("localhost:%v,%s", h.cfg.ProxyWebAddr.Port(defaults.HTTPListenPort), h.sshPort)
	}
	return fmt.Sprintf("%s,%s", h.cfg.ProxyWebAddr.String(), h.sshPort)
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
func makeTeleportClientConfig(ctx context.Context, sesCtx *SessionContext) (*client.Config, error) {
	agent, cert, err := sesCtx.GetAgent()
	if err != nil {
		return nil, trace.BadParameter("failed to get user credentials: %v", err)
	}

	signers, err := agent.Signers()
	if err != nil {
		return nil, trace.BadParameter("failed to get user credentials: %v", err)
	}

	tlsConfig, err := sesCtx.ClientTLSConfig()
	if err != nil {
		return nil, trace.BadParameter("failed to get client TLS config: %v", err)
	}

	callback, err := apisshutils.NewHostKeyCallback(
		apisshutils.HostKeyCallbackConfig{
			GetHostCheckers: sesCtx.getCheckers,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	proxyListenerMode, err := sesCtx.GetProxyListenerMode(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	config := &client.Config{
		Username:          sesCtx.user,
		Agent:             agent,
		SkipLocalAuth:     true,
		TLS:               tlsConfig,
		AuthMethods:       []ssh.AuthMethod{ssh.PublicKeys(signers...)},
		DefaultPrincipal:  cert.ValidPrincipals[0],
		HostKeyCallback:   callback,
		TLSRoutingEnabled: proxyListenerMode == types.ProxyListenerMode_Multiplex,
		Tracer:            tracing.NoopProvider().Tracer("test"),
	}

	return config, nil
}

type ssoRequestParams struct {
	clientRedirectURL string
	connectorID       string
	csrfToken         string
}

func parseSSORequestParams(r *http.Request) (*ssoRequestParams, error) {
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

	return &ssoRequestParams{
		clientRedirectURL: clientRedirectURL,
		connectorID:       connectorID,
		csrfToken:         csrfToken,
	}, nil
}

type ssoCallbackResponse struct {
	csrfToken         string
	username          string
	sessionName       string
	clientRedirectURL string
}

func ssoSetWebSessionAndRedirectURL(w http.ResponseWriter, r *http.Request, response *ssoCallbackResponse) error {
	if err := csrf.VerifyToken(response.csrfToken, r); err != nil {
		return trace.Wrap(err)
	}

	if err := SetSessionCookie(w, response.username, response.sessionName); err != nil {
		return trace.Wrap(err)
	}

	parsedURL, err := url.Parse(response.clientRedirectURL)
	if err != nil {
		return trace.Wrap(err)
	}

	response.clientRedirectURL = parsedURL.RequestURI()

	return nil
}
