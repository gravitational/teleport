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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetPartitionFromRegion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		region          string
		expectPartition string
	}{
		{
			region:          "cn-north-1",
			expectPartition: "aws-cn",
		},
		{
			region:          "cn-northwest-1",
			expectPartition: "aws-cn",
		},
		{
			region:          "us-gov-east-1",
			expectPartition: "aws-us-gov",
		},
		{
			region:          "us-gov-west-1",
			expectPartition: "aws-us-gov",
		},
		{
			region:          "us-east-1",
			expectPartition: "aws",
		},
		{
			region:          "us-west-1",
			expectPartition: "aws",
		},
		{
			region:          "",
			expectPartition: "aws",
		},
	}

	for _, test := range tests {
		t.Run(test.region, func(t *testing.T) {
			require.Equal(t, test.expectPartition, GetPartitionFromRegion(test.region))
		})
	}
}
