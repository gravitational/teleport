// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package servicecfg

import "time"

// IdentityCenterConfig holds configurable parameters for the IdentityCenter integration
type IdentityCenterConfig struct {
	// EventBatchDuration specifies how long to to collect events before acting
	// on them. Shorter durations make the service more responsive, but longer
	// durations are able to discard more work and are thus more efficient.
	EventBatchDuration time.Duration

	// AccountAssignmentRecalculationInterval is the interval between full
	// assignment recalculations for all Users and Account Assignments.
	AccountAssignmentRecalculationInterval time.Duration

	// ProvisioningStateRefreshInterval determines the interval between full
	// state refreshes (i.e. checks if the principal needs to be re-provisioned)
	// in the Identity Center SCIM provisioner.
	ProvisioningStateRefreshInterval time.Duration
}
