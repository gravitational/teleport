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
	"slices"
	"strings"
	"time"

	"github.com/gravitational/trace"

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/machineid/workloadidentityv1/expression"
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
