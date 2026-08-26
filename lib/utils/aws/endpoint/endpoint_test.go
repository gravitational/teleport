// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package endpoint_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils/aws/endpoint"
)

func TestCreateURI(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		insecure bool
		expected string
	}{
		{
			name: "empty endpoint",
		},
		{
			name:     "valid endpoint",
			endpoint: "https://test.example.com",
			expected: "https://test.example.com",
		},
		{
			name:     "invalid insecure endpoint",
			endpoint: "test.example.com",
			insecure: true,
			expected: "http://test.example.com",
		},
		{
			name:     "invalid secure endpoint",
			endpoint: "test.example.com",
			expected: "https://test.example.com",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			out := endpoint.CreateURI(test.endpoint, test.insecure)
			require.Equal(t, test.expected, out)
		})
	}
}
