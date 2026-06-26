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
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"log/slog"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

// EnrollPairingExpireDuration is the TTL of an EnrollPairing.
const EnrollPairingExpireDuration = 5 * time.Minute

// EnrollPairingService implements [services.EnrollPairing] on a [backend.Backend].
type EnrollPairingService struct {
	service *generic.ServiceWrapper[*devicepb.EnrollPairing]
	backend backend.Backend
	clock   clockwork.Clock
	log     *slog.Logger
}

// NewEnrollPairingService returns a new [EnrollPairingService] backed by b.
func NewEnrollPairingService(b backend.Backend) (*EnrollPairingService, error) {
	service, err := generic.NewServiceWrapper(generic.ServiceConfig[*devicepb.EnrollPairing]{
		Backend:       b,
		ResourceKind:  types.KindEnrollPairing,
		BackendPrefix: backend.NewKey("devices", "enroll_pairing"),
		MarshalFunc:   services.MarshalEnrollPairing,
		UnmarshalFunc: services.UnmarshalEnrollPairing,
		ValidateFunc:  validateEnrollPairing,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &EnrollPairingService{
		clock:   b.Clock(),
		service: service,
		backend: b,
		log:     slog.With(teleport.ComponentKey, teleport.Component("enrollpairing")),
	}, nil
}

// CreateEnrollPairing creates an EnrollPairing for user in the AWAITING_DEVICE
// state with a 5-minute TTL. Returns AlreadyExists if a pairing already exists
// for user.
func (s *EnrollPairingService) CreateEnrollPairing(ctx context.Context, user string) (*devicepb.EnrollPairing, error) {
	if user == "" {
		return nil, trace.BadParameter("user required")
	}

	const tokenLen = 32
	tokenRaw := make([]byte, tokenLen)
	if _, err := rand.Read(tokenRaw); err != nil {
		return nil, trace.Wrap(err, "generating enroll pairing token")
	}
	token := base64.RawURLEncoding.EncodeToString(tokenRaw)

	expires := s.clock.Now().UTC().Add(EnrollPairingExpireDuration)
	pairing := devicepb.EnrollPairing_builder{
		Kind:    types.KindEnrollPairing,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name:    user,
			Expires: timestamppb.New(expires),
		}.Build(),
		Spec: &devicepb.EnrollPairingSpec{},
		Status: devicepb.EnrollPairingStatus_builder{
			State: devicepb.EnrollPairingState_ENROLL_PAIRING_STATE_AWAITING_DEVICE,
			Token: token,
		}.Build(),
	}.Build()

	// HACK(ravicious): Attempt to clear an existing resource before creating a
	// new one.
	//
	// On some backends like DynamoDB and Firestore, expired items are not removed
	// immediately from the backend. In the case of DynamoDB, expired items are
	// removed within a few days [1].
	//
	// Our backends are not consistent when it comes to dealing with expired items
	// on Create. For DynamoDB, that was just addressed in #68038. [2]
	//
	// As a workaround, before creating a resource we first get the resource by
	// name to trigger removal of an expired item if it exists.
	//
	// Once #68038 lands and the issue is addressed in Firestore backend too, we
	// can remove this workaround.
	//
	// [1]: https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/TTL.html
	// [2]: https://github.com/gravitational/teleport/pull/68038
	if _, err := s.service.GetResource(ctx, user); err != nil && !trace.IsNotFound(err) {
		s.log.WarnContext(ctx, "Failed to clear expired pairing if any", "error", err)
	}

	if err := validateEnrollPairing(pairing); err != nil {
		return nil, trace.Wrap(err)
	}
	pairingItem, err := s.service.MakeBackendItem(pairing)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	indexItem := backend.Item{
		Key:     enrollPairingByTokenKey(token),
		Value:   []byte(user),
		Expires: expires,
	}

	revision, err := s.backend.AtomicWrite(ctx, []backend.ConditionalAction{
		{
			Key:       pairingItem.Key,
			Condition: backend.NotExists(),
			Action:    backend.Put(pairingItem),
		},
		{
			Key:       enrollPairingByTokenKey(token),
			Condition: backend.NotExists(),
			Action:    backend.Put(indexItem),
		},
	})
	if err != nil {
		if errors.Is(err, backend.ErrConditionFailed) {
			return nil, trace.AlreadyExists("enroll pairing for user %q already exists", user)
		}
		return nil, trace.Wrap(err)
	}

	pairing.GetMetadata().SetRevision(revision)
	return pairing, nil
}

// GetCurrentEnrollPairing returns the EnrollPairing for user, or NotFound if no
// pairing exists.
func (s *EnrollPairingService) GetCurrentEnrollPairing(ctx context.Context, user string) (*devicepb.EnrollPairing, error) {
	if user == "" {
		return nil, trace.BadParameter("user required")
	}

	pairing, err := s.service.GetResource(ctx, user)
	return pairing, trace.Wrap(err)
}

// GetEnrollPairingByToken returns the EnrollPairing whose status token matches
// token, or NotFound if none matches.
func (s *EnrollPairingService) GetEnrollPairingByToken(ctx context.Context, token string) (*devicepb.EnrollPairing, error) {
	if token == "" {
		return nil, trace.BadParameter("token required")
	}

	item, err := s.backend.Get(ctx, enrollPairingByTokenKey(token))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pairing, err := s.service.GetResource(ctx, string(item.Value))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Verify that the index and the pairing have the same token, in case the
	// pairing was updated between the two Gets.
	if pairing.GetStatus().GetToken() != token {
		return nil, trace.NotFound("enroll pairing not found")
	}

	return pairing, nil
}

// RequestEnrollPairingApproval transitions the pairing identified by token from
// AWAITING_DEVICE to AWAITING_APPROVAL, persisting device, and returns the
// updated pairing.
func (s *EnrollPairingService) RequestEnrollPairingApproval(ctx context.Context, token string, device *devicepb.EnrollPairingDevice) (*devicepb.EnrollPairing, error) {
	if token == "" {
		return nil, trace.BadParameter("token required")
	}
	if device == nil {
		return nil, trace.BadParameter("device required")
	}

	pairing, err := s.GetEnrollPairingByToken(ctx, token)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if pairing.GetStatus().GetState() != devicepb.EnrollPairingState_ENROLL_PAIRING_STATE_AWAITING_DEVICE {
		return nil, trace.CompareFailed("enroll pairing is not awaiting a device")
	}

	pairing.GetStatus().SetState(devicepb.EnrollPairingState_ENROLL_PAIRING_STATE_AWAITING_APPROVAL)
	pairing.GetStatus().SetDevice(device)

	updated, err := s.service.ConditionalUpdateResource(ctx, pairing)
	if trace.IsCompareFailed(err) {
		// Lost the CAS to a concurrent claim — same outcome as the state check above.
		return nil, trace.CompareFailed("enroll pairing is not awaiting a device")
	}
	return updated, trace.Wrap(err)
}

func enrollPairingByTokenKey(token string) backend.Key {
	hash := sha256.Sum256([]byte(token))
	return backend.NewKey("devices", "enroll_pairing_by_token", hex.EncodeToString(hash[:]))
}

func validateEnrollPairing(pairing *devicepb.EnrollPairing) error {
	if pairing.GetMetadata().GetName() == "" {
		return trace.BadParameter("enroll pairing metadata.name is missing")
	}
	if !pairing.HasStatus() {
		return trace.BadParameter("enroll pairing status is missing")
	}
	if pairing.GetStatus().GetToken() == "" {
		return trace.BadParameter("enroll pairing status.token is missing")
	}
	if pairing.GetStatus().GetState() == devicepb.EnrollPairingState_ENROLL_PAIRING_STATE_UNSPECIFIED {
		return trace.BadParameter("enroll pairing status.state is missing")
	}
	return nil
}
