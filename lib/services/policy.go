/**
 * Copyright 2022 Gravitational, Inc.
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
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
)

// UnmarshalPolicy unmarshals the Policy resource from JSON.
func UnmarshalPolicy(data []byte, opts ...MarshalOption) (types.Policy, error) {
	var policy types.AccessPolicyV1

	if len(data) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := utils.FastUnmarshal(data, &policy); err != nil {
		return nil, trace.BadParameter(err.Error())
	}
	if err := policy.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.ID != 0 {
		policy.SetResourceID(cfg.ID)
	}
	if !cfg.Expires.IsZero() {
		policy.SetExpiry(cfg.Expires)
	}
	return &policy, nil
}

// MarshalPolicy marshals the Policy resource to JSON.
func MarshalPolicy(policy types.Policy, opts ...MarshalOption) ([]byte, error) {
	if err := policy.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch policy := policy.(type) {
	case *types.AccessPolicyV1:
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *policy
			copy.SetResourceID(0)
			policy = &copy
		}
		return utils.FastMarshal(policy)
	default:
		return nil, trace.BadParameter("unrecognized policy version %T", policy)
	}
}
