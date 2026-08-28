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
		resp.Diagnostics.AddError("Version validation error", fmt.Sprintf("Attribute %v (%v) is not a valid version (vXX)", req.AttributePath.String(), value.Value))
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

// SAMLIdPServiceProviderAttributeNameFormatValidator validates that an attribute name_format is a URN.
//
// The Teleport API accepts short aliases ("basic", "uri", "unspecified") but normalizes
// them to full URNs on write. If the provider passes a short alias, the returned value
// will differ from the planned value, causing Terraform to report an inconsistent result
// and taint the resource.
//
// We validate only that the value contains ":" rather than enumerating known URNs because
// name_format is defined as anyURI in the SAML 2.0 spec (§2.7.3.1), and any valid URN
// contains ":". This rejects all short aliases without hardcoding a list that could become
// stale if Teleport adds support for new formats.
//
// TODO: Figure out how to make the generator support plan modifiers on fields nested inside
// ListNestedAttributes, then replace this validator with an attribute plan modifier on
// SAMLAttributeMapping.NameFormat that prefixes short names with
// "urn:oasis:names:tc:SAML:2.0:attrname-format:" and mark the field Computed.
type SAMLIdPServiceProviderAttributeNameFormatValidator struct{}

// SAMLIdPAttributeNameFormatValidator returns SAMLIdPServiceProviderAttributeNameFormatValidator.
func SAMLIdPAttributeNameFormatValidator() tfsdk.AttributeValidator {
	return SAMLIdPServiceProviderAttributeNameFormatValidator{}
}

func (v SAMLIdPServiceProviderAttributeNameFormatValidator) Description(_ context.Context) string {
	return "name_format must be a URN (e.g. urn:oasis:names:tc:SAML:2.0:attrname-format:basic)"
}

func (v SAMLIdPServiceProviderAttributeNameFormatValidator) MarkdownDescription(_ context.Context) string {
	return "`name_format` must be a URN (e.g. `urn:oasis:names:tc:SAML:2.0:attrname-format:basic`)"
}

func (v SAMLIdPServiceProviderAttributeNameFormatValidator) Validate(_ context.Context, req tfsdk.ValidateAttributeRequest, resp *tfsdk.ValidateAttributeResponse) {
	if req.AttributeConfig == nil {
		return
	}
	value, ok := req.AttributeConfig.(types.String)
	if !ok {
		resp.Diagnostics.AddError("name_format validation error", fmt.Sprintf("Attribute %v can not be converted to String", req.AttributePath.String()))
		return
	}
	if value.Null || value.Unknown || value.Value == "" {
		return
	}
	if !strings.Contains(value.Value, ":") {
		resp.Diagnostics.AddError(
			"name_format validation error",
			fmt.Sprintf("Attribute %v: %q is not a valid URN; use the full URN form (e.g. urn:oasis:names:tc:SAML:2.0:attrname-format:basic)", req.AttributePath.String(), value.Value),
		)
	}
}

// SAMLIdPServiceProviderSpecValidator validates the spec field combination rules
//
// The only error omitted is if neither entity_descriptor nor entity_id+acs_url
// are provided. It warns on other combinations that can cause idempotency or
// other issues.
type SAMLIdPServiceProviderSpecValidator struct{}

// SAMLIdPSpecValidator returns SAMLIdPServiceProviderSpecValidator.
func SAMLIdPSpecValidator() tfsdk.AttributeValidator {
	return SAMLIdPServiceProviderSpecValidator{}
}

const SamlIdPServiceProviderDescription = "When setting `attribute_mapping`, prefer `entity_id` and `acs_url` to `entity_descriptor`. " +
	"The API will update `entity_descriptor` with `attribute_mapping`, causing Terraform to taint the " +
	"resource if the a matching attribute_mapping isn't in the user-provided entity_descriptor."

func (v SAMLIdPServiceProviderSpecValidator) Description(_ context.Context) string {
	return SamlIdPServiceProviderDescription
}

func (v SAMLIdPServiceProviderSpecValidator) MarkdownDescription(_ context.Context) string {
	return SamlIdPServiceProviderDescription
}

func (v SAMLIdPServiceProviderSpecValidator) Validate(_ context.Context, req tfsdk.ValidateAttributeRequest, resp *tfsdk.ValidateAttributeResponse) {
	if req.AttributeConfig == nil {
		return
	}
	spec, ok := req.AttributeConfig.(types.Object)
	if !ok {
		resp.Diagnostics.AddError("Spec validation error", fmt.Sprintf("Attribute %v can not be converted to Object", req.AttributePath.String()))
		return
	}
	if spec.Null || spec.Unknown {
		return
	}

	entityDescriptor, _ := spec.Attrs["entity_descriptor"].(types.String)
	entityID, _ := spec.Attrs["entity_id"].(types.String)
	acsURL, _ := spec.Attrs["acs_url"].(types.String)
	attrMapping, _ := spec.Attrs["attribute_mapping"].(types.List)

	if !entityDescriptor.IsNull() && !attrMapping.IsNull() {
		resp.Diagnostics.AddWarning("entity_descriptor and attribute_mapping cause idempotency issues",
			"You've provided both `entity_descriptor` and `attribute_mapping`. The Teleport API accepts "+
				"this configuration, but it will rewrite the `entity_descriptor` to include `attribute_mapping`. "+
				"This will cause Terraform to detect changes on the next apply. Prefer `entity_id` and `acs_url` "+
				"with `attribute_mapping`.")
	}

	if entityDescriptor.IsNull() && (entityID.IsNull() || acsURL.IsNull()) {
		resp.Diagnostics.AddError(
			"Entity not provided",
			"Either entity_descriptor or entity_id and acs_url must be provided",
		)
	}
	if !entityDescriptor.IsNull() && (!entityID.IsNull() || !acsURL.IsNull()) {
		resp.Diagnostics.AddWarning(
			"User must match multiple entity_id/acs_url values",
			"You've provided both `entity_id` or `acs_url` and `entity_descriptor` (which contains entity_id and acs_url). "+
				"Either provide only one or make sure the two values match. Apply will fail if the entity_id values differ.",
		)
	}

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
