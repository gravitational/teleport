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
	"encoding/json"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// ValidateReverseTunnel validates the OIDC connector and sets default values
func ValidateReverseTunnel(rt types.ReverseTunnel) error {
	if err := rt.CheckAndSetDefaults(); err != nil {
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
			return nil, trace.BadParameter(err.Error())
		}
		if err := ValidateReverseTunnel(&r); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.ID != 0 {
			r.SetResourceID(cfg.ID)
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
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *reverseTunnel
			copy.SetResourceID(0)
			reverseTunnel = &copy
		}
		return utils.FastMarshal(reverseTunnel)
	default:
		return nil, trace.BadParameter("unrecognized reverse tunnel version %T", reverseTunnel)
	}
}
