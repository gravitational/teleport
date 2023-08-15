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
	"strings"
	"unicode"

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

// matchRoleName is a regex that matches against AWS IAM Role Names.
var matchRoleName = regexp.MustCompile(`^[\w+=,.@-]+$`).MatchString

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

// IsValidRegion ensures the region looks to be valid.
// It does not do a full validation, because AWS doesn't provide documentation for that.
// However, they usually only have the following chars: [a-z0-9\-]
func IsValidRegion(region string) error {
	indexNotFound := -1

	if len(region) == 0 {
		return trace.BadParameter("region is invalid")
	}

	if strings.IndexFunc(region, func(r rune) bool {
		return !(unicode.IsDigit(r) || unicode.IsLetter(r) || r == '-')
	}) == indexNotFound {
		return nil
	}

	return trace.BadParameter("region is invalid")
}
