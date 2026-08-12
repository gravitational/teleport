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
	a, err := arn.Parse(accountARN)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return organizationIDFromARN(a, "account")
}

// organizationIDFromARN extracts the organization ID from an AWS Organizations ARN
// whose resource is of the given type.
// Example root ARN: arn:aws:organizations::<org-master-account-id>:root/<org-id>/<root-ou-id>
func organizationIDFromARN(a arn.ARN, resourceType string) (string, error) {
	parts := strings.Split(a.Resource, "/")
	if len(parts) != 3 {
		return "", trace.BadParameter("unexpected resource received in ARN from organizations API call: %s", a)
	}
	if parts[0] != resourceType {
		return "", trace.BadParameter("expected resource type %s but received unexpected resource type %s in ARN from organizations API call: %s", resourceType, parts[0], a)
	}

	return parts[1], nil
}
