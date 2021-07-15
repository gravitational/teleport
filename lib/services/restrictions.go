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
	"context"
	"encoding/json"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
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
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *restrictions
			copy.SetResourceID(0)
			restrictions = &copy
		}
		return utils.FastMarshal(restrictions)
	default:
		return nil, trace.BadParameter("unrecognized network restrictions version %T", restrictions)
	}
}
