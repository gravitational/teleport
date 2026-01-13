// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package iamjoin

import (
	"slices"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestValidateSTSHost(t *testing.T) {
	validEndpoints := []string{
		"sts-fips.us-east-1.amazonaws.com",
		"sts-fips.us-east-2.amazonaws.com",
		"sts-fips.us-west-1.amazonaws.com",
		"sts-fips.us-west-2.amazonaws.com",
		"sts.af-south-1.amazonaws.com",
		"sts.ap-east-1.amazonaws.com",
		"sts.ap-east-2.amazonaws.com",
		"sts.ap-northeast-1.amazonaws.com",
		"sts.ap-northeast-2.amazonaws.com",
		"sts.ap-northeast-3.amazonaws.com",
		"sts.ap-south-1.amazonaws.com",
		"sts.ap-south-2.amazonaws.com",
		"sts.ap-southeast-1.amazonaws.com",
		"sts.ap-southeast-2.amazonaws.com",
		"sts.ap-southeast-3.amazonaws.com",
		"sts.ap-southeast-4.amazonaws.com",
		"sts.ap-southeast-5.amazonaws.com",
		"sts.ap-southeast-6.amazonaws.com",
		"sts.ap-southeast-7.amazonaws.com",
		"sts.ca-central-1.amazonaws.com",
		"sts.ca-west-1.amazonaws.com",
		"sts.cn-north-1.amazonaws.com.cn",
		"sts.cn-northwest-1.amazonaws.com.cn",
		"sts.eu-central-1.amazonaws.com",
		"sts.eu-central-2.amazonaws.com",
		"sts.eu-north-1.amazonaws.com",
		"sts.eu-south-1.amazonaws.com",
		"sts.eu-south-2.amazonaws.com",
		"sts.eu-west-1.amazonaws.com",
		"sts.eu-west-2.amazonaws.com",
		"sts.eu-west-3.amazonaws.com",
		"sts.il-central-1.amazonaws.com",
		"sts.me-central-1.amazonaws.com",
		"sts.me-south-1.amazonaws.com",
		"sts.sa-east-1.amazonaws.com",
		"sts.us-east-1.amazonaws.com",
		"sts.us-east-2.amazonaws.com",
		"sts.us-gov-east-1.amazonaws.com",
		"sts.us-gov-west-1.amazonaws.com",
		"sts.us-iso-east-1.c2s.ic.gov",
		"sts.us-iso-west-1.c2s.ic.gov",
		"sts.us-isob-east-1.sc2s.sgov.gov",
		"sts.us-west-1.amazonaws.com",
		"sts.us-west-2.amazonaws.com",
	}

	fipsEndpoints := []string{
		"sts-fips.us-east-1.amazonaws.com",
		"sts-fips.us-east-2.amazonaws.com",
		"sts-fips.us-west-1.amazonaws.com",
		"sts-fips.us-west-2.amazonaws.com",
		"sts.us-gov-east-1.amazonaws.com",
		"sts.us-gov-west-1.amazonaws.com",
	}

	for _, endpoint := range validEndpoints {
		t.Run(endpoint, func(t *testing.T) {
			t.Run("fips not required", func(t *testing.T) {
				require.NoError(t, validateSTSHost(t.Context(), endpoint, false /*requireFIPS*/))
			})

			t.Run("fips required", func(t *testing.T) {
				err := validateSTSHost(t.Context(), endpoint, true /*requireFIPS*/)
				if slices.Contains(fipsEndpoints, endpoint) {
					require.NoError(t, err)
				} else {
					require.ErrorAs(t, err, new(*trace.AccessDeniedError))
				}
			})
		})
	}

	invalidEndpoints := []string{
		"sts-fips.ap-south-1.amazonaws.com", // No real FIPS endpoint in this region.
		"sts.evil.amazonaws.com",
		"sts-fips.evil.amazonaws.com",
		"evil.us-west-2.amazonaws.com",
		"example.com",
		"sts.example.com",
		"sts-fips.example.com",
		"sts.us-west-2.example.com",
		"sts.us-west-2.amazonaws.com.evil",
	}

	for _, endpoint := range invalidEndpoints {
		t.Run("fips not required", func(t *testing.T) {
			err := validateSTSHost(t.Context(), endpoint, false /*requireFIPS*/)
			require.ErrorAs(t, err, new(*trace.AccessDeniedError))
		})

		t.Run("fips required", func(t *testing.T) {
			err := validateSTSHost(t.Context(), endpoint, true /*requireFIPS*/)
			require.ErrorAs(t, err, new(*trace.AccessDeniedError))
		})
	}
}
