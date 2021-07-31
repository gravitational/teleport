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
	"testing"

	"github.com/stretchr/testify/require"
)

// TestFilterAWSRoles verifies filtering AWS role ARNs by AWS account ID.
func TestFilterAWSRoles(t *testing.T) {
	acc1ARN1 := AWSRole{
		ARN:     "arn:aws:iam::1234567890:role/EC2FullAccess",
		Display: "EC2FullAccess",
	}
	acc1ARN2 := AWSRole{
		ARN:     "arn:aws:iam::1234567890:role/EC2ReadOnly",
		Display: "EC2ReadOnly",
	}
	acc2ARN1 := AWSRole{
		ARN:     "arn:aws:iam::0987654321:role/test-role",
		Display: "test-role",
	}
	invalidARN := AWSRole{
		ARN: "invalid-arn",
	}
	allARNS := []string{
		acc1ARN1.ARN, acc1ARN2.ARN, acc2ARN1.ARN, invalidARN.ARN,
	}
	tests := []struct {
		name      string
		accountID string
		outARNs   []AWSRole
	}{
		{
			name:      "first account roles",
			accountID: "1234567890",
			outARNs:   []AWSRole{acc1ARN1, acc1ARN2},
		},
		{
			name:      "second account roles",
			accountID: "0987654321",
			outARNs:   []AWSRole{acc2ARN1},
		},
		{
			name:      "all roles",
			accountID: "",
			outARNs:   []AWSRole{acc1ARN1, acc1ARN2, acc2ARN1},
		},
	}
	for _, test := range tests {
		require.Equal(t, test.outARNs, FilterAWSRoles(allARNS, test.accountID))
	}
}
