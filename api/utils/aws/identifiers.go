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

package aws

import (
	"regexp"
	"slices"
	"strings"

	"github.com/gravitational/trace"
)

// IsValidAccountID checks whether the accountID is a valid AWS Account ID
//
// https://docs.aws.amazon.com/accounts/latest/reference/manage-acct-identifiers.html
func IsValidAccountID(accountID string) error {
	if len(accountID) != 12 {
		return trace.BadParameter("must be 12-digit")
	}
	for _, d := range accountID {
		if d < '0' || d > '9' {
			return trace.BadParameter("must be 12-digit")
		}
	}

	return nil
}

// IsValidIAMRoleName checks whether the role name is a valid AWS IAM Role identifier.
//
// > Length Constraints: Minimum length of 1. Maximum length of 64.
// > Pattern: [\w+=,.@-]+
// https://docs.aws.amazon.com/IAM/latest/APIReference/API_CreateRole.html
func IsValidIAMRoleName(roleName string) error {
	if len(roleName) == 0 || len(roleName) > 64 || !matchRoleName(roleName) {
		return trace.BadParameter("role is invalid")
	}

	return nil
}

// IsValidIAMPolicyName checks whether the policy name is a valid AWS IAM Policy
// identifier.
//
// > Length Constraints: Minimum length of 1. Maximum length of 128.
// > Pattern: [\w+=,.@-]+
// https://docs.aws.amazon.com/IAM/latest/APIReference/API_CreatePolicy.html
func IsValidIAMPolicyName(policyName string) error {
	// The same regex is used for role and policy names.
	if len(policyName) == 0 || len(policyName) > 128 || !matchRoleName(policyName) {
		return trace.BadParameter("policy name is invalid")
	}
	return nil
}

const (
	// AWSGlobalRegion is a sentinel value used by AWS to be able to use global endpoints, instead of region specific ones.
	// Useful for STS API Calls.
	// https://docs.aws.amazon.com/sdkref/latest/guide/feature-region.html
	AWSGlobalRegion = "aws-global"
)

// IsValidRegion ensures the region looks to be valid.
// It does not do a full validation, because AWS doesn't provide documentation for that.
// However, they usually only have the following chars: [a-z0-9\-]
func IsValidRegion(region string) error {
	if region == AWSGlobalRegion {
		return nil
	}
	if matchRegion.MatchString(region) {
		return nil
	}
	if matchGlobalRegion.MatchString(region) {
		return nil
	}
	return trace.BadParameter("region %q is invalid", region)
}

// IsValidPartition checks if partition is a valid AWS partition
func IsValidPartition(partition string) error {
	if slices.Contains(validPartitions, partition) {
		return nil
	}
	return trace.BadParameter("partition %q is invalid", partition)
}

// IsValidAthenaWorkgroupName checks whether the name is a valid AWS Athena
// workgroup name.
func IsValidAthenaWorkgroupName(workgroup string) error {
	if matchAthenaWorkgroupName(workgroup) {
		return nil
	}
	return trace.BadParameter("athena workgroup name %q is invalid", workgroup)
}

// IsValidGlueResourceName check whether the name is valid for an AWS Glue
// database or table used with AWS Athena
func IsValidGlueResourceName(name string) error {
	if matchGlueName(name) {
		return nil
	}
	return trace.BadParameter("glue resource name %q is invalid", name)
}

const (
	arnDelimiter    = ":"
	arnPrefix       = "arn:"
	arnSections     = 6
	sectionService  = 2 // arn:<partition>:<service>:...
	sectionAccount  = 4 // arn:<partition>:<service>:<region>:<accountid>:...
	sectionResource = 5 // arn:<partition>:<service>:<region>:<accountid>:<resource>
	iamServiceName  = "iam"
)

// CheckRoleARN returns whether a string is a valid IAM Role ARN.
// Example role ARN: arn:aws:iam::123456789012:role/some-role-name
func CheckRoleARN(arn string) error {
	if !strings.HasPrefix(arn, arnPrefix) {
		return trace.BadParameter("arn: invalid prefix: %q", arn)
	}

	sections := strings.SplitN(arn, arnDelimiter, arnSections)
	if len(sections) != arnSections {
		return trace.BadParameter("arn: not enough sections: %q", arn)
	}

	resourceParts := strings.SplitN(sections[sectionResource], "/", 2)

	if resourceParts[0] != "role" || sections[sectionService] != iamServiceName {
		return trace.BadParameter("%q is not an AWS IAM role ARN", arn)
	}

	if len(resourceParts) < 2 || resourceParts[1] == "" {
		return trace.BadParameter("%q is missing AWS IAM role name", arn)
	}

	if err := IsValidAccountID(sections[sectionAccount]); err != nil {
		return trace.BadParameter("%q invalid account ID: %v", arn, err)
	}

	return nil
}

var (
	// matchRoleName is a regex that matches against AWS IAM Role Names.
	matchRoleName = regexp.MustCompile(`^[\w+=,.@-]+$`).MatchString

	// matchRegion is a regex that defines the format of AWS regions.
	//
	// The regex matches the following from left to right:
	// - starts with 2 lower case letters that represents a geo region like a
	//   country code
	// - optional -gov, -iso, -isob for corresponding partitions
	// - a word that should be a direction like "east", "west", etc.
	// - a number counter
	//
	// Reference:
	// https://github.com/aws/aws-sdk-go-v2/blob/main/codegen/smithy-aws-go-codegen/src/main/resources/software/amazon/smithy/aws/go/codegen/endpoints.json
	matchRegion = regexp.MustCompile(`^[a-z]{2}(-gov|-global|-iso|-isob|-iso-b|-isoe|-iso-e|-isof|-iso-f)?-\w+-\d+$`)

	// matchGlobalRegion is a regex that defines the format of AWS global regions.
	// Those regions are usually used for endpoint resolution.
	//
	// The regex matches the following from left to right:
	// - `aws` prefix.
	// - optional -us-gov, -cn, -iso, -iso-b for corresponding partitions
	// - `global` suffix.
	//
	// Reference:
	// https://github.com/aws/aws-sdk-go-v2/blob/main/codegen/smithy-aws-go-codegen/src/main/resources/software/amazon/smithy/aws/go/codegen/endpoints.json
	matchGlobalRegion = regexp.MustCompile(`^aws(-us-gov|-cn|-iso|-iso-b|-iso-e|-iso-f)?-global$`)

	// https://docs.aws.amazon.com/athena/latest/APIReference/API_CreateWorkGroup.html
	matchAthenaWorkgroupName = regexp.MustCompile(`^[a-zA-Z0-9._-]{1,128}$`).MatchString

	// https://docs.aws.amazon.com/athena/latest/ug/tables-databases-columns-names.html
	// More strict than strictly necessary, but a good baseline
	// > database, table, and column names must be 255 characters or fewer
	// > Athena accepts mixed case in DDL and DML queries, but lower cases the names when it executes the query
	// > avoid using mixed case for table or column names
	// > special characters other than underscore (_) are not supported
	matchGlueName = regexp.MustCompile(`^[a-z0-9_]{1,255}$`).MatchString

	// https://docs.aws.amazon.com/IAM/latest/UserGuide/reference-arns.html
	validPartitions = []string{"aws", "aws-cn", "aws-us-gov"}
)
