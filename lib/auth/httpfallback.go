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
	"net/url"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
)

// httpfallback.go holds endpoints that have been converted to gRPC
// but still need http fallback logic in the old client.

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
//
// DELETE IN 11.0.0, replaced by GetResourcesWithFilters
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
