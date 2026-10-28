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
	"cmp"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/itertools"
	"github.com/gravitational/teleport/lib/itertools/stream"
)

// KubeAccessChecker provides kube-specific access checking, abstracting over scoped and unscoped identities.
// It is obtained from [ScopedAccessChecker.Kube] and should not be constructed directly. Methods on this type
// implement kube-specific behavior, branching internally between the scoped and unscoped paths of the underlying
// [ScopedAccessChecker].
type KubeAccessChecker struct {
	checker *ScopedAccessChecker
}

// CheckAccessToCluster checks access to a kube cluster.
func (c *KubeAccessChecker) CheckAccessToCluster(target types.KubeCluster, state AccessState, matchers ...RoleMatcher) error {
	if !c.checker.isScoped() {
		return c.checker.unscopedChecker.CheckAccess(target, state, matchers...)
	}
	return c.checker.scopedCompatChecker.CheckAccess(target, state, matchers...)
}

// CanAccessCluster checks whether read access to the specified kube server is possible without
// regard to a specific MFA state. Used for listing/filtering.
func (c *KubeAccessChecker) CanAccessCluster(target types.KubeCluster) error {
	if !c.checker.isScoped() {
		return c.checker.unscopedChecker.CheckAccess(target, AccessState{MFAVerified: true})
	}
	return c.checker.scopedCompatChecker.CheckAccess(target, AccessState{MFAVerified: true})
}

// GetGroupsAndUsers returns the kube groups and users that are permitted for impersonation.
//
// NOTE: This method is somewhat of an outlier in terms of its scoped behavior. Most scoped controls
// don't rely on RoleMatcher selection because scoped RBAC structurally enforces role-matching at decision
// time (i.e. controls are derived from a checker that *only* consists of matching roles already). Kube
// access's current structure sometimes results in the primary decision being made in a context where
// the target resources are not known, and subsequent sub-decisions further narrowing down matching roles
// by resource rules. As a result, we currently defer to the unscoped implementation for this control, which
// may result in kube resource based narrowing.
func (c *KubeAccessChecker) GetGroupsAndUsers(ttl time.Duration, overrideTTL bool, matchers ...RoleMatcher) ([]string, []string, error) {
	if !c.checker.isScoped() {
		return c.checker.unscopedChecker.CheckKubeGroupsAndUsers(ttl, overrideTTL, matchers...)
	}

	return c.checker.aggregateRoleSet().CheckKubeGroupsAndUsers(ttl, overrideTTL, matchers...)
}

// GetResources returns the kube resources that are permitted for access.
func (c *KubeAccessChecker) GetResources(target types.KubeCluster) (allowed []types.KubernetesResource, denied []types.KubernetesResource) {
	if !c.checker.isScoped() {
		return c.checker.unscopedChecker.GetKubeResources(target)
	}

	return c.checker.aggregateRoleSet().GetKubeResources(target, c.checker.AccessInfo().Username, c.checker.Traits())
}

// AdjustClientIdleTimeout determines the kube client idle timeout to apply. The supplied argument must be
// the globally defined most-permissive value. For scoped identities, the most restrictive non-zero value
// across all roles is selected (kube.client_idle_timeout takes precedence over defaults.client_idle_timeout).
// If the selected value is more restrictive than the global value it is returned; otherwise the global value
// is returned unchanged. An error is returned if any role contains a non-empty duration string that cannot
// be parsed.
func (c *KubeAccessChecker) AdjustClientIdleTimeout(timeout time.Duration) (time.Duration, error) {
	if !c.checker.isScoped() {
		return c.checker.unscopedChecker.AdjustClientIdleTimeout(timeout), nil
	}

	// aggregate stream of all non-empty client_idle_timeout strings
	idleStrs := itertools.FilterMap(c.checker.roles(), func(role *scopedaccessv1.ScopedRole) (string, bool) {
		// kube.client_idle_timeout takes precedence over defaults.client_idle_timeout
		s := cmp.Or(role.GetSpec().GetKube().GetClientIdleTimeout(), role.GetSpec().GetDefaults().GetClientIdleTimeout())
		return s, s != ""
	})

	// convert to fallible stream of parsed durations
	decoded := stream.TryMap(stream.FromSeq(idleStrs), func(s string) (time.Duration, error) {
		d, err := time.ParseDuration(s)
		if err != nil {
			return 0, trace.Wrap(err, "failed to parse client_idle_timeout %q", s)
		}
		return d, nil
	})

	// add the global timeout to the stream. since we always select the lowest non-zero value, the global
	// needs no special handling.
	durations := stream.Chain(decoded, stream.Once(timeout))

	// select the lowest non-zero duration
	return minNonZeroStream(durations)
}

// AdjustDisconnectExpiredCert adjusts whether to disconnect on certificate expiry. For scoped identities, an explicit
// true in any role takes precedence over an explicit false in any role. The supplied default is only used if no role
// specifies a value (kube.disconnect_expired_cert takes precedence over defaults.disconnect_expired_cert).
func (c *KubeAccessChecker) AdjustDisconnectExpiredCert(disconnect bool) bool {
	if !c.checker.isScoped() {
		return c.checker.unscopedChecker.AdjustDisconnectExpiredCert(disconnect)
	}

	// aggregate all explicitly set disconnect_expired_cert values
	disconnects := itertools.FilterMap(c.checker.roles(), func(role *scopedaccessv1.ScopedRole) (bool, bool) {
		if kube := role.GetSpec().GetKube(); kube.HasDisconnectExpiredCert() {
			return kube.GetDisconnectExpiredCert(), true
		}
		return role.GetSpec().GetDefaults().GetDisconnectExpiredCert(), role.GetSpec().GetDefaults().HasDisconnectExpiredCert()
	})

	// an explicit true takes precedence over an explicit false.
	if d, ok := highestPrecedence(disconnects, true, false); ok {
		return d
	}

	// the cluster default is only applied if no role specifies a value.
	return disconnect
}

// LockingMode returns the kube lock enforcement mode to apply. For scoped identities, strict in any role takes
// precedence over best_effort in any role. The supplied default is only used if no role specifies a mode
// (kube.lock takes precedence over defaults.lock).
func (c *KubeAccessChecker) LockingMode(defaultMode constants.LockingMode) constants.LockingMode {
	if !c.checker.isScoped() {
		return c.checker.unscopedChecker.LockingMode(defaultMode)
	}

	// aggregate all explicitly set lock modes
	modes := itertools.FilterMap(c.checker.roles(), func(role *scopedaccessv1.ScopedRole) (constants.LockingMode, bool) {
		lock := role.GetSpec().GetKube().GetLock()
		if lock == nil {
			lock = role.GetSpec().GetDefaults().GetLock()
		}
		return constants.LockingMode(lock.GetMode()), lock.GetMode() != ""
	})

	// strict takes precedence over best effort
	if mode, ok := highestPrecedence(modes, constants.LockingModeStrict, constants.LockingModeBestEffort); ok {
		return mode
	}

	// the cluster default is only applied if no role specifies a recognized mode.
	return defaultMode
}
