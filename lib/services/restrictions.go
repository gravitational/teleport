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
	"encoding/json"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

type Restrictions interface {
	GetNetworkRestrictions(context.Context) (types.NetworkRestrictions, error)
	SetNetworkRestrictions(context.Context, types.NetworkRestrictions) error
	DeleteNetworkRestrictions(context.Context) error
}

// ValidateNetworkRestrictions validates the network restrictions and sets defaults
func ValidateNetworkRestrictions(nr *types.NetworkRestrictionsV4) error {
	if err := nr.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// UnmarshalReverseTunnel unmarshals the ReverseTunnel resource from JSON.
func UnmarshalNetworkRestrictions(bytes []byte, opts ...MarshalOption) (types.NetworkRestrictions, error) {
	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing network restrictions data")
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
	case types.V4:
		var nr types.NetworkRestrictionsV4
		if err := utils.FastUnmarshal(bytes, &nr); err != nil {
			return nil, trace.BadParameter(err.Error())
		}
		if err := ValidateNetworkRestrictions(&nr); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.ID != 0 {
			nr.SetResourceID(cfg.ID)
		}
		if cfg.Revision != "" {
			nr.SetRevision(cfg.Revision)
		}
		if !cfg.Expires.IsZero() {
			nr.SetExpiry(cfg.Expires)
		}
		return &nr, nil
	}
	return nil, trace.BadParameter("network restrictions version %v is not supported", h.Version)
}

// MarshalNetworkRestrictions marshals the NetworkRestrictions resource to JSON.
func MarshalNetworkRestrictions(restrictions types.NetworkRestrictions, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if version := restrictions.GetVersion(); version != types.V4 {
		return nil, trace.BadParameter("mismatched network restrictions version %v and type %T", version, restrictions)
	}

	switch restrictions := restrictions.(type) {
	case *types.NetworkRestrictionsV4:
		return utils.FastMarshal(maybeResetProtoResourceID(cfg.PreserveResourceID, restrictions))
	default:
		return nil, trace.BadParameter("unrecognized network restrictions version %T", restrictions)
	}
}
