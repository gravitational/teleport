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

package v1

import (
	"github.com/gravitational/trace"

	discoveryconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryconfig/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	headerv1 "github.com/gravitational/teleport/api/types/header/convert/v1"
)

// FromProto converts a v1 discovery config into an internal discovery config object.
func FromProto(msg *discoveryconfigv1.DiscoveryConfig) (*discoveryconfig.DiscoveryConfig, error) {
	if msg == nil {
		return nil, trace.BadParameter("discovery config message is nil")
	}

	if msg.Spec == nil {
		return nil, trace.BadParameter("spec is missing")
	}
	if msg.Spec.DiscoveryGroup == "" {
		return nil, trace.BadParameter("discovery group is missing")
	}

	awsMatchers := make([]types.AWSMatcher, 0, len(msg.Spec.Aws))
	for _, m := range msg.Spec.Aws {
		awsMatchers = append(awsMatchers, *m)
	}

	azureMatchers := make([]types.AzureMatcher, 0, len(msg.Spec.Azure))
	for _, m := range msg.Spec.Azure {
		azureMatchers = append(azureMatchers, *m)
	}

	gcpMatchers := make([]types.GCPMatcher, 0, len(msg.Spec.Gcp))
	for _, m := range msg.Spec.Gcp {
		gcpMatchers = append(gcpMatchers, *m)
	}

	kubeMatchers := make([]types.KubernetesMatcher, 0, len(msg.Spec.Kube))
	for _, m := range msg.Spec.Kube {
		kubeMatchers = append(kubeMatchers, *m)
	}

	discoveryConfig, err := discoveryconfig.NewDiscoveryConfig(
		headerv1.FromMetadataProto(msg.Header.Metadata),
		discoveryconfig.Spec{
			DiscoveryGroup: msg.Spec.DiscoveryGroup,
			AWS:            awsMatchers,
			Azure:          azureMatchers,
			GCP:            gcpMatchers,
			Kube:           kubeMatchers,
		},
	)

	return discoveryConfig, trace.Wrap(err)
}

// ToProto converts an internal discovery config into a v1 discovery config object.
func ToProto(discoveryConfig *discoveryconfig.DiscoveryConfig) *discoveryconfigv1.DiscoveryConfig {
	awsMatchers := make([]*types.AWSMatcher, 0, len(discoveryConfig.Spec.AWS))
	for _, m := range discoveryConfig.Spec.AWS {
		m := m
		awsMatchers = append(awsMatchers, &m)
	}

	azureMatchers := make([]*types.AzureMatcher, 0, len(discoveryConfig.Spec.Azure))
	for _, m := range discoveryConfig.Spec.Azure {
		azureMatchers = append(azureMatchers, &m)
	}

	gcpMatchers := make([]*types.GCPMatcher, 0, len(discoveryConfig.Spec.GCP))
	for _, m := range discoveryConfig.Spec.GCP {
		gcpMatchers = append(gcpMatchers, &m)
	}

	kubeMatchers := make([]*types.KubernetesMatcher, 0, len(discoveryConfig.Spec.Kube))
	for _, m := range discoveryConfig.Spec.Kube {
		kubeMatchers = append(kubeMatchers, &m)
	}

	return &discoveryconfigv1.DiscoveryConfig{
		Header: headerv1.ToResourceHeaderProto(discoveryConfig.ResourceHeader),
		Spec: &discoveryconfigv1.DiscoveryConfigSpec{
			DiscoveryGroup: discoveryConfig.GetDiscoveryGroup(),
			Aws:            awsMatchers,
			Azure:          azureMatchers,
			Gcp:            gcpMatchers,
			Kube:           kubeMatchers,
		},
	}
}
