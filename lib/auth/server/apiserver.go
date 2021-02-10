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

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	libauth "github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/resource"
	"github.com/gravitational/teleport/lib/auth/u2f"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/plugin"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/form"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/julienschmidt/httprouter"
)

type APIConfig struct {
	PluginRegistry plugin.Registry
	AuthServer     *Server
	SessionService session.Service
	AuditLog       events.IAuditLog
	Authorizer     Authorizer
	Emitter        events.Emitter
	// KeepAlivePeriod defines period between keep alives
	KeepAlivePeriod time.Duration
	// KeepAliveCount specifies amount of missed keep alives
	// to wait for until declaring connection as broken
	KeepAliveCount int
	// MetadataGetter retrieves additional metadata about session uploads.
	// Will be nil if audit logging is not enabled.
	MetadataGetter events.UploadMetadataGetter
}

// CheckAndSetDefaults checks and sets default values
func (a *APIConfig) CheckAndSetDefaults() error {
	if a.KeepAlivePeriod == 0 {
		a.KeepAlivePeriod = defaults.ServerKeepAliveTTL
	}
	if a.KeepAliveCount == 0 {
		a.KeepAliveCount = defaults.KeepAliveCountMax
	}
	return nil
}

// APIServer implements http API server for AuthServer interface
type APIServer struct {
	APIConfig
	httprouter.Router
	clockwork.Clock
}

// NewAPIServer returns a new instance of APIServer HTTP handler
func NewAPIServer(config *APIConfig) (http.Handler, error) {
	srv := APIServer{
		APIConfig: *config,
		Clock:     clockwork.NewRealClock(),
	}
	srv.Router = *httprouter.New()

	// Kubernetes extensions
	srv.POST("/:version/kube/csr", srv.withAuth(srv.processKubeCSR))

	// Operations on certificate authorities
	srv.GET("/:version/domain", srv.withAuth(srv.getDomainName))
	srv.GET("/:version/cacert", srv.withAuth(srv.getClusterCACert))

	srv.POST("/:version/authorities/:type", srv.withAuth(srv.upsertCertAuthority))
	srv.POST("/:version/authorities/:type/rotate", srv.withAuth(srv.rotateCertAuthority))
	srv.POST("/:version/authorities/:type/rotate/external", srv.withAuth(srv.rotateExternalCertAuthority))
	srv.DELETE("/:version/authorities/:type/:domain", srv.withAuth(srv.deleteCertAuthority))
	srv.GET("/:version/authorities/:type/:domain", srv.withAuth(srv.getCertAuthority))
	srv.GET("/:version/authorities/:type", srv.withAuth(srv.getCertAuthorities))

	// Generating certificates for user and host authorities
	srv.POST("/:version/ca/host/certs", srv.withAuth(srv.generateHostCert))
	srv.POST("/:version/ca/user/certs", srv.withAuth(srv.generateUserCert)) // DELETE IN: 4.2.0

	// Operations on users
	srv.GET("/:version/users", srv.withAuth(srv.getUsers))
	srv.GET("/:version/users/:user", srv.withAuth(srv.getUser))
	srv.DELETE("/:version/users/:user", srv.withAuth(srv.deleteUser)) // DELETE IN: 5.2 REST method is replaced by grpc method with context.

	// Generating keypairs
	srv.POST("/:version/keypair", srv.withAuth(srv.generateKeyPair))

	// Passwords and sessions
	srv.POST("/:version/users", srv.withAuth(srv.upsertUser))
	srv.PUT("/:version/users/:user/web/password", srv.withAuth(srv.changePassword))
	srv.POST("/:version/users/:user/web/password", srv.withAuth(srv.upsertPassword))
	srv.POST("/:version/users/:user/web/password/check", srv.withRate(srv.withAuth(srv.checkPassword)))
	srv.POST("/:version/users/:user/web/sessions", srv.withAuth(srv.createWebSession))
	srv.POST("/:version/users/:user/web/authenticate", srv.withAuth(srv.authenticateWebUser))
	srv.POST("/:version/users/:user/ssh/authenticate", srv.withAuth(srv.authenticateSSHUser))
	srv.GET("/:version/users/:user/web/sessions/:sid", srv.withAuth(srv.getWebSession))
	srv.DELETE("/:version/users/:user/web/sessions/:sid", srv.withAuth(srv.deleteWebSession))
	srv.POST("/:version/web/password/token", srv.withRate(srv.withAuth(srv.changePasswordWithToken)))

	// Servers and presence heartbeat
	srv.POST("/:version/namespaces/:namespace/nodes", srv.withAuth(srv.upsertNode))
	srv.POST("/:version/namespaces/:namespace/nodes/keepalive", srv.withAuth(srv.keepAliveNode))
	srv.PUT("/:version/namespaces/:namespace/nodes", srv.withAuth(srv.upsertNodes))
	srv.GET("/:version/namespaces/:namespace/nodes", srv.withAuth(srv.getNodes))
	srv.DELETE("/:version/namespaces/:namespace/nodes", srv.withAuth(srv.deleteAllNodes))
	srv.DELETE("/:version/namespaces/:namespace/nodes/:name", srv.withAuth(srv.deleteNode))
	srv.POST("/:version/authservers", srv.withAuth(srv.upsertAuthServer))
	srv.GET("/:version/authservers", srv.withAuth(srv.getAuthServers))
	srv.POST("/:version/proxies", srv.withAuth(srv.upsertProxy))
	srv.GET("/:version/proxies", srv.withAuth(srv.getProxies))
	srv.DELETE("/:version/proxies", srv.withAuth(srv.deleteAllProxies))
	srv.DELETE("/:version/proxies/:name", srv.withAuth(srv.deleteProxy))
	srv.POST("/:version/tunnelconnections", srv.withAuth(srv.upsertTunnelConnection))
	srv.GET("/:version/tunnelconnections/:cluster", srv.withAuth(srv.getTunnelConnections))
	srv.GET("/:version/tunnelconnections", srv.withAuth(srv.getAllTunnelConnections))
	srv.DELETE("/:version/tunnelconnections/:cluster/:conn", srv.withAuth(srv.deleteTunnelConnection))
	srv.DELETE("/:version/tunnelconnections/:cluster", srv.withAuth(srv.deleteTunnelConnections))
	srv.DELETE("/:version/tunnelconnections", srv.withAuth(srv.deleteAllTunnelConnections))

	// Server Credentials
	srv.POST("/:version/server/credentials", srv.withAuth(srv.generateServerKeys))

	srv.POST("/:version/remoteclusters", srv.withAuth(srv.createRemoteCluster))
	srv.GET("/:version/remoteclusters/:cluster", srv.withAuth(srv.getRemoteCluster))
	srv.GET("/:version/remoteclusters", srv.withAuth(srv.getRemoteClusters))
	srv.DELETE("/:version/remoteclusters/:cluster", srv.withAuth(srv.deleteRemoteCluster))
	srv.DELETE("/:version/remoteclusters", srv.withAuth(srv.deleteAllRemoteClusters))

	// Reverse tunnels
	srv.POST("/:version/reversetunnels", srv.withAuth(srv.upsertReverseTunnel))
	srv.GET("/:version/reversetunnels", srv.withAuth(srv.getReverseTunnels))
	srv.DELETE("/:version/reversetunnels/:domain", srv.withAuth(srv.deleteReverseTunnel))

	// trusted clusters
	srv.POST("/:version/trustedclusters", srv.withAuth(srv.upsertTrustedCluster))
	srv.POST("/:version/trustedclusters/validate", srv.withAuth(srv.validateTrustedCluster))
	srv.GET("/:version/trustedclusters", srv.withAuth(srv.getTrustedClusters))
	srv.GET("/:version/trustedclusters/:name", srv.withAuth(srv.getTrustedCluster))
	srv.DELETE("/:version/trustedclusters/:name", srv.withAuth(srv.deleteTrustedCluster))

	// Tokens
	srv.POST("/:version/tokens", srv.withAuth(srv.generateToken))
	srv.POST("/:version/tokens/register", srv.withAuth(srv.registerUsingToken))
	srv.POST("/:version/tokens/register/auth", srv.withAuth(srv.registerNewAuthServer))

	// active sesssions
	srv.POST("/:version/namespaces/:namespace/sessions", srv.withAuth(srv.createSession))
	srv.PUT("/:version/namespaces/:namespace/sessions/:id", srv.withAuth(srv.updateSession))
	srv.DELETE("/:version/namespaces/:namespace/sessions/:id", srv.withAuth(srv.deleteSession))
	srv.GET("/:version/namespaces/:namespace/sessions", srv.withAuth(srv.getSessions))
	srv.GET("/:version/namespaces/:namespace/sessions/:id", srv.withAuth(srv.getSession))
	srv.POST("/:version/namespaces/:namespace/sessions/:id/slice", srv.withAuth(srv.postSessionSlice))
	srv.POST("/:version/namespaces/:namespace/sessions/:id/recording", srv.withAuth(srv.uploadSessionRecording))
	srv.GET("/:version/namespaces/:namespace/sessions/:id/stream", srv.withAuth(srv.getSessionChunk))
	srv.GET("/:version/namespaces/:namespace/sessions/:id/events", srv.withAuth(srv.getSessionEvents))

	// Namespaces
	srv.POST("/:version/namespaces", srv.withAuth(srv.upsertNamespace))
	srv.GET("/:version/namespaces", srv.withAuth(srv.getNamespaces))
	srv.GET("/:version/namespaces/:namespace", srv.withAuth(srv.getNamespace))
	srv.DELETE("/:version/namespaces/:namespace", srv.withAuth(srv.deleteNamespace))

	// Roles - Moved to grpc
	// DELETE IN 7.0
	srv.POST("/:version/roles", srv.withAuth(srv.upsertRole))
	srv.GET("/:version/roles", srv.withAuth(srv.getRoles))
	srv.GET("/:version/roles/:role", srv.withAuth(srv.getRole))
	srv.DELETE("/:version/roles/:role", srv.withAuth(srv.deleteRole))

	// cluster configuration
	srv.GET("/:version/configuration", srv.withAuth(srv.getClusterConfig))
	srv.POST("/:version/configuration", srv.withAuth(srv.setClusterConfig))
	srv.GET("/:version/configuration/name", srv.withAuth(srv.getClusterName))
	srv.POST("/:version/configuration/name", srv.withAuth(srv.setClusterName))
	srv.GET("/:version/configuration/static_tokens", srv.withAuth(srv.getStaticTokens))
	srv.DELETE("/:version/configuration/static_tokens", srv.withAuth(srv.deleteStaticTokens))
	srv.POST("/:version/configuration/static_tokens", srv.withAuth(srv.setStaticTokens))
	srv.GET("/:version/authentication/preference", srv.withAuth(srv.getClusterAuthPreference))
	srv.POST("/:version/authentication/preference", srv.withAuth(srv.setClusterAuthPreference))

	// OIDC
	srv.POST("/:version/oidc/connectors", srv.withAuth(srv.upsertOIDCConnector))
	srv.GET("/:version/oidc/connectors", srv.withAuth(srv.getOIDCConnectors))
	srv.GET("/:version/oidc/connectors/:id", srv.withAuth(srv.getOIDCConnector))
	srv.DELETE("/:version/oidc/connectors/:id", srv.withAuth(srv.deleteOIDCConnector))
	srv.POST("/:version/oidc/requests/create", srv.withAuth(srv.createOIDCAuthRequest))
	srv.POST("/:version/oidc/requests/validate", srv.withAuth(srv.validateOIDCAuthCallback))

	// SAML handlers
	srv.POST("/:version/saml/connectors", srv.withAuth(srv.createSAMLConnector))
	srv.PUT("/:version/saml/connectors", srv.withAuth(srv.upsertSAMLConnector))
	srv.GET("/:version/saml/connectors", srv.withAuth(srv.getSAMLConnectors))
	srv.GET("/:version/saml/connectors/:id", srv.withAuth(srv.getSAMLConnector))
	srv.DELETE("/:version/saml/connectors/:id", srv.withAuth(srv.deleteSAMLConnector))
	srv.POST("/:version/saml/requests/create", srv.withAuth(srv.createSAMLAuthRequest))
	srv.POST("/:version/saml/requests/validate", srv.withAuth(srv.validateSAMLResponse))

	// Github connector
	srv.POST("/:version/github/connectors", srv.withAuth(srv.createGithubConnector))
	srv.PUT("/:version/github/connectors", srv.withAuth(srv.upsertGithubConnector))
	srv.GET("/:version/github/connectors", srv.withAuth(srv.getGithubConnectors))
	srv.GET("/:version/github/connectors/:id", srv.withAuth(srv.getGithubConnector))
	srv.DELETE("/:version/github/connectors/:id", srv.withAuth(srv.deleteGithubConnector))
	srv.POST("/:version/github/requests/create", srv.withAuth(srv.createGithubAuthRequest))
	srv.POST("/:version/github/requests/validate", srv.withAuth(srv.validateGithubAuthCallback))

	// U2F
	srv.GET("/:version/u2f/signuptokens/:token", srv.withAuth(srv.getSignupU2FRegisterRequest))
	srv.POST("/:version/u2f/users/:user/sign", srv.withAuth(srv.u2fSignRequest))
	srv.GET("/:version/u2f/appid", srv.withAuth(srv.getU2FAppID))

	// Provisioning tokens- Moved to grpc
	// DELETE IN 8.0
	srv.GET("/:version/tokens", srv.withAuth(srv.getTokens))
	srv.GET("/:version/tokens/:token", srv.withAuth(srv.getToken))
	srv.DELETE("/:version/tokens/:token", srv.withAuth(srv.deleteToken))

	// Audit logs AKA events
	srv.POST("/:version/events", srv.withAuth(srv.emitAuditEvent))
	srv.GET("/:version/events", srv.withAuth(srv.searchEvents))
	srv.GET("/:version/events/session", srv.withAuth(srv.searchSessionEvents))

	if config.PluginRegistry != nil {
		if err := config.PluginRegistry.RegisterAuthWebHandlers(&srv); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return httplib.RewritePaths(&srv.Router,
		httplib.Rewrite("/v1/nodes", "/v1/namespaces/default/nodes"),
		httplib.Rewrite("/v1/sessions", "/v1/namespaces/default/sessions"),
		httplib.Rewrite("/v1/sessions/([^/]+)/(.*)", "/v1/namespaces/default/sessions/$1/$2"),
	), nil
}

// apiServerAuth defines the auth server as required by the web-based APIServer
type apiServerAuth interface {
	libauth.AccessPoint
	libauth.Presence
	libauth.Trust
	libauth.ClusterConfiguration
	libauth.Access

	serverIdentity
	webService
	serverCluster
	serverProvisioning

	session.Service

	events.IAuditLog

	types.WebSessionsGetter
	types.WebTokensGetter
}

// serverCluster represents a API view of the auth server
type serverCluster interface {
	// GetClusterCACert returns the CAs for the local cluster without signing keys.
	GetClusterCACert() (*LocalCAResponse, error)

	// ProcessKubeCSR processes CSR request against Kubernetes CA, returns
	// signed certificate if successful.
	ProcessKubeCSR(req KubeCSR) (*KubeCSRResponse, error)

	// GenerateDatabaseCert generates client certificate used by a database
	// service to authenticate with the database instance.
	GenerateDatabaseCert(context.Context, *proto.DatabaseCertRequest) (*proto.DatabaseCertResponse, error)

	// GetDomainName returns auth server cluster name
	GetDomainName() (string, error)

	// ValidateTrustedCluster validates trusted cluster token with
	// main cluster, in case if validation is successful, main cluster
	// adds remote cluster
	ValidateTrustedCluster(*ValidateTrustedClusterRequest) (*ValidateTrustedClusterResponse, error)
}

// serverProvisioning is a service in control
// of adding new nodes, auth servers and proxies to the cluster
type serverProvisioning interface {
	// GetTokens returns a list of active invitation tokens for nodes and users
	GetTokens(ctx context.Context, opts ...libauth.MarshalOption) (tokens []types.ProvisionToken, err error)

	// GetToken returns provisioning token
	GetToken(ctx context.Context, token string) (types.ProvisionToken, error)

	// DeleteToken deletes a given provisioning token on the auth server (CA). It
	// could be a reset password token or a machine token
	DeleteToken(ctx context.Context, token string) error

	// UpsertToken adds provisioning tokens for the auth server
	UpsertToken(ctx context.Context, token types.ProvisionToken) error

	// RegisterUsingToken calls the auth service API to register a new node via registration token
	// which has been previously issued via GenerateToken
	RegisterUsingToken(req RegisterUsingTokenRequest) (*PackedKeys, error)

	// RegisterNewAuthServer is used to register new auth server with token
	RegisterNewAuthServer(ctx context.Context, token string) error
}

// webService represents the web UI interface presented by the APIServer
type webService interface {
	// CreateAppSession creates an application web session. Application web
	// sessions represent a browser session the client holds.
	CreateAppSession(context.Context, types.CreateAppSessionRequest) (types.WebSession, error)

	// GetWebSession queries the existing web session described with req.
	// Implements ReadAccessPoint.
	GetWebSession(ctx context.Context, req types.GetWebSessionRequest) (types.WebSession, error)

	// GetWebToken queries the existing web token described with req.
	// Implements ReadAccessPoint.
	GetWebToken(ctx context.Context, req types.GetWebTokenRequest) (types.WebToken, error)

	// GetWebSessionInfo checks if a web sesion is valid, returns session id in case if
	// it is valid, or error otherwise.
	GetWebSessionInfo(ctx context.Context, user, sessionID string) (types.WebSession, error)

	// ExtendWebSession creates a new web session for a user based on another
	// valid web session
	ExtendWebSession(user, prevSessionID, accessRequestID string) (types.WebSession, error)

	// CreateWebSession creates a new web session for a user
	CreateWebSession(user string) (types.WebSession, error)

	// AuthenticateWebUser authenticates web user, creates and returns web session
	// if authentication is successful
	AuthenticateWebUser(req AuthenticateUserRequest) (types.WebSession, error)

	// AuthenticateSSHUser authenticates SSH console user, creates and  returns a pair of signed TLS and SSH
	// short lived certificates as a result
	AuthenticateSSHUser(req AuthenticateSSHRequest) (*SSHLoginResponse, error)

	// AppSession defines application session features.
	libauth.AppSession
}

// serverIdentity manages identities and users
type serverIdentity interface {
	// UpsertPassword updates web access password for the user
	UpsertPassword(user string, password []byte) error

	// UpsertOIDCConnector updates or creates OIDC connector
	UpsertOIDCConnector(ctx context.Context, connector types.OIDCConnector) error

	// GetOIDCConnector returns OIDC connector information by id
	GetOIDCConnector(ctx context.Context, id string, withSecrets bool) (types.OIDCConnector, error)

	// GetOIDCConnector gets OIDC connectors list
	GetOIDCConnectors(ctx context.Context, withSecrets bool) ([]types.OIDCConnector, error)

	// DeleteOIDCConnector deletes OIDC connector by ID
	DeleteOIDCConnector(ctx context.Context, connectorID string) error

	// CreateOIDCAuthRequest creates OIDCAuthRequest
	CreateOIDCAuthRequest(req libauth.OIDCAuthRequest) (*libauth.OIDCAuthRequest, error)

	// CreateSAMLConnector creates SAML connector
	CreateSAMLConnector(ctx context.Context, connector types.SAMLConnector) error

	// UpsertSAMLConnector updates or creates SAML connector
	UpsertSAMLConnector(ctx context.Context, connector types.SAMLConnector) error

	// GetSAMLConnector returns SAML connector information by id
	GetSAMLConnector(ctx context.Context, id string, withSecrets bool) (types.SAMLConnector, error)

	// GetSAMLConnector gets SAML connectors list
	GetSAMLConnectors(ctx context.Context, withSecrets bool) ([]types.SAMLConnector, error)

	// DeleteSAMLConnector deletes SAML connector by ID
	DeleteSAMLConnector(ctx context.Context, connectorID string) error

	// CreateSAMLAuthRequest creates SAML AuthnRequest
	CreateSAMLAuthRequest(req libauth.SAMLAuthRequest) (*libauth.SAMLAuthRequest, error)

	// CreateGithubConnector creates a new Github connector
	CreateGithubConnector(connector types.GithubConnector) error
	// UpsertGithubConnector creates or updates a Github connector
	UpsertGithubConnector(ctx context.Context, connector types.GithubConnector) error
	// GetGithubConnectors returns all configured Github connectors
	GetGithubConnectors(ctx context.Context, withSecrets bool) ([]types.GithubConnector, error)
	// GetGithubConnector returns the specified Github connector
	GetGithubConnector(ctx context.Context, id string, withSecrets bool) (types.GithubConnector, error)
	// DeleteGithubConnector deletes the specified Github connector
	DeleteGithubConnector(ctx context.Context, id string) error
	// CreateGithubAuthRequest creates a new request for Github OAuth2 flow
	CreateGithubAuthRequest(libauth.GithubAuthRequest) (*libauth.GithubAuthRequest, error)

	// GetUser returns user by name
	GetUser(name string, withSecrets bool) (types.User, error)

	// CreateUser inserts a new entry in a backend.
	CreateUser(ctx context.Context, user types.User) error

	// UpdateUser updates an existing user in a backend.
	UpdateUser(ctx context.Context, user types.User) error

	// UpsertUser user updates or inserts user entry
	UpsertUser(user types.User) error

	// DeleteUser deletes an existng user in a backend by username.
	DeleteUser(ctx context.Context, user string) error

	// GetUsers returns a list of usernames registered in the system
	GetUsers(withSecrets bool) ([]types.User, error)

	// ChangePassword changes user password
	ChangePassword(req libauth.ChangePasswordReq) error

	// CheckPassword checks if the suplied web access password is valid.
	CheckPassword(user string, password []byte, otpToken string) error

	// // GetSignupU2FRegisterRequest generates sign request for user trying to sign up with invite token
	GetSignupU2FRegisterRequest(token string) (*u2f.RegisterChallenge, error)

	// GenerateToken creates a special provisioning token for a new SSH server
	// that is valid for ttl period seconds.
	//
	// This token is used by SSH server to authenticate with Auth server
	// and get signed certificate and private key from the auth server.
	//
	// If token is not supplied, it will be auto generated and returned.
	// If TTL is not supplied, token will be valid until removed.
	GenerateToken(ctx context.Context, req GenerateTokenRequest) (string, error)

	// CreateResetPasswordToken creates a new user reset token
	CreateResetPasswordToken(ctx context.Context, req CreateResetPasswordTokenRequest) (types.ResetPasswordToken, error)

	// ChangePasswordWithToken changes password with token
	ChangePasswordWithToken(ctx context.Context, req ChangePasswordWithTokenRequest) (types.WebSession, error)

	// RotateCertAuthority starts or restarts certificate authority rotation process.
	RotateCertAuthority(RotateRequest) error

	// GenerateServerKeys generates new host private keys and certificates (signed
	// by the host certificate authority) for a node
	GenerateServerKeys(GenerateServerKeysRequest) (*PackedKeys, error)

	// // ValidateGithubAuthCallback validates Github auth callback
	ValidateGithubAuthCallback(q url.Values) (*GithubAuthResponse, error)

	// // ValidateSAMLResponse validates SAML auth response
	ValidateSAMLResponse(re string) (*SAMLAuthResponse, error)

	// // ValidateOIDCAuthCallback validates OIDC auth callback returned from redirect
	ValidateOIDCAuthCallback(q url.Values) (*OIDCAuthResponse, error)

	// GetMFAAuthenticateChallenge generates request for user trying to authenticate with U2F token
	GetMFAAuthenticateChallenge(user string, password []byte) (*MFAAuthenticateChallenge, error)

	// GenerateKeyPair generates SSH private/public key pair optionally protected
	// by password. If the pass parameter is an empty string, the key pair
	// is not password-protected.
	GenerateKeyPair(pass string) ([]byte, []byte, error)

	// GenerateHostCert takes the public key in the Open SSH ``authorized_keys``
	// plain text format, signs it using Host Certificate Authority private key and returns the
	// resulting certificate.
	GenerateHostCert(key []byte, hostID, nodeName string, principals []string, clusterName string, roles teleport.Roles, ttl time.Duration) ([]byte, error)

	// GenerateUserCerts takes the public key in the OpenSSH `authorized_keys` plain
	// text format, signs it using User Certificate Authority signing key and
	// returns the resulting certificates.
	GenerateUserCerts(ctx context.Context, req proto.UserCertsRequest) (*proto.Certs, error)

	// GetResetPasswordToken returns token
	GetResetPasswordToken(ctx context.Context, username string) (types.ResetPasswordToken, error)

	// RotateResetPasswordTokenSecrets rotates token secrets for a given tokenID
	RotateResetPasswordTokenSecrets(ctx context.Context, tokenID string) (types.ResetPasswordTokenSecrets, error)

	// RotateExternalCertAuthority rotates external certificate authority,
	// this method is used to update only public keys and certificates of the
	// the certificate authorities of trusted clusters.
	RotateExternalCertAuthority(types.CertAuthority) error
}

// HandlerWithAuthFunc is http handler with passed auth context
type HandlerWithAuthFunc func(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error)

func (s *APIServer) withAuth(handler HandlerWithAuthFunc) httprouter.Handle {
	const accessDeniedMsg = "auth API: access denied "
	return httplib.MakeHandler(func(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
		// HTTPS server expects auth context to be set by the auth middleware
		authContext, err := s.Authorizer.Authorize(r.Context())
		if err != nil {
			// propagate connection problem error so we can differentiate
			// between connection failed and access denied
			if trace.IsConnectionProblem(err) {
				return nil, trace.ConnectionProblem(err, "[07] failed to connect to the database")
			} else if trace.IsAccessDenied(err) {
				// don't print stack trace, just log the warning
				log.Warn(err)
			} else {
				log.Warn(trace.DebugReport(err))
			}

			return nil, trace.AccessDenied(accessDeniedMsg + "[00]")
		}
		auth := &ServerWithRoles{
			authServer: s.AuthServer,
			context:    *authContext,
			sessions:   s.SessionService,
			alog:       s.AuthServer.IAuditLog,
		}
		version := p.ByName("version")
		if version == "" {
			return nil, trace.BadParameter("missing version")
		}
		return handler(auth, w, r, p, version)
	})
}

// withRate wrap a rate limiter around the passed in httprouter.Handle and
// returns a httprouter.Handle. Because the rate limiter wraps a http.Handler,
// internally withRate converts to the standard handler and back.
func (s *APIServer) withRate(handle httprouter.Handle) httprouter.Handle {
	limiter := defaults.CheckPasswordLimiter()

	fromStandard := func(h http.Handler) httprouter.Handle {
		return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
			ctx := context.WithValue(r.Context(), contextParams, p)
			r = r.WithContext(ctx)
			h.ServeHTTP(w, r)
		}
	}
	toStandard := func(handle httprouter.Handle) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := r.Context().Value(contextParams).(httprouter.Params)
			if !ok {
				trace.WriteError(w, trace.BadParameter("parameters missing from request"))
				return
			}
			handle(w, r, p)
		})
	}
	limiter.WrapHandle(toStandard(handle))
	return fromStandard(limiter)
}

type UpsertServerRawReq struct {
	Server json.RawMessage `json:"server"`
	TTL    time.Duration   `json:"ttl"`
}

// upsertServer is a common utility function
func (s *APIServer) upsertServer(auth libauth.Presence, role teleport.Role, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req UpsertServerRawReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	var kind string
	switch role {
	case teleport.RoleNode:
		kind = services.KindNode
	case teleport.RoleAuth:
		kind = services.KindAuthServer
	case teleport.RoleProxy:
		kind = services.KindProxy
	default:
		return nil, trace.BadParameter("upsertServer with unknown role: %q", role)
	}
	server, err := resource.UnmarshalServer(req.Server, kind)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// if server sent "local" IP address to us, replace the ip/host part with the remote address we see
	// on the socket, but keep the original port:
	server.SetAddr(utils.ReplaceLocalhost(server.GetAddr(), r.RemoteAddr))
	if req.TTL != 0 {
		server.SetExpiry(s.Now().UTC().Add(req.TTL))
	}
	switch role {
	case teleport.RoleNode:
		namespace := p.ByName("namespace")
		if !services.IsValidNamespace(namespace) {
			return nil, trace.BadParameter("invalid namespace %q", namespace)
		}
		server.SetNamespace(namespace)
		handle, err := auth.UpsertNode(server)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return handle, nil
	case teleport.RoleAuth:
		if err := auth.UpsertAuthServer(server); err != nil {
			return nil, trace.Wrap(err)
		}
	case teleport.RoleProxy:
		if err := auth.UpsertProxy(server); err != nil {
			return nil, trace.Wrap(err)
		}
	default:
		return nil, trace.BadParameter("unknown server role %q", role)
	}
	return message("ok"), nil
}

// keepAliveNode updates node TTL in the backend
func (s *APIServer) keepAliveNode(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var handle services.KeepAlive
	if err := httplib.ReadJSON(r, &handle); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.KeepAliveServer(r.Context(), handle); err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

type UpsertNodesReq struct {
	Nodes     json.RawMessage `json:"nodes"`
	Namespace string          `json:"namespace"`
}

// upsertNodes is used to bulk insert nodes into the backend.
func (s *APIServer) upsertNodes(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var req UpsertNodesReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	if !services.IsValidNamespace(req.Namespace) {
		return nil, trace.BadParameter("invalid namespace %q", req.Namespace)
	}

	nodes, err := resource.UnmarshalServers(req.Nodes)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = auth.UpsertNodes(req.Namespace, nodes)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return message("ok"), nil
}

// upsertNode is called by remote SSH nodes when they ping back into the auth service
func (s *APIServer) upsertNode(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	return s.upsertServer(auth, teleport.RoleNode, r, p)
}

// getNodes returns registered SSH nodes
func (s *APIServer) getNodes(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	namespace := p.ByName("namespace")
	if !services.IsValidNamespace(namespace) {
		return nil, trace.BadParameter("invalid namespace %q", namespace)
	}
	skipValidation, _, err := httplib.ParseBool(r.URL.Query(), "skip_validation")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var opts []libauth.MarshalOption
	if skipValidation {
		opts = append(opts, resource.SkipValidation())
	}

	servers, err := auth.GetNodes(namespace, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return marshalServers(servers, version)
}

// deleteAllNodes deletes all nodes
func (s *APIServer) deleteAllNodes(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	namespace := p.ByName("namespace")
	if !services.IsValidNamespace(namespace) {
		return nil, trace.BadParameter("invalid namespace %q", namespace)
	}
	err := auth.DeleteAllNodes(namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

// deleteNode deletes node
func (s *APIServer) deleteNode(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	namespace := p.ByName("namespace")
	if !services.IsValidNamespace(namespace) {
		return nil, trace.BadParameter("invalid namespace %q", namespace)
	}
	name := p.ByName("name")
	if name == "" {
		return nil, trace.BadParameter("missing node name")
	}
	err := auth.DeleteNode(namespace, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

// upsertProxy is called by remote SSH nodes when they ping back into the auth service
func (s *APIServer) upsertProxy(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	return s.upsertServer(auth, teleport.RoleProxy, r, p)
}

// getProxies returns registered proxies
func (s *APIServer) getProxies(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	servers, err := auth.GetProxies()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return marshalServers(servers, version)
}

// deleteAllProxies deletes all proxies
func (s *APIServer) deleteAllProxies(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	err := auth.DeleteAllProxies()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

// deleteProxy deletes proxy
func (s *APIServer) deleteProxy(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	name := p.ByName("name")
	if name == "" {
		return nil, trace.BadParameter("missing proxy name")
	}
	err := auth.DeleteProxy(name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

// upsertAuthServer is called by remote Auth servers when they ping back into the auth service
func (s *APIServer) upsertAuthServer(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	return s.upsertServer(auth, teleport.RoleAuth, r, p)
}

// getAuthServers returns registered auth servers
func (s *APIServer) getAuthServers(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	servers, err := auth.GetAuthServers()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return marshalServers(servers, version)
}

func marshalServers(servers []services.Server, version string) (interface{}, error) {
	items := make([]json.RawMessage, len(servers))
	for i, server := range servers {
		data, err := resource.MarshalServer(server, resource.WithVersion(version), resource.PreserveResourceID())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		items[i] = data
	}
	return items, nil
}

type UpsertReverseTunnelRawReq struct {
	ReverseTunnel json.RawMessage `json:"reverse_tunnel"`
	TTL           time.Duration   `json:"ttl"`
}

// upsertReverseTunnel is called by admin to create a reverse tunnel to remote proxy
func (s *APIServer) upsertReverseTunnel(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var req UpsertReverseTunnelRawReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	tun, err := resource.UnmarshalReverseTunnel(req.ReverseTunnel)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := libauth.ValidateReverseTunnel(tun); err != nil {
		return nil, trace.Wrap(err)
	}
	if req.TTL != 0 {
		tun.SetExpiry(s.Now().UTC().Add(req.TTL))
	}
	if err := auth.UpsertReverseTunnel(tun); err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

// getReverseTunnels returns a list of reverse tunnels
func (s *APIServer) getReverseTunnels(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	reverseTunnels, err := auth.GetReverseTunnels()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	items := make([]json.RawMessage, len(reverseTunnels))
	for i, tunnel := range reverseTunnels {
		data, err := resource.MarshalReverseTunnel(tunnel, resource.WithVersion(version), resource.PreserveResourceID())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		items[i] = data
	}
	return items, nil
}

// deleteReverseTunnel deletes reverse tunnel
func (s *APIServer) deleteReverseTunnel(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	domainName := p.ByName("domain")
	err := auth.DeleteReverseTunnel(domainName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return message(fmt.Sprintf("reverse tunnel %v deleted", domainName)), nil
}

type UpsertTrustedClusterReq struct {
	TrustedCluster json.RawMessage `json:"trusted_cluster"`
}

// upsertTrustedCluster creates or updates a trusted cluster.
func (s *APIServer) upsertTrustedCluster(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var req *UpsertTrustedClusterReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	trustedCluster, err := resource.UnmarshalTrustedCluster(req.TrustedCluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := libauth.ValidateTrustedCluster(trustedCluster); err != nil {
		return nil, trace.Wrap(err)
	}
	out, err := auth.UpsertTrustedCluster(r.Context(), trustedCluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return rawMessage(resource.MarshalTrustedCluster(out, resource.WithVersion(version), resource.PreserveResourceID()))
}

func (s *APIServer) validateTrustedCluster(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var validateRequestRaw ValidateTrustedClusterRequestRaw
	if err := httplib.ReadJSON(r, &validateRequestRaw); err != nil {
		return nil, trace.Wrap(err)
	}

	validateRequest, err := validateRequestRaw.ToNative()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	validateResponse, err := auth.ValidateTrustedCluster(validateRequest)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	validateResponseRaw, err := validateResponse.ToRaw()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return validateResponseRaw, nil
}

func (s *APIServer) getTrustedCluster(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	return auth.GetTrustedCluster(r.Context(), p.ByName("name"))
}

func (s *APIServer) getTrustedClusters(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	return auth.GetTrustedClusters(r.Context())
}

// deleteTrustedCluster deletes a trusted cluster by name.
func (s *APIServer) deleteTrustedCluster(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	err := auth.DeleteTrustedCluster(r.Context(), p.ByName("name"))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return message("ok"), nil
}

// getTokens returns a list of active provisioning tokens. expired (inactive) tokens are not returned
func (s *APIServer) getTokens(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	tokens, err := auth.GetTokens(r.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return services.ProvisionTokensToV1(tokens), nil
}

// getTokens returns provisioning token by name
func (s *APIServer) getToken(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	token, err := auth.GetToken(r.Context(), p.ByName("token"))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return token, nil
}

// deleteToken deletes (revokes) a token by its value
func (s *APIServer) deleteToken(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	token := p.ByName("token")
	if err := auth.DeleteToken(r.Context(), token); err != nil {
		return nil, trace.Wrap(err)
	}
	return message(fmt.Sprintf("Token %v deleted", token)), nil
}

func (s *APIServer) deleteWebSession(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	user, sessionID := p.ByName("user"), p.ByName("sid")
	err := auth.WebSessions().Delete(r.Context(), types.DeleteWebSessionRequest{
		User:      user,
		SessionID: sessionID,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return message(fmt.Sprintf("session %q for user %q deleted", sessionID, user)), nil
}

func (s *APIServer) getWebSession(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	user, sid := p.ByName("user"), p.ByName("sid")
	sess, err := auth.GetWebSessionInfo(r.Context(), user, sid)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return rawMessage(resource.MarshalWebSession(sess, resource.WithVersion(version)))
}

// DELETE IN: 4.2.0
type generateUserCertReq struct {
	Key           []byte        `json:"key"`
	User          string        `json:"user"`
	TTL           time.Duration `json:"ttl"`
	Compatibility string        `json:"compatibility,omitempty"`
}

// DELETE IN: 4.2.0
func (s *APIServer) generateUserCert(auth apiServerAuth, w http.ResponseWriter, r *http.Request, _ httprouter.Params, version string) (interface{}, error) {
	var req *generateUserCertReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	certificateFormat, err := utils.CheckCertificateFormatFlag(req.Compatibility)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	certs, err := auth.GenerateUserCerts(r.Context(), proto.UserCertsRequest{
		PublicKey: req.Key,
		Username:  req.User,
		Expires:   s.Now().UTC().Add(req.TTL),
		Format:    certificateFormat,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return string(certs.SSH), nil
}

type SignInReq struct {
	Password string `json:"password"`
}

func (s *APIServer) u2fSignRequest(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var req *SignInReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	user := p.ByName("user")
	pass := []byte(req.Password)
	u2fSignReq, err := auth.GetMFAAuthenticateChallenge(user, pass)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return u2fSignReq, nil
}

type CreateWebSessionReq struct {
	PrevSessionID   string `json:"prev_session_id"`
	AccessRequestID string `json:"access_request_id"`
}

func (s *APIServer) createWebSession(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var req *CreateWebSessionReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	user := p.ByName("user")
	if req.PrevSessionID != "" {
		sess, err := auth.ExtendWebSession(user, req.PrevSessionID, req.AccessRequestID)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return sess, nil
	}
	sess, err := auth.CreateWebSession(user)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return rawMessage(resource.MarshalWebSession(sess, resource.WithVersion(version)))
}

func (s *APIServer) authenticateWebUser(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var req AuthenticateUserRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	req.Username = p.ByName("user")
	sess, err := auth.AuthenticateWebUser(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return rawMessage(resource.MarshalWebSession(sess, resource.WithVersion(version)))
}

func (s *APIServer) authenticateSSHUser(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var req AuthenticateSSHRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	req.Username = p.ByName("user")
	return auth.AuthenticateSSHUser(req)
}

// changePassword updates users password based on the old password.
func (s *APIServer) changePassword(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var req libauth.ChangePasswordReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := auth.ChangePassword(req); err != nil {
		return nil, trace.Wrap(err)
	}

	return message(fmt.Sprintf("password has been changed for user %q", req.User)), nil
}

type UpsertPasswordReq struct {
	Password string `json:"password"`
}

func (s *APIServer) upsertPassword(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var req *UpsertPasswordReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	user := p.ByName("user")
	err := auth.UpsertPassword(user, []byte(req.Password))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return message(fmt.Sprintf("password for for user %q upserted", user)), nil
}

type UpsertUserRawReq struct {
	User json.RawMessage `json:"user"`
}

func (s *APIServer) upsertUser(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var req *UpsertUserRawReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	user, err := resource.UnmarshalUser(req.User)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = auth.UpsertUser(user)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return message(fmt.Sprintf("'%v' user upserted", user.GetName())), nil
}

type CheckPasswordReq struct {
	Password string `json:"password"`
	OTPToken string `json:"otp_token"`
}

func (s *APIServer) checkPassword(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var req CheckPasswordReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	user := p.ByName("user")
	if err := auth.CheckPassword(user, []byte(req.Password), req.OTPToken); err != nil {
		return nil, trace.Wrap(err)
	}

	return message(fmt.Sprintf("%q user password matches", user)), nil
}

func (s *APIServer) getUser(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	user, err := auth.GetUser(p.ByName("user"), false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return rawMessage(resource.MarshalUser(user, resource.WithVersion(version), resource.PreserveResourceID()))
}

func rawMessage(data []byte, err error) (interface{}, error) {
	if err != nil {
		return nil, trace.Wrap(err)
	}
	m := json.RawMessage(data)
	return &m, nil
}

func (s *APIServer) getUsers(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	users, err := auth.GetUsers(false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out := make([]json.RawMessage, len(users))
	for i, user := range users {
		data, err := resource.MarshalUser(user, resource.WithVersion(version), resource.PreserveResourceID())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out[i] = data
	}
	return out, nil
}

// DELETE IN: 5.2 REST method is replaced by grpc method with context.
func (s *APIServer) deleteUser(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	user := p.ByName("user")
	if err := auth.DeleteUser(r.Context(), user); err != nil {
		return nil, trace.Wrap(err)
	}
	return message(fmt.Sprintf("user %q deleted", user)), nil
}

type GenerateKeyPairReq struct {
	Password string `json:"password"`
}

type GenerateKeyPairResponse struct {
	PrivKey []byte `json:"privkey"`
	PubKey  string `json:"pubkey"`
}

func (s *APIServer) generateKeyPair(auth apiServerAuth, w http.ResponseWriter, r *http.Request, _ httprouter.Params, version string) (interface{}, error) {
	var req *GenerateKeyPairReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	priv, pub, err := auth.GenerateKeyPair(req.Password)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &GenerateKeyPairResponse{PrivKey: priv, PubKey: string(pub)}, nil
}

type GenerateHostCertReq struct {
	Key         []byte         `json:"key"`
	HostID      string         `json:"hostname"`
	NodeName    string         `json:"node_name"`
	Principals  []string       `json:"principals"`
	ClusterName string         `json:"auth_domain"`
	Roles       teleport.Roles `json:"roles"`
	TTL         time.Duration  `json:"ttl"`
}

func (s *APIServer) generateHostCert(auth apiServerAuth, w http.ResponseWriter, r *http.Request, _ httprouter.Params, version string) (interface{}, error) {
	var req *GenerateHostCertReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	cert, err := auth.GenerateHostCert(req.Key, req.HostID, req.NodeName, req.Principals, req.ClusterName, req.Roles, req.TTL)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return string(cert), nil
}

func (s *APIServer) generateToken(auth apiServerAuth, w http.ResponseWriter, r *http.Request, _ httprouter.Params, version string) (interface{}, error) {
	var req GenerateTokenRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	token, err := auth.GenerateToken(r.Context(), req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return token, nil
}

func (s *APIServer) registerUsingToken(auth apiServerAuth, w http.ResponseWriter, r *http.Request, _ httprouter.Params, version string) (interface{}, error) {
	var req RegisterUsingTokenRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	// Pass along the remote address the request came from to the registration function.
	req.RemoteAddr = r.RemoteAddr

	keys, err := auth.RegisterUsingToken(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return keys, nil
}

type RegisterNewAuthServerReq struct {
	Token string `json:"token"`
}

func (s *APIServer) registerNewAuthServer(auth apiServerAuth, w http.ResponseWriter, r *http.Request, _ httprouter.Params, version string) (interface{}, error) {
	var req *RegisterNewAuthServerReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	err := auth.RegisterNewAuthServer(r.Context(), req.Token)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

func (s *APIServer) generateServerKeys(auth apiServerAuth, w http.ResponseWriter, r *http.Request, _ httprouter.Params, version string) (interface{}, error) {
	var req GenerateServerKeysRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	// Pass along the remote address the request came from to the registration function.
	req.RemoteAddr = r.RemoteAddr

	keys, err := auth.GenerateServerKeys(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return keys, nil
}

func (s *APIServer) rotateCertAuthority(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var req RotateRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.RotateCertAuthority(req); err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

type UpsertCertAuthorityRawReq struct {
	CA  json.RawMessage `json:"ca"`
	TTL time.Duration   `json:"ttl"`
}

func (s *APIServer) upsertCertAuthority(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var req *UpsertCertAuthorityRawReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	ca, err := resource.UnmarshalCertAuthority(req.CA)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if req.TTL != 0 {
		ca.SetExpiry(s.Now().UTC().Add(req.TTL))
	}
	if err = libauth.ValidateCertAuthority(ca); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.UpsertCertAuthority(ca); err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

type RotateExternalCertAuthorityRawReq struct {
	CA json.RawMessage `json:"ca"`
}

func (s *APIServer) rotateExternalCertAuthority(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var req RotateExternalCertAuthorityRawReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	ca, err := resource.UnmarshalCertAuthority(req.CA)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.RotateExternalCertAuthority(ca); err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

func (s *APIServer) getCertAuthorities(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	loadKeys, _, err := httplib.ParseBool(r.URL.Query(), "load_keys")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	certs, err := auth.GetCertAuthorities(services.CertAuthType(p.ByName("type")), loadKeys)

	if err != nil {
		return nil, trace.Wrap(err)
	}
	items := make([]json.RawMessage, len(certs))
	for i, cert := range certs {
		data, err := resource.MarshalCertAuthority(cert, resource.WithVersion(version), resource.PreserveResourceID())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		items[i] = data
	}
	return items, nil
}

func (s *APIServer) getCertAuthority(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	loadKeys, _, err := httplib.ParseBool(r.URL.Query(), "load_keys")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	id := services.CertAuthID{
		Type:       services.CertAuthType(p.ByName("type")),
		DomainName: p.ByName("domain"),
	}
	ca, err := auth.GetCertAuthority(id, loadKeys)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return rawMessage(resource.MarshalCertAuthority(ca, resource.WithVersion(version), resource.PreserveResourceID()))
}

func (s *APIServer) getDomainName(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	domain, err := auth.GetDomainName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return domain, nil
}

// getClusterCACert returns the CAs for the local cluster without signing keys.
func (s *APIServer) getClusterCACert(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	localCA, err := auth.GetClusterCACert()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return localCA, nil
}

func (s *APIServer) changePasswordWithToken(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var req ChangePasswordWithTokenRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	webSession, err := auth.ChangePasswordWithToken(r.Context(), req)
	if err != nil {
		log.Debugf("Failed to change user password with token: %v.", err)
		return nil, trace.Wrap(err)
	}

	return rawMessage(resource.MarshalWebSession(webSession, resource.WithVersion(version)))
}

// getU2FAppID returns the U2F AppID in the auth configuration
func (s *APIServer) getU2FAppID(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	cap, err := auth.GetAuthPreference()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	universalSecondFactor, err := cap.GetU2F()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	w.Header().Set("Content-Type", "application/fido.trusted-apps+json")
	return universalSecondFactor.AppID, nil
}

func (s *APIServer) deleteCertAuthority(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	id := services.CertAuthID{
		DomainName: p.ByName("domain"),
		Type:       services.CertAuthType(p.ByName("type")),
	}
	if err := auth.DeleteCertAuthority(id); err != nil {
		return nil, trace.Wrap(err)
	}
	return message(fmt.Sprintf("cert '%v' deleted", id)), nil
}

type CreateSessionReq struct {
	Session session.Session `json:"session"`
}

func (s *APIServer) createSession(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var req *CreateSessionReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	namespace := p.ByName("namespace")
	if !services.IsValidNamespace(namespace) {
		return nil, trace.BadParameter("invalid namespace %q", namespace)
	}
	req.Session.Namespace = namespace
	if err := auth.CreateSession(req.Session); err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

type UpdateSessionReq struct {
	Update session.UpdateRequest `json:"update"`
}

func (s *APIServer) updateSession(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var req *UpdateSessionReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	namespace := p.ByName("namespace")
	if !services.IsValidNamespace(namespace) {
		return nil, trace.BadParameter("invalid namespace %q", namespace)
	}
	req.Update.Namespace = namespace
	if err := auth.UpdateSession(req.Update); err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

func (s *APIServer) deleteSession(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	err := auth.DeleteSession(p.ByName("namespace"), session.ID(p.ByName("id")))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

func (s *APIServer) getSessions(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	namespace := p.ByName("namespace")
	if !services.IsValidNamespace(namespace) {
		return nil, trace.BadParameter("invalid namespace %q", namespace)
	}
	sessions, err := auth.GetSessions(namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return sessions, nil
}

func (s *APIServer) getSession(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	sid, err := session.ParseID(p.ByName("id"))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	namespace := p.ByName("namespace")
	if !services.IsValidNamespace(namespace) {
		return nil, trace.BadParameter("invalid namespace %q", namespace)
	}
	se, err := auth.GetSession(namespace, *sid)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return se, nil
}

func (s *APIServer) getSignupU2FRegisterRequest(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	token := p.ByName("token")
	u2fRegReq, err := auth.GetSignupU2FRegisterRequest(token)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return u2fRegReq, nil
}

type UpsertOIDCConnectorRawReq struct {
	Connector json.RawMessage `json:"connector"`
	TTL       time.Duration   `json:"ttl"`
}

func (s *APIServer) upsertOIDCConnector(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var req *UpsertOIDCConnectorRawReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	connector, err := resource.UnmarshalOIDCConnector(req.Connector)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if req.TTL != 0 {
		connector.SetExpiry(s.Now().UTC().Add(req.TTL))
	}
	if err = libauth.ValidateOIDCConnector(connector); err != nil {
		return nil, trace.Wrap(err)
	}
	err = auth.UpsertOIDCConnector(r.Context(), connector)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

func (s *APIServer) getOIDCConnector(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	withSecrets, _, err := httplib.ParseBool(r.URL.Query(), "with_secrets")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	connector, err := auth.GetOIDCConnector(r.Context(), p.ByName("id"), withSecrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return rawMessage(resource.MarshalOIDCConnector(connector, resource.WithVersion(version)))
}

func (s *APIServer) deleteOIDCConnector(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	err := auth.DeleteOIDCConnector(r.Context(), p.ByName("id"))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

func (s *APIServer) getOIDCConnectors(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	withSecrets, _, err := httplib.ParseBool(r.URL.Query(), "with_secrets")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	connectors, err := auth.GetOIDCConnectors(r.Context(), withSecrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	items := make([]json.RawMessage, len(connectors))
	for i, connector := range connectors {
		data, err := resource.MarshalOIDCConnector(connector, resource.WithVersion(version))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		items[i] = data
	}
	return items, nil
}

type CreateOIDCAuthRequestReq struct {
	Req libauth.OIDCAuthRequest `json:"req"`
}

func (s *APIServer) createOIDCAuthRequest(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var req *CreateOIDCAuthRequestReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	response, err := auth.CreateOIDCAuthRequest(req.Req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return response, nil
}

type ValidateOIDCAuthCallbackReq struct {
	Query url.Values `json:"query"`
}

// OIDCAuthRawResponse is returned when auth server validated callback parameters
// returned from OIDC provider
type OIDCAuthRawResponse struct {
	// Username is authenticated teleport username
	Username string `json:"username"`
	// Identity contains validated OIDC identity
	Identity services.ExternalIdentity `json:"identity"`
	// Web session will be generated by auth server if requested in OIDCAuthRequest
	Session json.RawMessage `json:"session,omitempty"`
	// Cert will be generated by certificate authority
	Cert []byte `json:"cert,omitempty"`
	// TLSCert is PEM encoded TLS certificate
	TLSCert []byte `json:"tls_cert,omitempty"`
	// Req is original oidc auth request
	Req libauth.OIDCAuthRequest `json:"req"`
	// HostSigners is a list of signing host public keys
	// trusted by proxy, used in console login
	HostSigners []json.RawMessage `json:"host_signers"`
}

func (s *APIServer) validateOIDCAuthCallback(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var req *ValidateOIDCAuthCallbackReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	response, err := auth.ValidateOIDCAuthCallback(req.Query)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	raw := OIDCAuthRawResponse{
		Username: response.Username,
		Identity: response.Identity,
		Cert:     response.Cert,
		TLSCert:  response.TLSCert,
		Req:      response.Req,
	}
	if response.Session != nil {
		rawSession, err := resource.MarshalWebSession(response.Session, resource.WithVersion(version))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		raw.Session = rawSession
	}
	raw.HostSigners = make([]json.RawMessage, len(response.HostSigners))
	for i, ca := range response.HostSigners {
		data, err := resource.MarshalCertAuthority(ca, resource.WithVersion(version))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		raw.HostSigners[i] = data
	}
	return &raw, nil
}

type CreateSAMLConnectorRawReq struct {
	Connector json.RawMessage `json:"connector"`
}

func (s *APIServer) createSAMLConnector(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var req *CreateSAMLConnectorRawReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	connector, err := resource.UnmarshalSAMLConnector(req.Connector)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := libauth.ValidateSAMLConnector(connector); err != nil {
		return nil, trace.Wrap(err)
	}
	err = auth.CreateSAMLConnector(r.Context(), connector)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

type UpsertSAMLConnectorRawReq struct {
	Connector json.RawMessage `json:"connector"`
}

func (s *APIServer) upsertSAMLConnector(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var req *UpsertSAMLConnectorRawReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	connector, err := resource.UnmarshalSAMLConnector(req.Connector)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := libauth.ValidateSAMLConnector(connector); err != nil {
		return nil, trace.Wrap(err)
	}
	err = auth.UpsertSAMLConnector(r.Context(), connector)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

func (s *APIServer) getSAMLConnector(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	withSecrets, _, err := httplib.ParseBool(r.URL.Query(), "with_secrets")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	connector, err := auth.GetSAMLConnector(r.Context(), p.ByName("id"), withSecrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return rawMessage(resource.MarshalSAMLConnector(connector, resource.WithVersion(version)))
}

func (s *APIServer) deleteSAMLConnector(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	err := auth.DeleteSAMLConnector(r.Context(), p.ByName("id"))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

func (s *APIServer) getSAMLConnectors(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	withSecrets, _, err := httplib.ParseBool(r.URL.Query(), "with_secrets")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	connectors, err := auth.GetSAMLConnectors(r.Context(), withSecrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	items := make([]json.RawMessage, len(connectors))
	for i, connector := range connectors {
		data, err := resource.MarshalSAMLConnector(connector, resource.WithVersion(version))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		items[i] = data
	}
	return items, nil
}

type CreateSAMLAuthRequestReq struct {
	Req libauth.SAMLAuthRequest `json:"req"`
}

func (s *APIServer) createSAMLAuthRequest(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var req *CreateSAMLAuthRequestReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	response, err := auth.CreateSAMLAuthRequest(req.Req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return response, nil
}

type ValidateSAMLResponseReq struct {
	Response string `json:"response"`
}

// SAMLAuthRawResponse is returned when auth server validated callback parameters
// returned from SAML provider
type SAMLAuthRawResponse struct {
	// Username is authenticated teleport username
	Username string `json:"username"`
	// Identity contains validated OIDC identity
	Identity services.ExternalIdentity `json:"identity"`
	// Web session will be generated by auth server if requested in OIDCAuthRequest
	Session json.RawMessage `json:"session,omitempty"`
	// Cert will be generated by certificate authority
	Cert []byte `json:"cert,omitempty"`
	// Req is original oidc auth request
	Req libauth.SAMLAuthRequest `json:"req"`
	// HostSigners is a list of signing host public keys
	// trusted by proxy, used in console login
	HostSigners []json.RawMessage `json:"host_signers"`
	// TLSCert is TLS certificate authority certificate
	TLSCert []byte `json:"tls_cert,omitempty"`
}

func (s *APIServer) validateSAMLResponse(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var req *ValidateSAMLResponseReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	response, err := auth.ValidateSAMLResponse(req.Response)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	raw := SAMLAuthRawResponse{
		Username: response.Username,
		Identity: response.Identity,
		Cert:     response.Cert,
		Req:      response.Req,
		TLSCert:  response.TLSCert,
	}
	if response.Session != nil {
		rawSession, err := resource.MarshalWebSession(response.Session, resource.WithVersion(version))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		raw.Session = rawSession
	}
	raw.HostSigners = make([]json.RawMessage, len(response.HostSigners))
	for i, ca := range response.HostSigners {
		data, err := resource.MarshalCertAuthority(ca, resource.WithVersion(version))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		raw.HostSigners[i] = data
	}
	return &raw, nil
}

// CreateGithubConnectorRawReq is a request to create a new Github connector
type CreateGithubConnectorRawReq struct {
	// Connector is the connector data
	Connector json.RawMessage `json:"connector"`
}

/* createGithubConnector creates a new Github connector

   POST /:version/github/connectors

   Success response: {"message": "ok"}
*/
func (s *APIServer) createGithubConnector(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var req CreateGithubConnectorRawReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	connector, err := resource.UnmarshalGithubConnector(req.Connector)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.CreateGithubConnector(connector); err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

// UpsertGithubConnectorRawReq is a request to upsert a Github connector
type UpsertGithubConnectorRawReq struct {
	// Connector is the connector data
	Connector json.RawMessage `json:"connector"`
}

/* upsertGithubConnector creates or updates a Github connector

   PUT /:version/github/connectors

   Success response: {"message": "ok"}
*/
func (s *APIServer) upsertGithubConnector(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var req UpsertGithubConnectorRawReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	connector, err := resource.UnmarshalGithubConnector(req.Connector)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.UpsertGithubConnector(r.Context(), connector); err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

/* getGithubConnectors returns a list of all configured Github connectors

   GET /:version/github/connectors

   Success response: []services.GithubConnector
*/
func (s *APIServer) getGithubConnectors(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	withSecrets, _, err := httplib.ParseBool(r.URL.Query(), "with_secrets")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	connectors, err := auth.GetGithubConnectors(r.Context(), withSecrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	items := make([]json.RawMessage, len(connectors))
	for i, connector := range connectors {
		cbytes, err := resource.MarshalGithubConnector(connector, resource.PreserveResourceID())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		items[i] = cbytes
	}
	return items, nil
}

/* getGithubConnector returns the specified Github connector

   GET /:version/github/connectors/:id

   Success response: services.GithubConnector
*/
func (s *APIServer) getGithubConnector(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	withSecrets, _, err := httplib.ParseBool(r.URL.Query(), "with_secrets")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	connector, err := auth.GetGithubConnector(r.Context(), p.ByName("id"), withSecrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return rawMessage(resource.MarshalGithubConnector(connector, resource.PreserveResourceID()))
}

/* deleteGithubConnector deletes the specified Github connector

   DELETE /:version/github/connectors/:id

   Success response: {"message": "ok"}
*/
func (s *APIServer) deleteGithubConnector(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	if err := auth.DeleteGithubConnector(r.Context(), p.ByName("id")); err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

// CreateGithubAuthRequestReq is a request to start Github OAuth2 flow
type CreateGithubAuthRequestReq struct {
	// Req is the request parameters
	Req libauth.GithubAuthRequest `json:"req"`
}

/* createGithubAuthRequest creates a new request for Github OAuth2 flow

   POST /:version/github/requests/create

   Success response: services.GithubAuthRequest
*/
func (s *APIServer) createGithubAuthRequest(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var req CreateGithubAuthRequestReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	response, err := auth.CreateGithubAuthRequest(req.Req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return response, nil
}

// ValidateGithubAuthCallbackReq is a request to validate Github OAuth2 callback
type ValidateGithubAuthCallbackReq struct {
	// Query is the callback query string
	Query url.Values `json:"query"`
}

// GithubAuthRawResponse is returned when auth server validated callback
// parameters returned from Github during OAuth2 flow
type GithubAuthRawResponse struct {
	// Username is authenticated teleport username
	Username string `json:"username"`
	// Identity contains validated OIDC identity
	Identity services.ExternalIdentity `json:"identity"`
	// Web session will be generated by auth server if requested in OIDCAuthRequest
	Session json.RawMessage `json:"session,omitempty"`
	// Cert will be generated by certificate authority
	Cert []byte `json:"cert,omitempty"`
	// TLSCert is PEM encoded TLS certificate
	TLSCert []byte `json:"tls_cert,omitempty"`
	// Req is original oidc auth request
	Req libauth.GithubAuthRequest `json:"req"`
	// HostSigners is a list of signing host public keys
	// trusted by proxy, used in console login
	HostSigners []json.RawMessage `json:"host_signers"`
}

/* validateGithubAuthRequest validates Github auth callback redirect

   POST /:version/github/requests/validate

   Success response: GithubAuthRawResponse
*/
func (s *APIServer) validateGithubAuthCallback(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var req ValidateGithubAuthCallbackReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	response, err := auth.ValidateGithubAuthCallback(req.Query)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	raw := GithubAuthRawResponse{
		Username: response.Username,
		Identity: response.Identity,
		Cert:     response.Cert,
		TLSCert:  response.TLSCert,
		Req:      response.Req,
	}
	if response.Session != nil {
		rawSession, err := resource.MarshalWebSession(
			response.Session, resource.WithVersion(version), resource.PreserveResourceID())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		raw.Session = rawSession
	}
	raw.HostSigners = make([]json.RawMessage, len(response.HostSigners))
	for i, ca := range response.HostSigners {
		data, err := resource.MarshalCertAuthority(
			ca, resource.WithVersion(version), resource.PreserveResourceID())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		raw.HostSigners[i] = data
	}
	return &raw, nil
}

// HTTP GET /:version/events?query
//
// Query fields:
//	'from'  : time filter in RFC3339 format
//	'to'    : time filter in RFC3339 format
//  ...     : other fields are passed directly to the audit backend
func (s *APIServer) searchEvents(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var err error
	to := time.Now().In(time.UTC)
	from := to.AddDate(0, -1, 0) // one month ago
	query := r.URL.Query()
	// parse 'to' and 'from' params:
	fromStr := query.Get("from")
	if fromStr != "" {
		from, err = time.Parse(time.RFC3339, fromStr)
		if err != nil {
			return nil, trace.BadParameter("from")
		}
	}
	toStr := query.Get("to")
	if toStr != "" {
		to, err = time.Parse(time.RFC3339, toStr)
		if err != nil {
			return nil, trace.BadParameter("to")
		}
	}
	var limit int
	limitStr := query.Get("limit")
	if limitStr != "" {
		limit, err = strconv.Atoi(limitStr)
		if err != nil {
			return nil, trace.BadParameter("failed to parse limit: %q", limit)
		}
	}
	// remove 'to', 'from' and 'limit' fields, passing the rest of the query unmodified
	// to whatever pluggable search is implemented by the backend
	query.Del("to")
	query.Del("from")
	query.Del("limit")
	eventsList, err := auth.SearchEvents(from, to, query.Encode(), limit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return eventsList, nil
}

// searchSessionEvents only allows searching audit log for events related to session playback.
func (s *APIServer) searchSessionEvents(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var err error

	// default values for "to" and "from" fields
	to := time.Now().In(time.UTC) // now
	from := to.AddDate(0, -1, 0)  // one month ago

	// parse query for "to" and "from"
	query := r.URL.Query()
	fromStr := query.Get("from")
	if fromStr != "" {
		from, err = time.Parse(time.RFC3339, fromStr)
		if err != nil {
			return nil, trace.BadParameter("from")
		}
	}
	toStr := query.Get("to")
	if toStr != "" {
		to, err = time.Parse(time.RFC3339, toStr)
		if err != nil {
			return nil, trace.BadParameter("to")
		}
	}
	var limit int
	limitStr := query.Get("limit")
	if limitStr != "" {
		limit, err = strconv.Atoi(limitStr)
		if err != nil {
			return nil, trace.BadParameter("failed to parse limit: %q", limit)
		}
	}
	// only pull back start and end events to build list of completed sessions
	eventsList, err := auth.SearchSessionEvents(from, to, limit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return eventsList, nil
}

type AuditEventReq struct {
	// Event is the event that's being emitted.
	Event events.Event `json:"event"`
	// Fields is the additional event fields.
	Fields events.EventFields `json:"fields"`
	// Type is the event type.
	//
	// This field is obsolete and kept for backwards compatibility.
	Type string `json:"type"`
}

// HTTP	POST /:version/events
func (s *APIServer) emitAuditEvent(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var req AuditEventReq
	err := httplib.ReadJSON(r, &req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Validate serverID field in event matches server ID from x509 identity. This
	// check makes sure nodes can only submit events for themselves.
	serverID, err := s.getServerID(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = events.ValidateEvent(req.Fields, serverID)
	if err != nil {
		log.Warnf("Rejecting audit event %v from %v: %v. System may be under attack, a "+
			"node is attempting to submit events for an identity other than its own.",
			req.Type, serverID, err)
		return nil, trace.AccessDenied("failed to validate event")
	}

	// DELETE IN: 4.1.0.
	//
	// For backwards compatibility, check if the full event struct has
	// been sent in the request or just the event type.
	if req.Event.Name != "" {
		err = auth.EmitAuditEventLegacy(req.Event, req.Fields)
	} else {
		err = auth.EmitAuditEventLegacy(events.Event{Name: req.Type}, req.Fields)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

// HTTP POST /:version/sessions/:id/slice
func (s *APIServer) postSessionSlice(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var slice events.SessionSlice
	if err := slice.Unmarshal(data); err != nil {
		return nil, trace.BadParameter("failed to unmarshal %v", err)
	}

	// Validate serverID field in event matches server ID from x509 identity. This
	// check makes sure nodes can only submit events for themselves.
	serverID, err := s.getServerID(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, v := range slice.GetChunks() {
		var f events.EventFields
		err = utils.FastUnmarshal(v.GetData(), &f)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		err := events.ValidateEvent(f, serverID)
		if err != nil {
			log.Warnf("Rejecting audit event %v from %v: %v. System may be under attack, a "+
				"node is attempting to submit events for an identity other than its own.",
				f.GetType(), serverID, err)
			return nil, trace.AccessDenied("failed to validate event")
		}
	}

	if err := auth.PostSessionSlice(slice); err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

// HTTP POST /:version/sessions/:id/recording
func (s *APIServer) uploadSessionRecording(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var files form.Files
	var namespace, sid string

	err := form.Parse(r,
		form.FileSlice("recording", &files),
		form.String("namespace", &namespace, form.Required()),
		form.String("sid", &sid, form.Required()),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	if !services.IsValidNamespace(namespace) {
		return nil, trace.BadParameter("invalid namespace %q", namespace)
	}
	if len(files) != 1 {
		return nil, trace.BadParameter("expected a single file parameter but got %d", len(files))
	}
	defer files[0].Close()
	_, err = session.ParseID(sid)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Make a copy of the archive because it needs to be read twice: once to
	// validate it and then again to upload it.
	var buf bytes.Buffer
	recording := io.TeeReader(files[0], &buf)

	// Validate namespace and serverID fields in the archive match namespace and
	// serverID of the authenticated client. This check makes sure nodes can
	// only submit recordings for themselves.
	serverID, err := s.getServerID(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = events.ValidateArchive(recording, serverID)
	if err != nil {
		log.Warnf("Rejecting session recording from %v: %v. System may be under attack, a "+
			"node is attempting to submit events for an identity other than its own.",
			serverID, err)
		return nil, trace.BadParameter("failed to validate archive")
	}

	if err = auth.UploadSessionRecording(events.SessionRecording{
		SessionID: session.ID(sid),
		Namespace: namespace,
		Recording: &buf,
	}); err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

// HTTP GET /:version/sessions/:id/stream?offset=x&bytes=y
// Query parameters:
//   "offset"   : bytes from the beginning
//   "bytes"    : number of bytes to read (it won't return more than 512Kb)
func (s *APIServer) getSessionChunk(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	sid, err := session.ParseID(p.ByName("id"))
	if err != nil {
		return nil, trace.BadParameter("missing parameter id")
	}
	namespace := p.ByName("namespace")
	if !services.IsValidNamespace(namespace) {
		return nil, trace.BadParameter("invalid namespace %q", namespace)
	}

	// "offset bytes" query param
	offsetBytes, err := strconv.Atoi(r.URL.Query().Get("offset"))
	if err != nil || offsetBytes < 0 {
		offsetBytes = 0
	}
	// "max bytes" query param
	max, err := strconv.Atoi(r.URL.Query().Get("bytes"))
	if err != nil || offsetBytes < 0 {
		offsetBytes = 0
	}
	log.Debugf("apiserver.GetSessionChunk(%v, %v, offset=%d)", namespace, *sid, offsetBytes)
	w.Header().Set("Content-Type", "text/plain")

	buffer, err := auth.GetSessionChunk(namespace, *sid, offsetBytes, max)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if _, err = w.Write(buffer); err != nil {
		return nil, trace.Wrap(err)
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	return nil, nil
}

// HTTP GET /:version/sessions/:id/events?maxage=n
// Query:
//    'after' : cursor value to return events newer than N. Defaults to 0, (return all)
func (s *APIServer) getSessionEvents(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	sid, err := session.ParseID(p.ByName("id"))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	namespace := p.ByName("namespace")
	if !services.IsValidNamespace(namespace) {
		return nil, trace.BadParameter("invalid namespace %q", namespace)
	}
	afterN, err := strconv.Atoi(r.URL.Query().Get("after"))
	if err != nil {
		afterN = 0
	}
	includePrintEvents, err := strconv.ParseBool(r.URL.Query().Get("print"))
	if err != nil {
		includePrintEvents = false
	}

	return auth.GetSessionEvents(namespace, *sid, afterN, includePrintEvents)
}

type UpsertNamespaceReq struct {
	Namespace services.Namespace `json:"namespace"`
}

func (s *APIServer) upsertNamespace(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var req *UpsertNamespaceReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.UpsertNamespace(req.Namespace); err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

func (s *APIServer) getNamespaces(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	namespaces, err := auth.GetNamespaces()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return namespaces, nil
}

func (s *APIServer) getNamespace(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	name := p.ByName("namespace")
	if !services.IsValidNamespace(name) {
		return nil, trace.BadParameter("invalid namespace %q", name)
	}

	namespace, err := auth.GetNamespace(name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return namespace, nil
}

func (s *APIServer) deleteNamespace(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	name := p.ByName("namespace")
	if !services.IsValidNamespace(name) {
		return nil, trace.BadParameter("invalid namespace %q", name)
	}

	err := auth.DeleteNamespace(name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

type UpsertRoleRawReq struct {
	Role json.RawMessage `json:"role"`
}

func (s *APIServer) upsertRole(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var req *UpsertRoleRawReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	role, err := resource.UnmarshalRole(req.Role)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err = libauth.ValidateRole(role); err != nil {
		return nil, trace.Wrap(err)
	}
	err = auth.UpsertRole(r.Context(), role)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return message(fmt.Sprintf("'%v' role upserted", role.GetName())), nil
}

func (s *APIServer) getRole(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	role, err := auth.GetRole(r.Context(), p.ByName("role"))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return rawMessage(resource.MarshalRole(role, resource.WithVersion(version), resource.PreserveResourceID()))
}

func (s *APIServer) getRoles(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	roles, err := auth.GetRoles(r.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out := make([]json.RawMessage, len(roles))
	for i, role := range roles {
		raw, err := resource.MarshalRole(role, resource.WithVersion(version), resource.PreserveResourceID())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out[i] = raw
	}
	return out, nil
}

func (s *APIServer) deleteRole(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	role := p.ByName("role")
	if err := auth.DeleteRole(r.Context(), role); err != nil {
		return nil, trace.Wrap(err)
	}
	return message(fmt.Sprintf("role %q deleted", role)), nil
}

func (s *APIServer) getClusterConfig(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	cc, err := auth.GetClusterConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return rawMessage(resource.MarshalClusterConfig(cc, resource.WithVersion(version), resource.PreserveResourceID()))
}

type SetClusterConfigReq struct {
	ClusterConfig json.RawMessage `json:"cluster_config"`
}

func (s *APIServer) setClusterConfig(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var req SetClusterConfigReq

	err := httplib.ReadJSON(r, &req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cc, err := resource.UnmarshalClusterConfig(req.ClusterConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = auth.SetClusterConfig(cc)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return message("cluster config set"), nil
}

func (s *APIServer) getClusterName(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	cn, err := auth.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return rawMessage(resource.MarshalClusterName(cn, resource.WithVersion(version), resource.PreserveResourceID()))
}

type SetClusterNameReq struct {
	ClusterName json.RawMessage `json:"cluster_name"`
}

func (s *APIServer) setClusterName(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var req SetClusterNameReq

	err := httplib.ReadJSON(r, &req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cn, err := resource.UnmarshalClusterName(req.ClusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = auth.SetClusterName(cn)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return message(fmt.Sprintf("cluster name set: %+v", cn)), nil
}

func (s *APIServer) getStaticTokens(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	st, err := auth.GetStaticTokens()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return rawMessage(resource.MarshalStaticTokens(st, resource.WithVersion(version), resource.PreserveResourceID()))
}

func (s *APIServer) deleteStaticTokens(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	err := auth.DeleteStaticTokens()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

type SetStaticTokensReq struct {
	StaticTokens json.RawMessage `json:"static_tokens"`
}

func (s *APIServer) setStaticTokens(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var req SetStaticTokensReq

	err := httplib.ReadJSON(r, &req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	st, err := resource.UnmarshalStaticTokens(req.StaticTokens)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = auth.SetStaticTokens(st)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return message(fmt.Sprintf("static tokens set: %+v", st)), nil
}

func (s *APIServer) getClusterAuthPreference(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	cap, err := auth.GetAuthPreference()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return rawMessage(resource.MarshalAuthPreference(cap, resource.WithVersion(version), resource.PreserveResourceID()))
}

type SetClusterAuthPreferenceReq struct {
	ClusterAuthPreference json.RawMessage `json:"cluster_auth_prerference"`
}

func (s *APIServer) setClusterAuthPreference(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var req *SetClusterAuthPreferenceReq

	err := httplib.ReadJSON(r, &req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cap, err := resource.UnmarshalAuthPreference(req.ClusterAuthPreference)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = auth.SetAuthPreference(cap)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return message(fmt.Sprintf("cluster authentication preference set: %+v", cap)), nil
}

type UpsertTunnelConnectionRawReq struct {
	TunnelConnection json.RawMessage `json:"tunnel_connection"`
}

// upsertTunnelConnection updates or inserts tunnel connection
func (s *APIServer) upsertTunnelConnection(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var req UpsertTunnelConnectionRawReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	conn, err := resource.UnmarshalTunnelConnection(req.TunnelConnection)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.UpsertTunnelConnection(conn); err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

// getTunnelConnections returns a list of tunnel connections from a cluster
func (s *APIServer) getTunnelConnections(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	conns, err := auth.GetTunnelConnections(p.ByName("cluster"))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	items := make([]json.RawMessage, len(conns))
	for i, conn := range conns {
		data, err := resource.MarshalTunnelConnection(conn, resource.WithVersion(version), resource.PreserveResourceID())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		items[i] = data
	}
	return items, nil
}

// getAllTunnelConnections returns a list of tunnel connections from a cluster
func (s *APIServer) getAllTunnelConnections(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	conns, err := auth.GetAllTunnelConnections()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	items := make([]json.RawMessage, len(conns))
	for i, conn := range conns {
		data, err := resource.MarshalTunnelConnection(conn, resource.WithVersion(version), resource.PreserveResourceID())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		items[i] = data
	}
	return items, nil
}

// deleteTunnelConnection deletes tunnel connection by name
func (s *APIServer) deleteTunnelConnection(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	err := auth.DeleteTunnelConnection(p.ByName("cluster"), p.ByName("conn"))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

// deleteTunnelConnections deletes all tunnel connections for cluster
func (s *APIServer) deleteTunnelConnections(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	err := auth.DeleteTunnelConnections(p.ByName("cluster"))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

// deleteAllTunnelConnections deletes all tunnel connections
func (s *APIServer) deleteAllTunnelConnections(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	err := auth.DeleteAllTunnelConnections()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

type CreateRemoteClusterRawReq struct {
	// RemoteCluster is marshalled remote cluster resource
	RemoteCluster json.RawMessage `json:"remote_cluster"`
}

// createRemoteCluster creates remote cluster
func (s *APIServer) createRemoteCluster(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var req CreateRemoteClusterRawReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	conn, err := resource.UnmarshalRemoteCluster(req.RemoteCluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.CreateRemoteCluster(conn); err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

// getRemoteClusters returns a list of remote clusters
func (s *APIServer) getRemoteClusters(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	clusters, err := auth.GetRemoteClusters()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	items := make([]json.RawMessage, len(clusters))
	for i, cluster := range clusters {
		data, err := resource.MarshalRemoteCluster(cluster, resource.WithVersion(version), resource.PreserveResourceID())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		items[i] = data
	}
	return items, nil
}

// getRemoteCluster returns a remote cluster by name
func (s *APIServer) getRemoteCluster(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	cluster, err := auth.GetRemoteCluster(p.ByName("cluster"))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return rawMessage(resource.MarshalRemoteCluster(cluster, resource.WithVersion(version), resource.PreserveResourceID()))
}

// deleteRemoteCluster deletes remote cluster by name
func (s *APIServer) deleteRemoteCluster(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	err := auth.DeleteRemoteCluster(p.ByName("cluster"))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

// deleteAllRemoteClusters deletes all remote clusters
func (s *APIServer) deleteAllRemoteClusters(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	err := auth.DeleteAllRemoteClusters()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return message("ok"), nil
}

func (s *APIServer) processKubeCSR(auth apiServerAuth, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var req KubeCSR

	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	re, err := auth.ProcessKubeCSR(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return re, nil
}

// getServerID returns the ID of the connected client.
func (s *APIServer) getServerID(r *http.Request) (string, error) {
	role, ok := r.Context().Value(ContextUser).(BuiltinRole)
	if !ok {
		return "", trace.BadParameter("invalid role %T", r.Context().Value(ContextUser))
	}

	clusterName, err := s.AuthServer.GetDomainName()
	if err != nil {
		return "", trace.Wrap(err)
	}

	// The username extracted from the node's identity (x.509 certificate)
	// is expected to consist of "<server-id>.<cluster-name>" so strip the
	// cluster name suffix to get the server id.
	//
	// Note that as of right now Teleport expects server id to be a uuid4
	// but older Gravity clusters used to override it with strings like
	// "192_168_1_1.<cluster-name>" so this code can't rely on it being
	// uuid4 to account for clusters upgraded from older versions.
	return strings.TrimSuffix(role.Username, "."+clusterName), nil
}

func message(msg string) map[string]interface{} {
	return map[string]interface{}{"message": msg}
}

type contextParamsKey string

// contextParams is the name of of the key that holds httprouter.Params in
// a context.
const contextParams contextParamsKey = "params"
