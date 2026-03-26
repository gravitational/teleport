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
func (c *KubeAccessChecker) CheckAccessToCluster(target types.KubeCluster, state AccessState) error {
	if !c.checker.isScoped() {
		return c.checker.unscopedChecker.CheckAccess(target, state)
	}
	return c.checker.scopedCompatChecker.CheckAccess(target, state)
}

// CanAccessCluster checks whether read access to the specified kube server is possible without
// regard to a specific MFA state. Used for listing/filtering.
func (c *KubeAccessChecker) CanAccessCluster(target types.KubeCluster) error {
	if !c.checker.isScoped() {
		return c.checker.unscopedChecker.CheckAccess(target, AccessState{MFAVerified: true})
	}
	return c.checker.scopedCompatChecker.CheckAccess(target, AccessState{MFAVerified: true})
}
