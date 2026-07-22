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

// OrganizationIDFromAccountARN extracts the organization ID from an account ARN.
// Example ARN: arn:aws:organizations::<org-master-account-id>:account/<org-id>/<account-id>
func OrganizationIDFromAccountARN(accountARN string) (string, error) {
	return organizationIDFromARN(accountARN, "account")
}

// organizationIDFromRootOUARN extracts the organization ID from an root Organizational Unit ARN.
// Example ARN: arn:aws:organizations::<org-master-account-id>:root/<org-id>/<root-ou-id>
func organizationIDFromRootOUARN(rootOUARN string) (string, error) {
	return organizationIDFromARN(rootOUARN, "root")
}

func organizationIDFromARN(orgARN string, resourceType string) (string, error) {
	arnParsed, err := arn.Parse(orgARN)
	if err != nil {
		return "", trace.Wrap(err)
	}
	resourceSplitted := strings.Split(arnParsed.Resource, "/")
	if len(resourceSplitted) != 3 {
		return "", trace.BadParameter("unexpected resource received in ARN from organizations API call: %s", orgARN)
	}
	if resourceSplitted[0] != resourceType {
		return "", trace.BadParameter("expected resource type %s but received unexpected resource type %s in ARN from organizations API call: %s", resourceType, resourceSplitted[0], orgARN)
	}
	organizationID := resourceSplitted[1]

	return organizationID, nil
}
