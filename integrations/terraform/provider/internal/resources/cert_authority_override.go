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

	subcav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/subca/v1"
	"github.com/gravitational/teleport/api/types"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/teleport"
	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
	schemav1 "github.com/gravitational/teleport/integrations/terraform/tfschema/subca/v1"
)

// NewCertAuthorityOverrideDataSourceType returns the cert authority override data source type.
func NewCertAuthorityOverrideDataSourceType() tfdriver.DataSourceType[subcav1.CertAuthorityOverride, tfdriver.NameIdentifier] {
	return tfdriver.DataSourceType[subcav1.CertAuthorityOverride, tfdriver.NameIdentifier]{
		NewDataSourceClient: func(p tfsdk.Provider) tfdriver.DataSourceClient[subcav1.CertAuthorityOverride, tfdriver.NameIdentifier] {
			return teleport.NewCertAuthorityOverrideClient(clientFromProvider(p))
		},
		Kind: types.KindCertAuthorityOverride,
		Codec: tfdriver.DataSourceCodecFuncs[subcav1.CertAuthorityOverride]{
			SchemaFunc:  schemav1.GenSchemaCertAuthorityOverride,
			ToStateFunc: schemav1.CopyCertAuthorityOverrideToTerraform,
		},
		Identifier: tfdriver.NameIdentifierFromPath(path.Root("sub_kind")),
	}
}

// NewCertAuthorityOverrideResourceType returns the cert authority override resource type.
func NewCertAuthorityOverrideResourceType() tfdriver.ResourceType[subcav1.CertAuthorityOverride, tfdriver.NameIdentifier] {
	return tfdriver.ResourceType[subcav1.CertAuthorityOverride, tfdriver.NameIdentifier]{
		NewResourceClient: func(p tfsdk.Provider) tfdriver.ResourceClient[subcav1.CertAuthorityOverride, tfdriver.NameIdentifier] {
			return teleport.NewCertAuthorityOverrideClient(clientFromProvider(p))
		},
		Kind: types.KindCertAuthorityOverride,
		Codec: tfdriver.ResourceCodecFuncs[subcav1.CertAuthorityOverride]{
			SchemaFunc:   schemav1.GenSchemaCertAuthorityOverride,
			ToStateFunc:  schemav1.CopyCertAuthorityOverrideToTerraform,
			FromPlanFunc: schemav1.CopyCertAuthorityOverrideFromTerraform,
		},
		Normalizer: tfdriver.ForceKind[subcav1.CertAuthorityOverride](types.KindCertAuthorityOverride),
		Identifier: tfdriver.NameIdentifierPolicy(
			path.Root("sub_kind"),
			func(override *subcav1.CertAuthorityOverride) string {
				return override.GetSubKind()
			}),
		ResourceRevision: func(st *subcav1.CertAuthorityOverride) string {
			return st.GetMetadata().GetRevision()
		},
	}
}
