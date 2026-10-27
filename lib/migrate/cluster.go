// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package migrate

import (
	"context"
	"iter"

	"github.com/gravitational/trace"

	apiclient "github.com/gravitational/teleport/api/client"
	presencepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/presence/v1"
	joiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/itertools/stream"
)

// node is the part of an SSH server resource the tool needs. Inventories are
// compacted to it while paging so a 20k-node fleet never sits in memory as
// protos.
type node struct {
	// ID is the node's host UUID.
	ID string
	// Hostname is the node's hostname.
	Hostname string
	// Scope is the node's scope, empty for an unscoped node.
	Scope string
	// Labels are the node's static and dynamic labels.
	Labels map[string]string
}

// newNode compacts an SSH server resource to the part of it the tool needs.
func newNode(s types.Server) node {
	return node{
		ID:       s.GetName(),
		Hostname: s.GetHostname(),
		Scope:    s.GetScope(),
		Labels:   s.GetAllLabels(),
	}
}

// scopedToken is the part of a scoped token the tool needs.
type scopedToken struct {
	// Name is the token's name.
	Name string
	// Scope is the scope the token belongs to.
	Scope string
	// AssignedScope is the scope an agent joining with the token lands in.
	AssignedScope string
	// Method is the token's join method.
	Method string
	// Roles are the system roles the token may provision, such as Node or App.
	Roles []string
	// UsageMode is the token's usage_mode, such as unlimited or single_use.
	UsageMode string
}

// newScopedToken compacts a scoped token to the part of it the tool needs.
func newScopedToken(token *joiningv1.ScopedToken) scopedToken {
	return scopedToken{
		Name:          token.GetMetadata().GetName(),
		Scope:         token.GetScope(),
		AssignedScope: token.GetSpec().GetAssignedScope(),
		Method:        token.GetSpec().GetJoinMethod(),
		Roles:         token.GetSpec().GetRoles(),
		UsageMode:     token.GetSpec().GetUsageMode(),
	}
}

// clusterAPI is what the migration reads from a cluster. It exists so tests
// can substitute a fake for the cluster's API client.
type clusterAPI interface {
	// ClusterName is the cluster's name.
	ClusterName() string
	// Nodes streams every SSH node in every scope.
	Nodes(ctx context.Context) iter.Seq2[node, error]
}

// targetClusterAPI is clusterAPI plus what the migration reads only from the
// target cluster.
type targetClusterAPI interface {
	clusterAPI
	// ScopedTokens streams the scoped tokens defined in scope, not those of
	// its ancestors or descendants.
	ScopedTokens(ctx context.Context, scope string) iter.Seq2[scopedToken, error]
}

// clusterClient reads one cluster through its API client.
type clusterClient struct {
	clt  *apiclient.Client
	name string
}

// dialCluster dials cluster's proxy with its identity file. The caller closes
// the returned client.
func dialCluster(ctx context.Context, cluster ClusterConfig) (*clusterClient, error) {
	clt, err := apiclient.New(ctx, apiclient.Config{
		Addrs:       []string{cluster.ProxyServer},
		Credentials: []apiclient.Credentials{apiclient.LoadIdentityFile(cluster.IdentityFile)},
	})
	if err != nil {
		return nil, trace.Wrap(err, "dialing %q", cluster.ProxyServer)
	}
	ping, err := clt.Ping(ctx)
	if err != nil {
		return nil, trace.NewAggregate(trace.Wrap(err, "pinging %q", cluster.ProxyServer), clt.Close())
	}
	return &clusterClient{
		clt:  clt,
		name: ping.GetClusterName(),
	}, nil
}

// Close closes the API client.
func (c *clusterClient) Close() error { return trace.Wrap(c.clt.Close()) }

// ClusterName is the cluster's name as its proxy reported it when dialed.
func (c *clusterClient) ClusterName() string { return c.name }

// Nodes streams every SSH node in every scope, a page at a time.
func (c *clusterClient) Nodes(ctx context.Context) iter.Seq2[node, error] {
	servers := c.clt.RangeSSHServers(ctx, presencepb.ListSSHServersRequest_builder{
		ScopeFilter: scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_ALL}.Build(),
	}.Build())
	return stream.Map(servers, newNode)
}

// ScopedTokens streams the scoped tokens defined in scope, a page at a time.
func (c *clusterClient) ScopedTokens(ctx context.Context, scope string) iter.Seq2[scopedToken, error] {
	tokens := clientutils.Resources(ctx, func(ctx context.Context, pageSize int, cursor string) ([]*joiningv1.ScopedToken, string, error) {
		resp, err := c.clt.ListScopedTokens(ctx, joiningv1.ListScopedTokensRequest_builder{
			Limit:       uint32(pageSize),
			Cursor:      cursor,
			ScopeFilter: scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_EXACT, Scope: scope}.Build(),
		}.Build())
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		return resp.GetTokens(), resp.GetCursor(), nil
	})
	return stream.MapErr(stream.Map(tokens, newScopedToken), func(err error) error { return trace.Wrap(err, "listing scoped tokens in %s", scope) })
}
