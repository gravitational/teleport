/*
Copyright 2023 Gravitational, Inc.

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

package resource153

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	datasource_schema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	resource_schema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	apitypes "github.com/gravitational/teleport/api/types"

	"github.com/gravitational/teleport/integrations/terraform/tfschema"
)

func GenSchemaTimestampDataSource(_ context.Context, attr datasource_schema.StringAttribute) datasource_schema.StringAttribute {
	attr.Optional = true
	attr.CustomType = tfschema.UseRFC3339Time()
	return attr
}

func GenSchemaTimestampResource(ctx context.Context, attr resource_schema.StringAttribute) resource_schema.StringAttribute {
	attr.Optional = true
	attr.CustomType = tfschema.UseRFC3339Time()
	return attr
}

func CopyFromTimestamp(diags diag.Diagnostics, v attr.Value, o **timestamppb.Timestamp) {
	value, ok := v.(tfschema.TimeValue)
	if !ok {
		diags.AddError("Error reading from Terraform object", fmt.Sprintf("Can not convert %T to String", v))
		return
	}

	if value.IsNull() || value.IsUnknown() {
		*o = nil
	} else {
		*o = timestamppb.New(value.Value)
	}
}

// CopyToTimestamp converts a Teleport [timestamppb.Timestamp] into a Terraform
// [tfschema.TimeValue]. Set `preserveUnknown` to preserve unknown values.
func CopyToTimestamp(_ diag.Diagnostics, o *timestamppb.Timestamp, _ attr.Type, v attr.Value, preserveUnknown bool) attr.Value {
	if preserveUnknown && v != nil && v.IsUnknown() {
		return tfschema.UnknownTime()
	}

	if o == nil {
		return tfschema.NullTime()
	}

	return tfschema.NewTime(o.AsTime())
}

func GenSchemaDurationDataSource(_ context.Context, attr datasource_schema.StringAttribute) datasource_schema.StringAttribute {
	attr.Optional = true
	attr.CustomType = tfschema.DurationType{}
	return attr
}

func GenSchemaDurationResource(ctx context.Context, attr resource_schema.StringAttribute) resource_schema.StringAttribute {
	attr.Optional = true
	attr.CustomType = tfschema.DurationType{}
	return attr
}

func CopyFromDuration(diags diag.Diagnostics, v attr.Value, o **durationpb.Duration) {
	value, ok := v.(tfschema.DurationValue)
	if !ok {
		diags.AddError("Error reading from Terraform object", fmt.Sprintf("Can not convert %T to String", v))
		return
	}

	if value.IsNull() || value.IsUnknown() {
		*o = nil
		return
	}

	*o = durationpb.New(value.Value)
}

// CopyToDuration converts a Teleport [durationpb.Duration] into a Terraform
// [tfschema.DurationValue]. Set `preserveUnknown` to preserve unknown values.
func CopyToDuration(_ diag.Diagnostics, o *durationpb.Duration, _ attr.Type, v attr.Value, preserveUnknown bool) attr.Value {
	if preserveUnknown && v != nil && v.IsUnknown() {
		return tfschema.DurationValue{Unknown: true}
	}

	if o == nil {
		return tfschema.DurationValue{
			Null: true,
		}
	}

	// Note: Prior Terraform attribute value is returned if possible to preserve
	// the rawValue, which contains the original string value.
	duration := o.AsDuration()
	if existing, ok := v.(tfschema.DurationValue); ok &&
		!existing.IsNull() &&
		!existing.IsUnknown() &&
		existing.ValueDuration() == duration {
		return existing
	}

	return tfschema.DurationValue{
		Value: duration,
	}
}

func GenSchemaLabelsDataSource(ctx context.Context, attr datasource_schema.MapAttribute) datasource_schema.MapAttribute {
	return tfschema.GenSchemaLabelsDataSource(ctx, attr)
}

func GenSchemaLabelsResource(ctx context.Context, attr resource_schema.MapAttribute) resource_schema.MapAttribute {
	return tfschema.GenSchemaLabelsResource(ctx, attr)
}

// GenSchemaStringValuesDataSource exposes a `repeated google.protobuf.StringValue`
// field as a plain list of strings rather than a list of single-field objects.
func GenSchemaStringValuesDataSource(_ context.Context, attr datasource_schema.ListAttribute) datasource_schema.ListAttribute {
	attr.Optional = true
	attr.ElementType = types.StringType
	return attr
}

// GenSchemaStringValuesResource exposes a `repeated google.protobuf.StringValue`
// field as a plain list of strings rather than a list of single-field objects.
func GenSchemaStringValuesResource(ctx context.Context, attr resource_schema.ListAttribute) resource_schema.ListAttribute {
	attr.Optional = true
	attr.ElementType = types.StringType
	return attr
}

// CopyFromStringValues converts a Terraform list of strings into a slice of
// [wrapperspb.StringValue]. The received value must be of List type.
func CopyFromStringValues(diags diag.Diagnostics, v attr.Value, o *[]*wrapperspb.StringValue) {
	value, ok := v.(types.List)
	if !ok {
		diags.AddError("Error reading from Terraform object", fmt.Sprintf("Can not convert %T to types.List", v))
		return
	}

	if value.IsNull() || value.IsUnknown() {
		*o = nil
		return
	}

	*o = make([]*wrapperspb.StringValue, len(value.Elements()))
	for i, tfValue := range value.Elements() {
		tfString, ok := tfValue.(types.String)
		if !ok {
			diags.AddError("Error reading from Terraform object", fmt.Sprintf("Can not convert %T to types.String", tfValue))
			return
		}

		if tfString.IsNull() || tfString.IsUnknown() {
			continue
		}

		(*o)[i] = wrapperspb.String(tfString.ValueString())
	}
}

// CopyToStringValues converts a slice of [wrapperspb.StringValue] into a
// Terraform List with an ElemType of String.
// Set `preserveUnknown` to preserve unknown values.
func CopyToStringValues(_ diag.Diagnostics, o []*wrapperspb.StringValue, _ attr.Type, v attr.Value, preserveUnknown bool) attr.Value {
	listVal, _ := v.(types.List)

	if preserveUnknown && listVal.IsUnknown() {
		return types.ListUnknown(types.StringType)
	}

	if o == nil {
		return types.ListNull(types.StringType)
	}

	elems := make([]attr.Value, len(o))
	for i, stringValue := range o {
		if preserveUnknown &&
			i < len(listVal.Elements()) &&
			listVal.Elements()[i] != nil &&
			listVal.Elements()[i].IsUnknown() {
			elems[i] = types.StringUnknown()
			continue
		}

		if stringValue == nil {
			elems[i] = types.StringNull()
			continue
		}

		elems[i] = types.StringValue(stringValue.GetValue())
	}

	return types.ListValueMust(types.StringType, elems)
}

func CopyFromLabels(diags diag.Diagnostics, v attr.Value, o *apitypes.Labels) {
	tfschema.CopyFromLabels(diags, v, o)
}

// CopyToLabels converts a Teleport [apitypes.Labels] into a Terraform
// `types.Map` value. Set `preserveUnknown` to preserve unknown values.
func CopyToLabels(diags diag.Diagnostics, o apitypes.Labels, t attr.Type, v attr.Value, preserveUnknown bool) attr.Value {
	return tfschema.CopyToLabels(diags, o, t, v, preserveUnknown)
}
