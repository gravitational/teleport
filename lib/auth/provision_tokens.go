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

package auth

import (
	"slices"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/join/oracle"
)

func validateProvisionToken(token types.ProvisionToken) error {
	switch token.GetJoinMethod() {
	case types.JoinMethodOracle:
		return validateOracleJoinToken(token)

	case types.JoinMethodEC2:
		return validateEC2Token(token)

	case types.JoinMethodIAM:
		return validateIAMToken(token)
	}

	return nil
}

func validateEC2Token(token types.ProvisionToken) error {
	for _, allowRule := range token.GetAllowRules() {
		// EC2 join method does not support AWS Organizational Unit matchers, so we return an
		// error if any of the token rules contain them.
		if tokenRuleHasAWSOrganizationalUnitMatchers(allowRule) {
			return trace.BadParameter(`the %q join method does not support the "aws_organizational_units" parameter`, types.JoinMethodEC2)
		}
	}
	return nil
}

func tokenRuleHasAWSOrganizationalUnitMatchers(tokenRule *types.TokenRule) bool {
	return tokenRule.AWSOrganizationalUnits != nil &&
		(len(tokenRule.AWSOrganizationalUnits.Include) > 0 || len(tokenRule.AWSOrganizationalUnits.Exclude) > 0)
}

func validateIAMToken(token types.ProvisionToken) error {
	for _, allowRule := range token.GetAllowRules() {
		if err := validateIAMOrganizationRule(allowRule); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func validateIAMOrganizationRule(tokenRule *types.TokenRule) error {
	// In order to use Organizational Unit matchers, the token must specify the AWS Organization ID.
	if tokenRule.AWSOrganizationID == "" && tokenRuleHasAWSOrganizationalUnitMatchers(tokenRule) {
		return trace.BadParameter(`allow rule with "aws_organizational_units" matchers must also specify "aws_organization_id" when using the %q join method`, types.JoinMethodIAM)
	}

	// Return early if no OU matchers are specified.
	if !tokenRuleHasAWSOrganizationalUnitMatchers(tokenRule) {
		return nil
	}

	if len(tokenRule.AWSOrganizationalUnits.Include) == 0 {
		return trace.BadParameter(`at least one entry in "aws_organizational_units.include" must be specified`)
	}

	if slices.Contains(tokenRule.AWSOrganizationalUnits.Include, types.Wildcard) && len(tokenRule.AWSOrganizationalUnits.Include) > 1 {
		return trace.BadParameter(`when using wildcard for "aws_organizational_units.include", no other values are allowed`)
	}
	if slices.Contains(tokenRule.AWSOrganizationalUnits.Exclude, types.Wildcard) {
		return trace.BadParameter(`using wildcard in "aws_organizational_units.exclude" is not allowed`)
	}

	return nil
}

// validateOracleJoinToken validates the fields in a token using the Oracle
// join method. It's done here instead of in the client so the client doesn't
// have to import the Oracle SDK.
func validateOracleJoinToken(token types.ProvisionToken) error {
	tokenV2, ok := token.(*types.ProvisionTokenV2)
	if !ok {
		return trace.BadParameter("%v join method requires ProvisionTokenV2", types.JoinMethodOracle)
	}
	oracleSpec := tokenV2.Spec.Oracle
	if oracleSpec == nil {
		return trace.BadParameter("missing spec")
	}
	for _, allow := range oracleSpec.Allow {
		if _, err := oracle.ParseRegionFromOCID(allow.Tenancy); err != nil {
			return trace.BadParameter("invalid tenant: %v", allow.Tenancy)
		}
		for _, compartment := range allow.ParentCompartments {
			if _, err := oracle.ParseRegionFromOCID(compartment); err != nil {
				return trace.BadParameter("invalid compartment: %v", compartment)
			}
		}
		for _, region := range allow.Regions {
			if canonicalRegion, _ := oracle.ParseRegion(region); canonicalRegion == "" {
				return trace.BadParameter("invalid region: %v", region)
			}
		}
		for _, instanceID := range allow.Instances {
			if _, err := oracle.ParseRegionFromOCID(instanceID); err != nil {
				return trace.BadParameter("invalid instance OCID: %s", instanceID)
			}
		}
	}
	return nil
}
