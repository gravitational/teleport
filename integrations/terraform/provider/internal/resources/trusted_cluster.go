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

	"github.com/gravitational/teleport/api/types"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/teleport"
	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
	"github.com/gravitational/teleport/integrations/terraform/tfschema"
)

// NewTrustedClusterDataSourceType returns the trusted cluster data source type.
func NewTrustedClusterDataSourceType() tfdriver.DataSourceType[types.TrustedClusterV2, tfdriver.NameIdentifier] {
	return tfdriver.DataSourceType[types.TrustedClusterV2, tfdriver.NameIdentifier]{
		NewDataSourceClient: func(p tfsdk.Provider) tfdriver.DataSourceClient[types.TrustedClusterV2, tfdriver.NameIdentifier] {
			return teleport.NewTrustedClusterClient(clientFromProvider(p))
		},
		Kind: types.KindTrustedCluster,
		Codec: tfdriver.DataSourceCodecFuncs[types.TrustedClusterV2]{
			SchemaFunc:  tfschema.GenSchemaTrustedClusterV2,
			ToStateFunc: tfschema.CopyTrustedClusterV2ToTerraform,
		},
		Identifier: tfdriver.NameIdentifierFromPath(path.Root("metadata").AtName("name")),
	}
}

// NewTrustedClusterResourceType returns the trusted cluster resource type.
func NewTrustedClusterResourceType() tfdriver.ResourceType[types.TrustedClusterV2, tfdriver.NameIdentifier] {
	return tfdriver.ResourceType[types.TrustedClusterV2, tfdriver.NameIdentifier]{
		NewResourceClient: func(p tfsdk.Provider) tfdriver.ResourceClient[types.TrustedClusterV2, tfdriver.NameIdentifier] {
			return teleport.NewTrustedClusterClient(clientFromProvider(p))
		},
		Kind: types.KindTrustedCluster,
		Codec: tfdriver.ResourceCodecFuncs[types.TrustedClusterV2]{
			SchemaFunc:   tfschema.GenSchemaTrustedClusterV2,
			ToStateFunc:  tfschema.CopyTrustedClusterV2ToTerraform,
			FromPlanFunc: tfschema.CopyTrustedClusterV2FromTerraform,
		},
		Normalizer: tfdriver.CheckAndSetDefaults[types.TrustedClusterV2](),
		Identifier: tfdriver.NameIdentifierPolicy(path.Root("metadata").AtName("name"), func(trustedCluster *types.TrustedClusterV2) string {
			return trustedCluster.GetMetadata().Name
		}),
		ResourceRevision: func(st *types.TrustedClusterV2) string {
			return st.GetMetadata().Revision
		},
	}
}
