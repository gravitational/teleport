/*
Copyright 2021 Gravitational, Inc.

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

// Package proto provides the protobuf API specification for Teleport.
package proto

import (
	"time"

	"github.com/gravitational/trace"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
)

// Duration is a wrapper around duration
type Duration time.Duration

// Get returns time.Duration value
func (d Duration) Get() time.Duration {
	return time.Duration(d)
}

// Set sets time.Duration value
func (d *Duration) Set(value time.Duration) {
	*d = Duration(value)
}

// CheckAndSetDefaults checks and sets default values
func (req *HostCertsRequest) CheckAndSetDefaults() error {
	if req.HostID == "" {
		return trace.BadParameter("missing parameter HostID")
	}

	return req.Role.Check()
}

// CheckAndSetDefaults checks and sets default values.
func (req *ListResourcesRequest) CheckAndSetDefaults() error {
	if req.Namespace == "" {
		req.Namespace = apidefaults.Namespace
	}
	// If the Limit parameter was not provided instead of returning an error fallback to the default limit.
	if req.Limit == 0 {
		req.Limit = apidefaults.DefaultChunkSize
	}

	if req.Limit < 0 {
		return trace.BadParameter("negative parameter limit")
	}

	return nil
}

// CheckAndSetDefaults checks and sets default values.
func (req *ListUnifiedResourcesRequest) CheckAndSetDefaults() error {
	// If the Limit parameter was not provided instead of returning an error fallback to the default limit.
	if req.Limit == 0 {
		req.Limit = apidefaults.DefaultChunkSize
	}

	if req.Limit < 0 {
		return trace.BadParameter("negative parameter: limit")
	}

	return nil
}

// RequiresFakePagination checks if we need to fallback to GetXXX calls
// that retrieves entire resources upfront rather than working with subsets.
func (req *ListResourcesRequest) RequiresFakePagination() bool {
	return req.SortBy.Field != "" ||
		req.NeedTotalCount ||
		req.ResourceType == types.KindKubernetesCluster ||
		// KindSAMLIdPServiceProvider supports paginated List, but it is not
		// available in the Presence service, hence defined here under
		// RequiresFakePagination.
		req.ResourceType == types.KindSAMLIdPServiceProvider
}

// UpstreamInventoryMessage is a sealed interface representing the possible
// upstream messages of the inventory control stream after the initial hello.
type UpstreamInventoryMessage interface {
	sealedUpstreamInventoryMessage()
}

func (h *UpstreamInventoryHello) sealedUpstreamInventoryMessage() {}

func (h *InventoryHeartbeat) sealedUpstreamInventoryMessage() {}

func (p *UpstreamInventoryPong) sealedUpstreamInventoryMessage() {}

func (a *UpstreamInventoryAgentMetadata) sealedUpstreamInventoryMessage() {}

func (h *UpstreamInventoryGoodbye) sealedUpstreamInventoryMessage() {}

// DownstreamInventoryMessage is a sealed interface representing the possible
// downstream messages of the inventory controls stream after initial hello.
type DownstreamInventoryMessage interface {
	sealedDownstreamInventoryMessage()
}

func (h *DownstreamInventoryHello) sealedDownstreamInventoryMessage() {}

func (p *DownstreamInventoryPing) sealedDownstreamInventoryMessage() {}

func (u *DownstreamInventoryUpdateLabels) sealedDownstreamInventoryMessage() {}

// AllowsMFAReuse returns true if the MFA response provided allows reuse.
func (r *UserCertsRequest) AllowsMFAReuse() bool {
	return r.RequesterName == UserCertsRequest_TSH_DB_EXEC
}
