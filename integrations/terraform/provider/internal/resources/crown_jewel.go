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

package resources

import (
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"

	crownjewelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/crownjewel/v1"
	"github.com/gravitational/teleport/api/types"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/teleport"
	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
	schemav1 "github.com/gravitational/teleport/integrations/terraform/tfschema/crownjewel/v1"
)

// NewCrownJewelDataSourceType returns the crown jewel data source type.
func NewCrownJewelDataSourceType() tfdriver.DataSourceType[crownjewelv1.CrownJewel, tfdriver.NameIdentifier] {
	return tfdriver.DataSourceType[crownjewelv1.CrownJewel, tfdriver.NameIdentifier]{
		NewDataSourceClient: func(p tfsdk.Provider) tfdriver.DataSourceClient[crownjewelv1.CrownJewel, tfdriver.NameIdentifier] {
			return teleport.NewCrownJewelClient(clientFromProvider(p))
		},
		Kind: types.KindCrownJewel,
		Codec: tfdriver.DataSourceCodecFuncs[crownjewelv1.CrownJewel]{
			SchemaFunc:  schemav1.GenSchemaCrownJewel,
			ToStateFunc: schemav1.CopyCrownJewelToTerraform,
		},
		Identifier: tfdriver.NameIdentifierFromPath(path.Root("metadata").AtName("name")),
	}
}

// NewCrownJewelResourceType returns the crown jewel resource type.
func NewCrownJewelResourceType() tfdriver.ResourceType[crownjewelv1.CrownJewel, tfdriver.NameIdentifier] {
	return tfdriver.ResourceType[crownjewelv1.CrownJewel, tfdriver.NameIdentifier]{
		NewResourceClient: func(p tfsdk.Provider) tfdriver.ResourceClient[crownjewelv1.CrownJewel, tfdriver.NameIdentifier] {
			return teleport.NewCrownJewelClient(clientFromProvider(p))
		},
		Kind: types.KindCrownJewel,
		Codec: tfdriver.ResourceCodecFuncs[crownjewelv1.CrownJewel]{
			SchemaFunc:   schemav1.GenSchemaCrownJewel,
			ToStateFunc:  schemav1.CopyCrownJewelToTerraform,
			FromPlanFunc: schemav1.CopyCrownJewelFromTerraform,
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
