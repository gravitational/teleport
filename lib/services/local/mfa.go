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
	"context"
	"log/slog"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	mfav2 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v2"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

// ValidatedMFAChallengeExpiry is the TTL for ValidatedMFAChallenge resources. This value is chosen to be long enough to
// allow for replication to leaf clusters and retries of replication in the event of transient errors, but short enough
// to ensure that stale challenges don't persist indefinitely if they fail to replicate.
const ValidatedMFAChallengeExpiry = 5 * time.Minute

// MFAService implements the storage layer for MFA resources.
type MFAService struct {
	logger  *slog.Logger
	service *generic.ServiceWrapper[*mfav2.ValidatedMFAChallenge]
}

// NewMFAService returns a new MFA storage service.
func NewMFAService(b backend.Backend) (*MFAService, error) {
	svc, err := generic.NewServiceWrapper(
		generic.ServiceConfig[*mfav2.ValidatedMFAChallenge]{
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
	chal *mfav2.ValidatedMFAChallenge,
) (*mfav2.ValidatedMFAChallenge, error) {
	switch {
	case targetCluster == "":
		return nil, trace.BadParameter("param targetCluster must not be empty")
	case chal == nil:
		return nil, trace.BadParameter("param chal must not be nil")
	}

	if err := checkValidatedMFAChallenge(chal); err != nil {
		return nil, trace.Wrap(err)
	}

	if chal.GetSpec().GetTargetCluster() != targetCluster {
		return nil, trace.BadParameter("param targetCluster does not match challenge target cluster")
	}

	// Scope resources by target cluster so the backend key is target-cluster/challenge-name.
	svc := s.service.WithPrefix(targetCluster)

	// All validated MFA challenges must expire after 5 minutes.
	chal.GetMetadata().SetExpires(timestamppb.New(time.Now().Add(ValidatedMFAChallengeExpiry)))

	res, err := svc.CreateResource(ctx, chal)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return res, nil
}

// GetValidatedMFAChallenge retrieves a ValidatedMFAChallenge resource.
func (s *MFAService) GetValidatedMFAChallenge(
	ctx context.Context,
	targetCluster string,
	chalName string,
) (*mfav2.ValidatedMFAChallenge, error) {
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

	return res, nil
}

// ListValidatedMFAChallenges lists all ValidatedMFAChallenge resources.
func (s *MFAService) ListValidatedMFAChallenges(
	ctx context.Context,
	pageSize int32,
	pageToken string,
	targetCluster string,
) ([]*mfav2.ValidatedMFAChallenge, string, error) {
	svc := s.service

	if targetCluster != "" {
		// Scope listing by target cluster when provided to avoid scanning unrelated keys.
		svc = svc.WithPrefix(targetCluster)
	}

	internalChallenges, nextPageToken, err := svc.ListResources(ctx, int(pageSize), pageToken)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	return internalChallenges, nextPageToken, nil
}

// MarshalValidatedMFAChallenge marshals a ValidatedMFAChallenge resource into JSON.
func MarshalValidatedMFAChallenge(chal *mfav2.ValidatedMFAChallenge, _ ...services.MarshalOption) ([]byte, error) {
	marshaler := &protojson.MarshalOptions{
		EmitUnpopulated: false,
		UseProtoNames:   true,
	}

	return marshaler.Marshal(chal)
}

// UnmarshalValidatedMFAChallenge unmarshals a ValidatedMFAChallenge resource from JSON.
func UnmarshalValidatedMFAChallenge(b []byte, opts ...services.MarshalOption) (*mfav2.ValidatedMFAChallenge, error) {
	unmarshaler := &protojson.UnmarshalOptions{
		AllowPartial:   false,
		DiscardUnknown: true,
	}

	challenge := &mfav2.ValidatedMFAChallenge{}

	if err := unmarshaler.Unmarshal(b, challenge); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := services.CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.Revision != "" || !cfg.Expires.IsZero() {
		metadata := challenge.GetMetadata()
		if metadata == nil {
			metadata = &headerv1.Metadata{}
			challenge.SetMetadata(metadata)
		}

		if cfg.Revision != "" {
			metadata.SetRevision(cfg.Revision)
		}

		if !cfg.Expires.IsZero() {
			metadata.SetExpires(timestamppb.New(cfg.Expires))
		}
	}

	return challenge, nil
}

// checkValidatedMFAChallenge checks that a ValidatedMFAChallenge resource is valid.
func checkValidatedMFAChallenge(chal *mfav2.ValidatedMFAChallenge) error {
	switch {
	case chal == nil:
		return trace.BadParameter("chal must not be nil")
	case chal.GetKind() != "validated_mfa_challenge":
		return trace.BadParameter("invalid kind: %q", chal.GetKind())
	case chal.GetVersion() != types.V1:
		return trace.BadParameter("invalid version: %q", chal.GetVersion())
	case chal.GetMetadata() == nil:
		return trace.BadParameter("metadata must be set")
	case chal.GetMetadata().GetName() == "":
		return trace.BadParameter("name must be set")
	case chal.GetSpec() == nil:
		return trace.BadParameter("spec must be set")
	case chal.GetSpec().GetPayload() == nil:
		return trace.BadParameter("payload must be set")
	case len(chal.GetSpec().GetPayload().GetSshSessionId()) == 0:
		return trace.BadParameter("ssh_session_id must be set")
	case chal.GetSpec().GetSourceCluster() == "":
		return trace.BadParameter("source_cluster must be set")
	case chal.GetSpec().GetTargetCluster() == "":
		return trace.BadParameter("target_cluster must be set")
	case chal.GetSpec().GetUsername() == "":
		return trace.BadParameter("username must be set")
	default:
		return nil
	}
}

type validatedMFAChallengeParser struct {
	baseParser
}

func newValidatedMFAChallengeParser() *validatedMFAChallengeParser {
	return &validatedMFAChallengeParser{
		baseParser: newBaseParser(backend.ExactKey(types.KindValidatedMFAChallenge)),
	}
}

func (p *validatedMFAChallengeParser) parse(event backend.Event) (types.Resource, error) {
	var chal *mfav2.ValidatedMFAChallenge

	switch event.Type {
	case types.OpDelete:
		// Inflate key components into a challenge so consumers can access the backend key structure directly from the
		// concrete resource type.
		key := event.Item.Key.TrimPrefix(backend.NewKey(types.KindValidatedMFAChallenge))

		keyComponents := key.Components()
		if len(keyComponents) < 2 {
			return nil, trace.NotFound("failed parsing %v", event.Item.Key.String())
		}

		chal = mfav2.ValidatedMFAChallenge_builder{
			Kind:    types.KindValidatedMFAChallenge,
			Version: types.V1,
			Metadata: headerv1.Metadata_builder{
				Name: keyComponents[len(keyComponents)-1], // The challenge name is the last component of the key.
			}.Build(),
			Spec: mfav2.ValidatedMFAChallengeSpec_builder{
				TargetCluster: keyComponents[0], // The target cluster is the first component of the key.
			}.Build(),
		}.Build()

	case types.OpPut:
		var err error
		chal, err = UnmarshalValidatedMFAChallenge(
			event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}

	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}

	return types.ProtoResource153ToLegacy(chal), nil
}
