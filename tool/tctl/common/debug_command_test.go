// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLooksLikeUUID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  bool
	}{
		{"550e8400-e29b-41d4-a716-446655440000", true},
		{"ABCDEF00-1234-5678-9ABC-DEF012345678", true},
		{"abcdef00-1234-5678-9abc-def012345678", true},
		{"not-a-uuid", false},
		{"", false},
		{"550e8400-e29b-41d4-a716-44665544000", false},   // too short
		{"550e8400-e29b-41d4-a716-4466554400000", false}, // too long
		{"550e8400xe29b-41d4-a716-446655440000", false},  // wrong separator
		{"550e8400-e29b-41d4-a716-44665544000g", false},  // invalid hex
		{"my-hostname", false},
		{"my-server", false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, looksLikeUUID(tt.input), "looksLikeUUID(%q)", tt.input)
	}
}

func TestMatchesComponent(t *testing.T) {
	t.Parallel()
	tests := []struct {
		component string
		patterns  []string
		want      bool
	}{
		// Exact match.
		{"auth", []string{"auth"}, true},
		{"AUTH", []string{"auth"}, true}, // case insensitive
		{"proc", []string{"auth"}, false},

		// Glob wildcards.
		{"proxy:1", []string{"proxy*"}, true},
		{"proxy", []string{"proxy*"}, true},
		{"debug:1", []string{"proxy*"}, false},
		{"anything", []string{"*"}, true},
		{"", []string{"*"}, true},

		// Multiple patterns.
		{"auth", []string{"proc", "auth"}, true},
		{"debug:1", []string{"debug*", "proc*"}, true},
		{"cache", []string{"debug*", "proc*"}, false},

		// Question mark glob.
		{"proc:1", []string{"proc:?"}, true},
		{"proc:12", []string{"proc:?"}, false},

		// Case insensitive (patterns pre-lowered by parseComponentPatterns).
		{"AUTH:1", []string{"auth*"}, true},
		{"auth:1", []string{"auth*"}, true},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, matchesComponent(tt.component, tt.patterns),
			"matchesComponent(%q, %v)", tt.component, tt.patterns)
	}
}

func TestParseComponentPatterns(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  []string
	}{
		{"auth", []string{"auth"}},
		{"auth,proc", []string{"auth", "proc"}},
		{"AUTH,PROC", []string{"auth", "proc"}},
		{" auth , proc ", []string{"auth", "proc"}},
		{"proxy*,auth", []string{"proxy*", "auth"}},
		{"*", []string{"*"}},
		{"", nil},
		{" , , ", nil},
	}
	for _, tt := range tests {
		cmd := &DebugCommand{logStreamComponent: tt.input}
		got := cmd.parseComponentPatterns()
		assert.Equal(t, tt.want, got, "parseComponentPatterns(%q)", tt.input)
	}
}
