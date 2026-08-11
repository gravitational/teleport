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
	"github.com/hashicorp/terraform-plugin-framework/provider"

	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	"github.com/gravitational/teleport/api/types"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/teleport"
	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
	schemav1 "github.com/gravitational/teleport/integrations/terraform/tfschema/accessmonitoringrules/v1"
)

// NewAccessMonitoringRuleDataSourceType returns the access monitoring rule data source type.
func NewAccessMonitoringRuleDataSourceType() tfdriver.DataSourceType[accessmonitoringrulesv1.AccessMonitoringRule, tfdriver.NameIdentifier] {
	return tfdriver.DataSourceType[accessmonitoringrulesv1.AccessMonitoringRule, tfdriver.NameIdentifier]{
		NewDataSourceClient: func(p provider.Provider) tfdriver.DataSourceClient[accessmonitoringrulesv1.AccessMonitoringRule, tfdriver.NameIdentifier] {
			return teleport.NewAccessMonitoringRuleClient(clientFromProvider(p))
		},
		Kind: types.KindAccessMonitoringRule,
		Name: types.KindAccessMonitoringRule,
		Codec: tfdriver.DataSourceCodecFuncs[accessmonitoringrulesv1.AccessMonitoringRule]{
			SchemaFunc:  schemav1.GenSchemaAccessMonitoringRule,
			ToStateFunc: schemav1.CopyAccessMonitoringRuleToTerraform,
		},
		Identifier: tfdriver.NameIdentifierFromPath(path.Root("metadata").AtName("name")),
	}
}

// NewAccessMonitoringRuleResourceType returns the access monitoring rule resource type.
func NewAccessMonitoringRuleResourceType() tfdriver.ResourceType[accessmonitoringrulesv1.AccessMonitoringRule, tfdriver.NameIdentifier] {
	return tfdriver.ResourceType[accessmonitoringrulesv1.AccessMonitoringRule, tfdriver.NameIdentifier]{
		NewResourceClient: func(p provider.Provider) tfdriver.ResourceClient[accessmonitoringrulesv1.AccessMonitoringRule, tfdriver.NameIdentifier] {
			return teleport.NewAccessMonitoringRuleClient(clientFromProvider(p))
		},
		Kind: types.KindAccessMonitoringRule,
		Name: types.KindAccessMonitoringRule,
		Codec: tfdriver.ResourceCodecFuncs[accessmonitoringrulesv1.AccessMonitoringRule]{
			SchemaFunc:   schemav1.GenSchemaAccessMonitoringRule,
			ToStateFunc:  schemav1.CopyAccessMonitoringRuleToTerraform,
			FromPlanFunc: schemav1.CopyAccessMonitoringRuleFromTerraform,
		},
		Normalizer: tfdriver.ForceKind[accessmonitoringrulesv1.AccessMonitoringRule](types.KindAccessMonitoringRule),
		Identifier: tfdriver.NameIdentifierPolicy(path.Root("metadata").AtName("name"), func(accessMonitoringRule *accessmonitoringrulesv1.AccessMonitoringRule) string {
			return accessMonitoringRule.GetMetadata().GetName()
		}),
		ResourceRevision: func(st *accessmonitoringrulesv1.AccessMonitoringRule) string {
			return st.GetMetadata().GetRevision()
		},
	}
}
