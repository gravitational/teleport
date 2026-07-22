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

// UnmarshalStaticTokens unmarshals the StaticTokens resource from JSON.
func UnmarshalStaticTokens(bytes []byte, opts ...MarshalOption) (types.StaticTokens, error) {
	var staticTokens types.StaticTokensV2

	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := utils.FastUnmarshal(bytes, &staticTokens); err != nil {
		return nil, trace.BadParameter("%s", err)
	}
	if err := staticTokens.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.Revision != "" {
		staticTokens.SetRevision(cfg.Revision)
	}
	if !cfg.Expires.IsZero() {
		staticTokens.SetExpiry(cfg.Expires)
	}
	return &staticTokens, nil
}

// MarshalStaticTokens marshals the StaticTokens resource to JSON.
func MarshalStaticTokens(staticToken types.StaticTokens, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch staticToken := staticToken.(type) {
	case *types.StaticTokensV2:
		if err := staticToken.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return utils.FastMarshal(maybeResetProtoRevision(cfg.PreserveRevision, staticToken))
	default:
		return nil, trace.BadParameter("unrecognized static token version %T", staticToken)
	}
}
