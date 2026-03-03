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

package unifiedresources

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils/aws"
)

func TestComputeAWSRolesWithRequiresRequest(t *testing.T) {
	const accountID = "123456789012"
	arnAdmin := "arn:aws:iam::123456789012:role/Admin"
	arnDev := "arn:aws:iam::123456789012:role/Developer"
	arnReadOnly := "arn:aws:iam::123456789012:role/ReadOnly"
	arnOther := "arn:aws:iam::999999999999:role/OtherAccount"

	grantedRoles := aws.Roles{
		{Name: "Admin", Display: "Admin", ARN: arnAdmin, AccountID: accountID},
		{Name: "Developer", Display: "Developer", ARN: arnDev, AccountID: accountID},
	}

	tests := []struct {
		name               string
		visibleRoleARNs    []string
		grantedRoles       aws.Roles
		accountID          string
		includeRequestable bool
		expected           aws.Roles
	}{
		{
			name:               "granted roles are not marked as requiring request",
			visibleRoleARNs:    []string{arnAdmin, arnDev},
			grantedRoles:       grantedRoles,
			accountID:          accountID,
			includeRequestable: false,
			expected: aws.Roles{
				{Name: "Admin", Display: "Admin", ARN: arnAdmin, AccountID: accountID, RequiresRequest: false},
				{Name: "Developer", Display: "Developer", ARN: arnDev, AccountID: accountID, RequiresRequest: false},
			},
		},
		{
			name:               "non-granted roles are excluded when includeRequestable is false",
			visibleRoleARNs:    []string{arnAdmin, arnReadOnly},
			grantedRoles:       grantedRoles,
			accountID:          accountID,
			includeRequestable: false,
			expected: aws.Roles{
				{Name: "Admin", Display: "Admin", ARN: arnAdmin, AccountID: accountID, RequiresRequest: false},
			},
		},
		{
			name:               "non-granted roles are included and marked as requiring request when includeRequestable is true",
			visibleRoleARNs:    []string{arnAdmin, arnReadOnly},
			grantedRoles:       grantedRoles,
			accountID:          accountID,
			includeRequestable: true,
			expected: aws.Roles{
				{Name: "Admin", Display: "Admin", ARN: arnAdmin, AccountID: accountID, RequiresRequest: false},
				{Name: "ReadOnly", Display: "ReadOnly", ARN: arnReadOnly, AccountID: accountID, RequiresRequest: true},
			},
		},
		{
			name:               "roles from other accounts are filtered out",
			visibleRoleARNs:    []string{arnAdmin, arnOther},
			grantedRoles:       grantedRoles,
			accountID:          accountID,
			includeRequestable: true,
			expected: aws.Roles{
				{Name: "Admin", Display: "Admin", ARN: arnAdmin, AccountID: accountID, RequiresRequest: false},
			},
		},
		{
			name:               "empty visible roles returns empty result",
			visibleRoleARNs:    []string{},
			grantedRoles:       grantedRoles,
			accountID:          accountID,
			includeRequestable: true,
			expected:           aws.Roles{},
		},
		{
			name:               "empty granted roles marks all visible roles as requiring request",
			visibleRoleARNs:    []string{arnAdmin, arnDev},
			grantedRoles:       aws.Roles{},
			accountID:          accountID,
			includeRequestable: true,
			expected: aws.Roles{
				{Name: "Admin", Display: "Admin", ARN: arnAdmin, AccountID: accountID, RequiresRequest: true},
				{Name: "Developer", Display: "Developer", ARN: arnDev, AccountID: accountID, RequiresRequest: true},
			},
		},
		{
			name:               "empty granted roles and includeRequestable false returns no roles",
			visibleRoleARNs:    []string{arnAdmin, arnDev},
			grantedRoles:       aws.Roles{},
			accountID:          accountID,
			includeRequestable: false,
			expected:           aws.Roles{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := computeAWSRolesWithRequiresRequest(
				tt.visibleRoleARNs,
				tt.grantedRoles,
				tt.accountID,
				tt.includeRequestable,
			)
			require.Equal(t, tt.expected, result)
		})
	}
}
