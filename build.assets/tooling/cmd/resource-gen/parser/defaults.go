/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package parser

import (
	"fmt"
	"strings"

	optionsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/options/v1"
	"github.com/gravitational/teleport/build.assets/tooling/cmd/resource-gen/spec"
	"github.com/gravitational/trace"
)

func applyDefaults(serviceName, kindName string, cfg *optionsv1.ResourceConfig) (spec.ResourceSpec, error) {
	if cfg == nil {
		return spec.ResourceSpec{}, trace.BadParameter("resource config is required")
	}
	if cfg.Storage == nil {
		return spec.ResourceSpec{}, trace.BadParameter("storage config is required")
	}

	rs := spec.ResourceSpec{
		ServiceName: serviceName,
		Kind:        spec.PascalToSnake(kindName),
		KindPascal:  kindName,
		Cache: spec.CacheConfig{
			Enabled: true,
			Indexes: []string{"metadata.name"},
		},
		Tctl: spec.TctlConfig{
			Description:    fmt.Sprintf("%s resources", kindName),
			MFARequired:    true,
			Columns:        []string{"metadata.name"},
			VerboseColumns: []string{"metadata.name", "metadata.revision", "metadata.expires"},
		},
		Audit: spec.AuditConfig{
			EmitOnCreate: true,
			EmitOnUpdate: true,
			EmitOnDelete: true,
			EmitOnGet:    false,
			CodePrefix:   defaultCodePrefix(kindName),
		},
		Hooks: spec.HooksConfig{
			EnableLifecycleHooks: false,
		},
		Pagination: spec.PaginationConfig{
			DefaultPageSize: 200,
			MaxPageSize:     1000,
		},
	}

	rs.Storage.BackendPrefix = cfg.Storage.GetBackendPrefix()
	switch pattern := cfg.Storage.Pattern.(type) {
	case *optionsv1.StorageConfig_Standard:
		_ = pattern
		rs.Storage.Pattern = spec.StoragePatternStandard
	case *optionsv1.StorageConfig_Singleton:
		rs.Storage.Pattern = spec.StoragePatternSingleton
		rs.Storage.SingletonName = pattern.Singleton.GetFixedName()
	case *optionsv1.StorageConfig_Scoped:
		rs.Storage.Pattern = spec.StoragePatternScoped
		rs.Storage.ScopeBy = pattern.Scoped.GetBy()
	default:
		return spec.ResourceSpec{}, trace.BadParameter("storage pattern is required")
	}

	if cfg.Cache != nil {
		if cfg.Cache.Enabled != nil {
			rs.Cache.Enabled = cfg.Cache.GetEnabled()
		}
		if len(cfg.Cache.GetIndexes()) > 0 {
			rs.Cache.Indexes = append([]string(nil), cfg.Cache.GetIndexes()...)
		}
	}
	if cfg.Tctl != nil {
		if cfg.Tctl.GetDescription() != "" {
			rs.Tctl.Description = cfg.Tctl.GetDescription()
		}
		if cfg.Tctl.MfaRequired != nil {
			rs.Tctl.MFARequired = cfg.Tctl.GetMfaRequired()
		}
		if len(cfg.Tctl.GetColumns()) > 0 {
			rs.Tctl.Columns = append([]string(nil), cfg.Tctl.GetColumns()...)
		}
		if len(cfg.Tctl.GetVerboseColumns()) > 0 {
			rs.Tctl.VerboseColumns = append([]string(nil), cfg.Tctl.GetVerboseColumns()...)
		}
		if len(cfg.Tctl.GetTimestampColumns()) > 0 {
			rs.Tctl.TimestampColumns = append([]string(nil), cfg.Tctl.GetTimestampColumns()...)
		}
	}
	if cfg.Audit != nil {
		if cfg.Audit.EmitOnCreate != nil {
			rs.Audit.EmitOnCreate = cfg.Audit.GetEmitOnCreate()
		}
		if cfg.Audit.EmitOnUpdate != nil {
			rs.Audit.EmitOnUpdate = cfg.Audit.GetEmitOnUpdate()
		}
		if cfg.Audit.EmitOnDelete != nil {
			rs.Audit.EmitOnDelete = cfg.Audit.GetEmitOnDelete()
		}
		if cfg.Audit.EmitOnGet != nil {
			rs.Audit.EmitOnGet = cfg.Audit.GetEmitOnGet()
		}
		if cfg.Audit.GetCodePrefix() != "" {
			rs.Audit.CodePrefix = cfg.Audit.GetCodePrefix()
		}
	}
	if cfg.Hooks != nil {
		rs.Hooks.EnableLifecycleHooks = cfg.Hooks.GetEnableLifecycleHooks()
	}
	if cfg.Pagination != nil {
		if cfg.Pagination.DefaultPageSize != nil {
			rs.Pagination.DefaultPageSize = cfg.Pagination.GetDefaultPageSize()
		}
		if cfg.Pagination.MaxPageSize != nil {
			rs.Pagination.MaxPageSize = cfg.Pagination.GetMaxPageSize()
		}
	}

	return rs, nil
}

// defaultCodePrefix derives a 2-character uppercase code prefix from the kind name.
// This is used as a default when audit events are enabled but no explicit code_prefix
// is configured. It will be overridden by an explicit code_prefix in the proto config.
func defaultCodePrefix(kindName string) string {
	prefix := strings.ToUpper(kindName)
	if len(prefix) > 2 {
		prefix = prefix[:2]
	}
	return prefix
}
