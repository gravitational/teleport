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
package local

import (
	"bytes"
	"context"
	"log/slog"
	"time"

	"github.com/gogo/protobuf/jsonpb" //nolint:depguard // needed for backwards compatibility

	"github.com/gravitational/teleport"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
	"github.com/gravitational/trace"
)

// ValidatedMFAChallengeService implements the storage layer for the ValidatedMFAChallenge resource.
type ValidatedMFAChallengeService struct {
	svc    *generic.ServiceWrapper[*validatedMFAChallenge]
	logger *slog.Logger
}

// NewValidatedMFAChallengeService returns a new ValidatedMFAChallenge storage service.
func NewValidatedMFAChallengeService(b backend.Backend) (*ValidatedMFAChallengeService, error) {
	svc, err := generic.NewServiceWrapper(
		generic.ServiceConfig[*validatedMFAChallenge]{
			Backend:       b,
			ResourceKind:  types.KindValidatedMFAChallenge,
			BackendPrefix: backend.NewKey(types.KindValidatedMFAChallenge),
			MarshalFunc:   MarshalValidatedMFAChallenge,
			UnmarshalFunc: UnmarshalValidatedMFAChallenge,
			ValidateFunc:  validateValidatedMFAChallenge,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &ValidatedMFAChallengeService{
		svc:    svc,
		logger: slog.With(teleport.ComponentKey, "ValidatedMFAChallenge.local"), // TODO: What should the component key be?
	}, nil
}

// CreateValidatedMFAChallenge persists the ValidatedMFAChallenge resource.
func (s *ValidatedMFAChallengeService) CreateValidatedMFAChallenge(
	ctx context.Context,
	username string,
	chal *mfav1.ValidatedMFAChallenge,
) (*mfav1.ValidatedMFAChallenge, error) {
	switch {
	case username == "":
		return nil, trace.BadParameter("param username is empty")
	case chal == nil:
		return nil, trace.BadParameter("param chal is nil")
	}

	// Scope the service to the given username, so that the resource is created under that user's prefix.
	// TODO: Copying can be expensive at scale, consult with team if this is acceptable or if there's a better way.
	svc := s.svc.WithPrefix(username)

	// All validated MFA challenges must expire after 5 minutes.
	chal.Metadata.SetExpiry(time.Now().Add(5 * time.Minute))

	res, err := svc.CreateResource(ctx, (*validatedMFAChallenge)(chal))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return (*mfav1.ValidatedMFAChallenge)(res), nil
}

// GetValidatedMFAChallenge retrieves a ValidatedMFAChallenge resource.
func (s *ValidatedMFAChallengeService) GetValidatedMFAChallenge(
	ctx context.Context,
	username string,
	chalName string,
) (*mfav1.ValidatedMFAChallenge, error) {
	switch {
	case username == "":
		return nil, trace.BadParameter("param username is empty")
	case chalName == "":
		return nil, trace.BadParameter("param chalName is empty")
	}

	// Scope the service to the given username, so that the resource is created under that user's prefix.
	// TODO: Copying can be expensive at scale, consult with team if this is acceptable or if there's a better way.
	svc := s.svc.WithPrefix(username)

	res, err := svc.GetResource(ctx, chalName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return (*mfav1.ValidatedMFAChallenge)(res), nil
}

// validatedMFAChallenge is an alias to mfav1.validatedMFAChallenge in order to satisfy the generic.ResourceMetadata
// interface. It should not be exported outside of this package to avoid confusion.
type validatedMFAChallenge mfav1.ValidatedMFAChallenge

func (r *validatedMFAChallenge) GetMetadata() *headerv1.Metadata {
	return types.LegacyTo153Metadata(*r.Metadata)
}

// MarshalValidatedMFAChallenge marshals a ValidatedMFAChallenge resource into JSON.
func MarshalValidatedMFAChallenge(chal *validatedMFAChallenge, opts ...services.MarshalOption) ([]byte, error) {
	buf := &bytes.Buffer{}

	if err := (&jsonpb.Marshaler{}).Marshal(buf, (*mfav1.ValidatedMFAChallenge)(chal)); err != nil {
		return nil, trace.Wrap(err)
	}

	return buf.Bytes(), nil
}

// UnmarshalValidatedMFAChallenge unmarshals a ValidatedMFAChallenge resource from JSON.
func UnmarshalValidatedMFAChallenge(b []byte, opts ...services.MarshalOption) (*validatedMFAChallenge, error) {
	chal := &mfav1.ValidatedMFAChallenge{}

	if err := (&jsonpb.Unmarshaler{AllowUnknownFields: true}).Unmarshal(bytes.NewReader(b), chal); err != nil {
		return nil, trace.Wrap(err)
	}

	return (*validatedMFAChallenge)(chal), nil
}

// validateValidatedMFAChallenge validates a ValidatedMFAChallenge resource.
func validateValidatedMFAChallenge(chal *validatedMFAChallenge) error {
	switch {
	case chal == nil:
		return trace.BadParameter("chal must not be nil")
	case chal.Kind != "validated_mfa_challenge":
		return trace.BadParameter("invalid kind: %q", chal.Kind)
	case chal.Version != "v1":
		return trace.BadParameter("invalid version: %q", chal.Version)
	case chal.Metadata.Name == "":
		return trace.BadParameter("name must be set")
	case chal.Spec.Payload == nil:
		return trace.BadParameter("payload must be set")
	case len(chal.Spec.Payload.GetSshSessionId()) == 0:
		return trace.BadParameter("ssh_session_id must be set")
	case chal.Spec.SourceCluster == "":
		return trace.BadParameter("source_cluster must be set")
	case chal.Spec.TargetCluster == "":
		return trace.BadParameter("target_cluster must be set")
	default:
		return nil
	}
}
