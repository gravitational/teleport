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
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

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

	if msg.GetSpec() == nil {
		return nil, trace.BadParameter("spec is missing")
	}
	if msg.GetSpec().GetDiscoveryGroup() == "" {
		return nil, trace.BadParameter("discovery group is missing")
	}

	awsMatchers := make([]types.AWSMatcher, 0, len(msg.GetSpec().GetAws()))
	for _, m := range msg.GetSpec().GetAws() {
		awsMatchers = append(awsMatchers, *m)
	}

	azureMatchers := make([]types.AzureMatcher, 0, len(msg.GetSpec().GetAzure()))
	for _, m := range msg.GetSpec().GetAzure() {
		azureMatchers = append(azureMatchers, *m)
	}

	gcpMatchers := make([]types.GCPMatcher, 0, len(msg.GetSpec().GetGcp()))
	for _, m := range msg.GetSpec().GetGcp() {
		gcpMatchers = append(gcpMatchers, *m)
	}

	kubeMatchers := make([]types.KubernetesMatcher, 0, len(msg.GetSpec().GetKube()))
	for _, m := range msg.GetSpec().GetKube() {
		kubeMatchers = append(kubeMatchers, *m)
	}

	discoveryConfig, err := discoveryconfig.NewDiscoveryConfig(
		headerv1.FromMetadataProto(msg.GetHeader().GetMetadata()),
		discoveryconfig.Spec{
			DiscoveryGroup: msg.GetSpec().GetDiscoveryGroup(),
			AWS:            awsMatchers,
			Azure:          azureMatchers,
			GCP:            gcpMatchers,
			Kube:           kubeMatchers,
			AccessGraph:    msg.GetSpec().GetAccessGraph(),
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	discoveryConfig.Status = StatusFromProto(msg.GetStatus())
	return discoveryConfig, nil
}

// StatusFromProto converts a v1 discovery config status into an internal discovery config status object.
func StatusFromProto(msg *discoveryconfigv1.DiscoveryConfigStatus) discoveryconfig.Status {
	if msg == nil {
		return discoveryconfig.Status{}
	}
	var lastSyncTime time.Time
	if msg.LastSyncTime != nil {
		lastSyncTime = msg.LastSyncTime.AsTime()
	}
	return discoveryconfig.Status{
		State:                          discoveryconfigv1.DiscoveryConfigState_name[int32(msg.State)],
		ErrorMessage:                   msg.ErrorMessage,
		DiscoveredResources:            msg.DiscoveredResources,
		LastSyncTime:                   lastSyncTime,
		IntegrationDiscoveredResources: msg.IntegrationDiscoveredResources,
	}
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
			AccessGraph:    discoveryConfig.Spec.AccessGraph,
		},
		Status: StatusToProto(discoveryConfig.Status),
	}
}

// StatusToProto converts a discovery config status into the protobuf discovery config status object.
func StatusToProto(status discoveryconfig.Status) *discoveryconfigv1.DiscoveryConfigStatus {
	var lastSyncTime *timestamppb.Timestamp
	if !status.LastSyncTime.IsZero() {
		lastSyncTime = timestamppb.New(status.LastSyncTime)
	}

	return &discoveryconfigv1.DiscoveryConfigStatus{
		State:                          discoveryconfigv1.DiscoveryConfigState(discoveryconfigv1.DiscoveryConfigState_value[status.State]),
		ErrorMessage:                   status.ErrorMessage,
		DiscoveredResources:            status.DiscoveredResources,
		LastSyncTime:                   lastSyncTime,
		IntegrationDiscoveredResources: status.IntegrationDiscoveredResources,
	}
}
