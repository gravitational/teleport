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
	"slices"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/timestamppb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
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
type CertAuthorityOverrideID = types.CertAuthorityOverrideID

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
	clock      clockwork.Clock
	service    *generic.ServiceWrapper[*subcav1.CertAuthorityOverride]
	csrService *generic.ServiceWrapper[*subcav1.PendingCSRRequest]
}

// Keep interface in-sync with implementation.
var _ services.SubCAService = (*SubCAService)(nil)
var _ services.PendingCSRRequestService = (*SubCAService)(nil)

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

	csrService, err := generic.NewServiceWrapper(generic.ServiceConfig[*subcav1.PendingCSRRequest]{
		Backend:       p.Backend,
		ResourceKind:  types.KindPendingCSRRequest,
		BackendPrefix: newPendingCSRRequestPrefix(),
		MarshalFunc:   marshalPendingCSRRequest,
		UnmarshalFunc: unmarshalPendingCSRRequest,
		ValidateFunc:  validatePendingCSRRequest,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clock := p.Backend.Clock()
	if clock == nil {
		clock = clockwork.NewRealClock()
	}

	return &SubCAService{
		clock:      clock,
		service:    service,
		csrService: csrService,
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
	name := id.FullName()

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
//
// Prefer [SubCAService.ConditionalDeleteCertAuthorityOverride].
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
	name := id.FullName()

	return trace.Wrap(service.DeleteResource(ctx, name))
}

// ConditionalDeleteCertAuthorityOverride conditionally deletes a CA override
// based on its revision.
// Returns a trace.CompareFailedError if the item is not found or the revision
// is incorrect.
func (s *SubCAService) ConditionalDeleteCertAuthorityOverride(
	ctx context.Context,
	id CertAuthorityOverrideID,
	revision string,
) error {
	service, err := s.serviceForClusterAndType(id.ClusterName, id.CAType)
	if err != nil {
		return trace.Wrap(err)
	}

	// Name has no effect on the delete, it's only used for errors.
	// See serviceForClusterAndType() / generic.Service.WithNameKeyFunc().
	name := id.FullName()

	return trace.Wrap(service.ConditionalDeleteResource(ctx, name, revision))
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

type certAuthorityOverrideParser struct {
	baseParser
}

func newCertAuthorityOverrideParser() *certAuthorityOverrideParser {
	return &certAuthorityOverrideParser{
		baseParser: newBaseParser(newCAOverridesPrefix()),
	}
}

func (p *certAuthorityOverrideParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		trimmedKey := event.Item.Key.TrimPrefix(newCAOverridesPrefix())
		parts := trimmedKey.Components()
		if len(parts) != 2 {
			return nil, trace.BadParameter("unexpected %s key: %s", types.KindCertAuthorityOverride, event.Item.Key)
		}
		// Note! Storage keys mimic CAs, so they go {ClusterName}/{CAType}.
		// This is the inverse of almost everything else (CertAuthorityOverrideID,
		// RPCs, audit, tctl, etc), which go {CAType}/{ClusterName} instead.
		name := parts[0]
		subKind := parts[1]

		return types.Resource153ToLegacy(subcav1.CertAuthorityOverride_builder{
			Kind:    types.KindCertAuthorityOverride,
			Version: types.V1,
			SubKind: subKind,
			Metadata: headerv1.Metadata_builder{
				Name: name,
			}.Build(),
		}.Build()), nil
	case types.OpPut:
		r, err := services.UnmarshalCertAuthorityOverride(event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return types.Resource153ToLegacy(r), nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

// itemFromCertAuthorityOverride is used by CreateResources.
func itemFromCertAuthorityOverride(resource *subcav1.CertAuthorityOverride) (*backend.Item, error) {
	if _, err := subca.ValidateAndParseCAOverride(resource); err != nil {
		return nil, trace.Wrap(err)
	}
	value, err := services.MarshalCertAuthorityOverride(resource)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	expires, err := types.GetExpiry(resource)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	key := newCAOverridesPrefix().AppendKey(backend.NewKey(
		resource.GetMetadata().GetName(),
		resource.GetSubKind(),
	))
	return &backend.Item{
		Key:      key,
		Value:    value,
		Expires:  expires,
		Revision: resource.GetMetadata().GetRevision(),
	}, nil
}

// CreatePendingCSRRequest creates a PendingCSRRequest.
//
// PendingCSRRequest instances must have an expiration. If they don't a
// default expiration is assigned on creation.
func (s *SubCAService) CreatePendingCSRRequest(ctx context.Context, resource *subcav1.PendingCSRRequest) (*subcav1.PendingCSRRequest, error) {
	s.setPendingCSRRequestDefaultExpires(resource)

	created, err := s.csrService.CreateResource(ctx, resource)
	return created, trace.Wrap(err)
}

// UpdatePendingCSRRequest conditionally updates a PendingCSRRequest.
func (s *SubCAService) UpdatePendingCSRRequest(ctx context.Context, resource *subcav1.PendingCSRRequest) (*subcav1.PendingCSRRequest, error) {
	s.setPendingCSRRequestDefaultExpires(resource)

	updated, err := s.csrService.ConditionalUpdateResource(ctx, resource)
	return updated, trace.Wrap(err)
}

// DeletePendingCSRRequest hard-deletes a PendingCSRRequest.
func (s *SubCAService) DeletePendingCSRRequest(ctx context.Context, name string) error {
	if name == "" {
		return trace.BadParameter("name required")
	}
	return trace.Wrap(s.csrService.DeleteResource(ctx, name))
}

// GetPendingCSRRequest reads a PendingCSRRequest by name.
func (s *SubCAService) GetPendingCSRRequest(ctx context.Context, name string) (*subcav1.PendingCSRRequest, error) {
	if name == "" {
		return nil, trace.BadParameter("name required")
	}
	resource, err := s.csrService.GetResource(ctx, name)
	return resource, trace.Wrap(err)
}

// ListPendingCSRRequests lists all PendingCSRRequests.
func (s *SubCAService) ListPendingCSRRequests(ctx context.Context, pageSize int, pageToken string) (_ []*subcav1.PendingCSRRequest, nextPageToken string, _ error) {
	resources, nextPageToken, err := s.csrService.ListResources(ctx, pageSize, pageToken)
	return resources, nextPageToken, trace.Wrap(err)
}

func (s *SubCAService) setPendingCSRRequestDefaultExpires(resource *subcav1.PendingCSRRequest) {
	if !resource.HasMetadata() || resource.GetMetadata().HasExpires() {
		return
	}
	const defaultExpires = 5 * time.Minute
	resource.GetMetadata().SetExpires(timestamppb.New(s.clock.Now().Add(defaultExpires)))
}

func newPendingCSRRequestPrefix() backend.Key {
	return backend.NewKey("cert_authority_overrides", "csr_req")
}

func marshalPendingCSRRequest(resource *subcav1.PendingCSRRequest, opts ...services.MarshalOption) ([]byte, error) {
	return services.MarshalProtoResource(resource, opts...)
}

func unmarshalPendingCSRRequest(data []byte, opts ...services.MarshalOption) (*subcav1.PendingCSRRequest, error) {
	return services.UnmarshalProtoResource[*subcav1.PendingCSRRequest](data, opts...)
}

func validatePendingCSRRequest(resource *subcav1.PendingCSRRequest) error {
	switch {
	case resource == nil:
		return trace.BadParameter("nil PendingCSRRequest")
	case resource.GetKind() != types.KindPendingCSRRequest:
		return trace.BadParameter("invalid kind: %q", resource.GetKind())
	case resource.GetSubKind() != "":
		return trace.BadParameter("invalid sub_kind: %q (sub_kind not supported)", resource.GetSubKind())
	case resource.GetVersion() != types.V1:
		return trace.BadParameter("invalid or unsupported version: %q", resource.GetVersion())
	case resource.GetMetadata().GetName() == "":
		return trace.BadParameter("metadata.name required")
	case !resource.GetMetadata().HasExpires():
		return trace.BadParameter("metadata.expires required")
	case !resource.HasSpec():
		return trace.BadParameter("spec required")
	case resource.GetSpec().GetClusterName() == "":
		return trace.BadParameter("spec.cluster_name required")
	case !slices.Contains(subca.SupportedCATypes(), resource.GetSpec().GetCaType()):
		return trace.BadParameter("spec.ca_type invalid or unsupported: %q", resource.GetSpec().GetCaType())
	case len(resource.GetSpec().GetPublicKeyHashes()) == 0:
		return trace.BadParameter("spec.public_key_hashes required")
	}

	// spec.custom_subject.
	if subj := resource.GetSpec().GetCustomSubject(); subj != nil {
		if _, err := subca.DistinguishedNameProtoToRDNSequence(subj); err != nil {
			return trace.Wrap(err, "parse spec.custom_subject")
		}
	}

	// spec.public_key_hashes.
	knownPKHs := make(map[string]struct{})
	for i, pkh := range resource.GetSpec().GetPublicKeyHashes() {
		if pkh.GetValue() == "" {
			return trace.BadParameter("spec.public_key_hashes[%d]: value required", i)
		}
		knownPKHs[pkh.GetValue()] = struct{}{}
	}

	// status.public_key_hash_to_pending_csr.
	const pkhToPendingField = "status.public_key_hash_to_pending_csr"
	for k, v := range resource.GetStatus().GetPublicKeyHashToPendingCsr() {
		if _, ok := knownPKHs[k]; !ok {
			return trace.BadParameter("%s[%s]: unrequested key not allowed", pkhToPendingField, k)
		}
		switch {
		case !v.HasStatus():
			return trace.BadParameter("%s[%s]: status required", pkhToPendingField, k)
		case codes.Code(v.GetStatus().GetCode()) == codes.OK && v.GetCsr().GetPem() == "":
			return trace.BadParameter("%s[%s]: csr required for status OK", pkhToPendingField, k)
		}
	}

	return nil
}

type pendingCSRRequestParser struct {
	baseParser
}

func newPendingCSRRequestParser() *pendingCSRRequestParser {
	return &pendingCSRRequestParser{
		baseParser: newBaseParser(newPendingCSRRequestPrefix()),
	}
}

func (p *pendingCSRRequestParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		name := event.Item.Key.TrimPrefix(newPendingCSRRequestPrefix()).String()
		name = strings.TrimPrefix(name, backend.SeparatorString)
		if name == "" {
			return nil, trace.BadParameter("unexpected %s key: %s", types.KindPendingCSRRequest, event.Item.Key)
		}

		return types.Resource153ToLegacy(subcav1.PendingCSRRequest_builder{
			Kind:    types.KindPendingCSRRequest,
			Version: types.V1,
			Metadata: headerv1.Metadata_builder{
				Name: name,
			}.Build(),
		}.Build()), nil
	case types.OpPut:
		r, err := unmarshalPendingCSRRequest(event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return types.Resource153ToLegacy(r), nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}
