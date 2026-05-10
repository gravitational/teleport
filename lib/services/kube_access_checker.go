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
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
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
func (c *KubeAccessChecker) GetGroupsAndUsers(ttl time.Duration, overrideTTL bool, matchers ...RoleMatcher) ([]string, []string, error) {
	if !c.checker.isScoped() {
		return c.checker.unscopedChecker.CheckKubeGroupsAndUsers(ttl, overrideTTL, matchers...)
	}

	return c.checker.scopedCompatChecker.CheckKubeGroupsAndUsers(ttl, overrideTTL, matchers...)
}

// GetResources returns the kube resources that are permitted for access.
func (c *KubeAccessChecker) GetResources(target types.KubeCluster) (allowed []types.KubernetesResource, denied []types.KubernetesResource) {
	if !c.checker.isScoped() {
		return c.checker.unscopedChecker.GetKubeResources(target)
	}

	return c.checker.scopedCompatChecker.GetKubeResources(target)
}

// AdjustClientIdleTimeout determines the kube client idle timeout to apply. The supplied argument must be
// the globally defined most-permissive value. For scoped identities, the value is read directly from the
// scoped role proto (kube.client_idle_timeout takes precedence over defaults.client_idle_timeout). If the
// role specifies a more restrictive value it is returned; otherwise the global value is returned unchanged.
// An error is returned if the role contains a non-empty duration string that cannot be parsed.
func (c *KubeAccessChecker) AdjustClientIdleTimeout(timeout time.Duration) (time.Duration, error) {
	if !c.checker.isScoped() {
		return c.checker.unscopedChecker.AdjustClientIdleTimeout(timeout), nil
	}
	// Kube block takes precedence over defaults block.
	idleStr := c.checker.role.GetSpec().GetKube().GetClientIdleTimeout()
	if idleStr == "" {
		idleStr = c.checker.role.GetSpec().GetDefaults().GetClientIdleTimeout()
	}
	if idleStr != "" {
		d, err := time.ParseDuration(idleStr)
		if err != nil {
			return 0, trace.Errorf("invalid client_idle_timeout %q in scoped role %q: %w", idleStr, c.checker.role.GetMetadata().GetName(), err)
		}
		if d > 0 && (timeout == 0 || d < timeout) {
			return max(d, 0), nil
		}
	}
	return max(timeout, 0), nil
}
