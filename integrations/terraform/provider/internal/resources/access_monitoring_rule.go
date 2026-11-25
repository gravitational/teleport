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
			SchemaFunc:  schemav1.GenSchemaAccessMonitoringRuleDataSource,
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
			SchemaFunc:   schemav1.GenSchemaAccessMonitoringRuleResource,
			ToStateFunc:  schemav1.CopyAccessMonitoringRuleToTerraform,
			FromPlanFunc: schemav1.CopyAccessMonitoringRuleFromTerraform,
			ModifyPlanFunc: tfdriver.NullToZeroModifyPlan(
				schemav1.CopyAccessMonitoringRuleFromTerraform,
				schemav1.CopyAccessMonitoringRuleToTerraformPreserveUnknown,
			),
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
