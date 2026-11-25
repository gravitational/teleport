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

package resources

import (
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"

	crownjewelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/crownjewel/v1"
	"github.com/gravitational/teleport/api/types"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/teleport"
	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
	schemav1 "github.com/gravitational/teleport/integrations/terraform/tfschema/crownjewel/v1"
)

// NewCrownJewelDataSourceType returns the crown jewel data source type.
func NewCrownJewelDataSourceType() tfdriver.DataSourceType[crownjewelv1.CrownJewel, tfdriver.NameIdentifier] {
	return tfdriver.DataSourceType[crownjewelv1.CrownJewel, tfdriver.NameIdentifier]{
		NewDataSourceClient: func(p provider.Provider) tfdriver.DataSourceClient[crownjewelv1.CrownJewel, tfdriver.NameIdentifier] {
			return teleport.NewCrownJewelClient(clientFromProvider(p))
		},
		Kind: types.KindCrownJewel,
		Name: types.KindCrownJewel,
		Codec: tfdriver.DataSourceCodecFuncs[crownjewelv1.CrownJewel]{
			SchemaFunc:  schemav1.GenSchemaCrownJewelDataSource,
			ToStateFunc: schemav1.CopyCrownJewelToTerraform,
		},
		Identifier: tfdriver.NameIdentifierFromPath(path.Root("metadata").AtName("name")),
	}
}

// NewCrownJewelResourceType returns the crown jewel resource type.
func NewCrownJewelResourceType() tfdriver.ResourceType[crownjewelv1.CrownJewel, tfdriver.NameIdentifier] {
	return tfdriver.ResourceType[crownjewelv1.CrownJewel, tfdriver.NameIdentifier]{
		NewResourceClient: func(p provider.Provider) tfdriver.ResourceClient[crownjewelv1.CrownJewel, tfdriver.NameIdentifier] {
			return teleport.NewCrownJewelClient(clientFromProvider(p))
		},
		Kind: types.KindCrownJewel,
		Name: types.KindCrownJewel,
		Codec: tfdriver.ResourceCodecFuncs[crownjewelv1.CrownJewel]{
			SchemaFunc:   schemav1.GenSchemaCrownJewelResource,
			ToStateFunc:  schemav1.CopyCrownJewelToTerraform,
			FromPlanFunc: schemav1.CopyCrownJewelFromTerraform,
			ModifyPlanFunc: tfdriver.NullToZeroModifyPlan(
				schemav1.CopyCrownJewelFromTerraform,
				schemav1.CopyCrownJewelToTerraformPreserveUnknown,
			),
		},
		Normalizer: tfdriver.ForceKind[crownjewelv1.CrownJewel](types.KindCrownJewel),
		Identifier: tfdriver.NameIdentifierPolicy(path.Root("metadata").AtName("name"), func(crownJewel *crownjewelv1.CrownJewel) string {
			return crownJewel.GetMetadata().GetName()
		}),
		ResourceRevision: func(st *crownjewelv1.CrownJewel) string {
			return st.GetMetadata().GetRevision()
		},
	}
}
