/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package auth

var (
	// validSTSEndpoints holds a sorted list of all known valid public endpoints for
	// the AWS STS service. You can generate this list by running
	// $ go run github.com/nklaassen/sts-endpoints@latest --go-list
	// Update aws-sdk-go in that package to learn about new endpoints.
	validSTSEndpoints = []string{
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

	globalSTSEndpoints = []string{
		"sts.amazonaws.com",
		// This is not a real endpoint, but the SDK will select it if
		// AWS_USE_FIPS_ENDPOINT is set and a region is not.
		"sts-fips.aws-global.amazonaws.com",
	}

	fipsSTSEndpoints = []string{
		"sts-fips.us-east-1.amazonaws.com",
		"sts-fips.us-east-2.amazonaws.com",
		"sts-fips.us-west-1.amazonaws.com",
		"sts-fips.us-west-2.amazonaws.com",
		"sts.us-gov-east-1.amazonaws.com",
		"sts.us-gov-west-1.amazonaws.com",
	}
)
