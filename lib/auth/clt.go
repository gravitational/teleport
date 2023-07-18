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

package auth

import (
	"context"
	"net"
	"net/url"
	"time"

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	assistpb "github.com/gravitational/teleport/api/gen/proto/go/assist/v1"
	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	loginrulepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"
	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	samlidppb "github.com/gravitational/teleport/api/gen/proto/go/teleport/samlidp/v1"
	"github.com/gravitational/teleport/api/internalutils/stream"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
)

const (
	// CurrentVersion is a current API version
	CurrentVersion = types.V2

	// MissingNamespaceError indicates that the client failed to
	// provide the namespace in the request.
	MissingNamespaceError = "missing required parameter: namespace"
)

// APIClient is aliased here so that it can be embedded in Client.
type APIClient = client.Client

// Client is the Auth API client. It works by connecting to auth servers
// via gRPC and HTTP.
//
// When Teleport servers connect to auth API, they usually establish an SSH
// tunnel first, and then do HTTP-over-SSH. This client is wrapped by auth.TunClient
// in lib/auth/tun.go
//
// NOTE: This client is being deprecated in favor of the gRPC Client in
// teleport/api/client. This Client should only be used internally, or for
// functionality that hasn't been ported to the new client yet.
type Client struct {
	// APIClient is used to make gRPC requests to the server
	*APIClient
	// HTTPClient is used to make http requests to the server
	*HTTPClient
}

// Make sure Client implements all the necessary methods.
var _ ClientI = &Client{}

// NewClient creates a new API client with a connection to a Teleport server.
//
// The client will use the first credentials and the given dialer. If
// no dialer is given, the first address will be used. This address must
// be an auth server address.
//
// NOTE: This client is being deprecated in favor of the gRPC Client in
// teleport/api/client. This Client should only be used internally, or for
// functionality that hasn't been ported to the new client yet.
func NewClient(cfg client.Config, params ...roundtrip.ClientParam) (*Client, error) {
	cfg.DialInBackground = true
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	apiClient, err := client.New(cfg.Context, cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// apiClient configures the tls.Config, so we clone it and reuse it for http.
	httpTLS := apiClient.Config().Clone()
	httpDialer := cfg.Dialer
	if httpDialer == nil {
		if len(cfg.Addrs) == 0 {
			return nil, trace.BadParameter("no addresses to dial")
		}
		httpDialer = client.ContextDialerFunc(func(ctx context.Context, network, _ string) (conn net.Conn, err error) {
			for _, addr := range cfg.Addrs {
				contextDialer := client.NewDialer(cfg.Context, cfg.KeepAlivePeriod, cfg.DialTimeout,
					client.WithInsecureSkipVerify(httpTLS.InsecureSkipVerify),
					client.WithALPNConnUpgrade(cfg.ALPNConnUpgradeRequired),
					client.WithPROXYHeaderGetter(cfg.PROXYHeaderGetter),
				)
				conn, err = contextDialer.DialContext(ctx, network, addr)
				if err == nil {
					return conn, nil
				}
			}
			// not wrapping on purpose to preserve the original error
			return nil, err
		})
	}
	httpClientCfg := &HTTPClientConfig{
		TLS:                        httpTLS,
		Dialer:                     httpDialer,
		ALPNSNIAuthDialClusterName: cfg.ALPNSNIAuthDialClusterName,
	}
	httpClient, err := NewHTTPClient(httpClientCfg, params...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Client{
		APIClient:  apiClient,
		HTTPClient: httpClient,
	}, nil
}

func (c *Client) Close() error {
	c.HTTPClient.Close()
	return c.APIClient.Close()
}

// CreateCertAuthority not implemented: can only be called locally.
func (c *Client) CreateCertAuthority(ctx context.Context, ca types.CertAuthority) error {
	return trace.NotImplemented(notImplementedMessage)
}

// UpsertCertAuthority updates or inserts new cert authority
func (c *Client) UpsertCertAuthority(ctx context.Context, ca types.CertAuthority) error {
	if err := services.ValidateCertAuthority(ca); err != nil {
		return trace.Wrap(err)
	}

	_, err := c.APIClient.UpsertCertAuthority(ctx, ca)
	return trace.Wrap(err)
}

// CompareAndSwapCertAuthority updates existing cert authority if the existing cert authority
// value matches the value stored in the backend.
func (c *Client) CompareAndSwapCertAuthority(new, existing types.CertAuthority) error {
	return trace.BadParameter("this function is not supported on the client")
}

// GetCertAuthorities returns a list of certificate authorities
func (c *Client) GetCertAuthorities(ctx context.Context, caType types.CertAuthType, loadKeys bool) ([]types.CertAuthority, error) {
	if err := caType.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	cas, err := c.APIClient.GetCertAuthorities(ctx, caType, loadKeys)
	return cas, trace.Wrap(err)
}

// GetCertAuthority returns certificate authority by given id. Parameter loadSigningKeys
// controls if signing keys are loaded
func (c *Client) GetCertAuthority(ctx context.Context, id types.CertAuthID, loadSigningKeys bool) (types.CertAuthority, error) {
	if err := id.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	ca, err := c.APIClient.GetCertAuthority(ctx, id, loadSigningKeys)
	return ca, trace.Wrap(err)
}

// DeleteCertAuthority deletes cert authority by ID
func (c *Client) DeleteCertAuthority(ctx context.Context, id types.CertAuthID) error {
	if err := id.Check(); err != nil {
		return trace.Wrap(err)
	}

	err := c.APIClient.DeleteCertAuthority(ctx, id)
	return trace.Wrap(err)
}

// ActivateCertAuthority not implemented: can only be called locally.
func (c *Client) ActivateCertAuthority(id types.CertAuthID) error {
	return trace.NotImplemented(notImplementedMessage)
}

// DeactivateCertAuthority not implemented: can only be called locally.
func (c *Client) DeactivateCertAuthority(id types.CertAuthID) error {
	return trace.NotImplemented(notImplementedMessage)
}

// UpdateUserCARoleMap not implemented: can only be called locally.
func (c *Client) UpdateUserCARoleMap(ctx context.Context, name string, roleMap types.RoleMap, activated bool) error {
	return trace.NotImplemented(notImplementedMessage)
}

// KeepAliveServer not implemented: can only be called locally.
func (c *Client) KeepAliveServer(ctx context.Context, keepAlive types.KeepAlive) error {
	return trace.BadParameter("not implemented, use StreamKeepAlives instead")
}

// GetReverseTunnel not implemented: can only be called locally.
func (c *Client) GetReverseTunnel(name string, opts ...services.MarshalOption) (types.ReverseTunnel, error) {
	return nil, trace.NotImplemented(notImplementedMessage)
}

// DeleteAllTokens not implemented: can only be called locally.
func (c *Client) DeleteAllTokens() error {
	return trace.NotImplemented(notImplementedMessage)
}

// AddUserLoginAttempt logs user login attempt
func (c *Client) AddUserLoginAttempt(user string, attempt services.LoginAttempt, ttl time.Duration) error {
	panic("not implemented")
}

// GetUserLoginAttempts returns user login attempts
func (c *Client) GetUserLoginAttempts(user string) ([]services.LoginAttempt, error) {
	panic("not implemented")
}

// DeleteAllAuthServers not implemented: can only be called locally.
func (c *Client) DeleteAllAuthServers() error {
	return trace.NotImplemented(notImplementedMessage)
}

// DeleteAuthServer not implemented: can only be called locally.
func (c *Client) DeleteAuthServer(name string) error {
	return trace.NotImplemented(notImplementedMessage)
}

// CompareAndSwapUser not implemented: can only be called locally
func (c *Client) CompareAndSwapUser(ctx context.Context, new, expected types.User) error {
	return trace.NotImplemented(notImplementedMessage)
}

// GetNodeStream not implemented: can only be called locally
func (c *Client) GetNodeStream(_ context.Context, _ string) stream.Stream[types.Server] {
	return stream.Fail[types.Server](trace.NotImplemented(notImplementedMessage))
}

// StreamSessionEvents streams all events from a given session recording. An error is returned on the first
// channel if one is encountered. Otherwise the event channel is closed when the stream ends.
// The event channel is not closed on error to prevent race conditions in downstream select statements.
func (c *Client) StreamSessionEvents(ctx context.Context, sessionID session.ID, startIndex int64) (chan apievents.AuditEvent, chan error) {
	return c.APIClient.StreamSessionEvents(ctx, string(sessionID), startIndex)
}

// SearchEvents allows searching for audit events with pagination support.
func (c *Client) SearchEvents(ctx context.Context, req events.SearchEventsRequest) ([]apievents.AuditEvent, string, error) {
	events, lastKey, err := c.APIClient.SearchEvents(ctx, req.From, req.To, apidefaults.Namespace, req.EventTypes, req.Limit, req.Order, req.StartKey)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	return events, lastKey, nil
}

// SearchSessionEvents returns session related events to find completed sessions.
func (c *Client) SearchSessionEvents(ctx context.Context, req events.SearchSessionEventsRequest) ([]apievents.AuditEvent, string, error) {
	events, lastKey, err := c.APIClient.SearchSessionEvents(ctx, req.From, req.To, req.Limit, req.Order, req.StartKey)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	return events, lastKey, nil
}

// CreateRole not implemented: can only be called locally.
func (c *Client) CreateRole(ctx context.Context, role types.Role) error {
	return trace.NotImplemented(notImplementedMessage)
}

// UpsertClusterName not implemented: can only be called locally.
func (c *Client) UpsertClusterName(cn types.ClusterName) error {
	return trace.NotImplemented(notImplementedMessage)
}

// DeleteClusterName not implemented: can only be called locally.
func (c *Client) DeleteClusterName() error {
	return trace.NotImplemented(notImplementedMessage)
}

// DeleteAllCertAuthorities not implemented: can only be called locally.
func (c *Client) DeleteAllCertAuthorities(caType types.CertAuthType) error {
	return trace.NotImplemented(notImplementedMessage)
}

// DeleteAllReverseTunnels not implemented: can only be called locally.
func (c *Client) DeleteAllReverseTunnels() error {
	return trace.NotImplemented(notImplementedMessage)
}

// DeleteAllCertNamespaces not implemented: can only be called locally.
func (c *Client) DeleteAllNamespaces() error {
	return trace.NotImplemented(notImplementedMessage)
}

// DeleteAllRoles not implemented: can only be called locally.
func (c *Client) DeleteAllRoles() error {
	return trace.NotImplemented(notImplementedMessage)
}

// ListWindowsDesktops not implemented: can only be called locally.
func (c *Client) ListWindowsDesktops(ctx context.Context, req types.ListWindowsDesktopsRequest) (*types.ListWindowsDesktopsResponse, error) {
	return nil, trace.NotImplemented(notImplementedMessage)
}

// ListWindowsDesktopServices not implemented: can only be called locally.
func (c *Client) ListWindowsDesktopServices(ctx context.Context, req types.ListWindowsDesktopServicesRequest) (*types.ListWindowsDesktopServicesResponse, error) {
	return nil, trace.NotImplemented(notImplementedMessage)
}

// DeleteAllUsers not implemented: can only be called locally.
func (c *Client) DeleteAllUsers() error {
	return trace.NotImplemented(notImplementedMessage)
}

// CreateResetPasswordToken creates reset password token
func (c *Client) CreateResetPasswordToken(ctx context.Context, req CreateUserTokenRequest) (types.UserToken, error) {
	return c.APIClient.CreateResetPasswordToken(ctx, &proto.CreateResetPasswordTokenRequest{
		Name: req.Name,
		TTL:  proto.Duration(req.TTL),
		Type: req.Type,
	})
}

// GetDatabaseServers returns all registered database proxy servers.
func (c *Client) GetDatabaseServers(ctx context.Context, namespace string, opts ...services.MarshalOption) ([]types.DatabaseServer, error) {
	return c.APIClient.GetDatabaseServers(ctx, namespace)
}

// UpsertAppSession not implemented: can only be called locally.
func (c *Client) UpsertAppSession(ctx context.Context, session types.WebSession) error {
	return trace.NotImplemented(notImplementedMessage)
}

// UpsertSnowflakeSession not implemented: can only be called locally.
func (c *Client) UpsertSnowflakeSession(_ context.Context, _ types.WebSession) error {
	return trace.NotImplemented(notImplementedMessage)
}

// UpsertSAMLIdPSession not implemented: can only be called locally.
func (c *Client) UpsertSAMLIdPSession(_ context.Context, _ types.WebSession) error {
	return trace.NotImplemented(notImplementedMessage)
}

// ResumeAuditStream resumes existing audit stream.
func (c *Client) ResumeAuditStream(ctx context.Context, sid session.ID, uploadID string) (apievents.Stream, error) {
	return c.APIClient.ResumeAuditStream(ctx, string(sid), uploadID)
}

// CreateAuditStream creates new audit stream.
func (c *Client) CreateAuditStream(ctx context.Context, sid session.ID) (apievents.Stream, error) {
	return c.APIClient.CreateAuditStream(ctx, string(sid))
}

// GetClusterAuditConfig gets cluster audit configuration.
func (c *Client) GetClusterAuditConfig(ctx context.Context, opts ...services.MarshalOption) (types.ClusterAuditConfig, error) {
	return c.APIClient.GetClusterAuditConfig(ctx)
}

// GetClusterNetworkingConfig gets cluster networking configuration.
func (c *Client) GetClusterNetworkingConfig(ctx context.Context, opts ...services.MarshalOption) (types.ClusterNetworkingConfig, error) {
	return c.APIClient.GetClusterNetworkingConfig(ctx)
}

// GetSessionRecordingConfig gets session recording configuration.
func (c *Client) GetSessionRecordingConfig(ctx context.Context, opts ...services.MarshalOption) (types.SessionRecordingConfig, error) {
	return c.APIClient.GetSessionRecordingConfig(ctx)
}

// GenerateCertAuthorityCRL generates an empty CRL for a CA.
func (c *Client) GenerateCertAuthorityCRL(ctx context.Context, caType types.CertAuthType) ([]byte, error) {
	resp, err := c.APIClient.GenerateCertAuthorityCRL(ctx, &proto.CertAuthorityRequest{Type: caType})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp.CRL, nil
}

// DeleteClusterNetworkingConfig not implemented: can only be called locally.
func (c *Client) DeleteClusterNetworkingConfig(ctx context.Context) error {
	return trace.NotImplemented(notImplementedMessage)
}

// DeleteSessionRecordingConfig not implemented: can only be called locally.
func (c *Client) DeleteSessionRecordingConfig(ctx context.Context) error {
	return trace.NotImplemented(notImplementedMessage)
}

// DeleteAuthPreference not implemented: can only be called locally.
func (c *Client) DeleteAuthPreference(context.Context) error {
	return trace.NotImplemented(notImplementedMessage)
}

// SetClusterAuditConfig not implemented: can only be called locally.
func (c *Client) SetClusterAuditConfig(ctx context.Context, auditConfig types.ClusterAuditConfig) error {
	return trace.NotImplemented(notImplementedMessage)
}

// DeleteClusterAuditConfig not implemented: can only be called locally.
func (c *Client) DeleteClusterAuditConfig(ctx context.Context) error {
	return trace.NotImplemented(notImplementedMessage)
}

// DeleteAllLocks not implemented: can only be called locally.
func (c *Client) DeleteAllLocks(context.Context) error {
	return trace.NotImplemented(notImplementedMessage)
}

func (c *Client) UpdatePresence(ctx context.Context, sessionID, user string) error {
	return trace.NotImplemented(notImplementedMessage)
}

func (c *Client) StreamNodes(ctx context.Context, namespace string) stream.Stream[types.Server] {
	return stream.Fail[types.Server](trace.NotImplemented(notImplementedMessage))
}

func (c *Client) GetLicense(ctx context.Context) (string, error) {
	return c.APIClient.GetLicense(ctx)
}

func (c *Client) ListReleases(ctx context.Context) ([]*types.Release, error) {
	return c.APIClient.ListReleases(ctx, &proto.ListReleasesRequest{})
}

func (c *Client) OktaClient() services.Okta {
	return c.APIClient.OktaClient()
}

func (c *Client) AccessListClient() accesslistv1.AccessListServiceClient {
	return c.APIClient.AccessListClient()
}

// WebService implements features used by Web UI clients
type WebService interface {
	// GetWebSessionInfo checks if a web session is valid, returns session id in case if
	// it is valid, or error otherwise.
	GetWebSessionInfo(ctx context.Context, user, sessionID string) (types.WebSession, error)
	// ExtendWebSession creates a new web session for a user based on another
	// valid web session
	ExtendWebSession(ctx context.Context, req WebSessionReq) (types.WebSession, error)
	// CreateWebSession creates a new web session for a user
	CreateWebSession(ctx context.Context, user string) (types.WebSession, error)

	// AppSession defines application session features.
	services.AppSession
	// SnowflakeSession defines Snowflake session features.
	services.SnowflakeSession
}

// IdentityService manages identities and users
type IdentityService interface {
	// UpsertOIDCConnector updates or creates OIDC connector
	UpsertOIDCConnector(ctx context.Context, connector types.OIDCConnector) error
	// GetOIDCConnector returns OIDC connector information by id
	GetOIDCConnector(ctx context.Context, id string, withSecrets bool) (types.OIDCConnector, error)
	// GetOIDCConnectors gets OIDC connectors list
	GetOIDCConnectors(ctx context.Context, withSecrets bool) ([]types.OIDCConnector, error)
	// DeleteOIDCConnector deletes OIDC connector by ID
	DeleteOIDCConnector(ctx context.Context, connectorID string) error
	// CreateOIDCAuthRequest creates OIDCAuthRequest
	CreateOIDCAuthRequest(ctx context.Context, req types.OIDCAuthRequest) (*types.OIDCAuthRequest, error)
	// GetOIDCAuthRequest returns OIDC auth request if found
	GetOIDCAuthRequest(ctx context.Context, id string) (*types.OIDCAuthRequest, error)
	// ValidateOIDCAuthCallback validates OIDC auth callback returned from redirect
	ValidateOIDCAuthCallback(ctx context.Context, q url.Values) (*OIDCAuthResponse, error)

	// UpsertSAMLConnector updates or creates SAML connector
	UpsertSAMLConnector(ctx context.Context, connector types.SAMLConnector) error
	// GetSAMLConnector returns SAML connector information by id
	GetSAMLConnector(ctx context.Context, id string, withSecrets bool) (types.SAMLConnector, error)
	// GetSAMLConnectors gets SAML connectors list
	GetSAMLConnectors(ctx context.Context, withSecrets bool) ([]types.SAMLConnector, error)
	// DeleteSAMLConnector deletes SAML connector by ID
	DeleteSAMLConnector(ctx context.Context, connectorID string) error
	// CreateSAMLAuthRequest creates SAML AuthnRequest
	CreateSAMLAuthRequest(ctx context.Context, req types.SAMLAuthRequest) (*types.SAMLAuthRequest, error)
	// ValidateSAMLResponse validates SAML auth response
	ValidateSAMLResponse(ctx context.Context, re string, connectorID string) (*SAMLAuthResponse, error)
	// GetSAMLAuthRequest returns SAML auth request if found
	GetSAMLAuthRequest(ctx context.Context, authRequestID string) (*types.SAMLAuthRequest, error)

	// UpsertGithubConnector creates or updates a Github connector
	UpsertGithubConnector(ctx context.Context, connector types.GithubConnector) error
	// GetGithubConnectors returns all configured Github connectors
	GetGithubConnectors(ctx context.Context, withSecrets bool) ([]types.GithubConnector, error)
	// GetGithubConnector returns the specified Github connector
	GetGithubConnector(ctx context.Context, id string, withSecrets bool) (types.GithubConnector, error)
	// DeleteGithubConnector deletes the specified Github connector
	DeleteGithubConnector(ctx context.Context, id string) error
	// CreateGithubAuthRequest creates a new request for Github OAuth2 flow
	CreateGithubAuthRequest(ctx context.Context, req types.GithubAuthRequest) (*types.GithubAuthRequest, error)
	// GetGithubAuthRequest returns Github auth request if found
	GetGithubAuthRequest(ctx context.Context, id string) (*types.GithubAuthRequest, error)
	// ValidateGithubAuthCallback validates Github auth callback
	ValidateGithubAuthCallback(ctx context.Context, q url.Values) (*GithubAuthResponse, error)

	// GetSSODiagnosticInfo returns SSO diagnostic info records.
	GetSSODiagnosticInfo(ctx context.Context, authKind string, authRequestID string) (*types.SSODiagnosticInfo, error)

	// GetUser returns user by name
	GetUser(name string, withSecrets bool) (types.User, error)

	// GetCurrentUser returns current user as seen by the server.
	// Useful especially in the context of remote clusters which perform role and trait mapping.
	GetCurrentUser(ctx context.Context) (types.User, error)

	// GetCurrentUserRoles returns current user's roles.
	GetCurrentUserRoles(ctx context.Context) ([]types.Role, error)

	// CreateUser inserts a new entry in a backend.
	CreateUser(ctx context.Context, user types.User) error

	// UpdateUser updates an existing user in a backend.
	UpdateUser(ctx context.Context, user types.User) error

	// UpsertUser user updates or inserts user entry
	UpsertUser(user types.User) error

	// CompareAndSwapUser updates an existing user in a backend, but fails if
	// the user in the backend does not match the expected value.
	CompareAndSwapUser(ctx context.Context, new, expected types.User) error

	// DeleteUser deletes an existng user in a backend by username.
	DeleteUser(ctx context.Context, user string) error

	// GetUsers returns a list of usernames registered in the system
	GetUsers(withSecrets bool) ([]types.User, error)

	// ChangePassword changes user password
	ChangePassword(ctx context.Context, req *proto.ChangePasswordRequest) error

	// GenerateHostCert takes the public key in the Open SSH ``authorized_keys``
	// plain text format, signs it using Host Certificate Authority private key and returns the
	// resulting certificate.
	GenerateHostCert(ctx context.Context, key []byte, hostID, nodeName string, principals []string, clusterName string, role types.SystemRole, ttl time.Duration) ([]byte, error)

	// GenerateUserCerts takes the public key in the OpenSSH `authorized_keys` plain
	// text format, signs it using User Certificate Authority signing key and
	// returns the resulting certificates.
	GenerateUserCerts(ctx context.Context, req proto.UserCertsRequest) (*proto.Certs, error)

	// GenerateUserSingleUseCerts is like GenerateUserCerts but issues a
	// certificate for a single session
	// (https://github.com/gravitational/teleport/blob/3a1cf9111c2698aede2056513337f32bfc16f1f1/rfd/0014-session-2FA.md#sessions).
	GenerateUserSingleUseCerts(ctx context.Context) (proto.AuthService_GenerateUserSingleUseCertsClient, error)

	// IsMFARequired is a request to check whether MFA is required to
	// access the Target.
	IsMFARequired(ctx context.Context, req *proto.IsMFARequiredRequest) (*proto.IsMFARequiredResponse, error)

	// DeleteAllUsers deletes all users
	DeleteAllUsers() error

	// CreateResetPasswordToken creates a new user reset token
	CreateResetPasswordToken(ctx context.Context, req CreateUserTokenRequest) (types.UserToken, error)

	// CreateBot creates a new certificate renewal bot and associated resources.
	CreateBot(ctx context.Context, req *proto.CreateBotRequest) (*proto.CreateBotResponse, error)
	// DeleteBot removes a certificate renewal bot and associated resources.
	DeleteBot(ctx context.Context, botName string) error
	// GetBotUsers gets all bot users.
	GetBotUsers(ctx context.Context) ([]types.User, error)

	// ChangeUserAuthentication allows a user with a reset or invite token to change their password and if enabled also adds a new mfa device.
	// Upon success, creates new web session and creates new set of recovery codes (if user meets requirements).
	ChangeUserAuthentication(ctx context.Context, req *proto.ChangeUserAuthenticationRequest) (*proto.ChangeUserAuthenticationResponse, error)

	// GetResetPasswordToken returns a reset password token.
	GetResetPasswordToken(ctx context.Context, username string) (types.UserToken, error)

	// GetMFADevices fetches all MFA devices registered for the calling user.
	GetMFADevices(ctx context.Context, in *proto.GetMFADevicesRequest) (*proto.GetMFADevicesResponse, error)
	// AddMFADevice adds a new MFA device for the calling user.
	AddMFADevice(ctx context.Context) (proto.AuthService_AddMFADeviceClient, error)
	// DeleteMFADevice deletes a MFA device for the calling user.
	DeleteMFADevice(ctx context.Context) (proto.AuthService_DeleteMFADeviceClient, error)
	// AddMFADeviceSync adds a new MFA device (nonstream).
	AddMFADeviceSync(ctx context.Context, req *proto.AddMFADeviceSyncRequest) (*proto.AddMFADeviceSyncResponse, error)
	// DeleteMFADeviceSync deletes a users MFA device (nonstream).
	DeleteMFADeviceSync(ctx context.Context, req *proto.DeleteMFADeviceSyncRequest) error
	// CreateAuthenticateChallenge creates and returns MFA challenges for a users registered MFA devices.
	CreateAuthenticateChallenge(ctx context.Context, req *proto.CreateAuthenticateChallengeRequest) (*proto.MFAAuthenticateChallenge, error)
	// CreateRegisterChallenge creates and returns MFA register challenge for a new MFA device.
	CreateRegisterChallenge(ctx context.Context, req *proto.CreateRegisterChallengeRequest) (*proto.MFARegisterChallenge, error)

	// MaintainSessionPresence establishes a channel used to continuously verify the presence for a session.
	MaintainSessionPresence(ctx context.Context) (proto.AuthService_MaintainSessionPresenceClient, error)

	// StartAccountRecovery creates a recovery start token for a user who successfully verified their username and their recovery code.
	// This token is used as part of a URL that will be emailed to the user (not done in this request).
	// Represents step 1 of the account recovery process.
	StartAccountRecovery(ctx context.Context, req *proto.StartAccountRecoveryRequest) (types.UserToken, error)
	// VerifyAccountRecovery creates a recovery approved token after successful verification of users password or second factor
	// (authn depending on what user needed to recover). This token will allow users to perform protected actions while not logged in.
	// Represents step 2 of the account recovery process after RPC StartAccountRecovery.
	VerifyAccountRecovery(ctx context.Context, req *proto.VerifyAccountRecoveryRequest) (types.UserToken, error)
	// CompleteAccountRecovery sets a new password or adds a new mfa device,
	// allowing user to regain access to their account using the new credentials.
	// Represents the last step in the account recovery process after RPC's StartAccountRecovery and VerifyAccountRecovery.
	CompleteAccountRecovery(ctx context.Context, req *proto.CompleteAccountRecoveryRequest) error

	// CreateAccountRecoveryCodes creates new set of recovery codes for a user, replacing and invalidating any previously owned codes.
	CreateAccountRecoveryCodes(ctx context.Context, req *proto.CreateAccountRecoveryCodesRequest) (*proto.RecoveryCodes, error)
	// GetAccountRecoveryToken returns a user token resource after verifying the token in
	// request is not expired and is of the correct recovery type.
	GetAccountRecoveryToken(ctx context.Context, req *proto.GetAccountRecoveryTokenRequest) (types.UserToken, error)
	// GetAccountRecoveryCodes returns the user in context their recovery codes resource without any secrets.
	GetAccountRecoveryCodes(ctx context.Context, req *proto.GetAccountRecoveryCodesRequest) (*proto.RecoveryCodes, error)

	// CreatePrivilegeToken creates a privilege token for the logged in user who has successfully re-authenticated with their second factor.
	// A privilege token allows users to perform privileged action eg: add/delete their MFA device.
	CreatePrivilegeToken(ctx context.Context, req *proto.CreatePrivilegeTokenRequest) (*types.UserTokenV3, error)

	// UpdateHeadlessAuthenticationState updates a headless authentication state.
	UpdateHeadlessAuthenticationState(ctx context.Context, id string, state types.HeadlessAuthenticationState, mfaResponse *proto.MFAAuthenticateResponse) error
	// GetHeadlessAuthentication retrieves a headless authentication by id.
	GetHeadlessAuthentication(ctx context.Context, id string) (*types.HeadlessAuthentication, error)
	// WatchPendingHeadlessAuthentications creates a watcher for pending headless authentication for the current user.
	WatchPendingHeadlessAuthentications(ctx context.Context) (types.Watcher, error)
}

// ProvisioningService is a service in control
// of adding new nodes, auth servers and proxies to the cluster
type ProvisioningService interface {
	// GetTokens returns a list of active invitation tokens for nodes and users
	GetTokens(ctx context.Context) (tokens []types.ProvisionToken, err error)

	// GetToken returns provisioning token
	GetToken(ctx context.Context, token string) (types.ProvisionToken, error)

	// DeleteToken deletes a given provisioning token on the auth server (CA). It
	// could be a reset password token or a machine token
	DeleteToken(ctx context.Context, token string) error

	// DeleteAllTokens deletes all provisioning tokens
	DeleteAllTokens() error

	// UpsertToken adds provisioning tokens for the auth server
	UpsertToken(ctx context.Context, token types.ProvisionToken) error

	// CreateToken creates a new provision token for the auth server
	CreateToken(ctx context.Context, token types.ProvisionToken) error

	// RegisterUsingToken calls the auth service API to register a new node via registration token
	// which has been previously issued via GenerateToken
	RegisterUsingToken(ctx context.Context, req *types.RegisterUsingTokenRequest) (*proto.Certs, error)
}

// ClientI is a client to Auth service
type ClientI interface {
	IdentityService
	ProvisioningService
	services.Trust
	events.AuditLogSessionStreamer
	events.Streamer
	apievents.Emitter
	services.Presence
	services.Access
	services.DynamicAccess
	services.DynamicAccessOracle
	services.Restrictions
	services.Apps
	services.Databases
	services.DatabaseServices
	services.Kubernetes
	services.WindowsDesktops
	services.SAMLIdPServiceProviders
	services.UserGroups
	services.Assistant
	services.UserPreferences
	WebService
	services.Status
	services.ClusterConfiguration
	services.SessionTrackerService
	services.ConnectionsDiagnostic
	services.SAMLIdPSession
	services.Integrations
	types.Events

	types.WebSessionsGetter
	types.WebTokensGetter

	// DevicesClient returns a Device Trust client.
	// Clients connecting to non-Enterprise clusters, or older Teleport versions,
	// still get a client when calling this method, but all RPCs will return
	// "not implemented" errors (as per the default gRPC behavior).
	DevicesClient() devicepb.DeviceTrustServiceClient

	// LoginRuleClient returns a client to the Login Rule gRPC service.
	// Clients connecting to non-Enterprise clusters, or older Teleport versions,
	// still get a client when calling this method, but all RPCs will return
	// "not implemented" errors (as per the default gRPC behavior).
	LoginRuleClient() loginrulepb.LoginRuleServiceClient

	// EmbeddingClient returns a client to the Embedding gRPC service.
	EmbeddingClient() assistpb.AssistEmbeddingServiceClient

	// NewKeepAliver returns a new instance of keep aliver
	NewKeepAliver(ctx context.Context) (types.KeepAliver, error)

	// RotateCertAuthority starts or restarts certificate authority rotation process.
	RotateCertAuthority(ctx context.Context, req RotateRequest) error

	// RotateExternalCertAuthority rotates external certificate authority,
	// this method is used to update only public keys and certificates of the
	// the certificate authorities of trusted clusters.
	RotateExternalCertAuthority(ctx context.Context, ca types.CertAuthority) error

	// ValidateTrustedCluster validates trusted cluster token with
	// main cluster, in case if validation is successful, main cluster
	// adds remote cluster
	ValidateTrustedCluster(context.Context, *ValidateTrustedClusterRequest) (*ValidateTrustedClusterResponse, error)

	// GetDomainName returns auth server cluster name
	GetDomainName(ctx context.Context) (string, error)

	// GetClusterCACert returns the PEM-encoded TLS certs for the local cluster.
	// If the cluster has multiple TLS certs, they will all be concatenated.
	GetClusterCACert(ctx context.Context) (*proto.GetClusterCACertResponse, error)

	// GenerateHostCerts generates new host certificates (signed
	// by the host certificate authority) for a node
	GenerateHostCerts(context.Context, *proto.HostCertsRequest) (*proto.Certs, error)
	// GenerateOpenSSHCert signs a SSH certificate with OpenSSH CA that
	// can be used to connect to Agentless nodes.
	GenerateOpenSSHCert(ctx context.Context, req *proto.OpenSSHCertRequest) (*proto.OpenSSHCert, error)
	// AuthenticateWebUser authenticates web user, creates and  returns web session
	// in case if authentication is successful
	AuthenticateWebUser(ctx context.Context, req AuthenticateUserRequest) (types.WebSession, error)
	// AuthenticateSSHUser authenticates SSH console user, creates and  returns a pair of signed TLS and SSH
	// short-lived certificates as a result
	AuthenticateSSHUser(ctx context.Context, req AuthenticateSSHRequest) (*SSHLoginResponse, error)

	// ProcessKubeCSR processes CSR request against Kubernetes CA, returns
	// signed certificate if successful.
	ProcessKubeCSR(req KubeCSR) (*KubeCSRResponse, error)

	// Ping gets basic info about the auth server.
	Ping(ctx context.Context) (proto.PingResponse, error)

	// CreateAppSession creates an application web session. Application web
	// sessions represent a browser session the client holds.
	CreateAppSession(context.Context, types.CreateAppSessionRequest) (types.WebSession, error)

	// CreateSnowflakeSession creates a Snowflake web session. Snowflake web
	// sessions represent Database Access Snowflake session the client holds.
	CreateSnowflakeSession(context.Context, types.CreateSnowflakeSessionRequest) (types.WebSession, error)

	// CreateSAMLIdPSession creates a SAML IdP. SAML IdP sessions represent
	// sessions created by the SAML identity provider.
	CreateSAMLIdPSession(context.Context, types.CreateSAMLIdPSessionRequest) (types.WebSession, error)

	// GenerateDatabaseCert generates client certificate used by a database
	// service to authenticate with the database instance.
	GenerateDatabaseCert(context.Context, *proto.DatabaseCertRequest) (*proto.DatabaseCertResponse, error)

	// GetWebSession queries the existing web session described with req.
	// Implements ReadAccessPoint.
	GetWebSession(ctx context.Context, req types.GetWebSessionRequest) (types.WebSession, error)

	// GetWebToken queries the existing web token described with req.
	// Implements ReadAccessPoint.
	GetWebToken(ctx context.Context, req types.GetWebTokenRequest) (types.WebToken, error)

	// GenerateAWSOIDCToken generates a token to be used to execute an AWS OIDC Integration action.
	GenerateAWSOIDCToken(ctx context.Context, req types.GenerateAWSOIDCTokenRequest) (string, error)

	// ResetAuthPreference resets cluster auth preference to defaults.
	ResetAuthPreference(ctx context.Context) error

	// ResetClusterNetworkingConfig resets cluster networking configuration to defaults.
	ResetClusterNetworkingConfig(ctx context.Context) error

	// ResetSessionRecordingConfig resets session recording configuration to defaults.
	ResetSessionRecordingConfig(ctx context.Context) error

	// GenerateWindowsDesktopCert generates client smartcard certificate used
	// by an RDP client to authenticate with Windows.
	GenerateWindowsDesktopCert(context.Context, *proto.WindowsDesktopCertRequest) (*proto.WindowsDesktopCertResponse, error)
	// GenerateCertAuthorityCRL generates an empty CRL for a CA.
	GenerateCertAuthorityCRL(context.Context, types.CertAuthType) ([]byte, error)

	// GetInventoryStatus gets basic status info about instance inventory.
	GetInventoryStatus(ctx context.Context, req proto.InventoryStatusRequest) (proto.InventoryStatusSummary, error)

	// PingInventory attempts to trigger a downstream ping against a connected instance.
	PingInventory(ctx context.Context, req proto.InventoryPingRequest) (proto.InventoryPingResponse, error)

	// SubmitUsageEvent submits an external usage event.
	SubmitUsageEvent(ctx context.Context, req *proto.SubmitUsageEventRequest) error

	// GetLicense returns the license used to start Teleport Enterprise
	GetLicense(ctx context.Context) (string, error)

	// ListReleases returns a list of Teleport Enterprise releases
	ListReleases(ctx context.Context) ([]*types.Release, error)

	// PluginsClient returns a Plugins client.
	// Clients connecting to non-Enterprise clusters, or older Teleport versions,
	// still get a plugins client when calling this method, but all RPCs will return
	// "not implemented" errors (as per the default gRPC behavior).
	PluginsClient() pluginspb.PluginServiceClient

	// SAMLIdPClient returns a SAML IdP client.
	// Clients connecting to non-Enterprise clusters, or older Teleport versions,
	// still get a SAML IdP client when calling this method, but all RPCs will return
	// "not implemented" errors (as per the default gRPC behavior).
	SAMLIdPClient() samlidppb.SAMLIdPServiceClient

	// OktaClient returns an Okta client.
	// Clients connecting to non-Enterprise clusters, or older Teleport versions,
	// still get an Okta client when calling this method, but all RPCs will return
	// "not implemented" errors (as per the default gRPC behavior).
	OktaClient() services.Okta

	// AccessListClient returns an access list client.
	// Clients connecting to  older Teleport versions, still get an access list client
	// when calling this method, but all RPCs will return "not implemented" errors
	// (as per the default gRPC behavior).
	AccessListClient() accesslistv1.AccessListServiceClient

	// CloneHTTPClient creates a new HTTP client with the same configuration.
	CloneHTTPClient(params ...roundtrip.ClientParam) (*HTTPClient, error)

	// GetResources returns a paginated list of resources.
	GetResources(ctx context.Context, req *proto.ListResourcesRequest) (*proto.ListResourcesResponse, error)
}
