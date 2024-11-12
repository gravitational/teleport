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
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/integrations/terraform/tfschema"
)

func GenSchemaTimestamp(_ context.Context, attr tfsdk.Attribute) tfsdk.Attribute {
	return tfsdk.Attribute{
		Optional:    true,
		Type:        tfschema.UseRFC3339Time(),
		Description: attr.Description,
	}
}

func CopyFromTimestamp(diags diag.Diagnostics, v attr.Value, o **timestamppb.Timestamp) {
	value, ok := v.(tfschema.TimeValue)
	if !ok {
		diags.AddError("Error reading from Terraform object", fmt.Sprintf("Can not convert %T to String", v))
		return
	}

	if value.IsNull() {
		*o = nil
	} else {
		*o = timestamppb.New(value.Value)
	}
}

func CopyToTimestamp(diags diag.Diagnostics, o *timestamppb.Timestamp, t attr.Type, v attr.Value) attr.Value {
	value, ok := v.(tfschema.TimeValue)
	if !ok {
		value = tfschema.TimeValue{}
	}

	if o == nil {
		value.Null = true
		return value
	}

	value.Value = (*o).AsTime()

	return value
}

func GenSchemaDuration(_ context.Context, attr tfsdk.Attribute) tfsdk.Attribute {
	return tfsdk.Attribute{
		Optional:    true,
		Type:        tfschema.DurationType{},
		Description: attr.Description,
	}
}

func CopyFromDuration(diags diag.Diagnostics, v attr.Value, o **durationpb.Duration) {
	value, ok := v.(tfschema.DurationValue)
	if !ok {
		diags.AddError("Error reading from Terraform object", fmt.Sprintf("Can not convert %T to String", v))
		return
	}

	*o = durationpb.New(value.Value)
}

func CopyToDuration(diags diag.Diagnostics, o *durationpb.Duration, t attr.Type, v attr.Value) attr.Value {
	value, ok := v.(tfschema.DurationValue)
	if !ok {
		value = tfschema.DurationValue{}
	}

	if o == nil {
		value.Null = true
		return value
	}

	value.Value = (*o).AsDuration()

	return value
}
