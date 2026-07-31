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
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"

	apitypes "github.com/gravitational/teleport/api/types"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/teleport"
	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
	"github.com/gravitational/teleport/integrations/terraform/tfschema"
)

// NewServerDataSourceType returns the SSH server data source type.
func NewServerDataSourceType() tfdriver.DataSourceType[apitypes.ServerV2, tfdriver.ScopeQualifiedNameIdentifier] {
	return tfdriver.DataSourceType[apitypes.ServerV2, tfdriver.ScopeQualifiedNameIdentifier]{
		NewDataSourceClient: func(p tfsdk.Provider) tfdriver.DataSourceClient[apitypes.ServerV2, tfdriver.ScopeQualifiedNameIdentifier] {
			return teleport.NewServerClient(clientFromProvider(p))
		},
		Kind: apitypes.KindNode,
		Codec: tfdriver.DataSourceCodecFuncs[apitypes.ServerV2]{
			SchemaFunc:  tfschema.GenSchemaServerV2,
			ToStateFunc: tfschema.CopyServerV2ToTerraform,
		},
		Identifier: tfdriver.PossiblyUnscopedScopeQualifiedNameIdentifierFromPath(
			path.Root("metadata").AtName("name"),
			path.Root("scope"),
		),
	}
}

// NewServerResourceType returns the SSH server resource type.
func NewServerResourceType() tfdriver.ResourceType[apitypes.ServerV2, tfdriver.ScopeQualifiedNameIdentifier] {
	return tfdriver.ResourceType[apitypes.ServerV2, tfdriver.ScopeQualifiedNameIdentifier]{
		NewResourceClient: func(p tfsdk.Provider) tfdriver.ResourceClient[apitypes.ServerV2, tfdriver.ScopeQualifiedNameIdentifier] {
			return teleport.NewServerClient(clientFromProvider(p))
		},
		Kind: apitypes.KindNode,
		Codec: tfdriver.ResourceCodecFuncs[apitypes.ServerV2]{
			SchemaFunc:   tfschema.GenSchemaServerV2,
			ToStateFunc:  tfschema.CopyServerV2ToTerraform,
			FromPlanFunc: tfschema.CopyServerV2FromTerraform,
		},
		Normalizer: tfdriver.ResourceNormalizers[apitypes.ServerV2]{
			// The schema carries no kind, and ServerV2.CheckAndSetDefaults
			// rejects an empty one, so it must be set first. ServerV2 has no
			// SetKind method, so tfdriver.ForceKind cannot be used here.
			forceNodeKind(),
			tfdriver.CheckAndSetDefaults[apitypes.ServerV2](),
		},
		Identifier: tfdriver.PossiblyUnscopedScopeQualifiedNameIdentifierPolicy(
			path.Root("metadata").AtName("name"),
			path.Root("scope"),
			func(sv *apitypes.ServerV2) (string, string) {
				return sv.GetMetadata().Name, sv.GetScope()
			}),
		ResourceRevision: func(st *apitypes.ServerV2) string {
			return st.GetMetadata().Revision
		},
	}
}

// forceNodeKind sets the node kind on create and update.
func forceNodeKind() tfdriver.ResourceNormalizer[apitypes.ServerV2] {
	setKind := func(_ context.Context, server *apitypes.ServerV2) error {
		server.Kind = apitypes.KindNode
		return nil
	}

	return tfdriver.ResourceNormalizerFuncs[apitypes.ServerV2]{
		Create: setKind,
		Update: setKind,
	}
}
