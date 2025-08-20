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
	"time"

	"github.com/gravitational/teleport/api/constants"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/api/utils/keys"
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
		common:   checker,
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
