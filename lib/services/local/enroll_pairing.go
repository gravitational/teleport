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
	"encoding/base64"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
)

// EnrollPairingExpireDuration is the TTL of an EnrollPairing.
const EnrollPairingExpireDuration = 5 * time.Minute

// EnrollPairingService implements [services.EnrollPairing] on a [backend.Backend].
type EnrollPairingService struct {
	backend backend.Backend
}

// NewEnrollPairingService returns a new [EnrollPairingService] backed by b.
func NewEnrollPairingService(b backend.Backend) *EnrollPairingService {
	return &EnrollPairingService{backend: b}
}

// CreateEnrollPairing creates an EnrollPairing for user in the
// AWAITING_DEVICE state with a 5-minute TTL. Returns AlreadyExists
// if a pairing already exists for user.
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

	expires := s.backend.Clock().Now().UTC().Add(EnrollPairingExpireDuration)
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

	value, err := services.MarshalEnrollPairing(pairing)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	lease, err := s.backend.Create(ctx, backend.Item{
		Key:     enrollPairingKey(user),
		Value:   value,
		Expires: expires,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pairing.GetMetadata().SetRevision(lease.Revision)
	return pairing, nil
}

// GetCurrentEnrollPairing returns the EnrollPairing for user, or NotFound if no
// pairing exists.
func (s *EnrollPairingService) GetCurrentEnrollPairing(ctx context.Context, user string) (*devicepb.EnrollPairing, error) {
	if user == "" {
		return nil, trace.BadParameter("user required")
	}

	item, err := s.backend.Get(ctx, enrollPairingKey(user))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pairing, err := services.UnmarshalEnrollPairing(item.Value)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pairing.GetMetadata().SetRevision(item.Revision)
	return pairing, nil
}

func enrollPairingKey(user string) backend.Key {
	return backend.NewKey("devices", "enroll_pairing", user)
}
