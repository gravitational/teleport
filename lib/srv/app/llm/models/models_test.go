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

package models

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestConvertModelName(t *testing.T) {
	for name, tc := range map[string]struct {
		mappings      []*types.LLM_Model
		fallbackModel string
		reqModel      string
		expected      string
		expectedFound bool
	}{
		"exact match": {
			mappings: []*types.LLM_Model{
				{Name: "claude-sonnet", ProviderName: "us.anthropic.claude-sonnet-v2:0"},
			},
			reqModel:      "claude-sonnet",
			expected:      "us.anthropic.claude-sonnet-v2:0",
			expectedFound: true,
		},
		"case insensitive match": {
			mappings: []*types.LLM_Model{
				{Name: "claude-sonnet", ProviderName: "us.anthropic.claude-sonnet-v2:0"},
			},
			reqModel:      "Claude-Sonnet",
			expected:      "us.anthropic.claude-sonnet-v2:0",
			expectedFound: true,
		},
		"leading and trailing whitespace trimmed": {
			mappings: []*types.LLM_Model{
				{Name: "claude-sonnet", ProviderName: "us.anthropic.claude-sonnet-v2:0"},
			},
			reqModel:      "  claude-sonnet  ",
			expected:      "us.anthropic.claude-sonnet-v2:0",
			expectedFound: true,
		},
		"first matching mapping wins": {
			mappings: []*types.LLM_Model{
				{Name: "claude-sonnet", ProviderName: "first-match"},
				{Name: "claude-sonnet", ProviderName: "second-match"},
			},
			reqModel:      "claude-sonnet",
			expected:      "first-match",
			expectedFound: true,
		},
		"no match returns fallback model": {
			mappings: []*types.LLM_Model{
				{Name: "claude-opus", ProviderName: "us.anthropic.claude-opus:0"},
			},
			fallbackModel: "claude-opus",
			reqModel:      "claude-haiku",
			expected:      "us.anthropic.claude-opus:0",
			expectedFound: true,
		},
		"nil mappings returns provided model": {
			mappings:      nil,
			reqModel:      "claude-sonnet",
			expected:      "claude-sonnet",
			expectedFound: true,
		},
		"empty requested model found false": {
			mappings: []*types.LLM_Model{
				{Name: "claude-sonnet", ProviderName: "us.anthropic.claude-sonnet-v2:0"},
			},
			reqModel:      "",
			expected:      "",
			expectedFound: false,
		},
		"empty fallback model and no match returns empty": {
			mappings: []*types.LLM_Model{
				{Name: "claude-sonnet", ProviderName: "us.anthropic.claude-sonnet-v2:0"},
			},
			fallbackModel: "",
			reqModel:      "unknown-model",
			expected:      "",
			expectedFound: false,
		},
		"match without provider name": {
			mappings: []*types.LLM_Model{
				{Name: "claude-sonnet"},
			},
			reqModel:      "claude-sonnet",
			expected:      "claude-sonnet",
			expectedFound: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			result, found := ConvertName(tc.mappings, tc.fallbackModel, tc.reqModel)
			require.Equal(t, tc.expected, result)
			require.Equal(t, tc.expectedFound, found)
		})
	}
}
