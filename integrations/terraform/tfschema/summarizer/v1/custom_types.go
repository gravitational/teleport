// Copyright 2026 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	datasource_schema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	resource_schema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"

	"github.com/gravitational/teleport/integrations/terraform/tfschema/resource153"
)

var (
	GenSchemaTimestampDataSource = resource153.GenSchemaTimestampDataSource
	GenSchemaTimestampResource   = resource153.GenSchemaTimestampResource
	CopyToTimestamp              = resource153.CopyToTimestamp
	CopyFromTimestamp            = resource153.CopyFromTimestamp
)

// GenSchemaClassifierActionModeDataSource returns the Terraform schema for a
// ClassifierActionMode tri-state enum. The enum is exposed as an optional
// boolean, relying on the natural tri-state of an optional attribute:
//   - null/unset -> CLASSIFIER_ACTION_MODE_UNSPECIFIED (the default behavior)
//   - true       -> CLASSIFIER_ACTION_MODE_ENABLED
//   - false      -> CLASSIFIER_ACTION_MODE_DISABLED
func GenSchemaClassifierActionModeDataSource(_ context.Context, attr datasource_schema.BoolAttribute) datasource_schema.BoolAttribute {
	attr.Optional = true
	return attr
}

// GenSchemaClassifierActionModeResource returns the Terraform schema for a
// ClassifierActionMode tri-state enum. The enum is exposed as an optional
// boolean, relying on the natural tri-state of an optional attribute:
//   - null/unset -> CLASSIFIER_ACTION_MODE_UNSPECIFIED (the default behavior)
//   - true       -> CLASSIFIER_ACTION_MODE_ENABLED
//   - false      -> CLASSIFIER_ACTION_MODE_DISABLED
func GenSchemaClassifierActionModeResource(ctx context.Context, attr resource_schema.BoolAttribute) resource_schema.BoolAttribute {
	attr.Optional = true
	return attr
}

// CopyFromClassifierActionMode converts an optional Terraform boolean into a
// ClassifierActionMode enum value.
func CopyFromClassifierActionMode(diags diag.Diagnostics, v attr.Value, o *summarizerv1.ClassifierActionMode) {
	value, ok := v.(types.Bool)
	if !ok {
		diags.AddError("Error reading from Terraform object", fmt.Sprintf("Can not convert %T to types.Bool", v))
		return
	}

	switch {
	case value.IsNull() || value.IsUnknown():
		*o = summarizerv1.ClassifierActionMode_CLASSIFIER_ACTION_MODE_UNSPECIFIED
	case value.ValueBool():
		*o = summarizerv1.ClassifierActionMode_CLASSIFIER_ACTION_MODE_ENABLED
	default:
		*o = summarizerv1.ClassifierActionMode_CLASSIFIER_ACTION_MODE_DISABLED
	}
}

// CopyToClassifierActionMode converts a ClassifierActionMode enum value into an
// optional Terraform boolean. Set `preserveUnknown` to preserve unknown values.
func CopyToClassifierActionMode(_ diag.Diagnostics, o summarizerv1.ClassifierActionMode, _ attr.Type, v attr.Value, preserveUnknown bool) attr.Value {
	if preserveUnknown && v != nil && v.IsUnknown() {
		return types.BoolUnknown()
	}

	switch o {
	case summarizerv1.ClassifierActionMode_CLASSIFIER_ACTION_MODE_ENABLED:
		return types.BoolValue(true)
	case summarizerv1.ClassifierActionMode_CLASSIFIER_ACTION_MODE_DISABLED:
		return types.BoolValue(false)
	default: // CLASSIFIER_ACTION_MODE_UNSPECIFIED
		return types.BoolNull()
	}
}

var riskLevelToString = map[summarizerv1.RiskLevel]string{
	summarizerv1.RiskLevel_RISK_LEVEL_LOW:      "low",
	summarizerv1.RiskLevel_RISK_LEVEL_MEDIUM:   "medium",
	summarizerv1.RiskLevel_RISK_LEVEL_HIGH:     "high",
	summarizerv1.RiskLevel_RISK_LEVEL_CRITICAL: "critical",
}

var riskLevelFromString = map[string]summarizerv1.RiskLevel{
	"low":      summarizerv1.RiskLevel_RISK_LEVEL_LOW,
	"medium":   summarizerv1.RiskLevel_RISK_LEVEL_MEDIUM,
	"high":     summarizerv1.RiskLevel_RISK_LEVEL_HIGH,
	"critical": summarizerv1.RiskLevel_RISK_LEVEL_CRITICAL,
}

// GenSchemaRiskLevelDataSource exposes a RiskLevel enum as an optional "low".."critical" string.
func GenSchemaRiskLevelDataSource(_ context.Context, attr datasource_schema.StringAttribute) datasource_schema.StringAttribute {
	attr.Optional = true
	attr.Validators = append(attr.Validators, stringEnumValidator{attribute: "risk_level_floor", allowed: riskLevels})
	return attr
}

// GenSchemaRiskLevelResource exposes a RiskLevel enum as an optional "low".."critical" string.
func GenSchemaRiskLevelResource(ctx context.Context, attr resource_schema.StringAttribute) resource_schema.StringAttribute {
	attr.Optional = true
	attr.Validators = append(attr.Validators, stringEnumValidator{attribute: "risk_level_floor", allowed: riskLevels})
	return attr
}

func CopyFromRiskLevel(diags diag.Diagnostics, v attr.Value, o *summarizerv1.RiskLevel) {
	value, ok := v.(types.String)
	if !ok {
		diags.AddError("Error reading from Terraform object", fmt.Sprintf("Can not convert %T to types.String", v))
		return
	}

	if value.IsNull() || value.IsUnknown() {
		*o = summarizerv1.RiskLevel_RISK_LEVEL_UNSPECIFIED
		return
	}

	level, ok := riskLevelFromString[strings.ToLower(value.ValueString())]
	if !ok {
		diags.AddError("Invalid risk_level_floor", fmt.Sprintf("%q must be one of low, medium, high, critical", value.ValueString()))
		return
	}
	*o = level
}

// CopyToRiskLevel converts a [summarizerv1.RiskLevel] into a Terraform
// [types.String] value. Set `preserveUnknown` to preserve unknown values.
func CopyToRiskLevel(_ diag.Diagnostics, o summarizerv1.RiskLevel, _ attr.Type, v attr.Value, preserveUnknown bool) attr.Value {
	if preserveUnknown && v != nil && v.IsUnknown() {
		return types.StringUnknown()
	}

	s, ok := riskLevelToString[o]
	if !ok { // RISK_LEVEL_UNSPECIFIED
		return types.StringNull()
	}

	return types.StringValue(s)
}

var _ validator.String = stringEnumValidator{}

// riskLevels lists the accepted risk_level_floor values in documentation order.
var riskLevels = []string{"low", "medium", "high", "critical"}

// stringEnumValidator rejects, at plan time, string values outside a fixed set.
type stringEnumValidator struct {
	// attribute names the attribute in error summaries.
	attribute string
	// allowed lists the accepted values, compared case-insensitively.
	allowed []string
}

func (v stringEnumValidator) Description(context.Context) string         { return v.message() }
func (v stringEnumValidator) MarkdownDescription(context.Context) string { return v.message() }

func (v stringEnumValidator) message() string {
	quoted := make([]string, len(v.allowed))
	for i, a := range v.allowed {
		quoted[i] = strconv.Quote(a)
	}
	return "must be one of " + strings.Join(quoted, ", ")
}

func (v stringEnumValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	value := req.ConfigValue
	if value.IsNull() || value.IsUnknown() {
		return
	}
	if !slices.Contains(v.allowed, strings.ToLower(value.ValueString())) {
		resp.Diagnostics.AddError("Invalid "+v.attribute, fmt.Sprintf("%q %s", value.ValueString(), v.message()))
	}
}
