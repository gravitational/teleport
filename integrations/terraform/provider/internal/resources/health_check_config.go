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

	healthcheckconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/healthcheckconfig/v1"
	"github.com/gravitational/teleport/api/types"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/teleport"
	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
	schemav1 "github.com/gravitational/teleport/integrations/terraform/tfschema/healthcheckconfig/v1"
)

// NewHealthCheckConfigDataSourceType returns the health check config data source type.
func NewHealthCheckConfigDataSourceType() tfdriver.DataSourceType[healthcheckconfigv1.HealthCheckConfig, tfdriver.NameIdentifier] {
	return tfdriver.DataSourceType[healthcheckconfigv1.HealthCheckConfig, tfdriver.NameIdentifier]{
		NewDataSourceClient: func(p tfsdk.Provider) tfdriver.DataSourceClient[healthcheckconfigv1.HealthCheckConfig, tfdriver.NameIdentifier] {
			return teleport.NewHealthCheckConfigClient(clientFromProvider(p))
		},
		Kind: types.KindHealthCheckConfig,
		Codec: tfdriver.DataSourceCodecFuncs[healthcheckconfigv1.HealthCheckConfig]{
			SchemaFunc:  schemav1.GenSchemaHealthCheckConfig,
			ToStateFunc: schemav1.CopyHealthCheckConfigToTerraform,
		},
		Identifier: tfdriver.NameIdentifierFromPath(path.Root("metadata").AtName("name")),
	}
}

// NewHealthCheckConfigResourceType returns the health check config resource type.
func NewHealthCheckConfigResourceType() tfdriver.ResourceType[healthcheckconfigv1.HealthCheckConfig, tfdriver.NameIdentifier] {
	return tfdriver.ResourceType[healthcheckconfigv1.HealthCheckConfig, tfdriver.NameIdentifier]{
		NewResourceClient: func(p tfsdk.Provider) tfdriver.ResourceClient[healthcheckconfigv1.HealthCheckConfig, tfdriver.NameIdentifier] {
			return teleport.NewHealthCheckConfigClient(clientFromProvider(p))
		},
		Kind: types.KindHealthCheckConfig,
		Codec: tfdriver.ResourceCodecFuncs[healthcheckconfigv1.HealthCheckConfig]{
			SchemaFunc:   schemav1.GenSchemaHealthCheckConfig,
			ToStateFunc:  schemav1.CopyHealthCheckConfigToTerraform,
			FromPlanFunc: schemav1.CopyHealthCheckConfigFromTerraform,
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
