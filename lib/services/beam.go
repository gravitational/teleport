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
	"time"

	"github.com/gravitational/trace"

	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	"github.com/gravitational/teleport/api/types"
)

// BeamReader defines methods for reading beam resources.
type BeamReader interface {
	// GetBeam fetches a beam by name.
	GetBeam(ctx context.Context, name string) (*beamsv1.Beam, error)
	// GetBeamByAlias fetches a beam by alias.
	GetBeamByAlias(ctx context.Context, alias string) (*beamsv1.Beam, error)
	// ListBeams lists beams with pagination.
	ListBeams(ctx context.Context, limit int, startKey string) ([]*beamsv1.Beam, string, error)
}

// Beams is a service that manages [beamsv1.Beam] resources.
type Beams interface {
	BeamReader

	// CreateBeam creates a new beam.
	CreateBeam(ctx context.Context, in *beamsv1.Beam) (*beamsv1.Beam, error)
	// UpdateBeam updates an existing beam.
	UpdateBeam(ctx context.Context, in *beamsv1.Beam) (*beamsv1.Beam, error)
	// UpsertBeam creates or updates a beam.
	UpsertBeam(ctx context.Context, in *beamsv1.Beam) (*beamsv1.Beam, error)
	// DeleteBeam deletes a beam.
	DeleteBeam(ctx context.Context, name string) error
	// CreateBeamAliasLease creates an alias lease for a beam.
	CreateBeamAliasLease(ctx context.Context, alias, beamID string, expiry time.Time) error
	// DeleteBeamAliasLease deletes an alias lease for a beam.
	DeleteBeamAliasLease(ctx context.Context, alias string) error
}

// ValidateBeam validates the given beam resource.
func ValidateBeam(b *beamsv1.Beam) error {
	switch {
	case b == nil:
		return trace.BadParameter("beam must not be nil")
	case b.Version != types.V1:
		return trace.BadParameter("beam only supports version %q, got %q", types.V1, b.Version)
	case b.Kind != types.KindBeam:
		return trace.BadParameter("beam kind must be %q, got %q", types.KindBeam, b.Kind)
	case b.Metadata == nil:
		return trace.BadParameter("beam metadata is missing")
	case b.Metadata.Name == "":
		return trace.BadParameter("beam metadata.name is missing")
	case b.Spec == nil:
		// TODO: validate the allowed_domains.
		return trace.BadParameter("beam spec is missing")
	case b.Status == nil:
		return trace.BadParameter("beam status is missing")
	}

	return nil
}
