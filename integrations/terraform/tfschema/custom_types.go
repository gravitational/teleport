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
	datasource_schema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	resource_schema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	apitypes "github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/api/utils"
)

// GenSchemaBoolOptionDataSource returns Terraform schema for BoolOption type
func GenSchemaBoolOptionDataSource(_ context.Context, attr datasource_schema.BoolAttribute) datasource_schema.BoolAttribute {
	attr.Optional = true
	return attr
}

// GenSchemaBoolOptionResource returns Terraform schema for BoolOption type
func GenSchemaBoolOptionResource(_ context.Context, attr resource_schema.BoolAttribute) resource_schema.BoolAttribute {
	attr.Optional = true
	return attr
}

// GenSchemaTraitsDataSource returns Terraform schema for Traits type.
func GenSchemaTraitsDataSource(_ context.Context, attr datasource_schema.MapAttribute) datasource_schema.MapAttribute {
	attr.Optional = true
	attr.ElementType = types.ListType{
		ElemType: types.StringType,
	}
	return attr
}

// GenSchemaTraitsResource returns Terraform schema for Traits type.
func GenSchemaTraitsResource(ctx context.Context, attr resource_schema.MapAttribute) resource_schema.MapAttribute {
	attr.Optional = true
	attr.ElementType = types.ListType{
		ElemType: types.StringType,
	}
	return attr
}

// GenSchemaLabelsDataSource returns Terraform schema for Labels type.
func GenSchemaLabelsDataSource(ctx context.Context, attr datasource_schema.MapAttribute) datasource_schema.MapAttribute {
	return GenSchemaTraitsDataSource(ctx, attr)
}

// GenSchemaLabelsResource returns Terraform schema for Labels type.
func GenSchemaLabelsResource(ctx context.Context, attr resource_schema.MapAttribute) resource_schema.MapAttribute {
	return GenSchemaTraitsResource(ctx, attr)
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

	value := apitypes.BoolOption{Value: v.ValueBool()}
	*o = &value
}

// CopyToBoolOption converts a Teleport [apitypes.BoolOption] into a Terraform
// [types.Bool] value. Set `preserveUnknown` to preserve unknown values.
func CopyToBoolOption(_ diag.Diagnostics, o *apitypes.BoolOption, _ attr.Type, v attr.Value, preserveUnknown bool) attr.Value {
	if preserveUnknown && v != nil && v.IsUnknown() {
		return types.BoolUnknown()
	}

	if o == nil {
		return types.BoolNull()
	}

	return types.BoolValue(o.Value)
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

	*o = make(map[string]utils.Strings, len(value.Elements()))
	for k, e := range value.Elements() {
		l, ok := e.(types.List)
		if !ok {
			diags.AddError("Error reading from Terraform object", fmt.Sprintf("Can not convert %T to types.List", l))
			return
		}

		(*o)[k] = make(utils.Strings, len(l.Elements()))

		for i, v := range l.Elements() {
			s, ok := v.(types.String)
			if !ok {
				diags.AddError("Error reading from Terraform object", fmt.Sprintf("Can not convert %T to types.String", s))
				return
			}

			(*o)[k][i] = s.ValueString()
		}
	}
}

// CopyToLabels converts a Teleport [apitypes.Labels] into a Terraform
// [types.Map] value. Set `preserveUnknown` to preserve unknown values.
func CopyToLabels(diags diag.Diagnostics, o apitypes.Labels, t attr.Type, v attr.Value, preserveUnknown bool) attr.Value {
	typ := t.(types.MapType) // By the convention, t comes type-asserted so there would be no failure

	if preserveUnknown && v != nil && v.IsUnknown() {
		return types.MapUnknown(typ.ElemType)
	}

	value, _ := v.(types.Map)

	elems := make(map[string]attr.Value, len(o))
	for k, labels := range o {
		listValue, copyDiags := copyToList(labels, value.Elements()[k], preserveUnknown)
		diags.Append(copyDiags...)
		if diags.HasError() {
			return types.MapUnknown(typ.ElemType)
		}
		elems[k] = listValue
	}

	labels, labelsDiags := types.MapValue(typ.ElemType, elems)
	diags.Append(labelsDiags...)
	return labels
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

	*o = make(wrappers.Traits, len(value.Elements()))
	for k, e := range value.Elements() {
		l, ok := e.(types.List)
		if !ok {
			diags.AddError("Error reading from Terraform object", fmt.Sprintf("Can not convert %T to types.List", l))
			return
		}

		(*o)[k] = make(utils.Strings, len(l.Elements()))

		for i, v := range l.Elements() {
			s, ok := v.(types.String)
			if !ok {
				diags.AddError("Error reading from Terraform object", fmt.Sprintf("Can not convert %T to types.String", s))
				return
			}

			(*o)[k][i] = s.ValueString()
		}
	}
}

// CopyToTraits converts a Teleport [wrappers.Traits] into a Terraform
// [types.Map] value. Set `preserveUnknown` to preserve unknown values.
func CopyToTraits(diags diag.Diagnostics, o wrappers.Traits, t attr.Type, v attr.Value, preserveUnknown bool) attr.Value {
	typ := t.(types.MapType) // By the convention, t comes type-asserted so there would be no failure

	if preserveUnknown && v != nil && v.IsUnknown() {
		return types.MapUnknown(typ.ElemType)
	}

	value, _ := v.(types.Map)

	elems := make(map[string]attr.Value, len(o))
	for k, labels := range o {
		listValue, copyDiags := copyToList(labels, value.Elements()[k], preserveUnknown)
		diags.Append(copyDiags...)
		if diags.HasError() {
			return types.MapUnknown(typ.ElemType)
		}
		elems[k] = listValue
	}

	traits, traitsDiags := types.MapValue(typ.ElemType, elems)
	diags.Append(traitsDiags...)
	return traits
}

// GenSchemaStringsDataSource returns Terraform schema for Strings type.
func GenSchemaStringsDataSource(_ context.Context, attr datasource_schema.ListAttribute) datasource_schema.ListAttribute {
	attr.Optional = true
	attr.ElementType = types.StringType
	return attr
}

// GenSchemaStringsResource returns Terraform schema for Strings type.
func GenSchemaStringsResource(ctx context.Context, attr resource_schema.ListAttribute) resource_schema.ListAttribute {
	attr.Optional = true
	attr.ElementType = types.StringType
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

	*o = make(wrappers.Strings, len(value.Elements()))
	for i, tfValue := range value.Elements() {
		tfString, ok := tfValue.(types.String)
		if !ok {
			diags.AddError("Error reading from Terraform object", fmt.Sprintf("Can not convert %T to types.String", tfValue))
			return
		}

		(*o)[i] = tfString.ValueString()
	}
}

// CopyFromStrings converts from a Teleport wrappers.Strings into a Terraform List with ElemType of String
// Set `preserveUnknown` to preserve unknown values.
func CopyToStrings(diags diag.Diagnostics, o wrappers.Strings, _ attr.Type, v attr.Value, preserveUnknown bool) attr.Value {
	value, copyDiags := copyToList(o, v, preserveUnknown)
	diags.Append(copyDiags...)
	return value
}

func copyToList(strList []string, v attr.Value, preserveUnknown bool) (attr.Value, diag.Diagnostics) {
	listVal, _ := v.(types.List)

	if preserveUnknown && listVal.IsUnknown() {
		return types.ListUnknown(types.StringType), nil
	}

	elems := make([]attr.Value, len(strList))
	for i, str := range strList {
		if preserveUnknown &&
			i < len(listVal.Elements()) &&
			listVal.Elements()[i] != nil &&
			listVal.Elements()[i].IsUnknown() {
			elems[i] = types.StringUnknown()
			continue
		}

		elems[i] = types.StringValue(str)
	}

	return types.ListValue(types.StringType, elems)
}

// GenSchemaDurationDataSource returns Terraform schema for duration type.
func GenSchemaDurationDataSource(_ context.Context, attr datasource_schema.StringAttribute) datasource_schema.StringAttribute {
	attr.CustomType = DurationType{}
	return attr
}

// GenSchemaDurationResource returns Terraform schema for duration type.
func GenSchemaDurationResource(_ context.Context, attr resource_schema.StringAttribute) resource_schema.StringAttribute {
	attr.CustomType = DurationType{}
	return attr
}

// CopyFromDuration converts a Terraform value into a Teleport duration.
func CopyFromDuration(diags diag.Diagnostics, v attr.Value, o *apitypes.Duration) {
	value, ok := v.(DurationValue)
	if !ok {
		diags.AddError("Error reading from Terraform object", fmt.Sprintf("Can not convert %T to String", v))
		return
	}

	if value.IsNull() || value.IsUnknown() {
		return
	}

	*o = apitypes.Duration(value.ValueDuration())
}

// CopyToDuration converts from a Teleport duration into a Terraform duration value.
// Set `preserveUnknown` to preserve unknown values.
func CopyToDuration(_ diag.Diagnostics, o apitypes.Duration, _ attr.Type, v attr.Value, preserveUnknown bool) attr.Value {
	if preserveUnknown && v != nil && v.IsUnknown() {
		return DurationValue{Unknown: true}
	}

	if existing, ok := v.(DurationValue); ok &&
		!existing.IsNull() &&
		!existing.IsUnknown() &&
		existing.ValueDuration() == o.Value() {
		return existing
	}

	return NewDuration(o.Value())
}
