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
	// ValidSTSEndpoints returns a sorted list of all known valid public endpoints for
	// the AWS STS service.
	//
	// TODO(nklaassen): find a better way to validate STS endpoints or generate
	// this list and get notified when it needs to be updated. The original
	// solution was https://github.com/nklaassen/sts-endpoints which is based on
	// aws-sdk-go v1 which no longer gets updates for new regions.
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

	// FIPSSTSEndpoints returns the set of known valid FIPS AWS STS endpoints.
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

	// FIPSSTSRegions returns the set of known AWS regions with FIPS STS endpoints.
	FIPSSTSRegions = sync.OnceValue(func() []string {
		return []string{
			"us-east-1",
			"us-east-2",
			"us-west-1",
			"us-west-2",
			"us-gov-east-1",
			"us-gov-west-1",
		}
	})
)
