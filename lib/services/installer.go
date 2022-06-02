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

// UnmarshalInstaller unmarshals the installer resource from JSON.
func UnmarshalInstaller(data []byte, opts ...MarshalOption) (types.Installer, error) {
	var installer types.InstallerV1

	if len(data) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := utils.FastUnmarshal(data, &installer); err != nil {
		return nil, trace.BadParameter(err.Error())
	}
	if err := installer.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.ID != 0 {
		installer.SetResourceID(cfg.ID)
	}
	if !cfg.Expires.IsZero() {
		installer.SetExpiry(cfg.Expires)
	}
	return &installer, nil
}

// MarshalInstaller marshals the Installer resource to JSON.
func MarshalInstaller(installer types.Installer, opts ...MarshalOption) ([]byte, error) {
	if err := installer.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch installer := installer.(type) {
	case *types.InstallerV1:
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *installer
			copy.SetResourceID(0)
			installer = &copy
		}
		return utils.FastMarshal(installer)
	default:
		return nil, trace.BadParameter("unrecognized installer version %T", installer)
	}
}
