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
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"

	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	apitypes "github.com/gravitational/teleport/api/types"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/teleport"
	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
	schemav1 "github.com/gravitational/teleport/integrations/terraform/tfschema/beams/v1"
)

// NewBeamsConfigDataSourceType returns the beams config data source type.
func NewBeamsConfigDataSourceType() tfdriver.DataSourceType[beamsv1.BeamsConfig, tfdriver.SingletonIdentifier] {
	return tfdriver.DataSourceType[beamsv1.BeamsConfig, tfdriver.SingletonIdentifier]{
		NewDataSourceClient: func(p tfsdk.Provider) tfdriver.DataSourceClient[beamsv1.BeamsConfig, tfdriver.SingletonIdentifier] {
			return teleport.NewBeamsConfigClient(clientFromProvider(p))
		},
		Identifier: tfdriver.SingletonIdentifierFromName(apitypes.MetaNameBeamsConfig),
		Kind:       apitypes.KindBeamsConfig,
		Codec: tfdriver.DataSourceCodecFuncs[beamsv1.BeamsConfig]{
			SchemaFunc:  schemav1.GenSchemaBeamsConfig,
			ToStateFunc: schemav1.CopyBeamsConfigToTerraform,
		},
	}
}

// NewBeamsConfigResourceType returns the beams config resource type.
func NewBeamsConfigResourceType() tfdriver.ResourceType[beamsv1.BeamsConfig, tfdriver.SingletonIdentifier] {
	return tfdriver.ResourceType[beamsv1.BeamsConfig, tfdriver.SingletonIdentifier]{
		NewResourceClient: func(p tfsdk.Provider) tfdriver.ResourceClient[beamsv1.BeamsConfig, tfdriver.SingletonIdentifier] {
			return teleport.NewBeamsConfigClient(clientFromProvider(p))
		},
		Kind: apitypes.KindBeamsConfig,
		Codec: tfdriver.ResourceCodecFuncs[beamsv1.BeamsConfig]{
			SchemaFunc:   schemav1.GenSchemaBeamsConfig,
			ToStateFunc:  schemav1.CopyBeamsConfigToTerraform,
			FromPlanFunc: schemav1.CopyBeamsConfigFromTerraform,
		},
		Normalizer: tfdriver.ResourceNormalizers[beamsv1.BeamsConfig]{
			tfdriver.ForceKind[beamsv1.BeamsConfig](apitypes.KindBeamsConfig),
			tfdriver.SetDefaultName[beamsv1.BeamsConfig](apitypes.MetaNameBeamsConfig),
		},
		Identifier: tfdriver.SingletonIdentifierPolicy[beamsv1.BeamsConfig](apitypes.MetaNameBeamsConfig),
		ResourceRevision: func(bc *beamsv1.BeamsConfig) string {
			return bc.GetMetadata().GetRevision()
		},
	}
}
