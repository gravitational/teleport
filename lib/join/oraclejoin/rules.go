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

package oraclejoin

import (
	"slices"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/join/provision"
)

// CheckOracleAllowRules checks that a the oracle rules in a provision token
// allow an OCI instance with the given claims to join the cluster.
func CheckOracleAllowRules(claims *Claims, token provision.Token) error {
	instanceRegion := claims.Region()
	for _, rule := range token.GetOracle().Allow {
		if rule.Tenancy != claims.TenancyID {
			// rule.Tenancy is required, if it doesn't match the instance skip this rule.
			continue
		}
		if !ruleAllowsCompartment(rule, claims.CompartmentID) {
			// Skip this rule if it does not allow the instance's compartment.
			continue
		}
		if !ruleAllowsRegion(rule, instanceRegion) {
			// Skip this rule if it does not allow the instance's region.
			continue
		}
		if !ruleAllowsInstanceID(rule, claims.InstanceID) {
			// Skip this rule if it does not allow the instance's ID.
			continue
		}
		// This rule allows the instance to join.
		return nil
	}
	return trace.AccessDenied("instance %v did not match any allow rules in token %v", claims.InstanceID, token.GetName())
}

func ruleAllowsCompartment(rule *types.ProvisionTokenSpecV2Oracle_Rule, instanceCompartment string) bool {
	if len(rule.ParentCompartments) == 0 {
		// Empty list means all compartments are allowed.
		return true
	}
	return slices.Contains(rule.ParentCompartments, instanceCompartment)
}

func ruleAllowsRegion(rule *types.ProvisionTokenSpecV2Oracle_Rule, instanceRegion string) bool {
	if len(rule.Regions) == 0 {
		// Empty list means all regions are allowed.
		return true
	}
	return slices.ContainsFunc(rule.Regions, func(allowedRegion string) bool {
		canonicalAllowedRegion, _ := ParseRegion(allowedRegion)
		return canonicalAllowedRegion == instanceRegion
	})
}

func ruleAllowsInstanceID(rule *types.ProvisionTokenSpecV2Oracle_Rule, instanceID string) bool {
	if len(rule.Instances) == 0 {
		// Empty list means all instance IDs are allowed.
		return true
	}
	return slices.Contains(rule.Instances, instanceID)
}
