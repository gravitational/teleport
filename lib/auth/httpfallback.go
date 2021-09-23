/*
Copyright 2021 Gravitational, Inc.

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
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/u2f"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// httpfallback.go holds endpoints that have been converted to gRPC
// but still need http fallback logic in the old client.

// DELETE IN 7.0

// GetRoles returns a list of roles
func (c *Client) GetRoles(ctx context.Context) ([]types.Role, error) {
	if resp, err := c.APIClient.GetRoles(ctx); err != nil {
		if !trace.IsNotImplemented(err) {
			return nil, trace.Wrap(err)
		}
	} else {
		return resp, nil
	}

	out, err := c.Get(c.Endpoint("roles"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var items []json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &items); err != nil {
		return nil, trace.Wrap(err)
	}
	roles := make([]types.Role, len(items))
	for i, roleBytes := range items {
		role, err := services.UnmarshalRole(roleBytes)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		roles[i] = role
	}
	return roles, nil
}

// UpsertRole creates or updates role
func (c *Client) UpsertRole(ctx context.Context, role types.Role) error {
	if err := c.APIClient.UpsertRole(ctx, role); err != nil {
		if !trace.IsNotImplemented(err) {
			return trace.Wrap(err)
		}
	} else {
		return nil
	}

	data, err := services.MarshalRole(role)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = c.PostJSON(c.Endpoint("roles"), &upsertRoleRawReq{Role: data})
	return trace.Wrap(err)
}

// GetRole returns role by name
func (c *Client) GetRole(ctx context.Context, name string) (types.Role, error) {
	if resp, err := c.APIClient.GetRole(ctx, name); err != nil {
		if !trace.IsNotImplemented(err) {
			return nil, trace.Wrap(err)
		}
	} else {
		return resp, nil
	}

	if name == "" {
		return nil, trace.BadParameter("missing name")
	}
	out, err := c.Get(c.Endpoint("roles", name), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	role, err := services.UnmarshalRole(out.Bytes())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return role, nil
}

// DeleteRole deletes role by name
func (c *Client) DeleteRole(ctx context.Context, name string) error {
	if err := c.APIClient.DeleteRole(ctx, name); err != nil {
		if !trace.IsNotImplemented(err) {
			return trace.Wrap(err)
		}
	} else {
		return nil
	}

	if name == "" {
		return trace.BadParameter("missing name")
	}
	_, err := c.Delete(c.Endpoint("roles", name))
	return trace.Wrap(err)
}

// DELETE IN 8.0

// UpsertToken adds provisioning tokens for the auth server
func (c *Client) UpsertToken(ctx context.Context, tok types.ProvisionToken) error {
	if err := c.APIClient.UpsertToken(ctx, tok); err != nil {
		if !trace.IsNotImplemented(err) {
			return trace.Wrap(err)
		}
	} else {
		return nil
	}

	_, err := c.PostJSON(c.Endpoint("tokens"), GenerateTokenRequest{
		Token: tok.GetName(),
		Roles: tok.GetRoles(),
		TTL:   backend.TTL(clockwork.NewRealClock(), tok.Expiry()),
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetTokens returns a list of active invitation tokens for nodes and users
func (c *Client) GetTokens(ctx context.Context, opts ...services.MarshalOption) ([]types.ProvisionToken, error) {
	if resp, err := c.APIClient.GetTokens(ctx); err != nil {
		if !trace.IsNotImplemented(err) {
			return nil, trace.Wrap(err)
		}
	} else {
		return resp, nil
	}

	out, err := c.Get(c.Endpoint("tokens"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var tokens []types.ProvisionTokenV1
	if err := json.Unmarshal(out.Bytes(), &tokens); err != nil {
		return nil, trace.Wrap(err)
	}
	return types.ProvisionTokensFromV1(tokens), nil
}

// GetToken returns provisioning token
func (c *Client) GetToken(ctx context.Context, token string) (types.ProvisionToken, error) {
	if resp, err := c.APIClient.GetToken(ctx, token); err != nil {
		if !trace.IsNotImplemented(err) {
			return nil, trace.Wrap(err)
		}
	} else {
		return resp, nil
	}

	out, err := c.Get(c.Endpoint("tokens", token), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return services.UnmarshalProvisionToken(out.Bytes())
}

// DeleteToken deletes a given provisioning token on the auth server (CA). It
// could be a reset password token or a machine token
func (c *Client) DeleteToken(ctx context.Context, token string) error {
	if err := c.APIClient.DeleteToken(ctx, token); err != nil {
		if !trace.IsNotImplemented(err) {
			return trace.Wrap(err)
		}
	} else {
		return nil
	}

	_, err := c.Delete(c.Endpoint("tokens", token))
	return trace.Wrap(err)
}

// UpsertOIDCConnector updates or creates OIDC connector
func (c *Client) UpsertOIDCConnector(ctx context.Context, connector types.OIDCConnector) error {
	if err := c.APIClient.UpsertOIDCConnector(ctx, connector); err != nil {
		if !trace.IsNotImplemented(err) {
			return trace.Wrap(err)
		}
	} else {
		return nil
	}

	data, err := services.MarshalOIDCConnector(connector)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = c.PostJSON(c.Endpoint("oidc", "connectors"), &upsertOIDCConnectorRawReq{
		Connector: data,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetOIDCConnector returns OIDC connector information by id
func (c *Client) GetOIDCConnector(ctx context.Context, id string, withSecrets bool) (types.OIDCConnector, error) {
	if resp, err := c.APIClient.GetOIDCConnector(ctx, id, withSecrets); err != nil {
		if !trace.IsNotImplemented(err) {
			return nil, trace.Wrap(err)
		}
	} else {
		return resp, nil
	}

	if id == "" {
		return nil, trace.BadParameter("missing connector id")
	}
	out, err := c.Get(c.Endpoint("oidc", "connectors", id),
		url.Values{"with_secrets": []string{fmt.Sprintf("%t", withSecrets)}})
	if err != nil {
		return nil, err
	}
	return services.UnmarshalOIDCConnector(out.Bytes())
}

// GetOIDCConnectors gets OIDC connectors list
func (c *Client) GetOIDCConnectors(ctx context.Context, withSecrets bool) ([]types.OIDCConnector, error) {
	if resp, err := c.APIClient.GetOIDCConnectors(ctx, withSecrets); err != nil {
		if !trace.IsNotImplemented(err) {
			return nil, trace.Wrap(err)
		}
	} else {
		return resp, nil
	}

	out, err := c.Get(c.Endpoint("oidc", "connectors"),
		url.Values{"with_secrets": []string{fmt.Sprintf("%t", withSecrets)}})
	if err != nil {
		return nil, err
	}
	var items []json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &items); err != nil {
		return nil, trace.Wrap(err)
	}
	connectors := make([]types.OIDCConnector, len(items))
	for i, raw := range items {
		connector, err := services.UnmarshalOIDCConnector(raw)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		connectors[i] = connector
	}
	return connectors, nil
}

// DeleteOIDCConnector deletes OIDC connector by ID
func (c *Client) DeleteOIDCConnector(ctx context.Context, connectorID string) error {
	if err := c.APIClient.DeleteOIDCConnector(ctx, connectorID); err != nil {
		if !trace.IsNotImplemented(err) {
			return trace.Wrap(err)
		}
	} else {
		return nil
	}

	if connectorID == "" {
		return trace.BadParameter("missing connector id")
	}
	_, err := c.Delete(c.Endpoint("oidc", "connectors", connectorID))
	return trace.Wrap(err)
}

// UpsertSAMLConnector updates or creates SAML connector
func (c *Client) UpsertSAMLConnector(ctx context.Context, connector types.SAMLConnector) error {
	if err := c.APIClient.UpsertSAMLConnector(ctx, connector); err != nil {
		if !trace.IsNotImplemented(err) {
			return trace.Wrap(err)
		}
	} else {
		return nil
	}

	data, err := services.MarshalSAMLConnector(connector)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = c.PutJSON(c.Endpoint("saml", "connectors"), &upsertSAMLConnectorRawReq{
		Connector: data,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetSAMLConnector returns SAML connector information by id
func (c *Client) GetSAMLConnector(ctx context.Context, id string, withSecrets bool) (types.SAMLConnector, error) {
	if resp, err := c.APIClient.GetSAMLConnector(ctx, id, withSecrets); err != nil {
		if !trace.IsNotImplemented(err) {
			return nil, trace.Wrap(err)
		}
	} else {
		return resp, nil
	}

	if id == "" {
		return nil, trace.BadParameter("missing connector id")
	}
	out, err := c.Get(c.Endpoint("saml", "connectors", id),
		url.Values{"with_secrets": []string{fmt.Sprintf("%t", withSecrets)}})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return services.UnmarshalSAMLConnector(out.Bytes())
}

// GetSAMLConnectors gets SAML connectors list
func (c *Client) GetSAMLConnectors(ctx context.Context, withSecrets bool) ([]types.SAMLConnector, error) {
	if resp, err := c.APIClient.GetSAMLConnectors(ctx, withSecrets); err != nil {
		if !trace.IsNotImplemented(err) {
			return nil, trace.Wrap(err)
		}
	} else {
		return resp, nil
	}

	out, err := c.Get(c.Endpoint("saml", "connectors"),
		url.Values{"with_secrets": []string{fmt.Sprintf("%t", withSecrets)}})
	if err != nil {
		return nil, err
	}
	var items []json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &items); err != nil {
		return nil, trace.Wrap(err)
	}
	connectors := make([]types.SAMLConnector, len(items))
	for i, raw := range items {
		connector, err := services.UnmarshalSAMLConnector(raw)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		connectors[i] = connector
	}
	return connectors, nil
}

// DeleteSAMLConnector deletes SAML connector by ID
func (c *Client) DeleteSAMLConnector(ctx context.Context, connectorID string) error {
	if err := c.APIClient.DeleteSAMLConnector(ctx, connectorID); err != nil {
		if !trace.IsNotImplemented(err) {
			return trace.Wrap(err)
		}
	} else {
		return nil
	}

	if connectorID == "" {
		return trace.BadParameter("missing connector id")
	}
	_, err := c.Delete(c.Endpoint("saml", "connectors", connectorID))
	return trace.Wrap(err)
}

// UpsertGithubConnector creates or updates a Github connector
func (c *Client) UpsertGithubConnector(ctx context.Context, connector types.GithubConnector) error {
	if err := c.APIClient.UpsertGithubConnector(ctx, connector); err != nil {
		if !trace.IsNotImplemented(err) {
			return trace.Wrap(err)
		}
	} else {
		return nil
	}

	bytes, err := services.MarshalGithubConnector(connector)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = c.PutJSON(c.Endpoint("github", "connectors"), &upsertGithubConnectorRawReq{
		Connector: bytes,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetGithubConnectors returns all configured Github connectors
func (c *Client) GetGithubConnectors(ctx context.Context, withSecrets bool) ([]types.GithubConnector, error) {
	if resp, err := c.APIClient.GetGithubConnectors(ctx, withSecrets); err != nil {
		if !trace.IsNotImplemented(err) {
			return nil, trace.Wrap(err)
		}
	} else {
		return resp, nil
	}

	out, err := c.Get(c.Endpoint("github", "connectors"), url.Values{
		"with_secrets": []string{strconv.FormatBool(withSecrets)},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var items []json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &items); err != nil {
		return nil, trace.Wrap(err)
	}
	connectors := make([]types.GithubConnector, len(items))
	for i, raw := range items {
		connector, err := services.UnmarshalGithubConnector(raw)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		connectors[i] = connector
	}
	return connectors, nil
}

// GetGithubConnector returns the specified Github connector
func (c *Client) GetGithubConnector(ctx context.Context, id string, withSecrets bool) (types.GithubConnector, error) {
	if resp, err := c.APIClient.GetGithubConnector(ctx, id, withSecrets); err != nil {
		if !trace.IsNotImplemented(err) {
			return nil, trace.Wrap(err)
		}
	} else {
		return resp, nil
	}

	out, err := c.Get(c.Endpoint("github", "connectors", id), url.Values{
		"with_secrets": []string{strconv.FormatBool(withSecrets)},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return services.UnmarshalGithubConnector(out.Bytes())
}

// DeleteGithubConnector deletes the specified Github connector
func (c *Client) DeleteGithubConnector(ctx context.Context, id string) error {
	if err := c.APIClient.DeleteGithubConnector(ctx, id); err != nil {
		if !trace.IsNotImplemented(err) {
			return trace.Wrap(err)
		}
	} else {
		return nil
	}

	_, err := c.Delete(c.Endpoint("github", "connectors", id))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (c *Client) GetTrustedCluster(ctx context.Context, name string) (types.TrustedCluster, error) {
	if resp, err := c.APIClient.GetTrustedCluster(ctx, name); err != nil {
		if !trace.IsNotImplemented(err) {
			return nil, trace.Wrap(err)
		}
	} else {
		return resp, nil
	}

	out, err := c.Get(c.Endpoint("trustedclusters", name), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	trustedCluster, err := services.UnmarshalTrustedCluster(out.Bytes())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return trustedCluster, nil
}

func (c *Client) GetTrustedClusters(ctx context.Context) ([]types.TrustedCluster, error) {
	if resp, err := c.APIClient.GetTrustedClusters(ctx); err != nil {
		if !trace.IsNotImplemented(err) {
			return nil, trace.Wrap(err)
		}
	} else {
		return resp, nil
	}

	out, err := c.Get(c.Endpoint("trustedclusters"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var items []json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &items); err != nil {
		return nil, trace.Wrap(err)
	}
	trustedClusters := make([]types.TrustedCluster, len(items))
	for i, bytes := range items {
		trustedCluster, err := services.UnmarshalTrustedCluster(bytes)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		trustedClusters[i] = trustedCluster
	}

	return trustedClusters, nil
}

// UpsertTrustedCluster creates or updates a trusted cluster.
func (c *Client) UpsertTrustedCluster(ctx context.Context, trustedCluster types.TrustedCluster) (types.TrustedCluster, error) {
	if resp, err := c.APIClient.UpsertTrustedCluster(ctx, trustedCluster); err != nil {
		if !trace.IsNotImplemented(err) {
			return nil, trace.Wrap(err)
		}
	} else {
		return resp, nil
	}

	trustedClusterBytes, err := services.MarshalTrustedCluster(trustedCluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out, err := c.PostJSON(c.Endpoint("trustedclusters"), &upsertTrustedClusterReq{
		TrustedCluster: trustedClusterBytes,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return services.UnmarshalTrustedCluster(out.Bytes())
}

// DeleteTrustedCluster deletes a trusted cluster by name.
func (c *Client) DeleteTrustedCluster(ctx context.Context, name string) error {
	if err := c.APIClient.DeleteTrustedCluster(ctx, name); err != nil {
		if !trace.IsNotImplemented(err) {
			return trace.Wrap(err)
		}
	} else {
		return nil
	}

	_, err := c.Delete(c.Endpoint("trustedclusters", name))
	return trace.Wrap(err)
}

// DeleteAllNodes deletes all nodes in a given namespace
func (c *Client) DeleteAllNodes(ctx context.Context, namespace string) error {
	if err := c.APIClient.DeleteAllNodes(ctx, namespace); err != nil {
		if !trace.IsNotImplemented(err) {
			return trace.Wrap(err)
		}
	} else {
		return nil
	}

	_, err := c.Delete(c.Endpoint("namespaces", namespace, "nodes"))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteNode deletes node in the namespace by name
func (c *Client) DeleteNode(ctx context.Context, namespace string, name string) error {
	if err := c.APIClient.DeleteNode(ctx, namespace, name); err != nil {
		if !trace.IsNotImplemented(err) {
			return trace.Wrap(err)
		}
	} else {
		return nil
	}

	_, err := c.Delete(c.Endpoint("namespaces", namespace, "nodes", name))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetNodes returns the list of servers registered in the cluster.
func (c *Client) GetNodes(ctx context.Context, namespace string, opts ...services.MarshalOption) ([]types.Server, error) {
	if resp, err := c.APIClient.GetNodes(ctx, namespace); err != nil {
		if !trace.IsNotImplemented(err) {
			return nil, trace.Wrap(err)
		}
	} else {
		return resp, nil
	}

	out, err := c.Get(c.Endpoint("namespaces", namespace, "nodes"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var items []json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &items); err != nil {
		return nil, trace.Wrap(err)
	}
	re := make([]types.Server, len(items))
	for i, raw := range items {
		s, err := services.UnmarshalServer(
			raw,
			types.KindNode,
			opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		re[i] = s
	}

	return re, nil
}

// DELETE IN 9.0.0, to remove fallback and grpc call is already defined in api/client/client.go
//
// ChangeUserAuthentication changes user password with a user reset token and starts a web session.
// Returns recovery tokens for cloud users with second factors turned on.
func (c *Client) ChangeUserAuthentication(ctx context.Context, req *proto.ChangeUserAuthenticationRequest) (*proto.ChangeUserAuthenticationResponse, error) {
	switch resp, err := c.APIClient.ChangeUserAuthentication(ctx, req); {
	// ChangeUserAuthentication available
	case err == nil:
		return resp, nil
	// ChangeUserAuthentication errored
	case !trace.IsNotImplemented(err):
		return nil, trace.Wrap(err)
	}

	// Convert request back to fallback compatible object.
	httpReq := ChangePasswordWithTokenRequest{
		SecondFactorToken: req.GetNewMFARegisterResponse().GetTOTP().GetCode(),
		TokenID:           req.GetTokenID(),
		Password:          req.GetNewPassword(),
	}

	if req.NewMFARegisterResponse.GetU2F() != nil {
		httpReq.U2FRegisterResponse = &u2f.RegisterChallengeResponse{
			RegistrationData: req.NewMFARegisterResponse.GetU2F().GetRegistrationData(),
			ClientData:       req.NewMFARegisterResponse.GetU2F().GetClientData(),
		}
	}

	out, err := c.PostJSON(c.Endpoint("web", "password", "token"), httpReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	webSession, err := services.UnmarshalWebSession(out.Bytes())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sess, ok := webSession.(*types.WebSessionV2)
	if !ok {
		return nil, trace.BadParameter("unexpected WebSessionV2 type %T", sess)
	}

	return &proto.ChangeUserAuthenticationResponse{
		WebSession: sess,
	}, nil
}

// GenerateHostCerts generates new host certificates (signed by the host CA).
// DELETE IN 9.0.0 (zmb3)
func (c *Client) GenerateHostCerts(ctx context.Context, req *proto.HostCertsRequest) (*proto.Certs, error) {
	switch certs, err := c.APIClient.GenerateHostCerts(ctx, req); {
	case err == nil: // GRPC version is available and succeeded
		return certs, nil
	case !trace.IsNotImplemented(err): // GRPC version available but failed
		return nil, trace.Wrap(err)
	}

	// fallback to legacy JSON API
	out, err := c.PostJSON(c.Endpoint("server", "credentials"), legacyHostCertsRequest{
		HostID:               req.HostID,
		NodeName:             req.NodeName,
		Roles:                types.SystemRoles{req.Role}, // old API requires a list of roles
		AdditionalPrincipals: req.AdditionalPrincipals,
		DNSNames:             req.DNSNames,
		PublicTLSKey:         req.PublicTLSKey,
		PublicSSHKey:         req.PublicSSHKey,
		RemoteAddr:           req.RemoteAddr,
		Rotation:             req.Rotation,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var certs proto.Certs
	if err := json.Unmarshal(out.Bytes(), &certs); err != nil {
		return nil, trace.Wrap(err)
	}

	return &certs, nil
}

// DELETE IN 9.0.0, to remove fallback and grpc call is already defined in api/client/client.go
//
// CreateAuthenticateChallenge creates and returns MFA challenges for a users registered MFA devices.
func (c *Client) CreateAuthenticateChallenge(ctx context.Context, req *proto.CreateAuthenticateChallengeRequest) (*proto.MFAAuthenticateChallenge, error) {
	switch resp, err := c.APIClient.CreateAuthenticateChallenge(ctx, req); {
	case err == nil:
		return resp, nil
	case !trace.IsNotImplemented(err):
		return nil, trace.Wrap(err)
	}

	// HTTP fallback for auth version <7.x
	out, err := c.PostJSON(
		c.Endpoint("u2f", "users", req.GetUserCredentials().GetUsername(), "sign"),
		signInReq{
			Password: string(req.GetUserCredentials().GetPassword()),
		},
	)

	if err != nil {
		return nil, trace.Wrap(err)
	}

	var jsonChal *MFAAuthenticateChallenge
	if err := json.Unmarshal(out.Bytes(), &jsonChal); err != nil {
		return nil, err
	}

	// Convert JSON back to proto.
	// Webauthn is not handled here b/c the feature
	// does not exist for auth version <7.x
	protoChal := &proto.MFAAuthenticateChallenge{}

	if jsonChal.TOTPChallenge {
		protoChal.TOTP = &proto.TOTPChallenge{}
	}

	for _, u2fChal := range jsonChal.U2FChallenges {
		protoChal.U2F = append(protoChal.U2F, &proto.U2FChallenge{
			KeyHandle: u2fChal.KeyHandle,
			Challenge: u2fChal.Challenge,
			AppID:     u2fChal.AppID,
		})
	}

	return protoChal, nil
}

// DELETE IN 9.0.0, to remove fallback and grpc call is already defined in api/client/client.go
//
// CreateRegisterChallenge creates and returns MFA register challenge for a new MFA device.
func (c *Client) CreateRegisterChallenge(ctx context.Context, req *proto.CreateRegisterChallengeRequest) (*proto.MFARegisterChallenge, error) {
	switch resp, err := c.APIClient.CreateRegisterChallenge(ctx, req); {
	case err == nil:
		return resp, nil
	case !trace.IsNotImplemented(err):
		return nil, trace.Wrap(err)
	}

	// Fallback for auth version <7.x
	// Does not handle webauthn since this feature will not exist <7.x.
	switch req.DeviceType {
	case proto.DeviceType_DEVICE_TYPE_TOTP:
		resp, err := c.APIClient.RotateUserTokenSecrets(ctx, req.GetTokenID())
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// Only the QRCode is returned b/c that was the only value the caller was using/needed.
		return &proto.MFARegisterChallenge{Request: &proto.MFARegisterChallenge_TOTP{
			TOTP: &proto.TOTPRegisterChallenge{QRCode: resp.GetQRCode()},
		}}, nil

	case proto.DeviceType_DEVICE_TYPE_U2F:
		out, err := c.Get(c.Endpoint("u2f", "signuptokens", req.GetTokenID()), url.Values{})
		if err != nil {
			return nil, err
		}
		var u2fRegReq u2f.RegisterChallenge
		if err := json.Unmarshal(out.Bytes(), &u2fRegReq); err != nil {
			return nil, err
		}

		return &proto.MFARegisterChallenge{Request: &proto.MFARegisterChallenge_U2F{
			U2F: &proto.U2FRegisterChallenge{
				Challenge: u2fRegReq.Challenge,
				AppID:     u2fRegReq.AppID,
				Version:   u2fRegReq.Version,
			},
		}}, nil

	default:
		return nil, trace.BadParameter("MFA device type %v unsupported", req.GetDeviceType().String())
	}
}
