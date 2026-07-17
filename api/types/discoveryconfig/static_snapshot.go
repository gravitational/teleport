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
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/utils"
)

// SubKindStaticSnapshot marks a DiscoveryConfig that was self-published by a
// Discovery Service to expose its static (file-based) matcher configuration to
// the backend. The snapshot reuses the regular DiscoveryConfig schema: the
// observed discovery group and sanitized matchers live in the spec.
//
// Static snapshots are informational only and must never be loaded back as
// dynamic matchers by Discovery Services: the publishing service already runs
// the same matchers from its file configuration, so consuming the snapshot
// would duplicate them. Snapshots therefore live in an isolated backend range
// that generates no DiscoveryConfig watch events and is absent from generic
// listings; the only read path is a named lookup.
//
// The publication contract reserves writes for the owning Discovery Service
// (see StaticSnapshotName) and uses periodic upserts with a short TTL so
// snapshots disappear shortly after the publishing service stops.
const SubKindStaticSnapshot = "static-snapshot"

const (
	// StaticSnapshotTTL is how long the publication contract keeps a static
	// snapshot without an owner renewal.
	StaticSnapshotTTL = 10 * time.Minute
	// MaxStaticSnapshotSize is the publication contract's maximum serialized
	// stored static snapshot size, covering the spec inventory and the
	// reported status together. 256KiB is the largest power of two that
	// stays comfortably under the tightest backend item limit (DynamoDB's
	// 400KB), leaving the remainder for the backend key, lease metadata,
	// and value-encoding overhead.
	MaxStaticSnapshotSize = 256 * 1024
)

const (
	staticSnapshotNamePrefix       = "static-snapshot-"
	staticSnapshotHashedNamePrefix = "static-snapshot-hashed-"
)

var staticSnapshotNameNamespace = uuid.NewSHA1(uuid.NameSpaceOID, []byte("teleport.discovery-config.static-snapshot"))

// StaticSnapshotName returns the name of the static snapshot published by the
// Discovery Service running on the host identified by serverID. The name
// preserves canonical UUID server IDs and deterministically maps historical
// non-UUID IDs into the UUID-shaped reserved namespace. This lets Auth derive
// and authorize every owner's name without reserving legacy human-readable
// names such as "static-snapshot-aws-prod".
func StaticSnapshotName(serverID string) string {
	if isCanonicalUUID(serverID) {
		return staticSnapshotNamePrefix + serverID
	}
	return staticSnapshotHashedNamePrefix + uuid.NewSHA1(staticSnapshotNameNamespace, []byte(serverID)).String()
}

// IsReservedStaticSnapshotName reports whether name is in the canonical
// static snapshot namespace reserved for UUID-based Discovery Service IDs.
// Prefix-matching alone is intentionally insufficient: a regular
// DiscoveryConfig named "static-snapshot-aws-prod" predating the reservation
// must remain recreatable.
func IsReservedStaticSnapshotName(name string) bool {
	if serverID, ok := strings.CutPrefix(name, staticSnapshotHashedNamePrefix); ok {
		return isCanonicalUUID(serverID)
	}
	serverID, ok := strings.CutPrefix(name, staticSnapshotNamePrefix)
	return ok && isCanonicalUUID(serverID)
}

func isCanonicalUUID(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id.String() == value
}

// IsStaticSnapshot returns true if the discovery config is a static snapshot
// self-published by a Discovery Service. See SubKindStaticSnapshot.
func (m *DiscoveryConfig) IsStaticSnapshot() bool {
	return m.SubKind == SubKindStaticSnapshot
}

// NewStaticSnapshotDiscoveryConfig creates the static snapshot DiscoveryConfig
// describing the file-based configuration of the Discovery Service running on
// the host identified by serverID.
//
// The constructor owns the resource-envelope invariants: canonical name
// derivation, the static-snapshot subkind, and the config-file origin. The
// supplied spec is the observed inventory; it is copied during construction so
// the caller cannot mutate the resource afterward.
//
// Publication is fail-closed: a spec still carrying installer params is
// rejected rather than silently stripped, because sanitizing observed
// configuration is the publisher's job (see SanitizeStaticSnapshotSpec).
func NewStaticSnapshotDiscoveryConfig(serverID string, spec Spec) (*DiscoveryConfig, error) {
	if serverID == "" {
		return nil, trace.BadParameter("server ID is required")
	}
	if err := CheckStaticSnapshotSpecSanitized(&spec); err != nil {
		return nil, trace.Wrap(err)
	}
	dc, err := NewDiscoveryConfigWithSubKind(header.Metadata{
		Name:   StaticSnapshotName(serverID),
		Labels: map[string]string{types.OriginLabel: types.OriginConfigFile},
	}, spec, SubKindStaticSnapshot)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := CheckStaticSnapshotDiscoveryConfig(dc, serverID); err != nil {
		return nil, trace.Wrap(err)
	}
	return dc, nil
}

// CheckStaticSnapshotDiscoveryConfig verifies the invariants that must hold
// for a static snapshot published by the Discovery Service running on the host
// identified by serverID. The owner-only publication path enforces it on
// writes and the publisher can use it as a sanity check.
//
// The discovery group is intentionally unconstrained: it mirrors whatever the
// owning service has configured, including no group at all.
func CheckStaticSnapshotDiscoveryConfig(dc *DiscoveryConfig, serverID string) error {
	if dc == nil {
		return trace.BadParameter("discovery config is required")
	}
	if serverID == "" {
		return trace.BadParameter("server ID is required")
	}
	if !dc.IsStaticSnapshot() {
		return trace.BadParameter("discovery config %q is not a static snapshot", dc.GetName())
	}
	if expected := StaticSnapshotName(serverID); dc.GetName() != expected {
		return trace.BadParameter("static snapshot published by server %q must be named %q, got %q", serverID, expected, dc.GetName())
	}
	if dc.Origin() != types.OriginConfigFile {
		return trace.BadParameter("static snapshot %q must have origin %q", dc.GetName(), types.OriginConfigFile)
	}
	if err := CheckStaticSnapshotSpecSanitized(&dc.Spec); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// SanitizeStaticSnapshotSpec removes secret-bearing installer settings that
// are not part of a static snapshot's matcher inventory contract.
func SanitizeStaticSnapshotSpec(spec *Spec) {
	if spec == nil {
		return
	}
	spec.eachInstallerParams(func(p **types.InstallerParams) {
		*p = nil
	})
}

// CheckStaticSnapshotSpecSanitized verifies the spec carries no installer
// params, the fail-closed invariant every static snapshot write path
// enforces instead of silently stripping observed configuration.
func CheckStaticSnapshotSpecSanitized(spec *Spec) error {
	var unsanitized bool
	spec.eachInstallerParams(func(p **types.InstallerParams) { unsanitized = unsanitized || *p != nil })
	if unsanitized {
		return trace.BadParameter("static snapshot matchers must not contain installer params")
	}
	return nil
}

// validateStaticSnapshotSpec validates snapshot inventory against a throwaway
// deep copy of the whole spec, so no value derived by matcher defaulting
// (installer params, SSM document names, wildcard scoping such as Azure
// regions or GCP locations) reaches the stored record. A snapshot records
// the publisher's configuration; values derived on the Auth side can differ
// from what the publishing service derived for itself (defaulting an absent
// installer document from a sanitized, params-less matcher picks the
// agentless document where the service's own defaulting picks the agent one)
// and must never be persisted as if published.
//
// The temporary SCRIPT enroll mode prevents ordinary EC2 defaulting from
// deriving the removed EICE enrollment mode for an integration-based matcher
// whose installer params were removed by snapshot sanitization. Params
// supplied by the caller are validated as they are: the isolated snapshot
// persistence path, not a caller-controlled subkind, is the sanitization
// trust boundary.
func validateStaticSnapshotSpec(spec *Spec) error {
	var scratch Spec
	if err := utils.StrictObjectToStruct(spec, &scratch); err != nil {
		return trace.Wrap(err)
	}
	for i := range scratch.AWS {
		matcher := &scratch.AWS[i]
		if matcher.Params != nil || !slices.Contains(matcher.Types, types.AWSMatcherEC2) {
			continue
		}
		// Mirror ordinary file-config defaulting (InstallTeleport: true) so
		// the copy is validated in the shape the publishing service actually
		// runs; only the enroll mode is pinned to SCRIPT, to avoid deriving
		// and validating the removed EICE mode.
		matcher.Params = &types.InstallerParams{
			InstallTeleport: true,
			EnrollMode:      types.InstallParamEnrollMode_INSTALL_PARAM_ENROLL_MODE_SCRIPT,
		}
	}
	if err := scratch.checkAndSetMatcherDefaults(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// eachInstallerParams calls fn with each matcher installer params slot in the
// spec. New matcher families with installer params must be added here so
// static snapshot sanitization and its checks stay in sync.
// TestEachInstallerParamsCoversAllFamilies enforces that contract by
// reflection.
func (s *Spec) eachInstallerParams(fn func(p **types.InstallerParams)) {
	for i := range s.AWS {
		fn(&s.AWS[i].Params)
	}
	for i := range s.Azure {
		fn(&s.Azure[i].Params)
	}
	for i := range s.GCP {
		fn(&s.GCP[i].Params)
	}
}
