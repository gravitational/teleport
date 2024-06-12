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

// UnmarshalUserTokenSecrets unmarshals the UserTokenSecrets resource from JSON.
func UnmarshalUserTokenSecrets(bytes []byte, opts ...MarshalOption) (types.UserTokenSecrets, error) {
	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}

	var secrets types.UserTokenSecretsV3
	if err := utils.FastUnmarshal(bytes, &secrets); err != nil {
		return nil, trace.BadParameter(err.Error())
	}
	if err := secrets.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &secrets, nil
}

// MarshalUserTokenSecrets marshals the UserTokenSecrets resource to JSON.
func MarshalUserTokenSecrets(secrets types.UserTokenSecrets, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch t := secrets.(type) {
	case *types.UserTokenSecretsV3:
		if err := t.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		if !cfg.PreserveResourceID {
			copy := *t
			copy.SetResourceID(0)
			copy.SetRevision("")
			t = &copy
		}
		return utils.FastMarshal(t)
	default:
		return nil, trace.BadParameter("unsupported user token secrets resource %T", t)
	}
}
