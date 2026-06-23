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

package local_test

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services/local"
)

func newEnrollPairingService(t *testing.T) *local.EnrollPairingService {
	t.Helper()
	bk, err := memory.New(memory.Config{
		Context: t.Context(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = bk.Close() })

	service, err := local.NewEnrollPairingService(bk)
	require.NoError(t, err)
	return service
}

func TestEnrollPairingService_CreateEnrollPairing(t *testing.T) {
	t.Parallel()

	s := newEnrollPairingService(t)

	t.Run("ok", func(t *testing.T) {
		pairing, err := s.CreateEnrollPairing(t.Context(), "create-ok")
		require.NoError(t, err)

		want := devicepb.EnrollPairing_builder{
			Kind:    types.KindEnrollPairing,
			Version: types.V1,
			Metadata: headerv1.Metadata_builder{
				Name:     "create-ok",
				Revision: pairing.GetMetadata().GetRevision(),
				Expires:  pairing.GetMetadata().GetExpires(),
			}.Build(),
			Spec: devicepb.EnrollPairingSpec_builder{}.Build(),
			Status: devicepb.EnrollPairingStatus_builder{
				State: devicepb.EnrollPairingState_ENROLL_PAIRING_STATE_AWAITING_DEVICE,
				Token: pairing.GetStatus().GetToken(),
			}.Build(),
		}.Build()
		assert.Empty(t, cmp.Diff(want, pairing, protocmp.Transform()))
		assert.NotEmpty(t, pairing.GetMetadata().GetRevision())
		assert.NotEmpty(t, pairing.GetStatus().GetToken())
		assert.WithinDuration(t,
			time.Now().Add(local.EnrollPairingExpireDuration),
			pairing.GetMetadata().GetExpires().AsTime(),
			time.Second)
	})

	t.Run("distinct users get distinct pairings", func(t *testing.T) {
		ctx := t.Context()
		alice, err := s.CreateEnrollPairing(ctx, "create-alice")
		require.NoError(t, err)

		bob, err := s.CreateEnrollPairing(ctx, "create-bob")
		require.NoError(t, err)
		assert.NotEqual(t, alice.GetStatus().GetToken(), bob.GetStatus().GetToken())
	})

	t.Run("creates a fresh pairing after the existing one expires", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			s := newEnrollPairingService(t)

			ctx := t.Context()
			want, err := s.CreateEnrollPairing(ctx, "create-after-ttl")
			require.NoError(t, err)

			time.Sleep(local.EnrollPairingExpireDuration + time.Second)

			got, err := s.CreateEnrollPairing(ctx, "create-after-ttl")
			require.NoError(t, err)
			// Verify the pairings are equal except the token and certain metadata.
			assert.NotEqual(t, want.GetStatus().GetToken(), got.GetStatus().GetToken())
			assert.Empty(t, cmp.Diff(want, got,
				protocmp.IgnoreFields(&headerv1.Metadata{}, "revision", "expires"),
				protocmp.IgnoreFields(&devicepb.EnrollPairingStatus{}, "token"),
				protocmp.Transform(),
			))
		})
	})

	t.Run("rejects empty user", func(t *testing.T) {
		_, err := s.CreateEnrollPairing(t.Context(), "")
		assert.ErrorAs(t, err, new(*trace.BadParameterError))
	})
}

func TestEnrollPairingService_GetCurrentEnrollPairing(t *testing.T) {
	t.Parallel()

	s := newEnrollPairingService(t)

	t.Run("returns the existing pairing", func(t *testing.T) {
		ctx := t.Context()
		want, err := s.CreateEnrollPairing(ctx, "get-existing")
		require.NoError(t, err)

		got, err := s.GetCurrentEnrollPairing(ctx, "get-existing")
		require.NoError(t, err)
		assert.Empty(t, cmp.Diff(want, got, protocmp.Transform()))
	})

	t.Run("returns NotFound when no pairing exists", func(t *testing.T) {
		_, err := s.GetCurrentEnrollPairing(t.Context(), "get-missing")
		assert.ErrorAs(t, err, new(*trace.NotFoundError))
	})

	t.Run("rejects empty user", func(t *testing.T) {
		_, err := s.GetCurrentEnrollPairing(t.Context(), "")
		assert.ErrorAs(t, err, new(*trace.BadParameterError))
	})
}
