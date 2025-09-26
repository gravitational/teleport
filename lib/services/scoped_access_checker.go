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
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/scopes"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
	"github.com/gravitational/teleport/lib/scopes/pinning"
	"github.com/gravitational/teleport/lib/utils/once"
)

// ScopedAccessCheckerContext is the top-level scoped access checker state. It builds and caches scoped access
// checkers "at" specific scopes, based on a user's identity.
type ScopedAccessCheckerContext struct {
	builder scopedAccessCheckerBuilder
	// cachedCheckerForScope wraps builder.newCheckerAtScope with a [once.KeyedValue] to retain previously built
	// checkers. this retention behavior is intended to support APIs that require multiple separate scoped
	// access checks to be performed for separate resources within the same request.
	cachedCheckerForScope func(ctx context.Context, scope string) (*ScopedAccessChecker, error)
}

// NewScopedAccessCheckerContext builds a scoped access checker context for a given identity. The context is
// used to build scoped access checkers at various scopes, and to evaluate access to resources within those
// scopes. Note that the supplied context.Context is captured and used to propagate cancellation during loading
// of scoped roles. Cancellation of the context while access checks are still in progress may result in
// spurious access denied errors.
func NewScopedAccessCheckerContext(ctx context.Context, info *AccessInfo, localCluster string, reader ScopedRoleReader) (*ScopedAccessCheckerContext, error) {
	builder := scopedAccessCheckerBuilder{
		info:         info,
		localCluster: localCluster,
		reader:       reader,
	}

	if err := builder.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	cachedCheckerForScope, _ := once.KeyedValue(builder.newCheckerAtScope)

	return &ScopedAccessCheckerContext{
		builder:               builder,
		cachedCheckerForScope: cachedCheckerForScope,
	}, nil
}

// CheckersForResourceScope returns a stream of scoped access checkers, descending from root to the specified resource
// scope. This is the mechanism that *must* be used for getting checkers when checking access to a resource. This
// method both evaluates immediate compliance of the resource scope with the scope pin, and yields correctly ordered
// checkers for resource access evaluation. Subsequent decision parameterization must be performed with the checker that
// yielded the initial allow decision.
func (c *ScopedAccessCheckerContext) CheckersForResourceScope(ctx context.Context, scope string) stream.Stream[*ScopedAccessChecker] {
	return c.checkersForResourceScope(ctx, scope, true /* enforce pin */)
}

// RiskyUnpinnedCheckersForResourceScope returns a stream of scoped access checkers, descending from root to the specified
// resource scope, but does not enforce the pinning scope. This is a risky operation that should only be used for certain APIs
// that make an exception to pinning exclusion rules (e.g. allowing read operations to succeed for resources in a parent to the
// pinned scope).
func (c *ScopedAccessCheckerContext) RiskyUnpinnedCheckersForResourceScope(ctx context.Context, scope string) stream.Stream[*ScopedAccessChecker] {
	// this method is a risky variant of CheckersForResourceScope that does not enforce the pinning scope, and should only be used
	// in contexts where the caller is certain that the resource scope is compatible with the pinning scope.
	return c.checkersForResourceScope(ctx, scope, false /* enforce pin */)
}

func (c *ScopedAccessCheckerContext) checkersForResourceScope(ctx context.Context, scope string, enforcePin bool) stream.Stream[*ScopedAccessChecker] {
	return func(yield func(*ScopedAccessChecker, error) bool) {
		// deny immediately if the resource scope is not subject to the pinned scope. note that this denial isn't just an
		// optimization, we have to perform this check separately from whatever access checks are performed by particular
		// checkers. This is vital as the pin scope itself may deny access to a resource that would be permitted by any
		// particular checker. For example, if a user has a scoped role assigned at /foo which grants access to all ssh
		// nodes, but they are pinned to scope /foo/bar, the checker at /foo needs to permit access to all nodes subject to
		// /foo, but the pin only permits access to resources subject to /foo/bar, even if the final allow decision is
		// determined by a checker at /foo.
		if enforcePin && !pinning.PinAppliesToResourceScope(c.builder.info.ScopePin, scope) {
			yield(nil, trace.AccessDenied("scope pin %q does not apply to resource scope %q", c.builder.info.ScopePin.GetScope(), scope))
			return
		}

		for checkerScope := range scopes.DescendingScopes(scope) {
			checker, err := c.cachedCheckerForScope(ctx, checkerScope)
			if err != nil {
				yield(nil, trace.Wrap(err))
				return
			}

			if !yield(checker, nil) {
				return
			}
		}
	}
}

// DoScopedDecision is a helper function that takes care of boilerplate for simple scoped decisions.  It calls the provided
// decision function until one of three conditions is met:
//
// 1. Decision function executes without error (allow)
// 2. Decision function returns an AccessExplicitlyDenied error (explicit deny)
// 3. Checker stream is exhausted (implicit deny)
//
// The generic parameter may be used to propagate additional information/parameterization of the decision. For basic checks,
// conventionally set D to struct{} to indicate that the presence/absence of the error alone is the entire decision state.
// For logging/debuggin purposes, returning checker.Scope() as the decision value may be convenient.
func DoScopedDecision[D any](checkers stream.Stream[*ScopedAccessChecker], decisionFn func(*ScopedAccessChecker) (D, error)) (D, error) {
	for checker, err := range checkers {
		if err != nil {
			return *new(D), trace.Wrap(err)
		}

		result, err := decisionFn(checker)
		switch {
		case err == nil:
			return result, nil
		case IsAccessExplicitlyDenied(err):
			return *new(D), trace.Wrap(err)
		default:
			// implicit deny, continue to the next check
			continue
		}
	}

	return *new(D), trace.AccessDenied("access denied (scoped)")
}

// scopedAccessCheckerBuilder is a helper that builds scoped access checkers.
type scopedAccessCheckerBuilder struct {
	info         *AccessInfo
	localCluster string
	reader       ScopedRoleReader
}

// Check verifies that the builder was provided will all necessary parameters and that they are well-formed.
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

	if len(b.info.AllowedResourceIDs) != 0 {
		return trace.BadParameter("cannot create scoped access checkers for identity with active resource IDs")
	}

	// validate that the scope pin is well-formed
	if err := pinning.WeakValidate(b.info.ScopePin); err != nil {
		return trace.Errorf("cannot create scoped access checkers: %w", err)
	}

	return nil
}

func (b *scopedAccessCheckerBuilder) newCheckerAtScope(ctx context.Context, scope string) (*ScopedAccessChecker, error) {
	// TODO(fspmarshall/scopes): implement the ability to pull already loaded roles from parent checkers
	// in order to reduce redunant role fetches.

	// validate that the target scope is compatible with the scope pin. note that this check is conceptually
	// distinct from checking whether or not the pinned scope permits access to resources at the target scope.
	// scope pinning rules disallow access to resources that are not equivalent or descendant to the pinned
	// scope, but during access evaluation we must create checkers for all parent scopes as well. this check
	// just verifies that we aren't creating a checker for an *orthogonal* scope. a full access-control check
	// implementation must use [ScopedAccessCheckerContext.CheckersForResourceScope] in order to ensure that
	// pinning is properly enforced for the resource scope.
	if !pinning.PinCompatibleWithPolicyScope(b.info.ScopePin, scope) {
		return nil, trace.AccessDenied("an identity pinned to scope %q cannot inform access decisions at scope %q", b.info.ScopePin.GetScope(), scope)
	}

	// fetch all assigned scoped roles and convert them to classic roles to drive the underlying access checks.
	// we need to use the unchecked variant of assignments for resource scope because the standard variant
	// refuses to produce an iterator for a scope that is a parent of the pinned scope, and this method will
	// get called to build parent checkers as part of legitimate access checks. it is the responsibility of the
	// caller of this method to determine that it is being used in a context that respects correct pinning rules.
	var roles []types.Role
	for scope, assigned := range pinning.AssignmentsForResourceScopeUnchecked(b.info.ScopePin, scope) {
		if len(assigned.GetRoles()) == 0 {
			continue
		}

		rolesForScope, err := fetchAndConvertScopedRoles(ctx, scope, assigned.GetRoles(), b.reader)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		roles = append(roles, rolesForScope...)
	}

	checker := newAccessChecker(b.info, b.localCluster, NewRoleSet(roles...))

	return &ScopedAccessChecker{
		scope:   scope,
		checker: checker,
	}, nil
}

// ScopedAccessChecker is similar to AccessChecker, but performs scoped checks. Note that usage patterns differ with
// scoped access checks in comparison to standard access checkers since scoped access checkers can be set up "at"
// various scopes, and the scope of subsequent checks may depend on the scope at which an allow was reached in a
// previous check. For example, if an allow decision was reached for ssh access at a particular node, then checks for
// parameters (e.g. x11 forwarding) must be performed at that same scope, even if that scope is a parent of the
// node's scope rather than the node's scope itself. On the other hand, a ListNodes operation should have read
// permssions for each individual node evaluated starting from the root scope, and descending to the node's scope if
// necessary. In general, a decision *about* access to a resource always happens at the highest scope along the
// resource's scope path at which primary access to the resource is allowed.
type ScopedAccessChecker struct {
	scope   string
	checker *accessChecker
}

// RiskyNewScopedAccessCheckerAtScope creates a scoped access checker at the specified scope. Note that creating a scoped
// access checker at a specific target scope rather than using [ScopedAccessCheckerContext] is typically incorrect as it
// bypasses scope pin enforcement. This function exists solely to support a few niche use-cases where standard scope pinning
// behavior is not applicable.
func RiskyNewScopedAccessCheckerAtScope(ctx context.Context, scope string, info *AccessInfo, localCluster string, reader ScopedRoleReader) (*ScopedAccessChecker, error) {
	builder := scopedAccessCheckerBuilder{
		info:         info,
		localCluster: localCluster,
		reader:       reader,
	}

	if err := builder.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	return builder.newCheckerAtScope(ctx, scope)
}

// Scope returns the scope "at" which the checker was created. A scoped access checker created at a given scope includes
// all scoped roles assigned to the user at that scope, and all ancestral scopes.
func (c *ScopedAccessChecker) Scope() string {
	return c.scope
}

// ScopePin returns the scope pin that this checker was created from.
func (c *ScopedAccessChecker) ScopePin() *scopesv1.Pin {
	return c.checker.info.ScopePin
}

// Traits returns the set of user traits.
func (c *ScopedAccessChecker) Traits() wrappers.Traits {
	// identical in scoped/unscoped contexts generally (there is no concept of
	// scoped traits currently, and none is planned or would be feasible at least
	// until we've fully migrated to PDP and deprecated certificate-based traits).
	return c.checker.Traits()
}

// CheckAccessToRules verifies that *all* of a series of verbs are permitted for the specified resource. This function differs
// from the unscoped AccessChecker.CheckAccessToRule in a number of ways. It does not support advanced context-based features
// or namespacing, and accepts a set of verbs all of which must evaluate to allow for the check to succeed.
func (c *ScopedAccessChecker) CheckAccessToRules(resource string, verbs ...string) error {
	if len(verbs) == 0 {
		return trace.AccessDenied("malformed rule check, no verbs provided (this is a bug)")
	}

	// scoped roles currently do not support any rule-context features.
	ctx := &Context{}

	for _, verb := range verbs {
		if err := c.checker.CheckAccessToRule(ctx, apidefaults.Namespace, resource, verb); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// CheckAccessToRemoteCluster checks access to remote cluster
func (c *ScopedAccessChecker) CheckAccessToRemoteCluster(cluster types.RemoteCluster) error {
	// remote cluster access is never permitted for scoped identities
	// NOTE: it is unclear whether or not this method should even be implemented for the scoped access checker. it may be more
	// sensible to force outer enforcement logic to grapple with the fact that a scoped checker does not support remote clusters
	// at the type-level. this has been implemented experimentally to explore the pattern of having the scoped access checker
	// implement methods that always deny for unsupported features.
	return trace.AccessDenied("remote cluster access is not permitted for scoped identities")
}

// CheckLoginDuration checks if role set can login up to given duration and
// returns a combined list of allowed logins.
func (c *ScopedAccessChecker) CheckLoginDuration(ttl time.Duration) ([]string, error) {
	// the naive implementation of this method for scopes may have problematic interactions with
	// cert parameter generation. see ../scopes/access/compat.go for more detailed discussion.
	return c.checker.CheckLoginDuration(ttl)
}

// AdjustSessionTTL will reduce the requested ttl to lowest max allowed TTL
// for this role set, otherwise it returns ttl unchanged
func (c *ScopedAccessChecker) AdjustSessionTTL(ttl time.Duration) time.Duration {
	// the naive implementation of this method for scopes may have problematic interactions with
	// cert parameter generation. see ../scopes/access/compat.go for more detailed discussion.
	return c.checker.AdjustSessionTTL(ttl)
}

// PrivateKeyPolicy returns the enforced private key policy for this role set,
// or the provided defaultPolicy - whichever is stricter.
func (c *ScopedAccessChecker) PrivateKeyPolicy(defaultPolicy keys.PrivateKeyPolicy) (keys.PrivateKeyPolicy, error) {
	// the naive implementation of this method for scopes may have problematic interactions with
	// cert parameter generation. see ../scopes/access/compat.go for more detailed discussion.
	return c.checker.PrivateKeyPolicy(defaultPolicy)
}

// PinSourceIP forces the same client IP for certificate generation and SSH usage
func (c *ScopedAccessChecker) PinSourceIP() bool {
	// the naive implementation of this method for scopes may have problematic interactions with
	// cert parameter generation. see ../scopes/access/compat.go for more detailed discussion.
	return c.checker.PinSourceIP()
}

// CanPortForward returns true if this RoleSet can forward ports.
func (c *ScopedAccessChecker) CanPortForward() bool {
	// the naive implementation of this method for scopes may have problematic interactions with
	// cert parameter generation. see ../scopes/access/compat.go for more detailed discussion.

	// NOTE: internally this method relies upon the SSHPortForwardMode() method. if future work
	// on the scoped access checker causes us to change the behavior of that method, we will
	// need to rework this method as well to ensure that it behaves consistently.
	return c.checker.CanPortForward()
}

// CanForwardAgents returns true if this role set offers capability to forward
// agents.
func (c *ScopedAccessChecker) CanForwardAgents() bool {
	// the naive implementation of this method for scopes may have problematic interactions with
	// cert parameter generation. see ../scopes/access/compat.go for more detailed discussion.
	return c.checker.CanForwardAgents()
}

// PermitX11Forwarding returns true if this RoleSet allows X11 Forwarding.
func (c *ScopedAccessChecker) PermitX11Forwarding() bool {
	// the naive implementation of this method for scopes may have problematic interactions with
	// cert parameter generation. see ../scopes/access/compat.go for more detailed discussion.
	return c.checker.PermitX11Forwarding()
}

// LockingMode returns the locking mode to apply with this checker.
func (c *ScopedAccessChecker) LockingMode(defaultMode constants.LockingMode) constants.LockingMode {
	// the naive implementation of this method for scopes may have problematic interactions with
	// cert parameter generation. see ../scopes/access/compat.go for more detailed discussion.
	return c.checker.LockingMode(defaultMode)
}

// AccessInfo returns the AccessInfo that this access checker is based on.
func (c *ScopedAccessChecker) AccessInfo() *AccessInfo {
	return c.checker.info
}

// fetchAndConvertScopedRoles fetches scoped roles by name and converts them to classic roles.
func fetchAndConvertScopedRoles(ctx context.Context, scope string, names []string, reader ScopedRoleReader) ([]types.Role, error) {
	roles := make([]types.Role, 0, len(names))
	for _, name := range names {
		rsp, err := reader.GetScopedRole(ctx, &scopedaccessv1.GetScopedRoleRequest{
			Name: name,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		role, err := scopedaccess.ScopedRoleToRole(rsp.Role, scope)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// TODO(fspmarshall): figure out how/when we want to support trait interpolation in scoped
		// roles. When we do, that will likely need to be done here. Wether we should perform trait
		// interpolation on the per-conversion scoped role and add support piecemeal, or inherit
		// identical trait interpolation from classic role behavior isn't clear yet, so at this
		// stage we're just opting to skip entirely.

		roles = append(roles, role)
	}

	return roles, nil
}
