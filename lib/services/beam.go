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
	"strings"
	"time"

	"github.com/gravitational/trace"
	"k8s.io/apimachinery/pkg/util/validation"

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

// BeamWriter defines methods for writing beam resources.
type BeamWriter interface {
	// CreateBeam creates a new beam.
	CreateBeam(ctx context.Context, in *beamsv1.Beam) (*beamsv1.Beam, error)

	// UpdateBeam updates an existing beam.
	UpdateBeam(ctx context.Context, in *beamsv1.Beam) (*beamsv1.Beam, error)

	// DeleteBeam deletes a beam.
	DeleteBeam(ctx context.Context, name string) error
}

// BeamAliasLeaseWriter defines methods for writing beam alias lease records.
type BeamAliasLeaseWriter interface {
	// CreateBeamAliasLease creates an alias lease for a beam.
	CreateBeamAliasLease(ctx context.Context, alias, beamID string, expiry time.Time) error

	// DeleteBeamAliasLease deletes an alias lease for a beam.
	DeleteBeamAliasLease(ctx context.Context, alias string) error
}

// Beams defines methods for managing beam resources.
type Beams interface {
	BeamReader
	BeamWriter
	BeamAliasLeaseWriter
}

// ValidateBeam validates the given beam resource.
func ValidateBeam(b *beamsv1.Beam) error {
	switch {
	case b == nil:
		return trace.BadParameter("beam must not be nil")
	case b.Version != types.V1:
		return trace.BadParameter("version: only supports version %q, got %q", types.V1, b.Version)
	case b.Kind != types.KindBeam:
		return trace.BadParameter("kind: must be %q, got %q", types.KindBeam, b.Kind)
	case b.Metadata == nil:
		return trace.BadParameter("metadata: is required")
	case b.Metadata.Name == "":
		return trace.BadParameter("metadata.name: is required")
	case b.Spec == nil:
		return trace.BadParameter("spec: is required")
	case b.Status == nil:
		return trace.BadParameter("status: is required")
	}

	switch b.Spec.GetEgress() {
	case beamsv1.EgressMode_EGRESS_MODE_RESTRICTED:
		for i, domain := range b.Spec.GetAllowedDomains() {
			// Must be fully-qualified, up to the root.
			if !strings.HasSuffix(domain, ".") {
				return trace.BadParameter("spec.allowed_domains[%d]: %q must be a fully qualified domain name ending with '.'", i, domain)
			}
			trimmedDomain := strings.TrimSuffix(domain, ".")

			// TLDs like "com." or "net." are invalid, as is "localhost."
			if !strings.Contains(trimmedDomain, ".") {
				return trace.BadParameter("spec.allowed_domains[%d]: %q must be a fully qualified domain name ending with '.'", i, domain)
			}

			// Note: wildcard like "*.example.com." are explicitly not supported
			// because we don't yet have a way to proxy them via VNet.
			if errs := validation.IsDNS1123Subdomain(trimmedDomain); len(errs) > 0 {
				return trace.BadParameter("spec.allowed_domains[%d]: %q is invalid: %s", i, domain, errs)
			}
		}
	case beamsv1.EgressMode_EGRESS_MODE_UNRESTRICTED:
		if len(b.Spec.GetAllowedDomains()) > 0 {
			return trace.BadParameter("spec.allowed_domains: may only be set when spec.egress is EGRESS_MODE_RESTRICTED")
		}
	default:
		return trace.BadParameter("spec.egress: must be EGRESS_MODE_RESTRICTED or EGRESS_MODE_UNRESTRICTED, got %s", b.Spec.GetEgress())
	}

	return nil
}
