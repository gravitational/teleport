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
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	headerv1 "github.com/gravitational/teleport/api/types/header/convert/v1"
)

// FromProto converts a v1 discovery config into an internal one, discarding the client-supplied
// subkind so that it cannot relax validation or alter installer parameters. This preserves the
// conversion behavior from before static snapshots existed and is the safe default for every user-input path.
func FromProto(msg *discoveryconfigv1.DiscoveryConfig) (*discoveryconfig.DiscoveryConfig, error) {
	if msg == nil {
		return nil, trace.BadParameter("discovery config message is nil")
	}
	return fromProto(msg, "")
}

// FromProtoWithSubKind converts a server-returned v1 discovery config into an internal one,
// preserving its subkind. Use this only for trusted read responses, where retaining the
// server-assigned static-snapshot marker is required.
func FromProtoWithSubKind(msg *discoveryconfigv1.DiscoveryConfig) (*discoveryconfig.DiscoveryConfig, error) {
	if msg == nil {
		return nil, trace.BadParameter("discovery config message is nil")
	}
	return fromProto(msg, msg.GetHeader().GetSubKind())
}

func fromProto(msg *discoveryconfigv1.DiscoveryConfig, subKind string) (*discoveryconfig.DiscoveryConfig, error) {
	if msg.GetSpec() == nil {
		return nil, trace.BadParameter("spec is missing")
	}
	// Static snapshots mirror the owning service's file configuration, which may legitimately
	// have no discovery group. Unknown subkinds retain regular validation. Snapshot specs are
	// additionally sanitized inside the constructor, so unsupported installer params in a
	// received snapshot are discarded rather than rejected.
	if msg.GetSpec().GetDiscoveryGroup() == "" && subKind != discoveryconfig.SubKindStaticSnapshot {
		return nil, trace.BadParameter("discovery group is missing")
	}

	discoveryConfig, err := discoveryconfig.NewDiscoveryConfigWithSubKind(
		headerv1.FromMetadataProto(msg.GetHeader().GetMetadata()),
		SpecFromProto(msg.GetSpec()),
		subKind,
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

	integrationDiscoveredResources := make(map[string]*discoveryconfig.IntegrationDiscoveredSummary, len(msg.IntegrationDiscoveredResources))
	for k, v := range msg.IntegrationDiscoveredResources {
		integrationDiscoveredResources[k] = &discoveryconfig.IntegrationDiscoveredSummary{
			IntegrationDiscoveredSummary: v,
		}
	}

	serverStatus := make(map[string]*discoveryconfig.DiscoveryStatusServer, len(msg.ServerStatus))
	for k, v := range msg.ServerStatus {
		serverStatus[k] = &discoveryconfig.DiscoveryStatusServer{
			DiscoveryStatusServer: v,
		}
	}
	return discoveryconfig.Status{
		State:                          discoveryconfigv1.DiscoveryConfigState_name[int32(msg.State)],
		ErrorMessage:                   msg.ErrorMessage,
		DiscoveredResources:            msg.GetDiscoveredResources(),
		LastSyncTime:                   lastSyncTime,
		IntegrationDiscoveredResources: integrationDiscoveredResources,
		ServerStatus:                   serverStatus,
	}
}

// SpecFromProto converts a v1 discovery config spec into its internal
// representation.
func SpecFromProto(msg *discoveryconfigv1.DiscoveryConfigSpec) discoveryconfig.Spec {
	s := discoveryconfig.Spec{DiscoveryGroup: msg.GetDiscoveryGroup(), AccessGraph: msg.GetAccessGraph()}
	for _, m := range msg.GetAws() {
		s.AWS = append(s.AWS, *m)
	}
	for _, m := range msg.GetAzure() {
		s.Azure = append(s.Azure, *m)
	}
	for _, m := range msg.GetGcp() {
		s.GCP = append(s.GCP, *m)
	}
	for _, m := range msg.GetKube() {
		s.Kube = append(s.Kube, *m)
	}
	return s
}

// ToProto converts an internal discovery config into a v1 discovery config object.
func ToProto(discoveryConfig *discoveryconfig.DiscoveryConfig) *discoveryconfigv1.DiscoveryConfig {
	return &discoveryconfigv1.DiscoveryConfig{
		Header: headerv1.ToResourceHeaderProto(discoveryConfig.ResourceHeader),
		Spec:   specToProto(discoveryConfig.Spec),
		Status: StatusToProto(discoveryConfig.Status),
	}
}

// StatusToProto converts a discovery config status into the protobuf discovery config status object.
func StatusToProto(status discoveryconfig.Status) *discoveryconfigv1.DiscoveryConfigStatus {
	var lastSyncTime *timestamppb.Timestamp
	if !status.LastSyncTime.IsZero() {
		lastSyncTime = timestamppb.New(status.LastSyncTime)
	}

	integrationDiscoveredResources := make(map[string]*discoveryconfigv1.IntegrationDiscoveredSummary, len(status.IntegrationDiscoveredResources))
	for k, v := range status.IntegrationDiscoveredResources {
		if v == nil {
			v = &discoveryconfig.IntegrationDiscoveredSummary{}
		}
		if v.IntegrationDiscoveredSummary == nil {
			v.IntegrationDiscoveredSummary = &discoveryconfigv1.IntegrationDiscoveredSummary{}
		}
		integrationDiscoveredResources[k] = v.IntegrationDiscoveredSummary
	}

	serverStatus := make(map[string]*discoveryconfigv1.DiscoveryStatusServer, len(status.ServerStatus))
	for k, v := range status.ServerStatus {
		if v == nil {
			v = &discoveryconfig.DiscoveryStatusServer{}
		}
		if v.DiscoveryStatusServer == nil {
			v.DiscoveryStatusServer = &discoveryconfigv1.DiscoveryStatusServer{}
		}
		serverStatus[k] = v.DiscoveryStatusServer
	}

	return &discoveryconfigv1.DiscoveryConfigStatus{
		State:                          discoveryconfigv1.DiscoveryConfigState(discoveryconfigv1.DiscoveryConfigState_value[status.State]),
		ErrorMessage:                   status.ErrorMessage,
		DiscoveredResources:            status.DiscoveredResources,
		LastSyncTime:                   lastSyncTime,
		IntegrationDiscoveredResources: integrationDiscoveredResources,
		ServerStatus:                   serverStatus,
	}
}

// specToProto shallow-copies each matcher so the returned message does not
// alias the source spec's slice elements.
func specToProto(s discoveryconfig.Spec) *discoveryconfigv1.DiscoveryConfigSpec {
	p := &discoveryconfigv1.DiscoveryConfigSpec{DiscoveryGroup: s.DiscoveryGroup, AccessGraph: s.AccessGraph}
	for _, m := range s.AWS {
		p.Aws = append(p.Aws, &m)
	}
	for _, m := range s.Azure {
		p.Azure = append(p.Azure, &m)
	}
	for _, m := range s.GCP {
		p.Gcp = append(p.Gcp, &m)
	}
	for _, m := range s.Kube {
		p.Kube = append(p.Kube, &m)
	}
	return p
}
