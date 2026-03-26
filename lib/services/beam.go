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
	"regexp"
	"strings"

	"github.com/gravitational/trace"
	"k8s.io/apimachinery/pkg/util/validation"

	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	workloadidentityv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
)

var beamAliasRegexp = regexp.MustCompile(`^[a-z]+-[a-z]+$`)

// BeamReader defines methods for reading beam resources.
type BeamReader interface {
	// GetBeam fetches a beam by name.
	GetBeam(ctx context.Context, name string) (*beamsv1.Beam, error)

	// GetBeamByAlias fetches a beam by alias.
	GetBeamByAlias(ctx context.Context, alias string) (*beamsv1.Beam, error)

	// ListBeams lists beams with pagination.
	ListBeams(ctx context.Context, limit int, startKey string) ([]*beamsv1.Beam, string, error)
}

// BeamWriter defines methods for writing beam resources. They atomically operate
// on multiple records (i.e. the beam and its supported resources) so diverge from
// the usual simple CRUD methods.
type BeamWriter interface {
	// CreateBeam atomically writes the beam and its supporting resources to the
	// backend. If the beam's alias is already in-use, or any other resource name
	// conflicts, an AlreadyExists error will be returned, and the caller should
	// generate a new alias and resource names and try again.
	//
	// This function should be called before the actual VM is provisioned so that
	// if a subsequent operation fails, we maintain a record of it, and can clean
	// the VM up later.
	CreateBeam(ctx context.Context, p CreateBeamParams) (*beamsv1.Beam, error)

	// UpdateBeamCreateNode atomically writes the beam and node to the backend.
	// It is used to "finalize" the creation of the beam.
	UpdateBeamCreateNode(ctx context.Context, beam *beamsv1.Beam, node types.Server) (*beamsv1.Beam, error)

	// UpdateBeamCreateApp atomically writes the beam and app to the backend. It
	// is used to "publish" the beam.
	UpdateBeamCreateApp(ctx context.Context, beam *beamsv1.Beam, app types.Application) (*beamsv1.Beam, error)

	// UpdateBeamDeleteApp atomically writes the beam and deletes its app from
	// the backend. It is used to "unpublish" the beam.
	UpdateBeamDeleteApp(ctx context.Context, beam *beamsv1.Beam, appName string) (*beamsv1.Beam, error)

	// DeleteBeam atomically deletes the beam and its supporting resources from
	// the backend. It should not be called until the VM has been cleaned up.
	DeleteBeam(ctx context.Context, name string) error
}

// CreateBeamParams contains the parameters to CreateBeam, including the
// resources that must exist before the VM is provisioned.
//
// TODO(boxofrad): Add DelegationSession once #64772 is merged.
type CreateBeamParams struct {
	Beam             *beamsv1.Beam
	Token            types.ProvisionToken
	BotUser          types.User
	BotRole          types.Role
	WorkloadIdentity *workloadidentityv1.WorkloadIdentity
}

func (p *CreateBeamParams) Validate() error {
	if err := ValidateBeam(p.Beam); err != nil {
		return trace.Wrap(err, "validating beam")
	}

	if p.Token == nil {
		return trace.BadParameter("token is required")
	}

	if p.BotUser == nil {
		return trace.BadParameter("bot user is required")
	}
	if err := ValidateUser(p.BotUser); err != nil {
		return trace.Wrap(err, "validating user")
	}

	if p.BotRole == nil {
		return trace.BadParameter("bot role is required")
	}
	if err := ValidateRole(p.BotRole); err != nil {
		return trace.Wrap(err, "validating bot role")
	}

	if err := ValidateWorkloadIdentity(p.WorkloadIdentity); err != nil {
		return trace.Wrap(err, "validating workload identity")
	}
	return nil
}

// Beams defines methods for managing beam resources.
type Beams interface {
	BeamReader
	BeamWriter
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
	case b.Spec.Expires == nil:
		return trace.BadParameter("spec.expires: is required")
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

	if pub := b.Spec.Publish; pub != nil {
		if pub.Port != 8080 {
			return trace.BadParameter("spec.publish.port: must be 8080")
		}
		switch pub.Protocol {
		case beamsv1.Protocol_PROTOCOL_HTTP, beamsv1.Protocol_PROTOCOL_TCP:
		default:
			return trace.BadParameter("spec.publish.protocol: must be HTTP or TCP")
		}
	}

	if err := ValidateBeamAlias(b.GetStatus().GetAlias()); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func ValidateBeamAlias(alias string) error {
	if beamAliasRegexp.MatchString(alias) {
		return nil
	}
	return trace.BadParameter("beam alias must be a hyphen-separated pair of two lowercase words")
}
