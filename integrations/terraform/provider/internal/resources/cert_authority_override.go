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

	subcav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/subca/v1"
	"github.com/gravitational/teleport/api/types"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/teleport"
	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
	schemav1 "github.com/gravitational/teleport/integrations/terraform/tfschema/subca/v1"
)

// NewCertAuthorityOverrideDataSourceType returns the cert authority override data source type.
func NewCertAuthorityOverrideDataSourceType() tfdriver.DataSourceType[subcav1.CertAuthorityOverride, tfdriver.NameIdentifier] {
	return tfdriver.DataSourceType[subcav1.CertAuthorityOverride, tfdriver.NameIdentifier]{
		NewDataSourceClient: func(p provider.Provider) tfdriver.DataSourceClient[subcav1.CertAuthorityOverride, tfdriver.NameIdentifier] {
			return teleport.NewCertAuthorityOverrideClient(clientFromProvider(p))
		},
		Kind: types.KindCertAuthorityOverride,
		Name: types.KindCertAuthorityOverride,
		Codec: tfdriver.DataSourceCodecFuncs[subcav1.CertAuthorityOverride]{
			SchemaFunc:  schemav1.GenSchemaCertAuthorityOverrideDataSource,
			ToStateFunc: schemav1.CopyCertAuthorityOverrideToTerraform,
		},
		Identifier: tfdriver.NameIdentifierFromPath(path.Root("sub_kind")),
	}
}

// NewCertAuthorityOverrideResourceType returns the cert authority override resource type.
func NewCertAuthorityOverrideResourceType() tfdriver.ResourceType[subcav1.CertAuthorityOverride, tfdriver.NameIdentifier] {
	return tfdriver.ResourceType[subcav1.CertAuthorityOverride, tfdriver.NameIdentifier]{
		NewResourceClient: func(p provider.Provider) tfdriver.ResourceClient[subcav1.CertAuthorityOverride, tfdriver.NameIdentifier] {
			return teleport.NewCertAuthorityOverrideClient(clientFromProvider(p))
		},
		Kind: types.KindCertAuthorityOverride,
		Name: types.KindCertAuthorityOverride,
		Codec: tfdriver.ResourceCodecFuncs[subcav1.CertAuthorityOverride]{
			SchemaFunc:   schemav1.GenSchemaCertAuthorityOverrideResource,
			ToStateFunc:  schemav1.CopyCertAuthorityOverrideToTerraform,
			FromPlanFunc: schemav1.CopyCertAuthorityOverrideFromTerraform,
			ModifyPlanFunc: tfdriver.NullToZeroModifyPlan(
				schemav1.CopyCertAuthorityOverrideFromTerraform,
				schemav1.CopyCertAuthorityOverrideToTerraformPreserveUnknown,
			),
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
