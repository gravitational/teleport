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
	tests := []struct {
		name            string
		region          string
		expectPartition string
	}{
		{
			name:            "cn north region",
			region:          "cn-north-1",
			expectPartition: "aws-cn",
		},
		{
			name:            "cn north west region ",
			region:          "cn-northwest-1",
			expectPartition: "aws-cn",
		},
		{
			name:            "US West (Northern California) Region",
			region:          "us-west-1",
			expectPartition: "aws",
		},
		{
			name:            "region is null",
			region:          "",
			expectPartition: "aws",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, test.expectPartition, GetPartitionFromRegion(test.region))
		})
	}
}
