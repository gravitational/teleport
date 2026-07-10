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

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	discoveryservicev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryservice/v1"
	apitypes "github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/common"
)

// DiscoveryServices defines an interface for managing discovery_service
// resources: the configuration heartbeats of Discovery Service instances.
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

// ValidateDiscoveryService will check the given discovery service heartbeat
// for validity. Should be called before writing a new value in the cluster
// state storage and before using a value. The value will not be modified.
func ValidateDiscoveryService(resource *discoveryservicev1.DiscoveryService) error {
	if expected, actual := apitypes.KindDiscoveryService, resource.GetKind(); expected != actual {
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

// MarshalDiscoveryService marshals a discovery_service resource to JSON.
//
// It uses encoding/json rather than [MarshalProtoResource]/protojson because
// the spec embeds legacy gogoproto matcher types whose customtype fields
// (e.g. types.Labels tags) protojson silently drops; the hybrid-API generated
// struct carries proper json tags, so encoding/json round-trips faithfully.
func MarshalDiscoveryService(resource *discoveryservicev1.DiscoveryService, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !cfg.PreserveRevision {
		resource = proto.CloneOf(resource)
		resource.GetMetadata().SetRevision("")
	}
	data, err := json.Marshal(resource)
	if err != nil {
		return nil, trace.Wrap(err)
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
	if cfg.Revision != "" {
		resource.GetMetadata().SetRevision(cfg.Revision)
	}
	if !cfg.Expires.IsZero() {
		resource.GetMetadata().SetExpires(timestamppb.New(cfg.Expires))
	}
	return &resource, nil
}
