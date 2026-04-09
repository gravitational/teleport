// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package local

import (
	"context"

	"github.com/gravitational/trace"

	subcav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/subca/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
	"github.com/gravitational/teleport/lib/subca"
)

// newCAOverridesPrefix returns the CertAuthorityOverride storage prefix key.
//
// CA override keys are in the format
// `/cert_authority_overrides/cluster/{clusterName}/{caType}`.
//
// CA override resources mimic CA resources, which means:
//   - ClusterName is recorded at ca.Resource.Name.
//   - CAType is recorded at ca.SubKind.
func newCAOverridesPrefix() backend.Key {
	return backend.NewKey("cert_authority_overrides", "cluster")
}

// CertAuthorityOverrideID uniquely identifies a CertAuthorityOverride resource.
type CertAuthorityOverrideID struct {
	ClusterName string
	CAType      string
}

// displayName returns a user-friendly string for the ID.
func (id *CertAuthorityOverrideID) displayName() string {
	return id.CAType + "/" + id.ClusterName
}

// CertAuthorityOverrideIDFromResource returns the id of the specified resource.
//
// If the resource is nil or missing components, then the corresponding ID
// fields will be empty.
func CertAuthorityOverrideIDFromResource(r *subcav1.CertAuthorityOverride) CertAuthorityOverrideID {
	return CertAuthorityOverrideID{
		ClusterName: r.GetMetadata().GetName(),
		CAType:      r.GetSubKind(),
	}
}

// SubCAServiceParams holds creation parameters for [SubCAService].
type SubCAServiceParams struct {
	Backend backend.Backend
}

// SubCAService manages backend storage of CertAuthorityOverride resources.
//
// SubCAService does not perform lateral validation against CA objects, it only
// ensures CertAuthorityOverride resources are valid within themselves. This
// allows callers with direct storage access to re-create storage configurations
// that were valid on conception but drifted over time (CA keyset changed,
// certificates expired, etc).
//
// Follows RFD 153 / generic.Service semantics.
type SubCAService struct {
	service *generic.ServiceWrapper[*subcav1.CertAuthorityOverride]
}

// NewSubCAService creates a new service using the provided params.
func NewSubCAService(p SubCAServiceParams) (*SubCAService, error) {
	service, err := generic.NewServiceWrapper(generic.ServiceConfig[*subcav1.CertAuthorityOverride]{
		Backend:       p.Backend,
		ResourceKind:  types.KindCertAuthorityOverride,
		BackendPrefix: newCAOverridesPrefix(),
		MarshalFunc:   services.MarshalCertAuthorityOverride,
		UnmarshalFunc: services.UnmarshalCertAuthorityOverride,
		ValidateFunc: func(resource *subcav1.CertAuthorityOverride) error {
			_, err := subca.ValidateAndParseCAOverride(resource)
			return trace.Wrap(err)
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &SubCAService{
		service: service,
	}, nil
}

// CreateCertAuthorityOverride creates a CA override in the backend.
func (s *SubCAService) CreateCertAuthorityOverride(
	ctx context.Context,
	resource *subcav1.CertAuthorityOverride,
) (*subcav1.CertAuthorityOverride, error) {
	service, err := s.serviceForResource(resource)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	created, err := service.CreateResource(ctx, resource)
	return created, trace.Wrap(err)
}

// UpdateCertAuthorityOverride conditionally updates a CA override in the
// backend.
func (s *SubCAService) UpdateCertAuthorityOverride(
	ctx context.Context,
	resource *subcav1.CertAuthorityOverride,
) (*subcav1.CertAuthorityOverride, error) {
	service, err := s.serviceForResource(resource)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	updated, err := service.ConditionalUpdateResource(ctx, resource)
	return updated, trace.Wrap(err)
}

// UpsertCertAuthorityOverride unconditionally creates or updates a CA override
// in the backend.
func (s *SubCAService) UpsertCertAuthorityOverride(
	ctx context.Context,
	resource *subcav1.CertAuthorityOverride,
) (*subcav1.CertAuthorityOverride, error) {
	service, err := s.serviceForResource(resource)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	updated, err := service.UpsertResource(ctx, resource)
	return updated, trace.Wrap(err)
}

// GetCertAuthorityOverride reads a CA override from the backend.
func (s *SubCAService) GetCertAuthorityOverride(
	ctx context.Context,
	id CertAuthorityOverrideID,
) (*subcav1.CertAuthorityOverride, error) {
	service, err := s.serviceForClusterAndType(id.ClusterName, id.CAType)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Name has no effect on the query, it's only used for errors.
	// See serviceForClusterAndType() / generic.Service.WithNameKeyFunc().
	name := id.displayName()

	resource, err := service.GetResource(ctx, name)
	return resource, trace.Wrap(err)
}

// ListCertAuthorityOverrides lists all CA overrides from the backend, using
// paginated responses.
func (s *SubCAService) ListCertAuthorityOverrides(
	ctx context.Context,
	pageSize int,
	pageToken string,
) (_ []*subcav1.CertAuthorityOverride, nextPageToken string, _ error) {
	// Note: We don't use serviceForClusterAndType here, it lists all clusters.
	resp, nextPageToken, err := s.service.ListResources(ctx, pageSize, pageToken)
	return resp, nextPageToken, trace.Wrap(err)
}

// DeleteCertAuthorityOverride unconditionally deletes a CA override from the
// backend.
// Returns a trace.NotFoundError if the resource cannot be found.
func (s *SubCAService) DeleteCertAuthorityOverride(
	ctx context.Context,
	id CertAuthorityOverrideID,
) error {
	service, err := s.serviceForClusterAndType(id.ClusterName, id.CAType)
	if err != nil {
		return trace.Wrap(err)
	}

	// Name has no effect on the query, it's only used for errors.
	// See serviceForClusterAndType() / generic.Service.WithNameKeyFunc().
	name := id.displayName()

	return trace.Wrap(service.DeleteResource(ctx, name))
}

func (s *SubCAService) serviceForResource(
	resource *subcav1.CertAuthorityOverride,
) (*generic.ServiceWrapper[*subcav1.CertAuthorityOverride], error) {
	return s.serviceForClusterAndType(
		resource.GetMetadata().GetName(),
		resource.GetSubKind(),
	)
}

func (s *SubCAService) serviceForClusterAndType(
	clusterName string,
	caType string,
) (*generic.ServiceWrapper[*subcav1.CertAuthorityOverride], error) {
	switch {
	case clusterName == "":
		return nil, trace.BadParameter("resource name/clusterName required")
	case caType == "":
		return nil, trace.BadParameter("resource sub_kind/caType required")
	}

	return s.service.WithNameKeyFunc(func() backend.Key {
		return backend.NewKey(clusterName, caType)
	}), nil
}
