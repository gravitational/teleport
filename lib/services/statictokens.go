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
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
)

// DefaultStaticTokens is used to get the default static tokens (empty list)
// when nothing is specified in file configuration.
func DefaultStaticTokens() StaticTokens {
	return &types.StaticTokensV2{
		Kind:    KindStaticTokens,
		Version: V2,
		Metadata: types.Metadata{
			Name:      MetaNameStaticTokens,
			Namespace: defaults.Namespace,
		},
		Spec: types.StaticTokensSpecV2{
			StaticTokens: []types.ProvisionTokenV1{},
		},
	}
}

// UnmarshalStaticTokens unmarshals the StaticTokens resource from JSON.
func UnmarshalStaticTokens(bytes []byte, opts ...MarshalOption) (StaticTokens, error) {
	var staticTokens types.StaticTokensV2

	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := utils.FastUnmarshal(bytes, &staticTokens); err != nil {
		return nil, trace.BadParameter(err.Error())
	}
	if err := staticTokens.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.ID != 0 {
		staticTokens.SetResourceID(cfg.ID)
	}
	if !cfg.Expires.IsZero() {
		staticTokens.SetExpiry(cfg.Expires)
	}
	return &staticTokens, nil
}

// MarshalStaticTokens marshals the StaticTokens resource to JSON.
func MarshalStaticTokens(staticToken StaticTokens, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch staticToken := staticToken.(type) {
	case *types.StaticTokensV2:
		if version := staticToken.GetVersion(); version != V2 {
			return nil, trace.BadParameter("mismatched static token version %v and type %T", version, staticToken)
		}
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *staticToken
			copy.SetResourceID(0)
			staticToken = &copy
		}
		return utils.FastMarshal(staticToken)
	default:
		return nil, trace.BadParameter("unrecognized static token version %T", staticToken)
	}
}
