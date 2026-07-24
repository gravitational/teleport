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
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	apitypes "github.com/gravitational/teleport/api/types"

	"github.com/gravitational/teleport/integrations/terraform/tfschema"
)

func GenSchemaTimestamp(_ context.Context, attr tfsdk.Attribute) tfsdk.Attribute {
	attr.Optional = true
	attr.Type = tfschema.UseRFC3339Time()
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
		return tfschema.TimeValue{Unknown: true}
	}

	format := time.RFC3339

	if o == nil {
		return tfschema.TimeValue{
			Null:   true,
			Format: format,
		}
	}

	return tfschema.TimeValue{
		Value:  o.AsTime(),
		Format: format,
	}
}

func GenSchemaDuration(_ context.Context, attr tfsdk.Attribute) tfsdk.Attribute {
	attr.Optional = true
	attr.Type = tfschema.DurationType{}
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
	value, ok := v.(tfschema.DurationValue)
	if !ok {
		value = tfschema.DurationValue{}
	}

	value.Null = false
	value.Value = o.AsDuration()
	value.Unknown = false
	return value
}

func GenSchemaLabels(ctx context.Context, attr tfsdk.Attribute) tfsdk.Attribute {
	return tfschema.GenSchemaLabels(ctx, attr)
}

func CopyFromLabels(diags diag.Diagnostics, v attr.Value, o *apitypes.Labels) {
	tfschema.CopyFromLabels(diags, v, o)
}

// CopyToLabels converts a Teleport [apitypes.Labels] into a Terraform
// `types.Map` value. Set `preserveUnknown` to preserve unknown values.
func CopyToLabels(diags diag.Diagnostics, o apitypes.Labels, t attr.Type, v attr.Value, preserveUnknown bool) attr.Value {
	return tfschema.CopyToLabels(diags, o, t, v, preserveUnknown)
}
