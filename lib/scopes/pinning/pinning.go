/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package pinning

import (
	"iter"
	"slices"

	"github.com/gravitational/trace"

	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/lib/scopes"
)

// StrongValidate checks if the scope pin is well-formed according to all scope pin rules. This function
// *must* be used to validate any scope pin being created from scratch. Use of this function should be
// avoided for checking the validity of existing scope pins extracted from certificates and/or as part
// of any agent-side checks. Prefer using [WeakValidate] in those cases.
func StrongValidate(pin *scopesv1.Pin) error {
	if pin == nil {
		return trace.BadParameter("missing scope pin")
	}

	if err := scopes.StrongValidate(pin.GetScope()); err != nil {
		return trace.Errorf("invalid pinned scope: %w", err)
	}

	if pin.GetAssignmentTree() == nil {
		// in theory there isn't any harm in allowing pins to be created without any assignments, but we're choosing to err
		// on the side of caution for now. this limitation may be lifted later.
		// NOTE: if lifting this restriction, the equivalent check in the pin building logic must also be lifted.
		return trace.BadParameter("scope pin at %q contains no assignment tree", pin.GetScope())
	}

	if len(pin.GetAssignments()) != 0 { //nolint:staticcheck // SA1019.
		return trace.BadParameter("scope pin uses outdated format, plase ensure all teleport components are upgraded and relogin")
	}

	// Validate all assignments in the tree by enumerating them
	hasAssignments := false
	for assignment := range EnumerateAllAssignments(pin) {
		hasAssignments = true

		if err := scopes.StrongValidate(assignment.ScopeOfOrigin); err != nil {
			return trace.Errorf("invalid scope of origin %q in pinned assignment: %w", assignment.ScopeOfOrigin, err)
		}

		if err := scopes.StrongValidate(assignment.ScopeOfEffect); err != nil {
			return trace.Errorf("invalid scope of effect %q in pinned assignment: %w", assignment.ScopeOfEffect, err)
		}

		if assignment.RoleName == "" {
			return trace.BadParameter("scope pin at %q contains assignment with empty role name", pin.GetScope())
		}

		if !PinCompatibleWithPolicyScope(pin, assignment.ScopeOfOrigin) {
			return trace.BadParameter("scope pin at %q contains assignment with incompatible scope of origin %q", pin.GetScope(), assignment.ScopeOfOrigin)
		}

		if !PinCompatibleWithPolicyScope(pin, assignment.ScopeOfEffect) {
			return trace.BadParameter("scope pin at %q contains assignment with incompatible scope of effect %q", pin.GetScope(), assignment.ScopeOfEffect)
		}
	}

	if !hasAssignments {
		return trace.BadParameter("scope pin at %q contains no assignments", pin.GetScope())
	}

	return nil
}

// WeakValidate performs a weak form of validation on a scope pin. This function is intended to catch
// bugs/incompatibilities that might have resulted in a scope pin too malformed for us to safely reason
// about (e.g. due to significant version drift). Use this function to validate scope pins extracted from
// certificates or otherwsie propagated from the control plane. Prefer using [StrongValidate] when building
// a new scope pin from scratch.
func WeakValidate(pin *scopesv1.Pin) error {
	if pin == nil {
		return trace.BadParameter("missing scope pin")
	}

	if err := scopes.WeakValidate(pin.GetScope()); err != nil {
		return trace.Errorf("invalid pinned scope: %w", err)
	}

	if len(pin.GetAssignments()) != 0 { //nolint:staticcheck // SA1019.
		return trace.BadParameter("scope pin uses outdated format, plase ensure all teleport components are upgraded and relogin")
	}

	// Note that we do not valdiate the assignment tree here. Scoped access checks are designed to omit
	// any assignments that are malformed. This is allowable because the scoped role model promises to
	// never apply deny rules or cross-role side effects via roles. This makes it safe to simply skip
	// anything we don't understand.

	return nil
}

// PinCompatibleWithPolicyScope checks if a given policy scope might affect access subject to the pin. When building a
// pin, all compatible scoped role assignments must be encoded on the pin. Access while pinned may be affected by any policy
// that is ancestral, equivalent, or descendant to the pin scope, meaning that all non-orthogonal scopes are compatible.
func PinCompatibleWithPolicyScope(pin *scopesv1.Pin, scope string) bool {
	return scopes.Compare(pin.GetScope(), scope) != scopes.Orthogonal
}

// PinAppliesToResourceScope verifies that the given pin pins a scope that applies to the given resource scope. Resources
// that are not subject to the pinned scope cannot be accessed by the pinned identity, even if assigned roles would
// otherwise allow access. Note that this is conceptually distinct from whether or not we can perform *access checks* with
// a pin at a given scope. A pin at scope /foo/bar might still yield an allow decision at resource scope /foo, but only if
// the target resource is assigned to /foo/bar or one of its descendants.
func PinAppliesToResourceScope(pin *scopesv1.Pin, resourceScope string) bool {
	return scopes.PolicyScope(pin.GetScope()).AppliesToResourceScope(resourceScope)
}

// AssignmentTreeFromMap builds an assignment tree from a nested mapping of the form scopeOfOrigin -> scopeOfEffect -> roles. This is useful
// for tests as it greatly simplifies the construction of asssignment tree literals, which are difficult and verbose to build by hand. Note that
// this function does not perform any validation and should not be used to build pins for production use.
func AssignmentTreeFromMap(m map[string]map[string][]string) *scopesv1.AssignmentNode {
	var pin scopesv1.Pin
	for scopeOfOrigin := range m {
		for scopeOfEffect := range m[scopeOfOrigin] {
			for _, roleName := range m[scopeOfOrigin][scopeOfEffect] {
				WriteRoleAssignmentUnchecked(&pin, RoleAssignment{
					ScopeOfOrigin: scopeOfOrigin,
					ScopeOfEffect: scopeOfEffect,
					RoleName:      roleName,
				})
			}
		}
	}

	return pin.AssignmentTree
}

// AssignmentTreeIntoMap converts an assignment tree back into a nested map of the form scopeOfOrigin -> scopeOfEffect -> roles.
// This is the inverse of AssignmentTreeFromMap and is useful for debug logging and tests.
func AssignmentTreeIntoMap(tree *scopesv1.AssignmentNode) map[string]map[string][]string {
	if tree == nil {
		return nil
	}

	out := make(map[string]map[string][]string)

	for assignment := range EnumerateAllAssignments(&scopesv1.Pin{AssignmentTree: tree}) {
		if out[assignment.ScopeOfOrigin] == nil {
			out[assignment.ScopeOfOrigin] = make(map[string][]string)
		}
		out[assignment.ScopeOfOrigin][assignment.ScopeOfEffect] = append(
			out[assignment.ScopeOfOrigin][assignment.ScopeOfEffect],
			assignment.RoleName,
		)
	}

	return out
}

// RoleAssignment contains the details of a pinned role assignment. This type is used when building and iterating the assignment
// tree structure within a scope pin.
type RoleAssignment struct {
	// ScopeOfOrigin is the scope that the role was assigned *from* (i.e. the scope of the assignment resource that specified the
	// role assignment). Roles with a more ancestral Scope of Origin take precedence over roles with a more descendant Scope of Origin.
	ScopeOfOrigin string

	// ScopeOfEffect is the scope that the role is assigned *to* (i.e. the scope of resources that the role's privileges apply to). Roles with
	// a more descendant/specific Scope of Effect take precedence over roles with a more ancestral/general Scope of Effect.
	ScopeOfEffect string

	// RoleName is the name of the role that is assigned.
	RoleName string
}

// WriteRoleAssignment encodes a role assignment into the given scope pin's assignment tree. The pin must be compatible
// with both the scope of origin and scope of effect of the assignment, and the scopes must be valid.
func WriteRoleAssignment(pin *scopesv1.Pin, assignment RoleAssignment) error {
	// verify that the assignment's scopes look like valid scopes.
	if err := scopes.WeakValidate(assignment.ScopeOfOrigin); err != nil {
		return trace.Errorf("cannot write role assignment to scope pin, invalid scope of origin %q for role %q: %w", assignment.ScopeOfOrigin, assignment.RoleName, err)
	}
	if err := scopes.WeakValidate(assignment.ScopeOfEffect); err != nil {
		return trace.Errorf("cannot write role assignment to scope pin, invalid scope of effect %q for role %q: %w", assignment.ScopeOfEffect, assignment.RoleName, err)
	}

	// verify that the assignemnt's scopes actually apply to the pin's scope.
	if !PinCompatibleWithPolicyScope(pin, assignment.ScopeOfOrigin) {
		return trace.Errorf("cannot write role assignment with scope of origin %q to pin at %q: incompatible scopes", assignment.ScopeOfOrigin, pin.GetScope())
	}
	if !PinCompatibleWithPolicyScope(pin, assignment.ScopeOfEffect) {
		return trace.Errorf("cannot write role assignment with scope of effect %q to pin at %q: incompatible scopes", assignment.ScopeOfEffect, pin.GetScope())
	}

	WriteRoleAssignmentUnchecked(pin, assignment)

	return nil
}

// WriteRoleAssignmentUnchecked is like WriteRoleAssignment, but does not perform any validation on the pin or assignment. This is useful
// for tests.
func WriteRoleAssignmentUnchecked(pin *scopesv1.Pin, assignment RoleAssignment) {
	// ensure the pin's assignment tree is initialized
	if pin.AssignmentTree == nil {
		pin.AssignmentTree = &scopesv1.AssignmentNode{}
	}

	// start at the root of the assignment tree
	assignmentNode := pin.AssignmentTree

	// descend to the correct assignment node for the scope of origin
	for segment := range scopes.DescendingSegments(assignment.ScopeOfOrigin) {
		if assignmentNode.Children == nil {
			assignmentNode.Children = make(map[string]*scopesv1.AssignmentNode)
		}

		child, ok := assignmentNode.Children[segment]
		if !ok {
			child = &scopesv1.AssignmentNode{}
			assignmentNode.Children[segment] = child
		}

		assignmentNode = child
	}

	// ensure the role tree is initialized for this assignment node
	if assignmentNode.RoleTree == nil {
		assignmentNode.RoleTree = &scopesv1.RoleNode{}
	}

	// start at the root of the role tree for this assignment node
	roleNode := assignmentNode.RoleTree

	// descend to the correct role node for the scope of effect
	for segment := range scopes.DescendingSegments(assignment.ScopeOfEffect) {
		if roleNode.Children == nil {
			roleNode.Children = make(map[string]*scopesv1.RoleNode)
		}

		child, ok := roleNode.Children[segment]
		if !ok {
			child = &scopesv1.RoleNode{}
			roleNode.Children[segment] = child
		}

		roleNode = child
	}

	// append the role to the role list for this role node if it's not already present
	if !slices.Contains(roleNode.Roles, assignment.RoleName) {
		roleNode.Roles = append(roleNode.Roles, assignment.RoleName)
		// ensure the role list is sorted for deterministic iteration
		slices.Sort(roleNode.Roles)
	}
}

// DescendAssignmentTree is the helper used to determine the sequence of pinned role assignments applicable to a given
// resource. The order in which assignments are yielded is the order in which roles should be evaluated for access
// checking. Ordering is determined by a combination of both the Scope of Origin and Scope of Effect of a role. Roles with
// more ancestral Scopes of Origin are yielded before roles with more descendant Scope of Origin to preserve scope hierarchy.
// Within a given Scope of Origin, roles with more descendant/specific Scopes of Effect are yielded before roles with more
// ancestral/general Scopes of Effect to allow more specific assignments to override more general ones. See the Scopes RFD for
// an in-depth discussion of scoped role evaluation ordering and why it matters.
func DescendAssignmentTree(pin *scopesv1.Pin, resourceScope string) (iter.Seq[RoleAssignment], error) {
	if !PinAppliesToResourceScope(pin, resourceScope) {
		// a pin with a scope that does not apply to the resource scope should be caught at an
		// earlier stage, but failure to catch this may be a security issue, so we include a
		// redundant check here to prevent accidental misuse.
		return nil, trace.Errorf("invalid resource scope %q for scope pin at %q in assignment lookup (this is a bug)", resourceScope, pin.GetScope())
	}
	return func(yield func(RoleAssignment) bool) {
		if pin.AssignmentTree == nil {
			return
		}

		resourceScopeSegments := scopes.Split(resourceScope)
		yieldAssignmentNode(pin.AssignmentTree, resourceScopeSegments, 0 /*depth*/, yield)
	}, nil
}

// yeildAssignmentNode recursively descends the assignment tree, yielding all role assignments that match the given resource
// scope segments. The assignment tree represents the Scope of Origin of the roles it contains and is descended from root to
// leaf, with roles with an ancestral Scope of Origin being yielded before roles with a more specific Scope of Origin in order
// to preserve scope hierarchy during evaluation.
func yieldAssignmentNode(node *scopesv1.AssignmentNode, resourceScopeSegments []string, depth int, yield func(RoleAssignment) bool) bool {
	// first yield any matching roles from the current depth's role tree
	if node.RoleTree != nil {
		scopeOfOrigin := scopes.Join(resourceScopeSegments[:depth]...)
		if !yeildRoleNode(node.RoleTree, scopeOfOrigin, resourceScopeSegments, 0 /*role tree depth*/, yield) {
			return false
		}
	}

	if len(resourceScopeSegments) > depth {
		if child, ok := node.Children[resourceScopeSegments[depth]]; ok {
			if !yieldAssignmentNode(child, resourceScopeSegments, depth+1, yield) {
				return false
			}
		}
	}

	return true
}

// yeildRoleNode recursively yeidls a sequence of role assignments encoded in the pinned role tree matching the given
// resource scope segments. The assignments are yielded in specificity order, starting from the most specific (leaf) scope
// and ascending to the least specific (root) scope. Note that this is the opposite of how we typically traverse the scope
// hierarchy. Most hierarchical operations in scopes are performed from top to bottom in order to preserve scope hierarchy.
// Because all roles within a given role tree were assigned *from* the same Scope of Origin, they are of equivalent seniority
// from a scope hierarchy perspective. This frees us to process them using a most-specific-first approach, which allows
// admins within a given scope to have greater expressiveness and control when authoring scoped roles. See the scopes RFD for
// details on scoped role evaluation ordering and its implications.
func yeildRoleNode(node *scopesv1.RoleNode, scopeOfOrigin string, resourceScopeSegments []string, depth int, yield func(RoleAssignment) bool) bool {
	if len(resourceScopeSegments) > depth {
		if child, ok := node.Children[resourceScopeSegments[depth]]; ok {
			if !yeildRoleNode(child, scopeOfOrigin, resourceScopeSegments, depth+1, yield) {
				return false
			}
		}
	}

	if len(node.Roles) == 0 {
		return true
	}

	scopeOfEffect := scopes.Join(resourceScopeSegments[:depth]...)
	for _, roleName := range node.Roles {
		if !yield(RoleAssignment{
			ScopeOfOrigin: scopeOfOrigin,
			ScopeOfEffect: scopeOfEffect,
			RoleName:      roleName,
		}) {
			return false
		}
	}

	return true
}

// GetRolesAtEnforcementPoint returns an iterator over role names assigned at the specified enforcement point
// within the assignment tree. Returns an empty iterator if no roles are assigned at that combination.
//
// This function is intended to be composed with [scopes.EnforcementPointsForResourceScope] to allow callers to fetch roles
// at each hierarchy level as they evaluate access. Note that it would be more efficient to build an iterator that
// traverses the tree once in enforcement order rather than repeatedly navigating to specific points, but the decoupling
// of reading from ordering results in better decoupling of concerns and keeps our options open for future policy
// types to be added to higher level logic without needing to care about assignment tree internals.
//
// NOTE: the order in which roles are evaluated is *extremely important* for correct scoped role behavior. Before calling
// this function in an access evaluation context, ensure that the enforcement points are being processed in the correct
// order as described in the scopes RFD.
func GetRolesAtEnforcementPoint(pin *scopesv1.Pin, point scopes.EnforcementPoint) iter.Seq[string] {
	return func(yield func(string) bool) {
		if pin == nil || pin.AssignmentTree == nil {
			return
		}

		if point.ScopeOfOrigin == "" || point.ScopeOfEffect == "" {
			return
		}

		// navigate to the assignment node for the Scope of Origin
		assignmentNode := pin.AssignmentTree
		for segment := range scopes.DescendingSegments(point.ScopeOfOrigin) {
			child, ok := assignmentNode.Children[segment]
			if !ok {
				// the scope of origin doesn't exist in the tree
				return
			}
			assignmentNode = child
		}

		// navigate to the role node for the Scope of Effect within this assignment node
		if assignmentNode.RoleTree == nil {
			return
		}

		roleNode := assignmentNode.RoleTree
		for segment := range scopes.DescendingSegments(point.ScopeOfEffect) {
			child, ok := roleNode.Children[segment]
			if !ok {
				// the scope of effect doesn't exist in the tree
				return
			}
			roleNode = child
		}

		// yield each role name in the order that they are stored. it is the responsibility
		// of pin construction logic to ensure deterministic ordering.
		for _, roleName := range roleNode.Roles {
			if !yield(roleName) {
				return
			}
		}
	}
}

// EnumerateAllAssignments yields all role assignments contained in the pin's assignment tree,
// regardless of any target resource scope. The order is undefined and should not be relied upon
// for access control decisions. This is primarily useful for operations that need to examine
// the full set of possible permissions (e.g. determining all possible logins a user might have).
//
// *NOTE*: this function is not suitable for being the basis of access control evaluation ordering.
func EnumerateAllAssignments(pin *scopesv1.Pin) iter.Seq[RoleAssignment] {
	return func(yield func(RoleAssignment) bool) {
		if pin == nil || pin.AssignmentTree == nil {
			return
		}

		// Start enumeration at the root of the assignment tree with empty segment list (representing root scope)
		enumerateAssignmentNode(pin.AssignmentTree, nil, yield)
	}
}

// enumerateAssignmentNode recursively walks an assignment tree node and all its descendants,
// yielding all role assignments found. The originSegments parameter tracks the scope of origin
// path from root to the current node.
func enumerateAssignmentNode(node *scopesv1.AssignmentNode, originSegments []string, yield func(RoleAssignment) bool) bool {
	scopeOfOrigin := scopes.Join(originSegments...)

	// Enumerate all roles in this node's role tree
	if node.RoleTree != nil {
		if !enumerateRoleNode(node.RoleTree, scopeOfOrigin, nil, yield) {
			return false
		}
	}

	// Recursively enumerate all child assignment nodes
	for segment, child := range node.Children {
		childOriginSegments := append(originSegments, segment)
		if !enumerateAssignmentNode(child, childOriginSegments, yield) {
			return false
		}
	}

	return true
}

// enumerateRoleNode recursively walks a role tree node and all its descendants, yielding all
// role assignments found. The effectSegments parameter tracks the scope of effect path from
// root to the current node.
func enumerateRoleNode(node *scopesv1.RoleNode, scopeOfOrigin string, effectSegments []string, yield func(RoleAssignment) bool) bool {
	scopeOfEffect := scopes.Join(effectSegments...)

	// Yield all roles at this node
	for _, roleName := range node.Roles {
		if !yield(RoleAssignment{
			ScopeOfOrigin: scopeOfOrigin,
			ScopeOfEffect: scopeOfEffect,
			RoleName:      roleName,
		}) {
			return false
		}
	}

	// Recursively enumerate all child role nodes
	for segment, child := range node.Children {
		childEffectSegments := append(effectSegments, segment)
		if !enumerateRoleNode(child, scopeOfOrigin, childEffectSegments, yield) {
			return false
		}
	}

	return true
}
