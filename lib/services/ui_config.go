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
	"github.com/gravitational/teleport/lib/utils"
)

// UnmarshalUIConfig unmarshals the UIConfig resource from JSON.
func UnmarshalUIConfig(data []byte, opts ...MarshalOption) (types.UIConfig, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var uiconfig types.UIConfigV1
	if err := utils.FastUnmarshal(data, &uiconfig); err != nil {
		return nil, trace.BadParameter("%s", err)
	}
	if err := uiconfig.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.Revision != "" {
		uiconfig.SetRevision(cfg.Revision)
	}
	if !cfg.Expires.IsZero() {
		uiconfig.SetExpiry(cfg.Expires)
	}
	return &uiconfig, nil
}

// MarshalUIConfig marshals the UIConfig resource to JSON.
func MarshalUIConfig(uiconfig types.UIConfig, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch uiconfig := uiconfig.(type) {
	case *types.UIConfigV1:
		if err := uiconfig.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return utils.FastMarshal(maybeResetProtoRevision(cfg.PreserveRevision, uiconfig))
	default:
		return nil, trace.BadParameter("unrecognized uiconfig version %T", uiconfig)
	}
}
