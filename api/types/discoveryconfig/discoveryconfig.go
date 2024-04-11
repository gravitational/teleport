/*
Copyright 2023 Gravitational, Inc.

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

package discoveryconfig

import (
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/header/convert/legacy"
	"github.com/gravitational/teleport/api/utils"
)

// DiscoveryConfig describes extra discovery matchers that are added to DiscoveryServices that share the same Discovery Group.
type DiscoveryConfig struct {
	// ResourceHeader is the common resource header for all resources.
	header.ResourceHeader

	// Spec is the specification for the discovery config.
	Spec Spec `json:"spec" yaml:"spec"`

	// Status is the status for the discovery config.
	Status Status `json:"status" yaml:"status"`
}

// Spec is the specification for a discovery config.
type Spec struct {
	// DiscoveryGroup is the Discovery Group for the current DiscoveryConfig.
	// DiscoveryServices should include all the matchers if the DiscoveryGroup matches with their own group.
	DiscoveryGroup string `json:"discovery_group" yaml:"discovery_group"`

	// AWS is a list of matchers for the supported resources in AWS.
	AWS []types.AWSMatcher `json:"aws,omitempty" yaml:"aws"`
	// Azure is a list of matchers for the supported resources in Azure.
	Azure []types.AzureMatcher `json:"azure,omitempty" yaml:"azure"`
	// GCP is a list of matchers for the supported resources in GCP.
	GCP []types.GCPMatcher `json:"gcp,omitempty" yaml:"gcp"`
	// Kube is a list of matchers for the supported resources in Kubernetes.
	Kube []types.KubernetesMatcher `json:"kube,omitempty" yaml:"kube"`
	// AccessGraph is the configuration for the Access Graph Cloud sync.
	AccessGraph *types.AccessGraphSync `json:"access_graph,omitempty" yaml:"access_graph"`
}

// Equal checks if the discovery config is equal to another.
func (m *DiscoveryConfig) Equal(n *DiscoveryConfig) bool {
	return deriveTeleportEqualDiscoveryConfig(m, n)
}

// Status holds dynamic information about the discovery configuration
// running status such as errors, state and count of the resources.
type Status struct {
	// State is the current state of the discovery config.
	State string `json:"state" yaml:"state"`
	// ErrorMessage holds the error message when state is DISCOVERY_CONFIG_STATE_ERROR.
	ErrorMessage *string `json:"error_message,omitempty" yaml:"error_message,omitempty"`
	// DiscoveredResources holds the count of the discovered resources in the previous iteration.
	DiscoveredResources uint64 `json:"discovered_resources" yaml:"discovered_resources"`
	// LastSyncTime is the timestamp when the Discovery Config was last sync.
	LastSyncTime time.Time `json:"last_sync_time,omitempty" yaml:"last_sync_time,omitempty"`
}

// NewDiscoveryConfig will create a new discovery config.
func NewDiscoveryConfig(metadata header.Metadata, spec Spec) (*DiscoveryConfig, error) {
	discoveryConfig := &DiscoveryConfig{
		ResourceHeader: header.ResourceHeaderFromMetadata(metadata),
		Spec:           spec,
	}

	if err := discoveryConfig.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return discoveryConfig, nil
}

// CheckAndSetDefaults validates fields and populates empty fields with default values.
func (a *DiscoveryConfig) CheckAndSetDefaults() error {
	a.SetKind(types.KindDiscoveryConfig)
	a.SetVersion(types.V1)

	if err := a.ResourceHeader.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if a.Spec.DiscoveryGroup == "" {
		return trace.BadParameter("discovery config group required")
	}

	if a.Spec.AWS == nil {
		a.Spec.AWS = make([]types.AWSMatcher, 0)
	}
	for i := range a.Spec.AWS {
		if err := a.Spec.AWS[i].CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
	}

	if a.Spec.Azure == nil {
		a.Spec.Azure = make([]types.AzureMatcher, 0)
	}
	for i := range a.Spec.Azure {
		if err := a.Spec.Azure[i].CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
	}

	if a.Spec.GCP == nil {
		a.Spec.GCP = make([]types.GCPMatcher, 0)
	}
	for i := range a.Spec.GCP {
		if err := a.Spec.GCP[i].CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
	}

	if a.Spec.Kube == nil {
		a.Spec.Kube = make([]types.KubernetesMatcher, 0)
	}
	for i := range a.Spec.Kube {
		if err := a.Spec.Kube[i].CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
	}

	if a.Spec.AccessGraph != nil {
		if err := a.Spec.AccessGraph.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// GetDiscoveryGroup returns the DiscoveryGroup from the discovery config.
func (a *DiscoveryConfig) GetDiscoveryGroup() string {
	return a.Spec.DiscoveryGroup
}

// GetMetadata returns metadata. This is specifically for conforming to the Resource interface,
// and should be removed when possible.
func (a *DiscoveryConfig) GetMetadata() types.Metadata {
	return legacy.FromHeaderMetadata(a.Metadata)
}

// MatchSearch goes through select field values of a resource
// and tries to match against the list of search values.
func (a *DiscoveryConfig) MatchSearch(values []string) bool {
	fieldVals := append(utils.MapToStrings(a.GetAllLabels()), a.GetName(), a.GetDiscoveryGroup())
	return types.MatchSearch(fieldVals, values, nil)
}

// CloneResource returns a copy of the resource as types.ResourceWithLabels.
func (a *DiscoveryConfig) CloneResource() types.ResourceWithLabels {
	var copy *DiscoveryConfig
	utils.StrictObjectToStruct(a, &copy)
	return copy
}
