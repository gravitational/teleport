// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package iam

import "sync"

var (
	// ValidSTSEndpoints holds a sorted list of all known valid public endpoints for
	// the AWS STS service. You can generate this list by running
	// $ go run github.com/nklaassen/sts-endpoints@latest --go-list
	// Update aws-sdk-go in that package to learn about new endpoints.
	ValidSTSEndpoints = sync.OnceValue(func() []string {
		return []string{
			"sts-fips.us-east-1.amazonaws.com",
			"sts-fips.us-east-2.amazonaws.com",
			"sts-fips.us-west-1.amazonaws.com",
			"sts-fips.us-west-2.amazonaws.com",
			"sts.af-south-1.amazonaws.com",
			"sts.amazonaws.com",
			"sts.ap-east-1.amazonaws.com",
			"sts.ap-northeast-1.amazonaws.com",
			"sts.ap-northeast-2.amazonaws.com",
			"sts.ap-northeast-3.amazonaws.com",
			"sts.ap-south-1.amazonaws.com",
			"sts.ap-south-2.amazonaws.com",
			"sts.ap-southeast-1.amazonaws.com",
			"sts.ap-southeast-2.amazonaws.com",
			"sts.ap-southeast-3.amazonaws.com",
			"sts.ap-southeast-4.amazonaws.com",
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
	})

	GlobalSTSEndpoints = sync.OnceValue(func() []string {
		return []string{
			"sts.amazonaws.com",
			// This is not a real endpoint, but the SDK will select it if
			// AWS_USE_FIPS_ENDPOINT is set and a region is not.
			"sts-fips.aws-global.amazonaws.com",
		}
	})

	FIPSSTSEndpoints = sync.OnceValue(func() []string {
		return []string{
			"sts-fips.us-east-1.amazonaws.com",
			"sts-fips.us-east-2.amazonaws.com",
			"sts-fips.us-west-1.amazonaws.com",
			"sts-fips.us-west-2.amazonaws.com",
			"sts.us-gov-east-1.amazonaws.com",
			"sts.us-gov-west-1.amazonaws.com",
		}
	})
)
