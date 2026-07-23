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
	"github.com/google/uuid"
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

func TestEnrollPairingService_GetEnrollPairingByToken(t *testing.T) {
	t.Parallel()

	s := newEnrollPairingService(t)

	t.Run("returns the pairing matching the token", func(t *testing.T) {
		ctx := t.Context()
		want, err := s.CreateEnrollPairing(ctx, "by-token-ok")
		require.NoError(t, err)

		got, err := s.GetEnrollPairingByToken(ctx, want.GetStatus().GetToken())
		require.NoError(t, err)
		assert.Empty(t, cmp.Diff(want, got, protocmp.Transform()))
	})

	t.Run("returns NotFound for an unknown token", func(t *testing.T) {
		_, err := s.GetEnrollPairingByToken(t.Context(), "does-not-exist")
		assert.ErrorAs(t, err, new(*trace.NotFoundError))
	})

	t.Run("rejects empty token", func(t *testing.T) {
		_, err := s.GetEnrollPairingByToken(t.Context(), "")
		assert.ErrorAs(t, err, new(*trace.BadParameterError))
	})
}

func TestEnrollPairingService_RequestEnrollPairingApproval(t *testing.T) {
	t.Parallel()

	s := newEnrollPairingService(t)
	device := makeDevice()

	t.Run("transitions to awaiting approval and persists the device", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		created, err := s.CreateEnrollPairing(ctx, "approve-ok")
		require.NoError(t, err)

		updated, err := s.RequestEnrollPairingApproval(ctx, created, device)
		require.NoError(t, err)
		assert.Equal(t,
			devicepb.EnrollPairingState_ENROLL_PAIRING_STATE_AWAITING_APPROVAL,
			updated.GetStatus().GetState())
		assert.Empty(t, cmp.Diff(device, updated.GetStatus().GetDevice(), protocmp.Transform()))

		// The transition is persisted and the pairing is still resolvable by token.
		got, err := s.GetEnrollPairingByToken(ctx, created.GetStatus().GetToken())
		require.NoError(t, err)
		assert.Empty(t, cmp.Diff(updated, got, protocmp.Transform()))
	})

	t.Run("rejects a pairing that is no longer awaiting a device", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		created, err := s.CreateEnrollPairing(ctx, "approve-twice")
		require.NoError(t, err)

		_, err = s.RequestEnrollPairingApproval(ctx, created, device)
		require.NoError(t, err)

		// created has advanced past AWAITING_DEVICE, so a second attempt is rejected.
		_, err = s.RequestEnrollPairingApproval(ctx, created, device)
		assert.ErrorAs(t, err, new(*trace.CompareFailedError))
	})

	t.Run("rejects a stale pairing that lost the compare-and-swap", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		created, err := s.CreateEnrollPairing(ctx, "approve-stale")
		require.NoError(t, err)

		// Claim via a fresh copy, advancing the stored revision so that created,
		// still AWAITING_DEVICE in memory, no longer matches the backend.
		fresh, err := s.GetEnrollPairingByToken(ctx, created.GetStatus().GetToken())
		require.NoError(t, err)
		_, err = s.RequestEnrollPairingApproval(ctx, fresh, device)
		require.NoError(t, err)

		_, err = s.RequestEnrollPairingApproval(ctx, created, device)
		assert.ErrorAs(t, err, new(*trace.CompareFailedError))
	})

	t.Run("rejects a nil pairing", func(t *testing.T) {
		t.Parallel()
		_, err := s.RequestEnrollPairingApproval(t.Context(), nil, device)
		assert.ErrorAs(t, err, new(*trace.BadParameterError))
	})

	badOSType := makeDevice()
	badOSType.SetOsType(devicepb.OSType_OS_TYPE_UNSPECIFIED)
	badOSVersion := makeDevice()
	badOSVersion.SetOsVersion(" ")
	badSerialNumber := makeDevice()
	badSerialNumber.SetSerialNumber(" ")

	badParameterTests := []struct {
		name   string
		device *devicepb.EnrollPairingDevice
		errMsg string
	}{
		{
			name:   "rejects a nil device",
			device: nil,
			errMsg: "device required",
		},
		{
			name:   "rejects empty device OS type",
			device: badOSType,
			errMsg: "os_type is missing",
		},
		{
			name:   "rejects empty device OS version",
			device: badOSVersion,
			errMsg: "os_version is missing",
		},
		{
			name:   "rejects empty device serial number",
			device: badSerialNumber,
			errMsg: "serial_number is missing",
		},
	}
	for _, test := range badParameterTests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			created, err := s.CreateEnrollPairing(ctx, uuid.NewString())
			require.NoError(t, err)

			_, err = s.RequestEnrollPairingApproval(ctx, created, test.device)
			assert.ErrorAs(t, err, new(*trace.BadParameterError))
			assert.ErrorContains(t, err, test.errMsg)
		})
	}
}

func makeDevice() *devicepb.EnrollPairingDevice {
	return devicepb.EnrollPairingDevice_builder{
		OsType:       devicepb.OSType_OS_TYPE_IOS,
		SerialNumber: "CXXXXXXXXX01",
		OsVersion:    "26.3.1",
	}.Build()
}
