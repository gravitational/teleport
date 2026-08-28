/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// envValue returns the value for key in an instanceEnv result, and how many
// entries for that key were found (so tests can assert there's never more
// than one).
func envValue(env []string, key string) (value string, count int) {
	for _, kv := range env {
		k, v, ok := strings.Cut(kv, "=")
		if ok && k == key {
			value = v
			count++
		}
	}
	return value, count
}

func TestInstanceEnv(t *testing.T) {
	tests := []struct {
		name      string
		setEnv    map[string]string
		overrides map[string]string
		checkKey  string
		wantCount int
		wantValue string
	}{
		{
			name:      "strip key with no override is absent entirely",
			setEnv:    map[string]string{"TELEPORT_CLOUD_HOSTPORT": "ambient.example.com"},
			checkKey:  "TELEPORT_CLOUD_HOSTPORT",
			wantCount: 0,
		},
		{
			name:      "override wins over ambient value with no duplicate",
			setEnv:    map[string]string{"TELEPORT_CLOUD_HOSTPORT": "ambient.example.com"},
			overrides: map[string]string{"TELEPORT_CLOUD_HOSTPORT": "declared.example.com"},
			checkKey:  "TELEPORT_CLOUD_HOSTPORT",
			wantCount: 1,
			wantValue: "declared.example.com",
		},
		{
			name:      "non-strip var passes through unchanged",
			setEnv:    map[string]string{"SOME_UNRELATED_VAR": "hello"},
			checkKey:  "SOME_UNRELATED_VAR",
			wantCount: 1,
			wantValue: "hello",
		},
		{
			name:      "override of a non-strip var still dedupes",
			setEnv:    map[string]string{"SOME_UNRELATED_VAR": "ambient"},
			overrides: map[string]string{"SOME_UNRELATED_VAR": "overridden"},
			checkKey:  "SOME_UNRELATED_VAR",
			wantCount: 1,
			wantValue: "overridden",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.setEnv {
				t.Setenv(k, v)
			}

			env := instanceEnv(tt.overrides)

			value, count := envValue(env, tt.checkKey)
			require.Equal(t, tt.wantCount, count)
			if tt.wantCount > 0 {
				require.Equal(t, tt.wantValue, value)
			}
		})
	}
}
