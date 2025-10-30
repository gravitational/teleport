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
	ptv2, ok := token.(*types.ProvisionTokenV2)
	if !ok {
		return trace.Errorf("Oracle join method only supports ProvisionTokenV2")
	}
	instanceRegion := claims.Region()
	for _, rule := range ptv2.Spec.Oracle.Allow {
		if rule.Tenancy != claims.TenancyID {
			// rule.Tenancy is required, if it doesn't match the instance skip this rule.
			continue
		}
		if len(rule.ParentCompartments) != 0 && !slices.Contains(rule.ParentCompartments, claims.CompartmentID) {
			// rule.ParentCompartments must match the instance if it is set.
			continue
		}
		if !ruleAllowsRegion(rule, instanceRegion) {
			// Skip this rule if it does not allow the instance's region.
			continue
		}
		// This rule allows the instance to join.
		return nil
	}
	return trace.AccessDenied("instance %v did not match any allow rules in token %v", claims.InstanceID, token.GetName())
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
