/*
Copyright 2026 Gravitational, Inc.

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
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/header"
)

// SubKindSynthetic marks a DiscoveryConfig that was self-published by a
// Discovery Service to expose its static (file-based) matcher configuration to
// the backend.
//
// Synthetic DiscoveryConfigs are informational only and must never be loaded
// back as dynamic matchers by Discovery Services: the publishing service
// already runs the same matchers from its file configuration, so consuming the
// synthetic copy would duplicate them.
//
// Synthetic DiscoveryConfigs can only be written by the Discovery Service that
// owns them (see SyntheticName) and are kept alive by periodic upserts with a
// short TTL, so they disappear shortly after the publishing service stops.
const SubKindSynthetic = "synthetic"

// syntheticNamePrefix prefixes the host UUID of the publishing Discovery
// Service to form the name of its synthetic DiscoveryConfig.
const syntheticNamePrefix = "synthetic-"

// SyntheticName returns the name of the synthetic DiscoveryConfig published by
// the Discovery Service running on the host identified by serverID. The name
// is derived strictly from the server ID so that the backend can authorize
// writes by comparing the resource name against the caller identity.
func SyntheticName(serverID string) string {
	return syntheticNamePrefix + serverID
}

// IsSynthetic returns true if the discovery config is a synthetic resource
// self-published by a Discovery Service. See SubKindSynthetic.
func (m *DiscoveryConfig) IsSynthetic() bool {
	return m.SubKind == SubKindSynthetic
}

// NewSyntheticDiscoveryConfig creates the synthetic DiscoveryConfig describing
// the static (file-based) configuration of the Discovery Service running on
// the host identified by serverID.
//
// The spec is expected to mirror the file configuration. Its DiscoveryGroup is
// moved to the types.TeleportInternalDiscoveryGroupName label and replaced
// with the resource name: older Discovery Services select dynamic configs
// purely by matching spec.discovery_group against their own group, so the
// sentinel value guarantees they never consume the synthetic copy. Newer
// services skip synthetic configs explicitly by subkind.
func NewSyntheticDiscoveryConfig(serverID string, spec Spec) (*DiscoveryConfig, error) {
	if serverID == "" {
		return nil, trace.BadParameter("server ID is required")
	}

	name := SyntheticName(serverID)
	labels := map[string]string{
		types.OriginLabel: types.OriginConfigFile,
	}
	if spec.DiscoveryGroup != "" {
		labels[types.TeleportInternalDiscoveryGroupName] = spec.DiscoveryGroup
	}
	spec.DiscoveryGroup = name

	dc, err := NewDiscoveryConfig(header.Metadata{
		Name:   name,
		Labels: labels,
	}, spec)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	dc.SetSubKind(SubKindSynthetic)
	return dc, nil
}

// CheckSyntheticDiscoveryConfig verifies the invariants that must hold for a
// synthetic DiscoveryConfig published by the Discovery Service running on the
// host identified by serverID. It is enforced by the backend on writes and
// used by the publisher as a sanity check.
func CheckSyntheticDiscoveryConfig(dc *DiscoveryConfig, serverID string) error {
	if !dc.IsSynthetic() {
		return trace.BadParameter("discovery config %q is not synthetic", dc.GetName())
	}
	if expected := SyntheticName(serverID); dc.GetName() != expected {
		return trace.BadParameter("synthetic discovery config published by server %q must be named %q, got %q", serverID, expected, dc.GetName())
	}
	// The sentinel discovery group prevents older Discovery Services, which
	// select dynamic configs solely by group, from consuming synthetic configs.
	if dc.Spec.DiscoveryGroup != dc.GetName() {
		return trace.BadParameter("synthetic discovery config %q must use its own name as the discovery group, got %q", dc.GetName(), dc.Spec.DiscoveryGroup)
	}
	// Synthetic configs are kept alive by periodic upserts; requiring an
	// expiry guarantees stale copies vanish after the publisher stops.
	if dc.Expiry().IsZero() {
		return trace.BadParameter("synthetic discovery config %q must have an expiry", dc.GetName())
	}
	return nil
}
