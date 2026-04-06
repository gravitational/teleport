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

	"github.com/gravitational/teleport/api/types"
)

// KubeAccessChecker provides kube-specific access checking, abstracting over scoped and unscoped identities.
// It is obtained from [ScopedAccessChecker.Kube] and should not be constructed directly. Methods on this type
// implement kube-specific behavior, branching internally between the scoped and unscoped paths of the underlying
// [ScopedAccessChecker].
type KubeAccessChecker struct {
	checker *ScopedAccessChecker
}

// CheckAccessToServer checks access to a kube server.
func (c *KubeAccessChecker) CheckAccessToServer(target types.KubeServer, state AccessState) error {
	if !c.checker.isScoped() {
		return c.checker.unscopedChecker.CheckAccess(target, state)
	}
	return c.checker.scopedCompatChecker.CheckAccess(target, state)
}

// CanAccessServer checks whether read access to the specified kube server is possible without
// regard to a specific MFA state. Used for listing/filtering.
func (c *KubeAccessChecker) CanAccessServer(target types.KubeServer) error {
	if !c.checker.isScoped() {
		return c.checker.unscopedChecker.CheckAccess(target, AccessState{MFAVerified: true})
	}
	return c.checker.scopedCompatChecker.CheckAccess(target, AccessState{MFAVerified: true})
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

// GetGroupsAndUsers returns the kube groups and users that are permitted for
// impersonation.
func (c *KubeAccessChecker) GetGroupsAndUsers(ttl time.Duration, overrideTTL bool, matchers ...RoleMatcher) ([]string, []string, error) {
	if !c.checker.isScoped() {
		return c.checker.unscopedChecker.CheckKubeGroupsAndUsers(ttl, overrideTTL, matchers...)
	}
	return c.checker.scopedCompatChecker.CheckKubeGroupsAndUsers(ttl, overrideTTL, matchers...)
}

// GetGroupsAndUsers returns the kube groups and users that are permitted for
// impersonation.
func (c *KubeAccessChecker) GetResources(cluster types.KubeCluster) (allowed []types.KubernetesResource, denied []types.KubernetesResource) {
	if !c.checker.isScoped() {
		return c.checker.unscopedChecker.GetKubeResources(cluster)
	}
	// resources are not yet supported for scoped identities
	return nil, nil
}

func (c *KubeAccessChecker) AdjustClientIdleTimeout(timeout time.Duration) time.Duration {
	if !c.checker.isScoped() {
		return c.checker.unscopedChecker.AdjustClientIdleTimeout(timeout)
	}
	// kube block takes precedence over defaults block.
	idleStr := c.checker.role.GetSpec().GetKube().GetClientIdleTimeout()
	if idleStr == "" {
		idleStr = c.checker.role.GetSpec().GetDefaults().GetClientIdleTimeout()
	}
	if idleStr != "" {
		if d, err := time.ParseDuration(idleStr); err == nil && d > 0 {
			if timeout == 0 || d < timeout {
				return d
			}
		}
	}
	return timeout
}
