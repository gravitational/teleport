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

package organizations

import (
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/gravitational/trace"
)

// accountARNBelongsToOrganizationID extracts the organization ID from an account ARN, and checks if it matches the provided organization ID.
// Example ARN: arn:aws:organizations::<org-master-account-id>:account/<org-id>/<account-id>
func accountARNBelongsToOrganizationID(accountARN string, organizationID string) error {
	return orgARNBelongsToOrganizationID(accountARN, organizationID)
}

// accountARNBelongsToOrganizationID extracts the organization ID from an account ARN, and checks if it matches the provided organization ID.
// Example ARN: arn:aws:organizations::<org-master-account-id>:root/<org-id>/<root-ou-id>
func rootOUARNBelongsToOrganizationID(rootOUARN string, organizationID string) error {
	return orgARNBelongsToOrganizationID(rootOUARN, organizationID)
}

func orgARNBelongsToOrganizationID(orgARN string, organizationID string) error {
	arnParsed, err := arn.Parse(orgARN)
	if err != nil {
		return trace.Wrap(err)
	}
	resourceSplitted := strings.Split(arnParsed.Resource, "/")
	if len(resourceSplitted) != 3 {
		return trace.BadParameter("unexpected resource received in ARN from organizations API call: %s", orgARN)
	}
	arnOrgID := resourceSplitted[1]

	if arnOrgID != organizationID {
		return trace.BadParameter("requested organization ID %q is not accessible from current assumed IAM role", organizationID)
	}

	return nil
}
