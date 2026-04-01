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
	"log/slog"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
	decisionpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/decision/v1alpha1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
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

// ErrScopedIdentity is returned when a component intended for use only with unscoped identities receives a scoped
// identity. Methods that implement scoping support may check for this error and fallback to scoped authorization
// as appropriate.
var ErrScopedIdentity = &trace.AccessDeniedError{
	Message: "scoped identities not supported",
}

// roleCheckerKey identifies a unique single-role checker configuration by the combination of
// scope of origin, scope of effect, and role name.
type roleCheckerKey struct {
	scopeOfOrigin string
	scopeOfEffect string
	roleName      string
}

// defaultImplicitRoleKey is a sentinel key used to identify the default implicit role checker in caches.
var defaultImplicitRoleKey = roleCheckerKey{}

// ScopedAccessCheckerContext is the top-level scoped access checker state. It builds and caches scoped access
// checkers for individual roles based on a user's identity and scope hierarchy.
type ScopedAccessCheckerContext struct {
	builder scopedAccessCheckerBuilder
	// cachedCheckerForRole wraps builder.newCheckerForRole with a [once.KeyedValue] to retain previously built
	// checkers. Checkers are cached by (scopeOfOrigin, scopeOfEffect, roleName) to support efficient reuse
	// across multiple access checks within the same request.
	cachedCheckerForRole func(ctx context.Context, key roleCheckerKey) (*ScopedAccessChecker, error)
}

// NewScopedAccessCheckerContext builds a scoped access checker context for a given identity. The context is
// used to build scoped access checkers for individual roles, and to evaluate access to resources using
// single-role evaluation semantics. Note that the supplied context.Context is captured and used to propagate
// cancellation during loading of scoped roles. Cancellation of the context while access checks are still in
// progress may result in spurious access denied errors.
func NewScopedAccessCheckerContext(ctx context.Context, info *AccessInfo, localCluster string, reader ScopedRoleReader) (*ScopedAccessCheckerContext, error) {
	builder := scopedAccessCheckerBuilder{
		info:         info,
		localCluster: localCluster,
		reader:       reader,
	}

	if err := builder.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	// Cache checkers by (scopeOfOrigin, scopeOfEffect, roleName) tuple for efficient reuse
	cachedCheckerForRole, _ := once.KeyedValue(builder.newCheckerForRole)

	return &ScopedAccessCheckerContext{
		builder:              builder,
		cachedCheckerForRole: cachedCheckerForRole,
	}, nil
}

// ScopePin returns the scope pin that this context was created from.
func (c *ScopedAccessCheckerContext) ScopePin() *scopesv1.Pin {
	return c.builder.info.ScopePin
}

// CheckersForResourceScope returns a stream of scoped access checkers in evaluation order for the specified
// resource scope. Each checker represents a single role assignment, ordered first by scope of origin (ancestral
// to descendant) and then by scope of effect (descendant to ancestral within each origin). This ordering ensures
// that role evaluation follows the scoped role hierarchy rules where:
//  1. Roles assigned from more ancestral scopes take precedence (preserving scope hierarchy)
//  2. Within each origin, more specific role assignments take precedence (allowing specialization)
//  3. The first role that permits access determines all parameters (single-role evaluation)
//
// This is the mechanism that *must* be used for getting checkers when checking access to a resource. This method
// validates immediate compliance of the resource scope with the scope pin and yields correctly ordered checkers
// for resource access evaluation. Subsequent decision parameterization must be performed with the checker that
// yielded the initial allow decision.
func (c *ScopedAccessCheckerContext) CheckersForResourceScope(ctx context.Context, scope string) stream.Stream[*ScopedAccessChecker] {
	return c.checkersForResourceScope(ctx, scope, true /* enforce pin */)
}

// RiskyUnpinnedCheckersForResourceScope returns a stream of scoped access checkers for the specified resource
// scope, but does not enforce the pinning scope. This is a risky operation that should only be used for certain
// APIs that make an exception to pinning exclusion rules (e.g. allowing read operations to succeed for resources
// in a parent to the pinned scope).
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
		// particular role. For example, if a user has a scoped role assigned at /foo which grants access to all ssh
		// nodes, but they are pinned to scope /foo/bar, even if a role at /foo permits access, the pin restricts
		// access to only resources subject to /foo/bar.
		if enforcePin && !pinning.PinAppliesToResourceScope(c.builder.info.ScopePin, scope) {
			yield(nil, trace.AccessDenied("scope pin %q does not apply to resource scope %q", c.builder.info.ScopePin.GetScope(), scope))
			return
		}

		var successfullyResolved int
		var lastErr error

		defaultImplicitChecker, err := c.cachedCheckerForRole(ctx, defaultImplicitRoleKey)
		if err != nil {
			slog.WarnContext(ctx, "skipping default implicit role evaluation due to error", "error", err)
			lastErr = err
		} else {
			// yield the default implicit role checker first. This simulates the presence of the default implicit
			// role at root scope, ensuring that its privileges are always considered first in evaluation.
			if !yield(defaultImplicitChecker, nil) {
				return
			}

			// note that we are not incrementing successfullyResolved here. the default implicit role doesn't
			// really count from the perspective of deciding wether or not we're hitting a systemic failure.
		}

		// iterate through the ordered enforcement points for this resource scope. policy evaluation by scope is ordered first by
		// Scope of Origin (ancestral to descendant) and then by Scope of Effect (descendant to ancestral within each origin).
		// We proceed through each permutation in order, evaluating any roles assigned at that specific point.
		for point := range scopes.EnforcementPointsForResourceScope(scope) {
			// get all roles assigned at this (scopeOfOrigin, scopeOfEffect) pair
			for roleName := range pinning.GetRolesAtEnforcementPoint(c.builder.info.ScopePin, point) {
				// create/retrieve cached checker for this specific role
				key := roleCheckerKey{
					scopeOfOrigin: point.ScopeOfOrigin,
					scopeOfEffect: point.ScopeOfEffect,
					roleName:      roleName,
				}

				checker, err := c.cachedCheckerForRole(ctx, key)
				if err != nil {
					// in classic teleport access checking skipping a role would be unacceptable due to side effects and deny rules. the scoped model
					// however relies on cross-role isolation and explicitly allows omission of roles.
					slog.WarnContext(ctx, "skipping role evaluation due to error", "role_name", roleName, "scope_of_origin", point.ScopeOfOrigin, "scope_of_effect", point.ScopeOfEffect, "error", err)
					lastErr = err
					continue
				}

				if !yield(checker, nil) {
					return
				}

				successfullyResolved++
			}
		}

		if successfullyResolved == 0 && lastErr != nil {
			// if we didn't successfully build any assignment-derived checkers and encountered errors, return the last error encountered
			// as it may be indicative of some kind of systemic failure rather than a problem with a specific assignment.
			yield(nil, lastErr)
		}
	}
}

// RiskyEnumerateCheckers returns a stream of all possible scoped access checkers for the identity. This method
// enumerates every role assignment in the pin's assignment tree, yielding a checker for each one. The order is
// undefined and should not be relied upon for access control decisions.
//
// This method is not relevant for traditional access-control decisions as it yields checkers unrelated to any
// particular resource scope, but is necessary for examining the full set of possible permissions during certain
// operations, such as when determining the full set of ssh logins that a user might have access to. Note that use
// of this method should be treated with extreme caution. Accidental misuse could easily result in a scope isolation
// violation.
func (c *ScopedAccessCheckerContext) RiskyEnumerateCheckers(ctx context.Context) stream.Stream[*ScopedAccessChecker] {
	return func(yield func(*ScopedAccessChecker, error) bool) {
		// enumerate all role assignments in the entire pin, including assignments at scopes
		// descendant to the pinned scope. This provides the complete set of possible permissions.
		var yielded int
		var lastErr error
		for assignment := range pinning.EnumerateAllAssignments(c.builder.info.ScopePin) {
			// create/retrieve cached checker for this specific role
			key := roleCheckerKey{
				scopeOfOrigin: assignment.ScopeOfOrigin,
				scopeOfEffect: assignment.ScopeOfEffect,
				roleName:      assignment.RoleName,
			}

			checker, err := c.cachedCheckerForRole(ctx, key)
			if err != nil {
				slog.WarnContext(ctx, "skipping role evaluation due to error", "role_name", assignment.RoleName, "scope_of_origin", assignment.ScopeOfOrigin, "scope_of_effect", assignment.ScopeOfEffect, "error", err)
				continue
			}

			if !yield(checker, nil) {
				return
			}

			yielded++
		}

		if yielded == 0 && lastErr != nil {
			// if we didn't yield any checkers and encountered errors, return the last error encountered.
			yield(nil, lastErr)
		}
	}
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

	if len(b.info.AllowedResourceAccessIDs) != 0 {
		return trace.BadParameter("cannot create scoped access checkers for identity with active resource IDs")
	}

	// validate that the scope pin is well-formed
	if err := pinning.WeakValidate(b.info.ScopePin); err != nil {
		return trace.Errorf("cannot create scoped access checkers: %w", err)
	}

	return nil
}

func (b *scopedAccessCheckerBuilder) newCheckerForRole(ctx context.Context, key roleCheckerKey) (*ScopedAccessChecker, error) {
	if key == defaultImplicitRoleKey {
		return b.newDefaultImplicitChecker(ctx), nil
	}
	// fetch the scoped role by name
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
	// roles. When we do, that will likely need to be done here. Whether we should perform trait
	// interpolation on the per-conversion scoped role and add support piecemeal, or inherit
	// identical trait interpolation from classic role behavior isn't clear yet, so at this
	// stage we're just opting to skip entirely.

	// Create an access checker with this single role. Single-role evaluation is a core principle
	// of the scoped access model - the first role that permits access determines all parameters.
	checker := newAccessChecker(b.info, b.localCluster, NewRoleSet(role))

	return &ScopedAccessChecker{
		scopeOfOrigin: key.scopeOfOrigin,
		scopeOfEffect: key.scopeOfEffect,
		checker:       checker,
		role:          rsp.Role,
	}, nil
}

// newDefaultImplicitChecker builds a scoped access checker representing the default implicit role. We rely on the privileges conferred
// by the default implicit role always being "assigned" at root as if they came from a root scoped role assignment. We achieve this by
// creating a fake scoped access checker that wraps an "empty" unscoped access checker. Since all unscoped access checkers automatically
// include the default implicit role, this effectively simulates the presence of the default implicit role at root scope. Note that as
// functionality of scoped roles further diverges from unscoped roles, we may need to revisit this approach in favor of defining our
// own default implicit scoped role instead.
func (b *scopedAccessCheckerBuilder) newDefaultImplicitChecker(ctx context.Context) *ScopedAccessChecker {
	return &ScopedAccessChecker{
		scopeOfOrigin: scopes.Root,
		scopeOfEffect: scopes.Root,
		checker:       newAccessChecker(b.info, b.localCluster, NewRoleSet()), // default implicit role definition is auto-populated by NewRoleSet()
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

// ScopedAccessChecker is similar to AccessChecker, but performs scoped checks using single-role evaluation
// semantics. Each ScopedAccessChecker represents a single role assignment characterized by:
//   - Scope of Origin: The scope from which the role assignment originates (determines authority/seniority)
//   - Scope of Effect: The scope at which the role's privileges apply (determines applicability)
//   - Role Name: The specific role being evaluated
//
// In the scoped access model, the first role (in evaluation order) that permits access to a resource determines
// all subsequent access parameters. This differs from classic role evaluation where roles are aggregated and
// the most restrictive settings win. For parameter checks (e.g. x11 forwarding, port forwarding), the same
// checker instance that yielded the initial allow decision must be used to maintain consistency.
type ScopedAccessChecker struct {
	// scopeOfOrigin is the scope from which this role assignment originates
	scopeOfOrigin string
	// scopeOfEffect is the scope at which this role's privileges apply
	scopeOfEffect string
	// checker is the underlying classic access checker with this single role
	checker *accessChecker
	// role is the original scoped role being evaluated
	role *scopedaccessv1.ScopedRole
}

// ScopeOfOrigin returns the scope from which this role assignment originates. Roles assigned from
// more ancestral scopes take precedence over roles assigned from more descendant scopes.
func (c *ScopedAccessChecker) ScopeOfOrigin() string {
	return c.scopeOfOrigin
}

// ScopeOfEffect returns the scope at which this role's privileges apply. Within a given scope of
// origin, roles with more descendant/specific scopes of effect take precedence over roles with
// more ancestral/general scopes of effect.
func (c *ScopedAccessChecker) ScopeOfEffect() string {
	return c.scopeOfEffect
}

// RoleName returns the name of the role being evaluated by this checker.
func (c *ScopedAccessChecker) RoleName() string {
	return c.role.GetMetadata().GetName()
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

// CheckAccessToRules verifies that *all* of a series of verbs are permitted for the specified resource.
func (c *ScopedAccessChecker) CheckAccessToRules(ctx RuleContext, resource string, verbs ...string) error {
	return checkAccessToRulesImpl(c.checker, ctx, resource, verbs...)
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

// GetSSHLogins returns the list of all SSH logins permitted by this scoped role.
func (c *ScopedAccessChecker) GetSSHLogins() []string {
	return c.role.GetSpec().GetAllow().GetLogins()
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

// HostSudoers returns the sudoers rules for the host.
func (c *ScopedAccessChecker) HostSudoers(srv types.Server) ([]string, error) {
	// scoped roles do not currently support host sudoers, but we don't currently foresee
	// issues with mirroring the classic role interface here since host sudoers are not
	// certificate-bound and are not calculated pre-access-check. depeding on wether or not
	// we end up permitting scoped roles side-effects, the server parameter may be unnecessary.
	return c.checker.HostSudoers(srv)
}

// EnhancedRecordingSet returns the set of enhanced session recording
// events to capture.
func (c *ScopedAccessChecker) EnhancedRecordingSet() map[string]bool {
	// scoped roles do not currently support enhanced session recording, but we don't currently
	// foresee issues with mirroring the classic role interface here since enhanced session
	// recording settings are not certificate-bound and are not calculated pre-access-check.
	return c.checker.EnhancedRecordingSet()
}

// HostUsers returns host user information matching a server or nil if
// a role disallows host user creation
func (c *ScopedAccessChecker) HostUsers(srv types.Server) (*decisionpb.HostUsersInfo, error) {
	// scoped roles do not currently support host users, but we don't currently foresee
	// issues with mirroring the classic role interface here since host users are not
	// certificate-bound and are not calculated pre-access-check. depeding on wether or not
	// we end up permitting scoped roles side-effects, the server parameter may be unnecessary.
	return c.checker.HostUsers(srv)
}

// CheckAgentForward checks if the role can request to forward the SSH agent
// for this user.
func (c *ScopedAccessChecker) CheckAgentForward(login string) error {
	// scoped roles do not currently support agent forwarding, but we don't currently foresee
	// issues with mirroring the classic role interface for the login-dependant variant of the
	// check since this variant of the check is not related to the certificate-bound agent forwarding
	// permission, and is not calculated pre-access-check. depeding on wether or not
	// we end up permitting scoped roles side-effects, the login parameter may be unnecessary.
	return c.checker.CheckAgentForward(login)
}

// MaxConnections returns the maximum number of concurrent ssh connections
// allowed.  If MaxConnections is zero then no maximum was defined
// and the number of concurrent connections is unconstrained.
func (c *ScopedAccessChecker) MaxConnections() int64 {
	// scoped roles do not currently support max connections, but we don't currently foresee
	// issues with mirroring the classic role interface here since max connections is not
	// certificate-bound and is not calculated pre-access-check.
	return c.checker.MaxConnections()
}

// MaxSessions returns the maximum number of concurrent ssh sessions
// per connection.  If MaxSessions is zero then no maximum was defined
// and the number of sessions is unconstrained.
func (c *ScopedAccessChecker) MaxSessions() int64 {
	// scoped roles do not currently support max sessions, but we don't currently foresee
	// issues with mirroring the classic role interface here since max sessions is not
	// certificate-bound and is not calculated pre-access-check.
	return c.checker.MaxSessions()
}

// CanCopyFiles returns true if the role set has enabled remote file
// operations via SCP or SFTP.
func (c *ScopedAccessChecker) CanCopyFiles() bool {
	// scoped roles do not currently support remote file operations, but we don't currently foresee
	// issues with mirroring the classic role interface here since remote file operation permission
	// is not certificate-bound and is not calculated pre-access-check.
	return c.checker.CanCopyFiles()
}

// SSHPortForwardMode returns the SSHPortForwardMode.
func (c *ScopedAccessChecker) SSHPortForwardMode() decisionpb.SSHPortForwardMode {
	// scoped roles do not currently support port forwarding modes, but we don't currently foresee
	// issues with mirroring the classic role interface here sine this method is not certificate-bound
	// and is not calculated pre-access-check. note that due to the fact that the more general
	// CanPortForward() method does affect certificate parameters, this method's behavior is currently
	// determined by some hard-coding in ../scopes/access/compat.go. this method isn't currently useful
	// as a result, but resolution of questions around the port forwarding certificate parameter will
	// unblock this method.
	return c.checker.SSHPortForwardMode()
}

// AdjustClientIdleTimeout adjusts requested idle timeout
// to the lowest max allowed timeout, the most restrictive
// option will be picked, negative values will be assumed as 0
func (c *ScopedAccessChecker) AdjustClientIdleTimeout(timeout time.Duration) time.Duration {
	// scoped roles do not currently support client idle timeouts, but we don't currently foresee
	// issues with mirroring the classic role interface here and this method is not used to
	// derive certificate parameters.  *however*, there are some usages of this method that
	// are not pre-access-check. This method can be used now for post-access-check adjustments,
	// but further work will need to be done to determine how we want to handle client idle
	// timeouts in the context of scoped roles. See ../scopes/access/compat.go for more
	// discussion.
	return c.checker.AdjustClientIdleTimeout(timeout)
}

// AdjustDisconnectExpiredCert adjusts the value based on the role set
// the most restrictive option will be picked
func (c *ScopedAccessChecker) AdjustDisconnectExpiredCert(disconnect bool) bool {
	// scoped roles do not currently support disconnect on expired certs, but we don't currently foresee
	// issues with mirroring the classic role interface here since this method is not used to
	// derive certificate parameters. *however*, there are some usages of this method that
	// are not pre-access-check. This method can be used now for post-access-check adjustments,
	// but further work will need to be done to determine how we want to handle disconnect on
	// expired certs in the context of scoped roles. See ../scopes/access/compat.go for more
	// discussion.
	return c.checker.AdjustDisconnectExpiredCert(disconnect)
}

// SessionRecordingMode returns the recording mode for a specific service.
func (c *ScopedAccessChecker) SessionRecordingMode(service constants.SessionRecordingService) constants.SessionRecordingMode {
	// scoped roles do not currently support session recording modes, but we don't currently foresee
	// issues with mirroring the classic role interface here since session recording mode is not
	// certificate-bound and is not calculated pre-access-check.
	return c.checker.SessionRecordingMode(service)
}

// CheckAccessToSSHServer checks access to an SSH server with optional role matchers. Note that this function
// is a thin wrapper around the standard [AccessChecker.CheckAccess] method. The purpose of this method is to
// provide a more constrained access-checking API since the majority of access-checkable resources are not
// supported by scopes yet.
func (c *ScopedAccessChecker) CheckAccessToSSHServer(target types.Server, state AccessState, osUser string) error {
	return c.checker.CheckAccess(
		target,
		state,
		NewLoginMatcher(osUser),
	)
}

// CanAccessSSHServer is a helper method that checkes whether access to the specified SSH server is possible.
// This method is used to determine read access to SSH servers, and does not take into account elements like
// MFA state or os login. This helper is based on the behavior of auth.resourceChecker.CanAccess. The purpose
// of this method is to provide a more constrained access-checking API since the majority of access-checkable
// resources are not supported by scopes yet.
func (c *ScopedAccessChecker) CanAccessSSHServer(target types.Server) error {
	return c.checker.CheckAccess(target, AccessState{MFAVerified: true})
}
