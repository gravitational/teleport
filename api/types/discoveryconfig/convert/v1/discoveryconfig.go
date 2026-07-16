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
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	discoveryconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryconfig/v1"
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
	// Synthetic resources carry observed inventory in status and intentionally
	// keep their spec empty. Unknown subkinds retain regular validation.
	if msg.GetSpec().GetDiscoveryGroup() == "" && msg.GetHeader().GetSubKind() != discoveryconfig.SubKindSynthetic {
		return nil, trace.BadParameter("discovery group is missing")
	}
	if msg.GetHeader().GetSubKind() == discoveryconfig.SubKindSynthetic {
		msg = SanitizeSyntheticDiscoveryConfig(msg)
	}

	discoveryConfig, err := discoveryconfig.NewDiscoveryConfigWithSubKind(
		headerv1.FromMetadataProto(msg.GetHeader().GetMetadata()),
		specFromProto(msg.GetSpec()),
		msg.GetHeader().GetSubKind(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	discoveryConfig.Status = StatusFromProto(msg.GetStatus())
	return discoveryConfig, nil
}

// SanitizeSyntheticDiscoveryConfig returns a clone with unsupported installer
// parameters removed according to the synthetic DiscoveryConfig contract.
func SanitizeSyntheticDiscoveryConfig(msg *discoveryconfigv1.DiscoveryConfig) *discoveryconfigv1.DiscoveryConfig {
	if msg == nil {
		return nil
	}
	cloned := proto.CloneOf(msg)
	matchers := cloned.GetStatus().GetSynthetic().GetMatchers()
	for _, matcher := range matchers.GetAws() {
		matcher.Params = discoveryconfig.SanitizeInstallerParams(matcher.Params)
	}
	for _, matcher := range matchers.GetAzure() {
		matcher.Params = discoveryconfig.SanitizeInstallerParams(matcher.Params)
	}
	for _, matcher := range matchers.GetGcp() {
		matcher.Params = discoveryconfig.SanitizeInstallerParams(matcher.Params)
	}
	return cloned
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
		Synthetic:                      syntheticStatusFromProto(msg.GetSynthetic()),
	}
}

func syntheticStatusFromProto(msg *discoveryconfigv1.SyntheticDiscoveryConfigStatus) *discoveryconfig.SyntheticStatus {
	if msg == nil {
		return nil
	}
	var matchers *discoveryconfig.Spec
	if msg.GetMatchers() != nil {
		s := specFromProto(msg.GetMatchers())
		matchers = &s
	}
	var counts *discoveryconfig.StaticMatcherCounts
	if c := msg.GetMatcherCounts(); c != nil {
		counts = &discoveryconfig.StaticMatcherCounts{AWS: c.GetAws(), Azure: c.GetAzure(), GCP: c.GetGcp(), Kube: c.GetKube(), AccessGraph: c.GetAccessGraph()}
	}
	return &discoveryconfig.SyntheticStatus{DiscoveryGroup: msg.GetDiscoveryGroup(), Matchers: matchers, MatchersTruncated: msg.GetMatchersTruncated(), MatcherCounts: counts}
}

func specFromProto(msg *discoveryconfigv1.DiscoveryConfigSpec) discoveryconfig.Spec {
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
		Synthetic:                      SyntheticStatusToProto(status.Synthetic),
	}
}

// SyntheticStatusToProto converts an internal synthetic inventory status into
// its v1 message.
func SyntheticStatusToProto(s *discoveryconfig.SyntheticStatus) *discoveryconfigv1.SyntheticDiscoveryConfigStatus {
	if s == nil {
		return nil
	}
	var counts *discoveryconfigv1.StaticMatcherCounts
	if s.MatcherCounts != nil {
		counts = &discoveryconfigv1.StaticMatcherCounts{Aws: s.MatcherCounts.AWS, Azure: s.MatcherCounts.Azure, Gcp: s.MatcherCounts.GCP, Kube: s.MatcherCounts.Kube, AccessGraph: s.MatcherCounts.AccessGraph}
	}
	var matchers *discoveryconfigv1.DiscoveryConfigSpec
	if s.Matchers != nil {
		matchers = specToProto(*s.Matchers)
	}
	return &discoveryconfigv1.SyntheticDiscoveryConfigStatus{DiscoveryGroup: s.DiscoveryGroup, Matchers: matchers, MatchersTruncated: s.MatchersTruncated, MatcherCounts: counts}
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
