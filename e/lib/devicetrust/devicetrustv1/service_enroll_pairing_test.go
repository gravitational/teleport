package devicetrustv1_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/devicetrust/testenv"
)

func TestService_CreateEnrollPairing(t *testing.T) {
	t.Parallel()

	env := testenv.NewUsingT(t, testenv.WithAuthorizer(&userAwareAuthorizer{
		knownUsers:      []string{"alice", "bob"},
		authorizedUsers: []string{"alice", "bob"},
	}))
	devices := env.DevicesClient

	t.Run("ok", func(t *testing.T) {
		ctx := contextWithUser(t.Context(), "alice")

		createResp, err := devices.CreateEnrollPairing(ctx, &devicepb.CreateEnrollPairingRequest{})
		require.NoError(t, err)
		created := createResp.GetEnrollPairing()
		// Just a quick check since the exact fields of the struct are verified in
		// tests of lib/services/local.EnrollPairingService.
		assert.Equal(t, types.KindEnrollPairing, created.GetKind())
		assert.Equal(t, "alice", created.GetMetadata().GetName())

		getResp, err := devices.GetCurrentEnrollPairing(ctx, &devicepb.GetCurrentEnrollPairingRequest{})
		require.NoError(t, err)
		got := getResp.GetEnrollPairing()
		assert.Empty(t, cmp.Diff(created, got, protocmp.Transform()))
	})

	t.Run("returns AlreadyExists on conflict", func(t *testing.T) {
		ctx := contextWithUser(t.Context(), "bob")

		_, err := devices.CreateEnrollPairing(ctx, &devicepb.CreateEnrollPairingRequest{})
		require.NoError(t, err)

		_, err = devices.CreateEnrollPairing(ctx, &devicepb.CreateEnrollPairingRequest{})
		assert.ErrorAs(t, err, new(*trace.AlreadyExistsError))
	})
}

func TestService_GetCurrentEnrollPairing(t *testing.T) {
	t.Parallel()

	env := testenv.NewUsingT(t, testenv.WithAuthorizer(&userAwareAuthorizer{
		knownUsers:      []string{"alice", "bob"},
		authorizedUsers: []string{"alice", "bob"},
	}))
	devices := env.DevicesClient

	t.Run("returns NotFound when no pairing exists", func(t *testing.T) {
		aliceCtx := contextWithUser(t.Context(), "alice")
		_, err := devices.CreateEnrollPairing(aliceCtx, &devicepb.CreateEnrollPairingRequest{})
		require.NoError(t, err)

		bobCtx := contextWithUser(t.Context(), "bob")
		_, err = devices.GetCurrentEnrollPairing(bobCtx, &devicepb.GetCurrentEnrollPairingRequest{})
		assert.ErrorAs(t, err, new(*trace.NotFoundError))
	})
}
