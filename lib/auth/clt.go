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
	"github.com/gravitational/teleport/lib/auth/authclient"
	"net/url"
	"time"

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/client/secreport"
	assistpb "github.com/gravitational/teleport/api/gen/proto/go/assist/v1"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	loginrulepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"
	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	resourceusagepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/resourceusage/v1"
	samlidppb "github.com/gravitational/teleport/api/gen/proto/go/teleport/samlidp/v1"
	userpreferencesv1 "github.com/gravitational/teleport/api/gen/proto/go/userpreferences/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	accessgraphv1 "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
)

const (
	// CurrentVersion is a current API version
	CurrentVersion = types.V2

	// MissingNamespaceError indicates that the client failed to
	// provide the namespace in the request.
	MissingNamespaceError = "missing required parameter: namespace"
)

// Make sure Client implements all the necessary methods.
var _ ClientI = &authclient.Client{}

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
	// CreateOIDCConnector creates a new OIDC connector.
	CreateOIDCConnector(ctx context.Context, connector types.OIDCConnector) (types.OIDCConnector, error)
	// UpdateOIDCConnector updates an existing OIDC connector.
	UpdateOIDCConnector(ctx context.Context, connector types.OIDCConnector) (types.OIDCConnector, error)
	// UpsertOIDCConnector updates or creates an OIDC connector.
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

	// CreateSAMLConnector creates a new SAML connector.
	CreateSAMLConnector(ctx context.Context, connector types.SAMLConnector) (types.SAMLConnector, error)
	// UpdateSAMLConnector updates an existing SAML connector
	UpdateSAMLConnector(ctx context.Context, connector types.SAMLConnector) (types.SAMLConnector, error)
	// UpsertSAMLConnector updates or creates a SAML connector
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

	// CreateGithubConnector creates a new Github connector.
	CreateGithubConnector(ctx context.Context, connector types.GithubConnector) (types.GithubConnector, error)
	// UpdateGithubConnector updates an existing Github connector.
	UpdateGithubConnector(ctx context.Context, connector types.GithubConnector) (types.GithubConnector, error)
	// UpsertGithubConnector creates or updates a Github connector.
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
	GetUser(ctx context.Context, name string, withSecrets bool) (types.User, error)

	// GetCurrentUser returns current user as seen by the server.
	// Useful especially in the context of remote clusters which perform role and trait mapping.
	GetCurrentUser(ctx context.Context) (types.User, error)

	// GetCurrentUserRoles returns current user's roles.
	GetCurrentUserRoles(ctx context.Context) ([]types.Role, error)

	// CreateUser inserts a new entry in a backend.
	CreateUser(ctx context.Context, user types.User) (types.User, error)

	// UpdateUser updates an existing user in a backend.
	UpdateUser(ctx context.Context, user types.User) (types.User, error)

	// UpdateAndSwapUser reads an existing user, runs `fn` against it and writes
	// the result to storage. Return `false` from `fn` to avoid storage changes.
	// Roughly equivalent to [GetUser] followed by [CompareAndSwapUser].
	// Returns the storage user.
	UpdateAndSwapUser(ctx context.Context, user string, withSecrets bool, fn func(types.User) (changed bool, err error)) (types.User, error)

	// UpsertUser user updates or inserts user entry
	UpsertUser(ctx context.Context, user types.User) (types.User, error)

	// CompareAndSwapUser updates an existing user in a backend, but fails if
	// the user in the backend does not match the expected value.
	CompareAndSwapUser(ctx context.Context, new, expected types.User) error

	// DeleteUser deletes an existng user in a backend by username.
	DeleteUser(ctx context.Context, user string) error

	// GetUsers returns a list of usernames registered in the system
	GetUsers(ctx context.Context, withSecrets bool) ([]types.User, error)

	// ListUsers returns a page of users.
	ListUsers(ctx context.Context, pageSize int, pageToken string, withSecrets bool) ([]types.User, string, error)

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
	//
	// Deprecated: Use GenerateUserCerts instead.
	GenerateUserSingleUseCerts(ctx context.Context) (proto.AuthService_GenerateUserSingleUseCertsClient, error)

	// IsMFARequired is a request to check whether MFA is required to
	// access the Target.
	IsMFARequired(ctx context.Context, req *proto.IsMFARequiredRequest) (*proto.IsMFARequiredResponse, error)

	// DeleteAllUsers deletes all users
	DeleteAllUsers(ctx context.Context) error

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
	// Deprecated: Use AddMFADeviceSync instead.
	AddMFADevice(ctx context.Context) (proto.AuthService_AddMFADeviceClient, error)
	// Deprecated: Use DeleteMFADeviceSync instead.
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

	// AccessGraphClient returns a client to the Access Graph gRPC service.
	AccessGraphClient() accessgraphv1.AccessGraphServiceClient

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
	// Clients connecting to older Teleport versions still get an access list client
	// when calling this method, but all RPCs will return "not implemented" errors
	// (as per the default gRPC behavior).
	AccessListClient() services.AccessLists

	// SecReportsClient returns a client for security reports.
	// Clients connecting to  older Teleport versions, still get an access list client
	// when calling this method, but all RPCs will return "not implemented" errors
	// (as per the default gRPC behavior).
	SecReportsClient() *secreport.Client

	// UserLoginStateClient returns a user login state client.
	// Clients connecting to older Teleport versions still get a user login state client
	// when calling this method, but all RPCs will return "not implemented" errors
	// (as per the default gRPC behavior).
	UserLoginStateClient() services.UserLoginStates

	// DiscoveryConfigClient returns a DiscoveryConfig client.
	// Clients connecting to older Teleport versions, still get an DiscoveryConfig client
	// when calling this method, but all RPCs will return "not implemented" errors
	// (as per the default gRPC behavior).
	DiscoveryConfigClient() services.DiscoveryConfigs

	// ResourceUsageClient returns a resource usage service client.
	// Clients connecting to non-Enterprise clusters, or older Teleport versions,
	// still get a client when calling this method, but all RPCs will return
	// "not implemented" errors (as per the default gRPC behavior).
	ResourceUsageClient() resourceusagepb.ResourceUsageServiceClient

	// ExternalCloudAuditClient returns an external cloud audit client.
	// Clients connecting to non-Enterprise clusters, or older Teleport versions,
	// still get a client when calling this method, but all RPCs will return
	// "not implemented" errors (as per the default gRPC behavior).
	ExternalCloudAuditClient() services.ExternalCloudAudits

	// CloneHTTPClient creates a new HTTP client with the same configuration.
	CloneHTTPClient(params ...roundtrip.ClientParam) (*authclient.HTTPClient, error)

	// GetResources returns a paginated list of resources.
	GetResources(ctx context.Context, req *proto.ListResourcesRequest) (*proto.ListResourcesResponse, error)

	// GetUserPreferences returns the user preferences for a given user.
	GetUserPreferences(ctx context.Context, req *userpreferencesv1.GetUserPreferencesRequest) (*userpreferencesv1.GetUserPreferencesResponse, error)

	// UpsertUserPreferences creates or updates user preferences for a given username.
	UpsertUserPreferences(ctx context.Context, req *userpreferencesv1.UpsertUserPreferencesRequest) error

	// ListUnifiedResources returns a paginated list of unified resources.
	ListUnifiedResources(ctx context.Context, req *proto.ListUnifiedResourcesRequest) (*proto.ListUnifiedResourcesResponse, error)

	// GetSSHTargets gets all servers that would match an equivalent ssh dial request. Note that this method
	// returns all resources directly accessible to the user *and* all resources available via 'SearchAsRoles',
	// which is what we want when handling things like ambiguous host errors and resource-based access requests,
	// but may result in confusing behavior if it is used outside of those contexts.
	GetSSHTargets(ctx context.Context, req *proto.GetSSHTargetsRequest) (*proto.GetSSHTargetsResponse, error)

	// ValidateMFAAuthResponse validates an MFA or passwordless challenge.
	// Returns the device used to solve the challenge (if applicable) and the username.
	ValidateMFAAuthResponse(ctx context.Context, resp *proto.MFAAuthenticateResponse, user string, passwordless bool) (*types.MFADevice, string, error)
}
