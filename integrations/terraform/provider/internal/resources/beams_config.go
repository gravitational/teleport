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
	"github.com/hashicorp/terraform-plugin-framework/provider"

	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	apitypes "github.com/gravitational/teleport/api/types"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/teleport"
	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
	schemav1 "github.com/gravitational/teleport/integrations/terraform/tfschema/beams/v1"
)

// NewBeamsConfigDataSourceType returns the beams config data source type.
func NewBeamsConfigDataSourceType() tfdriver.DataSourceType[beamsv1.BeamsConfig, tfdriver.SingletonIdentifier] {
	return tfdriver.DataSourceType[beamsv1.BeamsConfig, tfdriver.SingletonIdentifier]{
		NewDataSourceClient: func(p provider.Provider) tfdriver.DataSourceClient[beamsv1.BeamsConfig, tfdriver.SingletonIdentifier] {
			return teleport.NewBeamsConfigClient(clientFromProvider(p))
		},
		Identifier: tfdriver.SingletonIdentifierFromName(apitypes.MetaNameBeamsConfig),
		Kind:       apitypes.KindBeamsConfig,
		Name:       apitypes.KindBeamsConfig,
		Codec: tfdriver.DataSourceCodecFuncs[beamsv1.BeamsConfig]{
			SchemaFunc:  schemav1.GenSchemaBeamsConfigDataSource,
			ToStateFunc: schemav1.CopyBeamsConfigToTerraform,
		},
	}
}

// NewBeamsConfigResourceType returns the beams config resource type.
func NewBeamsConfigResourceType() tfdriver.ResourceType[beamsv1.BeamsConfig, tfdriver.SingletonIdentifier] {
	return tfdriver.ResourceType[beamsv1.BeamsConfig, tfdriver.SingletonIdentifier]{
		NewResourceClient: func(p provider.Provider) tfdriver.ResourceClient[beamsv1.BeamsConfig, tfdriver.SingletonIdentifier] {
			return teleport.NewBeamsConfigClient(clientFromProvider(p))
		},
		Kind: apitypes.KindBeamsConfig,
		Name: apitypes.KindBeamsConfig,
		Codec: tfdriver.ResourceCodecFuncs[beamsv1.BeamsConfig]{
			SchemaFunc:   schemav1.GenSchemaBeamsConfigResource,
			ToStateFunc:  schemav1.CopyBeamsConfigToTerraform,
			FromPlanFunc: schemav1.CopyBeamsConfigFromTerraform,
			ModifyPlanFunc: tfdriver.NullToZeroModifyPlan(
				schemav1.CopyBeamsConfigFromTerraform,
				schemav1.CopyBeamsConfigToTerraformPreserveUnknown,
			),
		},
		Normalizer: tfdriver.ForceKind[beamsv1.BeamsConfig](apitypes.KindBeamsConfig),
		Identifier: tfdriver.SingletonIdentifierPolicy[beamsv1.BeamsConfig](apitypes.MetaNameBeamsConfig),
		ResourceRevision: func(bc *beamsv1.BeamsConfig) string {
			return bc.GetMetadata().GetRevision()
		},
	}
}
