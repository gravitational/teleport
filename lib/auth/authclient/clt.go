/*
Copyright 2023 Gravitational, Inc.

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

package authclient

import (
	"context"
	"github.com/gravitational/roundtrip"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/client/secreport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	accessgraphv1 "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/trace"
	"net"
	"time"
)

// APIClient is aliased here so that it can be embedded in Client.
type APIClient = client.Client

// notImplementedMessage is the message to return for endpoints that are not
// implemented. This is due to how service interfaces are used with Teleport.
const notImplementedMessage = "not implemented: can only be called by auth locally"

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

func (c *Client) GetLicense(ctx context.Context) (string, error) {
	return c.APIClient.GetLicense(ctx)
}

func (c *Client) ListReleases(ctx context.Context) ([]*types.Release, error) {
	return c.APIClient.ListReleases(ctx, &proto.ListReleasesRequest{})
}

func (c *Client) OktaClient() services.Okta {
	return c.APIClient.OktaClient()
}

// SecReportsClient returns a client for security reports.
func (c *Client) SecReportsClient() *secreport.Client {
	return c.APIClient.SecReportsClient()
}

func (c *Client) AccessListClient() services.AccessLists {
	return c.APIClient.AccessListClient()
}

func (c *Client) ExternalCloudAuditClient() services.ExternalCloudAudits {
	return c.APIClient.ExternalCloudAuditClient()
}

func (c *Client) UserLoginStateClient() services.UserLoginStates {
	return c.APIClient.UserLoginStateClient()
}

func (c *Client) AccessGraphClient() accessgraphv1.AccessGraphServiceClient {
	return accessgraphv1.NewAccessGraphServiceClient(c.APIClient.GetConnection())
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

	if err != nil {
		return nil, trace.Wrap(err)
	}

	upserted, err = c.GetUser(ctx, user.GetName(), false)
	return upserted, trace.Wrap(err)
}

// DiscoveryConfigClient returns a client for managing the DiscoveryConfig resource.
func (c *Client) DiscoveryConfigClient() services.DiscoveryConfigs {
	return c.APIClient.DiscoveryConfigClient()
}

// ValidateMFAAuthResponse validates an MFA or passwordless challenge.
// Returns the device used to solve the challenge (if applicable) and the
// username.
func (c *Client) ValidateMFAAuthResponse(ctx context.Context, resp *proto.MFAAuthenticateResponse, user string, passwordless bool) (*types.MFADevice, string, error) {
	return nil, "", trace.NotImplemented(notImplementedMessage)
}
