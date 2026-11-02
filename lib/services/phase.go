/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package services

import (
	"context"
	"iter"
	"regexp"
	"strings"

	"github.com/gravitational/trace"
	"k8s.io/apimachinery/pkg/util/validation"

	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils/set"
)

const (
	// MaxPhaseBuildConfigSize keeps Phase resources small enough to distribute
	// safely through the protobuf and cache watch paths.
	MaxPhaseBuildConfigSize = 32 * 1024
	// MaxPhaseLastErrorSize bounds compute errors persisted in Phase status.
	MaxPhaseLastErrorSize = 4 * 1024
	// MaxPhaseSuppressRebuilds limits the child IDs retained with an operation
	// and its published image.
	MaxPhaseSuppressRebuilds = 100

	// DefaultPhaseName selects the standard Beam image without looking up a
	// stored Phase resource.
	DefaultPhaseName = "default:v1"
)

// PhaseBuildTypeString returns the startup type understood by Beam compute.
// Unknown and unspecified types return an empty string.
func PhaseBuildTypeString(t beamsv1.PhaseBuildType) string {
	switch t {
	case beamsv1.PhaseBuildType_PHASE_BUILD_TYPE_SNAPSHOT:
		return "snapshot"
	case beamsv1.PhaseBuildType_PHASE_BUILD_TYPE_REBUILD:
		return "rebuild"
	case beamsv1.PhaseBuildType_PHASE_BUILD_TYPE_COPY:
		return "copy"
	default:
		return ""
	}
}

// ValidatePhaseBuildRequest checks that the configuration matches the requested
// startup type. The contents of build_config remain opaque to Teleport.
func ValidatePhaseBuildRequest(req *beamsv1.CreatePhaseRequest) error {
	if PhaseBuildTypeString(req.GetBuildType()) == "" {
		return trace.BadParameter("build_type must be snapshot, rebuild, or copy")
	}
	rebuild := req.GetBuildType() == beamsv1.PhaseBuildType_PHASE_BUILD_TYPE_REBUILD
	if rebuild != (req.GetBuildConfig() != "") {
		return trace.BadParameter("build_config is required only for rebuilds")
	}
	if req.GetWorkingDigest() != "" &&
		req.GetBuildType() != beamsv1.PhaseBuildType_PHASE_BUILD_TYPE_SNAPSHOT {
		return trace.BadParameter("working_digest requires a snapshot build")
	}
	if req.GetStreamBuildLogs() && !rebuild {
		return trace.BadParameter("streaming build logs requires a rebuild")
	}
	return validatePhaseSuppressRebuilds(req.GetSuppressRebuilds())
}

func validatePhaseSuppressRebuilds(ids []string) error {
	if len(ids) > MaxPhaseSuppressRebuilds {
		return trace.BadParameter("suppress_rebuilds must contain at most %d Phase IDs", MaxPhaseSuppressRebuilds)
	}
	return nil
}

// ParsePhaseSnapshot splits a snapshot reference into its parent Phase name
// and image digest. The reference identifies the image used as a build input,
// not the Phase that will receive the new image.
func ParsePhaseSnapshot(ref string) (string, string, error) {
	parent, digest, ok := strings.Cut(ref, "@")
	if !ok ||
		parent == "" ||
		!IsValidImageDigest(digest) {
		return "", "", trace.BadParameter("snapshot must be parent@sha256:<64 lowercase hex digits>")
	}
	return parent, digest, nil
}

var phaseRetentionTagPattern = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9._-]{0,127}$`)

var imageDigestRegexp = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// PhaseReader reads Phase resources.
type PhaseReader interface {
	// GetPhase reads a Phase by name.
	GetPhase(context.Context, string) (*beamsv1.Phase, error)
	// ListPhases returns a filtered page and the next pagination token.
	ListPhases(context.Context, int, string, *ListPhasesRequestOptions) ([]*beamsv1.Phase, string, error)
	// IteratePhases yields Phases and pagination errors from the supplied token.
	IteratePhases(context.Context, string) iter.Seq2[*beamsv1.Phase, error]
}

// PhaseWriter writes Phase resources using optimistic concurrency.
type PhaseWriter interface {
	// CreatePhase persists a new Phase and advances the graph revision.
	CreatePhase(context.Context, *beamsv1.Phase) (*beamsv1.Phase, error)
	// GetPhaseGraphRevision must precede reads used to validate a graph change.
	GetPhaseGraphRevision(context.Context) (string, error)
	// CreatePhaseWithGraphRevision requires an absent resource and the expected
	// graph revision, then creates the resource and advances the graph revision.
	CreatePhaseWithGraphRevision(context.Context, *beamsv1.Phase, string) (*beamsv1.Phase, error)
	// UpdatePhaseWithGraphRevision checks and advances both revisions atomically.
	UpdatePhaseWithGraphRevision(context.Context, *beamsv1.Phase, string) (*beamsv1.Phase, error)
	// UpdatePhase checks the resource revision without changing the graph revision.
	UpdatePhase(context.Context, *beamsv1.Phase) (*beamsv1.Phase, error)
	// DeletePhase removes the resource and advances the graph revision.
	DeletePhase(context.Context, string) error
	// DeletePhaseRevision also requires the stored resource revision to match.
	DeletePhaseRevision(context.Context, string, string) error
}

// Phases manages Phase resources.
type Phases interface {
	PhaseReader
	PhaseWriter
}

// ListPhasesRequestOptions configures Phase filtering.
type ListPhasesRequestOptions struct {
	// Users restricts results to these owners. An empty set includes all owners.
	Users map[string]struct{}
	// FilterFn applies an additional filter after owner matching.
	FilterFn func(*beamsv1.Phase) bool
}

// ValidatePhase validates a Phase before persistence.
func ValidatePhase(phase *beamsv1.Phase) error {
	switch {
	case phase == nil:
		return trace.BadParameter("phase must not be nil")
	case phase.GetKind() != types.KindPhase:
		return trace.BadParameter("kind: must be %q", types.KindPhase)
	case phase.GetVersion() != types.V1:
		return trace.BadParameter("version: must be %q", types.V1)
	case !phase.HasMetadata() || phase.GetMetadata().GetName() == "":
		return trace.BadParameter("metadata.name: is required")
	// Backend expiry bypasses guarded deletion and resource cleanup.
	case phase.GetMetadata().HasExpires():
		return trace.BadParameter("metadata.expires: must not be set for a durable Phase")
	case phase.GetMetadata().GetName() == DefaultPhaseName:
		return trace.BadParameter("metadata.name: %q is reserved", DefaultPhaseName)
	case len(phase.GetMetadata().GetName()) > MaxPhaseNameLength:
		return trace.BadParameter("metadata.name: Phase names must be at most %d characters", MaxPhaseNameLength)
	case len(validation.IsDNS1123Label(phase.GetMetadata().GetName())) != 0:
		return trace.BadParameter("metadata.name: must be a valid DNS label")
	case !phase.HasSpec():
		return trace.BadParameter("spec: is required")
	case !phase.HasStatus():
		return trace.BadParameter("status: is required")
	case phase.GetStatus().GetUser() == "":
		return trace.BadParameter("status.user: is required")
	}

	buildConfig := phase.GetSpec().GetBuildConfig()
	if len(buildConfig) > MaxPhaseBuildConfigSize {
		return trace.BadParameter("spec.build_config: exceeds maximum size of %d bytes", MaxPhaseBuildConfigSize)
	}
	upstream := phase.GetSpec().GetParent()
	if upstream == "" {
		return trace.BadParameter("spec.parent: is required")
	}
	if upstream != DefaultPhaseName {
		switch {
		case upstream == phase.GetMetadata().GetName():
			return trace.BadParameter("spec.parent: must not reference itself")
		case len(validation.IsDNS1123Label(upstream)) != 0:
			return trace.BadParameter("spec.parent: must be a valid DNS label")
		}
	}
	switch phase.GetStatus().GetStrategy() {
	case beamsv1.PhaseStrategy_PHASE_STRATEGY_STATIC:
		if buildConfig != "" {
			return trace.BadParameter("spec.build_config: must be empty for static strategy")
		}
	case beamsv1.PhaseStrategy_PHASE_STRATEGY_REBUILD:
		if buildConfig == "" {
			return trace.BadParameter("spec.build_config: is required for rebuild strategy")
		}
	case beamsv1.PhaseStrategy_PHASE_STRATEGY_SNAPSHOT, beamsv1.PhaseStrategy_PHASE_STRATEGY_COPY:
		if buildConfig != "" {
			return trace.BadParameter("spec.build_config: must be empty for snapshot or copy strategy")
		}
	default:
		return trace.BadParameter("status.strategy: must be static, rebuild, snapshot, or copy")
	}

	status := phase.GetStatus()
	if err := validatePhasePending(status, buildConfig); err != nil {
		return trace.Wrap(err)
	}
	if err := validatePhaseOperation(status); err != nil {
		return trace.Wrap(err)
	}
	if err := validatePhasePublished(status.GetPublished()); err != nil {
		return trace.Wrap(err)
	}
	if err := validatePhaseFailedBuilds(status.GetFailedBuilds()); err != nil {
		return trace.Wrap(err)
	}
	if len(status.GetRetiredSnapshots()) > 64 {
		return trace.BadParameter("too many retired snapshot tags")
	}
	seenTags := set.New[string]()
	for _, tag := range append([]*beamsv1.PhaseSnapshotTag{
		beamsv1.PhaseSnapshotTag_builder{Tag: status.GetPublished().GetGeneration()}.Build(),
		beamsv1.PhaseSnapshotTag_builder{Tag: status.GetPending().GetGeneration()}.Build(),
	}, status.GetRetiredSnapshots()...) {
		if tag.GetTag() != "" {
			if !phaseRetentionTagPattern.MatchString(tag.GetTag()) {
				return trace.BadParameter("invalid snapshot retention tag")
			}
			if seenTags.Contains(tag.GetTag()) {
				return trace.BadParameter("duplicate snapshot retention tag")
			}
			seenTags.Add(tag.GetTag())
		}
		if deadline := tag.GetDeleteAfter(); deadline != nil && !deadline.IsValid() {
			return trace.BadParameter("invalid snapshot retention deadline")
		}
	}
	if len(status.GetLastError()) > MaxPhaseLastErrorSize {
		return trace.BadParameter("status.last_error: exceeds maximum size of %d bytes", MaxPhaseLastErrorSize)
	}
	if timestamp := status.GetNextBuildAt(); timestamp != nil && !timestamp.IsValid() {
		return trace.BadParameter("status.next_build_at: is invalid")
	}
	if generation := status.GetLastAttemptedUpstreamGeneration(); generation != "" &&
		!phaseRetentionTagPattern.MatchString(generation) {
		return trace.BadParameter("status.last_attempted_upstream_generation: is invalid")
	}
	return nil
}

func validatePhasePending(status *beamsv1.PhaseStatus, buildConfig string) error {
	pending := status.GetPending()
	if pending.GetDigest() != "" && !IsValidImageDigest(pending.GetDigest()) {
		return trace.BadParameter("status.pending.digest: invalid digest")
	}
	if (pending.GetDigest() == "") != (pending.GetGeneration() == "") {
		return trace.BadParameter("status.pending.digest and status.pending.generation must be set together")
	}
	if pending.GetCopy() {
		if !pending.GetRebase() ||
			pending.GetDigest() == "" ||
			buildConfig != "" {
			return trace.BadParameter("status.pending.copy: requires a rebase-safe digest and commandless Phase")
		}
	}
	if status.GetStrategy() == beamsv1.PhaseStrategy_PHASE_STRATEGY_COPY &&
		pending.GetDigest() != "" &&
		!pending.GetCopy() {
		return trace.BadParameter("copy Phases require copy snapshots")
	}
	if pending.GetManual() {
		if buildConfig != "" ||
			pending.GetDigest() == "" ||
			status.GetState() != beamsv1.PhaseBuildState_PHASE_BUILD_STATE_BUILDING {
			return trace.BadParameter("manual publication requires a pending digest and commandless building Phase")
		}
	}
	if generation := pending.GetUpstreamGeneration(); generation != "" &&
		!phaseRetentionTagPattern.MatchString(generation) {
		return trace.BadParameter("status.pending.upstream_generation: is invalid")
	}
	return nil
}

func validatePhaseOperation(status *beamsv1.PhaseStatus) error {
	op := status.GetOperation()
	if err := validatePhaseSuppressRebuilds(op.GetSuppressRebuilds()); err != nil {
		return trace.Wrap(err, "status.operation")
	}
	if deadline := op.GetProvisionExpires(); deadline != nil {
		if !deadline.IsValid() ||
			op.GetId() == "" ||
			status.GetState() != beamsv1.PhaseBuildState_PHASE_BUILD_STATE_BUILDING ||
			op.GetBuildBeam() != "" ||
			status.GetPending().GetDigest() != "" {
			return trace.BadParameter("status.operation: invalid provisioning claim")
		}
	}
	if op.GetDeleteOnProvisionFailure() &&
		(op.GetProvisionExpires() == nil ||
			status.GetPublished().GetDigest() != "" ||
			len(status.GetRetiredSnapshots()) != 0) {
		return trace.BadParameter("status.operation.delete_on_provision_failure: requires an unpublished initial provisioning claim")
	}
	if digest := op.GetWorkingDigest(); digest != "" && !IsValidImageDigest(digest) {
		return trace.BadParameter("status.operation.working_digest: must be a canonical sha256 digest")
	}
	if digest := op.GetUpstreamImageDigest(); digest != "" && !IsValidImageDigest(digest) {
		return trace.BadParameter("status.operation.upstream_image_digest: must be a canonical sha256 digest")
	}
	if (op.GetUpstreamGeneration() == "") != (op.GetUpstreamImageDigest() == "") {
		return trace.BadParameter("status.operation.upstream_generation and status.operation.upstream_image_digest: must be set together")
	}
	if timestamp := op.GetStartedAt(); timestamp != nil && !timestamp.IsValid() {
		return trace.BadParameter("status.operation.started_at: is invalid")
	}
	if generation := op.GetUpstreamGeneration(); generation != "" && !phaseRetentionTagPattern.MatchString(generation) {
		return trace.BadParameter("status.operation.upstream_generation: is invalid")
	}
	return nil
}

func validatePhasePublished(published *beamsv1.PhasePublishedImage) error {
	if err := validatePhaseSuppressRebuilds(published.GetSuppressRebuilds()); err != nil {
		return trace.Wrap(err, "status.published")
	}
	if digest := published.GetDigest(); digest != "" && !IsValidImageDigest(digest) {
		return trace.BadParameter("status.published.digest: must be a canonical sha256 digest")
	}
	if (published.GetDigest() == "") != (published.GetGeneration() == "") {
		return trace.BadParameter("status.published.digest and status.published.generation must be set together")
	}
	if timestamp := published.GetPublishedAt(); timestamp != nil && !timestamp.IsValid() {
		return trace.BadParameter("status.published.published_at: is invalid")
	}
	if generation := published.GetUpstreamGeneration(); generation != "" &&
		!phaseRetentionTagPattern.MatchString(generation) {
		return trace.BadParameter("status.published.upstream_generation: is invalid")
	}
	return nil
}

func validatePhaseFailedBuilds(builds []*beamsv1.FailedPhaseBuild) error {
	if len(builds) > 3 {
		return trace.BadParameter("status.failed_builds: at most three entries are allowed")
	}
	for _, failed := range builds {
		if !IsValidImageDigest(failed.GetDigest()) ||
			!failed.GetTimestamp().IsValid() ||
			!failed.GetExpiration().IsValid() ||
			!failed.GetExpiration().AsTime().After(failed.GetTimestamp().AsTime()) {
			return trace.BadParameter("status.failed_builds: invalid digest, timestamp, or expiration")
		}
	}
	return nil
}

// IsValidImageDigest reports whether digest is a canonical sha256 OCI digest.
func IsValidImageDigest(digest string) bool {
	return imageDigestRegexp.MatchString(digest)
}

// MakePhaseFilterFunc returns the filter for a Phase list operation.
func MakePhaseFilterFunc(opts *ListPhasesRequestOptions) func(*beamsv1.Phase) bool {
	return func(phase *beamsv1.Phase) bool {
		if opts == nil {
			return true
		}
		if len(opts.Users) > 0 {
			if _, ok := opts.Users[phase.GetStatus().GetUser()]; !ok {
				return false
			}
		}
		return opts.FilterFn == nil || opts.FilterFn(phase)
	}
}
