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
	"github.com/gravitational/teleport/lib/utils"
)

// ClusterConfigDerivedResources holds a set of the ClusterConfig-derived
// resources following the reorganization of RFD 28.
type ClusterConfigDerivedResources struct {
	types.ClusterAuditConfig
	types.ClusterNetworkingConfig
	types.SessionRecordingConfig
}

// NewDerivedResourcesFromClusterConfig converts a legacy ClusterConfig to the new
// configuration resources described in RFD 28.
// DELETE IN 8.0.0
func NewDerivedResourcesFromClusterConfig(cc types.ClusterConfig) (*ClusterConfigDerivedResources, error) {
	ccV3, ok := cc.(*types.ClusterConfigV3)
	if !ok {
		return nil, trace.BadParameter("unexpected ClusterConfig type %T", cc)
	}

	auditConfig, err := ccV3.GetClusterAuditConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	netConfig, err := ccV3.GetClusterNetworkingConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	recConfig, err := ccV3.GetSessionRecordingConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &ClusterConfigDerivedResources{
		ClusterAuditConfig:      auditConfig,
		ClusterNetworkingConfig: netConfig,
		SessionRecordingConfig:  recConfig,
	}, nil
}

// UpdateAuthPreferenceWithLegacyClusterConfig updates an AuthPreference with
// auth-related values that used to be stored in ClusterConfig.
// DELETE IN 8.0.0
func UpdateAuthPreferenceWithLegacyClusterConfig(cc types.ClusterConfig, authPref types.AuthPreference) error {
	ccV3, ok := cc.(*types.ClusterConfigV3)
	if !ok {
		return trace.BadParameter("unexpected ClusterConfig type %T", cc)
	}
	authPref.SetDisconnectExpiredCert(ccV3.Spec.DisconnectExpiredCert.Value())
	authPref.SetAllowLocalAuth(ccV3.Spec.AllowLocalAuth.Value())
	return nil
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
	if err := clusterConfig.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch clusterConfig := clusterConfig.(type) {
	case *types.ClusterConfigV3:
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
