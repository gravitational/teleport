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
	"github.com/gravitational/teleport/api/types"
	"github.com/hashicorp/terraform-plugin-framework/provider"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/teleport"
	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
	"github.com/gravitational/teleport/integrations/terraform/tfschema"
)

// authPreferenceID is the static identifier used in the terraform state.
// Note that types.MetaNameClusterAuthPreference is "cluster-auth-preference"
// which can't be used here without causing churn.
const authPreferenceID = "auth_preference"

// NewAuthPreferenceDataSourceType returns the cluster auth preference data source type.
func NewAuthPreferenceDataSourceType() tfdriver.DataSourceType[types.AuthPreferenceV2, tfdriver.SingletonIdentifier] {
	return tfdriver.DataSourceType[types.AuthPreferenceV2, tfdriver.SingletonIdentifier]{
		NewDataSourceClient: func(p provider.Provider) tfdriver.DataSourceClient[types.AuthPreferenceV2, tfdriver.SingletonIdentifier] {
			return teleport.NewAuthPreferenceClient(clientFromProvider(p))
		},
		Kind: types.KindClusterAuthPreference,
		Name: "auth_preference",
		Codec: tfdriver.DataSourceCodecFuncs[types.AuthPreferenceV2]{
			SchemaFunc:  tfschema.GenSchemaAuthPreferenceV2,
			ToStateFunc: tfschema.CopyAuthPreferenceV2ToTerraform,
		},
		Identifier: tfdriver.SingletonIdentifierFromName(authPreferenceID),
	}
}

// NewAuthPreferenceResourceType returns the cluster auth preference resource type.
func NewAuthPreferenceResourceType() tfdriver.ResourceType[types.AuthPreferenceV2, tfdriver.SingletonIdentifier] {
	return tfdriver.ResourceType[types.AuthPreferenceV2, tfdriver.SingletonIdentifier]{
		NewResourceClient: func(p provider.Provider) tfdriver.ResourceClient[types.AuthPreferenceV2, tfdriver.SingletonIdentifier] {
			return teleport.NewAuthPreferenceClient(clientFromProvider(p))
		},
		Kind: types.KindClusterAuthPreference,
		Name: "auth_preference",
		Codec: tfdriver.ResourceCodecFuncs[types.AuthPreferenceV2]{
			SchemaFunc:   tfschema.GenSchemaAuthPreferenceV2,
			ToStateFunc:  tfschema.CopyAuthPreferenceV2ToTerraform,
			FromPlanFunc: tfschema.CopyAuthPreferenceV2FromTerraform,
		},
		Normalizer: tfdriver.CheckAndSetDefaults[types.AuthPreferenceV2](),
		Identifier: tfdriver.SingletonIdentifierPolicy[types.AuthPreferenceV2](authPreferenceID),
		ResourceRevision: func(st *types.AuthPreferenceV2) string {
			return st.GetMetadata().Revision
		},
	}
}
