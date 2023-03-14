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

import "github.com/gravitational/teleport/lib/services"

type DiscoveryConfig struct {
	Enabled bool
	// AWSMatchers are used to match EC2 instances for auto enrollment.
	AWSMatchers []services.AWSMatcher
	// AzureMatchers are used to match resources for auto enrollment.
	AzureMatchers []services.AzureMatcher
	// GCPMatchers are used to match GCP resources for auto discovery.
	GCPMatchers []services.GCPMatcher
}

// IsEmpty validates if the Discovery Service config has no cloud matchers.
func (d DiscoveryConfig) IsEmpty() bool {
	return len(d.AWSMatchers) == 0 &&
		len(d.AzureMatchers) == 0 && len(d.GCPMatchers) == 0
}
