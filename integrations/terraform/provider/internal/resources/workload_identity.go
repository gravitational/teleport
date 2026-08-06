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

	workloadidentityv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/teleport"
	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
	schemav1 "github.com/gravitational/teleport/integrations/terraform/tfschema/workloadidentity/v1"
)

// NewWorkloadIdentityDataSourceType returns the workload identity data source type.
func NewWorkloadIdentityDataSourceType() tfdriver.DataSourceType[workloadidentityv1.WorkloadIdentity, tfdriver.NameIdentifier] {
	return tfdriver.DataSourceType[workloadidentityv1.WorkloadIdentity, tfdriver.NameIdentifier]{
		NewDataSourceClient: func(p tfsdk.Provider) tfdriver.DataSourceClient[workloadidentityv1.WorkloadIdentity, tfdriver.NameIdentifier] {
			return teleport.NewWorkloadIdentityClient(clientFromProvider(p))
		},
		Kind: types.KindWorkloadIdentity,
		Codec: tfdriver.DataSourceCodecFuncs[workloadidentityv1.WorkloadIdentity]{
			SchemaFunc:  schemav1.GenSchemaWorkloadIdentity,
			ToStateFunc: schemav1.CopyWorkloadIdentityToTerraform,
		},
		Identifier: tfdriver.NameIdentifierFromPath(path.Root("metadata").AtName("name")),
	}
}

// NewWorkloadIdentityResourceType returns the workload identity resource type.
func NewWorkloadIdentityResourceType() tfdriver.ResourceType[workloadidentityv1.WorkloadIdentity, tfdriver.NameIdentifier] {
	return tfdriver.ResourceType[workloadidentityv1.WorkloadIdentity, tfdriver.NameIdentifier]{
		NewResourceClient: func(p tfsdk.Provider) tfdriver.ResourceClient[workloadidentityv1.WorkloadIdentity, tfdriver.NameIdentifier] {
			return teleport.NewWorkloadIdentityClient(clientFromProvider(p))
		},
		Kind: types.KindWorkloadIdentity,
		Codec: tfdriver.ResourceCodecFuncs[workloadidentityv1.WorkloadIdentity]{
			SchemaFunc:   schemav1.GenSchemaWorkloadIdentity,
			ToStateFunc:  schemav1.CopyWorkloadIdentityToTerraform,
			FromPlanFunc: schemav1.CopyWorkloadIdentityFromTerraform,
		},
		Normalizer: tfdriver.ForceKind[workloadidentityv1.WorkloadIdentity](types.KindWorkloadIdentity),
		Identifier: tfdriver.NameIdentifierPolicy(path.Root("metadata").AtName("name"), func(workloadIdentity *workloadidentityv1.WorkloadIdentity) string {
			return workloadIdentity.GetMetadata().GetName()
		}),
		ResourceRevision: func(st *workloadidentityv1.WorkloadIdentity) string {
			return st.GetMetadata().GetRevision()
		},
	}
}
