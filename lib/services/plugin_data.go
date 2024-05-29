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
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// PluginDataGetter defines the interface for getting plugin data.
type PluginDataGetter interface {
	// GetPluginData loads all plugin data matching the supplied filter.
	GetPluginData(ctx context.Context, filter types.PluginDataFilter) ([]types.PluginData, error)
}

// PluginData defines the interface for managing plugin data.
type PluginData interface {
	PluginDataGetter

	// UpdatePluginData updates a per-resource PluginData entry.
	UpdatePluginData(ctx context.Context, params types.PluginDataUpdateParams) error
}

// MarshalPluginData marshals the PluginData resource to JSON.
func MarshalPluginData(pluginData types.PluginData, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch pluginData := pluginData.(type) {
	case *types.PluginDataV3:
		if err := pluginData.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return utils.FastMarshal(maybeResetProtoRevision(cfg.PreserveRevision, pluginData))
	default:
		return nil, trace.BadParameter("unrecognized plugin data type: %T", pluginData)
	}
}

// UnmarshalPluginData unmarshals the PluginData resource from JSON.
func UnmarshalPluginData(raw []byte, opts ...MarshalOption) (types.PluginData, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var data types.PluginDataV3
	if err := utils.FastUnmarshal(raw, &data); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := data.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	if cfg.Revision != "" {
		data.SetRevision(cfg.Revision)
	}
	if !cfg.Expires.IsZero() {
		data.SetExpiry(cfg.Expires)
	}
	return &data, nil
}
