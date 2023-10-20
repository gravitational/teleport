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
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/utils"
)

// ClusterAuditConfigSpecFromObject returns audit config spec from object.
func ClusterAuditConfigSpecFromObject(in interface{}) (*types.ClusterAuditConfigSpecV2, error) {
	var cfg types.ClusterAuditConfigSpecV2
	if in == nil {
		return &cfg, nil
	}
	if err := apiutils.ObjectToStruct(in, &cfg); err != nil {
		return nil, trace.Wrap(err)
	}
	return &cfg, nil
}

// UnmarshalClusterAuditConfig unmarshals the ClusterAuditConfig resource from JSON.
func UnmarshalClusterAuditConfig(bytes []byte, opts ...MarshalOption) (types.ClusterAuditConfig, error) {
	var auditConfig types.ClusterAuditConfigV2

	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := utils.FastUnmarshal(bytes, &auditConfig); err != nil {
		return nil, trace.BadParameter(err.Error())
	}
	if err := auditConfig.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.ID != 0 {
		auditConfig.SetResourceID(cfg.ID)
	}
	if cfg.Revision != "" {
		auditConfig.SetRevision(cfg.Revision)
	}
	if !cfg.Expires.IsZero() {
		auditConfig.SetExpiry(cfg.Expires)
	}
	return &auditConfig, nil
}

// MarshalClusterAuditConfig marshals the ClusterAuditConfig resource to JSON.
func MarshalClusterAuditConfig(auditConfig types.ClusterAuditConfig, opts ...MarshalOption) ([]byte, error) {
	if err := auditConfig.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch auditConfig := auditConfig.(type) {
	case *types.ClusterAuditConfigV2:
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *auditConfig
			copy.SetResourceID(0)
			copy.SetRevision("")
			auditConfig = &copy
		}
		return utils.FastMarshal(auditConfig)
	default:
		return nil, trace.BadParameter("unrecognized cluster audit config version %T", auditConfig)
	}
}
