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
	"encoding/json"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// ValidateReverseTunnel validates the OIDC connector and sets default values
func ValidateReverseTunnel(rt types.ReverseTunnel) error {
	if err := CheckAndSetDefaults(rt); err != nil {
		return trace.Wrap(err)
	}

	for _, addr := range rt.GetDialAddrs() {
		if _, err := utils.ParseAddr(addr); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// UnmarshalReverseTunnel unmarshals the ReverseTunnel resource from JSON.
func UnmarshalReverseTunnel(bytes []byte, opts ...MarshalOption) (types.ReverseTunnel, error) {
	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing tunnel data")
	}
	var h types.ResourceHeader
	err := json.Unmarshal(bytes, &h)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch h.Version {
	case types.V2:
		var r types.ReverseTunnelV2
		if err := utils.FastUnmarshal(bytes, &r); err != nil {
			return nil, trace.BadParameter("%s", err)
		}
		if err := ValidateReverseTunnel(&r); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.Revision != "" {
			r.SetRevision(cfg.Revision)
		}
		if !cfg.Expires.IsZero() {
			r.SetExpiry(cfg.Expires)
		}
		return &r, nil
	}
	return nil, trace.BadParameter("reverse tunnel version %v is not supported", h.Version)
}

// MarshalReverseTunnel marshals the ReverseTunnel resource to JSON.
func MarshalReverseTunnel(reverseTunnel types.ReverseTunnel, opts ...MarshalOption) ([]byte, error) {
	if err := ValidateReverseTunnel(reverseTunnel); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch reverseTunnel := reverseTunnel.(type) {
	case *types.ReverseTunnelV2:
		return utils.FastMarshal(maybeResetProtoRevision(cfg.PreserveRevision, reverseTunnel))
	default:
		return nil, trace.BadParameter("unrecognized reverse tunnel version %T", reverseTunnel)
	}
}
