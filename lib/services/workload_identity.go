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
	// GetWorkloadIdentity gets a SPIFFE Federation by name.
	GetWorkloadIdentity(
		ctx context.Context, name string,
	) (*workloadidentityv1pb.WorkloadIdentity, error)
	// ListWorkloadIdentities lists all WorkloadIdentities using Google style
	// pagination.
	ListWorkloadIdentities(
		ctx context.Context,
		pageSize int,
		lastToken string,
		options *ListWorkloadIdentitiesRequestOptions,
	) ([]*workloadidentityv1pb.WorkloadIdentity, string, error)
	// RangeWorkloadIdentities returns WorkloadIdentity resources within the
	// range [start, end), ordered by the given sort field (defaulting to name).
	// The start and end tokens must be in the keyspace of the selected sort
	// field (see [WorkloadIdentitySortKey]). If end is empty, iteration
	// continues to the end of the range.
	RangeWorkloadIdentities(
		ctx context.Context, start, end, sortField string, desc bool,
	) iter.Seq2[*workloadidentityv1pb.WorkloadIdentity, error]
	// CreateWorkloadIdentity creates a new WorkloadIdentity.
	CreateWorkloadIdentity(
		ctx context.Context, workloadIdentity *workloadidentityv1pb.WorkloadIdentity,
	) (*workloadidentityv1pb.WorkloadIdentity, error)
	// DeleteWorkloadIdentity deletes a SPIFFE Federation by name.
	DeleteWorkloadIdentity(ctx context.Context, name string) error
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
	// write to delete a WorkloadIdentity.
	AppendDeleteWorkloadIdentityActions(
		actions []backend.ConditionalAction,
		name string,
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
	case s.Version != types.V1:
		return trace.BadParameter("version: only %q is supported", types.V1)
	case s.Kind != types.KindWorkloadIdentity:
		return trace.BadParameter("kind: must be %q", types.KindWorkloadIdentity)
	case s.Metadata == nil:
		return trace.BadParameter("metadata: is required")
	case s.Metadata.Name == "":
		return trace.BadParameter("metadata.name: is required")
	case s.Spec == nil:
		return trace.BadParameter("spec: is required")
	case s.Spec.Spiffe.Id == "":
		return trace.BadParameter("spec.spiffe.id: is required")
	case !strings.HasPrefix(s.Spec.Spiffe.Id, "/"):
		return trace.BadParameter("spec.spiffe.id: must start with a /")
	case s.Spec.Spiffe.GetX509().GetMaximumTtl().AsDuration() > maxMaxX509SVIDTTL:
		return trace.BadParameter("spec.spiffe.x509.maximum_ttl: must be less than %s", maxMaxX509SVIDTTL)
	case s.Spec.Spiffe.GetJwt().GetMaximumTtl().AsDuration() > maxMaxJWTSVIDTTL:
		return trace.BadParameter("spec.spiffe.jwt.maximum_ttl: must be less than %s", maxMaxJWTSVIDTTL)
	}

	// When the WorkloadIdentity is scoped, the scope must be valid and the
	// SPIFFE ID must conform to the scoped SPIFFE ID structure (RFD 0229c).
	if s.Scope != "" {
		if err := scopes.StrongValidate(s.Scope); err != nil {
			return trace.Wrap(err, "scope")
		}
		if scopes.Compare(s.Scope, scopes.Root) == scopes.Equivalent {
			return trace.BadParameter("scope: must not be the root scope")
		}
		if err := ValidateScopedSPIFFEID(s.Scope, s.Spec.Spiffe.Id); err != nil {
			return trace.Wrap(err)
		}
	}

	for i, rule := range s.GetSpec().GetRules().GetAllow() {
		if rule.Expression == "" {
			if len(rule.Conditions) == 0 {
				return trace.BadParameter("spec.rules.allow[%d].conditions: must be non-empty", i)
			}
		} else {
			if len(rule.Conditions) != 0 {
				return trace.BadParameter("spec.rules.allow[%d].conditions: is mutually exclusive with expression", i)
			}
			if err := expression.Validate(rule.Expression); err != nil {
				return trace.BadParameter("spec.rules.allow[%d].expression: invalid expression: %s", i, err.Error())
			}
		}

		for j, condition := range rule.Conditions {
			if condition.Attribute == "" {
				return trace.BadParameter("spec.rules.allow[%d].conditions[%d].attribute: must be non-empty", i, j)
			}
			if condition.Operator == nil {
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

// ValidateScopedSPIFFEID validates that the given SPIFFE ID path conforms to
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
//
// This is enforced on the unrendered ID at create/update time (via
// [ValidateWorkloadIdentity]) and re-enforced on the rendered ID at issuance
// time as a defense-in-depth measure against templating bypassing the check.
func ValidateScopedSPIFFEID(scope, id string) error {
	if !strings.HasPrefix(id, "/") {
		return trace.BadParameter("spec.spiffe.id: must begin with a forward slash")
	}

	scopeSegments := splitPathSegments(scope)
	idSegments := splitPathSegments(id)

	// The ID must contain the scope section, the separator, and at least one
	// administratively-defined segment.
	if len(idSegments) < len(scopeSegments)+2 {
		return trace.BadParameter(
			"spec.spiffe.id %q must be prefixed with the scope %q, followed by the %q separator and at least one further segment",
			id, scope, scopedSPIFFEIDSeparator,
		)
	}

	// The scope section must strictly match the scope of origin segment-by-segment.
	for i, seg := range scopeSegments {
		if idSegments[i] != seg {
			return trace.BadParameter(
				"spec.spiffe.id %q must be prefixed with the scope %q", id, scope,
			)
		}
	}

	// The separator must immediately follow the scope section.
	if idSegments[len(scopeSegments)] != scopedSPIFFEIDSeparator {
		return trace.BadParameter(
			"spec.spiffe.id %q must contain the %q separator segment immediately after the scope %q",
			id, scopedSPIFFEIDSeparator, scope,
		)
	}

	// The administratively-defined section must not contain the separator. This
	// ensures a scoped SPIFFE ID contains exactly one separator segment so the
	// boundary between sections is unambiguous.
	for _, seg := range idSegments[len(scopeSegments)+1:] {
		if seg == scopedSPIFFEIDSeparator {
			return trace.BadParameter(
				"spec.spiffe.id %q must not contain the %q separator segment in its administratively-defined section",
				id, scopedSPIFFEIDSeparator,
			)
		}
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

type ListWorkloadIdentitiesRequestOptions struct {
	// The sort field to use for the results. If empty, the default sort field is used.
	SortField string
	// The sort order to use for the results. If empty, the default sort order is used.
	SortDesc bool
	// A search term used to filter the results. If non-empty, it's used to match against supported fields.
	FilterSearchTerm string
}

func (o *ListWorkloadIdentitiesRequestOptions) GetSortField() string {
	if o == nil {
		return ""
	}
	return o.SortField
}

func (o *ListWorkloadIdentitiesRequestOptions) GetSortDesc() bool {
	if o == nil {
		return false
	}
	return o.SortDesc
}

func (o *ListWorkloadIdentitiesRequestOptions) GetFilterSearchTerm() string {
	if o == nil {
		return ""
	}
	return o.FilterSearchTerm
}

// WorkloadIdentitySortKey returns the pagination cursor key for the given
// WorkloadIdentity under the given sort field. The returned value is suitable
// for use as the start token of a subsequent RangeWorkloadIdentities call and
// matches the next-page-token format returned by ListWorkloadIdentities.
func WorkloadIdentitySortKey(item *workloadidentityv1pb.WorkloadIdentity, sortField string) (string, error) {
	switch sortField {
	case "", "name":
		return WorkloadIdentityNameSortKey(item), nil
	case "spiffe_id":
		return WorkloadIdentitySpiffeIDSortKey(item), nil
	default:
		return "", trace.BadParameter("unsupported sort %q but expected name or spiffe_id", sortField)
	}
}

// WorkloadIdentityNameSortKey returns the name-ordered cursor key for a
// WorkloadIdentity.
func WorkloadIdentityNameSortKey(item *workloadidentityv1pb.WorkloadIdentity) string {
	return item.GetMetadata().GetName()
}

// WorkloadIdentitySpiffeIDSortKey returns the spiffe-id-ordered cursor key for a
// WorkloadIdentity.
func WorkloadIdentitySpiffeIDSortKey(item *workloadidentityv1pb.WorkloadIdentity) string {
	name := WorkloadIdentityNameSortKey(item)
	// Sort case-insensitively to keep /spiffe-1 and /Spiffe-1 together.
	spiffeID := cases.Fold().String(item.GetSpec().GetSpiffe().GetId())
	// Encode the id to avoid ambiguity; "a/b" + "/" + "c" vs. "a" + "/" + "b/c".
	// Base32 hex maintains the original ordering.
	spiffeID = base32.HexEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte(spiffeID))
	// SPIFFE IDs may not be unique, so append the resource name.
	return spiffeID + "/" + name
}

func MatchWorkloadIdentity(item *workloadidentityv1pb.WorkloadIdentity, filterSearchTerm string) bool {
	if item == nil {
		return false
	}
	if filterSearchTerm == "" {
		return true
	}

	values := []string{
		item.GetMetadata().GetName(),
		item.GetSpec().GetSpiffe().GetId(),
		item.GetSpec().GetSpiffe().GetHint(),
	}

	return slices.ContainsFunc(values, func(val string) bool {
		return strings.Contains(strings.ToLower(val), strings.ToLower(filterSearchTerm))
	})
}
