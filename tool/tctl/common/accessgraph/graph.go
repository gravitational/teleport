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

	models "github.com/gravitational/access-graph/api/client/models/graph"
	"github.com/oapi-codegen/runtime/types"
)

type EdgeWithTarget struct {
	Edge   *models.Edge
	Target *models.Node
}

type traversalGraph struct {
	// nodes keyed by UUID for O(1) lookup.
	nodes map[types.UUID]*models.Node
	// incomingEdges keyed by destination node ID for efficient reverse traversal.
	incomingEdges map[types.UUID][]EdgeWithTarget
	// outgoingEdges keyed by source node ID for efficient forward traversal.
	outgoingEdges map[types.UUID][]EdgeWithTarget
}

func newTraversalGraph(nodes *[]models.Node, edges *[]models.Edge) *traversalGraph {
	g := &traversalGraph{
		nodes:         make(map[types.UUID]*models.Node),
		incomingEdges: make(map[types.UUID][]EdgeWithTarget),
		outgoingEdges: make(map[types.UUID][]EdgeWithTarget),
	}

	if nodes != nil {
		for _, node := range *nodes {
			g.nodes[node.Id] = &node
		}
	}

	if edges != nil {
		for _, edge := range *edges {
			fromNode, fromExists := g.getNode(edge.From)
			toNode, toExists := g.getNode(edge.To)
			if fromExists && toExists {
				g.outgoingEdges[edge.From] = append(g.outgoingEdges[edge.From], EdgeWithTarget{Edge: &edge, Target: toNode})
				g.incomingEdges[edge.To] = append(g.incomingEdges[edge.To], EdgeWithTarget{Edge: &edge, Target: fromNode})
			}
		}
	}

	return g
}

func (g *traversalGraph) getNode(id types.UUID) (*models.Node, bool) {
	node, exists := g.nodes[id]
	return node, exists
}

func (g *traversalGraph) getOutgoingEdges(nodeID types.UUID) (*[]EdgeWithTarget, bool) {
	edges, exists := g.outgoingEdges[nodeID]
	return &edges, exists
}

func (g *traversalGraph) getIncomingEdges(nodeID types.UUID) (*[]EdgeWithTarget, bool) {
	edges, exists := g.incomingEdges[nodeID]
	return &edges, exists
}

// EdgeVisitFn is called for each edge and its target node during BFS traversal.
// Returning false stops traversal of that branch.
type EdgeVisitFn func(*EdgeWithTarget) bool

// GetEdgesFunc retrieves the edges associated with a node ID.
type GetEdgesFunc func(types.UUID) (*[]EdgeWithTarget, bool)

// visitOutgoing performs a BFS from startID following outgoing edges.
// fn is called for each edge and its destination node; returning false skips that branch.
func (g *traversalGraph) visitOutgoing(startID types.UUID, fn EdgeVisitFn) {
	g.visit(g.getOutgoingEdges, startID, fn)
}

// visitIncoming performs a BFS from startID following incoming edges.
// fn is called for each edge and its source node; returning false skips that branch.
func (g *traversalGraph) visitIncoming(startID types.UUID, fn EdgeVisitFn) {
	g.visit(g.getIncomingEdges, startID, fn)
}

// visit performs a BFS using getEdges to expand each node, calling fn for each
// edge visited. Edges are tracked by pointer so the same edge is never visited
// twice, but a node may be revisited via a different edge.
func (g *traversalGraph) visit(getEdges GetEdgesFunc, startID types.UUID, fn EdgeVisitFn) {
	visited := map[*models.Edge]bool{}
	queue := []types.UUID{startID}

	for len(queue) > 0 {
		nodeID := queue[0]
		queue = queue[1:]

		edges, exists := getEdges(nodeID)
		if !exists {
			continue
		}

		for _, edge := range *edges {
			if edge.Edge == nil || visited[edge.Edge] {
				continue
			}
			visited[edge.Edge] = true
			nextNode, exists := g.getNode(edge.Target.Id)
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

// GetIdentityNodesWithAccess returns all identity nodes that have standing
// access to the given resource node, excluding those whose access is blocked
// by deny actions or temporary (request-based) grants.
func (g *traversalGraph) GetIdentityNodesWithAccess(start models.Node) []*models.Node {
	return g.GetIdentityNodesWithAccessExcluding(start, nil)
}

// GetIdentityNodesWithAccessExcluding is like GetIdentityNodesWithAccess but
// treats every node in excluded as blocking: traversal stops at those nodes,
// so identities only reachable *through* them are not collected. Useful for
// checking alternative access paths that bypass specific subjects (e.g. the
// ACL/role being reviewed).
func (g *traversalGraph) GetIdentityNodesWithAccessExcluding(start models.Node, excluded map[types.UUID]bool) []*models.Node {
	// Phase 1: find nodes reachable without going through blocking/excluded
	// nodes. Deny actions are collected for phase 2.
	var denyActions []*models.Node
	unblockedReachable := map[types.UUID]bool{}
	g.visitIncoming(start.Id, func(edge *EdgeWithTarget) bool {
		node := edge.Target
		if excluded[node.Id] {
			return false
		}
		unblockedReachable[node.Id] = true
		if node.Kind == "action" {
			props, err := node.Properties.AsActionProperties()
			if err == nil && props.Type == "DENIED" {
				denyActions = append(denyActions, node)
			}
		}
		return !isBlocking(edge)
	})

	// Phase 2: collect identities reachable from deny actions so they can be
	// excluded from the final result.
	deniedIdentities := map[types.UUID]bool{}
	for _, action := range denyActions {
		g.visitIncoming(action.Id, func(edge *EdgeWithTarget) bool {
			node := edge.Target
			if excluded[node.Id] {
				return false
			}
			if node.Kind == "identity" {
				deniedIdentities[node.Id] = true
			}
			return true
		})
	}

	// Phase 3: collect identity nodes reachable from the resource, skipping
	// owner_of edges, excluded nodes, denied identities, and any path that
	// crosses a blocking edge (can_request/can_review actions, DENIED actions,
	// temporary identity groups). Blocked edges cannot provide standing access.
	identityNodes := make(map[types.UUID]*models.Node)
	g.visitIncoming(start.Id, func(edge *EdgeWithTarget) bool {
		if edge.Edge.EdgeType == "owner_of" {
			return false
		}
		node := edge.Target
		if excluded[node.Id] {
			return false
		}
		if node.Kind == "identity" && !deniedIdentities[node.Id] {
			identityNodes[node.Id] = node
		}
		return !isBlocking(edge)
	})
	return slices.Collect(maps.Values(identityNodes))
}

// isBlocking reports whether traversal should stop at node when computing
// standing privileges:
//
//   - Temporary identity groups: access was granted via an access request.
//   - can_request/can_review actions: meta-actions for the request system,
//     not actual resource access.
//   - Non-allowed actions: access is denied or conditional.
func isBlocking(edge *EdgeWithTarget) bool {

	node := edge.Target

	switch node.Kind {
	case "identity_group":
		props, err := node.Properties.AsIdentityGroupProperties()
		if err != nil || props.Temporary == nil {
			return false
		}
		return *props.Temporary
	case "action":
		if node.SubKind == "can_request" || node.SubKind == "can_review" {
			return true
		}
		props, err := node.Properties.AsActionProperties()
		if err != nil {
			return false
		}
		return props.Type != "ALLOWED"
	}
	return false
}
