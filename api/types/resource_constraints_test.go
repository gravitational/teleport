/*
Copyright 2026 Gravitational, Inc.

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

package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDatabaseConstraintsValidate(t *testing.T) {
	tests := []struct {
		name        string
		constraints *DatabaseResourceConstraints
		wantErr     string
	}{
		{
			name:        "empty dimensions rejected",
			constraints: &DatabaseResourceConstraints{},
			wantErr:     "at least one of users, names, or roles",
		},
		{
			name:        "literal values allowed",
			constraints: &DatabaseResourceConstraints{Users: []string{"alice"}, Names: []string{"sales"}, Roles: []string{"reader"}},
		},
		{
			name:        "sole wildcard user allowed",
			constraints: &DatabaseResourceConstraints{Users: []string{Wildcard}},
		},
		{
			name:        "sole wildcard name allowed",
			constraints: &DatabaseResourceConstraints{Names: []string{Wildcard}},
		},
		{
			name:        "wildcard user mixed with literals rejected",
			constraints: &DatabaseResourceConstraints{Users: []string{"alice", Wildcard}},
			wantErr:     `database user constraint "*" must be the only value`,
		},
		{
			name:        "wildcard name mixed with literals rejected",
			constraints: &DatabaseResourceConstraints{Names: []string{Wildcard, "sales"}},
			wantErr:     `database name constraint "*" must be the only value`,
		},
		{
			name:        "wildcard database role rejected",
			constraints: &DatabaseResourceConstraints{Roles: []string{Wildcard}},
			wantErr:     `database role constraints do not support "*"`,
		},
		{
			name:        "wildcard database role rejected among literals",
			constraints: &DatabaseResourceConstraints{Users: []string{"alice"}, Roles: []string{"reader", Wildcard}},
			wantErr:     `database role constraints do not support "*"`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := (&ResourceConstraints_Database{Database: tc.constraints}).Validate()
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}
