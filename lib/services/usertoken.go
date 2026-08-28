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

// UnmarshalUserToken unmarshals the UserToken resource from JSON.
func UnmarshalUserToken(bytes []byte, opts ...MarshalOption) (types.UserToken, error) {
	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}

	var token types.UserTokenV3
	if err := utils.FastUnmarshal(bytes, &token); err != nil {
		return nil, trace.BadParameter("%s", err)
	}
	if err := token.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &token, nil
}

// MarshalUserToken marshals the UserToken resource to JSON.
func MarshalUserToken(token types.UserToken, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch t := token.(type) {
	case *types.UserTokenV3:
		if err := t.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		if !cfg.PreserveRevision {
			copy := *t
			copy.SetRevision("")
			t = &copy
		}
		return utils.FastMarshal(t)
	default:
		return nil, trace.BadParameter("unsupported user token resource %T", t)
	}
}
