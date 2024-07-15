// Copyright 2024 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package types

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	apiutils "github.com/gravitational/teleport/api/utils"
)

func TestDatabasePermission(t *testing.T) {
	tests := []struct {
		name        string
		permissions []string
		match       Labels
		expectedErr error
	}{
		{
			name:        "valid permissions",
			permissions: []string{"read", "write"},
			match:       map[string]apiutils.Strings{"env": {"production"}},
			expectedErr: nil,
		},
		{
			name:        "empty permission list",
			permissions: []string{},
			match:       map[string]apiutils.Strings{"env": {"production"}},
			expectedErr: trace.BadParameter("database permission list cannot be empty"),
		},
		{
			name:        "empty individual permission",
			permissions: []string{"read", ""},
			match:       map[string]apiutils.Strings{"env": {"production"}},
			expectedErr: trace.BadParameter("individual database permissions cannot be empty strings"),
		},
		{
			name:        "wildcard selector with invalid value",
			permissions: []string{"read"},
			match:       map[string]apiutils.Strings{Wildcard: {"invalid"}},
			expectedErr: trace.BadParameter("database permission: selector *:<val> is not supported"),
		},
		{
			name:        "wildcard selector with valid value",
			permissions: []string{"read"},
			match:       map[string]apiutils.Strings{Wildcard: {Wildcard}},
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dbPermission := &DatabasePermission{
				Permissions: tt.permissions,
				Match:       tt.match,
			}

			err := dbPermission.CheckAndSetDefaults()
			require.ErrorIs(t, tt.expectedErr, err)
		})
	}
}
