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

package services

import (
	"context"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/scopes"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
	"github.com/gravitational/teleport/lib/scopes/pinning"
)

// ErrScopedIdentity is returned when a component intended for use only with unscoped identities receives a scoped
// identity. Methods that implement scoping support may check for this error and fallback to scoped authorization
// as appropriate.
var ErrScopedIdentity = &trace.AccessDeniedError{
	Message: "scoped identities not supported",
}

// scopedAccessCheckerBuilder is a helper that builds scoped access checkers.
type scopedAccessCheckerBuilder struct {
	info         *AccessInfo
	localCluster string
	reader       ScopedRoleReader
}

// Check verifies that the builder was provided with all necessary parameters and that they are well-formed.
func (b *scopedAccessCheckerBuilder) Check() error {
	if b.reader == nil {
		return trace.BadParameter("cannot create scoped access checkers without a scoped role reader")
	}
	if b.localCluster == "" {
		return trace.BadParameter("cannot create scoped access checkers without a local cluster name")
	}
	if b.info.ScopePin == nil {
		return trace.BadParameter("cannot create scoped access checkers for unscoped identity")
	}
	if len(b.info.AllowedResourceAccessIDs) != 0 {
		return trace.BadParameter("cannot create scoped access checkers for identity with active resource IDs")
	}
	if err := pinning.WeakValidate(b.info.ScopePin); err != nil {
		return trace.Errorf("cannot create scoped access checkers: %w", err)
	}
	return nil
}

func (b *scopedAccessCheckerBuilder) newCheckerForRole(ctx context.Context, key roleCheckerKey) (*ScopedAccessChecker, error) {
	if key == defaultImplicitRoleKey {
		return b.newDefaultImplicitChecker(ctx), nil
	}

	rsp, err := b.reader.GetScopedRole(ctx, &scopedaccessv1.GetScopedRoleRequest{
		Name: key.roleName,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// ensure that the role's resource scope makes it assignable from the scope of origin
	if !scopedaccess.RoleIsAssignableFromScopeOfOrigin(rsp.Role, key.scopeOfOrigin) {
		return nil, trace.BadParameter("scoped role %q is not assignable from scope of origin %q", key.roleName, key.scopeOfOrigin)
	}

	// ensure the role's configuration makes it assignable at the scope of effect
	if !scopedaccess.RoleIsAssignableToScopeOfEffect(rsp.Role, key.scopeOfEffect) {
		return nil, trace.BadParameter("scoped role %q is not assignable to scope of effect %q", key.roleName, key.scopeOfEffect)
	}

	// Convert the scoped role to a classic role using the scope of effect.
	// The scope of effect determines which resources this role's privileges apply to.
	role, err := scopedaccess.ScopedRoleToRole(rsp.Role, key.scopeOfEffect)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO(fspmarshall/scopes): figure out how/when we want to support trait interpolation in scoped
	// roles. When we do, that will likely need to be done here.

	// Create an access checker with this single role. Single-role evaluation is a core principle
	// of the scoped access model - the first role that permits access determines all parameters.
	checker := newAccessChecker(b.info, b.localCluster, NewRoleSet(role))

	return &ScopedAccessChecker{
		scopeOfOrigin:       key.scopeOfOrigin,
		scopeOfEffect:       key.scopeOfEffect,
		role:                rsp.Role,
		scopedCompatChecker: checker,
	}, nil
}

// newDefaultImplicitChecker builds a scoped access checker representing the default implicit role. We rely on the privileges conferred
// by the default implicit role always being "assigned" at root as if they came from a root scoped role assignment. We achieve this by
// creating a fake scoped access checker that wraps an "empty" unscoped access checker. Since all unscoped access checkers automatically
// include the default implicit role, this effectively simulates the presence of the default implicit role at root scope. Note that as
// functionality of scoped roles further diverges from unscoped roles, we may need to revisit this approach in favor of defining our
// own default implicit scoped role instead.
func (b *scopedAccessCheckerBuilder) newDefaultImplicitChecker(_ context.Context) *ScopedAccessChecker {
	return &ScopedAccessChecker{
		scopeOfOrigin:       scopes.Root,
		scopeOfEffect:       scopes.Root,
		scopedCompatChecker: newAccessChecker(b.info, b.localCluster, NewRoleSet()), // default implicit role definition is auto-populated by NewRoleSet()
		role: &scopedaccessv1.ScopedRole{
			Metadata: &headerv1.Metadata{
				Name: constants.DefaultImplicitRole,
			},
			Scope: scopes.Root,
			Spec: &scopedaccessv1.ScopedRoleSpec{
				AssignableScopes: []string{scopes.Root},
			},
			Version: types.V1,
		},
	}
}

// ScopedAccessChecker performs access checks abstracting over scoped and unscoped identities.
//
// For scoped identities, each ScopedAccessChecker represents a single role assignment characterized by:
//   - Scope of Origin: the scope from which the assignment originates (determines seniority)
//   - Scope of Effect: the scope at which the role's privileges apply (determines applicability)
//
// In the scoped access model, the first role (in evaluation order) that permits access to a resource
// determines all subsequent access parameters. This differs from classic role evaluation where roles are
// aggregated and the most restrictive settings win.
//
// For unscoped identities, the full AccessChecker is wrapped directly and all method calls are delegated to it.
//
// ScopedAccessChecker instances should be obtained from ScopedAccessCheckerContext rather than constructed
// directly. The exception is NewScopedAccessCheckerFromUnscoped for adapting an unscoped AccessChecker.
type ScopedAccessChecker struct {
	// scopeOfOrigin/scopeOfEffect are populated only for scoped identities; zero for unscoped.
	scopeOfOrigin string
	scopeOfEffect string

	// role is the scoped role being evaluated, or nil for unscoped identities.
	role *scopedaccessv1.ScopedRole

	// scopedCompatChecker is a classic AccessChecker built from the scoped role via ScopedRoleToRole.
	// Non-nil iff isScoped(). Used for checks that fall back to compat classic-role logic.
	scopedCompatChecker AccessChecker

	// unscopedChecker is the underlying unscoped AccessChecker.
	// Non-nil iff !isScoped().
	unscopedChecker AccessChecker
}

// NewScopedAccessCheckerFromUnscoped creates a ScopedAccessChecker wrapping an unscoped AccessChecker.
// This is used in code paths that accept *ScopedAccessChecker but operate on an unscoped identity.
func NewScopedAccessCheckerFromUnscoped(checker AccessChecker) *ScopedAccessChecker {
	return &ScopedAccessChecker{unscopedChecker: checker}
}

// isScoped reports whether this checker operates on a scoped identity.
func (c *ScopedAccessChecker) isScoped() bool {
	return c.role != nil
}

// SSH returns an SSH-specific access checker backed by this checker. All SSH-specific methods
// (logins, port forwarding, recording mode, idle timeout, etc.) live on [SSHAccessChecker].
func (c *ScopedAccessChecker) SSH() *SSHAccessChecker {
	return &SSHAccessChecker{checker: c}
}

// Kube returns a kube-specific access checker backed by this checker. All kube-specific methods
// (users, groups, idle timeout, etc.) live on [KubeAccessChecker].
func (c *ScopedAccessChecker) Kube() *KubeAccessChecker {
	return &KubeAccessChecker{checker: c}
}

// AccessInfo returns the AccessInfo that this access checker is based on.
func (c *ScopedAccessChecker) AccessInfo() *AccessInfo {
	if !c.isScoped() {
		return c.unscopedChecker.AccessInfo()
	}
	return c.scopedCompatChecker.AccessInfo()
}

// Traits returns the set of user traits.
func (c *ScopedAccessChecker) Traits() wrappers.Traits {
	// there is no concept of scoped traits currently, and none is planned or would be feasible at least
	// until we've fully migrated to PDP and deprecated certificate-based traits.
	if !c.isScoped() {
		return c.unscopedChecker.Traits()
	}
	return c.scopedCompatChecker.Traits()
}

// CheckAccessToRules verifies that *all* of a series of verbs are permitted for the specified resource.
func (c *ScopedAccessChecker) CheckAccessToRules(ctx RuleContext, resource string, verbs ...string) error {
	if !c.isScoped() {
		return checkAccessToRulesImpl(c.unscopedChecker, ctx, resource, verbs...)
	}
	return checkAccessToRulesImpl(c.scopedCompatChecker, ctx, resource, verbs...)
}

// CheckAccessToRemoteCluster checks access to a remote cluster.
func (c *ScopedAccessChecker) CheckAccessToRemoteCluster(cluster types.RemoteCluster) error {
	if !c.isScoped() {
		return c.unscopedChecker.CheckAccessToRemoteCluster(cluster)
	}
	// remote cluster access is never permitted for scoped identities.
	// NOTE: it is unclear whether or not this method should even be implemented for the scoped access checker. it may be more
	// sensible to force outer enforcement logic to grapple with the fact that a scoped checker does not support remote clusters
	// at the type-level. this has been implemented experimentally to explore the pattern of having the scoped access checker
	// implement methods that always deny for unsupported features.
	return trace.AccessDenied("remote cluster access is not permitted for scoped identities")
}

// AdjustSessionTTL will reduce the requested ttl to the lowest max allowed TTL for this role set.
func (c *ScopedAccessChecker) AdjustSessionTTL(ttl time.Duration) time.Duration {
	// the naive implementation of this method for scopes may have problematic interactions with
	// cert parameter generation. see ../scopes/access/compat.go for more detailed discussion.
	if !c.isScoped() {
		return c.unscopedChecker.AdjustSessionTTL(ttl)
	}
	return c.scopedCompatChecker.AdjustSessionTTL(ttl)
}

// PrivateKeyPolicy returns the enforced private key policy, or the provided default, whichever is stricter.
func (c *ScopedAccessChecker) PrivateKeyPolicy(defaultPolicy keys.PrivateKeyPolicy) (keys.PrivateKeyPolicy, error) {
	// the naive implementation of this method for scopes may have problematic interactions with
	// cert parameter generation. see ../scopes/access/compat.go for more detailed discussion.
	if !c.isScoped() {
		return c.unscopedChecker.PrivateKeyPolicy(defaultPolicy)
	}
	return c.scopedCompatChecker.PrivateKeyPolicy(defaultPolicy)
}

// PinSourceIP returns whether source IP pinning is enforced.
func (c *ScopedAccessChecker) PinSourceIP() bool {
	// the naive implementation of this method for scopes may have problematic interactions with
	// cert parameter generation. see ../scopes/access/compat.go for more detailed discussion.
	if !c.isScoped() {
		return c.unscopedChecker.PinSourceIP()
	}
	return c.scopedCompatChecker.PinSourceIP()
}

// LockingMode returns the locking mode to apply.
func (c *ScopedAccessChecker) LockingMode(defaultMode constants.LockingMode) constants.LockingMode {
	// the naive implementation of this method for scopes may have problematic interactions with
	// cert parameter generation. see ../scopes/access/compat.go for more detailed discussion.
	if !c.isScoped() {
		return c.unscopedChecker.LockingMode(defaultMode)
	}
	return c.scopedCompatChecker.LockingMode(defaultMode)
}

// checkAccessToRulesImpl verifies that *all* of a series of verbs are permitted for the specified resource. This
// function differs from AccessChecker.CheckAccessToRule in that it does not support advanced context-based features
// or namespacing, and accepts a set of verbs all of which must evaluate to allow for the check to succeed.
func checkAccessToRulesImpl(checker AccessChecker, ctx RuleContext, resource string, verbs ...string) error {
	if len(verbs) == 0 {
		return trace.BadParameter("malformed rule check for %q, no verbs provided (this is a bug)", resource)
	}
	for _, verb := range verbs {
		if err := checker.CheckAccessToRule(ctx, apidefaults.Namespace, resource, verb); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// checkMaybeHasAccessToRulesImpl returns an error if the checker definitely does not have access to the provided rules.
func checkMaybeHasAccessToRulesImpl(checker AccessChecker, ctx RuleContext, resource string, verbs ...string) error {
	if len(verbs) == 0 {
		return trace.BadParameter("malformed maybe has access to rule check for %q, no verbs provided (this is a bug)", resource)
	}
	for _, verb := range verbs {
		if err := checker.GuessIfAccessIsPossible(ctx, apidefaults.Namespace, resource, verb); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}
