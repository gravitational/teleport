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

package authclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"time"

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/externalauditstorage"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/client/secreport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	assistpb "github.com/gravitational/teleport/api/gen/proto/go/assist/v1"
	clusterconfigpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
	dbobjectv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobject/v1"
	dbobjectimportrulev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobjectimportrule/v1"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	integrationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	loginrulepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	resourceusagepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/resourceusage/v1"
	samlidppb "github.com/gravitational/teleport/api/gen/proto/go/teleport/samlidp/v1"
	userspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	userpreferencesv1 "github.com/gravitational/teleport/api/gen/proto/go/userpreferences/v1"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/keys"
	accessgraphv1 "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// CurrentVersion is a current API version
	CurrentVersion = types.V2

	// MissingNamespaceError indicates that the client failed to
	// provide the namespace in the request.
	MissingNamespaceError = "missing required parameter: namespace"
)

// ErrNoMFADevices is returned when an MFA ceremony is performed without possible devices to
// complete the challenge with.
var ErrNoMFADevices = &trace.AccessDeniedError{
	Message: "MFA is required to access this resource but user has no MFA devices; use 'tsh mfa add' to register MFA devices",
}

// HostFQDN consists of host UUID and cluster name joined via '.'.
func HostFQDN(hostUUID, clusterName string) string {
	return fmt.Sprintf("%v.%v", hostUUID, clusterName)
}

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

	cfg.CircuitBreakerConfig.TrippedErrorMessage = "Unable to communicate with the Teleport Auth Service"

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

// notImplementedMessage is the message to return for endpoints that are not
// implemented. This is due to how service interfaces are used with Teleport.
const notImplementedMessage = "not implemented: can only be called by auth locally"

// CreateAuthPreference not implemented: can only be called locally.
func (c *Client) CreateAuthPreference(context.Context, types.AuthPreference) (types.AuthPreference, error) {
	return nil, trace.NotImplemented(notImplementedMessage)
}

// CreateSessionRecordingConfig not implemented: can only be called locally.
func (c *Client) CreateSessionRecordingConfig(context.Context, types.SessionRecordingConfig) (types.SessionRecordingConfig, error) {
	return nil, trace.NotImplemented(notImplementedMessage)
}

// CreateClusterAuditConfig not implemented: can only be called locally.
func (c *Client) CreateClusterAuditConfig(context.Context, types.ClusterAuditConfig) (types.ClusterAuditConfig, error) {
	return nil, trace.NotImplemented(notImplementedMessage)
}

func (c *Client) UpdateClusterAuditConfig(context.Context, types.ClusterAuditConfig) (types.ClusterAuditConfig, error) {
	return nil, trace.NotImplemented(notImplementedMessage)
}

func (c *Client) UpsertClusterAuditConfig(context.Context, types.ClusterAuditConfig) (types.ClusterAuditConfig, error) {
	return nil, trace.NotImplemented(notImplementedMessage)
}

func (c *Client) CreateClusterNetworkingConfig(context.Context, types.ClusterNetworkingConfig) (types.ClusterNetworkingConfig, error) {
	return nil, trace.NotImplemented(notImplementedMessage)
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

// UpdateAndSwapUser not implemented: can only be called locally.
func (c *Client) UpdateAndSwapUser(ctx context.Context, user string, withSecrets bool, fn func(types.User) (bool, error)) (types.User, error) {
	return nil, trace.NotImplemented(notImplementedMessage)
}

// CompareAndSwapUser not implemented: can only be called locally
func (c *Client) CompareAndSwapUser(ctx context.Context, new, expected types.User) error {
	return trace.NotImplemented(notImplementedMessage)
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

// DeleteAllNamespaces not implemented: can only be called locally.
func (c *Client) DeleteAllNamespaces() error {
	return trace.NotImplemented(notImplementedMessage)
}

// DeleteAllRoles not implemented: can only be called locally.
func (c *Client) DeleteAllRoles(context.Context) error {
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
func (c *Client) DeleteAllUsers(ctx context.Context) error {
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
func (c *Client) GetClusterNetworkingConfig(ctx context.Context) (types.ClusterNetworkingConfig, error) {
	return c.APIClient.GetClusterNetworkingConfig(ctx)
}

// GetSessionRecordingConfig gets session recording configuration.
func (c *Client) GetSessionRecordingConfig(ctx context.Context) (types.SessionRecordingConfig, error) {
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

func (c *Client) GetLicense(ctx context.Context) (string, error) {
	return c.APIClient.GetLicense(ctx)
}

func (c *Client) ListReleases(ctx context.Context) ([]*types.Release, error) {
	return c.APIClient.ListReleases(ctx, &proto.ListReleasesRequest{})
}

func (c *Client) OktaClient() services.Okta {
	return c.APIClient.OktaClient()
}

func (c *Client) SCIMClient() services.SCIM {
	return c.APIClient.SCIMClient()
}

// SecReportsClient returns a client for security reports.
func (c *Client) SecReportsClient() *secreport.Client {
	return c.APIClient.SecReportsClient()
}

func (c *Client) AccessListClient() services.AccessLists {
	return c.APIClient.AccessListClient()
}

// AccessMonitoringRuleClient returns the access monitoring rules client.
func (c *Client) AccessMonitoringRuleClient() services.AccessMonitoringRules {
	return c.APIClient.AccessMonitoringRulesClient()
}

func (c *Client) ExternalAuditStorageClient() *externalauditstorage.Client {
	return c.APIClient.ExternalAuditStorageClient()
}

func (c *Client) UserLoginStateClient() services.UserLoginStates {
	return c.APIClient.UserLoginStateClient()
}

func (c *Client) AccessGraphClient() accessgraphv1.AccessGraphServiceClient {
	return accessgraphv1.NewAccessGraphServiceClient(c.APIClient.GetConnection())
}

func (c *Client) IntegrationAWSOIDCClient() integrationv1.AWSOIDCServiceClient {
	return integrationv1.NewAWSOIDCServiceClient(c.APIClient.GetConnection())
}

type upsertUserRawReq struct {
	User json.RawMessage `json:"user"`
}

// UpsertUser user updates user entry.
// TODO(tross): DELETE IN 16.0.0
func (c *Client) UpsertUser(ctx context.Context, user types.User) (types.User, error) {
	upserted, err := c.APIClient.UpsertUser(ctx, user)
	if err == nil {
		return upserted, nil
	}

	if !trace.IsNotImplemented(err) {
		return nil, trace.Wrap(err)
	}

	data, err := services.MarshalUser(user)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	_, err = c.HTTPClient.PostJSON(ctx, c.Endpoint("users"), &upsertUserRawReq{User: data})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	upserted, err = c.GetUser(ctx, user.GetName(), false)
	return upserted, trace.Wrap(err)
}

// DiscoveryConfigClient returns a client for managing the DiscoveryConfig resource.
func (c *Client) DiscoveryConfigClient() services.DiscoveryConfigWithStatusUpdater {
	return c.APIClient.DiscoveryConfigClient()
}

// DeleteStaticTokens deletes static tokens
func (c *Client) DeleteStaticTokens() error {
	return trace.NotImplemented(notImplementedMessage)
}

// GetStaticTokens returns a list of static register tokens
func (c *Client) GetStaticTokens() (types.StaticTokens, error) {
	return nil, trace.NotImplemented(notImplementedMessage)
}

// SetStaticTokens sets a list of static register tokens
func (c *Client) SetStaticTokens(st types.StaticTokens) error {
	return trace.NotImplemented(notImplementedMessage)
}

const (
	// UserTokenTypeResetPasswordInvite is a token type used for the UI invite flow that
	// allows users to change their password and set second factor (if enabled).
	UserTokenTypeResetPasswordInvite = "invite"
	// UserTokenTypeResetPassword is a token type used for the UI flow where user
	// re-sets their password and second factor (if enabled).
	UserTokenTypeResetPassword = "password"
	// UserTokenTypeRecoveryStart describes a recovery token issued to users who
	// successfully verified their recovery code.
	UserTokenTypeRecoveryStart = "recovery_start"
	// UserTokenTypeRecoveryApproved describes a recovery token issued to users who
	// successfully verified their second auth credential (either password or a second factor) and
	// can now start changing their password or add a new second factor device.
	// This token is also used to allow users to delete exisiting second factor devices
	// and retrieve their new set of recovery codes as part of the recovery flow.
	UserTokenTypeRecoveryApproved = "recovery_approved"
	// UserTokenTypePrivilege describes a token type that grants access to a privileged action
	// that requires users to re-authenticate with their second factor while looged in. This
	// token is issued to users who has successfully re-authenticated.
	UserTokenTypePrivilege = "privilege"
	// UserTokenTypePrivilegeException describes a token type that allowed a user to bypass
	// second factor re-authentication which in other cases would be required eg:
	// allowing user to add a mfa device if they don't have any registered.
	UserTokenTypePrivilegeException = "privilege_exception"

	// userTokenTypePrivilegeOTP is used to hold OTP data during (otherwise)
	// token-less registrations.
	// This kind of token is an internal artifact of Teleport and should only be
	// allowed for OTP device registrations.
	userTokenTypePrivilegeOTP = "privilege_otp"
)

// CreateUserTokenRequest is a request to create a new user token.
type CreateUserTokenRequest struct {
	// Name is the user name for token.
	Name string `json:"name"`
	// TTL specifies how long the generated token is valid for.
	TTL time.Duration `json:"ttl"`
	// Type is the token type.
	Type string `json:"type"`
}

// CheckAndSetDefaults checks and sets the defaults.
func (r *CreateUserTokenRequest) CheckAndSetDefaults() error {
	if r.Name == "" {
		return trace.BadParameter("user name can't be empty")
	}

	if r.TTL < 0 {
		return trace.BadParameter("TTL can't be negative")
	}

	if r.Type == "" {
		r.Type = UserTokenTypeResetPassword
	}

	switch r.Type {
	case UserTokenTypeResetPasswordInvite:
		if r.TTL == 0 {
			r.TTL = defaults.SignupTokenTTL
		}

		if r.TTL > defaults.MaxSignupTokenTTL {
			return trace.BadParameter(
				"failed to create user token for reset password invite: maximum token TTL is %v hours",
				defaults.MaxSignupTokenTTL)
		}

	case UserTokenTypeResetPassword:
		if r.TTL == 0 {
			r.TTL = defaults.ChangePasswordTokenTTL
		}

		if r.TTL > defaults.MaxChangePasswordTokenTTL {
			return trace.BadParameter(
				"failed to create user token for reset password: maximum token TTL is %v hours",
				defaults.MaxChangePasswordTokenTTL)
		}

	case UserTokenTypeRecoveryStart:
		r.TTL = defaults.RecoveryStartTokenTTL

	case UserTokenTypeRecoveryApproved:
		r.TTL = defaults.RecoveryApprovedTokenTTL

	case UserTokenTypePrivilege, UserTokenTypePrivilegeException, userTokenTypePrivilegeOTP:
		r.TTL = defaults.PrivilegeTokenTTL

	default:
		return trace.BadParameter("unknown user token request type(%v)", r.Type)
	}

	return nil
}

type WebSessionReq struct {
	// User is the user name associated with the session id.
	User string `json:"user"`
	// PrevSessionID is the id of current session.
	PrevSessionID string `json:"prev_session_id"`
	// AccessRequestID is an optional field that holds the id of an approved access request.
	AccessRequestID string `json:"access_request_id"`
	// Switchback is a flag to indicate if user is wanting to switchback from an assumed role
	// back to their default role.
	Switchback bool `json:"switchback"`
	// ReloadUser is a flag to indicate if user needs to be refetched from the backend
	// to apply new user changes e.g. user traits were updated.
	ReloadUser bool `json:"reload_user"`
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

// OIDCAuthResponse is returned when auth server validated callback parameters
// returned from OIDC provider
type OIDCAuthResponse struct {
	// Username is authenticated teleport username
	Username string `json:"username"`
	// Identity contains validated OIDC identity
	Identity types.ExternalIdentity `json:"identity"`
	// Web session will be generated by auth server if requested in OIDCAuthRequest
	Session types.WebSession `json:"session,omitempty"`
	// Cert will be generated by certificate authority
	Cert []byte `json:"cert,omitempty"`
	// TLSCert is PEM encoded TLS certificate
	TLSCert []byte `json:"tls_cert,omitempty"`
	// Req is original oidc auth request
	Req OIDCAuthRequest `json:"req"`
	// HostSigners is a list of signing host public keys
	// trusted by proxy, used in console login
	HostSigners []types.CertAuthority `json:"host_signers"`
}

// OIDCAuthRequest is an OIDC auth request that supports standard json marshaling.
type OIDCAuthRequest struct {
	// ConnectorID is ID of OIDC connector this request uses
	ConnectorID string `json:"connector_id"`
	// CSRFToken is associated with user web session token
	CSRFToken string `json:"csrf_token"`
	// PublicKey is an optional public key, users want these
	// keys to be signed by auth servers user CA in case
	// of successful auth
	PublicKey []byte `json:"public_key"`
	// CreateWebSession indicates if user wants to generate a web
	// session after successful authentication
	CreateWebSession bool `json:"create_web_session"`
	// ClientRedirectURL is a URL client wants to be redirected
	// after successful authentication
	ClientRedirectURL string `json:"client_redirect_url"`
}

// SAMLAuthResponse is returned when auth server validated callback parameters
// returned from SAML identity provider
type SAMLAuthResponse struct {
	// Username is an authenticated teleport username
	Username string `json:"username"`
	// Identity contains validated SAML identity
	Identity types.ExternalIdentity `json:"identity"`
	// Web session will be generated by auth server if requested in SAMLAuthRequest
	Session types.WebSession `json:"session,omitempty"`
	// Cert will be generated by certificate authority
	Cert []byte `json:"cert,omitempty"`
	// TLSCert is a PEM encoded TLS certificate
	TLSCert []byte `json:"tls_cert,omitempty"`
	// Req is an original SAML auth request
	Req SAMLAuthRequest `json:"req"`
	// HostSigners is a list of signing host public keys
	// trusted by proxy, used in console login
	HostSigners []types.CertAuthority `json:"host_signers"`
}

// SAMLAuthRequest is a SAML auth request that supports standard json marshaling.
type SAMLAuthRequest struct {
	// ID is a unique request ID.
	ID string `json:"id"`
	// PublicKey is an optional public key, users want these
	// keys to be signed by auth servers user CA in case
	// of successful auth.
	PublicKey []byte `json:"public_key"`
	// CSRFToken is associated with user web session token.
	CSRFToken string `json:"csrf_token"`
	// CreateWebSession indicates if user wants to generate a web
	// session after successful authentication.
	CreateWebSession bool `json:"create_web_session"`
	// ClientRedirectURL is a URL client wants to be redirected
	// after successful authentication.
	ClientRedirectURL string `json:"client_redirect_url"`
}

// GithubAuthResponse represents Github auth callback validation response
type GithubAuthResponse struct {
	// Username is the name of authenticated user
	Username string `json:"username"`
	// Identity is the external identity
	Identity types.ExternalIdentity `json:"identity"`
	// Session is the created web session
	Session types.WebSession `json:"session,omitempty"`
	// Cert is the generated SSH client certificate
	Cert []byte `json:"cert,omitempty"`
	// TLSCert is PEM encoded TLS client certificate
	TLSCert []byte `json:"tls_cert,omitempty"`
	// Req is the original auth request
	Req GithubAuthRequest `json:"req"`
	// HostSigners is a list of signing host public keys
	// trusted by proxy, used in console login
	HostSigners []types.CertAuthority `json:"host_signers"`
}

// GithubAuthRequest is an Github auth request that supports standard json marshaling
type GithubAuthRequest struct {
	// ConnectorID is the name of the connector to use.
	ConnectorID string `json:"connector_id"`
	// CSRFToken is used to protect against CSRF attacks.
	CSRFToken string `json:"csrf_token"`
	// PublicKey is an optional public key to sign in case of successful auth.
	PublicKey []byte `json:"public_key"`
	// CreateWebSession indicates that a user wants to generate a web session
	// after successful authentication.
	CreateWebSession bool `json:"create_web_session"`
	// ClientRedirectURL is the URL where client will be redirected after
	// successful auth.
	ClientRedirectURL string `json:"client_redirect_url"`
}

// IdentityService manages identities and users
type IdentityService interface {
	// CreateOIDCConnector creates a new OIDC connector.
	CreateOIDCConnector(ctx context.Context, connector types.OIDCConnector) (types.OIDCConnector, error)
	// UpdateOIDCConnector updates an existing OIDC connector.
	UpdateOIDCConnector(ctx context.Context, connector types.OIDCConnector) (types.OIDCConnector, error)
	// UpsertOIDCConnector updates or creates an OIDC connector.
	UpsertOIDCConnector(ctx context.Context, connector types.OIDCConnector) (types.OIDCConnector, error)
	// GetOIDCConnector returns OIDC connector information by id
	GetOIDCConnector(ctx context.Context, id string, withSecrets bool) (types.OIDCConnector, error)
	// GetOIDCConnectors gets valid OIDC connectors list
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
	UpsertSAMLConnector(ctx context.Context, connector types.SAMLConnector) (types.SAMLConnector, error)
	// GetSAMLConnector returns SAML connector information by id
	GetSAMLConnector(ctx context.Context, id string, withSecrets bool) (types.SAMLConnector, error)
	// GetSAMLConnectors gets valid SAML connectors list
	GetSAMLConnectors(ctx context.Context, withSecrets bool) ([]types.SAMLConnector, error)
	// DeleteSAMLConnector deletes SAML connector by ID
	DeleteSAMLConnector(ctx context.Context, connectorID string) error
	// CreateSAMLAuthRequest creates SAML AuthnRequest
	CreateSAMLAuthRequest(ctx context.Context, req types.SAMLAuthRequest) (*types.SAMLAuthRequest, error)
	// ValidateSAMLResponse validates SAML auth response
	ValidateSAMLResponse(ctx context.Context, samlResponse, connectorID, clientIP string) (*SAMLAuthResponse, error)
	// GetSAMLAuthRequest returns SAML auth request if found
	GetSAMLAuthRequest(ctx context.Context, authRequestID string) (*types.SAMLAuthRequest, error)

	// CreateGithubConnector creates a new Github connector.
	CreateGithubConnector(ctx context.Context, connector types.GithubConnector) (types.GithubConnector, error)
	// UpdateGithubConnector updates an existing Github connector.
	UpdateGithubConnector(ctx context.Context, connector types.GithubConnector) (types.GithubConnector, error)
	// UpsertGithubConnector creates or updates a Github connector.
	UpsertGithubConnector(ctx context.Context, connector types.GithubConnector) (types.GithubConnector, error)
	// GetGithubConnectors returns valid Github connectors
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

	// ListUsersExt is equivalent to ListUsers except it supports additional parameters.
	ListUsersExt(ctx context.Context, req *userspb.ListUsersRequest) (*userspb.ListUsersResponse, error)

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

type ValidateTrustedClusterRequest struct {
	Token           string                `json:"token"`
	CAs             []types.CertAuthority `json:"certificate_authorities"`
	TeleportVersion string                `json:"teleport_version"`
}

func (v *ValidateTrustedClusterRequest) ToRaw() (*ValidateTrustedClusterRequestRaw, error) {
	var cas [][]byte

	for _, certAuthority := range v.CAs {
		data, err := services.MarshalCertAuthority(certAuthority)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		cas = append(cas, data)
	}

	return &ValidateTrustedClusterRequestRaw{
		Token:           v.Token,
		CAs:             cas,
		TeleportVersion: v.TeleportVersion,
	}, nil
}

type ValidateTrustedClusterRequestRaw struct {
	Token           string   `json:"token"`
	CAs             [][]byte `json:"certificate_authorities"`
	TeleportVersion string   `json:"teleport_version"`
}

func (v *ValidateTrustedClusterRequestRaw) ToNative() (*ValidateTrustedClusterRequest, error) {
	var cas []types.CertAuthority

	for _, rawCertAuthority := range v.CAs {
		certAuthority, err := services.UnmarshalCertAuthority(rawCertAuthority)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		cas = append(cas, certAuthority)
	}

	return &ValidateTrustedClusterRequest{
		Token:           v.Token,
		CAs:             cas,
		TeleportVersion: v.TeleportVersion,
	}, nil
}

type ValidateTrustedClusterResponse struct {
	CAs []types.CertAuthority `json:"certificate_authorities"`
}

func (v *ValidateTrustedClusterResponse) ToRaw() (*ValidateTrustedClusterResponseRaw, error) {
	var cas [][]byte

	for _, certAuthority := range v.CAs {
		data, err := services.MarshalCertAuthority(certAuthority)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		cas = append(cas, data)
	}

	return &ValidateTrustedClusterResponseRaw{
		CAs: cas,
	}, nil
}

type ValidateTrustedClusterResponseRaw struct {
	CAs [][]byte `json:"certificate_authorities"`
}

func (v *ValidateTrustedClusterResponseRaw) ToNative() (*ValidateTrustedClusterResponse, error) {
	var cas []types.CertAuthority

	for _, rawCertAuthority := range v.CAs {
		certAuthority, err := services.UnmarshalCertAuthority(rawCertAuthority)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		cas = append(cas, certAuthority)
	}

	return &ValidateTrustedClusterResponse{
		CAs: cas,
	}, nil
}

// AuthenticateUserRequest is a request to authenticate interactive user
type AuthenticateUserRequest struct {
	// Username is a username
	Username string `json:"username"`
	// PublicKey is a public key in ssh authorized_keys format
	PublicKey []byte `json:"public_key"`
	// Pass is a password used in local authentication schemes
	Pass *PassCreds `json:"pass,omitempty"`
	// Webauthn is a signed credential assertion, used in MFA authentication
	Webauthn *wantypes.CredentialAssertionResponse `json:"webauthn,omitempty"`
	// OTP is a password and second factor, used for MFA authentication
	OTP *OTPCreds `json:"otp,omitempty"`
	// Session is a web session credential used to authenticate web sessions
	Session *SessionCreds `json:"session,omitempty"`
	// ClientMetadata includes forwarded information about a client
	ClientMetadata *ForwardedClientMetadata `json:"client_metadata,omitempty"`
	// HeadlessAuthenticationID is the ID for a headless authentication resource.
	HeadlessAuthenticationID string `json:"headless_authentication_id"`
}

// ForwardedClientMetadata can be used by the proxy web API to forward information about
// the client to the auth service.
type ForwardedClientMetadata struct {
	UserAgent string `json:"user_agent,omitempty"`
	// RemoteAddr is the IP address of the end user. This IP address is derived
	// either from a direct client connection, or from a PROXY protocol header
	// if the connection is forwarded through a load balancer.
	RemoteAddr string `json:"remote_addr,omitempty"`
}

// CheckAndSetDefaults checks and sets defaults
func (a *AuthenticateUserRequest) CheckAndSetDefaults() error {
	switch {
	case a.Username == "" && a.Webauthn != nil: // OK, passwordless.
	case a.Username == "":
		return trace.BadParameter("missing parameter 'username'")
	case a.Pass == nil && a.Webauthn == nil && a.OTP == nil && a.Session == nil && a.HeadlessAuthenticationID == "":
		return trace.BadParameter("at least one authentication method is required")
	}
	return nil
}

// PassCreds is a password credential
type PassCreds struct {
	// Password is a user password
	Password []byte `json:"password"`
}

// OTPCreds is a two-factor authentication credentials
type OTPCreds struct {
	// Password is a user password
	Password []byte `json:"password"`
	// Token is a user second factor token
	Token string `json:"token"`
}

// SessionCreds is a web session credentials
type SessionCreds struct {
	// ID is a web session id
	ID string `json:"id"`
}

// AuthenticateSSHRequest is a request to authenticate SSH client user via CLI
type AuthenticateSSHRequest struct {
	// AuthenticateUserRequest is a request with credentials
	AuthenticateUserRequest
	// TTL is a requested TTL for certificates to be issues
	TTL time.Duration `json:"ttl"`
	// CompatibilityMode sets certificate compatibility mode with old SSH clients
	CompatibilityMode string `json:"compatibility_mode"`
	RouteToCluster    string `json:"route_to_cluster"`
	// KubernetesCluster sets the target kubernetes cluster for the TLS
	// certificate. This can be empty on older clients.
	KubernetesCluster string `json:"kubernetes_cluster"`
	// AttestationStatement is an attestation statement associated with the given public key.
	AttestationStatement *keys.AttestationStatement `json:"attestation_statement,omitempty"`
}

// CheckAndSetDefaults checks and sets default certificate values
func (a *AuthenticateSSHRequest) CheckAndSetDefaults() error {
	if err := a.AuthenticateUserRequest.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if len(a.PublicKey) == 0 {
		return trace.BadParameter("missing parameter 'public_key'")
	}
	certificateFormat, err := utils.CheckCertificateFormatFlag(a.CompatibilityMode)
	if err != nil {
		return trace.Wrap(err)
	}
	a.CompatibilityMode = certificateFormat
	return nil
}

// SSHLoginResponse is a response returned by web proxy, it preserves backwards compatibility
// on the wire, which is the primary reason for non-matching json tags
type SSHLoginResponse struct {
	// User contains a logged-in user information
	Username string `json:"username"`
	// Cert is a PEM encoded  signed certificate
	Cert []byte `json:"cert"`
	// TLSCertPEM is a PEM encoded TLS certificate signed by TLS certificate authority
	TLSCert []byte `json:"tls_cert"`
	// HostSigners is a list of signing host public keys trusted by proxy
	HostSigners []TrustedCerts `json:"host_signers"`
}

// TrustedCerts contains host certificates, it preserves backwards compatibility
// on the wire, which is the primary reason for non-matching json tags
type TrustedCerts struct {
	// ClusterName identifies teleport cluster name this authority serves,
	// for host authorities that means base hostname of all servers,
	// for user authorities that means organization name
	ClusterName string `json:"domain_name"`
	// AuthorizedKeys is a list of SSH public keys in authorized_keys format
	// that can be used to check host key signatures.
	AuthorizedKeys [][]byte `json:"checking_keys"`
	// TLSCertificates is a list of TLS certificates of the certificate authority
	// of the authentication server
	TLSCertificates [][]byte `json:"tls_certs"`
}

// SSHCertPublicKeys returns a list of trusted host SSH certificate authority public keys
func (c *TrustedCerts) SSHCertPublicKeys() ([]ssh.PublicKey, error) {
	out := make([]ssh.PublicKey, 0, len(c.AuthorizedKeys))
	for _, keyBytes := range c.AuthorizedKeys {
		publicKey, _, _, _, err := ssh.ParseAuthorizedKey(keyBytes)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, publicKey)
	}
	return out, nil
}

// AuthoritiesToTrustedCerts serializes authorities to TrustedCerts data structure
func AuthoritiesToTrustedCerts(authorities []types.CertAuthority) []TrustedCerts {
	out := make([]TrustedCerts, len(authorities))
	for i, ca := range authorities {
		out[i] = TrustedCerts{
			ClusterName:     ca.GetClusterName(),
			AuthorizedKeys:  services.GetSSHCheckingKeys(ca),
			TLSCertificates: services.GetTLSCerts(ca),
		}
	}
	return out
}

// KubeCSR is a kubernetes CSR request
type KubeCSR struct {
	// Username of user's certificate
	Username string `json:"username"`
	// ClusterName is a name of the target cluster to generate certificate for
	ClusterName string `json:"cluster_name"`
	// CSR is a kubernetes CSR
	CSR []byte `json:"csr"`
}

// CheckAndSetDefaults checks and sets defaults
func (a *KubeCSR) CheckAndSetDefaults() error {
	if len(a.CSR) == 0 {
		return trace.BadParameter("missing parameter 'csr'")
	}
	return nil
}

// KubeCSRResponse is a response to kubernetes CSR request
type KubeCSRResponse struct {
	// Cert is a signed certificate PEM block
	Cert []byte `json:"cert"`
	// CertAuthorities is a list of PEM block with trusted cert authorities
	CertAuthorities [][]byte `json:"cert_authorities"`
	// TargetAddr is an optional target address
	// of the kubernetes API server that can be set
	// in the kubeconfig
	TargetAddr string `json:"target_addr"`
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
	services.KubeWaitingContainer
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

	// IntegrationAWSOIDCClient returns a client to the Integration AWS OIDC gRPC service.
	IntegrationAWSOIDCClient() integrationv1.AWSOIDCServiceClient

	// NewKeepAliver returns a new instance of keep aliver
	NewKeepAliver(ctx context.Context) (types.KeepAliver, error)

	// RotateCertAuthority starts or restarts certificate authority rotation process.
	RotateCertAuthority(ctx context.Context, req types.RotateRequest) error

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

	// GenerateDatabaseCert generates a client certificate used by a database
	// service to authenticate with the database instance, or a server certificate
	// for configuring a self-hosted database, depending on the requester_name.
	GenerateDatabaseCert(context.Context, *proto.DatabaseCertRequest) (*proto.DatabaseCertResponse, error)

	// GetWebSession queries the existing web session described with req.
	// Implements ReadAccessPoint.
	GetWebSession(ctx context.Context, req types.GetWebSessionRequest) (types.WebSession, error)

	// GetWebToken queries the existing web token described with req.
	// Implements ReadAccessPoint.
	GetWebToken(ctx context.Context, req types.GetWebTokenRequest) (types.WebToken, error)

	// GenerateAWSOIDCToken generates a token to be used to execute an AWS OIDC Integration action.
	GenerateAWSOIDCToken(ctx context.Context, integration string) (string, error)

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

	// SCIMClient returns a client for the SCIM provisioning service. Clients
	// connecting to OSS clusters will still get a client when calling this method,
	// but the back-end service will fail all requests with "Not Implemented" as per the
	// default GRPC behavior.
	SCIMClient() services.SCIM

	// AccessListClient returns an access list client.
	// Clients connecting to older Teleport versions still get an access list client
	// when calling this method, but all RPCs will return "not implemented" errors
	// (as per the default gRPC behavior).
	AccessListClient() services.AccessLists

	// AccessMonitoringRuleClient returns an access monitoring rule client.
	// Clients connecting to older Teleport versions still get an access list client
	// when calling this method, but all RPCs will return "not implemented" errors
	// (as per the default gRPC behavior).
	AccessMonitoringRuleClient() services.AccessMonitoringRules

	// DatabaseObjectImportRuleClient returns a database import rule client.
	DatabaseObjectImportRuleClient() dbobjectimportrulev1.DatabaseObjectImportRuleServiceClient

	// DatabaseObjectClient returns a database object client.
	DatabaseObjectClient() dbobjectv1.DatabaseObjectServiceClient

	// SecReportsClient returns a client for security reports.
	// Clients connecting to  older Teleport versions, still get an access list client
	// when calling this method, but all RPCs will return "not implemented" errors
	// (as per the default gRPC behavior).
	SecReportsClient() *secreport.Client

	// BotServiceClient returns a client for security reports.
	// Clients connecting to  older Teleport versions, still get a bot service client
	// when calling this method, but all RPCs will return "not implemented" errors
	// (as per the default gRPC behavior).
	BotServiceClient() machineidv1pb.BotServiceClient

	// UserLoginStateClient returns a user login state client.
	// Clients connecting to older Teleport versions still get a user login state client
	// when calling this method, but all RPCs will return "not implemented" errors
	// (as per the default gRPC behavior).
	UserLoginStateClient() services.UserLoginStates

	// DiscoveryConfigClient returns a DiscoveryConfig client.
	// Clients connecting to older Teleport versions, still get an DiscoveryConfig client
	// when calling this method, but all RPCs will return "not implemented" errors
	// (as per the default gRPC behavior).
	DiscoveryConfigClient() services.DiscoveryConfigWithStatusUpdater

	// ResourceUsageClient returns a resource usage service client.
	// Clients connecting to non-Enterprise clusters, or older Teleport versions,
	// still get a client when calling this method, but all RPCs will return
	// "not implemented" errors (as per the default gRPC behavior).
	ResourceUsageClient() resourceusagepb.ResourceUsageServiceClient

	// ExternalAuditStorageClient returns an External Audit Storage client.
	// Clients connecting to non-Enterprise clusters, or older Teleport versions,
	// still get a client when calling this method, but all RPCs will return
	// "not implemented" errors (as per the default gRPC behavior).
	ExternalAuditStorageClient() *externalauditstorage.Client

	// WorkloadIdentityServiceClient returns a workload identity service client.
	// Clients connecting to  older Teleport versions, still get a client
	// when calling this method, but all RPCs will return "not implemented" errors
	// (as per the default gRPC behavior).
	WorkloadIdentityServiceClient() machineidv1pb.WorkloadIdentityServiceClient

	// ClusterConfigClient returns a Cluster Configuration client.
	// Clients connecting to non-Enterprise clusters, or older Teleport versions,
	// still get a client when calling this method, but all RPCs will return
	// "not implemented" errors (as per the default gRPC behavior).
	ClusterConfigClient() clusterconfigpb.ClusterConfigServiceClient

	// CloneHTTPClient creates a new HTTP client with the same configuration.
	CloneHTTPClient(params ...roundtrip.ClientParam) (*HTTPClient, error)

	// GetResources returns a paginated list of resources.
	GetResources(ctx context.Context, req *proto.ListResourcesRequest) (*proto.ListResourcesResponse, error)

	// GetUserPreferences returns the user preferences for a given user.
	GetUserPreferences(ctx context.Context, req *userpreferencesv1.GetUserPreferencesRequest) (*userpreferencesv1.GetUserPreferencesResponse, error)

	// UpsertUserPreferences creates or updates user preferences for a given username.
	UpsertUserPreferences(ctx context.Context, req *userpreferencesv1.UpsertUserPreferencesRequest) error

	// ListAllAccessRequests is a helper for using the ListAccessRequests API's additional sort order/index features without
	// mucking about with pagination. It also implements backwards-comatibility with older control planes that only
	// support GetAccessRequests.
	ListAllAccessRequests(ctx context.Context, req *proto.ListAccessRequestsRequest) ([]*types.AccessRequestV3, error)

	// ListUnifiedResources returns a paginated list of unified resources.
	ListUnifiedResources(ctx context.Context, req *proto.ListUnifiedResourcesRequest) (*proto.ListUnifiedResourcesResponse, error)

	// GetSSHTargets gets all servers that would match an equivalent ssh dial request. Note that this method
	// returns all resources directly accessible to the user *and* all resources available via 'SearchAsRoles',
	// which is what we want when handling things like ambiguous host errors and resource-based access requests,
	// but may result in confusing behavior if it is used outside of those contexts.
	GetSSHTargets(ctx context.Context, req *proto.GetSSHTargetsRequest) (*proto.GetSSHTargetsResponse, error)

	// PerformMFACeremony retrieves an MFA challenge from the server with the given challenge extensions
	// and prompts the user to answer the challenge with the given promptOpts, and ultimately returning
	// an MFA challenge response for the user.
	PerformMFACeremony(ctx context.Context, challengeRequest *proto.CreateAuthenticateChallengeRequest, promptOpts ...mfa.PromptOpt) (*proto.MFAAuthenticateResponse, error)

	// GetClusterAccessGraphConfig retrieves the cluster Access Graph configuration from Auth server.
	GetClusterAccessGraphConfig(ctx context.Context) (*clusterconfigpb.AccessGraphConfig, error)

	// GenerateAppToken creates a JWT token with application access.
	GenerateAppToken(ctx context.Context, req types.GenerateAppTokenRequest) (string, error)
}

// WaitForAppSession will block until the requested application session shows up in the
// cache or a timeout occurs.
func WaitForAppSession(ctx context.Context, sessionID, user string, ap ReadProxyAccessPoint) error {
	req := waitForWebSessionReq{
		newWatcherFn: ap.NewWatcher,
		getSessionFn: func(ctx context.Context, sessionID string) (types.WebSession, error) {
			return ap.GetAppSession(ctx, types.GetAppSessionRequest{SessionID: sessionID})
		},
	}
	return trace.Wrap(waitForWebSession(ctx, sessionID, user, types.KindAppSession, req))
}

// WaitForSnowflakeSession waits until the requested Snowflake session shows up int the cache
// or a timeout occurs.
func WaitForSnowflakeSession(ctx context.Context, sessionID, user string, ap SnowflakeSessionWatcher) error {
	req := waitForWebSessionReq{
		newWatcherFn: ap.NewWatcher,
		getSessionFn: func(ctx context.Context, sessionID string) (types.WebSession, error) {
			return ap.GetSnowflakeSession(ctx, types.GetSnowflakeSessionRequest{SessionID: sessionID})
		},
	}
	return trace.Wrap(waitForWebSession(ctx, sessionID, user, types.KindSnowflakeSession, req))
}

// waitForWebSessionReq is a request to wait for web session to be populated in the application cache.
type waitForWebSessionReq struct {
	// newWatcherFn is a function that returns new event watcher.
	newWatcherFn func(ctx context.Context, watch types.Watch) (types.Watcher, error)
	// getSessionFn is a function that returns web session by given ID.
	getSessionFn func(ctx context.Context, sessionID string) (types.WebSession, error)
}

// waitForWebSession is an implementation for web session wait functions.
func waitForWebSession(ctx context.Context, sessionID, user string, evenSubKind string, req waitForWebSessionReq) error {
	_, err := req.getSessionFn(ctx, sessionID)
	if err == nil {
		return nil
	}
	logger := log.WithField("session", sessionID)
	if !trace.IsNotFound(err) {
		logger.WithError(err).Debug("Failed to query web session.")
	}
	// Establish a watch on application session.
	watcher, err := req.newWatcherFn(ctx, types.Watch{
		Name: teleport.ComponentAppProxy,
		Kinds: []types.WatchKind{
			{
				Kind:    types.KindWebSession,
				SubKind: evenSubKind,
				Filter:  (&types.WebSessionFilter{User: user}).IntoMap(),
			},
		},
		MetricComponent: teleport.ComponentAppProxy,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer watcher.Close()
	matchEvent := func(event types.Event) (types.Resource, error) {
		if event.Type == types.OpPut &&
			event.Resource.GetKind() == types.KindWebSession &&
			event.Resource.GetSubKind() == evenSubKind &&
			event.Resource.GetName() == sessionID {
			return event.Resource, nil
		}
		return nil, trace.CompareFailed("no match")
	}
	_, err = local.WaitForEvent(ctx, watcher, local.EventMatcherFunc(matchEvent), clockwork.NewRealClock())
	if err != nil {
		logger.WithError(err).Warn("Failed to wait for web session.")
		// See again if we maybe missed the event but the session was actually created.
		if _, err := req.getSessionFn(ctx, sessionID); err == nil {
			return nil
		}
	}
	return trace.Wrap(err)
}
