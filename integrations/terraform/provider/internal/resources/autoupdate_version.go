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
			SchemaFunc:  schemav1.GenSchemaAutoUpdateVersion,
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
			SchemaFunc:   schemav1.GenSchemaAutoUpdateVersion,
			ToStateFunc:  schemav1.CopyAutoUpdateVersionToTerraform,
			FromPlanFunc: schemav1.CopyAutoUpdateVersionFromTerraform,
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
