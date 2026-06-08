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

package provider

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/hashicorp/terraform-plugin-framework/attr"
)

// deepCopyAttrValue returns a deep copy of the value.
func deepCopyAttrValue(ctx context.Context, value attr.Value) (attr.Value, error) {
	tv, err := value.ToTerraformValue(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	copied, err := value.Type(ctx).ValueFromTerraform(ctx, tv)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return copied, nil
}
