/*
Copyright 2021 Gravitational, Inc.

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

package utils

import (
	"strings"

	"github.com/aws/aws-sdk-go/aws/arn"
)

// FilterAWSRoles returns role ARNs from the provided list that belong to the
// specified AWS account ID.
//
// If AWS account ID is empty, all roles are returned.
func FilterAWSRoles(arns []string, accountID string) (result []AWSRole) {
	for _, roleARN := range arns {
		parsed, err := arn.Parse(roleARN)
		if err != nil || (accountID != "" && parsed.AccountID != accountID) {
			continue
		}
		// Example ARN: arn:aws:iam::1234567890:role/EC2FullAccess.
		parts := strings.Split(parsed.Resource, "/")
		if len(parts) != 2 || parts[0] != "role" {
			continue
		}
		result = append(result, AWSRole{
			Display: parts[1],
			ARN:     roleARN,
		})
	}
	return result
}

// AWSRole describes an AWS IAM role for AWS console access.
type AWSRole struct {
	// Display is the role display name.
	Display string `json:"display"`
	// ARN is the full role ARN.
	ARN string `json:"arn"`
}
