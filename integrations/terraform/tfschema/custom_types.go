/*
Copyright 2015-2022 Gravitational, Inc.

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
	"context"
	fmt "fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	apitypes "github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/api/utils"
)

// GenSchemaBoolOptions returns Terraform schema for BoolOption type
func GenSchemaBoolOption(_ context.Context, attr tfsdk.Attribute) tfsdk.Attribute {
	return tfsdk.Attribute{
		Optional:    true,
		Type:        types.BoolType,
		Description: attr.Description,
	}
}

// GenSchemaBoolOptions returns Terraform schema for Traits type
func GenSchemaTraits(_ context.Context, attr tfsdk.Attribute) tfsdk.Attribute {
	return tfsdk.Attribute{
		Optional: true,
		Type: types.MapType{
			ElemType: types.ListType{
				ElemType: types.StringType,
			},
		},
		Description: attr.Description,
	}
}

// GenSchemaBoolOptions returns Terraform schema for Labels type
func GenSchemaLabels(ctx context.Context, attr tfsdk.Attribute) tfsdk.Attribute {
	return GenSchemaTraits(ctx, attr)
}

func CopyFromBoolOption(diags diag.Diagnostics, tf attr.Value, o **apitypes.BoolOption) {
	v, ok := tf.(types.Bool)
	if !ok {
		diags.AddError("Error reading from Terraform object", fmt.Sprintf("Can not convert %T to types.Bool", tf))
		return
	}
	value := apitypes.BoolOption{Value: v.Value}
	*o = &value
}

func CopyToBoolOption(diags diag.Diagnostics, o *apitypes.BoolOption, t attr.Type, v attr.Value) attr.Value {
	value, ok := v.(types.Bool)
	if !ok {
		value = types.Bool{}
	}

	if o == nil {
		value.Null = true
		return value
	}

	value.Value = o.Value

	return value
}

func CopyFromLabels(diags diag.Diagnostics, v attr.Value, o *apitypes.Labels) {
	value, ok := v.(types.Map)
	if !ok {
		diags.AddError("Error reading from Terraform object", fmt.Sprintf("Can not convert %T to types.Map", v))
		return
	}

	*o = make(map[string]utils.Strings, len(value.Elems))
	for k, e := range value.Elems {
		l, ok := e.(types.List)
		if !ok {
			diags.AddError("Error reading from Terraform object", fmt.Sprintf("Can not convert %T to types.List", l))
			return
		}

		(*o)[k] = make(utils.Strings, len(l.Elems))

		for i, v := range l.Elems {
			s, ok := v.(types.String)
			if !ok {
				diags.AddError("Error reading from Terraform object", fmt.Sprintf("Can not convert %T to types.String", s))
				return
			}

			(*o)[k][i] = s.Value
		}
	}
}

func CopyToLabels(diags diag.Diagnostics, o apitypes.Labels, t attr.Type, v attr.Value) attr.Value {
	typ := t.(types.MapType) // By the convention, t comes type-asserted so there would be no failure

	value, ok := v.(types.Map)
	if !ok {
		value = types.Map{ElemType: typ.ElemType}
	}

	if value.Elems == nil {
		value.Elems = make(map[string]attr.Value, len(o))
	}

	for k, l := range o {
		row := types.List{
			ElemType: types.StringType,
			Elems:    make([]attr.Value, len(l)),
		}

		for i, e := range l {
			row.Elems[i] = types.String{Value: e}
		}

		value.Elems[k] = row
	}

	return value
}

func CopyFromTraits(diags diag.Diagnostics, v attr.Value, o *wrappers.Traits) {
	value, ok := v.(types.Map)
	if !ok {
		diags.AddError("Error reading from Terraform object", fmt.Sprintf("Can not convert %T to types.Map", v))
		return
	}

	*o = make(wrappers.Traits, len(value.Elems))
	for k, e := range value.Elems {
		l, ok := e.(types.List)
		if !ok {
			diags.AddError("Error reading from Terraform object", fmt.Sprintf("Can not convert %T to types.List", l))
			return
		}

		(*o)[k] = make(utils.Strings, len(l.Elems))

		for i, v := range l.Elems {
			s, ok := v.(types.String)
			if !ok {
				diags.AddError("Error reading from Terraform object", fmt.Sprintf("Can not convert %T to types.String", s))
				return
			}

			(*o)[k][i] = s.Value
		}
	}
}

func CopyToTraits(diags diag.Diagnostics, o wrappers.Traits, t attr.Type, v attr.Value) attr.Value {
	typ := t.(types.MapType) // By the convention, t comes type-asserted so there would be no failure

	value, ok := v.(types.Map)
	if !ok {
		value = types.Map{ElemType: typ.ElemType}
	}

	if value.Elems == nil {
		value.Elems = make(map[string]attr.Value, len(o))
	}

	for k, l := range o {
		row := types.List{
			ElemType: types.StringType,
			Elems:    make([]attr.Value, len(l)),
		}

		for i, e := range l {
			row.Elems[i] = types.String{Value: e}
		}

		value.Elems[k] = row
	}

	return value
}

// GenSchemaStrings returns Terraform schema for Strings type
func GenSchemaStrings(_ context.Context, attr tfsdk.Attribute) tfsdk.Attribute {
	return tfsdk.Attribute{
		Optional: true,
		Type: types.ListType{
			ElemType: types.StringType,
		},
		Description: attr.Description,
	}
}

// CopyFromStrings converts from a Terraform value into a Teleport wrappers.Strings
// The received value must be of List type
func CopyFromStrings(diags diag.Diagnostics, v attr.Value, o *wrappers.Strings) {
	value, ok := v.(types.List)
	if !ok {
		diags.AddError("Error reading from Terraform object", fmt.Sprintf("Can not convert %T to types.List", v))
		return
	}

	*o = make(wrappers.Strings, len(value.Elems))
	for i, tfValue := range value.Elems {
		tfString, ok := tfValue.(types.String)
		if !ok {
			diags.AddError("Error reading from Terraform object", fmt.Sprintf("Can not convert %T to types.String", tfValue))
			return
		}

		(*o)[i] = tfString.Value
	}
}

// CopyFromStrings converts from a Teleport wrappers.Strings into a Terraform List with ElemType of String
func CopyToStrings(diags diag.Diagnostics, o wrappers.Strings, t attr.Type, v attr.Value) attr.Value {
	typ := t.(types.ListType) // By the convention, t comes type-asserted so there would be no failure

	value, ok := v.(types.List)
	if !ok {
		value = types.List{ElemType: typ.ElemType}
	}

	if value.Elems == nil {
		value.Elems = make([]attr.Value, len(o))
	}

	for k, l := range o {
		value.Elems[k] = types.String{
			Value: l,
		}
	}

	return value
}
