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
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"

	"github.com/gravitational/teleport/api/types"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/teleport"
	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
	"github.com/gravitational/teleport/integrations/terraform/tfschema"
)

// sessionRecordingConfigID is the static identifier used in the terraform state.
// Note that types.MetaNameSessionRecordingConfig is "session-recording-config"
// which can't be used here without causing churn.
const sessionRecordingConfigID = "session_recording_config"

// NewSessionRecordingConfigDataSourceType returns the session recording config data source type.
func NewSessionRecordingConfigDataSourceType() tfdriver.DataSourceType[types.SessionRecordingConfigV2, tfdriver.SingletonIdentifier] {
	return tfdriver.DataSourceType[types.SessionRecordingConfigV2, tfdriver.SingletonIdentifier]{
		NewDataSourceClient: func(p tfsdk.Provider) tfdriver.DataSourceClient[types.SessionRecordingConfigV2, tfdriver.SingletonIdentifier] {
			return teleport.NewSessionRecordingConfigClient(clientFromProvider(p))
		},
		Kind: types.KindSessionRecordingConfig,
		Codec: tfdriver.DataSourceCodecFuncs[types.SessionRecordingConfigV2]{
			SchemaFunc:  tfschema.GenSchemaSessionRecordingConfigV2,
			ToStateFunc: tfschema.CopySessionRecordingConfigV2ToTerraform,
		},
		Identifier: tfdriver.SingletonIdentifierFromName(sessionRecordingConfigID),
	}
}

// NewSessionRecordingConfigResourceType returns the session recording config resource type.
func NewSessionRecordingConfigResourceType() tfdriver.ResourceType[types.SessionRecordingConfigV2, tfdriver.SingletonIdentifier] {
	return tfdriver.ResourceType[types.SessionRecordingConfigV2, tfdriver.SingletonIdentifier]{
		NewResourceClient: func(p tfsdk.Provider) tfdriver.ResourceClient[types.SessionRecordingConfigV2, tfdriver.SingletonIdentifier] {
			return teleport.NewSessionRecordingConfigClient(clientFromProvider(p))
		},
		Kind: types.KindSessionRecordingConfig,
		Codec: tfdriver.ResourceCodecFuncs[types.SessionRecordingConfigV2]{
			SchemaFunc:   tfschema.GenSchemaSessionRecordingConfigV2,
			ToStateFunc:  tfschema.CopySessionRecordingConfigV2ToTerraform,
			FromPlanFunc: tfschema.CopySessionRecordingConfigV2FromTerraform,
		},
		Normalizer: tfdriver.CheckAndSetDefaults[types.SessionRecordingConfigV2](),
		Identifier: tfdriver.SingletonIdentifierPolicy[types.SessionRecordingConfigV2](sessionRecordingConfigID),
		ResourceRevision: func(st *types.SessionRecordingConfigV2) string {
			return st.GetMetadata().Revision
		},
	}
}
