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
	"time"

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// httpfallback.go holds endpoints that have been converted to gRPC
// but still need http fallback logic in the old client.

// DELETE IN 7.0

// GetRoles returns a list of roles
func (c *Client) GetRoles(ctx context.Context) ([]services.Role, error) {
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
	roles := make([]services.Role, len(items))
	for i, roleBytes := range items {
		role, err := services.UnmarshalRole(roleBytes, services.SkipValidation())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		roles[i] = role
	}
	return roles, nil
}

// UpsertRole creates or updates role
func (c *Client) UpsertRole(ctx context.Context, role services.Role) error {
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
func (c *Client) GetRole(ctx context.Context, name string) (services.Role, error) {
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
	role, err := services.UnmarshalRole(out.Bytes(), services.SkipValidation())
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
func (c *Client) UpsertToken(ctx context.Context, tok services.ProvisionToken) error {
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
func (c *Client) GetTokens(ctx context.Context, opts ...services.MarshalOption) ([]services.ProvisionToken, error) {
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
	var tokens []services.ProvisionTokenV1
	if err := json.Unmarshal(out.Bytes(), &tokens); err != nil {
		return nil, trace.Wrap(err)
	}
	return services.ProvisionTokensFromV1(tokens), nil
}

// GetToken returns provisioning token
func (c *Client) GetToken(ctx context.Context, token string) (services.ProvisionToken, error) {
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
	return services.UnmarshalProvisionToken(out.Bytes(), services.SkipValidation())
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
func (c *Client) UpsertOIDCConnector(ctx context.Context, connector services.OIDCConnector) error {
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
func (c *Client) GetOIDCConnector(ctx context.Context, id string, withSecrets bool) (services.OIDCConnector, error) {
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
	return services.UnmarshalOIDCConnector(out.Bytes(), services.SkipValidation())
}

// GetOIDCConnectors gets OIDC connectors list
func (c *Client) GetOIDCConnectors(ctx context.Context, withSecrets bool) ([]services.OIDCConnector, error) {
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
	connectors := make([]services.OIDCConnector, len(items))
	for i, raw := range items {
		connector, err := services.UnmarshalOIDCConnector(raw, services.SkipValidation())
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
func (c *Client) UpsertSAMLConnector(ctx context.Context, connector services.SAMLConnector) error {
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
func (c *Client) GetSAMLConnector(ctx context.Context, id string, withSecrets bool) (services.SAMLConnector, error) {
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
	return services.UnmarshalSAMLConnector(out.Bytes(), services.SkipValidation())
}

// GetSAMLConnectors gets SAML connectors list
func (c *Client) GetSAMLConnectors(ctx context.Context, withSecrets bool) ([]services.SAMLConnector, error) {
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
	connectors := make([]services.SAMLConnector, len(items))
	for i, raw := range items {
		connector, err := services.UnmarshalSAMLConnector(raw, services.SkipValidation())
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
func (c *Client) UpsertGithubConnector(ctx context.Context, connector services.GithubConnector) error {
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
func (c *Client) GetGithubConnectors(ctx context.Context, withSecrets bool) ([]services.GithubConnector, error) {
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
	connectors := make([]services.GithubConnector, len(items))
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
func (c *Client) GetGithubConnector(ctx context.Context, id string, withSecrets bool) (services.GithubConnector, error) {
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

func (c *Client) GetTrustedCluster(ctx context.Context, name string) (services.TrustedCluster, error) {
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

	trustedCluster, err := services.UnmarshalTrustedCluster(out.Bytes(), services.SkipValidation())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return trustedCluster, nil
}

func (c *Client) GetTrustedClusters(ctx context.Context) ([]services.TrustedCluster, error) {
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
	trustedClusters := make([]services.TrustedCluster, len(items))
	for i, bytes := range items {
		trustedCluster, err := services.UnmarshalTrustedCluster(bytes, services.SkipValidation())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		trustedClusters[i] = trustedCluster
	}

	return trustedClusters, nil
}

// UpsertTrustedCluster creates or updates a trusted cluster.
func (c *Client) UpsertTrustedCluster(ctx context.Context, trustedCluster services.TrustedCluster) (services.TrustedCluster, error) {
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

type nodeClient interface {
	ListNodes(ctx context.Context, req proto.ListNodesRequest) (nodes []types.Server, nextKey string, err error)
	GetNodes(ctx context.Context, namespace string, opts ...services.MarshalOption) ([]types.Server, error)
}

// GetNodesWithLabels is a helper for getting a list of nodes with optional label-based filtering.  This is essentially
// a wrapper around client.GetNodesWithLabels that performs fallback on NotImplemented errors.
func GetNodesWithLabels(ctx context.Context, clt nodeClient, namespace string, labels map[string]string) ([]types.Server, error) {
	nodes, err := client.GetNodesWithLabels(ctx, clt, namespace, labels)
	if err == nil || !trace.IsNotImplemented(err) {
		return nodes, trace.Wrap(err)
	}

	nodes, err = clt.GetNodes(ctx, namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var filtered []types.Server

	// we had to fallback to a method that does not perform server-side filtering,
	// so filter here instead.
	for _, node := range nodes {
		if node.MatchAgainst(labels) {
			filtered = append(filtered, node)
		}
	}

	return filtered, nil
}

// GetNodes returns the list of servers registered in the cluster.
func (c *Client) GetNodes(ctx context.Context, namespace string, opts ...services.MarshalOption) ([]services.Server, error) {
	if resp, err := c.APIClient.GetNodes(ctx, namespace); err != nil {
		if !trace.IsNotImplemented(err) {
			return nil, err
		}
	} else {
		return resp, nil
	}

	cfg, err := services.CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	out, err := c.Get(c.Endpoint("namespaces", namespace, "nodes"), url.Values{
		"skip_validation": []string{fmt.Sprintf("%t", cfg.SkipValidation)},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var items []json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &items); err != nil {
		return nil, trace.Wrap(err)
	}
	re := make([]services.Server, len(items))
	for i, raw := range items {
		s, err := services.UnmarshalServer(
			raw,
			services.KindNode,
			services.AddOptions(opts, services.SkipValidation())...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		re[i] = s
	}

	return re, nil
}

// SearchEvents allows searching for audit events with pagination support.
func (c *Client) SearchEvents(fromUTC, toUTC time.Time, namespace string, eventTypes []string, limit int, order types.EventOrder, startKey string) ([]events.AuditEvent, string, error) {
	events, lastKey, err := c.APIClient.SearchEvents(context.TODO(), fromUTC, toUTC, namespace, eventTypes, limit, order, startKey)
	if err != nil {
		if trace.IsNotImplemented(err) {
			log.WithError(err).Debug("Attempted to call SearchEvents over gRPC but received a notImplemented error, falling back to legacy API.")
			return c.searchEventsFallback(context.TODO(), fromUTC, toUTC, namespace, eventTypes, limit, startKey)
		}

		return nil, "", trace.Wrap(err)
	}

	return events, lastKey, nil
}

// SearchSessionEvents returns session related events to find completed sessions.
func (c *Client) SearchSessionEvents(fromUTC time.Time, toUTC time.Time, limit int, order types.EventOrder, startKey string) ([]events.AuditEvent, string, error) {
	events, lastKey, err := c.APIClient.SearchSessionEvents(context.TODO(), fromUTC, toUTC, limit, order, startKey)
	if err != nil {
		if trace.IsNotImplemented(err) {
			log.WithError(err).Debug("Attempted to call SearchSessionEvents over gRPC but received a notImplemented error, falling back to legacy API.")
			return c.searchSessionEventsFallback(context.TODO(), fromUTC, toUTC, limit, startKey)
		}

		return nil, "", trace.Wrap(err)
	}

	return events, lastKey, nil
}

// searchEventsFallback is a fallback version of SearchEvents that queries the deprecated
// HTTP API if the auth server is on a version < 6.2.
//
// DELETE IN 7.0
func (c *Client) searchEventsFallback(ctx context.Context, fromUTC, toUTC time.Time, namespace string, eventTypes []string, limit int, startKey string) ([]events.AuditEvent, string, error) {
	if startKey != "" {
		return nil, "", trace.BadParameter(`HTTP fallback API for SearchEvents does not support "startKey"`)
	}

	query := url.Values{
		"to":    []string{toUTC.Format(time.RFC3339)},
		"from":  []string{toUTC.Format(time.RFC3339)},
		"limit": []string{fmt.Sprintf("%v", limit)},
	}

	for _, eventType := range eventTypes {
		query.Add(events.EventType, eventType)
	}

	response, err := c.Get(c.Endpoint("events"), query)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	eventArr, err := c.unmarshalEventsResponse(response)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	return eventArr, "", nil
}

// searchSessionEventsFallback is a fallback version of SearchSessionEvents that queries the deprecated
// HTTP API if the auth server is on a version < 6.2.
//
// DELETE IN 7.0
func (c *Client) searchSessionEventsFallback(ctx context.Context, fromUTC time.Time, toUTC time.Time, limit int, startKey string) ([]events.AuditEvent, string, error) {
	if startKey != "" {
		return nil, "", trace.BadParameter(`HTTP fallback API for SearchSessionEvents does not support "startKey"`)
	}

	query := url.Values{
		"to":    []string{toUTC.Format(time.RFC3339)},
		"from":  []string{toUTC.Format(time.RFC3339)},
		"limit": []string{fmt.Sprintf("%v", limit)},
	}

	response, err := c.Get(c.Endpoint("events", "session"), query)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	eventArr, err := c.unmarshalEventsResponse(response)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	return eventArr, "", nil
}

// unmarshalEventsResponse extracts weakly typed JSON-style audit events from a HTTP body
// and converts them into a slice of strongly typed gRPC-style audit events.
func (c *Client) unmarshalEventsResponse(response *roundtrip.Response) ([]events.AuditEvent, error) {
	dynEventArr := make([]events.EventFields, 0)
	if err := json.Unmarshal(response.Bytes(), &dynEventArr); err != nil {
		return nil, trace.Wrap(err)
	}

	eventArr, err := events.FromEventFieldsSlice(dynEventArr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return eventArr, nil
}

// GetAuthPreference gets cluster auth preference.
func (c *Client) GetAuthPreference() (types.AuthPreference, error) {
	if resp, err := c.APIClient.GetAuthPreference(); err != nil {
		if !trace.IsNotImplemented(err) {
			return nil, trace.Wrap(err)
		}
	} else {
		return resp, nil
	}
	out, err := c.Get(c.Endpoint("authentication", "preference"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cap, err := services.UnmarshalAuthPreference(out.Bytes())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return cap, nil
}

// SetAuthPreference sets cluster auth preference.
func (c *Client) SetAuthPreference(cap types.AuthPreference) error {
	if err := c.APIClient.SetAuthPreference(cap); err != nil {
		if !trace.IsNotImplemented(err) {
			return trace.Wrap(err)
		}
	} else {
		return nil
	}
	data, err := services.MarshalAuthPreference(cap)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = c.PostJSON(c.Endpoint("authentication", "preference"), &setClusterAuthPreferenceReq{ClusterAuthPreference: data})
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}
