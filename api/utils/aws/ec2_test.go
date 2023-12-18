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

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestIsEC2NodeID(t *testing.T) {
	// EC2 Node IDs are {AWS account ID}-{EC2 resource ID} eg:
	//   123456789012-i-1234567890abcdef0
	// AWS account ID is always a 12 digit number, see
	//   https://docs.aws.amazon.com/general/latest/gr/acct-identifiers.html
	// EC2 resource ID is i-{8 or 17 hex digits}, see
	//   https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/resource-ids.html
	testCases := []struct {
		name     string
		id       string
		expected bool
	}{
		{
			name:     "8 digit",
			id:       "123456789012-i-12345678",
			expected: true,
		},
		{
			name:     "17 digit",
			id:       "123456789012-i-1234567890abcdef0",
			expected: true,
		},
		{
			name:     "foo",
			id:       "foo",
			expected: false,
		},
		{
			name:     "uuid",
			id:       uuid.NewString(),
			expected: false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, IsEC2NodeID(tc.id))
		})
	}
}
