// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package identitycenterutils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnsureSCIMEndpoint(t *testing.T) {
	testCases := []struct {
		name           string
		input          string
		expected       string
		errorAssertion require.ErrorAssertionFunc
	}{
		{
			name:           "missing scheme",
			input:          "scim.ca-central-1.amazonaws.com/bdh6a5e3698-0fc6-4232-a028-fea1a99ff77a/scim/v2",
			errorAssertion: require.Error,
		},
		{
			name:           "non https scheme",
			input:          "http://scim.ca-central-1.amazonaws.com/bdh6a5e3698-0fc6-4232-a028-fea1a99ff77a/scim/v2",
			errorAssertion: require.Error,
		},
		{
			name:           "tcp scheme",
			input:          "tcp://scim.ca-central-1.amazonaws.com/bdh6a5e3698-0fc6-4232-a028-fea1a99ff77a/scim/v2",
			errorAssertion: require.Error,
		},
		{
			name:           "invalid region",
			input:          "https://scim.test.amazonaws.com/bdh6a5e3698-0fc6-4232-a028-fea1a99ff77a/scim/v2",
			errorAssertion: require.Error,
		},
		{
			name:           "invalid region - with a domain",
			input:          "https://scim.anotherdomain:8080/.amazonaws.com/bdh6a5e3698-0fc6-4232-a028-fea1a99ff77a/scim/v2",
			errorAssertion: require.Error,
		},
		{
			name:           "invalid random id - contains URL",
			input:          "https://scim.ca-central-1.amazonaws.com/http://example.com/scim/v2",
			errorAssertion: require.Error,
		},
		{
			name:           "invalid random id - contains another host",
			input:          "https://scim.ca-central-1.amazonaws.com/.anotherdomain.com/scim/v2",
			errorAssertion: require.Error,
		},
		{
			name:           "invalid path",
			input:          "scim.ca-central-1.amazonaws.com/@example.com",
			errorAssertion: require.Error,
		},
		{
			name:           "non-scim subdomain",
			input:          "https://example.ca-central-1.example.com/bdh6a5e3698-0fc6-4232-a028-fea1a99ff77a/scim/v2",
			errorAssertion: require.Error,
		},
		{
			name:           "non-v2 version",
			input:          "https://scim.ca-central-1.amazonaws.com/bdh6a5e3698-0fc6-4232-a028-fea1a99ff77a/scim/v10",
			errorAssertion: require.Error,
		},
		{
			name:           "non-amazonaws.com domain",
			input:          "https://scim.ca-central-1.amazonaws.io/bdh6a5e3698-0fc6-4232-a028-fea1a99ff77a/scim/v2",
			errorAssertion: require.Error,
		},
		{
			name:           "valid base URL",
			input:          "https://scim.ca-central-1.amazonaws.com/bdh6a5e3698-0fc6-4232-a028-fea1a99ff77a/scim/v2",
			expected:       "https://scim.ca-central-1.amazonaws.com/bdh6a5e3698-0fc6-4232-a028-fea1a99ff77a/scim/v2",
			errorAssertion: require.NoError,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ensuredURL, err := EnsureSCIMEndpoint(tc.input)
			tc.errorAssertion(t, err)
			if tc.expected != "" {
				require.Equal(t, tc.expected, ensuredURL)
			}
		})
	}
}
