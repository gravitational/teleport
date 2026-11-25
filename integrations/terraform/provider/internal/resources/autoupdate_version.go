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

	autoupdatev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	"github.com/gravitational/teleport/api/types"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/teleport"
	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
	schemav1 "github.com/gravitational/teleport/integrations/terraform/tfschema/autoupdate/v1"
)

// NewAutoUpdateVersionDataSourceType returns the autoupdate version data source type.
func NewAutoUpdateVersionDataSourceType() tfdriver.DataSourceType[autoupdatev1.AutoUpdateVersion, tfdriver.SingletonIdentifier] {
	return tfdriver.DataSourceType[autoupdatev1.AutoUpdateVersion, tfdriver.SingletonIdentifier]{
		NewDataSourceClient: func(p provider.Provider) tfdriver.DataSourceClient[autoupdatev1.AutoUpdateVersion, tfdriver.SingletonIdentifier] {
			return teleport.NewAutoUpdateVersionClient(clientFromProvider(p))
		},
		Kind: types.KindAutoUpdateVersion,
		Name: types.KindAutoUpdateVersion,
		Codec: tfdriver.DataSourceCodecFuncs[autoupdatev1.AutoUpdateVersion]{
			SchemaFunc:  schemav1.GenSchemaAutoUpdateVersionDataSource,
			ToStateFunc: schemav1.CopyAutoUpdateVersionToTerraform,
		},
		Identifier: tfdriver.SingletonIdentifierFromName(types.MetaNameAutoUpdateVersion),
	}
}

// NewAutoUpdateVersionResourceType returns the autoupdate version resource type.
func NewAutoUpdateVersionResourceType() tfdriver.ResourceType[autoupdatev1.AutoUpdateVersion, tfdriver.SingletonIdentifier] {
	return tfdriver.ResourceType[autoupdatev1.AutoUpdateVersion, tfdriver.SingletonIdentifier]{
		NewResourceClient: func(p provider.Provider) tfdriver.ResourceClient[autoupdatev1.AutoUpdateVersion, tfdriver.SingletonIdentifier] {
			return teleport.NewAutoUpdateVersionClient(clientFromProvider(p))
		},
		Kind: types.KindAutoUpdateVersion,
		Name: types.KindAutoUpdateVersion,
		Codec: tfdriver.ResourceCodecFuncs[autoupdatev1.AutoUpdateVersion]{
			SchemaFunc:   schemav1.GenSchemaAutoUpdateVersionResource,
			ToStateFunc:  schemav1.CopyAutoUpdateVersionToTerraform,
			FromPlanFunc: schemav1.CopyAutoUpdateVersionFromTerraform,
			ModifyPlanFunc: tfdriver.NullToZeroModifyPlan(
				schemav1.CopyAutoUpdateVersionFromTerraform,
				schemav1.CopyAutoUpdateVersionToTerraformPreserveUnknown,
			),
		},
		Normalizer: tfdriver.ResourceNormalizers[autoupdatev1.AutoUpdateVersion]{
			tfdriver.ForceKind[autoupdatev1.AutoUpdateVersion](types.KindAutoUpdateVersion),
			tfdriver.ForceName[autoupdatev1.AutoUpdateVersion](types.MetaNameAutoUpdateVersion),
		},
		Identifier: tfdriver.SingletonIdentifierPolicy[autoupdatev1.AutoUpdateVersion](types.MetaNameAutoUpdateVersion),
		ResourceRevision: func(st *autoupdatev1.AutoUpdateVersion) string {
			return st.GetMetadata().GetRevision()
		},
	}
}
