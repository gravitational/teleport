/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

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
	// KubernetesMatchers are used to match services inside Kubernetes cluster for auto discovery
	KubernetesMatchers []types.KubernetesMatcher
	// AccessGraph is used to sync cloud provider resources into Access Graph.
	AccessGraph *types.AccessGraphSync
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

// IsEmpty validates if the Discovery Service config has no matchers and no discovery group.
// DiscoveryGroup is used to dynamically load Matchers when changing DiscoveryConfig resources.
func (d DiscoveryConfig) IsEmpty() bool {
	return len(d.AWSMatchers) == 0 && len(d.AzureMatchers) == 0 &&
		len(d.GCPMatchers) == 0 && len(d.KubernetesMatchers) == 0 &&
		d.DiscoveryGroup == "" &&
		(d.AccessGraph == nil || len(d.AccessGraph.AWS) == 0)
}
