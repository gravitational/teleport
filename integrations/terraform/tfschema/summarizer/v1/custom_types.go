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
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/integrations/terraform/tfschema/resource153"
)

var (
	GenSchemaTimestamp = resource153.GenSchemaTimestamp
	CopyToTimestamp    = resource153.CopyToTimestamp
	CopyFromTimestamp  = resource153.CopyFromTimestamp
)

// GenSchemaClassifierActionMode returns the Terraform schema for a
// ClassifierActionMode tri-state enum. The enum is exposed as an optional
// boolean, relying on the natural tri-state of an optional attribute:
//   - null/unset -> CLASSIFIER_ACTION_MODE_UNSPECIFIED (the default behavior)
//   - true       -> CLASSIFIER_ACTION_MODE_ENABLED
//   - false      -> CLASSIFIER_ACTION_MODE_DISABLED
func GenSchemaClassifierActionMode(_ context.Context, attr tfsdk.Attribute) tfsdk.Attribute {
	return tfsdk.Attribute{
		Optional:      true,
		Type:          types.BoolType,
		Description:   attr.Description,
		Computed:      attr.Computed,
		PlanModifiers: attr.PlanModifiers,
	}
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
	case value.Null || value.Unknown:
		*o = summarizerv1.ClassifierActionMode_CLASSIFIER_ACTION_MODE_UNSPECIFIED
	case value.Value:
		*o = summarizerv1.ClassifierActionMode_CLASSIFIER_ACTION_MODE_ENABLED
	default:
		*o = summarizerv1.ClassifierActionMode_CLASSIFIER_ACTION_MODE_DISABLED
	}
}

// CopyToClassifierActionMode converts a ClassifierActionMode enum value into an
// optional Terraform boolean.
func CopyToClassifierActionMode(diags diag.Diagnostics, o summarizerv1.ClassifierActionMode, t attr.Type, v attr.Value) attr.Value {
	value, ok := v.(types.Bool)
	if !ok {
		value = types.Bool{}
	}

	switch o {
	case summarizerv1.ClassifierActionMode_CLASSIFIER_ACTION_MODE_ENABLED:
		value.Null = false
		value.Unknown = false
		value.Value = true
	case summarizerv1.ClassifierActionMode_CLASSIFIER_ACTION_MODE_DISABLED:
		value.Null = false
		value.Unknown = false
		value.Value = false
	default: // CLASSIFIER_ACTION_MODE_UNSPECIFIED
		value.Null = true
		value.Unknown = false
	}

	return value
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

// GenSchemaRiskLevel exposes a RiskLevel enum as an optional "low".."critical" string.
func GenSchemaRiskLevel(_ context.Context, attr tfsdk.Attribute) tfsdk.Attribute {
	return tfsdk.Attribute{
		Optional:      true,
		Type:          types.StringType,
		Description:   attr.Description,
		Computed:      attr.Computed,
		PlanModifiers: attr.PlanModifiers,
		Validators:    append(attr.Validators, riskLevelValidator{}),
	}
}

func CopyFromRiskLevel(diags diag.Diagnostics, v attr.Value, o *summarizerv1.RiskLevel) {
	value, ok := v.(types.String)
	if !ok {
		diags.AddError("Error reading from Terraform object", fmt.Sprintf("Can not convert %T to types.String", v))
		return
	}

	if value.Null || value.Unknown {
		*o = summarizerv1.RiskLevel_RISK_LEVEL_UNSPECIFIED
		return
	}

	level, ok := riskLevelFromString[strings.ToLower(value.Value)]
	if !ok {
		diags.AddError("Invalid risk_level_floor", fmt.Sprintf("%q must be one of low, medium, high, critical", value.Value))
		return
	}
	*o = level
}

func CopyToRiskLevel(diags diag.Diagnostics, o summarizerv1.RiskLevel, t attr.Type, v attr.Value) attr.Value {
	value, ok := v.(types.String)
	if !ok {
		value = types.String{}
	}

	s, ok := riskLevelToString[o]
	if !ok { // RISK_LEVEL_UNSPECIFIED
		value.Null = true
		value.Unknown = false
		return value
	}

	value.Null = false
	value.Unknown = false
	value.Value = s

	return value
}

const riskLevelAllowed = `must be one of "low", "medium", "high", "critical"`

// riskLevelValidator rejects risk_level_floor values outside the allowed set at plan time.
type riskLevelValidator struct{}

func (riskLevelValidator) Description(context.Context) string         { return riskLevelAllowed }
func (riskLevelValidator) MarkdownDescription(context.Context) string { return riskLevelAllowed }

func (riskLevelValidator) Validate(_ context.Context, req tfsdk.ValidateAttributeRequest, resp *tfsdk.ValidateAttributeResponse) {
	if req.AttributeConfig == nil {
		return
	}
	value, ok := req.AttributeConfig.(types.String)
	if !ok || value.Null || value.Unknown {
		return
	}
	if _, ok := riskLevelFromString[strings.ToLower(value.Value)]; !ok {
		resp.Diagnostics.AddError("Invalid risk_level_floor", fmt.Sprintf("%q "+riskLevelAllowed, value.Value))
	}
}
