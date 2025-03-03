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
	"slices"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TimeValueInFutureValidator ensures that a time is in the future
type TimeValueInFutureValidator struct{}

// MustTimeBeInFuture returns TimeValueInFutureValidator
func MustTimeBeInFuture() tfsdk.AttributeValidator {
	return TimeValueInFutureValidator{}
}

// Description returns validator description
func (v TimeValueInFutureValidator) Description(_ context.Context) string {
	return "Checks that a time value is in future"
}

// MarkdownDescription returns validator markdown description
func (v TimeValueInFutureValidator) MarkdownDescription(_ context.Context) string {
	return "Checks that a time value is in future"
}

// Validate performs the validation.
func (v TimeValueInFutureValidator) Validate(_ context.Context, req tfsdk.ValidateAttributeRequest, resp *tfsdk.ValidateAttributeResponse) {
	if req.AttributeConfig == nil {
		return
	}

	value, ok := req.AttributeConfig.(TimeValue)
	if !ok {
		resp.Diagnostics.AddError("Time validation error", fmt.Sprintf("Attribute %v can not be converted to TimeValue", req.AttributePath.String()))
		return
	}

	if value.Null || value.Unknown {
		return
	}

	if time.Now().After(value.Value) {
		resp.Diagnostics.AddError("Time validation error", fmt.Sprintf("Attribute %v value must be in the future", req.AttributePath.String()))
	}
}

// VersionValidator validates that a resource version is in the specified range
type VersionValidator struct {
	Min int
	Max int
}

// UseVersionBetween creates VersionValidator
func UseVersionBetween(min, max int) tfsdk.AttributeValidator {
	return VersionValidator{min, max}
}

// Description returns validator description
func (v VersionValidator) Description(_ context.Context) string {
	return fmt.Sprintf("Checks that version string is between %v..%v", v.Min, v.Max)
}

// MarkdownDescription returns validator markdown description
func (v VersionValidator) MarkdownDescription(_ context.Context) string {
	return fmt.Sprintf("Checks that version string is between %v..%v", v.Min, v.Max)
}

// Validate performs the validation.
func (v VersionValidator) Validate(_ context.Context, req tfsdk.ValidateAttributeRequest, resp *tfsdk.ValidateAttributeResponse) {
	if req.AttributeConfig == nil {
		return
	}

	value, ok := req.AttributeConfig.(types.String)
	if !ok {
		resp.Diagnostics.AddError("Version validation error", fmt.Sprintf("Attribute %v can not be converted to StringValue", req.AttributePath.String()))
		return
	}

	if value.Null || value.Unknown {
		return
	}

	var version int
	fmt.Sscan(value.Value[1:], &version) // strip leading v<xx>

	if version == 0 {
		resp.Diagnostics.AddError("Version validation error", fmt.Sprintf("Attribute %v (%v) is not a vaild version (vXX)", req.AttributePath.String(), value.Value))
		return
	}

	if version < v.Min || version > v.Max {
		resp.Diagnostics.AddError("Version validation error", fmt.Sprintf("Version v%v (%v) is not in range v%v..v%v", version, req.AttributePath.String(), v.Min, v.Max))
		return
	}
}

// MapKeysPresentValidator validates that a map has the specified keys
type MapKeysPresentValidator struct {
	Keys []string
}

// UseKeysPresentValidator creates MapKeysPresentValidator
func UseMapKeysPresentValidator(keys ...string) tfsdk.AttributeValidator {
	return MapKeysPresentValidator{Keys: keys}
}

// Description returns validator description
func (v MapKeysPresentValidator) Description(_ context.Context) string {
	return fmt.Sprintf("Checks that a map has %v keys set", v.Keys)
}

// MarkdownDescription returns validator markdown description
func (v MapKeysPresentValidator) MarkdownDescription(_ context.Context) string {
	return fmt.Sprintf("Checks that a map has %v keys set", v.Keys)
}

// Validate performs the validation.
func (v MapKeysPresentValidator) Validate(_ context.Context, req tfsdk.ValidateAttributeRequest, resp *tfsdk.ValidateAttributeResponse) {
	if req.AttributeConfig == nil {
		return
	}

	value, ok := req.AttributeConfig.(types.Map)
	if !ok {
		resp.Diagnostics.AddError("Map keys validation error", fmt.Sprintf("Attribute %v can not be converted to Map", req.AttributePath.String()))
		return
	}

	if value.Null || value.Unknown {
		return
	}

OUTER:
	for _, k := range v.Keys {
		for e := range value.Elems {
			if e == k {
				break OUTER
			}
		}

		resp.Diagnostics.AddError("Map keys validation error", fmt.Sprintf("Key %v must be present in the map %v", k, req.AttributePath.String()))
	}

}

// AnyOfValidator validates that at least one of the attributes is set (known and not null)
type AnyOfValidator struct {
	Keys []string
}

// UseAnyOfValidator creates AnyOfValidator
func UseAnyOfValidator(keys ...string) tfsdk.AttributeValidator {
	return AnyOfValidator{
		Keys: keys,
	}
}

// Description returns validator description
func (v AnyOfValidator) Description(_ context.Context) string {
	return fmt.Sprintf("AnyOf '%s' attributes must be present", strings.Join(v.Keys, ", "))
}

// MarkdownDescription returns validator markdown description
func (v AnyOfValidator) MarkdownDescription(_ context.Context) string {
	return fmt.Sprintf("AnyOf `%s` attributes must be present", strings.Join(v.Keys, ", "))
}

// Validate performs the validation.
func (v AnyOfValidator) Validate(ctx context.Context, req tfsdk.ValidateAttributeRequest, resp *tfsdk.ValidateAttributeResponse) {
	if req.AttributeConfig == nil {
		return
	}

	value, ok := req.AttributeConfig.(types.Object)
	if !ok {
		resp.Diagnostics.AddError(
			"AnyOf Object validation error",
			fmt.Sprintf("Attribute %v can not be converted to Object", req.AttributePath.String()),
		)
		return
	}

	if value.Null || value.Unknown {
		return
	}

	for _, key := range v.Keys {
		attr, found := value.Attrs[key]
		if found {
			tfVal, err := attr.ToTerraformValue(ctx)
			if err != nil {
				resp.Diagnostics.AddError(
					"AnyOf keys validation error",
					fmt.Sprintf("Failed to convert ToTerraformValue attribute with key '%s': %v", key, err),
				)
				return
			}
			if !tfVal.IsNull() {
				return
			}
		}
	}

	resp.Diagnostics.AddError(
		"AnyOf keys validation error",
		fmt.Sprintf("AnyOf '%s' keys must be present", strings.Join(v.Keys, ", ")),
	)
}

// UseValueIn creates a StringValueValidator
func UseValueIn(allowed ...string) tfsdk.AttributeValidator {
	return StringValueValidator{allowed}
}

// StringValueValidator validates that a resource string field is in a set of allowed values.
type StringValueValidator struct {
	AllowedValues []string
}

// Description returns validator description
func (v StringValueValidator) Description(_ context.Context) string {
	return fmt.Sprintf("Checks that string field is one of %v", v.AllowedValues)
}

// MarkdownDescription returns validator markdown description
func (v StringValueValidator) MarkdownDescription(_ context.Context) string {
	return fmt.Sprintf("Checks that string field is one of %v", v.AllowedValues)
}

// Validate performs the validation.
func (v StringValueValidator) Validate(_ context.Context, req tfsdk.ValidateAttributeRequest, resp *tfsdk.ValidateAttributeResponse) {
	if req.AttributeConfig == nil {
		return
	}

	value, ok := req.AttributeConfig.(types.String)
	if !ok {
		resp.Diagnostics.AddError("Field validation error", fmt.Sprintf("Attribute %v can not be converted to StringValue", req.AttributePath.String()))
		return
	}

	if value.Null || value.Unknown {
		if !slices.Contains(v.AllowedValues, "") {
			resp.Diagnostics.AddError("Field validation error", fmt.Sprintf("Attribute %v (%v) is unset but empty string is not a valid value.", req.AttributePath.String(), value.Value))
		}
		return
	}

	if !slices.Contains(v.AllowedValues, value.Value) {
		resp.Diagnostics.AddError("Field validation error", fmt.Sprintf("Attribute %v (%v) is not in the allowed set (%v).", req.AttributePath.String(), value.Value, v.AllowedValues))
	}
}
