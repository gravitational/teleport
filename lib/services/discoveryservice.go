// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package services

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	discoveryservicev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryservice/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	apitypes "github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/common"
)

// validateHostHeartbeatEnvelope checks the resource envelope shared by host
// heartbeat resources ([ValidateDiscoveryService], [ValidateRelayServer]):
// exact kind, empty sub_kind, version v1, a nonempty name, and well-formed
// labels without an origin.
func validateHostHeartbeatEnvelope(resource interface {
	GetKind() string
	GetSubKind() string
	GetVersion() string
	GetMetadata() *headerv1.Metadata
}, kind string) error {
	if expected, actual := kind, resource.GetKind(); expected != actual {
		return trace.BadParameter("expected kind %v, got %q", expected, actual)
	}
	if expected, actual := "", resource.GetSubKind(); expected != actual {
		return trace.BadParameter("expected sub_kind %v, got %q", expected, actual)
	}
	if expected, actual := apitypes.V1, resource.GetVersion(); expected != actual {
		return trace.BadParameter("expected version %v, got %q", expected, actual)
	}
	if name := resource.GetMetadata().GetName(); name == "" {
		return trace.BadParameter("missing name")
	}
	for key := range resource.GetMetadata().GetLabels() {
		if key == apitypes.OriginLabel {
			return trace.BadParameter("origin label unsupported")
		}
		if !common.IsValidLabelKey(key) {
			return trace.BadParameter("invalid label key %q", key)
		}
	}
	return nil
}

// DiscoveryServices defines an interface for managing discovery_service resources:
// the self-reported heartbeats of Discovery Service instances.
// One resource exists per instance, named by its host ID, carrying only the instance's
// static configuration. Dynamic DiscoveryConfig contents are never included.
type DiscoveryServices interface {
	// GetDiscoveryService returns the discovery service heartbeat with a given name.
	GetDiscoveryService(ctx context.Context, name string) (*discoveryservicev1.DiscoveryService, error)
	// ListDiscoveryServices returns a paginated list of discovery service heartbeats.
	ListDiscoveryServices(ctx context.Context, pageSize int, pageToken string) (_ []*discoveryservicev1.DiscoveryService, nextPageToken string, _ error)
	// UpsertDiscoveryService upserts a discovery service heartbeat.
	UpsertDiscoveryService(ctx context.Context, svc *discoveryservicev1.DiscoveryService) (*discoveryservicev1.DiscoveryService, error)
	// DeleteDiscoveryService removes the discovery service heartbeat with a given name.
	DeleteDiscoveryService(ctx context.Context, name string) error
}

func hasInstallerParams[T any](matchers []*T, getParams func(*T) *apitypes.InstallerParams) bool {
	for _, matcher := range matchers {
		if matcher != nil && getParams(matcher) != nil {
			return true
		}
	}
	return false
}

// MaxDiscoveryServiceRecordBytes is the maximum encoded backend resource size, measured on
// the complete Auth-stored JSON record.
const MaxDiscoveryServiceRecordBytes = 256 * 1024

// StaticMatcherCountKey* are the allowed keys of the discovery_service resource's
// spec.static_matcher_counts map. The producer, [ValidateDiscoveryService], and display
// code must agree on this vocabulary; proto3 map keys cannot enforce a closed set, so
// these constants are the single source of truth. A producer that emits any other key
// has its whole heartbeat rejected at admission.
const (
	// StaticMatcherCountKeyAWS counts effective AWS matchers.
	StaticMatcherCountKeyAWS = "aws"
	// StaticMatcherCountKeyAzure counts effective Azure matchers.
	StaticMatcherCountKeyAzure = "azure"
	// StaticMatcherCountKeyGCP counts effective GCP matchers.
	StaticMatcherCountKeyGCP = "gcp"
	// StaticMatcherCountKeyKube counts effective Kubernetes matchers.
	StaticMatcherCountKeyKube = "kube"
	// StaticMatcherCountKeyAccessGraph counts effective Access Graph sync
	// matchers: the sum of the AWS and Azure sync entries.
	StaticMatcherCountKeyAccessGraph = "access_graph"
)

// staticMatcherCountKeys enumerates every StaticMatcherCountKey* constant in a
// slice so the closed set can be iterated: [ValidateDiscoveryService] rejects
// keys outside it, and the tctl summary tests iterate the clone returned by
// [StaticMatcherCountKeyList] so every key keeps a matching display branch.
// The constants above remain the source of truth; append here whenever a new
// key constant is added.
var staticMatcherCountKeys = []string{
	StaticMatcherCountKeyAWS,
	StaticMatcherCountKeyAzure,
	StaticMatcherCountKeyGCP,
	StaticMatcherCountKeyKube,
	StaticMatcherCountKeyAccessGraph,
}

// StaticMatcherCountKeyList returns a copy of staticMatcherCountKeys for
// enumeration outside this package, such as by the tctl summary tests;
// mutating the returned slice does not affect validation.
func StaticMatcherCountKeyList() []string {
	return slices.Clone(staticMatcherCountKeys)
}

// ValidateDiscoveryService will check the given discovery service heartbeat
// for validity. Should be called before writing a new value in the cluster
// state storage and before using a value. The value will not be modified.
func ValidateDiscoveryService(resource *discoveryservicev1.DiscoveryService) error {
	if resource == nil {
		return trace.BadParameter("missing discovery service")
	}
	if resource.GetMetadata() == nil {
		return trace.BadParameter("missing metadata")
	}
	if err := validateHostHeartbeatEnvelope(resource, apitypes.KindDiscoveryService); err != nil {
		return trace.Wrap(err)
	}
	if resource.GetSpec() == nil {
		return trace.BadParameter("missing spec")
	}
	if resource.GetSpec().GetMatchersTruncated() {
		if resource.GetSpec().GetStaticMatchers() != nil {
			return trace.BadParameter("static_matchers must be absent when matchers_truncated is true")
		}
		for cloud, count := range resource.GetSpec().GetStaticMatcherCounts() {
			if !slices.Contains(staticMatcherCountKeys, cloud) {
				return trace.BadParameter("unknown static matcher count key %q", cloud)
			}
			if count < 0 {
				return trace.BadParameter("static matcher count for %q must be nonnegative", cloud)
			}
		}
	} else if len(resource.GetSpec().GetStaticMatcherCounts()) != 0 {
		return trace.BadParameter("static_matcher_counts must be absent when matchers_truncated is false")
	}
	if matchers := resource.GetSpec().GetStaticMatchers(); matchers != nil {
		if hasInstallerParams(matchers.GetAws(), func(matcher *apitypes.AWSMatcher) *apitypes.InstallerParams { return matcher.Params }) ||
			hasInstallerParams(matchers.GetAzure(), func(matcher *apitypes.AzureMatcher) *apitypes.InstallerParams { return matcher.Params }) ||
			hasInstallerParams(matchers.GetGcp(), func(matcher *apitypes.GCPMatcher) *apitypes.InstallerParams { return matcher.Params }) {
			return trace.BadParameter("installer parameters are unsupported in static_matchers")
		}
	}
	return nil
}

// MarshalDiscoveryService marshals a discovery_service resource to JSON.
//
// It uses encoding/json rather than [MarshalProtoResource]/protojson because
// the spec embeds legacy gogoproto matcher types whose customtype fields
// (e.g. types.Labels tags) protojson silently drops; the hybrid-API generated
// struct carries proper json tags, so encoding/json round-trips faithfully.
// This codec therefore requires the generated open struct representation. A
// future protoopaque migration must first introduce a backward-compatible
// explicit storage codec that preserves the legacy matcher fields.
func MarshalDiscoveryService(resource *discoveryservicev1.DiscoveryService, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if resource == nil {
		return nil, trace.BadParameter("missing discovery service")
	}
	if resource.GetMetadata() == nil {
		return nil, trace.BadParameter("missing metadata")
	}
	if !cfg.PreserveRevision {
		resource = proto.CloneOf(resource)
		resource.GetMetadata().SetRevision("")
	}
	data, err := json.Marshal(resource)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(data) > MaxDiscoveryServiceRecordBytes {
		return nil, trace.BadParameter("discovery service record size %d exceeds limit %d", len(data), MaxDiscoveryServiceRecordBytes)
	}
	return data, nil
}

// UnmarshalDiscoveryService unmarshals a discovery_service resource from
// JSON. See [MarshalDiscoveryService] for why this is encoding/json rather
// than protojson.
func UnmarshalDiscoveryService(data []byte, opts ...MarshalOption) (*discoveryservicev1.DiscoveryService, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("nothing to unmarshal")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var resource discoveryservicev1.DiscoveryService
	if err := json.Unmarshal(data, &resource); err != nil {
		return nil, trace.Wrap(err)
	}
	if resource.GetMetadata() == nil {
		return nil, trace.BadParameter("missing metadata")
	}
	if cfg.Revision != "" {
		resource.GetMetadata().SetRevision(cfg.Revision)
	}
	if !cfg.Expires.IsZero() {
		resource.GetMetadata().SetExpires(timestamppb.New(cfg.Expires))
	}
	return &resource, nil
}
