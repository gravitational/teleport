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

package services

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"

	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	dtauthz "github.com/gravitational/teleport/lib/devicetrust/authz"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/scopes/pinning"
	"github.com/gravitational/teleport/lib/sshca"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils/once"
)

// ScopedAccessCheckerContext is the top-level access checker state, abstracting over scoped and unscoped
// identities. For scoped identities it builds and caches per-role checkers based on the scope pin and role
// assignments or system roles. For unscoped identities it wraps a standard AccessChecker.
//
// User-vs-agent differences are fully captured at construction time in the checkersAtPoint closure.
// Once constructed, this type is uniform across both scoped identity kinds.
type ScopedAccessCheckerContext struct {
	// pin is the scope pin for this identity. Non-nil iff isScoped().
	pin *scopesv1.Pin

	// traits are the user traits for this context. Nil for agent pin identities.
	traits wrappers.Traits

	// resolveRef resolves a RoleAssignment to a ScopedAccessChecker. The zero-value RoleAssignment is
	// the preamble sentinel: user pins return the default implicit role checker, agent pins return nil.
	// Non-nil iff isScoped().
	resolveRef func(ctx context.Context, ref pinning.RoleAssignment) (*ScopedAccessChecker, error)

	// enumerateAll enumerates checkers across all role assignments, for cert parameter aggregation.
	// Non-nil only for user pins; nil for agent pins (cert param aggregation is not supported).
	enumerateAll func(ctx context.Context) stream.Stream[*ScopedAccessChecker]

	// unscopedChecker wraps a standard AccessChecker for unscoped identities.
	// Non-nil iff !isScoped().
	unscopedChecker AccessChecker
}

// NewScopedAccessCheckerContext builds a ScopedAccessCheckerContext for a scoped user identity.
func NewScopedAccessCheckerContext(ctx context.Context, info *AccessInfo, localCluster string, reader ScopedRoleReader) (*ScopedAccessCheckerContext, error) {
	builder := scopedAccessCheckerBuilder{
		info:         info,
		localCluster: localCluster,
		reader:       reader,
	}

	if err := builder.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	pin := info.ScopePin
	if pin.GetKind() != scopesv1.PinKind_PIN_KIND_USER {
		return nil, trace.BadParameter("cannot create user pin checker context for pin of kind %v", pin.GetKind())
	}

	cachedCheckerForRole, _ := once.KeyedValue(builder.newCheckerForRole)

	resolveRef := func(ctx context.Context, ref pinning.RoleAssignment) (*ScopedAccessChecker, error) {
		if ref == (pinning.RoleAssignment{}) {
			// preamble sentinel: return the default implicit role checker
			return cachedCheckerForRole(ctx, pinning.RoleAssignment{})
		}
		return cachedCheckerForRole(ctx, ref)
	}

	enumerateAll := func(ctx context.Context) stream.Stream[*ScopedAccessChecker] {
		return func(yield func(*ScopedAccessChecker, error) bool) {
			var yielded int
			var lastErr error
			for assignment := range pinning.EnumerateAllAssignments(pin) {
				checker, err := cachedCheckerForRole(ctx, assignment)
				if err != nil {
					slog.WarnContext(ctx, "skipping role evaluation due to error", "role_name", assignment.RoleName, "scope_of_origin", assignment.ScopeOfOrigin, "scope_of_effect", assignment.ScopeOfEffect, "error", err)
					lastErr = err
					continue
				}
				if !yield(checker, nil) {
					return
				}
				yielded++
			}
			if yielded == 0 && lastErr != nil {
				yield(nil, lastErr)
			}
		}
	}

	return &ScopedAccessCheckerContext{
		pin:          pin,
		traits:       info.Traits,
		resolveRef:   resolveRef,
		enumerateAll: enumerateAll,
	}, nil
}

// NewScopedAccessCheckerContextFromUnscoped builds a ScopedAccessCheckerContext wrapping an unscoped AccessChecker.
func NewScopedAccessCheckerContextFromUnscoped(checker AccessChecker) *ScopedAccessCheckerContext {
	return &ScopedAccessCheckerContext{unscopedChecker: checker}
}

// NewScopedAccessCheckerContextForAgentPin builds a ScopedAccessCheckerContext for a scoped agent identity.
// Each entry in checkersByRole maps a system role name to its [ScopedAccessChecker], which must have been
// built via [NewScopedAccessCheckerForSystemRole].
//
// System role checkers are resolved at the (root, root) enforcement point, reflecting that system role
// permissions are treated as assigned at root scope. Note that this is not necessarily a permanent design
// choice. Future iterations may choose to represent system role permissions as a combination of root assigned
// permissions and agent scope assigned permissions. There are pros and cons to either appoach, but we've opted
// to start with the simpler model for now.
func NewScopedAccessCheckerContextForAgentPin(pin *scopesv1.Pin, checkersByRole map[string]*ScopedAccessChecker) (*ScopedAccessCheckerContext, error) {
	if pin == nil {
		return nil, trace.BadParameter("cannot create scoped access checker context without agent pin")
	}
	if pin.GetKind() != scopesv1.PinKind_PIN_KIND_AGENT {
		return nil, trace.BadParameter("cannot create scoped access checker context for unexpected pin kind %v, expected %v", pin.GetKind(), scopesv1.PinKind_PIN_KIND_AGENT)
	}
	if len(checkersByRole) == 0 {
		return nil, trace.BadParameter("cannot create scoped access checker context without any system role checkers")
	}

	if err := pinning.WeakValidate(pin); err != nil {
		return nil, trace.Wrap(err)
	}

	resolveRef := func(ctx context.Context, ref pinning.RoleAssignment) (*ScopedAccessChecker, error) {
		if ref == (pinning.RoleAssignment{}) {
			// preamble sentinel: agent pins have no implicit role preamble
			return nil, nil
		}
		if ref.RoleKind != pinning.RoleKindSystem {
			return nil, trace.BadParameter("agent pin resolver received non-system role kind %q (this is a bug)", ref.RoleKind)
		}
		checker, ok := checkersByRole[ref.RoleName]
		if !ok {
			return nil, trace.BadParameter("no checker found for system role %q", ref.RoleName)
		}
		return checker, nil
	}

	return &ScopedAccessCheckerContext{
		pin:        pin,
		resolveRef: resolveRef,
	}, nil
}

// isScoped reports whether this context operates on a scoped identity.
func (c *ScopedAccessCheckerContext) isScoped() bool {
	return c.unscopedChecker == nil
}

// ScopePin returns the scope pin for the identity, if the identity is scoped (user or agent).
// Returns (nil, false) for unscoped identities.
func (c *ScopedAccessCheckerContext) ScopePin() (*scopesv1.Pin, bool) {
	return c.pin, c.pin != nil
}

// CheckersForResourceScope returns a stream of ScopedAccessCheckers in evaluation order for the given resource
// scope. For scoped identities, this enforces pin compliance and yields per-role checkers ordered by scope of
// origin (ancestral to descendant) then scope of effect (descendant to ancestral). For unscoped identities,
// yields a single checker wrapping the full unscoped context.
//
// This is the mechanism that *must* be used for getting checkers when checking access to a resource.
//
// Callers may pass an empty string ("") as scope to indicate an unscoped resource. This will not be treated as a
// root scope resource - i.e. identities with privileges assigned in the root scope will not be able to access the
// resource.
func (c *ScopedAccessCheckerContext) CheckersForResourceScope(ctx context.Context, scope string) stream.Stream[*ScopedAccessChecker] {
	if !c.isScoped() {
		return stream.Once(NewScopedAccessCheckerFromUnscoped(c.unscopedChecker))
	}

	const enforcePinTrue = true

	return c.checkersForResourceScope(ctx, scope, enforcePinTrue)
}

// riskyUnpinnedCheckersForResourceScope is equivalent to CheckersForResourceScope except that it bypasses
// enforcement of the pinning scope. This is a risky operation that should only be used for certain APIs that
// make an exception to pinning exclusion rules (e.g. allowing read operations for resources at a parent scope).
func (c *ScopedAccessCheckerContext) riskyUnpinnedCheckersForResourceScope(ctx context.Context, scope string) stream.Stream[*ScopedAccessChecker] {
	if !c.isScoped() {
		return stream.Once(NewScopedAccessCheckerFromUnscoped(c.unscopedChecker))
	}

	const enforcePinFalse = false

	return c.checkersForResourceScope(ctx, scope, enforcePinFalse)
}

func (c *ScopedAccessCheckerContext) checkersForResourceScope(ctx context.Context, scope string, enforcePin bool) stream.Stream[*ScopedAccessChecker] {
	return func(yield func(*ScopedAccessChecker, error) bool) {
		// deny immediately if the resource scope is not subject to the pinned scope. note that this denial isn't just an
		// optimization, we have to perform this check separately from whatever access checks are performed by particular
		// checkers. This is vital as the pin scope itself may deny access to a resource that would be permitted by any
		// particular role. For example, if a user has a scoped role assigned at /foo which grants access to all ssh
		// nodes, but they are pinned to scope /foo/bar, even if a role at /foo permits access, the pin restricts
		// access to only resources subject to /foo/bar.
		if enforcePin && !pinning.PinAppliesToResourceScope(c.pin, scope) {
			yield(nil, trace.AccessDenied("scope pin %q does not apply to resource scope %q", c.pin.GetScope(), scope))
			return
		}

		var successfullyResolved int
		var lastErr error

		// resolve and yield preamble checker if one exists. this step is necessary for user identities in order to
		// ensure that default implicit role permissions are always evaluated first and always priority equivalent
		// to a root scope of origin. agent identities do not require this step as they always have checkers with
		// a root scope of origin. Note that the preamble checker does not count toward successfullyResolved.
		if preambleChecker, err := c.resolveRef(ctx, pinning.RoleAssignment{}); err != nil {
			slog.WarnContext(ctx, "skipping default implicit role evaluation due to error", "error", err)
			lastErr = err
		} else if preambleChecker != nil {
			if !yield(preambleChecker, nil) {
				return
			}
		}

		// iterate through the ordered enforcement points for this resource scope. policy evaluation by scope is ordered first by
		// Scope of Origin (ancestral to descendant) and then by Scope of Effect (descendant to ancestral within each origin).
		// We proceed through each permutation in order, evaluating any roles assigned at that specific point.
		for point := range scopes.EnforcementPointsForResourceScope(scope) {
			for ref := range pinning.GetRolesAtEnforcementPoint(c.pin, point) {
				checker, err := c.resolveRef(ctx, ref)
				if err != nil {
					// in classic teleport access checking skipping a role would be unacceptable due to side effects and deny rules. the scoped model
					// however relies on cross-role isolation and explicitly allows omission of roles.
					slog.WarnContext(ctx, "skipping role evaluation due to error",
						"role_name", ref.RoleName,
						"scope_of_origin", ref.ScopeOfOrigin,
						"scope_of_effect", ref.ScopeOfEffect,
						"error", err)
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

// riskyEnumerateScopedCheckers returns a stream of all possible scoped access checkers for the identity,
// enumerating every role assignment in the pin's assignment tree. The order is undefined and must not be
// relied upon for access control decisions. This method panics if called on an unscoped context or an agent
// pin context — it is only meaningful for scoped user identities.
//
// Note that use of this method should be treated with extreme caution. Accidental misuse could easily
// result in a scope isolation violation.
func (c *ScopedAccessCheckerContext) riskyEnumerateScopedCheckers(ctx context.Context) stream.Stream[*ScopedAccessChecker] {
	if !c.isScoped() {
		panic("riskyEnumerateScopedCheckers called on an unscoped access checker context (this is a bug)")
	}
	if c.enumerateAll == nil {
		panic("riskyEnumerateScopedCheckers called on an agent pin context (this is a bug)")
	}
	return c.enumerateAll(ctx)
}

// CheckMaybeHasAccessToRules returns an error if the context definitely does not have access to the provided
// rules. For scoped identities, always returns nil — the scoped access model evaluates permissions per-resource.
func (c *ScopedAccessCheckerContext) CheckMaybeHasAccessToRules(ctx RuleContext, resource string, verbs ...string) error {
	if !c.isScoped() {
		return checkMaybeHasAccessToRulesImpl(c.unscopedChecker, ctx, resource, verbs...)
	}
	return nil
}

// Decision calls fn against each checker in the resource scope evaluation order until one of three
// conditions is met: (1) fn succeeds, (2) fn returns an explicitly denied error, or (3) all checkers
// have been exhausted (implicit deny).
func (c *ScopedAccessCheckerContext) Decision(ctx context.Context, scope string, fn func(*ScopedAccessChecker) error) error {
	return c.decision(c.CheckersForResourceScope(ctx, scope), fn)
}

func (c *ScopedAccessCheckerContext) decision(checkers stream.Stream[*ScopedAccessChecker], fn func(*ScopedAccessChecker) error) error {
	for checker, err := range checkers {
		if err != nil {
			return trace.Wrap(err)
		}
		err = fn(checker)
		switch {
		case err == nil:
			return nil
		case IsAccessExplicitlyDenied(err):
			return trace.Wrap(err)
		default:
			// implicit deny, continue to the next check
			continue
		}
	}
	return trace.AccessDenied("access denied (decision)")
}

// AccessStateFromSSHIdentity builds an AccessState from an SSH identity, abstracting over scoped and
// unscoped access state construction.
func (c *ScopedAccessCheckerContext) AccessStateFromSSHIdentity(ctx context.Context, ident *sshca.Identity, authPrefGetter AuthPreferenceGetter) (AccessState, error) {
	if !c.isScoped() {
		return AccessStateFromSSHIdentity(ctx, ident, c.unscopedChecker, authPrefGetter)
	}

	authPref, err := authPrefGetter.GetAuthPreference(ctx)
	if err != nil {
		return AccessState{}, trace.Wrap(err)
	}

	if authPref.GetRequireMFAType().IsSessionMFARequired() {
		// TODO(fspmarshall/scopes): implement scoped MFA
		// NOTE: this will require additional refactoring of relevant access-checking logic. currently, we often
		// check MFA requirements *before* we determine access to the underlying resource, but a scoped MFA model
		// will need to first determine the scope of access *before* we can determine whether MFA is required for that scope.
		return AccessState{}, trace.AccessDenied("cannot perform scoped access when cluster-level MFA is required (scoped MFA is not implemented)")
	}

	return AccessState{
		// MFA state is hard-coded here because scoped roles do not support MFA yet, and the above check should reject
		// cases where cluster-level config would obligate MFA.
		MFARequired:              MFARequiredNever,
		MFAVerified:              false,
		EnableDeviceVerification: true,
		DeviceVerified:           dtauthz.IsSSHDeviceVerified(ident),
		IsBot:                    ident.IsBot(),
	}, nil
}

// AccessStateFromTLSIdentity builds an AccessState from an TLS identity, abstracting over scoped and
// unscoped access state construction.
func (c *ScopedAccessCheckerContext) AccessStateFromTLSIdentity(ctx context.Context, ident *tlsca.Identity, authPrefGetter AuthPreferenceGetter) (AccessState, error) {
	if !c.isScoped() {
		return AccessStateFromTLSIdentity(ctx, ident, c.unscopedChecker, authPrefGetter)
	}

	authPref, err := authPrefGetter.GetAuthPreference(ctx)
	if err != nil {
		return AccessState{}, trace.Wrap(err)
	}

	if authPref.GetRequireMFAType().IsSessionMFARequired() {
		// TODO(fspmarshall/scopes): implement scoped MFA
		// NOTE: this will require additional refactoring of relevant access-checking logic. currently, we often
		// check MFA requirements *before* we determine access to the underlying resource, but a scoped MFA model
		// will need to first determine the scope of access *before* we can determine whether MFA is required for that scope.
		return AccessState{}, trace.AccessDenied("cannot perform scoped access when cluster-level MFA is required (scoped MFA is not implemented)")
	}

	return AccessState{
		// MFA state is hard-coded here because scoped roles do not support MFA yet, and the above check should reject
		// cases where cluster-level config would obligate MFA.
		MFARequired:              MFARequiredNever,
		MFAVerified:              false,
		EnableDeviceVerification: true,
		DeviceVerified:           dtauthz.IsTLSDeviceVerified(&ident.DeviceExtensions),
		IsBot:                    ident.IsBot(),
	}, nil
}

// Traits returns the user traits for this context. Agent pin identities have no traits.
func (c *ScopedAccessCheckerContext) Traits() wrappers.Traits {
	if !c.isScoped() {
		return c.unscopedChecker.Traits()
	}
	return c.traits
}

// CertParams returns a sub-context for resolving certificate parameters during certificate generation.
// This should not be used outside of certificate generation logic.
func (c *ScopedAccessCheckerContext) CertParams() *CertificateParameterContext {
	return &CertificateParameterContext{ctx: c}
}

// RiskyAuthorizeUnpinnedRead authorizes a read-only access check that bypasses
// enforcement of the identity's pinned scope. This must only be used for
// specific APIs that make an exception to pinning exclusion rules (e.g.
// allowing read operations for resources at a parent scope). To avoid misuse,
// a specific [UnpinnedReadAuthorization] must be provided that will encode the
// effective scope of the access check and the allowed verbs.
func (c *ScopedAccessCheckerContext) RiskyAuthorizeUnpinnedRead(
	ctx context.Context,
	authz UnpinnedReadAuthorization,
	ruleCtx RuleContext,
) error {
	if err := authz.check(); err != nil {
		return trace.Wrap(err, "invalid unpinned read authorization")
	}

	return c.decision(
		c.riskyUnpinnedCheckersForResourceScope(ctx, authz.resourceScope),
		func(checker *ScopedAccessChecker) error {
			return checker.CheckAccessToRules(ruleCtx, authz.kind, authz.verbs...)
		},
	)
}

// UnpinnedReadAuthorization is a special authorization to complete an unscoped
// read-only access check. This is meant to be used for access checks on
// typically cluster-wide resources that need to be readable by identities with
// a pinned scope.
type UnpinnedReadAuthorization struct {
	resourceScope string
	kind          string
	verbs         []string
}

func (a UnpinnedReadAuthorization) check() error {
	switch {
	case a.kind == "":
		return trace.BadParameter("missing kind")
	case len(a.verbs) == 0:
		return trace.BadParameter("missing verbs")
	}
	for _, verb := range a.verbs {
		switch verb {
		case types.VerbList, types.VerbReadNoSecrets, types.VerbRead:
		default:
			return trace.BadParameter("invalid verb for unpinned read authorization: %q", verb)
		}
	}
	if err := scopes.WeakValidate(a.resourceScope); err != nil {
		return trace.Wrap(err, "invalid resourceScope")
	}
	return nil
}

var (
	// UnpinnedReadCertAuthority is a special authorization to complete an
	// unscoped access check to read a cert authority without secrets.
	UnpinnedReadCertAuthority = UnpinnedReadAuthorization{
		resourceScope: scopes.Root,
		kind:          types.KindCertAuthority,
		verbs:         []string{types.VerbReadNoSecrets},
	}
	// UnpinnedReadCertAuthorities is a special authorization to complete an
	// unscoped access check to list and read a cert authorities without secrets.
	UnpinnedReadCertAuthorities = UnpinnedReadAuthorization{
		resourceScope: scopes.Root,
		kind:          types.KindCertAuthority,
		verbs:         []string{types.VerbList, types.VerbReadNoSecrets},
	}
	// UnpinnedReadAuthServers is a special authorization to complete an
	// unscoped access check to list and read auth server resources.
	UnpinnedReadAuthServers = UnpinnedReadAuthorization{
		resourceScope: scopes.Root,
		kind:          types.KindAuthServer,
		verbs:         []string{types.VerbList, types.VerbRead},
	}
	// UnpinnedReadProxies is a special authorization to complete an
	// unscoped access check to list and read proxy resources.
	UnpinnedReadProxies = UnpinnedReadAuthorization{
		resourceScope: scopes.Root,
		kind:          types.KindProxy,
		verbs:         []string{types.VerbList, types.VerbRead},
	}
	// UnpinnedReadAuthPreference is a special authorization to complete an
	// unscoped access check to read a cluster auth preference.
	UnpinnedReadAuthPreference = UnpinnedReadAuthorization{
		resourceScope: scopes.Root,
		kind:          types.KindClusterAuthPreference,
		verbs:         []string{types.VerbRead},
	}
	// UnpinnedReadVnetConfig is a special authorization to complete an
	// unscoped access check to read a cluster VNet config.
	UnpinnedReadVnetConfig = UnpinnedReadAuthorization{
		resourceScope: scopes.Root,
		kind:          types.KindVnetConfig,
		verbs:         []string{types.VerbRead},
	}
	// UnpinnedReadSPIFFEFederation is a special authorization to complete an
	// unscoped access check to read a SPIFFE federation.
	UnpinnedReadSPIFFEFederation = UnpinnedReadAuthorization{
		resourceScope: scopes.Root,
		kind:          types.KindSPIFFEFederation,
		verbs:         []string{types.VerbRead},
	}
	// UnpinnedReadSPIFFEFederations is a special authorization to complete an
	// unscoped access check to list and read SPIFFE federations.
	UnpinnedReadSPIFFEFederations = UnpinnedReadAuthorization{
		resourceScope: scopes.Root,
		kind:          types.KindSPIFFEFederation,
		verbs:         []string{types.VerbList, types.VerbRead},
	}
)
