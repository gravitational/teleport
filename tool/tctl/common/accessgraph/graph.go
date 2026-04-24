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

// Package accessgraph provides a graph implementation for traversing access
// graph data returned by the Access Graph API. It supports path traversals to
// determine which resources are accessible to which identities, and to analyze
// how access paths have changed over time.
package accessgraph

import (
	"maps"
	"slices"

	"github.com/google/uuid"

	models "github.com/gravitational/teleport/lib/accessgraph/apiclient/models/graph"
)

// Node kind / sub-kind / edge kind taxonomies used by Access Graph.
// Only the subset we actually traverse is declared here; expand as new
// callers need more.

type NodeKind string

const (
	NodeKindAction        NodeKind = "action"
	NodeKindIdentity      NodeKind = "identity"
	NodeKindIdentityGroup NodeKind = "identity_group"
	NodeKindResource      NodeKind = "resource"
)

type NodeSubKind string

const (
	// NodeSubKindCanRequest is the can_request action sub-kind.
	NodeSubKindCanRequest NodeSubKind = "can_request"
	// NodeSubKindCanReview is the can_review action sub-kind.
	NodeSubKindCanReview NodeSubKind = "can_review"
)

type EdgeKind string

const (
	EdgeKindOwnerOf EdgeKind = "owner_of"
)

// ActionProperties.Type values reported by Access Graph.
const (
	ActionAllowed = "ALLOWED"
	ActionDenied  = "DENIED"
)

type edgeWithTarget struct {
	Edge   *models.Edge
	Target *models.Node
}

type traversalGraph struct {
	// nodes keyed by UUID for O(1) lookup.
	nodes map[uuid.UUID]*models.Node
	// incomingEdges keyed by destination node ID for efficient reverse traversal.
	incomingEdges map[uuid.UUID][]edgeWithTarget
	// outgoingEdges keyed by source node ID for efficient forward traversal.
	outgoingEdges map[uuid.UUID][]edgeWithTarget
}

func newTraversalGraph(nodes *[]models.Node, edges *[]models.Edge) *traversalGraph {
	g := &traversalGraph{
		nodes:         make(map[uuid.UUID]*models.Node),
		incomingEdges: make(map[uuid.UUID][]edgeWithTarget),
		outgoingEdges: make(map[uuid.UUID][]edgeWithTarget),
	}

	if nodes != nil {
		for _, node := range *nodes {
			g.nodes[node.Id] = &node
		}
	}

	if edges != nil {
		for _, edge := range *edges {
			fromNode, fromExists := g.nodes[edge.From]
			toNode, toExists := g.nodes[edge.To]
			if fromExists && toExists {
				g.outgoingEdges[edge.From] = append(g.outgoingEdges[edge.From], edgeWithTarget{Edge: &edge, Target: toNode})
				g.incomingEdges[edge.To] = append(g.incomingEdges[edge.To], edgeWithTarget{Edge: &edge, Target: fromNode})
			}
		}
	}

	return g
}

// edgeVisitFn is called for each edge and its target node during BFS
// traversal. Returning false stops traversal of that branch.
type edgeVisitFn func(*edgeWithTarget) bool

// getEdgesFn retrieves the edges associated with a node ID.
type getEdgesFn func(uuid.UUID) []edgeWithTarget

// visitOutgoing performs a BFS from startID following outgoing edges.
// fn is called for each edge and its destination node; returning false skips that branch.
func (g *traversalGraph) visitOutgoing(startID uuid.UUID, fn edgeVisitFn) {
	g.visit(func(id uuid.UUID) []edgeWithTarget { return g.outgoingEdges[id] }, startID, fn)
}

// visitIncoming performs a BFS from startID following incoming edges.
// fn is called for each edge and its source node; returning false skips that branch.
func (g *traversalGraph) visitIncoming(startID uuid.UUID, fn edgeVisitFn) {
	g.visit(func(id uuid.UUID) []edgeWithTarget { return g.incomingEdges[id] }, startID, fn)
}

// visit performs a BFS using getEdges to expand each node, calling fn for each
// edge visited. Edges are tracked by pointer so the same edge is never visited
// twice, but a node may be revisited via a different edge.
func (g *traversalGraph) visit(getEdges getEdgesFn, startID uuid.UUID, fn edgeVisitFn) {
	visited := map[*models.Edge]bool{}
	queue := []uuid.UUID{startID}

	for len(queue) > 0 {
		nodeID := queue[0]
		queue = queue[1:]

		for _, edge := range getEdges(nodeID) {
			if edge.Edge == nil || visited[edge.Edge] {
				continue
			}
			visited[edge.Edge] = true
			nextNode, exists := g.nodes[edge.Target.Id]
			if !exists {
				continue
			}
			if !fn(&edge) {
				continue
			}
			queue = append(queue, nextNode.Id)
		}
	}
}

// getIdentityNodesWithAccess returns all identity nodes that have standing
// access to the given resource node, excluding those whose access is blocked
// by deny actions or temporary (request-based) grants.
func (g *traversalGraph) getIdentityNodesWithAccess(start models.Node) []*models.Node {
	return g.getIdentityNodesWithAccessExcluding(start, nil)
}

// getIdentityNodesWithAccessExcluding is like getIdentityNodesWithAccess but
// treats every node in excluded as blocking: traversal stops at those nodes,
// so identities only reachable *through* them are not collected. Useful for
// checking alternative access paths that bypass specific subjects (e.g. the
// ACL/role being reviewed).
func (g *traversalGraph) getIdentityNodesWithAccessExcluding(start models.Node, excluded map[uuid.UUID]bool) []*models.Node {
	// Phase 1: walk the (unblocked) reachable subgraph collecting deny
	// actions. We don't care which non-action nodes are reachable, only
	// the deny actions — phase 2 expands from those.
	var denyActions []*models.Node
	g.visitIncoming(start.Id, func(edge *edgeWithTarget) bool {
		node := edge.Target
		if excluded[node.Id] {
			return false
		}
		if NodeKind(node.Kind) == NodeKindAction {
			props, err := node.Properties.AsActionProperties()
			if err == nil && props.Type == ActionDenied {
				denyActions = append(denyActions, node)
			}
		}
		return !isBlocking(edge)
	})

	// Phase 2: collect identities reachable from deny actions so they can be
	// excluded from the final result.
	deniedIdentities := map[uuid.UUID]bool{}
	for _, action := range denyActions {
		g.visitIncoming(action.Id, func(edge *edgeWithTarget) bool {
			node := edge.Target
			if excluded[node.Id] {
				return false
			}
			if NodeKind(node.Kind) == NodeKindIdentity {
				deniedIdentities[node.Id] = true
			}
			return true
		})
	}

	// Phase 3: collect identity nodes reachable from the resource, skipping
	// owner_of edges, excluded nodes, denied identities, and any path that
	// crosses a blocking edge (can_request/can_review actions, DENIED actions,
	// temporary identity groups). Blocked edges cannot provide standing access.
	identityNodes := make(map[uuid.UUID]*models.Node)
	g.visitIncoming(start.Id, func(edge *edgeWithTarget) bool {
		if EdgeKind(edge.Edge.EdgeType) == EdgeKindOwnerOf {
			return false
		}
		node := edge.Target
		if excluded[node.Id] {
			return false
		}
		if NodeKind(node.Kind) == NodeKindIdentity && !deniedIdentities[node.Id] {
			identityNodes[node.Id] = node
		}
		return !isBlocking(edge)
	})
	return slices.Collect(maps.Values(identityNodes))
}

// getResourceNodesWithAccess returns all resource nodes that are accessible
// to the given identity node, excluding those that are blocked by deny
// actions or temporary (request-based) grants. The logic mirrors
// getIdentityNodesWithAccess but traverses outgoing edges instead of
// incoming.
func (g *traversalGraph) getResourceNodesWithAccess(start models.Node) []*models.Node {
	return g.getResourceNodesWithAccessExcluding(start, nil)
}

func (g *traversalGraph) getResourceNodesWithAccessExcluding(start models.Node, excluded map[uuid.UUID]bool) []*models.Node {
	// See getIdentityNodesWithAccessExcluding for the phase-by-phase
	// rationale; this is the symmetric (outgoing) variant.
	var denyActions []*models.Node
	g.visitOutgoing(start.Id, func(edge *edgeWithTarget) bool {
		node := edge.Target
		if excluded[node.Id] {
			return false
		}
		if NodeKind(node.Kind) == NodeKindAction {
			props, err := node.Properties.AsActionProperties()
			if err == nil && props.Type == ActionDenied {
				denyActions = append(denyActions, node)
			}
		}
		return !isBlocking(edge)
	})

	deniedResources := map[uuid.UUID]bool{}
	for _, action := range denyActions {
		g.visitOutgoing(action.Id, func(edge *edgeWithTarget) bool {
			node := edge.Target
			if excluded[node.Id] {
				return false
			}
			if NodeKind(node.Kind) == NodeKindResource {
				deniedResources[node.Id] = true
			}
			return true
		})
	}

	resourceNodes := make(map[uuid.UUID]*models.Node)
	g.visitOutgoing(start.Id, func(edge *edgeWithTarget) bool {
		if EdgeKind(edge.Edge.EdgeType) == EdgeKindOwnerOf {
			return false
		}
		node := edge.Target
		if excluded[node.Id] {
			return false
		}
		if NodeKind(node.Kind) == NodeKindResource && !deniedResources[node.Id] {
			resourceNodes[node.Id] = node
		}
		return !isBlocking(edge)
	})

	return slices.Collect(maps.Values(resourceNodes))
}

// isBlocking reports whether traversal should stop at node when computing
// standing privileges:
//
//   - Temporary identity groups: access was granted via an access request.
//   - can_request/can_review actions: meta-actions for the request system,
//     not actual resource access.
//   - Non-allowed actions: access is denied or conditional.
func isBlocking(edge *edgeWithTarget) bool {
	node := edge.Target
	switch NodeKind(node.Kind) {
	case NodeKindIdentityGroup:
		props, err := node.Properties.AsIdentityGroupProperties()
		if err != nil || props.Temporary == nil {
			return false
		}
		return *props.Temporary
	case NodeKindAction:
		subKind := NodeSubKind(node.SubKind)
		if subKind == NodeSubKindCanRequest || subKind == NodeSubKindCanReview {
			return true
		}
		props, err := node.Properties.AsActionProperties()
		if err != nil {
			return false
		}
		return props.Type != ActionAllowed
	}
	return false
}
