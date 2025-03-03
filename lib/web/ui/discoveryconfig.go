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

package ui

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
)

// DiscoveryConfig describes DiscoveryConfig fields
type DiscoveryConfig struct {
	// Name is the DiscoveryConfig name.
	Name string `json:"name,omitempty"`
	// DiscoveryGroup is the Group of the DiscoveryConfig.
	DiscoveryGroup string `json:"discoveryGroup,omitempty"`
	// AWS is a list of matchers for AWS resources.
	AWS []types.AWSMatcher `json:"aws,omitempty"`
	// Azure is a list of matchers for Azure resources.
	Azure []types.AzureMatcher `json:"azureMatchers,omitempty"`
	// GCP is a list of matchers for GCP resources.
	GCP []types.GCPMatcher `json:"gcpMatchers,omitempty"`
	// Kube is a list of matchers for AWS resources.
	Kube []types.KubernetesMatcher `json:"kube,omitempty"`
	// AccessGraph is the configuration for the Access Graph Cloud sync.
	AccessGraph *types.AccessGraphSync `json:"accessGraph,omitempty"`
	// Status is the status of the DiscoveryConfig.
	Status discoveryconfig.Status `json:"status,omitempty"`
}

// CheckAndSetDefaults for the create request.
// Name and SubKind is required.
func (r *DiscoveryConfig) CheckAndSetDefaults() error {
	if r.Name == "" {
		return trace.BadParameter("missing discovery config name")
	}

	if r.DiscoveryGroup == "" {
		return trace.BadParameter("missing discovery group")
	}

	return nil
}

// UpdateDiscoveryConfigRequest is a request to update a DiscoveryConfig
type UpdateDiscoveryConfigRequest struct {
	// DiscoveryGroup is the Group of the DiscoveryConfig.
	DiscoveryGroup string `json:"discoveryGroup,omitempty"`
	// AWS is a list of matchers for AWS resources.
	AWS []types.AWSMatcher `json:"aws,omitempty"`
	// Azure is a list of matchers for Azure resources.
	Azure []types.AzureMatcher `json:"azureMatchers,omitempty"`
	// GCP is a list of matchers for GCP resources.
	GCP []types.GCPMatcher `json:"gcpMatchers,omitempty"`
	// Kube is a list of matchers for AWS resources.
	Kube []types.KubernetesMatcher `json:"kube,omitempty"`
	// AccessGraph is the configuration for the Access Graph Cloud sync.
	AccessGraph *types.AccessGraphSync `json:"accessGraph,omitempty"`
}

// CheckAndSetDefaults checks if the provided values are valid.
func (r *UpdateDiscoveryConfigRequest) CheckAndSetDefaults() error {
	if r.DiscoveryGroup == "" {
		return trace.BadParameter("missing discovery group")
	}

	return nil
}

// DiscoveryConfigsListResponse contains a list of DiscoveryConfigs.
// In case of exceeding the pagination limit (either via query param `limit` or the default 1000)
// a `nextToken` is provided and should be used to obtain the next page (as a query param `startKey`)
type DiscoveryConfigsListResponse struct {
	// Items is a list of resources retrieved.
	Items []DiscoveryConfig `json:"items"`
	// NextKey is the position to resume listing events.
	NextKey string `json:"nextKey"`
}

// MakeDiscoveryConfigs creates a UI list of DiscoveryConfigs.
func MakeDiscoveryConfigs(dcs []*discoveryconfig.DiscoveryConfig) []DiscoveryConfig {
	uiList := make([]DiscoveryConfig, 0, len(dcs))

	for _, dc := range dcs {
		uiList = append(uiList, MakeDiscoveryConfig(dc))
	}

	return uiList
}

// MakeDiscoveryConfig creates a UI DiscoveryConfig representation.
func MakeDiscoveryConfig(dc *discoveryconfig.DiscoveryConfig) DiscoveryConfig {
	return DiscoveryConfig{
		Name:           dc.GetName(),
		DiscoveryGroup: dc.GetDiscoveryGroup(),
		AWS:            dc.Spec.AWS,
		Azure:          dc.Spec.Azure,
		GCP:            dc.Spec.GCP,
		Kube:           dc.Spec.Kube,
		AccessGraph:    dc.Spec.AccessGraph,
		Status:         dc.Status,
	}
}
