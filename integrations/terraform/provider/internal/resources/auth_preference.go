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

	"github.com/gravitational/teleport/api/types"

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
			SchemaFunc:  tfschema.GenSchemaAuthPreferenceV2DataSource,
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
			SchemaFunc:   tfschema.GenSchemaAuthPreferenceV2Resource,
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
