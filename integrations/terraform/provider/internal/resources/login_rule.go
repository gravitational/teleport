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

	loginrulev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"
	"github.com/gravitational/teleport/api/types"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/teleport"
	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
	schemav1 "github.com/gravitational/teleport/integrations/terraform/tfschema/loginrule/v1"
)

// NewLoginRuleDataSourceType returns the login rule data source type.
func NewLoginRuleDataSourceType() tfdriver.DataSourceType[loginrulev1.LoginRule, tfdriver.NameIdentifier] {
	return tfdriver.DataSourceType[loginrulev1.LoginRule, tfdriver.NameIdentifier]{
		NewDataSourceClient: func(p tfsdk.Provider) tfdriver.DataSourceClient[loginrulev1.LoginRule, tfdriver.NameIdentifier] {
			return teleport.NewLoginRuleClient(clientFromProvider(p))
		},
		Kind: types.KindLoginRule,
		Codec: tfdriver.DataSourceCodecFuncs[loginrulev1.LoginRule]{
			SchemaFunc:  schemav1.GenSchemaLoginRule,
			ToStateFunc: schemav1.CopyLoginRuleToTerraform,
		},
		Identifier: tfdriver.NameIdentifierFromPath(path.Root("metadata").AtName("name")),
	}
}

// NewLoginRuleResourceType returns the login rule resource type.
func NewLoginRuleResourceType() tfdriver.ResourceType[loginrulev1.LoginRule, tfdriver.NameIdentifier] {
	return tfdriver.ResourceType[loginrulev1.LoginRule, tfdriver.NameIdentifier]{
		NewResourceClient: func(p tfsdk.Provider) tfdriver.ResourceClient[loginrulev1.LoginRule, tfdriver.NameIdentifier] {
			return teleport.NewLoginRuleClient(clientFromProvider(p))
		},
		Kind: types.KindLoginRule,
		Codec: tfdriver.ResourceCodecFuncs[loginrulev1.LoginRule]{
			SchemaFunc:   schemav1.GenSchemaLoginRule,
			ToStateFunc:  schemav1.CopyLoginRuleToTerraform,
			FromPlanFunc: schemav1.CopyLoginRuleFromTerraform,
		},
		Identifier: tfdriver.NameIdentifierPolicy(path.Root("metadata").AtName("name"), func(loginRule *loginrulev1.LoginRule) string {
			return loginRule.GetMetadata().GetName()
		}),
		ResourceRevision: func(st *loginrulev1.LoginRule) string {
			return st.GetMetadata().GetRevision()
		},
	}
}
