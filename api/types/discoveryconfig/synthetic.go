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
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/utils"
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
// The publication contract reserves writes for the owning Discovery Service
// (see SyntheticName) and uses periodic upserts with a short TTL so snapshots
// disappear shortly after the publishing service stops.
const SubKindSynthetic = "synthetic"

const (
	// SyntheticDiscoveryConfigTTL is how long the publication contract keeps a
	// synthetic DiscoveryConfig without an owner renewal.
	SyntheticDiscoveryConfigTTL = 10 * time.Minute
	// MaxSyntheticDiscoveryConfigSize is the publication contract's maximum
	// serialized stored synthetic DiscoveryConfig size.
	MaxSyntheticDiscoveryConfigSize = 256 * 1024
	// SyntheticMatcherDetailBudget is the serialized matcher detail budget.
	SyntheticMatcherDetailBudget = 64 * 1024
)

const (
	syntheticNamePrefix       = "synthetic-"
	syntheticHashedNamePrefix = "synthetic-hashed-"
)

var syntheticNameNamespace = uuid.NewSHA1(uuid.NameSpaceOID, []byte("teleport.discovery-config.synthetic"))

// SyntheticName returns the name of the synthetic DiscoveryConfig published by
// the Discovery Service running on the host identified by serverID. The name
// preserves canonical UUID server IDs and deterministically maps historical
// non-UUID IDs into the UUID-shaped reserved namespace. This lets Auth derive
// and authorize every owner's name without reserving legacy human-readable
// names such as "synthetic-aws-prod".
func SyntheticName(serverID string) string {
	id, err := uuid.Parse(serverID)
	if err == nil && id.String() == serverID {
		return syntheticNamePrefix + serverID
	}
	return syntheticHashedNamePrefix + uuid.NewSHA1(syntheticNameNamespace, []byte(serverID)).String()
}

// IsReservedSyntheticName reports whether name is in the canonical synthetic
// namespace reserved for UUID-based Discovery Service IDs. Prefix-matching
// alone is intentionally insufficient: regular DiscoveryConfigs using names
// such as "synthetic-aws-prod" predate the synthetic publication contract and
// must remain recreatable.
func IsReservedSyntheticName(name string) bool {
	if serverID, ok := strings.CutPrefix(name, syntheticHashedNamePrefix); ok {
		return isCanonicalUUID(serverID)
	}
	serverID, ok := strings.CutPrefix(name, syntheticNamePrefix)
	return ok && isCanonicalUUID(serverID)
}

func isCanonicalUUID(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id.String() == value
}

// IsSynthetic returns true if the discovery config is a synthetic resource
// self-published by a Discovery Service. See SubKindSynthetic.
//
// Synthetic configs mirror a service's static file configuration. They live in
// an isolated backend range and are absent from generic inventory listings.
func (m *DiscoveryConfig) IsSynthetic() bool {
	return m.SubKind == SubKindSynthetic
}

// ConfiguredDiscoveryGroup returns the Discovery Group configured on the
// owning Discovery Service. Synthetic configs keep the observed group in
// status because their spec is intentionally empty.
//
// TODO(carlisia): Use this in the deferred Discovery Service publisher.
func (m *DiscoveryConfig) ConfiguredDiscoveryGroup() string {
	if !m.IsSynthetic() {
		return m.GetDiscoveryGroup()
	}
	if m.Status.Synthetic != nil {
		return m.Status.Synthetic.DiscoveryGroup
	}
	return ""
}

// NewSyntheticDiscoveryConfig creates the synthetic DiscoveryConfig describing
// the static (file-based) configuration of the Discovery Service running on
// the host identified by serverID.
//
// The constructor owns the resource-envelope invariants: canonical name
// derivation, the synthetic subkind, the config-file origin, and an empty spec
// so generic DiscoveryConfig consumers cannot interpret the snapshot as
// dynamic configuration. The supplied status is copied so the caller cannot
// mutate the resource afterward.
//
// The status must already satisfy the publication contract (see
// CheckSyntheticDiscoveryConfig); turning static matcher configuration into a
// sanitized inventory status is the publisher's job.
func NewSyntheticDiscoveryConfig(serverID string, status SyntheticStatus) (*DiscoveryConfig, error) {
	if serverID == "" {
		return nil, trace.BadParameter("server ID is required")
	}
	var cloned SyntheticStatus
	if err := utils.StrictObjectToStruct(&status, &cloned); err != nil {
		return nil, trace.Wrap(err)
	}
	dc, err := NewDiscoveryConfigWithSubKind(header.Metadata{
		Name: SyntheticName(serverID), Labels: map[string]string{types.OriginLabel: types.OriginConfigFile},
	}, Spec{}, SubKindSynthetic)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	dc.Status.Synthetic = &cloned
	if err := CheckSyntheticDiscoveryConfig(dc, serverID); err != nil {
		return nil, trace.Wrap(err)
	}
	return dc, nil
}

// CheckSyntheticDiscoveryConfig verifies the invariants that must hold for a
// synthetic DiscoveryConfig published by the Discovery Service running on the
// host identified by serverID. The owner-only publication path will enforce it
// on writes and the publisher can use it as a sanity check.
func CheckSyntheticDiscoveryConfig(dc *DiscoveryConfig, serverID string) error {
	if serverID == "" {
		return trace.BadParameter("server ID is required")
	}
	if !dc.IsSynthetic() {
		return trace.BadParameter("discovery config %q is not synthetic", dc.GetName())
	}
	if expected := SyntheticName(serverID); dc.GetName() != expected {
		return trace.BadParameter("synthetic discovery config published by server %q must be named %q, got %q", serverID, expected, dc.GetName())
	}
	if dc.Spec.DiscoveryGroup != "" || len(dc.Spec.AWS) != 0 || len(dc.Spec.Azure) != 0 || len(dc.Spec.GCP) != 0 || len(dc.Spec.Kube) != 0 || dc.Spec.AccessGraph != nil {
		return trace.BadParameter("synthetic discovery config %q must have an empty spec", dc.GetName())
	}
	if dc.Origin() != types.OriginConfigFile {
		return trace.BadParameter("synthetic discovery config %q must have origin %q", dc.GetName(), types.OriginConfigFile)
	}
	if dc.Status.Synthetic == nil {
		return trace.BadParameter("synthetic inventory is required")
	}
	s := dc.Status.Synthetic
	completeForm := s.Matchers != nil && s.MatcherCounts == nil && !s.MatchersTruncated
	truncatedForm := s.Matchers == nil && s.MatcherCounts != nil && s.MatchersTruncated
	if !completeForm && !truncatedForm {
		return trace.BadParameter("synthetic matcher representation must contain either complete matchers or truncated counts")
	}
	if s.Matchers != nil {
		if s.Matchers.DiscoveryGroup != "" {
			return trace.BadParameter("synthetic matcher inventory must not contain a discovery group")
		}
		bad := false
		s.Matchers.eachInstallerParams(func(p **types.InstallerParams) { bad = bad || *p != nil })
		if bad {
			return trace.BadParameter("synthetic matchers must not contain installer params")
		}
	}
	return nil
}

// SanitizeInstallerParams removes the entire installer-parameter subtree.
// Installer parameters are not selectors and may contain join token names,
// installer settings, or proxy credentials.
func SanitizeInstallerParams(*types.InstallerParams) *types.InstallerParams {
	return nil
}

// SanitizeSyntheticDiscoveryConfigSpec removes secret-bearing installer
// settings that are not part of a synthetic DiscoveryConfig's matcher
// inventory contract.
func SanitizeSyntheticDiscoveryConfigSpec(spec *Spec) {
	if spec == nil {
		return
	}
	spec.eachInstallerParams(func(p **types.InstallerParams) {
		*p = SanitizeInstallerParams(*p)
	})
}

// eachInstallerParams calls fn with each matcher installer params slot in the
// spec. New matcher families with installer params must be added here (and to
// the proto-side loop in convert/v1.SanitizeSyntheticDiscoveryConfig) so
// synthetic sanitization and its checks stay in sync.
// TestEachInstallerParamsCoversAllFamilies and
// TestSanitizeSyntheticDiscoveryConfigCoversAllFamilies enforce that contract
// by reflection.
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
