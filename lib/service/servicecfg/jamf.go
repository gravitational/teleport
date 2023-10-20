// Copyright 2023 Gravitational, Inc
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

package servicecfg

import "github.com/gravitational/teleport/api/types"

// JamfConfig is the configuration for the Jamf MDM service.
type JamfConfig struct {
	// Spec is the configuration spec.
	Spec *types.JamfSpecV1
	// ExitOnSync controls whether the service performs a single sync operation
	// before exiting.
	ExitOnSync bool
}

func (j *JamfConfig) Enabled() bool {
	return j != nil && j.Spec != nil && j.Spec.Enabled
}
