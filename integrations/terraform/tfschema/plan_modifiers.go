/*
Copyright 2026 Gravitational, Inc.

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

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// DefaultStringModifier implements the planmodifier.String interface. It sets
// the default string value when the config value is omitted.
type DefaultStringModifier struct {
	value string
}

// DefaultString returns the default string modifier with the provided default value.
func DefaultString(value string) planmodifier.String {
	return DefaultStringModifier{
		value: value,
	}
}

// Description of the plan modifier.
func (d DefaultStringModifier) Description(_ context.Context) string {
	return "This modifier sets the default value when the config value is omitted."
}

// MarkdownDescription of the plan modier.
func (d DefaultStringModifier) MarkdownDescription(_ context.Context) string {
	return "This modifier sets the default value when the config value is omitted."
}

// PlanModifyString sets the configured default value when the config value is
// omitted.
func (d DefaultStringModifier) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	// Destroy plans must remain entirely null. Do not set a default value
	// the resource is being destroyed.
	if req.Plan.Raw.IsNull() {
		return
	}

	// If the configuration explicitly provided a value (even if unknown), do not overwrite it.
	if !req.ConfigValue.IsNull() {
		return
	}

	resp.PlanValue = types.StringValue(d.value)
}
