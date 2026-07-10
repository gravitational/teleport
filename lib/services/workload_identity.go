// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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
	"encoding/base32"
	"iter"
	"slices"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/text/cases"

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/machineid/workloadidentityv1/expression"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/scopes"
)

// WorkloadIdentities is an interface over the WorkloadIdentities service. This
// interface may also be implemented by a client to allow remote and local
// consumers to access the resource in a similar way.
type WorkloadIdentities interface {
	// GetWorkloadIdentity gets a WorkloadIdentity by the name and scope in the
	// request. An empty scope addresses an unscoped WorkloadIdentity.
	GetWorkloadIdentity(
		ctx context.Context, req *workloadidentityv1pb.GetWorkloadIdentityRequest,
	) (*workloadidentityv1pb.WorkloadIdentity, error)
	// RangeWorkloadIdentities returns WorkloadIdentity resources within the
	// range [start, end), ordered by the given sort field and direction.
	RangeWorkloadIdentities(
		ctx context.Context,
		start, end string,
		sortField WorkloadIdentitySortField,
		sortDesc bool,
	) iter.Seq2[*workloadidentityv1pb.WorkloadIdentity, error]
	// CreateWorkloadIdentity creates a new WorkloadIdentity.
	CreateWorkloadIdentity(
		ctx context.Context, workloadIdentity *workloadidentityv1pb.WorkloadIdentity,
	) (*workloadidentityv1pb.WorkloadIdentity, error)
	// DeleteWorkloadIdentity deletes a WorkloadIdentity by the name and scope in
	// the request.
	DeleteWorkloadIdentity(ctx context.Context, req *workloadidentityv1pb.DeleteWorkloadIdentityRequest) error
	// UpdateWorkloadIdentity updates a specific WorkloadIdentity. The resource must
	// already exist, and, condition update semantics are used - e.g the submitted
	// resource must have a revision matching the revision of the resource in the
	// backend.
	UpdateWorkloadIdentity(
		ctx context.Context, workloadIdentity *workloadidentityv1pb.WorkloadIdentity,
	) (*workloadidentityv1pb.WorkloadIdentity, error)
	// UpsertWorkloadIdentity creates or updates a WorkloadIdentity.
	UpsertWorkloadIdentity(
		ctx context.Context, workloadIdentity *workloadidentityv1pb.WorkloadIdentity,
	) (*workloadidentityv1pb.WorkloadIdentity, error)

	// AppendPutWorkloadIdentityActions adds conditional actions to an atomic
	// write to create or update a WorkloadIdentity.
	AppendPutWorkloadIdentityActions(
		actions []backend.ConditionalAction,
		resource *workloadidentityv1pb.WorkloadIdentity,
		condition backend.Condition,
	) ([]backend.ConditionalAction, error)

	// AppendDeleteWorkloadIdentityActions adds conditional actions to an atomic
	// write to delete a WorkloadIdentity given its scope-qualified name.
	AppendDeleteWorkloadIdentityActions(
		actions []backend.ConditionalAction,
		name scopes.QualifiedName,
		condition backend.Condition,
	) ([]backend.ConditionalAction, error)
}

// MarshalWorkloadIdentity marshals the WorkloadIdentity object into a JSON byte
// array.
func MarshalWorkloadIdentity(
	object *workloadidentityv1pb.WorkloadIdentity, opts ...MarshalOption,
) ([]byte, error) {
	return MarshalProtoResource(object, opts...)
}

// UnmarshalWorkloadIdentity unmarshals the WorkloadIdentity object from a
// JSON byte array.
func UnmarshalWorkloadIdentity(
	data []byte, opts ...MarshalOption,
) (*workloadidentityv1pb.WorkloadIdentity, error) {
	return UnmarshalProtoResource[*workloadidentityv1pb.WorkloadIdentity](data, opts...)
}

const (
	maxMaxJWTSVIDTTL  = time.Hour * 24
	maxMaxX509SVIDTTL = time.Hour * 24 * 14
)

// ValidateWorkloadIdentity validates the WorkloadIdentity object. This is
// performed prior to writing to the backend.
func ValidateWorkloadIdentity(s *workloadidentityv1pb.WorkloadIdentity) error {
	switch {
	case s == nil:
		return trace.BadParameter("object cannot be nil")
	case s.GetVersion() != types.V1:
		return trace.BadParameter("version: only %q is supported", types.V1)
	case s.GetKind() != types.KindWorkloadIdentity:
		return trace.BadParameter("kind: must be %q", types.KindWorkloadIdentity)
	case !s.HasMetadata():
		return trace.BadParameter("metadata: is required")
	case s.GetMetadata().GetName() == "":
		return trace.BadParameter("metadata.name: is required")
	case !s.HasSpec():
		return trace.BadParameter("spec: is required")
	case s.GetSpec().GetSpiffe().GetId() == "":
		return trace.BadParameter("spec.spiffe.id: is required")
	case !strings.HasPrefix(s.GetSpec().GetSpiffe().GetId(), "/"):
		return trace.BadParameter("spec.spiffe.id: must start with a /")
	case s.GetSpec().GetSpiffe().GetX509().GetMaximumTtl().AsDuration() > maxMaxX509SVIDTTL:
		return trace.BadParameter("spec.spiffe.x509.maximum_ttl: must be less than %s", maxMaxX509SVIDTTL)
	case s.GetSpec().GetSpiffe().GetJwt().GetMaximumTtl().AsDuration() > maxMaxJWTSVIDTTL:
		return trace.BadParameter("spec.spiffe.jwt.maximum_ttl: must be less than %s", maxMaxJWTSVIDTTL)
	}

	// When the WorkloadIdentity is scoped, the scope itself must be valid and
	// the SPIFFE ID must conform to the scoped SPIFFE ID structure (RFD 0229c).
	if s.GetScope() != "" {
		if err := scopes.StrongValidate(s.GetScope()); err != nil {
			return trace.Wrap(err, "scope")
		}
		if scopes.Compare(s.GetScope(), scopes.Root) == scopes.Equivalent {
			return trace.BadParameter("scope: must not be the root scope")
		}
		if err := validateScopedSPIFFEID(s.GetScope(), s.GetSpec().GetSpiffe().GetId()); err != nil {
			return trace.Wrap(err)
		}

		// TODO(strideynet): For now we only constrict the naming of scoped
		// workload identities - however - we should consider rolling out a
		// write-side restriction to unscoped workload identities in a major
		// version.
		if err := scopes.StrongValidateSegment(s.GetMetadata().GetName()); err != nil {
			return trace.Wrap(err, "metadata.name:")
		}
	}

	for i, rule := range s.GetSpec().GetRules().GetAllow() {
		if rule.GetExpression() == "" {
			if len(rule.GetConditions()) == 0 {
				return trace.BadParameter("spec.rules.allow[%d].conditions: must be non-empty", i)
			}
		} else {
			if len(rule.GetConditions()) != 0 {
				return trace.BadParameter("spec.rules.allow[%d].conditions: is mutually exclusive with expression", i)
			}
			if err := expression.Validate(rule.GetExpression()); err != nil {
				return trace.BadParameter("spec.rules.allow[%d].expression: invalid expression: %s", i, err.Error())
			}
		}

		for j, condition := range rule.GetConditions() {
			if condition.GetAttribute() == "" {
				return trace.BadParameter("spec.rules.allow[%d].conditions[%d].attribute: must be non-empty", i, j)
			}
			if !condition.HasOperator() {
				return trace.BadParameter("spec.rules.allow[%d].conditions[%d]: operator must be specified", i, j)
			}
		}
	}

	return nil
}

// scopedSPIFFEIDSeparator is the path segment that separates the scope-derived
// section of a scoped SPIFFE ID from the administratively-defined section. A
// scoped SPIFFE ID for a WorkloadIdentity defined in scope /security/eu looks
// like /security/eu/_/k8s/cluster-a. See RFD 0229c.
//
// The separator is a valid SPIFFE ID path segment but is deliberately not a
// valid scope segment (scope segments require at least two characters), which
// keeps the boundary between the two sections unambiguous.
const scopedSPIFFEIDSeparator = "_"

// validateScopedSPIFFEID validates that the given SPIFFE ID path conforms to
// the scoped SPIFFE ID structure for the given scope, as defined in RFD 0229c.
//
// A scoped SPIFFE ID path consists of three sections:
//   - the scope section: segments that strictly match the scope of origin
//   - the separator segment ("/_/")
//   - the administratively-defined section: one or more freely-defined segments
//
// For example, for scope /security/eu, /security/eu/_/k8s/cluster-a is valid.
//
// The check is performed on segments (not raw string prefixes) so that, e.g., a
// scope of /foo matches an ID beginning /foo/... but not /foo-buzz/.... The
// scope section must match the scope of origin exactly: it may not be an
// ancestor or descendant of it.
func validateScopedSPIFFEID(scope, id string) error {
	if !strings.HasPrefix(id, "/") {
		return trace.BadParameter("spec.spiffe.id: must begin with a forward slash")
	}
	// Reject empty path segments (e.g. a trailing slash or "//"), which
	// splitPathSegments would otherwise silently trim or mishandle.
	if strings.HasSuffix(id, "/") || strings.Contains(id, "//") {
		return trace.BadParameter("spec.spiffe.id %q must not contain empty path segments", id)
	}

	scopeSegments := splitPathSegments(scope)
	idSegments := splitPathSegments(id)

	separatorIndex := slices.Index(idSegments, scopedSPIFFEIDSeparator)
	if separatorIndex < 0 {
		return trace.BadParameter(
			"spec.spiffe.id %q is missing the %q separator segment that delimits the scope from the administratively-defined section",
			id, scopedSPIFFEIDSeparator,
		)
	}

	scopeSection := idSegments[:separatorIndex]
	adminSection := idSegments[separatorIndex+1:]

	if !slices.Equal(scopeSection, scopeSegments) {
		return trace.BadParameter(
			"spec.spiffe.id %q must be prefixed with the scope %q, immediately followed by the %q separator segment",
			id, scope, scopedSPIFFEIDSeparator,
		)
	}

	if len(adminSection) == 0 {
		return trace.BadParameter(
			"spec.spiffe.id %q must have at least one segment after the %q separator",
			id, scopedSPIFFEIDSeparator,
		)
	}

	if slices.Contains(adminSection, scopedSPIFFEIDSeparator) {
		return trace.BadParameter(
			"spec.spiffe.id %q must not contain the %q separator segment in its administratively-defined section",
			id, scopedSPIFFEIDSeparator,
		)
	}

	return nil
}

// splitPathSegments splits a slash-delimited path (a scope or a SPIFFE ID path)
// into its non-empty segments. A leading slash is expected; empty input or "/"
// yields no segments.
func splitPathSegments(path string) []string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

// WorkloadIdentitySortField identifies a field that WorkloadIdentities may be
// sorted (and ranged) by. An empty value defaults to
// [WorkloadIdentitySortFieldName].
type WorkloadIdentitySortField string

const (
	// WorkloadIdentitySortFieldName sorts WorkloadIdentities by name. It is the
	// default when no sort field is specified.
	WorkloadIdentitySortFieldName WorkloadIdentitySortField = "name"
	// WorkloadIdentitySortFieldSPIFFEID sorts WorkloadIdentities by SPIFFE ID.
	WorkloadIdentitySortFieldSPIFFEID WorkloadIdentitySortField = "spiffe_id"
)

// WorkloadIdentityKey returns a function deriving the canonical ordering key for
// a WorkloadIdentity for the given sort field. It is the single source of truth
// for both the order in which WorkloadIdentities are iterated and the pagination
// cursor used to resume iteration. Supported sort fields are "" (defaults to
// name), [WorkloadIdentitySortFieldName] and [WorkloadIdentitySortFieldSPIFFEID];
// any other sort field returns an error.
//
// The sort field is validated once, when the key function is obtained, so
// callers do not have to handle an error per resource.
func WorkloadIdentityKey(sortField WorkloadIdentitySortField) (func(*workloadidentityv1pb.WorkloadIdentity) string, error) {
	switch sortField {
	case "", WorkloadIdentitySortFieldName:
		return workloadIdentityCursor, nil
	case WorkloadIdentitySortFieldSPIFFEID:
		return workloadIdentitySPIFFEIDKey, nil
	default:
		return nil, trace.BadParameter("unsupported sort %q but expected %s or %s", sortField, WorkloadIdentitySortFieldName, WorkloadIdentitySortFieldSPIFFEID)
	}
}

// workloadIdentityCursor returns the canonical resource cursor for a
// WorkloadIdentity, used both as its in-memory cache index key and as its
// pagination cursor, so it must be stable and unique per resource.
func workloadIdentityCursor(wi *workloadidentityv1pb.WorkloadIdentity) string {
	return scopes.MakeResourceCursor(wi.GetScope(), wi.GetMetadata().GetName())
}

// workloadIdentitySPIFFEIDKey returns the ordering key for the spiffe_id sort.
func workloadIdentitySPIFFEIDKey(wi *workloadidentityv1pb.WorkloadIdentity) string {
	// Sort case-insensitively to keep /spiffe-1 and /Spiffe-1 together.
	spiffeID := cases.Fold().String(wi.GetSpec().GetSpiffe().GetId())
	// Encode to avoid ambiguity; "a/b" + "/" + "c" vs. "a" + "/" + "b/c". Base32
	// hex maintains the original ordering.
	spiffeID = base32.HexEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte(spiffeID))
	// SPIFFE IDs may not be unique, so append the resource cursor, which
	// uniquely identifies the resource across scopes.
	return spiffeID + "/" + workloadIdentityCursor(wi)
}
