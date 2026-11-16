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

package resource153

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/gravitational/trace"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/types/known/structpb"
)

// StructFromJSON converts a Terraform string holding a JSON object into a
// [structpb.Struct]. Null and unknown strings convert to nil.
func StructFromJSON(v types.String) (*structpb.Struct, error) {
	if v.IsNull() || v.IsUnknown() {
		return nil, nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(v.ValueString()), &m); err != nil {
		return nil, trace.Wrap(err)
	}
	if m == nil {
		return nil, trace.BadParameter("must be a JSON object, got null")
	}
	s, err := structpb.NewStruct(m)
	return s, trace.Wrap(err)
}

// JSONFromStruct converts a [structpb.Struct] into a Terraform JSON string.
func JSONFromStruct(s *structpb.Struct, prior attr.Value) (types.String, error) {
	if s == nil {
		return types.StringNull(), nil
	}
	// encoding/json sorts map keys, unlike protojson.
	b, err := json.Marshal(s.AsMap())
	if err != nil {
		return types.String{}, trace.Wrap(err)
	}
	if p, ok := prior.(types.String); ok && !p.IsNull() && !p.IsUnknown() && jsonEqual(p.ValueString(), string(b)) {
		return p, nil
	}
	return types.StringValue(string(b)), nil
}

func jsonEqual(a, b string) bool {
	var av, bv any
	if json.Unmarshal([]byte(a), &av) != nil || json.Unmarshal([]byte(b), &bv) != nil {
		return false
	}
	return reflect.DeepEqual(av, bv)
}

// JSONObjectValidator rejects strings that aren't a JSON object.
func JSONObjectValidator() validator.String {
	return jsonObjectValidator{}
}

type jsonObjectValidator struct{}

func (jsonObjectValidator) Description(context.Context) string {
	return "value must be a JSON object"
}

func (v jsonObjectValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (jsonObjectValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	var v any
	if err := json.Unmarshal([]byte(req.ConfigValue.ValueString()), &v); err != nil {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid JSON", fmt.Sprintf("%s: %v.", req.Path, err))
		return
	}
	if _, ok := v.(map[string]any); !ok {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid JSON object", fmt.Sprintf("%s: must be a JSON object, e.g. jsonencode({ claim = \"value\" }).", req.Path))
	}
}

// JSONEqualPlanModifier keeps the prior state when the configured JSON is
// equivalent.
func JSONEqualPlanModifier() planmodifier.String {
	return jsonEqualPlanModifier{}
}

type jsonEqualPlanModifier struct{}

func (jsonEqualPlanModifier) Description(context.Context) string {
	return "suppresses diffs between equivalent JSON values"
}

func (m jsonEqualPlanModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (jsonEqualPlanModifier) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.StateValue.IsNull() || req.StateValue.IsUnknown() || req.PlanValue.IsNull() || req.PlanValue.IsUnknown() {
		return
	}
	if jsonEqual(req.StateValue.ValueString(), req.PlanValue.ValueString()) {
		resp.PlanValue = req.StateValue
	}
}
