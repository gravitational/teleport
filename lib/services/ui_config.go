/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
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
		return nil, trace.BadParameter(err.Error())
	}
	if err := uiconfig.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.ID != 0 {
		uiconfig.SetResourceID(cfg.ID)
	}
	if !cfg.Expires.IsZero() {
		uiconfig.SetExpiry(cfg.Expires)
	}
	return &uiconfig, nil
}

// MarshalUIConfig marshals the UIConfig resource to JSON.
func MarshalUIConfig(uiconfig types.UIConfig, opts ...MarshalOption) ([]byte, error) {
	if err := uiconfig.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch uiconfig := uiconfig.(type) {
	case *types.UIConfigV1:
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *uiconfig
			copy.SetResourceID(0)
			uiconfig = &copy
		}
		return utils.FastMarshal(uiconfig)
	default:
		return nil, trace.BadParameter("unrecognized uiconfig version %T", uiconfig)
	}
}
