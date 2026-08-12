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

package v1

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"

	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
)

func TestCopyToClassifierActionMode(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		input    summarizerv1.ClassifierActionMode
		expected types.Bool
	}{
		{
			name:     "enabled",
			input:    summarizerv1.ClassifierActionMode_CLASSIFIER_ACTION_MODE_ENABLED,
			expected: types.Bool{Value: true},
		},
		{
			name:     "disabled",
			input:    summarizerv1.ClassifierActionMode_CLASSIFIER_ACTION_MODE_DISABLED,
			expected: types.Bool{Value: false},
		},
		{
			name:     "unspecified",
			input:    summarizerv1.ClassifierActionMode_CLASSIFIER_ACTION_MODE_UNSPECIFIED,
			expected: types.Bool{Null: true},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			diags := diag.Diagnostics{}
			terraformType := types.BoolType
			valueInitial := types.Bool{
				Unknown: true,
			}

			value := CopyToClassifierActionMode(diags, tc.input, terraformType, valueInitial, false)
			require.Empty(t, diags)
			require.Equal(t, tc.expected, value)
		})
	}
}

func TestCopyToClassifierActionModePreserveUnknown(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		input    summarizerv1.ClassifierActionMode
		expected types.Bool
	}{
		{
			name:     "enabled",
			input:    summarizerv1.ClassifierActionMode_CLASSIFIER_ACTION_MODE_ENABLED,
			expected: types.Bool{Unknown: true},
		},
		{
			name:     "disabled",
			input:    summarizerv1.ClassifierActionMode_CLASSIFIER_ACTION_MODE_DISABLED,
			expected: types.Bool{Unknown: true},
		},
		{
			name:     "unspecified",
			input:    summarizerv1.ClassifierActionMode_CLASSIFIER_ACTION_MODE_UNSPECIFIED,
			expected: types.Bool{Unknown: true},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			diags := diag.Diagnostics{}
			terraformType := types.BoolType
			valueInitial := types.Bool{
				Unknown: true,
			}

			value := CopyToClassifierActionMode(diags, tc.input, terraformType, valueInitial, true)
			require.Empty(t, diags)
			require.Equal(t, tc.expected, value)
		})
	}
}

func TestCopyToRiskLevel(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		input    summarizerv1.RiskLevel
		expected types.String
	}{
		{
			name:  "critical",
			input: summarizerv1.RiskLevel_RISK_LEVEL_CRITICAL,
			expected: types.String{
				Value: riskLevelToString[summarizerv1.RiskLevel_RISK_LEVEL_CRITICAL],
			},
		},
		{
			name:  "low",
			input: summarizerv1.RiskLevel_RISK_LEVEL_LOW,
			expected: types.String{
				Value: riskLevelToString[summarizerv1.RiskLevel_RISK_LEVEL_LOW],
			},
		},
		{
			name:  "unspecified",
			input: summarizerv1.RiskLevel_RISK_LEVEL_UNSPECIFIED,
			expected: types.String{
				Null: true,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			diags := diag.Diagnostics{}
			terraformType := types.StringType
			valueInitial := types.String{
				Unknown: true,
			}

			value := CopyToRiskLevel(diags, tc.input, terraformType, valueInitial, false)
			require.Empty(t, diags)
			require.Equal(t, tc.expected, value)
		})
	}
}

func TestCopyToRiskLevelPreserveUnknown(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		input    summarizerv1.RiskLevel
		expected types.String
	}{
		{
			name:     "critical",
			input:    summarizerv1.RiskLevel_RISK_LEVEL_CRITICAL,
			expected: types.String{Unknown: true},
		},
		{
			name:     "low",
			input:    summarizerv1.RiskLevel_RISK_LEVEL_LOW,
			expected: types.String{Unknown: true},
		},
		{
			name:     "unspecified",
			input:    summarizerv1.RiskLevel_RISK_LEVEL_UNSPECIFIED,
			expected: types.String{Unknown: true},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			diags := diag.Diagnostics{}
			terraformType := types.StringType
			valueInitial := types.String{
				Unknown: true,
			}

			value := CopyToRiskLevel(diags, tc.input, terraformType, valueInitial, true)
			require.Empty(t, diags)
			require.Equal(t, tc.expected, value)
		})
	}
}
