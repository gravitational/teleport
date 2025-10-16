/*
Copyright 2025 Gravitational, Inc.

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

package v1

import (
	context "context"
	fmt "fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"

	"github.com/gravitational/teleport/integrations/terraform/tfschema/resource153"
)

var (
	GenSchemaDuration = resource153.GenSchemaDuration
	CopyToDuration    = resource153.CopyToDuration
	CopyFromDuration  = resource153.CopyFromDuration

	GenSchemaTimestamp = resource153.GenSchemaTimestamp
	CopyToTimestamp    = resource153.CopyToTimestamp
	CopyFromTimestamp  = resource153.CopyFromTimestamp
)

func GenSchemaTraitsMap(_ context.Context, attr tfsdk.Attribute) tfsdk.Attribute {
	return tfsdk.Attribute{
		Optional: attr.Optional,
		Type: types.MapType{
			ElemType: types.ListType{
				ElemType: types.StringType,
			},
		},
		Description: attr.Description,
	}
}

func CopyFromTraitsMap(diags diag.Diagnostics, v attr.Value, o *[]*machineidv1.Trait) {
	value, ok := v.(types.Map)
	if !ok {
		diags.AddError("Error reading from Terraform object", fmt.Sprintf("Can not convert %T to Map", v))
		return
	}

	traits := make([]*machineidv1.Trait, 0, len(value.Elems))
	for name, elem := range value.Elems {
		list, ok := elem.(types.List)
		if !ok {
			diags.AddError("Error reading from Terraform object", fmt.Sprintf("Can not convert %T to List", elem))
			return
		}

		trait := &machineidv1.Trait{
			Name:   name,
			Values: make([]string, len(list.Elems)),
		}

		for idx, elem := range list.Elems {
			str, ok := elem.(types.String)
			if !ok {
				diags.AddError("Error reading from Terraform object", fmt.Sprintf("Can not convert %T to String", elem))
				return
			}
			trait.Values[idx] = str.Value
		}

		traits = append(traits, trait)
	}

	*o = traits
}

func CopyToTraitsMap(_ diag.Diagnostics, traits []*machineidv1.Trait, _ attr.Type, _ attr.Value) attr.Value {
	mapValue := &types.Map{
		Elems: make(map[string]attr.Value, len(traits)),
		ElemType: types.ListType{
			ElemType: types.StringType,
		},
	}

	if traits == nil {
		mapValue.Null = true
		return mapValue
	}

	for _, trait := range traits {
		listValue := types.List{
			Elems:    make([]attr.Value, len(trait.Values)),
			ElemType: types.StringType,
		}
		for idx, value := range trait.GetValues() {
			listValue.Elems[idx] = types.String{Value: value}
		}
		mapValue.Elems[trait.Name] = listValue
	}

	return mapValue
}
