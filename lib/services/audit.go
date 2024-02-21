/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
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
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch auditConfig := auditConfig.(type) {
	case *types.ClusterAuditConfigV2:
		if err := auditConfig.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return utils.FastMarshal(maybeResetProtoResourceID(cfg.PreserveResourceID, auditConfig))
	default:
		return nil, trace.BadParameter("unrecognized cluster audit config version %T", auditConfig)
	}
}
