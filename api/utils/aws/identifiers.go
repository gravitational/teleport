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
