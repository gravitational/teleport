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

package tfschema

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"

	apitypes "github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/api/utils"
)

func TestCopyFromBoolOption(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		input    types.Bool
		expected *apitypes.BoolOption
	}{
		{
			name:     "true",
			input:    types.Bool{Value: true},
			expected: &apitypes.BoolOption{Value: true},
		},
		{
			name:     "false",
			input:    types.Bool{Value: false},
			expected: &apitypes.BoolOption{Value: false},
		},
		{
			name:     "null",
			input:    types.Bool{Null: true},
			expected: nil,
		},
		{
			name:     "unknown",
			input:    types.Bool{Unknown: true},
			expected: nil,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			diags := diag.Diagnostics{}
			output := &apitypes.BoolOption{}

			CopyFromBoolOption(diags, tc.input, &output)
			require.Empty(t, diags)
			require.Equal(t, tc.expected, output)
		})
	}
}

func TestCopyToBoolOption(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		input    *apitypes.BoolOption
		expected types.Bool
	}{
		{
			name:     "true",
			input:    &apitypes.BoolOption{Value: true},
			expected: types.Bool{Value: true},
		},
		{
			name:     "false",
			input:    &apitypes.BoolOption{Value: false},
			expected: types.Bool{Value: false},
		},
		{
			name:     "null",
			input:    nil,
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

			value := CopyToBoolOption(diags, tc.input, terraformType, valueInitial)
			require.Empty(t, diags)
			require.Equal(t, tc.expected, value)
		})
	}
}

var labelListType = types.ListType{
	ElemType: types.StringType,
}

var labelMapType = types.MapType{
	ElemType: labelListType,
}

func labelList(values ...string) types.List {
	elems := make([]attr.Value, len(values))
	for i, value := range values {
		elems[i] = types.String{Value: value}
	}
	return types.List{
		ElemType: types.StringType,
		Elems:    elems,
	}
}

func TestCopyFromLabels(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		input    attr.Value
		expected apitypes.Labels
	}{
		{
			name: "single label",
			input: types.Map{
				ElemType: labelListType,
				Elems: map[string]attr.Value{
					"foo": labelList("bar"),
				},
			},
			expected: apitypes.Labels{
				"foo": utils.Strings{"bar"},
			},
		},
		{
			name: "multiple labels and values",
			input: types.Map{
				ElemType: labelListType,
				Elems: map[string]attr.Value{
					"env":  labelList("prod", "staging"),
					"team": labelList("cloud"),
				},
			},
			expected: apitypes.Labels{
				"env":  utils.Strings{"prod", "staging"},
				"team": utils.Strings{"cloud"},
			},
		},
		{
			name: "empty",
			input: types.Map{
				ElemType: labelListType,
				Elems:    map[string]attr.Value{},
			},
			expected: apitypes.Labels{},
		},
		{
			name: "null",
			input: types.Map{
				ElemType: labelListType,
				Null:     true,
			},
			expected: nil,
		},
		{
			name: "unknown",
			input: types.Map{
				ElemType: labelListType,
				Unknown:  true,
			},
			expected: nil,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			diags := diag.Diagnostics{}
			output := apitypes.Labels{
				"existing": utils.Strings{"value"},
			}

			CopyFromLabels(diags, tc.input, &output)
			require.Empty(t, diags)
			require.Equal(t, tc.expected, output)
		})
	}
}

func TestCopyFromTraits(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		input    attr.Value
		expected wrappers.Traits
	}{
		{
			name: "single trait",
			input: types.Map{
				ElemType: labelListType,
				Elems: map[string]attr.Value{
					"logins": labelList("root"),
				},
			},
			expected: wrappers.Traits{
				"logins": []string{"root"},
			},
		},
		{
			name: "multiple traits and values",
			input: types.Map{
				ElemType: labelListType,
				Elems: map[string]attr.Value{
					"logins": labelList("root", "ubuntu"),
					"roles":  labelList("admin"),
				},
			},
			expected: wrappers.Traits{
				"logins": []string{"root", "ubuntu"},
				"roles":  []string{"admin"},
			},
		},
		{
			name: "empty",
			input: types.Map{
				ElemType: labelListType,
				Elems:    map[string]attr.Value{},
			},
			expected: wrappers.Traits{},
		},
		{
			name: "null",
			input: types.Map{
				ElemType: labelListType,
				Null:     true,
			},
			expected: nil,
		},
		{
			name: "unknown",
			input: types.Map{
				ElemType: labelListType,
				Unknown:  true,
			},
			expected: nil,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			diags := diag.Diagnostics{}
			output := wrappers.Traits{
				"existing": []string{"value"},
			}

			CopyFromTraits(diags, tc.input, &output)
			require.Empty(t, diags)
			require.Equal(t, tc.expected, output)
		})
	}
}

func TestCopyToLabels(t *testing.T) {
	t.Parallel()

	requireLabels := func(t *testing.T, value attr.Value, expected apitypes.Labels) {
		t.Helper()

		mapVal, ok := value.(types.Map)
		require.True(t, ok)
		require.False(t, mapVal.IsNull())
		require.False(t, mapVal.IsUnknown())
		require.Len(t, mapVal.Elems, len(expected))

		for key, expectedValues := range expected {
			listVal, ok := mapVal.Elems[key].(types.List)
			require.True(t, ok)
			require.False(t, listVal.IsNull())
			require.False(t, listVal.IsUnknown())
			require.Len(t, listVal.Elems, len(expectedValues))

			for i, expectedValue := range expectedValues {
				require.Equal(t, types.String{Value: expectedValue}, listVal.Elems[i])
			}
		}
	}

	for _, tc := range []struct {
		name     string
		input    apitypes.Labels
		initial  attr.Value
		expected apitypes.Labels
	}{
		{
			name: "multiple labels and values",
			input: apitypes.Labels{
				"env":  utils.Strings{"prod", "staging"},
				"team": utils.Strings{"cloud"},
			},
			initial: types.Map{
				ElemType: labelListType,
				Unknown:  true,
			},
			expected: apitypes.Labels{
				"env":  utils.Strings{"prod", "staging"},
				"team": utils.Strings{"cloud"},
			},
		},
		{
			name: "replaces existing labels",
			input: apitypes.Labels{
				"foo": utils.Strings{"new"},
			},
			initial: types.Map{
				ElemType: labelListType,
				Elems: map[string]attr.Value{
					"foo":   labelList("old"),
					"stale": labelList("value"),
				},
			},
			expected: apitypes.Labels{
				"foo": utils.Strings{"new"},
			},
		},
		{
			name:     "empty labels",
			input:    apitypes.Labels{},
			initial:  types.Map{ElemType: labelListType},
			expected: apitypes.Labels{},
		},
		{
			name: "nil labels",
			initial: types.Map{
				ElemType: labelListType,
				Elems: map[string]attr.Value{
					"stale": labelList("value"),
				},
			},
			expected: apitypes.Labels{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			diags := diag.Diagnostics{}

			value := CopyToLabels(diags, tc.input, labelMapType, tc.initial)
			require.Empty(t, diags)
			requireLabels(t, value, tc.expected)
		})
	}
}

func TestCopyToTraits(t *testing.T) {
	t.Parallel()

	requireTraits := func(t *testing.T, value attr.Value, expected wrappers.Traits) {
		t.Helper()

		mapVal, ok := value.(types.Map)
		require.True(t, ok)
		require.False(t, mapVal.IsNull())
		require.False(t, mapVal.IsUnknown())
		require.Len(t, mapVal.Elems, len(expected))

		for key, expectedValues := range expected {
			listVal, ok := mapVal.Elems[key].(types.List)
			require.True(t, ok)
			require.False(t, listVal.IsNull())
			require.False(t, listVal.IsUnknown())
			require.Len(t, listVal.Elems, len(expectedValues))

			for i, expectedValue := range expectedValues {
				require.Equal(t, types.String{Value: expectedValue}, listVal.Elems[i])
			}
		}
	}

	for _, tc := range []struct {
		name     string
		input    wrappers.Traits
		initial  attr.Value
		expected wrappers.Traits
	}{
		{
			name: "multiple traits and values",
			input: wrappers.Traits{
				"logins": []string{"root", "ubuntu"},
				"roles":  []string{"admin"},
			},
			initial: types.Map{
				ElemType: types.StringType,
				Unknown:  true,
			},
			expected: wrappers.Traits{
				"logins": []string{"root", "ubuntu"},
				"roles":  []string{"admin"},
			},
		},
		{
			name: "replaces existing traits",
			input: wrappers.Traits{
				"logins": []string{"ubuntu"},
			},
			initial: types.Map{
				ElemType: labelListType,
				Elems: map[string]attr.Value{
					"logins": labelList("root"),
					"stale":  labelList("value"),
				},
			},
			expected: wrappers.Traits{
				"logins": []string{"ubuntu"},
			},
		},
		{
			name:     "empty traits",
			input:    wrappers.Traits{},
			initial:  types.Map{ElemType: labelListType},
			expected: wrappers.Traits{},
		},
		{
			name: "nil traits",
			initial: types.Map{
				ElemType: labelListType,
				Elems: map[string]attr.Value{
					"stale": labelList("value"),
				},
			},
			expected: wrappers.Traits{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			diags := diag.Diagnostics{}

			value := CopyToTraits(diags, tc.input, labelMapType, tc.initial)
			require.Empty(t, diags)
			requireTraits(t, value, tc.expected)
		})
	}
}

func TestStringsCopyTo(t *testing.T) {
	t.Parallel()

	requireStrings := func(t *testing.T, value attr.Value, expected wrappers.Strings) {
		t.Helper()

		listVal, ok := value.(types.List)
		require.True(t, ok)
		require.False(t, listVal.IsNull())
		require.False(t, listVal.IsUnknown())
		require.Equal(t, types.StringType, listVal.ElemType)
		require.Len(t, listVal.Elems, len(expected))

		for i, expectedValue := range expected {
			require.Equal(t, types.String{Value: expectedValue}, listVal.Elems[i])
		}
	}

	for _, tc := range []struct {
		name     string
		input    wrappers.Strings
		initial  attr.Value
		expected wrappers.Strings
	}{
		{
			name: "multiple strings",
			input: wrappers.Strings{
				"hello",
				"world",
			},
			initial: types.List{
				Unknown: true,
			},
			expected: wrappers.Strings{
				"hello",
				"world",
			},
		},
		{
			name: "replaces existing strings",
			input: wrappers.Strings{
				"new",
			},
			initial: types.List{
				ElemType: types.StringType,
				Elems: []attr.Value{
					types.String{Value: "old"},
					types.String{Value: "stale"},
				},
			},
			expected: wrappers.Strings{
				"new",
			},
		},
		{
			name:     "empty strings",
			input:    wrappers.Strings{},
			initial:  types.List{ElemType: types.StringType},
			expected: wrappers.Strings{},
		},
		{
			name: "nil strings",
			initial: types.List{
				ElemType: types.StringType,
				Elems: []attr.Value{
					types.String{Value: "stale"},
				},
			},
			expected: wrappers.Strings{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			diags := diag.Diagnostics{}

			value := CopyToStrings(diags, tc.input, labelListType, tc.initial)
			require.Empty(t, diags)
			requireStrings(t, value, tc.expected)
		})
	}
}

func TestStringsCopyFrom(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		input    attr.Value
		expected wrappers.Strings
	}{
		{
			name: "multiple strings",
			input: types.List{
				ElemType: types.StringType,
				Elems: []attr.Value{
					types.String{Value: "hello"},
					types.String{Value: "world"},
				},
			},
			expected: wrappers.Strings{
				"hello",
				"world",
			},
		},
		{
			name: "empty",
			input: types.List{
				ElemType: types.StringType,
				Elems:    []attr.Value{},
			},
			expected: wrappers.Strings{},
		},
		{
			name: "null",
			input: types.List{
				ElemType: types.StringType,
				Null:     true,
			},
			expected: nil,
		},
		{
			name: "unknown",
			input: types.List{
				ElemType: types.StringType,
				Unknown:  true,
			},
			expected: nil,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			diags := diag.Diagnostics{}
			got := wrappers.Strings{
				"existing",
			}

			CopyFromStrings(diags, tc.input, &got)
			require.Empty(t, diags)
			require.Equal(t, tc.expected, got)
		})
	}
}
