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
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
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
	apisshutils "github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/u2f"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/httplib/csrf"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/plugin"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/secret"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/web/app"
	"github.com/gravitational/teleport/lib/web/ui"

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
)

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

	// ProxySettings is a settings communicated to proxy
	ProxySettings webclient.ProxySettings

	// FIPS mode means Teleport started in a FedRAMP/FIPS 140-2 compliant
	// configuration.
	FIPS bool

	// AccessPoint holds a cache to the Auth Server.
	AccessPoint auth.AccessPoint

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
}

type RewritingHandler struct {
	http.Handler
	handler *Handler

	// appHandler is a http.Handler to forward requests to applications.
	appHandler *app.Handler
}

// Check if this request should be forwarded to an application handler to
// be handled by the UI and handle the request appropriately.
func (h *RewritingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
	h.Handler.ServeHTTP(w, r)
}

func (h *RewritingHandler) Close() error {
	return h.handler.Close()
}

// NewHandler returns a new instance of web proxy handler
func NewHandler(cfg Config, opts ...HandlerOption) (*RewritingHandler, error) {
	const apiPrefix = "/" + teleport.WebAPIVersion
	h := &Handler{
		cfg:             cfg,
		log:             newPackageLogger(),
		clock:           clockwork.NewRealClock(),
		ClusterFeatures: cfg.ClusterFeatures,
	}

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

	h.POST("/webapi/users", h.WithAuth(h.createUserHandle))
	h.PUT("/webapi/users", h.WithAuth(h.updateUserHandle))
	h.GET("/webapi/users", h.WithAuth(h.getUsersHandle))
	h.DELETE("/webapi/users/:username", h.WithAuth(h.deleteUserHandle))

	h.GET("/webapi/users/password/token/:token", httplib.MakeHandler(h.getResetPasswordTokenHandle))
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
	h.GET("/webapi/sites/:site/namespaces/:namespace/nodes", h.WithClusterAuth(h.siteNodesGet))

	// Get applications.
	h.GET("/webapi/sites/:site/apps", h.WithClusterAuth(h.clusterAppsGet))

	// active sessions handlers
	h.GET("/webapi/sites/:site/namespaces/:namespace/connect", h.WithClusterAuth(h.siteNodeConnect))       // connect to an active session (via websocket)
	h.GET("/webapi/sites/:site/namespaces/:namespace/sessions", h.WithClusterAuth(h.siteSessionsGet))      // get active list of sessions
	h.POST("/webapi/sites/:site/namespaces/:namespace/sessions", h.WithClusterAuth(h.siteSessionGenerate)) // create active session metadata
	h.GET("/webapi/sites/:site/namespaces/:namespace/sessions/:sid", h.WithClusterAuth(h.siteSessionGet))  // get active session metadata

	// Audit events handlers.
	h.GET("/webapi/sites/:site/events/search", h.WithClusterAuth(h.clusterSearchEvents))                               // search site events
	h.GET("/webapi/sites/:site/namespaces/:namespace/sessions/:sid/events", h.WithClusterAuth(h.siteSessionEventsGet)) // get recorded session's timing information (from events)
	h.GET("/webapi/sites/:site/namespaces/:namespace/sessions/:sid/stream", h.siteSessionStreamGet)                    // get recorded session's bytes (from events)

	// scp file transfer
	h.GET("/webapi/sites/:site/namespaces/:namespace/nodes/:server/:login/scp", h.WithClusterAuth(h.transferFile))
	h.POST("/webapi/sites/:site/namespaces/:namespace/nodes/:server/:login/scp", h.WithClusterAuth(h.transferFile))

	// web context
	h.GET("/webapi/sites/:site/context", h.WithClusterAuth(h.getUserContext))

	// Database access handlers.
	h.GET("/webapi/sites/:site/databases", h.WithClusterAuth(h.clusterDatabasesGet))

	// Kube access handlers.
	h.GET("/webapi/sites/:site/kubernetes", h.WithClusterAuth(h.clusterKubesGet))

	// OIDC related callback handlers
	h.GET("/webapi/oidc/login/web", h.WithRedirect(h.oidcLoginWeb))
	h.GET("/webapi/oidc/callback", h.WithRedirect(h.oidcCallback))
	h.POST("/webapi/oidc/login/console", httplib.MakeHandler(h.oidcLoginConsole))

	// SAML 2.0 handlers
	h.POST("/webapi/saml/acs", h.WithRedirect(h.samlACS))
	h.GET("/webapi/saml/sso", h.WithRedirect(h.samlSSO))
	h.POST("/webapi/saml/login/console", httplib.MakeHandler(h.samlSSOConsole))

	// Github connector handlers
	h.GET("/webapi/github/login/web", h.WithRedirect(h.githubLoginWeb))
	h.GET("/webapi/github/callback", h.WithRedirect(h.githubCallback))
	h.POST("/webapi/github/login/console", httplib.MakeHandler(h.githubLoginConsole))

	// U2F related APIs
	// DELETE IN 9.x, superseded by /mfa/ endpoints (codingllama)
	h.GET("/webapi/u2f/signuptokens/:token", httplib.MakeHandler(h.u2fRegisterRequest))
	h.POST("/webapi/u2f/password/changerequest", h.WithAuth(h.u2fChangePasswordRequest))
	h.POST("/webapi/u2f/signrequest", httplib.MakeHandler(h.mfaLoginBegin))
	h.POST("/webapi/u2f/sessions", httplib.MakeHandler(h.mfaLoginFinishSession))
	h.POST("/webapi/u2f/certs", httplib.MakeHandler(h.mfaLoginFinish))

	// MFA public endpoints.
	h.POST("/webapi/mfa/login/begin", httplib.MakeHandler(h.mfaLoginBegin))
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
	h.GET("/webapi/sites/:site/desktops", h.WithClusterAuth(h.getDesktopsHandle))
	h.GET("/webapi/sites/:site/desktop/:desktopUUID/connect", h.WithClusterAuth(h.handleDesktopAccessWebsocket))

	// if Web UI is enabled, check the assets dir:
	var indexPage *template.Template
	if cfg.StaticFS != nil {
		index, err := cfg.StaticFS.Open("/index.html")
		if err != nil {
			h.log.WithError(err).Error("Failed to open index file.")
			return nil, trace.Wrap(err)
		}
		defer index.Close()
		indexContent, err := ioutil.ReadAll(index)
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

	// Create application specific handler. This handler handles sessions and
	// forwarding for application access.
	appHandler, err := app.NewHandler(cfg.Context, &app.HandlerConfig{
		Clock:         h.clock,
		AuthClient:    cfg.ProxyClient,
		AccessPoint:   cfg.AccessPoint,
		ProxyClient:   cfg.Proxy,
		CipherSuites:  cfg.CipherSuites,
		WebPublicAddr: cfg.ProxySettings.SSH.PublicAddr,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &RewritingHandler{
		Handler: httplib.RewritePaths(h,
			httplib.Rewrite("/webapi/sites/([^/]+)/sessions/(.*)", "/webapi/sites/$1/namespaces/default/sessions/$2"),
			httplib.Rewrite("/webapi/sites/([^/]+)/sessions", "/webapi/sites/$1/namespaces/default/sessions"),
			httplib.Rewrite("/webapi/sites/([^/]+)/nodes", "/webapi/sites/$1/namespaces/default/nodes"),
			httplib.Rewrite("/webapi/sites/([^/]+)/connect", "/webapi/sites/$1/namespaces/default/connect"),
		),
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
	roleset, err := c.GetUserRoles()
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

	userContext, err := ui.NewUserContext(user, roleset, h.ClusterFeatures)
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
		Type:         constants.Local,
		SecondFactor: cap.GetSecondFactor(),
	}

	// U2F settings.
	if u2f, _ := cap.GetU2F(); u2f != nil {
		as.U2F = &webclient.U2FSettings{AppID: u2f.AppID}
	}

	// Webauthn settings.
	if webConfig, _ := cap.GetWebauthn(); webConfig != nil {
		as.Webauthn = &webclient.Webauthn{
			RPID: webConfig.RPID,
		}
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
		// if you falling back to local accounts
		SecondFactor: cap.GetSecondFactor(),
	}
}

func samlSettings(connector types.SAMLConnector, cap types.AuthPreference) webclient.AuthenticationSettings {
	return webclient.AuthenticationSettings{
		Type: constants.SAML,
		SAML: &webclient.SAMLSettings{
			Name:    connector.GetName(),
			Display: connector.GetDisplay(),
		},
		// if you are falling back to local accounts
		SecondFactor: cap.GetSecondFactor(),
	}
}

func githubSettings(connector types.GithubConnector, cap types.AuthPreference) webclient.AuthenticationSettings {
	return webclient.AuthenticationSettings{
		Type: constants.Github,
		Github: &webclient.GithubSettings{
			Name:    connector.GetName(),
			Display: connector.GetDisplay(),
		},
		SecondFactor: cap.GetSecondFactor(),
	}
}

func defaultAuthenticationSettings(ctx context.Context, authClient auth.ClientI) (webclient.AuthenticationSettings, error) {
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

	defaultSettings, err := defaultAuthenticationSettings(r.Context(), h.cfg.ProxyClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return webclient.PingResponse{
		Auth:             defaultSettings,
		Proxy:            h.cfg.ProxySettings,
		ServerVersion:    teleport.Version,
		MinClientVersion: teleport.MinClientVersion,
	}, nil
}

func (h *Handler) find(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	return webclient.PingResponse{
		Proxy:            h.cfg.ProxySettings,
		ServerVersion:    teleport.Version,
		MinClientVersion: teleport.MinClientVersion,
	}, nil
}

func (h *Handler) pingWithConnector(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	authClient := h.cfg.ProxyClient
	connectorName := p.ByName("connector")

	cap, err := authClient.GetAuthPreference(r.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response := &webclient.PingResponse{
		Proxy:         h.cfg.ProxySettings,
		ServerVersion: teleport.Version,
	}

	if connectorName == constants.Local {
		as, err := localSettings(cap)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		response.Auth = as
		return response, nil
	}

	// first look for a oidc connector with that name
	oidcConnector, err := authClient.GetOIDCConnector(r.Context(), connectorName, false)
	if err == nil {
		response.Auth = oidcSettings(oidcConnector, cap)
		return response, nil
	}

	// if no oidc connector was found, look for a saml connector
	samlConnector, err := authClient.GetSAMLConnector(r.Context(), connectorName, false)
	if err == nil {
		response.Auth = samlSettings(samlConnector, cap)
		return response, nil
	}

	// look for github connector
	githubConnector, err := authClient.GetGithubConnector(r.Context(), connectorName, false)
	if err == nil {
		response.Auth = githubSettings(githubConnector, cap)
		return response, nil
	}

	return nil, trace.BadParameter("invalid connector name %v", connectorName)
}

// getWebConfig returns configuration for the web application.
func (h *Handler) getWebConfig(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	httplib.SetWebConfigHeaders(w.Header())

	authProviders := []ui.WebConfigAuthProvider{}

	// get all OIDC connectors
	oidcConnectors, err := h.cfg.ProxyClient.GetOIDCConnectors(r.Context(), false)
	if err != nil {
		h.log.WithError(err).Error("Cannot retrieve OIDC connectors.")
	}
	for _, item := range oidcConnectors {
		authProviders = append(authProviders, ui.WebConfigAuthProvider{
			Type:        ui.WebConfigAuthProviderOIDCType,
			WebAPIURL:   ui.WebConfigAuthProviderOIDCURL,
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
		authProviders = append(authProviders, ui.WebConfigAuthProvider{
			Type:        ui.WebConfigAuthProviderSAMLType,
			WebAPIURL:   ui.WebConfigAuthProviderSAMLURL,
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
		authProviders = append(authProviders, ui.WebConfigAuthProvider{
			Type:        ui.WebConfigAuthProviderGitHubType,
			WebAPIURL:   ui.WebConfigAuthProviderGitHubURL,
			Name:        item.GetName(),
			DisplayName: item.GetDisplay(),
		})
	}

	// get auth type & second factor type
	authType := constants.Local
	secondFactor := constants.SecondFactorOff
	localAuth := true
	cap, err := h.cfg.ProxyClient.GetAuthPreference(r.Context())
	if err != nil {
		h.log.WithError(err).Error("Cannot retrieve AuthPreferences.")
	} else {
		authType = cap.GetType()
		secondFactor = cap.GetSecondFactor()
		localAuth = cap.GetAllowLocalAuth()
	}

	// disable joining sessions if proxy session recording is enabled
	canJoinSessions := true
	recCfg, err := h.cfg.ProxyClient.GetSessionRecordingConfig(r.Context())
	if err != nil {
		h.log.WithError(err).Error("Cannot retrieve SessionRecordingConfig.")
	} else {
		canJoinSessions = services.IsRecordAtProxy(recCfg.GetMode()) == false
	}

	authSettings := ui.WebConfigAuthSettings{
		Providers:        authProviders,
		SecondFactor:     secondFactor,
		LocalAuthEnabled: localAuth,
		AuthType:         authType,
	}

	webCfg := ui.WebConfig{
		Auth:            authSettings,
		CanJoinSessions: canJoinSessions,
		IsCloud:         h.ClusterFeatures.GetCloud(),
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
	clusterName, err := h.cfg.ProxyClient.GetDomainName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Fetch the JWT public keys only.
	ca, err := h.cfg.ProxyClient.GetCertAuthority(types.CertAuthID{
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

	response, err := h.cfg.ProxyClient.CreateOIDCAuthRequest(
		services.OIDCAuthRequest{
			CSRFToken:         req.csrfToken,
			ConnectorID:       req.connectorID,
			CreateWebSession:  true,
			ClientRedirectURL: req.clientRedirectURL,
			CheckUser:         true,
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

	response, err := h.cfg.ProxyClient.CreateGithubAuthRequest(
		services.GithubAuthRequest{
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

	response, err := h.cfg.ProxyClient.CreateGithubAuthRequest(
		services.GithubAuthRequest{
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

	response, err := h.cfg.ProxyClient.ValidateGithubAuthCallback(r.URL.Query())
	if err != nil {
		logger.WithError(err).Error("Error while processing callback.")
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

	response, err := h.cfg.ProxyClient.CreateOIDCAuthRequest(
		services.OIDCAuthRequest{
			ConnectorID:       req.ConnectorID,
			ClientRedirectURL: req.RedirectURL,
			PublicKey:         req.PublicKey,
			CertTTL:           req.CertTTL,
			CheckUser:         true,
			Compatibility:     req.Compatibility,
			RouteToCluster:    req.RouteToCluster,
			KubernetesCluster: req.KubernetesCluster,
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

	response, err := h.cfg.ProxyClient.ValidateOIDCAuthCallback(r.URL.Query())
	if err != nil {
		logger.WithError(err).Error("Error while processing callback.")
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
	roleset, err := ctx.GetUserRoles()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	_, err = roleset.CheckLoginDuration(0)
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

	var webSession types.WebSession

	switch cap.GetSecondFactor() {
	case constants.SecondFactorOff:
		webSession, err = h.auth.AuthWithoutOTP(req.User, req.Pass)
	case constants.SecondFactorOTP, constants.SecondFactorOn:
		webSession, err = h.auth.AuthWithOTP(req.User, req.Pass, req.SecondFactorToken)
	case constants.SecondFactorOptional:
		if req.SecondFactorToken == "" {
			webSession, err = h.auth.AuthWithoutOTP(req.User, req.Pass)
		} else {
			webSession, err = h.auth.AuthWithOTP(req.User, req.Pass, req.SecondFactorToken)
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

	newSession, err := ctx.extendWebSession(req.AccessRequestID, req.Switchback)
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

type changeUserAuthenticationRequest struct {
	// SecondFactorToken is the TOTP code.
	SecondFactorToken string `json:"second_factor_token"`
	// TokenID is the ID of a reset or invite token.
	TokenID string `json:"token"`
	// Password is user password string converted to bytes.
	Password []byte `json:"password"`
	// U2FRegisterResponse is U2F registration challenge response.
	U2FRegisterResponse *u2f.RegisterChallengeResponse `json:"u2f_register_response,omitempty"`
	// WebauthnRegisterResponse is the signed credential creation response.
	WebauthnRegisterResponse *wanlib.CredentialCreationResponse `json:"webauthn_register_response"`
}

func (h *Handler) changeUserAuthentication(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req changeUserAuthenticationRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	protoReq := &proto.ChangeUserAuthenticationRequest{
		TokenID:     req.TokenID,
		NewPassword: req.Password,
	}
	switch {
	case req.WebauthnRegisterResponse != nil:
		protoReq.NewMFARegisterResponse = &proto.MFARegisterResponse{
			Response: &proto.MFARegisterResponse_Webauthn{
				Webauthn: wanlib.CredentialCreationResponseToProto(req.WebauthnRegisterResponse),
			},
		}
	case req.U2FRegisterResponse != nil:
		protoReq.NewMFARegisterResponse = &proto.MFARegisterResponse{Response: &proto.MFARegisterResponse_U2F{
			U2F: &proto.U2FRegisterResponse{
				RegistrationData: req.U2FRegisterResponse.RegistrationData,
				ClientData:       req.U2FRegisterResponse.ClientData,
			},
		}}
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

	return res.RecoveryCodes, nil
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

// u2fRegisterRequest is called to get a U2F challenge for registering a U2F key
//
// GET /webapi/u2f/signuptokens/:token
//
// Response:
//
// {"version":"U2F_V2","challenge":"randombase64string","appId":"https://mycorp.com:3080"}
//
func (h *Handler) u2fRegisterRequest(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	res, err := h.auth.proxyClient.CreateRegisterChallenge(r.Context(), &proto.CreateRegisterChallengeRequest{
		TokenID:    p.ByName("token"),
		DeviceType: proto.DeviceType_DEVICE_TYPE_U2F,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	chal := client.MakeRegisterChallenge(res)
	return chal.U2F, nil
}

// mfaLoginBegin is the first step in the MFA authentication ceremony, which
// may be completed either via mfaLoginFinish (SSH) or mfaLoginFinishSession (Web).
//
// POST /webapi/u2f/signrequest (deprecated)
// POST /webapi/mfa/login/begin
//
// {"user": "alex", "pass": "abc123"}
//
// Successful response:
//
// {"webauthn_challenge": {...}, "u2f_challenges": [...], "totp_challenge": true}
func (h *Handler) mfaLoginBegin(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req *client.MFAChallengeRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	mfaChallenge, err := h.auth.proxyClient.CreateAuthenticateChallenge(r.Context(), &proto.CreateAuthenticateChallengeRequest{
		Request: &proto.CreateAuthenticateChallengeRequest_UserCredentials{UserCredentials: &proto.UserCredentials{
			Username: req.User,
			Password: []byte(req.Pass),
		}},
	})
	if err != nil {
		return nil, trace.AccessDenied("bad auth credentials")
	}
	return client.MakeAuthenticateChallenge(mfaChallenge), nil
}

// mfaLoginFinish completes the MFA login ceremony, returning a new SSH
// certificate if successful.
//
// POST /v1/webapi/u2f/certs (deprecated)
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

	cert, err := h.auth.AuthenticateSSHUser(*req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return cert, nil
}

// mfaLoginFinishSession completes the MFA login ceremony, returning a new web
// session if successful.
//
// POST /webapi/u2f/session (deprecated)
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

	session, err := h.auth.AuthenticateWebUser(req)
	if err != nil {
		return nil, trace.AccessDenied("bad auth credentials")
	}
	if err := SetSessionCookie(w, req.User, session.GetName()); err != nil {
		return nil, trace.Wrap(err)
	}
	ctx, err := h.auth.newSessionContext(req.User, session.GetName())
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

/* siteNodesGet returns a list of nodes for a given site and namespace

GET /v1/webapi/sites/:site/namespaces/:namespace/nodes

*/
func (h *Handler) siteNodesGet(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	namespace := p.ByName("namespace")
	if !types.IsValidNamespace(namespace) {
		return nil, trace.BadParameter("invalid namespace %q", namespace)
	}

	// Get a client to the Auth Server with the logged in user's identity. The
	// identity of the logged in user is used to fetch the list of nodes.
	clt, err := ctx.GetUserClient(site)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	servers, err := clt.GetNodes(r.Context(), namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	uiServers := ui.MakeServers(site.GetName(), servers)
	return makeResponse(uiServers)
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
// Session id can be empty
//
// Successful response is a websocket stream that allows read write to the server
//
func (h *Handler) siteNodeConnect(
	w http.ResponseWriter,
	r *http.Request,
	p httprouter.Params,
	ctx *SessionContext,
	site reversetunnel.RemoteSite) (interface{}, error) {

	namespace := p.ByName("namespace")
	if !types.IsValidNamespace(namespace) {
		return nil, trace.BadParameter("invalid namespace %q", namespace)
	}

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
	req.Namespace = namespace
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

	namespace := p.ByName("namespace")
	if !types.IsValidNamespace(namespace) {
		return nil, trace.BadParameter("invalid namespace %q", namespace)
	}

	var req *siteSessionGenerateReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

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

// siteSessionGet gets the list of site sessions filtered by creation time
// and either they're active or not
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

	namespace := p.ByName("namespace")
	if !types.IsValidNamespace(namespace) {
		return nil, trace.BadParameter("invalid namespace %q", namespace)
	}

	sessions, err := clt.GetSessions(namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// DELETE IN: 5.0.0
	// Teleport Nodes < v4.3 does not set ClusterName, ServerHostname with new sessions,
	// which 4.3 UI client relies on to create URL's and display node inform.
	clusterName := p.ByName("site")
	for i, session := range sessions {
		if session.ClusterName == "" {
			sessions[i].ClusterName = clusterName
		}
		if session.ServerHostname == "" {
			sessions[i].ServerHostname = session.ServerID
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

	namespace := p.ByName("namespace")
	if !types.IsValidNamespace(namespace) {
		return nil, trace.BadParameter("invalid namespace %q", namespace)
	}

	sess, err := clt.GetSession(namespace, *sessionID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// DELETE IN: 5.0.0
	// Teleport Nodes < v4.3 does not set ClusterName, ServerHostname with new sessions,
	// which 4.3 UI client relies on to create URL's and display node inform.
	if sess.ClusterName == "" {
		sess.ClusterName = p.ByName("site")
	}
	if sess.ServerHostname == "" {
		sess.ServerHostname = sess.ServerID
	}

	return *sess, nil
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

	clt, err := ctx.GetUserClient(site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var eventTypes []string
	if include := values.Get("include"); include != "" {
		eventTypes = strings.Split(include, ",")
	}

	startKey := values.Get("startKey")
	rawEvents, lastKey, err := clt.SearchEvents(from, to, apidefaults.Namespace, eventTypes, limit, order, startKey)
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
	namespace := p.ByName("namespace")
	if !types.IsValidNamespace(namespace) {
		onError(trace.BadParameter("invalid namespace %q", namespace))
		return
	}

	// call the site API to get the chunk:
	bytes, err := clt.GetSessionChunk(namespace, *sid, offset, max)
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
	namespace := p.ByName("namespace")
	if !types.IsValidNamespace(namespace) {
		return nil, trace.BadParameter("invalid namespace %q", namespace)
	}
	e, err := clt.GetSessionEvents(namespace, *sessionID, afterN, true)
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
	var req auth.RegisterUsingTokenRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	authClient := h.cfg.ProxyClient
	packedKeys, err := authClient.RegisterUsingToken(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return packedKeys, nil
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

	var cert *auth.SSHLoginResponse

	switch cap.GetSecondFactor() {
	case constants.SecondFactorOff:
		cert, err = h.auth.GetCertificateWithoutOTP(*req)
	case constants.SecondFactorOTP, constants.SecondFactorOn, constants.SecondFactorOptional:
		// convert legacy requests to new parameter here. remove once migration to TOTP is complete.
		if req.HOTPToken != "" {
			req.OTPToken = req.HOTPToken
		}
		cert, err = h.auth.GetCertificateWithOTP(*req)
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

	validateResponse, err := h.auth.ValidateTrustedCluster(validateRequest)
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

// WithRedirect is a handler that redirects to the path specified in the returned value.
func (h *Handler) WithRedirect(fn redirectHandlerFunc) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		// ensure that neither proxies nor browsers cache http traffic
		httplib.SetNoCacheHeaders(w.Header())

		redirectURL := fn(w, r, p)
		http.Redirect(w, r, redirectURL, http.StatusFound)
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
	roleset, err := ctx.GetUserRoles()
	if err != nil {
		h.log.WithError(err).Warn("Failed to get client roles.")
		return nil, trace.Wrap(err)
	}
	return reversetunnel.NewTunnelWithRoles(h.cfg.Proxy, roleset, h.cfg.AccessPoint), nil
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

type responseData struct {
	Items interface{} `json:"items"`
}

func makeResponse(items interface{}) (interface{}, error) {
	return responseData{Items: items}, nil
}

// makeTeleportClientConfig creates default teleport client configuration
// that is used to initiate an SSH terminal session or SCP file transfer
func makeTeleportClientConfig(ctx *SessionContext) (*client.Config, error) {
	agent, cert, err := ctx.GetAgent()
	if err != nil {
		return nil, trace.BadParameter("failed to get user credentials: %v", err)
	}

	signers, err := agent.Signers()
	if err != nil {
		return nil, trace.BadParameter("failed to get user credentials: %v", err)
	}

	tlsConfig, err := ctx.ClientTLSConfig()
	if err != nil {
		return nil, trace.BadParameter("failed to get client TLS config: %v", err)
	}

	callback, err := apisshutils.NewHostKeyCallback(
		apisshutils.HostKeyCallbackConfig{
			GetHostCheckers: ctx.getCheckers,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	config := &client.Config{
		Username:         ctx.user,
		Agent:            agent,
		SkipLocalAuth:    true,
		TLS:              tlsConfig,
		AuthMethods:      []ssh.AuthMethod{ssh.PublicKeys(signers...)},
		DefaultPrincipal: cert.ValidPrincipals[0],
		HostKeyCallback:  callback,
	}

	return config, nil
}

type ssoRequestParams struct {
	clientRedirectURL string
	connectorID       string
	csrfToken         string
}

func parseSSORequestParams(r *http.Request) (*ssoRequestParams, error) {
	query := r.URL.Query()

	clientRedirectURL := query.Get("redirect_url")
	if clientRedirectURL == "" {
		return nil, trace.BadParameter("missing redirect_url query parameter")
	}

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
