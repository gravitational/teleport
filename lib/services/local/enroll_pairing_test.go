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
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services/local"
)

func newEnrollPairingService(t *testing.T, clock clockwork.Clock) *local.EnrollPairingService {
	t.Helper()
	bk, err := memory.New(memory.Config{
		Context: t.Context(),
		Clock:   clock,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = bk.Close() })
	return local.NewEnrollPairingService(bk)
}

func TestEnrollPairingService_CreateEnrollPairing(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClock()
	s := newEnrollPairingService(t, clock)

	t.Run("ok", func(t *testing.T) {
		pairing, err := s.CreateEnrollPairing(t.Context(), "create-ok")
		require.NoError(t, err)
		assert.Equal(t, types.KindEnrollPairing, pairing.GetKind())
		assert.Equal(t, types.V1, pairing.GetVersion())
		assert.Equal(t, "create-ok", pairing.GetMetadata().GetName())
		assert.NotEmpty(t, pairing.GetMetadata().GetRevision())
		assert.Equal(t,
			devicepb.EnrollPairingState_ENROLL_PAIRING_STATE_AWAITING_DEVICE,
			pairing.GetStatus().GetState())
		assert.NotEmpty(t, pairing.GetStatus().GetToken())
	})

	t.Run("returns AlreadyExists when a pairing is already active", func(t *testing.T) {
		ctx := t.Context()
		_, err := s.CreateEnrollPairing(ctx, "create-conflict")
		require.NoError(t, err)

		_, err = s.CreateEnrollPairing(ctx, "create-conflict")
		assert.True(t, trace.IsAlreadyExists(err), "got %v", err)
	})

	t.Run("distinct users get distinct pairings", func(t *testing.T) {
		ctx := t.Context()
		alice, err := s.CreateEnrollPairing(ctx, "create-alice")
		require.NoError(t, err)

		bob, err := s.CreateEnrollPairing(ctx, "create-bob")
		require.NoError(t, err)
		assert.NotEqual(t, alice.GetStatus().GetToken(), bob.GetStatus().GetToken())
	})

	t.Run("rejects empty user", func(t *testing.T) {
		_, err := s.CreateEnrollPairing(t.Context(), "")
		assert.True(t, trace.IsBadParameter(err), "got %v", err)
	})

	t.Run("creates a fresh pairing after the existing one expires", func(t *testing.T) {
		ctx := t.Context()
		first, err := s.CreateEnrollPairing(ctx, "create-after-ttl")
		require.NoError(t, err)

		clock.Advance(local.EnrollPairingExpireDuration + time.Second)

		fresh, err := s.CreateEnrollPairing(ctx, "create-after-ttl")
		require.NoError(t, err)
		assert.NotEqual(t, first.GetStatus().GetToken(), fresh.GetStatus().GetToken())
	})
}

func TestEnrollPairingService_GetCurrentEnrollPairing(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClock()
	s := newEnrollPairingService(t, clock)

	t.Run("returns the existing pairing", func(t *testing.T) {
		ctx := t.Context()
		created, err := s.CreateEnrollPairing(ctx, "get-existing")
		require.NoError(t, err)

		got, err := s.GetCurrentEnrollPairing(ctx, "get-existing")
		require.NoError(t, err)
		assert.Equal(t, created.GetStatus().GetToken(), got.GetStatus().GetToken())
		assert.Equal(t, created.GetStatus().GetState(), got.GetStatus().GetState())
	})

	t.Run("returns NotFound when no pairing exists", func(t *testing.T) {
		_, err := s.GetCurrentEnrollPairing(t.Context(), "get-missing")
		assert.True(t, trace.IsNotFound(err), "got %v", err)
	})

	t.Run("rejects empty user", func(t *testing.T) {
		_, err := s.GetCurrentEnrollPairing(t.Context(), "")
		assert.True(t, trace.IsBadParameter(err), "got %v", err)
	})

	t.Run("returns NotFound after the pairing expires", func(t *testing.T) {
		ctx := t.Context()
		_, err := s.CreateEnrollPairing(ctx, "get-expired")
		require.NoError(t, err)

		clock.Advance(local.EnrollPairingExpireDuration + time.Second)

		_, err = s.GetCurrentEnrollPairing(ctx, "get-expired")
		assert.True(t, trace.IsNotFound(err), "got %v", err)
	})
}
