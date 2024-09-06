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

	"github.com/gravitational/teleport/api/types/wrappers"
)

func TestStringsCopyTo(t *testing.T) {
	t.Parallel()

	diags := diag.Diagnostics{}
	input := wrappers.Strings{"hello", "world"}
	expected := []attr.Value{
		types.String{Value: "hello"},
		types.String{Value: "world"},
	}
	terraformType := types.ListType{}
	valueInitial := types.List{}

	value := CopyToStrings(diags, input, terraformType, valueInitial)

	require.Empty(t, diags)
	require.ElementsMatch(t, expected, value.(types.List).Elems)
}

func TestStringsCopyFrom(t *testing.T) {
	t.Parallel()

	diags := diag.Diagnostics{}
	input := types.List{
		ElemType: types.StringType,
		Elems: []attr.Value{
			types.String{Value: "hello"},
			types.String{Value: "world"},
		},
	}
	expected := wrappers.Strings{"hello", "world"}
	got := wrappers.Strings{}

	CopyFromStrings(diags, input, &got)

	require.Empty(t, diags)
	require.ElementsMatch(t, expected, got)
}
