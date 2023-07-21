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

import (
	"time"

	"github.com/gravitational/teleport/api/types"
)

type DiscoveryConfig struct {
	Enabled bool
	// AWSMatchers are used to match EC2 instances for auto enrollment.
	AWSMatchers []types.AWSMatcher
	// AzureMatchers are used to match resources for auto enrollment.
	AzureMatchers []types.AzureMatcher
	// GCPMatchers are used to match GCP resources for auto discovery.
	GCPMatchers []types.GCPMatcher
	// DiscoveryGroup is the name of the discovery group that the current
	// discovery service is a part of.
	// It is used to filter out discovered resources that belong to another
	// discovery services. When running in high availability mode and the agents
	// have access to the same cloud resources, this field value must be the same
	// for all discovery services. If different agents are used to discover different
	// sets of cloud resources, this field must be different for each set of agents.
	DiscoveryGroup string
	// PollInterval is the cadence at which the discovery server will run each of its
	// discovery cycles.
	PollInterval time.Duration
}

// IsEmpty validates if the Discovery Service config has no cloud matchers.
func (d DiscoveryConfig) IsEmpty() bool {
	return len(d.AWSMatchers) == 0 &&
		len(d.AzureMatchers) == 0 && len(d.GCPMatchers) == 0
}
