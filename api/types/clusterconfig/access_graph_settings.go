/*
Copyright 2024 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package clusterconfig

import (
	"github.com/gravitational/trace"

	clusterconfigpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
)

// NewAccessGraphSettings creates a new AccessGraphSettings resource.
func NewAccessGraphSettings(spec *clusterconfigpb.AccessGraphSettingsSpec) (*clusterconfigpb.AccessGraphSettings, error) {
	settings := &clusterconfigpb.AccessGraphSettings{
		Kind:    types.KindAccessGraphSettings,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: types.MetaNameAccessGraphSettings,
		},
		Spec: spec,
	}
	if err := ValidateAccessGraphSettings(settings); err != nil {
		return nil, trace.Wrap(err)
	}

	return settings, nil

}

// ValidateAccessGraphSettings checks that required parameters are set
func ValidateAccessGraphSettings(s *clusterconfigpb.AccessGraphSettings) error {
	if s == nil {
		return trace.BadParameter("AccessGraphSettings is nil")
	}
	if s.Metadata == nil {
		return trace.BadParameter("Metadata is nil")
	}
	if s.Spec == nil {
		return trace.BadParameter("Spec is nil")
	}

	if s.Metadata.Name == "" {
		return trace.BadParameter("Name is unset")
	}

	if s.Metadata.Name != types.MetaNameAccessGraphSettings {
		return trace.BadParameter("Name is not %s", types.MetaNameAccessGraphSettings)
	}

	if s.Kind != types.KindAccessGraphSettings {
		return trace.BadParameter("Kind is not AccessGraphSettings")
	}
	if s.Version != types.V1 {
		return trace.BadParameter("Version is not V1")
	}

	switch s.Spec.GetSecretsScanConfig() {
	case clusterconfigpb.AccessGraphSecretsScanConfig_ACCESS_GRAPH_SECRETS_SCAN_CONFIG_ENABLED, clusterconfigpb.AccessGraphSecretsScanConfig_ACCESS_GRAPH_SECRETS_SCAN_CONFIG_DISABLED:
	default:
		return trace.BadParameter("SecretsScanConfig is invalid")
	}

	return nil
}
