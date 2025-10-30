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
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/itertools/stream"
)

// CommonAccessChecker defines the common methods that are identical across both scoped and unscoped access checkers.
type CommonAccessChecker interface {
	AccessInfo() *AccessInfo
	Traits() wrappers.Traits
	CheckLoginDuration(ttl time.Duration) ([]string, error)
	AdjustSessionTTL(ttl time.Duration) time.Duration
	PrivateKeyPolicy(defaultPolicy keys.PrivateKeyPolicy) (keys.PrivateKeyPolicy, error)
	PinSourceIP() bool
	CanPortForward() bool
	CanForwardAgents() bool
	PermitX11Forwarding() bool
	LockingMode(defaultMode constants.LockingMode) constants.LockingMode
	CheckAccessToRules(ctx RuleContext, resource string, verbs ...string) error
}

// ScopedAccessCheckerSubset defines the methods that are specific to scoped access checkers.
type ScopedAccessCheckerSubset interface {
	ScopePin() *scopesv1.Pin
}

// UnscopedAccessCheckerSubset defines the methods that are specific to unscoped access checkers.
type UnscopedAccessCheckerSubset interface {
	RoleNames() []string
	CertificateFormat() string
	GetAllowedResourceIDs() []types.ResourceID
	CertificateExtensions() []*types.CertExtension
	CheckAccessToRemoteCluster(cluster types.RemoteCluster) error
	CheckKubeGroupsAndUsers(ttl time.Duration, overrideTTL bool, matchers ...RoleMatcher) (groups []string, users []string, err error)
	CheckDatabaseNamesAndUsers(ttl time.Duration, overrideTTL bool) (names []string, users []string, err error)
	CheckAWSRoleARNs(ttl time.Duration, overrideTTL bool) ([]string, error)
	CheckAzureIdentities(ttl time.Duration, overrideTTL bool) ([]string, error)
	CheckGCPServiceAccounts(ttl time.Duration, overrideTTL bool) ([]string, error)
}

// SplitAccessChecker is used in logic that needs to branch based on whether it is operating on a scoped or unscoped access checker. It
// provides a Common interface that is always present, and one of either a Scoped or Unscoped interface that is present depending on
// which underlying access checker is being used. If a method that previously existed on one of the Subset interfaces is implemented
// by the second checker and moved to the Common interface, then the it should be removed from the Subset interface in order to ensure
// that we don't continue to accidentally call it on the old location.
type SplitAccessChecker struct {
	common   CommonAccessChecker
	unscoped UnscopedAccessCheckerSubset
	scoped   ScopedAccessCheckerSubset
}

func NewUnscopedSplitAccessChecker(checker AccessChecker) *SplitAccessChecker {
	return &SplitAccessChecker{
		common:   &accessCheckerShim{checker},
		unscoped: checker,
	}
}

func NewScopedSplitAccessChecker(checker *ScopedAccessChecker) *SplitAccessChecker {
	return &SplitAccessChecker{
		common: checker,
		scoped: checker,
	}
}

// Common gets the common access checker interface that is shared between both scoped and unscoped access checkers.
func (c *SplitAccessChecker) Common() CommonAccessChecker {
	return c.common
}

// Unscoped gets the unscoped access checker interface if it is present.
func (c *SplitAccessChecker) Unscoped() (checker UnscopedAccessCheckerSubset, ok bool) {
	return c.unscoped, c.unscoped != nil
}

// Scoped gets the scoped access checker interface if it is present.
func (c *SplitAccessChecker) Scoped() (checker ScopedAccessCheckerSubset, ok bool) {
	return c.scoped, c.scoped != nil
}

// accessCheckerShim provides a compatibility shim to allow using an unscoped checker in a manner more consistent with
// a scoped access checker. In particular, some scoped access checker methods are simpler than their unscoped equivalents
// and this shim implemented the simplified versions of those methods.
type accessCheckerShim struct {
	AccessChecker
}

// CheckAccessToRules verifies that *all* of a series of verbs are permitted for the specified resource. This function differs from
// the unscoped AccessChecker.CheckAccessToRule in a number of ways. It does not support advanced context-based features or namespacing,
// and accepts a set of verbs all of which must evaluate to allow for the check to succeed.
func (c *accessCheckerShim) CheckAccessToRules(ctx RuleContext, resource string, verbs ...string) error {
	return checkAccessToRulesImpl(c.AccessChecker, ctx, resource, verbs...)
}

// CheckMaybeHasAccessToRules behaves like [AccessChecker.GuessIfAccessIsPossible], except that it supports multiple
// verbs and does not support namespaces.
func (c *accessCheckerShim) CheckMaybeHasAccessToRules(ctx RuleContext, resource string, verbs ...string) error {
	return checkMaybeHasAccessToRulesImpl(c.AccessChecker, ctx, resource, verbs...)
}

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

// SplitAccessCheckerContext is an abstraction to allow passing either a scoped access checker context or a standard access checker
// to functions that need to operate on either.
type SplitAccessCheckerContext struct {
	scopedContext   *ScopedAccessCheckerContext
	unscopedChecker AccessChecker
}

// NewUnscopedSplitAccessCheckerContext creates a SplitAccessCheckerContext from an unscoped AccessChecker.
func NewUnscopedSplitAccessCheckerContext(checker AccessChecker) *SplitAccessCheckerContext {
	return &SplitAccessCheckerContext{
		unscopedChecker: checker,
	}
}

// NewScopedSplitAccessCheckerContext creates a SplitAccessCheckerContext from a scoped access checker context.
func NewScopedSplitAccessCheckerContext(ctx *ScopedAccessCheckerContext) *SplitAccessCheckerContext {
	return &SplitAccessCheckerContext{
		scopedContext: ctx,
	}
}

// CheckMaybeHasAccessToRules returns an error if the context definitely does not have access to the provided rules. in practice
// this currently just serves to exit early in the event that we are dealing with an unscoped identity without appropriate
// permissions, but it could be extended in future to perform more complex checks if needed.
func (c *SplitAccessCheckerContext) CheckMaybeHasAccessToRules(ctx RuleContext, resource string, verbs ...string) error {
	if c.scopedContext == nil {
		shim := accessCheckerShim{c.unscopedChecker}
		return shim.CheckMaybeHasAccessToRules(ctx, resource, verbs...)
	}

	return nil
}

// CheckersForResourceScope returns a stream of SplitAccessCheckers that are appropriate for the provided resource scope. If the
// underlying state is scoped this will be a stream of checkers descending to the target resource scope.
func (c *SplitAccessCheckerContext) CheckersForResourceScope(ctx context.Context, scope string) stream.Stream[*SplitAccessChecker] {
	return func(yield func(*SplitAccessChecker, error) bool) {
		if c.scopedContext == nil {
			yield(NewUnscopedSplitAccessChecker(c.unscopedChecker), nil)
			return
		}

		for checker, err := range c.scopedContext.CheckersForResourceScope(ctx, scope) {
			if err != nil {
				yield(nil, trace.Wrap(err))
				return
			}
			if !yield(NewScopedSplitAccessChecker(checker), nil) {
				return
			}
		}
	}
}

// RiskyUnpinnedCheckersForResourceScope is equivalent to CheckersForResourceScope except that it bypasses enforcement of pinning scope. This is a
// risky operation that should only be used for certain APIs that make an exception to pinning exclusion rules
func (c *SplitAccessCheckerContext) RiskyUnpinnedCheckersForResourceScope(ctx context.Context, scope string) stream.Stream[*SplitAccessChecker] {
	return func(yield func(*SplitAccessChecker, error) bool) {
		if c.scopedContext == nil {
			yield(NewUnscopedSplitAccessChecker(c.unscopedChecker), nil)
			return
		}

		for checker, err := range c.scopedContext.RiskyUnpinnedCheckersForResourceScope(ctx, scope) {
			if err != nil {
				yield(nil, trace.Wrap(err))
				return
			}
			if !yield(NewScopedSplitAccessChecker(checker), nil) {
				return
			}
		}
	}
}

// Decision is a helper function that takes care of boilerplate for simple potentially scoped decisions.  It calls
// the provided decision function until one of three conditions is met:
//
// 1. Decision function executes without error (allow)
// 2. Decision function returns an AccessExplicitlyDenied error (explicit deny)
// 3. Checkers for all scopes in resource scope path have been visited (implicit deny)
func (c *SplitAccessCheckerContext) Decision(ctx context.Context, scope string, fn func(*SplitAccessChecker) error) error {
	return c.decision(ctx, c.CheckersForResourceScope(ctx, scope), fn)
}

// RiskyUnpinnedDecision is equivalent to Decision except that it bypasses enforcement of pinning scope. This is a
// risky operation that should only be used for certain APIs that make an exception to pinning exclusion rules
// (e.g. allowing read operations to succeed for resources in a parent to the pinned scope).
func (c *SplitAccessCheckerContext) RiskyUnpinnedDecision(ctx context.Context, scope string, fn func(*SplitAccessChecker) error) error {
	return c.decision(ctx, c.RiskyUnpinnedCheckersForResourceScope(ctx, scope), fn)
}

func (c *SplitAccessCheckerContext) decision(ctx context.Context, checkers stream.Stream[*SplitAccessChecker], fn func(*SplitAccessChecker) error) error {
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
