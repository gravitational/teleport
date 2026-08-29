package devicetrustv1_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/e/lib/devicetrust/testenv"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
)

func TestService_CreateEnrollPairing(t *testing.T) {
	t.Parallel()

	env := testenv.NewUsingT(t, testenv.WithAuthorizer(newUserAwareAuthorizer(
		withKnownUsers("alice", "bob"),
		withAuthorizedUsers("alice", "bob"),
	)))
	devices := env.DevicesClient

	t.Run("ok", func(t *testing.T) {
		t.Parallel()
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
		t.Parallel()
		ctx := contextWithUser(t.Context(), "bob")

		_, err := devices.CreateEnrollPairing(ctx, &devicepb.CreateEnrollPairingRequest{})
		require.NoError(t, err)

		_, err = devices.CreateEnrollPairing(ctx, &devicepb.CreateEnrollPairingRequest{})
		assert.ErrorAs(t, err, new(*trace.AlreadyExistsError))
	})
}

func TestService_GetCurrentEnrollPairing(t *testing.T) {
	t.Parallel()

	env := testenv.NewUsingT(t, testenv.WithAuthorizer(newUserAwareAuthorizer(
		withKnownUsers("alice", "bob"),
		withAuthorizedUsers("alice", "bob"),
	)))
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

func TestService_ApproveEnrollPairing(t *testing.T) {
	t.Parallel()

	emitter := &testenv.KeyedEmitter{}
	env := testenv.NewUsingT(t,
		testenv.WithAuthorizer(newUserAwareAuthorizer(
			withKnownUsers("alice", "bob", "dave", "erin"),
			withAuthorizedUsers("alice", "bob", "dave", "erin"),
		)),
		testenv.WithEmitter(emitter))
	devices := env.DevicesClient

	t.Run("moves a claimed pairing to approved", func(t *testing.T) {
		t.Parallel()
		ctx := configureOutgoingContext(t.Context(), outgoingContextParams{
			User:       "alice",
			EmitterKey: "alice",
		})
		token := claimPairing(t, ctx, env)

		_, err := devices.ApproveEnrollPairing(ctx, devicepb.ApproveEnrollPairingRequest_builder{
			PairingToken: token,
		}.Build())
		require.NoError(t, err)

		got, err := env.EnrollPairing.GetEnrollPairingByToken(t.Context(), token)
		require.NoError(t, err)
		assert.Equal(t,
			devicepb.EnrollPairingState_ENROLL_PAIRING_STATE_APPROVED,
			got.GetStatus().GetState(),
		)

		// Audit assertions here and in the tests below are hand-rolled instead of
		// going through [assertEvents], which requires user metadata that the test
		// env does not populate and a device ID that a pairing cannot report, as it
		// only carries the device data self-reported by the app and never resolves
		// it to a registered device.
		evt, ok := emitter.LastEvent("alice").(*apievents.DeviceEvent2)
		require.True(t, ok, "expected a DeviceEvent2, got %T", emitter.LastEvent("alice"))
		assert.Equal(t, events.DeviceEnrollPairingApproveEvent, evt.GetType())
		assert.Equal(t, events.DeviceEnrollPairingApproveCode, evt.GetCode())
		assert.True(t, evt.Status.Success)

		require.NotNil(t, evt.Device)
		assert.Equal(t, "CXXXXXXXXX01", evt.Device.AssetTag)
		assert.Equal(t, apievents.OSType_OS_TYPE_IOS, evt.Device.OsType)
	})

	t.Run("returns NotFound when the token does not match the current pairing", func(t *testing.T) {
		t.Parallel()
		ctx := configureOutgoingContext(t.Context(), outgoingContextParams{
			User:       "bob",
			EmitterKey: "bob",
		})
		claimPairing(t, ctx, env)

		_, err := devices.ApproveEnrollPairing(ctx, devicepb.ApproveEnrollPairingRequest_builder{
			PairingToken: "does-not-match-pairing",
		}.Build())
		assert.ErrorAs(t, err, new(*trace.NotFoundError))
		assert.ErrorContains(t, err, "pairing token does not match the user's current enroll pairing")

		evt, ok := emitter.LastEvent("bob").(*apievents.DeviceEvent2)
		require.True(t, ok, "expected a DeviceEvent2, got %T", emitter.LastEvent("bob"))
		assert.Equal(t, events.DeviceEnrollPairingApproveFailureCode, evt.GetCode())
		assert.False(t, evt.Status.Success)
		assert.NotEmpty(t, evt.Status.Error)
		// The pairing itself was fetched, only the token mismatched, so the
		// event still identifies the device whose enrollment was pending.
		require.NotNil(t, evt.Device)
		assert.Equal(t, "CXXXXXXXXX01", evt.Device.AssetTag)
		assert.Equal(t, apievents.OSType_OS_TYPE_IOS, evt.Device.OsType)
	})

	t.Run("does not emit an audit event on a retried approval", func(t *testing.T) {
		t.Parallel()
		ctx := configureOutgoingContext(t.Context(), outgoingContextParams{
			User:       "dave",
			EmitterKey: "dave",
		})
		token := claimPairing(t, ctx, env)
		approve := devicepb.ApproveEnrollPairingRequest_builder{PairingToken: token}.Build()

		_, err := devices.ApproveEnrollPairing(ctx, approve)
		require.NoError(t, err)
		eventsBefore := len(emitter.Events("dave"))

		_, err = devices.ApproveEnrollPairing(ctx, approve)
		require.NoError(t, err)
		assert.Len(t, emitter.Events("dave"), eventsBefore)

		got, err := env.EnrollPairing.GetEnrollPairingByToken(t.Context(), token)
		require.NoError(t, err)
		assert.Equal(t,
			devicepb.EnrollPairingState_ENROLL_PAIRING_STATE_APPROVED,
			got.GetStatus().GetState())
	})

	t.Run("audits a failure after the pairing is read", func(t *testing.T) {
		t.Parallel()
		ctx := configureOutgoingContext(t.Context(), outgoingContextParams{
			User:       "erin",
			EmitterKey: "erin",
		})
		// The pairing stays in AWAITING_DEVICE, so approval fails at the state
		// transition, past the point where the pairing is read.
		resp, err := devices.CreateEnrollPairing(ctx, &devicepb.CreateEnrollPairingRequest{})
		require.NoError(t, err)

		_, err = devices.ApproveEnrollPairing(ctx, devicepb.ApproveEnrollPairingRequest_builder{
			PairingToken: resp.GetEnrollPairing().GetStatus().GetToken(),
		}.Build())
		assert.ErrorAs(t, err, new(*trace.CompareFailedError))
		assert.ErrorContains(t, err, "enroll pairing is not awaiting approval")

		evt, ok := emitter.LastEvent("erin").(*apievents.DeviceEvent2)
		require.True(t, ok, "expected a DeviceEvent2, got %T", emitter.LastEvent("erin"))
		assert.Equal(t, events.DeviceEnrollPairingApproveFailureCode, evt.GetCode())
		assert.False(t, evt.Status.Success)
		// No device is reported before a device claims the pairing.
		assert.Nil(t, evt.Device)
	})
}

func TestService_ApproveEnrollPairing_adminActionMFA(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		state authz.AdminActionAuthState
	}{
		{
			// What a cluster that enforces MFA for admin actions reports for a caller
			// that presented no MFA response.
			name:  "no MFA response",
			state: authz.AdminActionAuthUnauthorized,
		},
		{
			// Unlike the other mutating RPCs of this service, approval must reject
			// MFA responses created from reusable challenges.
			name:  "reused MFA response",
			state: authz.AdminActionAuthMFAVerifiedWithReuse,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			emitter := &testenv.KeyedEmitter{}
			env := testenv.NewUsingT(t,
				testenv.WithAuthorizer(newUserAwareAuthorizer(
					withKnownUsers("alice"),
					withAuthorizedUsers("alice"),
					withAdminActionAuthState(test.state),
				)),
				testenv.WithEmitter(emitter))
			ctx := configureOutgoingContext(t.Context(), outgoingContextParams{
				User:       "alice",
				EmitterKey: "alice",
			})
			token := claimPairing(t, ctx, env)

			_, err := env.DevicesClient.ApproveEnrollPairing(ctx, devicepb.ApproveEnrollPairingRequest_builder{
				PairingToken: token,
			}.Build())
			assert.ErrorIs(t, err, &mfa.ErrAdminActionMFARequired)

			// The MFA gate rejects the call before the audit defer is
			// installed, so the denied approval leaves no trace in the audit
			// log.
			assert.Empty(t, emitter.Events("alice"))

			// The pairing is left for the user to approve once they pass the
			// ceremony.
			got, err := env.EnrollPairing.GetEnrollPairingByToken(t.Context(), token)
			require.NoError(t, err)
			assert.Equal(t,
				devicepb.EnrollPairingState_ENROLL_PAIRING_STATE_AWAITING_APPROVAL,
				got.GetStatus().GetState())

			// Denial is deliberately not gated on the ceremony.
			_, err = env.DevicesClient.DenyEnrollPairing(ctx, devicepb.DenyEnrollPairingRequest_builder{
				PairingToken: token,
			}.Build())
			assert.NoError(t, err)
		})
	}
}

func TestService_DenyEnrollPairing(t *testing.T) {
	t.Parallel()

	emitter := &testenv.KeyedEmitter{}
	env := testenv.NewUsingT(t,
		testenv.WithAuthorizer(newUserAwareAuthorizer(
			withKnownUsers("alice", "bob"),
			withAuthorizedUsers("alice", "bob"),
		)),
		testenv.WithEmitter(emitter))
	devices := env.DevicesClient

	t.Run("deletes the pairing", func(t *testing.T) {
		t.Parallel()
		ctx := configureOutgoingContext(t.Context(), outgoingContextParams{
			User:       "alice",
			EmitterKey: "alice",
		})
		token := claimPairing(t, ctx, env)

		_, err := devices.DenyEnrollPairing(ctx, devicepb.DenyEnrollPairingRequest_builder{
			PairingToken: token,
		}.Build())
		require.NoError(t, err)

		_, err = env.EnrollPairing.GetEnrollPairingByToken(t.Context(), token)
		assert.ErrorAs(t, err, new(*trace.NotFoundError))
		assert.ErrorContains(t, err, "enroll_pairing") // Part of the backend key name.
		assert.ErrorContains(t, err, "not found")

		evt, ok := emitter.LastEvent("alice").(*apievents.DeviceEvent2)
		require.True(t, ok, "expected a DeviceEvent2, got %T", emitter.LastEvent("alice"))
		assert.Equal(t, events.DeviceEnrollPairingDenyEvent, evt.GetType())
		assert.Equal(t, events.DeviceEnrollPairingDenyCode, evt.GetCode())
		assert.False(t, evt.Status.Success)
		assert.Empty(t, evt.Status.Error)
		require.NotNil(t, evt.Device)
		assert.Equal(t, "CXXXXXXXXX01", evt.Device.AssetTag)
	})

	t.Run("returns NotFound when the token does not match the current pairing", func(t *testing.T) {
		t.Parallel()
		ctx := configureOutgoingContext(t.Context(), outgoingContextParams{
			User:       "bob",
			EmitterKey: "bob",
		})
		token := claimPairing(t, ctx, env)

		_, err := devices.DenyEnrollPairing(ctx, devicepb.DenyEnrollPairingRequest_builder{
			PairingToken: "does-not-match-pairing",
		}.Build())
		assert.ErrorAs(t, err, new(*trace.NotFoundError), "Expected DenyEnrollPairing to return NotFound")

		_, err = env.EnrollPairing.GetEnrollPairingByToken(t.Context(), token)
		assert.NoError(t, err, "Expected the pairing the caller does own to still exist")
	})
}

// TestService_enrollPairingInvalidRequests exercises the request validation
// shared by ApproveEnrollPairing and DenyEnrollPairing. Both go through the
// enrollPairingMatchingToken helper and the same RBAC gate, so invalid requests
// must fail the same way through either RPC.
func TestService_enrollPairingInvalidRequests(t *testing.T) {
	t.Parallel()

	env := testenv.NewUsingT(t, testenv.WithAuthorizer(newUserAwareAuthorizer(
		withKnownUsers("alice", "carol", "mallory"),
		withAuthorizedUsers("alice", "carol"),
	)))
	devices := env.DevicesClient

	rpcs := []struct {
		name string
		call func(ctx context.Context, token string) error
	}{
		{
			name: "ApproveEnrollPairing",
			call: func(ctx context.Context, token string) error {
				_, err := devices.ApproveEnrollPairing(ctx, devicepb.ApproveEnrollPairingRequest_builder{
					PairingToken: token,
				}.Build())
				return err
			},
		},
		{
			name: "DenyEnrollPairing",
			call: func(ctx context.Context, token string) error {
				_, err := devices.DenyEnrollPairing(ctx, devicepb.DenyEnrollPairingRequest_builder{
					PairingToken: token,
				}.Build())
				return err
			},
		},
	}
	for _, rpc := range rpcs {
		t.Run(rpc.name, func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name    string
				user    string
				token   string
				wantErr any
				wantMsg string
			}{
				{
					name:    "returns NotFound when the user has no pairing",
					user:    "carol",
					token:   "some-token",
					wantErr: new(*trace.NotFoundError),
					wantMsg: `user "carol" has no enroll pairing`,
				},
				{
					name:    "rejects an empty token",
					user:    "alice",
					token:   "",
					wantErr: new(*trace.BadParameterError),
					wantMsg: "pairing token required",
				},
				{
					name:    "rejects a user without mobile_device.create_enroll_token",
					user:    "mallory",
					token:   "some-token",
					wantErr: new(*trace.AccessDeniedError),
					wantMsg: "access denied",
				},
			}
			for _, test := range tests {
				t.Run(test.name, func(t *testing.T) {
					t.Parallel()
					ctx := contextWithUser(t.Context(), test.user)

					err := rpc.call(ctx, test.token)
					assert.ErrorAs(t, err, test.wantErr)
					assert.ErrorContains(t, err, test.wantMsg)
				})
			}
		})
	}
}

// claimPairing creates a pairing for the user in ctx and advances it to
// AWAITING_APPROVAL, the state the Web UI acts on, returning its token.
func claimPairing(t *testing.T, ctx context.Context, env *testenv.E) string {
	t.Helper()
	resp, err := env.DevicesClient.CreateEnrollPairing(ctx, &devicepb.CreateEnrollPairingRequest{})
	require.NoError(t, err)

	pairing, err := env.EnrollPairing.RequestEnrollPairingApproval(t.Context(), resp.GetEnrollPairing(),
		devicepb.EnrollPairingDevice_builder{
			OsType:       devicepb.OSType_OS_TYPE_IOS,
			SerialNumber: "CXXXXXXXXX01",
			OsVersion:    "26.3.1",
		}.Build())
	require.NoError(t, err)
	return pairing.GetStatus().GetToken()
}
