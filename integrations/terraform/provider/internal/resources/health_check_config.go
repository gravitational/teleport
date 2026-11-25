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

	healthcheckconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/healthcheckconfig/v1"
	"github.com/gravitational/teleport/api/types"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/teleport"
	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
	schemav1 "github.com/gravitational/teleport/integrations/terraform/tfschema/healthcheckconfig/v1"
)

// NewHealthCheckConfigDataSourceType returns the health check config data source type.
func NewHealthCheckConfigDataSourceType() tfdriver.DataSourceType[healthcheckconfigv1.HealthCheckConfig, tfdriver.NameIdentifier] {
	return tfdriver.DataSourceType[healthcheckconfigv1.HealthCheckConfig, tfdriver.NameIdentifier]{
		NewDataSourceClient: func(p provider.Provider) tfdriver.DataSourceClient[healthcheckconfigv1.HealthCheckConfig, tfdriver.NameIdentifier] {
			return teleport.NewHealthCheckConfigClient(clientFromProvider(p))
		},
		Kind: types.KindHealthCheckConfig,
		Name: types.KindHealthCheckConfig,
		Codec: tfdriver.DataSourceCodecFuncs[healthcheckconfigv1.HealthCheckConfig]{
			SchemaFunc:  schemav1.GenSchemaHealthCheckConfigDataSource,
			ToStateFunc: schemav1.CopyHealthCheckConfigToTerraform,
		},
		Identifier: tfdriver.NameIdentifierFromPath(path.Root("metadata").AtName("name")),
	}
}

// NewHealthCheckConfigResourceType returns the health check config resource type.
func NewHealthCheckConfigResourceType() tfdriver.ResourceType[healthcheckconfigv1.HealthCheckConfig, tfdriver.NameIdentifier] {
	return tfdriver.ResourceType[healthcheckconfigv1.HealthCheckConfig, tfdriver.NameIdentifier]{
		NewResourceClient: func(p provider.Provider) tfdriver.ResourceClient[healthcheckconfigv1.HealthCheckConfig, tfdriver.NameIdentifier] {
			return teleport.NewHealthCheckConfigClient(clientFromProvider(p))
		},
		Kind: types.KindHealthCheckConfig,
		Name: types.KindHealthCheckConfig,
		Codec: tfdriver.ResourceCodecFuncs[healthcheckconfigv1.HealthCheckConfig]{
			SchemaFunc:   schemav1.GenSchemaHealthCheckConfigResource,
			ToStateFunc:  schemav1.CopyHealthCheckConfigToTerraform,
			FromPlanFunc: schemav1.CopyHealthCheckConfigFromTerraform,
			ModifyPlanFunc: tfdriver.NullToZeroModifyPlan(
				schemav1.CopyHealthCheckConfigFromTerraform,
				schemav1.CopyHealthCheckConfigToTerraformPreserveUnknown,
			),
		},
		Normalizer: tfdriver.ForceKind[healthcheckconfigv1.HealthCheckConfig](types.KindHealthCheckConfig),
		Identifier: tfdriver.NameIdentifierPolicy(path.Root("metadata").AtName("name"), func(healthCheckConfig *healthcheckconfigv1.HealthCheckConfig) string {
			return healthCheckConfig.GetMetadata().GetName()
		}),
		ResourceRevision: func(st *healthcheckconfigv1.HealthCheckConfig) string {
			return st.GetMetadata().GetRevision()
		},
	}
}
