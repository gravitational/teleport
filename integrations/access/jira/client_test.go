/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package jira

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildSummary(t *testing.T) {
	for _, tt := range []struct {
		name     string
		reqData  RequestData
		expected string
	}{
		{
			name: "single role",
			reqData: RequestData{
				Roles: []string{"editor"},
				User:  "my-user",
			},
			expected: "my-user requested editor",
		},
		{
			name: "lots of roles that exceed the field size are truncated",
			reqData: RequestData{
				Roles: strings.Split(strings.TrimRight(strings.Repeat("editor,", 1000), ","), ","),
				User:  "my-user",
			},
			expected: "my-user requested access to 1000 roles",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := buildSummary(tt.reqData)
			require.Equal(t, tt.expected, got)
		})
	}
}
