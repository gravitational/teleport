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
	attr.Optional = true
	attr.Type = types.BoolType
	return attr
}

// GenSchemaTraits returns Terraform schema for Traits type
func GenSchemaTraits(_ context.Context, attr tfsdk.Attribute) tfsdk.Attribute {
	attr.Optional = true
	attr.Type = types.MapType{
		ElemType: types.ListType{
			ElemType: types.StringType,
		},
	}
	return attr
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

	if v.IsNull() || v.IsUnknown() {
		*o = nil
		return
	}

	value := apitypes.BoolOption{Value: v.Value}
	*o = &value
}

// CopyToBoolOption converts a Teleport [apitypes.BoolOption] into a Terraform
// [types.Bool] value. Set `preserveUnknown` to preserve unknown values.
func CopyToBoolOption(_ diag.Diagnostics, o *apitypes.BoolOption, _ attr.Type, v attr.Value, preserveUnknown bool) attr.Value {
	unknown := preserveUnknown && v != nil && v.IsUnknown()

	if o == nil {
		return types.Bool{
			Null:    true,
			Unknown: unknown,
		}
	}

	return types.Bool{
		Value:   o.Value,
		Unknown: unknown,
	}
}

func CopyFromLabels(diags diag.Diagnostics, v attr.Value, o *apitypes.Labels) {
	value, ok := v.(types.Map)
	if !ok {
		diags.AddError("Error reading from Terraform object", fmt.Sprintf("Can not convert %T to types.Map", v))
		return
	}

	if value.IsNull() || value.IsUnknown() {
		*o = nil
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

// CopyToLabels converts a Teleport [apitypes.Labels] into a Terraform
// [types.Map] value. Set `preserveUnknown` to preserve unknown values.
func CopyToLabels(_ diag.Diagnostics, o apitypes.Labels, t attr.Type, v attr.Value, preserveUnknown bool) attr.Value {
	typ := t.(types.MapType) // By the convention, t comes type-asserted so there would be no failure

	value, ok := v.(types.Map)
	if !ok {
		value = types.Map{ElemType: typ.ElemType}
	}

	elems := make(map[string]attr.Value, len(o))
	for k, labels := range o {
		elems[k] = copyToList(labels, value.Elems[k], preserveUnknown)
	}

	return types.Map{
		ElemType: typ.ElemType,
		Elems:    elems,
		Unknown:  preserveUnknown && value.IsUnknown(),
	}
}

func CopyFromTraits(diags diag.Diagnostics, v attr.Value, o *wrappers.Traits) {
	value, ok := v.(types.Map)
	if !ok {
		diags.AddError("Error reading from Terraform object", fmt.Sprintf("Can not convert %T to types.Map", v))
		return
	}

	if value.IsNull() || value.IsUnknown() {
		*o = nil
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

// CopyToTraits converts a Teleport [wrappers.Traits] into a Terraform
// [types.Map] value. Set `preserveUnknown` to preserve unknown values.
func CopyToTraits(_ diag.Diagnostics, o wrappers.Traits, t attr.Type, v attr.Value, preserveUnknown bool) attr.Value {
	typ := t.(types.MapType) // By the convention, t comes type-asserted so there would be no failure

	value, ok := v.(types.Map)
	if !ok {
		value = types.Map{ElemType: typ.ElemType}
	}

	elems := make(map[string]attr.Value, len(o))
	for k, traits := range o {
		elems[k] = copyToList(traits, value.Elems[k], preserveUnknown)
	}

	return types.Map{
		ElemType: typ.ElemType,
		Elems:    elems,
		Unknown:  preserveUnknown && value.IsUnknown(),
	}
}

// GenSchemaStrings returns Terraform schema for Strings type
func GenSchemaStrings(_ context.Context, attr tfsdk.Attribute) tfsdk.Attribute {
	attr.Optional = true
	attr.Type = types.ListType{
		ElemType: types.StringType,
	}
	return attr
}

// CopyFromStrings converts from a Terraform value into a Teleport wrappers.Strings
// The received value must be of List type
func CopyFromStrings(diags diag.Diagnostics, v attr.Value, o *wrappers.Strings) {
	value, ok := v.(types.List)
	if !ok {
		diags.AddError("Error reading from Terraform object", fmt.Sprintf("Can not convert %T to types.List", v))
		return
	}

	if value.IsNull() || value.IsUnknown() {
		*o = nil
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
// Set `preserveUnknown` to preserve unknown values.
func CopyToStrings(_ diag.Diagnostics, o wrappers.Strings, _ attr.Type, v attr.Value, preserveUnknown bool) attr.Value {
	return copyToList(o, v, preserveUnknown)
}

func copyToList(strList []string, v attr.Value, preserveUnknown bool) attr.Value {
	listVal, ok := v.(types.List)
	if !ok {
		listVal = types.List{ElemType: types.StringType}
	}

	elems := make([]attr.Value, len(strList))
	for i, str := range strList {
		unknown := preserveUnknown &&
			i < len(listVal.Elems) &&
			listVal.Elems[i] != nil &&
			listVal.Elems[i].IsUnknown()

		elems[i] = types.String{
			Value:   str,
			Unknown: unknown,
		}
	}

	return types.List{
		ElemType: types.StringType,
		Elems:    elems,
		Unknown:  preserveUnknown && listVal.IsUnknown(),
	}
}
