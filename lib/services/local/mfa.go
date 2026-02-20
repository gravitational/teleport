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

	"github.com/gogo/protobuf/jsonpb" //nolint:depguard // needed because mfav1.ValidatedMFAChallenge uses gogoproto
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

// MFAService implements the storage layer for MFA resources.
type MFAService struct {
	logger  *slog.Logger
	service *generic.ServiceWrapper[*validatedMFAChallenge]
}

// NewMFAService returns a new MFA storage service.
func NewMFAService(b backend.Backend) (*MFAService, error) {
	svc, err := generic.NewServiceWrapper(
		generic.ServiceConfig[*validatedMFAChallenge]{
			Backend:       b,
			ResourceKind:  types.KindValidatedMFAChallenge,
			BackendPrefix: backend.NewKey(types.KindValidatedMFAChallenge),
			MarshalFunc:   MarshalValidatedMFAChallenge,
			UnmarshalFunc: UnmarshalValidatedMFAChallenge,
			ValidateFunc:  checkValidatedMFAChallenge,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &MFAService{
		logger:  slog.With(teleport.ComponentKey, "mfa.backend"),
		service: svc,
	}, nil
}

// CreateValidatedMFAChallenge persists the ValidatedMFAChallenge resource.
func (s *MFAService) CreateValidatedMFAChallenge(
	ctx context.Context,
	targetCluster string,
	chal *mfav1.ValidatedMFAChallenge,
) (*mfav1.ValidatedMFAChallenge, error) {
	switch {
	case targetCluster == "":
		return nil, trace.BadParameter("param targetCluster must not be empty")
	case chal == nil:
		return nil, trace.BadParameter("param chal must not be nil")
	}

	challenge := (*validatedMFAChallenge)(chal)

	if err := checkValidatedMFAChallenge(challenge); err != nil {
		return nil, trace.Wrap(err)
	}

	if challenge.Spec.GetTargetCluster() != targetCluster {
		return nil, trace.BadParameter("param targetCluster does not match challenge target cluster")
	}

	// Scope resources by target cluster so the backend key is target-cluster/challenge-name.
	svc := s.service.WithPrefix(targetCluster)

	// All validated MFA challenges must expire after 5 minutes.
	chal.Metadata.SetExpiry(time.Now().Add(5 * time.Minute))

	res, err := svc.CreateResource(ctx, challenge)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return (*mfav1.ValidatedMFAChallenge)(res), nil
}

// GetValidatedMFAChallenge retrieves a ValidatedMFAChallenge resource.
func (s *MFAService) GetValidatedMFAChallenge(
	ctx context.Context,
	targetCluster string,
	chalName string,
) (*mfav1.ValidatedMFAChallenge, error) {
	switch {
	case targetCluster == "":
		return nil, trace.BadParameter("param targetCluster must not be empty")
	case chalName == "":
		return nil, trace.BadParameter("param chalName must not be empty")
	}

	// Scope resources by target cluster so the backend key is target-cluster/challenge-name.
	svc := s.service.WithPrefix(targetCluster)

	res, err := svc.GetResource(ctx, chalName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return (*mfav1.ValidatedMFAChallenge)(res), nil
}

// ListValidatedMFAChallenges lists all ValidatedMFAChallenge resources.
func (s *MFAService) ListValidatedMFAChallenges(
	ctx context.Context,
	pageSize int32,
	pageToken string,
	targetCluster string,
) ([]*mfav1.ValidatedMFAChallenge, string, error) {
	svc := s.service

	if targetCluster != "" {
		// Scope listing by target cluster when provided to avoid scanning unrelated keys.
		svc = svc.WithPrefix(targetCluster)
	}

	internalChallenges, nextPageToken, err := svc.ListResources(ctx, int(pageSize), pageToken)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	challenges := make([]*mfav1.ValidatedMFAChallenge, 0, len(internalChallenges))

	for _, chal := range internalChallenges {
		challenges = append(challenges, (*mfav1.ValidatedMFAChallenge)(chal))
	}

	return challenges, nextPageToken, nil
}

// validatedMFAChallenge wraps mfav1.ValidatedMFAChallenge in order to implement the generic.ResourceMetadata interface.
// It should not be exported outside of this package to avoid confusion.
type validatedMFAChallenge mfav1.ValidatedMFAChallenge

func (r *validatedMFAChallenge) GetMetadata() *headerv1.Metadata {
	return types.LegacyTo153Metadata(*r.Metadata)
}

// MarshalValidatedMFAChallenge marshals a ValidatedMFAChallenge resource into JSON. Marshal options are currently
// unsupported.
func MarshalValidatedMFAChallenge(chal *validatedMFAChallenge, _ ...services.MarshalOption) ([]byte, error) {
	marshaler := &jsonpb.Marshaler{
		EnumsAsInts: true,
	}

	buf := &bytes.Buffer{}

	challenge := (*mfav1.ValidatedMFAChallenge)(chal)

	if err := marshaler.Marshal(buf, challenge); err != nil {
		return nil, trace.Wrap(err)
	}

	return buf.Bytes(), nil
}

// UnmarshalValidatedMFAChallenge unmarshals a ValidatedMFAChallenge resource from JSON. Unmarshal options are currently
// unsupported.
func UnmarshalValidatedMFAChallenge(b []byte, _ ...services.MarshalOption) (*validatedMFAChallenge, error) {
	unmarshaler := &jsonpb.Unmarshaler{
		AllowUnknownFields: true,
	}

	challenge := &mfav1.ValidatedMFAChallenge{}

	if err := unmarshaler.Unmarshal(bytes.NewReader(b), challenge); err != nil {
		return nil, trace.Wrap(err)
	}

	return (*validatedMFAChallenge)(challenge), nil
}

// checkValidatedMFAChallenge checks that a ValidatedMFAChallenge resource is valid.
func checkValidatedMFAChallenge(chal *validatedMFAChallenge) error {
	switch {
	case chal == nil:
		return trace.BadParameter("chal must not be nil")
	case chal.Kind != "validated_mfa_challenge":
		return trace.BadParameter("invalid kind: %q", chal.Kind)
	case chal.Version != "v1":
		return trace.BadParameter("invalid version: %q", chal.Version)
	case chal.Metadata == nil:
		return trace.BadParameter("metadata must be set")
	case chal.Metadata.Name == "":
		return trace.BadParameter("name must be set")
	case chal.Spec == nil:
		return trace.BadParameter("spec must be set")
	case chal.Spec.Payload == nil:
		return trace.BadParameter("payload must be set")
	case len(chal.Spec.Payload.GetSshSessionId()) == 0:
		return trace.BadParameter("ssh_session_id must be set")
	case chal.Spec.SourceCluster == "":
		return trace.BadParameter("source_cluster must be set")
	case chal.Spec.TargetCluster == "":
		return trace.BadParameter("target_cluster must be set")
	case chal.Spec.Username == "":
		return trace.BadParameter("username must be set")
	default:
		return nil
	}
}
