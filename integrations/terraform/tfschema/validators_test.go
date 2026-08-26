/*
   Copyright 2022 Gravitational, Inc.

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
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"
)

func getObjectWithTwoKeys() types.Object {
	return types.Object{
		Attrs: map[string]attr.Value{
			"key1": types.String{
				Value:   "val1",
				Null:    false,
				Unknown: false,
			},
			"key2": types.Int64{
				Value:   12,
				Null:    false,
				Unknown: false,
			},
		},
		AttrTypes: map[string]attr.Type{
			"key1": types.StringType,
			"key2": types.StringType,
		},
	}
}

func TestAnyOfValidator(t *testing.T) {
	type testCase struct {
		input       types.Object
		anyOfValues []string
		expectError bool
	}
	tests := map[string]testCase{
		"one-of-two": {
			input:       getObjectWithTwoKeys(),
			anyOfValues: []string{"key1"},
			expectError: false,
		},
		"two-of-two": {
			input:       getObjectWithTwoKeys(),
			anyOfValues: []string{"key1", "key2"},
			expectError: false,
		},
		"none-of-two": {
			input:       getObjectWithTwoKeys(),
			anyOfValues: []string{"key3"},
			expectError: true,
		},
	}
	for name, test := range tests {
		name, test := name, test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			resp := &tfsdk.ValidateAttributeResponse{
				Diagnostics: make(diag.Diagnostics, 0),
			}

			req := tfsdk.ValidateAttributeRequest{
				AttributeConfig: test.input,
			}
			UseAnyOfValidator(test.anyOfValues...).Validate(ctx, req, resp)
			require.Equal(t, test.expectError, resp.Diagnostics.HasError())
		})
	}
}
