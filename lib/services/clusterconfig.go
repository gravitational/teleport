/*
Copyright 2021 Gravitational, Inc.

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

package services

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
)

// DefaultClusterConfig is used as the default cluster configuration when
// one is not specified (record at node).
func DefaultClusterConfig() types.ClusterConfig {
	return &types.ClusterConfigV3{
		Kind:    types.KindClusterConfig,
		Version: types.V3,
		Metadata: types.Metadata{
			Name:      types.MetaNameClusterConfig,
			Namespace: defaults.Namespace,
		},
		Spec: types.ClusterConfigSpecV3{},
	}
}

// AuditConfigFromObject returns audit config from interface object
func AuditConfigFromObject(in interface{}) (*types.AuditConfig, error) {
	var cfg types.AuditConfig
	if in == nil {
		return &cfg, nil
	}
	if err := utils.ObjectToStruct(in, &cfg); err != nil {
		return nil, trace.Wrap(err)
	}
	return &cfg, nil
}

// ShouldUploadSessions returns whether audit config
// instructs server to upload sessions
func ShouldUploadSessions(a types.AuditConfig) bool {
	return a.AuditSessionsURI != ""
}

// UnmarshalClusterConfig unmarshals the ClusterConfig resource from JSON.
func UnmarshalClusterConfig(bytes []byte, opts ...MarshalOption) (types.ClusterConfig, error) {
	var clusterConfig types.ClusterConfigV3

	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := utils.FastUnmarshal(bytes, &clusterConfig); err != nil {
		return nil, trace.BadParameter(err.Error())
	}

	err = clusterConfig.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.ID != 0 {
		clusterConfig.SetResourceID(cfg.ID)
	}
	if !cfg.Expires.IsZero() {
		clusterConfig.SetExpiry(cfg.Expires)
	}
	return &clusterConfig, nil
}

// MarshalClusterConfig marshals the ClusterConfig resource to JSON.
func MarshalClusterConfig(clusterConfig types.ClusterConfig, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch clusterConfig := clusterConfig.(type) {
	case *types.ClusterConfigV3:
		if version := clusterConfig.GetVersion(); version != types.V3 {
			return nil, trace.BadParameter("mismatched cluster config version %v and type %T", version, clusterConfig)
		}
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *clusterConfig
			copy.SetResourceID(0)
			clusterConfig = &copy
		}
		return utils.FastMarshal(clusterConfig)
	default:
		return nil, trace.BadParameter("unrecognized cluster config version %T", clusterConfig)
	}
}
