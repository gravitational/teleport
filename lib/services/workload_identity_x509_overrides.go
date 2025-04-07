// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
)

type WorkloadIdentityX509Overrides interface {
	// GetX509IssuerOverride gets a single override by name. If no override with
	// such a name exists, a [*trace.NotFoundError] is returned.
	GetX509IssuerOverride(ctx context.Context, name string) (*workloadidentityv1pb.X509IssuerOverride, error)
	// ListX509IssuerOverrides returns a page of overrides with a given size;
	// iteration starts at the beginning of the list with an empty page token,
	// then can be continued in following calls by using the returned next page
	// token until it's empty.
	ListX509IssuerOverrides(ctx context.Context, pageSize int, pageToken string) (_ []*workloadidentityv1pb.X509IssuerOverride, nextPageToken string, _ error)

	// CreateX509IssuerOverride creates a new override. A
	// [*trace.AlreadyExistsError] will be returned if an override with the same
	// name already exists.
	CreateX509IssuerOverride(ctx context.Context, resource *workloadidentityv1pb.X509IssuerOverride) (*workloadidentityv1pb.X509IssuerOverride, error)
	// UpdateX509IssuerOverride updates an override; an override with the same
	// name and revision as the one passed in must already exist, or a
	// [*trace.CompareFailedError] will be returned.
	UpdateX509IssuerOverride(ctx context.Context, resource *workloadidentityv1pb.X509IssuerOverride) (*workloadidentityv1pb.X509IssuerOverride, error)
	// UpsertX509IssuerOverride creates or updates an override unconditionally.
	UpsertX509IssuerOverride(ctx context.Context, resource *workloadidentityv1pb.X509IssuerOverride) (*workloadidentityv1pb.X509IssuerOverride, error)
	// DeleteX509IssuerOverride deletes an existing override by name. If no
	// override with such a name exists, a [*trace.NotFoundError] is returned.
	DeleteX509IssuerOverride(ctx context.Context, name string) error
}
