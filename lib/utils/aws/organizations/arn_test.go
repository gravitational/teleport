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
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/stretchr/testify/require"
)

func TestOrganizationIDFromAccountARN(t *testing.T) {
	for _, tt := range []struct {
		name        string
		accountARN  string
		expectedOrg string
		errCheck    require.ErrorAssertionFunc
	}{
		{
			name:        "valid account ARN",
			accountARN:  "arn:aws:organizations::123456789012:account/o-exampleorgid/111111111111",
			expectedOrg: "o-exampleorgid",
			errCheck:    require.NoError,
		},
		{
			name:       "invalid ARN format",
			accountARN: "invalid-arn-format",
			errCheck:   require.Error,
		},
		{
			name:       "wrong resource type",
			accountARN: "arn:aws:organizations::123456789012:root/o-exampleorgid/111111111111",
			errCheck:   require.Error,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			gotOrg, err := OrganizationIDFromAccountARN(tt.accountARN)
			tt.errCheck(t, err)
			require.Equal(t, tt.expectedOrg, gotOrg)
		})
	}
}

func TestOrganizationIDFromRootARN(t *testing.T) {
	for _, tt := range []struct {
		name        string
		rootARN     string
		expectedOrg string
		errCheck    require.ErrorAssertionFunc
	}{
		{
			name:        "valid root ARN",
			rootARN:     "arn:aws:organizations::123456789012:root/o-exampleorgid/r-exampleroot",
			expectedOrg: "o-exampleorgid",
			errCheck:    require.NoError,
		},
		{
			name:        "valid root ARN in GovCloud",
			rootARN:     "arn:aws-us-gov:organizations::123456789012:root/o-exampleorgid/r-exampleroot",
			expectedOrg: "o-exampleorgid",
			errCheck:    require.NoError,
		},
		{
			name:     "too few resource parts",
			rootARN:  "arn:aws:organizations::123456789012:root/o-exampleorgid",
			errCheck: require.Error,
		},
		{
			name:     "wrong resource type",
			rootARN:  "arn:aws:organizations::123456789012:account/o-exampleorgid/111111111111",
			errCheck: require.Error,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := arn.Parse(tt.rootARN)
			require.NoError(t, err)

			gotOrg, err := organizationIDFromARN(parsed, "root")
			tt.errCheck(t, err)
			require.Equal(t, tt.expectedOrg, gotOrg)
		})
	}
}
