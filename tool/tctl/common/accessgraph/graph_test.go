/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package accessgraph

import (
	"sort"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	models "github.com/gravitational/teleport/lib/accessgraph/apiclient/models/graph"
)

// graphBuilder is a small DSL for assembling fixture graphs in tests. It
// hands out stable UUIDs by name so assertions can be written against
// human-readable identifiers.
type graphBuilder struct {
	t     *testing.T
	ids   map[string]uuid.UUID
	nodes []models.Node
	edges []models.Edge
}

func newGraphBuilder(t *testing.T) *graphBuilder {
	t.Helper()
	return &graphBuilder{t: t, ids: map[string]uuid.UUID{}}
}

func (b *graphBuilder) idFor(name string) uuid.UUID {
	if id, ok := b.ids[name]; ok {
		return id
	}
	// Deterministic, name-keyed UUIDs let us re-derive an id mid-test
	// (e.g. when checking the result set) without holding a reference.
	id := uuid.NewSHA1(uuid.NameSpaceOID, []byte(name))
	b.ids[name] = id
	return id
}

// node adds a node with the given kind and name. extra is an optional
// configurator used for action-type / temporary-group flags.
type nodeOpt func(*models.Node)

func (b *graphBuilder) node(name string, kind NodeKind, opts ...nodeOpt) {
	b.t.Helper()
	n := models.Node{
		Id:   b.idFor(name),
		Kind: models.NodeKind(kind),
		Name: name,
	}
	for _, opt := range opts {
		opt(&n)
	}
	b.nodes = append(b.nodes, n)
}

func withActionType(actionType string) nodeOpt {
	return func(n *models.Node) {
		require.NoError(nil, n.Properties.FromActionProperties(models.ActionProperties{
			Type: actionType,
		}))
	}
}

func withActionSubKind(subKind string) nodeOpt {
	return func(n *models.Node) { n.SubKind = subKind }
}

func withTemporary(temp bool) nodeOpt {
	return func(n *models.Node) {
		require.NoError(nil, n.Properties.FromIdentityGroupProperties(models.IdentityGroupProperties{
			Temporary: &temp,
		}))
	}
}

// edge adds an edge from→to with the given EdgeType.
func (b *graphBuilder) edge(from, to, edgeType string) {
	b.t.Helper()
	b.edges = append(b.edges, models.Edge{
		Id:       uuid.New(),
		From:     b.idFor(from),
		To:       b.idFor(to),
		EdgeType: edgeType,
	})
}

func (b *graphBuilder) build() *traversalGraph {
	return newTraversalGraph(&b.nodes, &b.edges)
}

// nodeNames returns the names of nodes in alphabetical order — handy for
// stable comparisons since traversal order is not guaranteed.
func nodeNames(nodes []*models.Node) []string {
	out := make([]string, len(nodes))
	for i, n := range nodes {
		out[i] = n.Name
	}
	sort.Strings(out)
	return out
}

// startNode looks up the node the test seeded by name.
func (b *graphBuilder) startNode(name string) models.Node {
	id := b.idFor(name)
	for _, n := range b.nodes {
		if n.Id == id {
			return n
		}
	}
	b.t.Fatalf("startNode: %q not added to builder", name)
	return models.Node{}
}

func TestNewTraversalGraph_NilInputs(t *testing.T) {
	t.Parallel()
	g := newTraversalGraph(nil, nil)
	require.NotNil(t, g)
	require.Empty(t, g.nodes)
	require.Empty(t, g.outgoingEdges)
	require.Empty(t, g.incomingEdges)
}

func TestNewTraversalGraph_DanglingEdgesAreSkipped(t *testing.T) {
	t.Parallel()
	// An edge whose endpoints aren't in the node set must not be
	// indexed — otherwise traversal would dereference a nil target.
	b := newGraphBuilder(t)
	b.node("A", NodeKindIdentity)
	b.edge("A", "ghost", "permits")
	g := b.build()
	require.Empty(t, g.outgoingEdges, "edge to unknown node must not be indexed")
	require.Empty(t, g.incomingEdges)
}

// TestGetIdentityNodesWithAccess_Allowed walks identity → ALLOWED-action →
// resource and confirms the identity is returned.
func TestGetIdentityNodesWithAccess_Allowed(t *testing.T) {
	t.Parallel()
	b := newGraphBuilder(t)
	b.node("alice", NodeKindIdentity)
	b.node("read", NodeKindAction, withActionType(ActionAllowed))
	b.node("db", NodeKindResource)
	b.edge("alice", "read", "performs")
	b.edge("read", "db", "on")
	g := b.build()

	got := g.getIdentityNodesWithAccess(b.startNode("db"))
	require.Equal(t, []string{"alice"}, nodeNames(got))
}

// TestGetIdentityNodesWithAccess_DeniedExcludes ensures that an identity
// reachable via a DENIED action on the same resource is filtered out.
func TestGetIdentityNodesWithAccess_DeniedExcludes(t *testing.T) {
	t.Parallel()
	b := newGraphBuilder(t)
	b.node("alice", NodeKindIdentity)
	b.node("deny-read", NodeKindAction, withActionType(ActionDenied))
	b.node("db", NodeKindResource)
	b.edge("alice", "deny-read", "performs")
	b.edge("deny-read", "db", "on")
	g := b.build()

	got := g.getIdentityNodesWithAccess(b.startNode("db"))
	require.Empty(t, nodeNames(got), "identity reachable only via DENIED must be filtered")
}

// TestGetIdentityNodesWithAccess_OwnerOfSkipped verifies that owner_of
// edges aren't traversed — they represent ownership, not access.
func TestGetIdentityNodesWithAccess_OwnerOfSkipped(t *testing.T) {
	t.Parallel()
	b := newGraphBuilder(t)
	b.node("alice", NodeKindIdentity)
	b.node("db", NodeKindResource)
	// Only an owner_of edge — no access action.
	b.edge("alice", "db", string(EdgeKindOwnerOf))
	g := b.build()

	got := g.getIdentityNodesWithAccess(b.startNode("db"))
	require.Empty(t, nodeNames(got), "owner_of must not grant access")
}

// TestGetIdentityNodesWithAccess_TemporaryGroupBlocks confirms that an
// identity whose only path runs through a temporary identity group (the
// signal AG uses for request-based access) is filtered out.
func TestGetIdentityNodesWithAccess_TemporaryGroupBlocks(t *testing.T) {
	t.Parallel()
	b := newGraphBuilder(t)
	b.node("alice", NodeKindIdentity)
	b.node("temp-team", NodeKindIdentityGroup, withTemporary(true))
	b.node("read", NodeKindAction, withActionType(ActionAllowed))
	b.node("db", NodeKindResource)
	b.edge("alice", "temp-team", "member_of")
	b.edge("temp-team", "read", "performs")
	b.edge("read", "db", "on")
	g := b.build()

	got := g.getIdentityNodesWithAccess(b.startNode("db"))
	require.Empty(t, nodeNames(got), "temporary group must block standing access")
}

// TestGetIdentityNodesWithAccess_NonTemporaryGroupAllowed is the inverse:
// a regular (non-temporary) group must not block.
func TestGetIdentityNodesWithAccess_NonTemporaryGroupAllowed(t *testing.T) {
	t.Parallel()
	b := newGraphBuilder(t)
	b.node("alice", NodeKindIdentity)
	b.node("admins", NodeKindIdentityGroup, withTemporary(false))
	b.node("read", NodeKindAction, withActionType(ActionAllowed))
	b.node("db", NodeKindResource)
	b.edge("alice", "admins", "member_of")
	b.edge("admins", "read", "performs")
	b.edge("read", "db", "on")
	g := b.build()

	got := g.getIdentityNodesWithAccess(b.startNode("db"))
	require.Equal(t, []string{"alice"}, nodeNames(got))
}

// TestGetIdentityNodesWithAccess_CanRequestBlocks: can_request and
// can_review actions are meta-actions for the access-request system,
// not actual standing access.
func TestGetIdentityNodesWithAccess_CanRequestBlocks(t *testing.T) {
	t.Parallel()
	for _, subKind := range []string{string(NodeSubKindCanRequest), string(NodeSubKindCanReview)} {
		t.Run(subKind, func(t *testing.T) {
			t.Parallel()
			b := newGraphBuilder(t)
			b.node("alice", NodeKindIdentity)
			b.node("meta", NodeKindAction,
				withActionType(ActionAllowed),
				withActionSubKind(subKind),
			)
			b.node("db", NodeKindResource)
			b.edge("alice", "meta", "performs")
			b.edge("meta", "db", "on")
			g := b.build()

			got := g.getIdentityNodesWithAccess(b.startNode("db"))
			require.Empty(t, nodeNames(got), "%s must not grant standing access", subKind)
		})
	}
}

// TestGetIdentityNodesWithAccessExcluding_StopsAtExcludedNode verifies the
// excluded set acts as a hard cut: identities only reachable *through* an
// excluded node are not collected.
func TestGetIdentityNodesWithAccessExcluding_StopsAtExcludedNode(t *testing.T) {
	t.Parallel()
	b := newGraphBuilder(t)
	b.node("alice", NodeKindIdentity)
	b.node("bob", NodeKindIdentity)
	b.node("admins", NodeKindIdentityGroup, withTemporary(false))
	b.node("read", NodeKindAction, withActionType(ActionAllowed))
	b.node("db", NodeKindResource)
	b.edge("alice", "admins", "member_of")
	b.edge("bob", "read", "performs") // bob has direct access via read action
	b.edge("admins", "read", "performs")
	b.edge("read", "db", "on")
	g := b.build()

	excluded := map[uuid.UUID]bool{b.idFor("admins"): true}
	got := g.getIdentityNodesWithAccessExcluding(b.startNode("db"), excluded)
	// alice's only path runs through admins → excluded.
	// bob bypasses admins → still has access.
	require.Equal(t, []string{"bob"}, nodeNames(got))
}

// TestGetResourceNodesWithAccess_Symmetric confirms the outgoing variant
// produces the resource side of the same graph.
func TestGetResourceNodesWithAccess_Symmetric(t *testing.T) {
	t.Parallel()
	b := newGraphBuilder(t)
	b.node("alice", NodeKindIdentity)
	b.node("read", NodeKindAction, withActionType(ActionAllowed))
	b.node("db", NodeKindResource)
	b.node("api", NodeKindResource)
	b.node("deny-api", NodeKindAction, withActionType(ActionDenied))
	b.edge("alice", "read", "performs")
	b.edge("read", "db", "on")
	b.edge("alice", "deny-api", "performs")
	b.edge("deny-api", "api", "on")
	g := b.build()

	got := g.getResourceNodesWithAccess(b.startNode("alice"))
	require.Equal(t, []string{"db"}, nodeNames(got),
		"DENIED-blocked resources must be filtered, ALLOWED ones returned")
}

// TestVisit_HandlesCycle exercises the visited-edge set: a cycle in the
// graph must not loop forever.
func TestVisit_HandlesCycle(t *testing.T) {
	t.Parallel()
	b := newGraphBuilder(t)
	b.node("a", NodeKindIdentity)
	b.node("b", NodeKindIdentity)
	b.node("c", NodeKindIdentity)
	b.edge("a", "b", "x")
	b.edge("b", "c", "x")
	b.edge("c", "a", "x") // cycle
	g := b.build()

	visited := 0
	g.visitOutgoing(b.idFor("a"), func(*edgeWithTarget) bool {
		visited++
		return true
	})
	// Three distinct edges; each must be visited exactly once even
	// though the cycle could otherwise loop indefinitely.
	require.Equal(t, 3, visited)
}

// TestVisit_FnFalseStopsBranch confirms that returning false from the
// visit fn stops the traversal of that branch but doesn't abort sibling
// branches.
func TestVisit_FnFalseStopsBranch(t *testing.T) {
	t.Parallel()
	b := newGraphBuilder(t)
	b.node("root", NodeKindIdentity)
	b.node("stop", NodeKindIdentity)
	b.node("past-stop", NodeKindIdentity)
	b.node("sibling", NodeKindIdentity)
	b.edge("root", "stop", "x")
	b.edge("stop", "past-stop", "x")
	b.edge("root", "sibling", "x")
	g := b.build()

	visited := map[string]bool{}
	g.visitOutgoing(b.idFor("root"), func(e *edgeWithTarget) bool {
		visited[e.Target.Name] = true
		return e.Target.Name != "stop"
	})
	require.True(t, visited["stop"], "stop must be observed")
	require.True(t, visited["sibling"], "sibling must still be observed")
	require.False(t, visited["past-stop"], "branch past stop must be skipped")
}
