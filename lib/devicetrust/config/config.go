// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/trace"
)

// ValidateConfigAgainstModules verifies the device trust configuration against
// the current modules.
// This method exists to provide feedback to users about invalid configurations,
// Teleport itself checks the features where appropriate and reacts accordingly.
func ValidateConfigAgainstModules(cfg *types.DeviceTrust) error {
	switch {
	case modules.GetModules().BuildType() == modules.BuildEnterprise:
		// OK, all features enabled.
		return nil
	case cfg == nil, cfg.Mode == "", cfg.Mode == constants.DeviceTrustModeOff:
		// OK, absent/default/off are always allowed.
		return nil
	default:
		return trace.BadParameter("device trust mode %q requires Teleport Enterprise", cfg.Mode)
	}
}
