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

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
)

// httpfallback.go holds endpoints that have been converted to gRPC
// but still need http fallback logic in the old client.

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
	_, err = c.PostJSON(ctx, c.Endpoint("roles"), &upsertRoleRawReq{Role: data})
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
	out, err := c.Get(ctx, c.Endpoint("roles", name), url.Values{})
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
	_, err := c.Delete(ctx, c.Endpoint("roles", name))
	return trace.Wrap(err)
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

	out, err := c.Get(ctx, c.Endpoint("tokens"), url.Values{})
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

	out, err := c.Get(ctx, c.Endpoint("tokens", token), url.Values{})
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

	_, err := c.Delete(ctx, c.Endpoint("tokens", token))
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
	_, err = c.PostJSON(ctx, c.Endpoint("oidc", "connectors"), &upsertOIDCConnectorRawReq{
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
	out, err := c.Get(ctx, c.Endpoint("oidc", "connectors", id),
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

	out, err := c.Get(ctx, c.Endpoint("oidc", "connectors"),
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
	_, err := c.Delete(ctx, c.Endpoint("oidc", "connectors", connectorID))
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
	_, err = c.PutJSON(ctx, c.Endpoint("saml", "connectors"), &upsertSAMLConnectorRawReq{
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
	out, err := c.Get(ctx, c.Endpoint("saml", "connectors", id),
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

	out, err := c.Get(ctx, c.Endpoint("saml", "connectors"),
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
	_, err := c.Delete(ctx, c.Endpoint("saml", "connectors", connectorID))
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
	_, err = c.PutJSON(ctx, c.Endpoint("github", "connectors"), &upsertGithubConnectorRawReq{
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

	out, err := c.Get(ctx, c.Endpoint("github", "connectors"), url.Values{
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

	out, err := c.Get(ctx, c.Endpoint("github", "connectors", id), url.Values{
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

	_, err := c.Delete(ctx, c.Endpoint("github", "connectors", id))
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

	out, err := c.Get(ctx, c.Endpoint("trustedclusters", name), url.Values{})
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

	out, err := c.Get(ctx, c.Endpoint("trustedclusters"), url.Values{})
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
	out, err := c.PostJSON(ctx, c.Endpoint("trustedclusters"), &upsertTrustedClusterReq{
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

	_, err := c.Delete(ctx, c.Endpoint("trustedclusters", name))
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

	_, err := c.Delete(ctx, c.Endpoint("namespaces", namespace, "nodes"))
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

	_, err := c.Delete(ctx, c.Endpoint("namespaces", namespace, "nodes", name))
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
//
// DELETE IN 11.0.0, this function is only called by lib/client/client.go (*ProxyClient).FindServersByLabels
// which is also marked for deletion (replaced by FindNodesByFilters).
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
func (c *Client) GetNodes(ctx context.Context, namespace string, opts ...services.MarshalOption) ([]types.Server, error) {
	if resp, err := c.APIClient.GetNodes(ctx, namespace); err != nil {
		if !trace.IsNotImplemented(err) {
			return nil, trace.Wrap(err)
		}
	} else {
		return resp, nil
	}

	out, err := c.Get(ctx, c.Endpoint("namespaces", namespace, "nodes"), url.Values{})
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
