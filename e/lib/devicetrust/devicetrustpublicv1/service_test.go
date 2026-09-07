package devicetrustpublicv1_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/defaults"
	devicetrustpublicv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/public/v1"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/e/lib/devicetrust/devicetrustpublicv1"
	"github.com/gravitational/teleport/e/lib/devicetrust/devicetrustv1"
	dterrors "github.com/gravitational/teleport/e/lib/devicetrust/errors"
	"github.com/gravitational/teleport/e/lib/devicetrust/storage"
	"github.com/gravitational/teleport/e/lib/devicetrust/testenv"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	osstestenv "github.com/gravitational/teleport/lib/devicetrust/testenv"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
)

func TestService_authz(t *testing.T) {
	t.Parallel()

	authorizer := &fakeAuthorizer{}
	emitter := &testenv.KeyedEmitter{}
	testModules := &modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.DeviceTrust: {Enabled: true},
			},
		},
	}
	env := testenv.NewUsingT(t,
		testenv.WithAuthorizer(authorizer),
		testenv.WithEmitter(emitter),
		testenv.WithModules(testModules),
	)

	// Every case pairs with the same device, registered once so that the test
	// case suffixed with "-allowed" runs all the way to a real enrollment token.
	registerDevice(t, env, makeCollectedData())

	// Unlike in the private Device Trust service, the rule check runs against the
	// identity reconstructed from the pairing token instead of the caller's, so
	// each case seeds a user and a pairing for the handler to resolve.
	tests := []struct {
		name string
		want []wantRuleVerb
		// rpc creates user, seeds a pairing owned by it and runs the RPC.
		rpc func(t *testing.T, user string) (*devicetrustpublicv1pb.CreatePairedDeviceEnrollTokenResponse, error)
		// assertResult accepts the outcome of an RPC that passed authz.
		assertResult func(t *testing.T, resp *devicetrustpublicv1pb.CreatePairedDeviceEnrollTokenResponse, err error)
	}{
		{
			name: "CreatePairedDeviceEnrollToken",
			want: []wantRuleVerb{
				{rule: types.KindMobileDevice, verb: types.VerbCreateEnrollToken},
			},
			rpc: func(t *testing.T, user string) (*devicetrustpublicv1pb.CreatePairedDeviceEnrollTokenResponse, error) {
				createUser(t, env, user)
				token := createPairing(t, env, user)
				// Pre-approve the pairing so the call doesn't block waiting for
				// approval once authz passes.
				approvePairing(t, env, token, makeCollectedData())
				ctx := testenv.WithOutgoingEmitterKey(t.Context(), user)
				return env.PublicDevicesClient.CreatePairedDeviceEnrollToken(ctx,
					makeRequest(token, makeCollectedData()))
			},
			assertResult: func(t *testing.T, resp *devicetrustpublicv1pb.CreatePairedDeviceEnrollTokenResponse, err error) {
				require.NoError(t, err)
				assert.NotEmpty(t, resp.GetDeviceEnrollToken().GetToken())
			},
		},
	}

	// Verify that the right permission is checked.
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			checker := &ruleVerifyingChecker{want: test.want}
			authorizer.checker = checker

			resp, err := test.rpc(t, test.name+"-allowed")
			test.assertResult(t, resp, err)
			assert.NoError(t, checker.verifyMatches())
		})
	}

	// Verify that the audit event is attributed to the correct user if they don't
	// pass authz.
	for _, test := range tests {
		t.Run(test.name+"-denied", func(t *testing.T) {
			user := test.name + "-denied"
			authorizer.checker = &fakeChecker{authorized: false}

			_, err := test.rpc(t, user)
			assert.ErrorAs(t, err, new(*trace.AccessDeniedError))
			// Message from fakeChecker, which tells this apart from the other
			// AccessDenied paths.
			assert.ErrorContains(t, err, "access denied")

			last := lastDeviceEvent(t, emitter.LastEvent(user))
			assert.False(t, last.Status.Success)
			assert.Equal(t, user, last.UserMetadata.User)
		})
	}

	testModules.TestFeatures.Entitlements[entitlements.DeviceTrust] = modules.EntitlementInfo{Enabled: false}

	// Verify that the RPC fails with AccessDenied when the feature is disabled.
	// The feature check is bundled with the proxy authz. It runs before the
	// pairing is read, so there's no user to attribute this failure to.
	for _, test := range tests {
		t.Run(test.name+"-feature disabled", func(t *testing.T) {
			authorizer.checker = &testenv.NoopChecker{}

			user := test.name + "-feature-disabled"
			_, err := test.rpc(t, user)
			assert.ErrorAs(t, err, new(*trace.AccessDeniedError))
			assert.ErrorContains(t, err, "not licensed for device trust")

			assert.Empty(t, emitter.Events(user))
		})
	}
}

func TestService_rejectsNonProxyCaller(t *testing.T) {
	t.Parallel()

	emitter := &eventstest.MockRecorderEmitter{}
	env := testenv.NewUsingT(t,
		testenv.WithAuthorizer(builtinRoleAuthorizer{role: types.RoleNode}),
		testenv.WithEmitter(emitter),
	)
	client := env.PublicDevicesClient

	rpcs := []struct {
		name string
		call func(t *testing.T) error
	}{
		{
			name: "CreatePairedDeviceEnrollToken",
			call: func(t *testing.T) error {
				_, err := client.CreatePairedDeviceEnrollToken(t.Context(),
					makeRequest("some-token", makeCollectedData()))
				return err
			},
		},
		{
			name: "EnrollDevice",
			call: func(t *testing.T) error {
				stream, err := client.EnrollDevice(t.Context())
				require.NoError(t, err)
				// The proxy check runs before the handler reads anything, so the stream
				// fails without a single message sent.
				_, err = stream.Recv()
				return err
			},
		},
	}
	for _, rpc := range rpcs {
		t.Run(rpc.name, func(t *testing.T) {
			t.Parallel()
			err := rpc.call(t)
			assert.ErrorAs(t, err, new(*trace.AccessDeniedError))
			assert.ErrorContains(t, err, "can only be executed by a proxy")
			// The Proxy check runs before the user is resolved, so nothing is attributed.
			assert.Empty(t, emitter.Events())
		})
	}
}

func TestService_CreatePairedDeviceEnrollToken(t *testing.T) {
	t.Parallel()

	emitter := &testenv.KeyedEmitter{}
	env := testenv.NewUsingT(t,
		testenv.WithAuthorizer(&fakeAuthorizer{
			authorizedUsers: []string{"erin"},
		}),
		testenv.WithEmitter(emitter),
	)
	client := env.PublicDevicesClient

	t.Run("claims the pairing, waits for approval and returns the enrollment token", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			emitter := &testenv.KeyedEmitter{}
			env := testenv.NewUsingT(t,
				testenv.WithAuthorizer(&fakeAuthorizer{authorizedUsers: []string{"alice"}}),
				testenv.WithEmitter(emitter),
			)
			client := env.PublicDevicesClient
			createUser(t, env, "alice")
			token := createPairing(t, env, "alice")

			cd := makeCollectedDataFor("llama-approved")
			registerDevice(t, env, cd)

			ctx := testenv.WithOutgoingEmitterKey(t.Context(), "alice")
			resCh := createTokenAsync(ctx, client, token, cd)

			// The pairing has transitioned and recorded the claiming device. The exact
			// fields are verified in tests of lib/services/local.EnrollPairingService.
			pairing := waitForPairingState(t, env, token,
				devicepb.EnrollPairingState_ENROLL_PAIRING_STATE_AWAITING_APPROVAL)
			assert.Equal(t, cd.GetSerialNumber(), pairing.GetStatus().GetDevice().GetSerialNumber())

			_, err := env.EnrollPairing.ApproveEnrollPairing(t.Context(), pairing)
			require.NoError(t, err)

			res := awaitResult(t, resCh)
			require.NoError(t, res.err)
			assert.NotEmpty(t, res.resp.GetDeviceEnrollToken().GetToken())

			// Issuing the token consumed the pairing, making it single-use.
			_, err = env.EnrollPairing.GetEnrollPairingByToken(t.Context(), token)
			assert.ErrorAs(t, err, new(*trace.NotFoundError))

			// The claim and the issuance are audited, in that order.
			evts := deviceEvents(t, emitter.Events("alice"))
			require.Len(t, evts, 2)
			request, issuance := evts[0], evts[1]

			assert.Equal(t, events.DeviceEnrollPairingRequestEvent, request.GetType())
			assert.True(t, request.Status.Success)
			assert.Equal(t, "alice", request.UserMetadata.User)
			assert.Equal(t, events.DeviceEnrollPairingRequestCode, request.Metadata.Code)

			assert.Equal(t, events.DeviceEnrollTokenCreateEvent, issuance.GetType())
			assert.True(t, issuance.Status.Success)
			assert.Equal(t, "alice", issuance.UserMetadata.User)
			assert.Equal(t, events.DeviceEnrollTokenCreateCode, issuance.Metadata.Code)
			assert.Equal(t, cd.GetSerialNumber(), issuance.Device.AssetTag)
		})
	})

	t.Run("allows a retry from the same device", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			emitter := &testenv.KeyedEmitter{}
			env := testenv.NewUsingT(t,
				testenv.WithAuthorizer(&fakeAuthorizer{authorizedUsers: []string{"bob"}}),
				testenv.WithEmitter(emitter),
			)
			client := env.PublicDevicesClient
			createUser(t, env, "bob")
			token := createPairing(t, env, "bob")

			cd := makeCollectedDataFor("llama-retry")
			registerDevice(t, env, cd)

			// Drop the first call the way a mobile or proxy disconnect would, leaving
			// the pairing claimed with no handler waiting on it.
			//
			// A canceled client call returns while its handler still winds down, so
			// wait for the bubble to settle: cancellation keeps the handler runnable
			// until it exits, and fake time freezes meanwhile, so it cannot wake to
			// consume the approval below instead.
			droppedCtx, cancel := context.WithCancel(t.Context())
			firstResult := createTokenAsync(droppedCtx, client, token, cd)
			waitForPairingState(t, env, token,
				devicepb.EnrollPairingState_ENROLL_PAIRING_STATE_AWAITING_APPROVAL)
			cancel()
			synctest.Wait()
			// waitForPairingState already verifies that the pairing reached
			// AWAITING_APPROVAL, so it's enough to check merely if the err is present.
			assert.Error(t, awaitResult(t, firstResult).err)

			// The same device reattaches to its own in-progress claim. A retry is not a
			// fresh request, so no additional request event is emitted.
			retryCtx := testenv.WithOutgoingEmitterKey(t.Context(), "bob-retry")
			resCh := createTokenAsync(retryCtx, client, token, cd)

			pairing, err := env.EnrollPairing.GetEnrollPairingByToken(t.Context(), token)
			require.NoError(t, err)
			_, err = env.EnrollPairing.ApproveEnrollPairing(t.Context(), pairing)
			require.NoError(t, err)

			res := awaitResult(t, resCh)
			require.NoError(t, res.err)
			assert.NotEmpty(t, res.resp.GetDeviceEnrollToken().GetToken())
			evts := deviceEvents(t, emitter.Events("bob-retry"))
			require.Len(t, evts, 1)
			assert.Equal(t, events.DeviceEnrollTokenCreateEvent, evts[0].GetType())
		})
	})

	t.Run("consumes a pairing approved while no handler was waiting", func(t *testing.T) {
		t.Parallel()
		createUser(t, env, "erin")
		token := createPairing(t, env, "erin")

		cd := makeCollectedDataFor("llama-idle")
		registerDevice(t, env, cd)
		approvePairing(t, env, token, cd)

		// Reattaching to an already-approved pairing is not a fresh request, so no
		// request event is emitted.
		ctx := testenv.WithOutgoingEmitterKey(t.Context(), "erin")
		resp, err := client.CreatePairedDeviceEnrollToken(ctx, makeRequest(token, cd))
		require.NoError(t, err)
		assert.NotEmpty(t, resp.GetDeviceEnrollToken().GetToken())
		evts := deviceEvents(t, emitter.Events("erin"))
		require.Len(t, evts, 1)
		assert.Equal(t, events.DeviceEnrollTokenCreateEvent, evts[0].GetType())
	})

}

func TestService_CreatePairedDeviceEnrollToken_errors(t *testing.T) {
	t.Parallel()

	t.Run("returns AccessDenied when the request is denied", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			emitter := &testenv.KeyedEmitter{}
			env := testenv.NewUsingT(t,
				testenv.WithAuthorizer(&fakeAuthorizer{authorizedUsers: []string{"dave"}}),
				testenv.WithEmitter(emitter),
			)
			client := env.PublicDevicesClient
			createUser(t, env, "dave")
			token := createPairing(t, env, "dave")

			ctx := testenv.WithOutgoingEmitterKey(t.Context(), "dave")
			resCh := createTokenAsync(ctx, client, token, makeCollectedData())

			pairing := waitForPairingState(t, env, token,
				devicepb.EnrollPairingState_ENROLL_PAIRING_STATE_AWAITING_APPROVAL)
			require.NoError(t, env.EnrollPairing.DeleteEnrollPairing(t.Context(), pairing))

			res := awaitResult(t, resCh)
			assert.ErrorIs(t, res.err, devicetrustpublicv1.ErrPairingDeniedOrExpired)

			// A denial is audited by DenyEnrollPairing alone, so the handler emitted
			// nothing beyond the claim's request event.
			assert.Len(t, emitter.Events("dave"), 1)
		})
	})

	t.Run("rejects a claim from a different device", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			emitter := &testenv.KeyedEmitter{}
			env := testenv.NewUsingT(t,
				testenv.WithAuthorizer(&fakeAuthorizer{authorizedUsers: []string{"carol"}}),
				testenv.WithEmitter(emitter),
			)
			client := env.PublicDevicesClient
			createUser(t, env, "carol")
			token := createPairing(t, env, "carol")

			ctx := testenv.WithOutgoingEmitterKey(t.Context(), "carol")
			resCh := createTokenAsync(ctx, client, token, makeCollectedData())
			waitForPairingState(t, env, token,
				devicepb.EnrollPairingState_ENROLL_PAIRING_STATE_AWAITING_APPROVAL)

			hijack := makeCollectedData()
			hijack.SetSerialNumber("alpaca")
			hijackCtx := testenv.WithOutgoingEmitterKey(t.Context(), "carol-hijack")
			_, err := client.CreatePairedDeviceEnrollToken(hijackCtx, makeRequest(token, hijack))
			assert.ErrorIs(t, err, devicetrustpublicv1.ErrPairingClaimed)

			last := lastDeviceEvent(t, emitter.LastEvent("carol-hijack"))
			assert.False(t, last.Status.Success)
			assert.Equal(t, "carol", last.UserMetadata.User)

			// Release the handler still waiting on the pairing.
			pairing, err := env.EnrollPairing.GetEnrollPairingByToken(t.Context(), token)
			require.NoError(t, err)
			require.NoError(t, env.EnrollPairing.DeleteEnrollPairing(t.Context(), pairing))
			assert.ErrorIs(t, awaitResult(t, resCh).err, devicetrustpublicv1.ErrPairingDeniedOrExpired)
		})
	})

	// The subtests below don't block on the poll interval, so they don't need to
	// use synctest and they can share env and client.
	emitter := &testenv.KeyedEmitter{}
	env := testenv.NewUsingT(t,
		testenv.WithAuthorizer(&fakeAuthorizer{
			authorizedUsers: []string{"grace"},
		}),
		testenv.WithEmitter(emitter),
	)
	client := env.PublicDevicesClient

	t.Run("rejects an unknown token without leaking storage state", func(t *testing.T) {
		t.Parallel()
		ctx := testenv.WithOutgoingEmitterKey(t.Context(), "unknown-token")
		_, err := client.CreatePairedDeviceEnrollToken(ctx, makeRequest("does-not-exist", makeCollectedData()))
		assertErrorIsAndEqual(t, err, devicetrustpublicv1.ErrInvalidPairingToken)

		last := lastDeviceEvent(t, emitter.LastEvent("unknown-token"))
		assert.False(t, last.Status.Success)
		assert.Empty(t, last.UserMetadata.User)
		assert.Equal(t, events.DeviceEnrollPairingRequestFailureCode, last.Metadata.Code)
	})

	t.Run("rejects an unregistered device without leaking inventory state", func(t *testing.T) {
		t.Parallel()
		createUser(t, env, "grace")
		token := createPairing(t, env, "grace")

		// No registerDevice call: the serial number is approved for enrollment but
		// absent from the device inventory.
		cd := makeCollectedDataFor("llama-unregistered")
		approvePairing(t, env, token, cd)

		ctx := testenv.WithOutgoingEmitterKey(t.Context(), "grace")
		_, err := client.CreatePairedDeviceEnrollToken(ctx, makeRequest(token, cd))
		assertErrorIsAndEqual(t, err, devicetrustpublicv1.ErrEnrollVerificationFailed)

		// The redaction stops at the RPC boundary: the audit trail keeps the real
		// reason the token was not issued.
		last := lastDeviceEvent(t, emitter.LastEvent("grace"))
		assert.Equal(t, events.DeviceEnrollTokenCreateEvent, last.GetType())
		assert.False(t, last.Status.Success)
		assert.Equal(t, "grace", last.UserMetadata.User)
		assert.Contains(t, last.Status.UserMessage, "device not found")
	})

	t.Run("returns NotFound when the pairing user no longer exists", func(t *testing.T) {
		t.Parallel()
		// "sso-user" owns a pairing but has no user record.
		token := createPairing(t, env, "sso-user")

		ctx := testenv.WithOutgoingEmitterKey(t.Context(), "sso-user")
		_, err := client.CreatePairedDeviceEnrollToken(ctx, makeRequest(token, makeCollectedData()))
		assert.ErrorAs(t, err, new(*trace.NotFoundError))
		assert.ErrorContains(t, err, "user not found")

		last := lastDeviceEvent(t, emitter.LastEvent("sso-user"))
		assert.False(t, last.Status.Success)
		assert.Equal(t, "sso-user", last.UserMetadata.User)
	})
}

func TestService_CreatePairedDeviceEnrollToken_badParameters(t *testing.T) {
	t.Parallel()

	emitter := &testenv.KeyedEmitter{}
	env := testenv.NewUsingT(t,
		testenv.WithAuthorizer(&fakeAuthorizer{}),
		testenv.WithEmitter(emitter),
	)

	for _, test := range []struct {
		name    string
		token   string
		cd      *devicepb.DeviceCollectedData
		wantErr string
	}{
		{
			name:    "missing token",
			cd:      makeCollectedData(),
			wantErr: "enroll_pairing_token required",
		},
		{
			name:    "missing device data",
			token:   "foo",
			wantErr: "device collected data required",
		},
		{
			name:    "missing device collect time",
			token:   "foo",
			cd:      &devicepb.DeviceCollectedData{},
			wantErr: "device collect time missing or invalid",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx := testenv.WithOutgoingEmitterKey(t.Context(), test.name)
			_, err := env.PublicDevicesClient.CreatePairedDeviceEnrollToken(ctx,
				makeRequest(test.token, test.cd))
			assert.ErrorAs(t, err, new(*trace.BadParameterError))
			assert.ErrorContains(t, err, test.wantErr)

			assert.Empty(t, emitter.Events(test.name))
		})
	}
}

// TestService_CreatePairedDeviceEnrollToken_concurrentClaim exercises the
// branch where a competing device claims the pairing between the handler's read
// and its compare-and-swap: the swap fails and the handler re-reads to find the
// winner.
func TestService_CreatePairedDeviceEnrollToken_concurrentClaim(t *testing.T) {
	t.Parallel()

	bk, err := memory.New(memory.Config{Context: t.Context()})
	require.NoError(t, err)
	t.Cleanup(func() { _ = bk.Close() })
	real, err := local.NewEnrollPairingService(bk)
	require.NoError(t, err)

	winner := devicepb.EnrollPairingDevice_builder{
		OsType:       devicepb.OSType_OS_TYPE_IOS,
		SerialNumber: "alpaca",
		OsVersion:    "26.0",
	}.Build()

	emitter := &eventstest.MockRecorderEmitter{}
	env := testenv.NewUsingT(t,
		testenv.WithAuthorizer(&fakeAuthorizer{authorizedUsers: []string{"alice"}}),
		testenv.WithEmitter(emitter),
		testenv.WithEnrollPairing(&racingEnrollPairing{EnrollPairing: real, competitor: winner}),
	)

	createUser(t, env, "alice")
	created, err := real.CreateEnrollPairing(t.Context(), "alice")
	require.NoError(t, err)

	_, err = env.PublicDevicesClient.CreatePairedDeviceEnrollToken(t.Context(),
		makeRequest(created.GetStatus().GetToken(), makeCollectedData()))
	assert.ErrorIs(t, err, devicetrustpublicv1.ErrPairingClaimed)

	last := lastDeviceEvent(t, emitter.LastEvent())
	assert.False(t, last.Status.Success)
	assert.Equal(t, "alice", last.UserMetadata.User)
}

// TestService_CreatePairedDeviceEnrollToken_lookupFailure covers a pairing
// lookup or claim swap that fails for a reason other than a missing pairing:
// the token is still good, so the failure must not read as a rejection of it,
// and the backend error must not reach the caller.
func TestService_CreatePairedDeviceEnrollToken_lookupFailure(t *testing.T) {
	t.Parallel()

	// Message that must not reach the caller through any error the RPC returns.
	backendErr := errors.New("the database is down at the moment")
	verifyErr := func(t *testing.T, err error) {
		t.Helper()
		assertErrorIsAndEqual(t, err, devicetrustpublicv1.ErrPairingLookupUnavailable)
		// In case someone would change the type of ErrPairingLookupUnavailable, pin
		// its error type to ConnectionProblem. This error maps to Unavailable over
		// the wire and the client knows that it can retry the req.
		assert.ErrorAs(t, err, new(*trace.ConnectionProblemError))
	}

	t.Run("at claim time", func(t *testing.T) {
		t.Parallel()
		env, failing, token := newLookupFailureEnv(t, "alice")
		failing.failWith(backendErr)

		_, err := env.PublicDevicesClient.CreatePairedDeviceEnrollToken(t.Context(),
			makeRequest(token, makeCollectedData()))
		verifyErr(t, err)
	})

	t.Run("at the claim swap", func(t *testing.T) {
		t.Parallel()
		env, failing, token := newLookupFailureEnv(t, "bob")
		failing.failSwapWith(backendErr)

		_, err := env.PublicDevicesClient.CreatePairedDeviceEnrollToken(t.Context(),
			makeRequest(token, makeCollectedData()))
		verifyErr(t, err)
	})

	t.Run("during the approval wait", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			env, failing, token := newLookupFailureEnv(t, "carol")

			resCh := createTokenAsync(t.Context(), env.PublicDevicesClient, token, makeCollectedData())
			waitForPairingState(t, env, token,
				devicepb.EnrollPairingState_ENROLL_PAIRING_STATE_AWAITING_APPROVAL)
			failing.failWith(backendErr)

			err := awaitResult(t, resCh).err
			verifyErr(t, err)
		})
	})
}

// TestService_CreatePairedDeviceEnrollToken_concurrentWaiters exercises two
// handlers waiting on the same approval: consuming the pairing is guarded by a
// conditional delete, so exactly one of them issues a token and the other
// reports the pairing consumed.
//
// The bubble makes the race repeatable: both handlers are parked in their poll
// before the approval lands and wake on the same fake tick.
func TestService_CreatePairedDeviceEnrollToken_concurrentWaiters(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		bk, err := memory.New(memory.Config{Context: t.Context()})
		require.NoError(t, err)
		t.Cleanup(func() { _ = bk.Close() })
		real, err := local.NewEnrollPairingService(bk)
		require.NoError(t, err)

		emitter := &testenv.KeyedEmitter{}
		env := testenv.NewUsingT(t,
			testenv.WithAuthorizer(&fakeAuthorizer{authorizedUsers: []string{"alice"}}),
			testenv.WithEmitter(emitter),
			testenv.WithEnrollPairing(newBlockingDeleteEnrollPairing(real, 2)),
		)
		client := env.PublicDevicesClient
		createUser(t, env, "alice")
		token := createPairing(t, env, "alice")
		cd := makeCollectedData()
		registerDevice(t, env, cd)

		// Both calls present the same device, so the second reattaches to the
		// first's claim instead of being rejected as a hijack.
		ctx := testenv.WithOutgoingEmitterKey(t.Context(), "alice")
		first := createTokenAsync(ctx, client, token, cd)
		second := createTokenAsync(ctx, client, token, cd)
		waitForPairingState(t, env, token,
			devicepb.EnrollPairingState_ENROLL_PAIRING_STATE_AWAITING_APPROVAL)

		pairing, err := real.GetEnrollPairingByToken(t.Context(), token)
		require.NoError(t, err)
		_, err = real.ApproveEnrollPairing(t.Context(), pairing)
		require.NoError(t, err)

		// Which handler wins the delete is up to the scheduler, but exactly one
		// must, and the loser settles on the consumed error.
		winner, loser := awaitResult(t, first), awaitResult(t, second)
		if winner.err != nil {
			winner, loser = loser, winner
		}
		require.NoError(t, winner.err, "Expected one of the goroutines to return no error")
		assert.NotEmpty(t, winner.resp.GetDeviceEnrollToken().GetToken())
		assert.ErrorIs(t, loser.err, devicetrustpublicv1.ErrPairingConsumed)

		// Exactly one fresh claim and one issuance are audited: the reattaching
		// call and the losing waiter add nothing.
		evts := deviceEvents(t, emitter.Events("alice"))
		require.Len(t, evts, 2)
		assert.Equal(t, events.DeviceEnrollPairingRequestEvent, evts[0].GetType())
		assert.Equal(t, events.DeviceEnrollTokenCreateEvent, evts[1].GetType())
	})
}

// TestService_CreatePairedDeviceEnrollToken_approvalWaitTimeout exercises the
// bound on the approval wait: storage keeps returning a pairing that outlived
// its TTL, standing in for a backend that missed the expiry, and the handler
// gives up on its own instead of polling forever.
func TestService_CreatePairedDeviceEnrollToken_approvalWaitTimeout(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		bk, err := memory.New(memory.Config{Context: t.Context()})
		require.NoError(t, err)
		t.Cleanup(func() { _ = bk.Close() })
		real, err := local.NewEnrollPairingService(bk)
		require.NoError(t, err)

		env := testenv.NewUsingT(t,
			testenv.WithAuthorizer(&fakeAuthorizer{authorizedUsers: []string{"alice"}}),
			testenv.WithEnrollPairing(&staleEnrollPairing{EnrollPairing: real}),
		)
		createUser(t, env, "alice")
		token := createPairing(t, env, "alice")

		start := time.Now()
		resCh := createTokenAsync(t.Context(), env.PublicDevicesClient, token, makeCollectedData())
		waitForPairingState(t, env, token,
			devicepb.EnrollPairingState_ENROLL_PAIRING_STATE_AWAITING_APPROVAL)

		// [awaitResult]'s failsafe is shorter than the wait, so select inline. The
		// cap catches a regression that would otherwise poll unbounded.
		select {
		case res := <-resCh:
			assert.ErrorIs(t, res.err, devicetrustpublicv1.ErrPairingDeniedOrExpired)
			assert.Equal(t, devicetrustpublicv1.EnrollPairingApprovalTimeout, time.Since(start))
		case <-time.After(devicetrustpublicv1.EnrollPairingApprovalTimeout + time.Minute):
			t.Fatal("timed out waiting for CreatePairedDeviceEnrollToken")
		}
	})
}

// TestService_CreatePairedDeviceEnrollToken_pairingExpiry exercises the healthy
// counterpart of the wait timeout: a pairing that nobody approves ages out of
// storage and the poll surfaces the expiry at the pairing TTL, before the poll
// timeout fires.
func TestService_CreatePairedDeviceEnrollToken_pairingExpiry(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		env := testenv.NewUsingT(t,
			testenv.WithAuthorizer(&fakeAuthorizer{authorizedUsers: []string{"alice"}}),
		)
		createUser(t, env, "alice")
		token := createPairing(t, env, "alice")

		start := time.Now()
		resCh := createTokenAsync(t.Context(), env.PublicDevicesClient, token, makeCollectedData())
		waitForPairingState(t, env, token,
			devicepb.EnrollPairingState_ENROLL_PAIRING_STATE_AWAITING_APPROVAL)

		// [awaitResult]'s failsafe is shorter than the wait, so select inline.
		select {
		case res := <-resCh:
			assert.ErrorIs(t, res.err, devicetrustpublicv1.ErrPairingDeniedOrExpired)
			// The elapsed bounds tell the expiry apart from the poll timeout, which
			// surfaces as the same error.
			elapsed := time.Since(start)
			assert.GreaterOrEqual(t, elapsed, local.EnrollPairingExpireDuration)
			assert.Less(t, elapsed, devicetrustpublicv1.EnrollPairingApprovalTimeout)
		case <-time.After(devicetrustpublicv1.EnrollPairingApprovalTimeout + time.Minute):
			t.Fatal("timed out waiting for CreatePairedDeviceEnrollToken")
		}
	})
}

// TestService_CreatePairedDeviceEnrollToken_deleteFailure exercises a backend
// failure while consuming the pairing: unlike a lost conditional delete, the
// pairing is still there, so the caller gets a retryable error with the
// backend state erased.
func TestService_CreatePairedDeviceEnrollToken_deleteFailure(t *testing.T) {
	t.Parallel()

	backendErr := trace.Errorf("shard 7 on fire")

	bk, err := memory.New(memory.Config{Context: t.Context()})
	require.NoError(t, err)
	t.Cleanup(func() { _ = bk.Close() })
	real, err := local.NewEnrollPairingService(bk)
	require.NoError(t, err)

	env := testenv.NewUsingT(t,
		testenv.WithAuthorizer(&fakeAuthorizer{authorizedUsers: []string{"alice"}}),
		testenv.WithEnrollPairing(&failingDeleteEnrollPairing{EnrollPairing: real, err: backendErr}),
	)

	createUser(t, env, "alice")
	token := createPairing(t, env, "alice")
	cd := makeCollectedData()
	approvePairing(t, env, token, cd)

	_, err = env.PublicDevicesClient.CreatePairedDeviceEnrollToken(t.Context(),
		makeRequest(token, cd))
	assertErrorIsAndEqual(t, err, devicetrustpublicv1.ErrEnrollTokenIssuanceFailed)
}

func TestService_EnrollDevice(t *testing.T) {
	t.Parallel()

	emitter := &testenv.KeyedEmitter{}
	env := testenv.NewUsingT(t,
		testenv.WithAuthorizer(&fakeAuthorizer{authorizedUsers: []string{"alice", "bob"}}),
		testenv.WithEmitter(emitter),
	)

	tests := []struct {
		osType devicepb.OSType
		user   string
	}{
		{osType: devicepb.OSType_OS_TYPE_IOS, user: "alice"},
		{osType: devicepb.OSType_OS_TYPE_IPADOS, user: "bob"},
	}
	for _, test := range tests {
		t.Run(test.osType.String(), func(t *testing.T) {
			t.Parallel()
			dev, err := osstestenv.NewFakeIOSDevice(test.osType)
			require.NoError(t, err)
			createUser(t, env, test.user)
			registerDevice(t, env, dev.CollectDeviceData())
			token := createUserBoundEnrollToken(t, env, test.user, dev.CollectDeviceData())

			ctx := testenv.WithOutgoingEmitterKey(t.Context(), test.user)
			resp, err := runEnrollDevice(ctx, env.PublicDevicesClient, dev.EnrollDeviceInit(token), dev.SignChallenge)
			require.NoError(t, err)

			enrolled := resp.GetSuccess().GetDevice()
			require.NotNil(t, enrolled, "expected EnrollDeviceSuccess, got %v", resp)
			assert.Equal(t, devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_ENROLLED, enrolled.GetEnrollStatus())
			assert.Equal(t, test.user, enrolled.GetOwner())
			assert.Equal(t, test.osType, enrolled.GetOsType())
			assert.Equal(t, dev.ID, enrolled.GetCredential().GetId())

			evt := lastDeviceEvent(t, emitter.LastEvent(test.user))
			assert.Equal(t, events.DeviceEnrollEvent, evt.GetType())
			assert.Equal(t, events.DeviceEnrollCode, evt.Metadata.Code)
			assert.True(t, evt.Status.Success)
			assert.Equal(t, test.user, evt.UserMetadata.User)
			assert.NotNil(t, evt.UserMetadata.TrustedDevice)
			assert.Equal(t, dev.SerialNumber, evt.Device.AssetTag)
		})
	}
}

// TestService_EnrollDevice_authz verifies the permission the handler checks for
// the user reconstructed from the token, and the feature gate.
// It doesn't ride the shared authz table: the fixture that mints the token
// calls CreatePairedDeviceEnrollToken itself, which the table's checkers would
// reject or misattribute.
func TestService_EnrollDevice_authz(t *testing.T) {
	t.Parallel()

	authorizer := &fakeAuthorizer{authorizedUsers: []string{"alice"}}
	testModules := &modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.DeviceTrust: {Enabled: true},
			},
		},
	}
	env := testenv.NewUsingT(t,
		testenv.WithAuthorizer(authorizer),
		testenv.WithModules(testModules),
	)

	createUser(t, env, "alice")
	dev, err := osstestenv.NewFakeIOSDevice(devicepb.OSType_OS_TYPE_IOS)
	require.NoError(t, err)
	registerDevice(t, env, dev.CollectDeviceData())
	token := createUserBoundEnrollToken(t, env, "alice", dev.CollectDeviceData())

	checker := &ruleVerifyingChecker{want: []wantRuleVerb{
		{rule: types.KindMobileDevice, verb: types.VerbCreateEnrollToken},
	}}
	authorizer.checker = checker

	resp, err := runEnrollDevice(t.Context(), env.PublicDevicesClient, dev.EnrollDeviceInit(token), dev.SignChallenge)
	require.NoError(t, err)
	assert.NotNil(t, resp.GetSuccess())
	assert.NoError(t, checker.verifyMatches())

	// The feature check is bundled with the proxy authz, so a disabled feature
	// fails the stream before anything is read from it.
	testModules.TestFeatures.Entitlements[entitlements.DeviceTrust] = modules.EntitlementInfo{Enabled: false}
	stream, err := env.PublicDevicesClient.EnrollDevice(t.Context())
	require.NoError(t, err)
	_, err = stream.Recv()
	assert.ErrorAs(t, err, new(*trace.AccessDeniedError))
	assert.ErrorContains(t, err, "not licensed for device trust")
}

func TestService_EnrollDevice_errors(t *testing.T) {
	t.Parallel()

	emitter := &testenv.KeyedEmitter{}
	env := testenv.NewUsingT(t,
		testenv.WithAuthorizer(&fakeAuthorizer{authorizedUsers: []string{"carol", "dave", "erin", "grace"}}),
		testenv.WithEmitter(emitter),
	)
	client := env.PublicDevicesClient

	t.Run("rejects an invalid token without leaking storage state", func(t *testing.T) {
		t.Parallel()
		// dev is deliberately not registered: an unknown device and a bad token
		// must be indistinguishable to the caller.
		dev, err := osstestenv.NewFakeIOSDevice(devicepb.OSType_OS_TYPE_IOS)
		require.NoError(t, err)
		ctx := testenv.WithOutgoingEmitterKey(t.Context(), "unknown-device")
		_, unknownDeviceErr := runEnrollDevice(ctx, client, dev.EnrollDeviceInit("some-token"), dev.SignChallenge)
		assertErrorIsAndEqual(t, unknownDeviceErr, dterrors.ErrInvalidDeviceEnrollToken)

		registered, err := osstestenv.NewFakeIOSDevice(devicepb.OSType_OS_TYPE_IOS)
		require.NoError(t, err)
		registeredDev := registerDevice(t, env, registered.CollectDeviceData())
		ctx = testenv.WithOutgoingEmitterKey(t.Context(), "bad-token")
		_, badTokenErr := runEnrollDevice(ctx, client, registered.EnrollDeviceInit("bad-token"), registered.SignChallenge)
		assert.Equal(t, unknownDeviceErr.Error(), badTokenErr.Error())

		// The failure is audited without a user, as none could be resolved. The
		// audit trail keeps the real error behind the redacted one.
		evt := lastDeviceEvent(t, emitter.LastEvent("bad-token"))
		assert.False(t, evt.Status.Success)
		assert.Empty(t, evt.UserMetadata.User)
		assert.Equal(t, registeredDev.GetId(), evt.Device.DeviceId)

		unknownEvt := lastDeviceEvent(t, emitter.LastEvent("unknown-device"))
		assert.Contains(t, unknownEvt.Status.Error, "device not found")
		assert.Empty(t, unknownEvt.Device.DeviceId)
	})

	t.Run("rejects a token that is not bound to a user", func(t *testing.T) {
		t.Parallel()
		dev, err := osstestenv.NewFakeIOSDevice(devicepb.OSType_OS_TYPE_IOS)
		require.NoError(t, err)
		registered := registerDevice(t, env, dev.CollectDeviceData())
		token := createAdminEnrollToken(t, env, registered.GetId())

		ctx := testenv.WithOutgoingEmitterKey(t.Context(), "admin-token")
		_, err = runEnrollDevice(ctx, client, dev.EnrollDeviceInit(token), dev.SignChallenge)
		assertErrorIsAndEqual(t, err, dterrors.ErrInvalidDeviceEnrollToken)

		// The redacted error is told apart from the other token failures only in the
		// audit trail.
		evt := lastDeviceEvent(t, emitter.LastEvent("admin-token"))
		assert.False(t, evt.Status.Success)
		assert.Contains(t, evt.Status.Error, "token has no user")
		assert.Equal(t, registered.GetId(), evt.Device.DeviceId)
	})

	t.Run("returns NotFound when the token user no longer exists", func(t *testing.T) {
		t.Parallel()
		createUser(t, env, "erin")
		dev, err := osstestenv.NewFakeIOSDevice(devicepb.OSType_OS_TYPE_IOS)
		require.NoError(t, err)
		registerDevice(t, env, dev.CollectDeviceData())
		token := createUserBoundEnrollToken(t, env, "erin", dev.CollectDeviceData())
		require.NoError(t, env.IdentityService.DeleteUser(t.Context(), "erin"))

		ctx := testenv.WithOutgoingEmitterKey(t.Context(), "erin-gone")
		_, err = runEnrollDevice(ctx, client, dev.EnrollDeviceInit(token), dev.SignChallenge)
		// The caller never supplies a username, it comes out of the verified
		// token's own record, so it's safe to disclose this NotFound error.
		// This should help somewhat in situations where an SSO user expires before
		// the device is enrolled.
		assert.ErrorAs(t, err, new(*trace.NotFoundError))
		assert.ErrorContains(t, err, "user not found")
	})

	t.Run("a failed ceremony spends the token but doesn't enroll the device", func(t *testing.T) {
		t.Parallel()
		createUser(t, env, "carol")
		dev, err := osstestenv.NewFakeIOSDevice(devicepb.OSType_OS_TYPE_IOS)
		require.NoError(t, err)
		registerDevice(t, env, dev.CollectDeviceData())
		token := createUserBoundEnrollToken(t, env, "carol", dev.CollectDeviceData())

		ctx := testenv.WithOutgoingEmitterKey(t.Context(), "carol")
		_, err = runEnrollDevice(ctx, client, dev.EnrollDeviceInit(token),
			func([]byte) ([]byte, error) { return []byte("not a valid signature"), nil })
		assert.ErrorAs(t, err, new(*trace.BadParameterError))
		assert.ErrorContains(t, err, "signature verification failed")

		evt := lastDeviceEvent(t, emitter.LastEvent("carol"))
		assert.False(t, evt.Status.Success)
		assert.Equal(t, "carol", evt.UserMetadata.User)

		// The ceremony spends the token before the challenge, so the failed attempt
		// consumed it.
		_, err = runEnrollDevice(ctx, client, dev.EnrollDeviceInit(token), dev.SignChallenge)
		assertErrorIsAndEqual(t, err, dterrors.ErrInvalidDeviceEnrollToken)

		// The device is not enrolled: a fresh token and a correct signature enroll
		// it all the way.
		token = createUserBoundEnrollToken(t, env, "carol", dev.CollectDeviceData())
		resp, err := runEnrollDevice(ctx, client, dev.EnrollDeviceInit(token), dev.SignChallenge)
		require.NoError(t, err)
		assert.NotNil(t, resp.GetSuccess())
	})

	t.Run("an erased failure after the token is spent is terminal", func(t *testing.T) {
		t.Parallel()
		createUser(t, env, "grace")
		dev, err := osstestenv.NewFakeIOSDevice(devicepb.OSType_OS_TYPE_IOS)
		require.NoError(t, err)
		registered := registerDevice(t, env, dev.CollectDeviceData())
		token := createUserBoundEnrollToken(t, env, "grace", dev.CollectDeviceData())

		ctx := testenv.WithOutgoingEmitterKey(t.Context(), "grace")
		stream, err := client.EnrollDevice(ctx)
		require.NoError(t, err)
		require.NoError(t, stream.Send(devicetrustpublicv1pb.EnrollDeviceRequest_builder{
			Init: dev.EnrollDeviceInit(token),
		}.Build()))
		resp, err := stream.Recv()
		require.NoError(t, err)
		require.NotNil(t, resp.GetIosChallenge(), "expected IOSEnrollChallenge, got %v", resp)

		// The token is spent by the time the challenge arrives. Removing the device
		// now makes the final storage write fail with an error the caller must not
		// see, past the point where a retry with the same token could succeed.
		deleteDevice(t, env, registered.GetId())

		sig, err := dev.SignChallenge(resp.GetIosChallenge().GetChallenge())
		require.NoError(t, err)
		require.NoError(t, stream.Send(devicetrustpublicv1pb.EnrollDeviceRequest_builder{
			IosChallengeResponse: devicetrustpublicv1pb.IOSEnrollChallengeResponse_builder{
				Signature: sig,
			}.Build(),
		}.Build()))
		_, err = stream.Recv()
		assertErrorIsAndEqual(t, err, devicetrustpublicv1.ErrEnrollDeviceFailed)

		// The audit trail keeps the real error behind the redacted one.
		evt := lastDeviceEvent(t, emitter.LastEvent("grace"))
		assert.False(t, evt.Status.Success)
		assert.Contains(t, evt.Status.Error, "not found")
	})

	t.Run("rejects a second message that is not a challenge response", func(t *testing.T) {
		t.Parallel()
		createUser(t, env, "dave")
		dev, err := osstestenv.NewFakeIOSDevice(devicepb.OSType_OS_TYPE_IOS)
		require.NoError(t, err)
		registerDevice(t, env, dev.CollectDeviceData())
		token := createUserBoundEnrollToken(t, env, "dave", dev.CollectDeviceData())

		ctx := testenv.WithOutgoingEmitterKey(t.Context(), "dave")
		stream, err := client.EnrollDevice(ctx)
		require.NoError(t, err)
		require.NoError(t, stream.Send(devicetrustpublicv1pb.EnrollDeviceRequest_builder{
			Init: dev.EnrollDeviceInit(token),
		}.Build()))
		resp, err := stream.Recv()
		require.NoError(t, err)
		require.NotNil(t, resp.GetIosChallenge(), "expected IOSEnrollChallenge, got %v", resp)

		// Replay the init instead of answering the challenge. The error is written
		// for the caller, so redactEnrollError must pass it through intact.
		require.NoError(t, stream.Send(devicetrustpublicv1pb.EnrollDeviceRequest_builder{
			Init: dev.EnrollDeviceInit(token),
		}.Build()))
		_, err = stream.Recv()
		assert.ErrorAs(t, err, new(*trace.BadParameterError))
		assert.ErrorContains(t, err, "expected IOSEnrollChallengeResponse")
	})

	t.Run("redacts a transient failure of the user authorization", func(t *testing.T) {
		t.Parallel()
		authorizer := &fakeAuthorizer{authorizedUsers: []string{"henry"}}
		emitter := &testenv.KeyedEmitter{}
		// The subtest injects failures into its authorizer, so it builds an env of
		// its own instead of sharing the suite's.
		env := testenv.NewUsingT(t,
			testenv.WithAuthorizer(authorizer),
			testenv.WithEmitter(emitter),
		)

		createUser(t, env, "henry")
		dev, err := osstestenv.NewFakeIOSDevice(devicepb.OSType_OS_TYPE_IOS)
		require.NoError(t, err)
		registerDevice(t, env, dev.CollectDeviceData())
		token := createUserBoundEnrollToken(t, env, "henry", dev.CollectDeviceData())

		// Set userAuthorizeErr to a transient backend failure rather than a
		// denial: its details must not reach the caller, unlike the deliberate
		// NotFound and AccessDenied outcomes.
		authorizer.userAuthorizeErr = trace.Errorf("backend shard 7 unavailable")

		ctx := testenv.WithOutgoingEmitterKey(t.Context(), "henry")
		_, err = runEnrollDevice(ctx, env.PublicDevicesClient, dev.EnrollDeviceInit(token), dev.SignChallenge)
		assertErrorIsAndEqual(t, err, devicetrustpublicv1.ErrUserAuthzUnavailable)

		// The audit trail keeps the real error behind the redacted one.
		evt := lastDeviceEvent(t, emitter.LastEvent("henry"))
		assert.False(t, evt.Status.Success)
		assert.Equal(t, "henry", evt.UserMetadata.User)
		assert.Contains(t, evt.Status.Error, "shard")
	})
}

// TestService_EnrollDevice_authzFailurePreservesToken verifies that the rule
// check runs against the user reconstructed from the token, and, unlike in the
// private service, before the token is spent, so a rejected attempt doesn't
// burn the token.
func TestService_EnrollDevice_authzFailurePreservesToken(t *testing.T) {
	t.Parallel()

	authorizer := &fakeAuthorizer{authorizedUsers: []string{"frank"}}
	emitter := &testenv.KeyedEmitter{}
	env := testenv.NewUsingT(t,
		testenv.WithAuthorizer(authorizer),
		testenv.WithEmitter(emitter),
	)

	createUser(t, env, "frank")
	dev, err := osstestenv.NewFakeIOSDevice(devicepb.OSType_OS_TYPE_IOS)
	require.NoError(t, err)
	registered := registerDevice(t, env, dev.CollectDeviceData())
	token := createUserBoundEnrollToken(t, env, "frank", dev.CollectDeviceData())

	// Deny the rule check, simulating a user whose permissions were revoked
	// between token issuance and the enrollment attempt.
	authorizer.checker = &fakeChecker{authorized: false}

	ctx := testenv.WithOutgoingEmitterKey(t.Context(), "frank")
	_, err = runEnrollDevice(ctx, env.PublicDevicesClient, dev.EnrollDeviceInit(token), dev.SignChallenge)
	assert.ErrorAs(t, err, new(*trace.AccessDeniedError))
	assert.ErrorContains(t, err, "access denied")

	evt := lastDeviceEvent(t, emitter.LastEvent("frank"))
	assert.False(t, evt.Status.Success)
	assert.Equal(t, "frank", evt.UserMetadata.User)
	assert.Equal(t, registered.GetId(), evt.Device.DeviceId)

	authorizer.checker = nil
	resp, err := runEnrollDevice(ctx, env.PublicDevicesClient, dev.EnrollDeviceInit(token), dev.SignChallenge)
	require.NoError(t, err)
	assert.NotNil(t, resp.GetSuccess())
}

func TestService_EnrollDevice_badParameters(t *testing.T) {
	t.Parallel()

	emitter := &testenv.KeyedEmitter{}
	env := testenv.NewUsingT(t,
		testenv.WithAuthorizer(&fakeAuthorizer{}),
		testenv.WithEmitter(emitter),
	)

	dev, err := osstestenv.NewFakeIOSDevice(devicepb.OSType_OS_TYPE_IOS)
	require.NoError(t, err)

	makeInit := func(mutate func(init *devicetrustpublicv1pb.EnrollDeviceInit)) *devicetrustpublicv1pb.EnrollDeviceInit {
		init := dev.EnrollDeviceInit("fake-token")
		mutate(init)
		return init
	}

	for _, test := range []struct {
		name    string
		init    *devicetrustpublicv1pb.EnrollDeviceInit
		wantErr string
	}{
		{
			name:    "missing token",
			init:    makeInit(func(init *devicetrustpublicv1pb.EnrollDeviceInit) { init.SetToken("") }),
			wantErr: "enrollment token required",
		},
		{
			name:    "missing device data",
			init:    makeInit(func(init *devicetrustpublicv1pb.EnrollDeviceInit) { init.ClearDeviceData() }),
			wantErr: "device data required",
		},
		{
			name: "missing OS type",
			init: makeInit(func(init *devicetrustpublicv1pb.EnrollDeviceInit) {
				init.GetDeviceData().SetOsType(devicepb.OSType_OS_TYPE_UNSPECIFIED)
			}),
			wantErr: "device OS type required",
		},
		{
			name: "missing serial number",
			init: makeInit(func(init *devicetrustpublicv1pb.EnrollDeviceInit) {
				init.GetDeviceData().SetSerialNumber("")
			}),
			wantErr: "device serial number required",
		},
		{
			// Desktop devices enroll through the private service. The request is
			// rejected based on the submitted OS type, before any storage access, so
			// a stolen desktop-device token cannot be probed for validity through
			// this RPC.
			//
			// Devices are resolved by (serial number, OS type), so if an attacker
			// steals a desktop token and then lies about OS type, they still won't be
			// able to probe anything through the public EnrollDevice RPC.
			name: "OS type outside iOS and iPadOS",
			init: makeInit(func(init *devicetrustpublicv1pb.EnrollDeviceInit) {
				init.GetDeviceData().SetOsType(devicepb.OSType_OS_TYPE_MACOS)
			}),
			wantErr: "unsupported OS type",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctx := testenv.WithOutgoingEmitterKey(t.Context(), test.name)
			_, err := runEnrollDevice(ctx, env.PublicDevicesClient, test.init, dev.SignChallenge)
			assert.ErrorAs(t, err, new(*trace.BadParameterError))
			assert.ErrorContains(t, err, test.wantErr)
			assert.Empty(t, emitter.Events(test.name))
		})
	}

	t.Run("first message is not init", func(t *testing.T) {
		t.Parallel()
		ctx := testenv.WithOutgoingEmitterKey(t.Context(), "not-init")
		stream, err := env.PublicDevicesClient.EnrollDevice(ctx)
		require.NoError(t, err)
		require.NoError(t, stream.Send(devicetrustpublicv1pb.EnrollDeviceRequest_builder{
			IosChallengeResponse: devicetrustpublicv1pb.IOSEnrollChallengeResponse_builder{
				Signature: []byte("signature"),
			}.Build(),
		}.Build()))
		_, err = stream.Recv()
		assert.ErrorAs(t, err, new(*trace.BadParameterError))
		assert.ErrorContains(t, err, "expected EnrollDeviceInit")
		assert.Empty(t, emitter.Events("not-init"))
	})
}

// TestService_EnrollDevice_timeout checks that the timeout ends a stream whose
// client stalls. Before the init message the handler cannot even identify the
// caller, so ending the stream on its own is the only way to free the handler,
// and the proxy stream in front of it.
func TestService_EnrollDevice_timeout(t *testing.T) {
	t.Parallel()

	// awaitTimeout waits for the stream to end and asserts that the timeout is
	// what ended it. Recv runs in a goroutine, so that a regression that leaves
	// the handler blocked fails the test instead of hanging it.
	awaitTimeout := func(t *testing.T, stream devicetrustpublicv1pb.DeviceTrustService_EnrollDeviceClient, start time.Time) {
		t.Helper()
		errCh := make(chan error, 1)
		go func() {
			_, err := stream.Recv()
			errCh <- err
		}()
		select {
		case err := <-errCh:
			assertErrorIsAndEqual(t, err, devicetrustpublicv1.ErrEnrollDeviceTimeout)
			assert.Equal(t, devicetrustpublicv1.EnrollDeviceTimeout, time.Since(start))
		case <-time.After(devicetrustpublicv1.EnrollDeviceTimeout + time.Minute):
			t.Fatal("timed out waiting for EnrollDevice to end the stream")
		}
	}

	t.Run("client never sends init", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			env := testenv.NewUsingT(t, testenv.WithAuthorizer(&fakeAuthorizer{}))

			start := time.Now()
			stream, err := env.PublicDevicesClient.EnrollDevice(t.Context())
			require.NoError(t, err)
			awaitTimeout(t, stream, start)
		})
	})

	t.Run("client never answers the challenge", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			emitter := &testenv.KeyedEmitter{}
			env := testenv.NewUsingT(t,
				testenv.WithAuthorizer(&fakeAuthorizer{authorizedUsers: []string{"alice"}}),
				testenv.WithEmitter(emitter),
			)
			createUser(t, env, "alice")
			dev, err := osstestenv.NewFakeIOSDevice(devicepb.OSType_OS_TYPE_IOS)
			require.NoError(t, err)
			registerDevice(t, env, dev.CollectDeviceData())
			token := createUserBoundEnrollToken(t, env, "alice", dev.CollectDeviceData())

			start := time.Now()
			ctx := testenv.WithOutgoingEmitterKey(t.Context(), "alice")
			stream, err := env.PublicDevicesClient.EnrollDevice(ctx)
			require.NoError(t, err)
			require.NoError(t, stream.Send(devicetrustpublicv1pb.EnrollDeviceRequest_builder{
				Init: dev.EnrollDeviceInit(token),
			}.Build()))
			resp, err := stream.Recv()
			require.NoError(t, err)
			require.NotNil(t, resp.GetIosChallenge(), "expected IOSEnrollChallenge, got %v", resp)
			awaitTimeout(t, stream, start)

			// The ceremony audits once its pending Recv fails, which happens after
			// the handler has returned and the client has seen the error. The audit
			// trail must name the timeout, not the stream teardown it caused.
			synctest.Wait()
			evt := lastDeviceEvent(t, emitter.LastEvent("alice"))
			assert.False(t, evt.Status.Success)
			assert.Equal(t, devicetrustpublicv1.ErrEnrollDeviceTimeout.Error(), evt.Status.Error)
		})
	})
}

// TestRedactEnrollError pins that the public boundary redacts errors
// deny-by-default: only ceremony errors written for the caller pass through,
// anything else is erased, including errors added to the shared ceremony in the
// future.
func TestRedactEnrollError(t *testing.T) {
	t.Parallel()

	t.Run("nil stays nil", func(t *testing.T) {
		assert.NoError(t, devicetrustpublicv1.RedactEnrollError(t.Context(), nil, false /* tokenSpent */))
	})

	t.Run("caller-facing errors pass through", func(t *testing.T) {
		err := devicetrustpublicv1.RedactEnrollError(t.Context(),
			trace.Wrap(dterrors.ErrInvalidDeviceEnrollToken), false /* tokenSpent */)
		assert.ErrorIs(t, err, dterrors.ErrInvalidDeviceEnrollToken)

		// A bad signature is only ever detected after the token is spent.
		payloadErr := trace.BadParameter("signature verification failed")
		err = devicetrustpublicv1.RedactEnrollError(t.Context(), payloadErr, true /* tokenSpent */)
		assert.ErrorIs(t, err, payloadErr)
	})

	t.Run("drift is reported without the drifted field", func(t *testing.T) {
		driftErr := trace.Wrap(storage.NewCollectedDataDriftError("os_type drift detected"))
		err := devicetrustpublicv1.RedactEnrollError(t.Context(), driftErr, true /* tokenSpent */)
		assert.ErrorAs(t, err, new(*trace.AccessDeniedError))
		assert.EqualError(t, err, devicetrustv1.DataDriftDetectedMessage)
		assert.NotContains(t, err.Error(), "os_type")
	})

	t.Run("anything else is erased", func(t *testing.T) {
		for _, unexpected := range []error{
			trace.NotFound("device %q/%v not registered", "serial", "macOS"),
			trace.Errorf("shard 7 on fire"),
		} {
			err := devicetrustpublicv1.RedactEnrollError(t.Context(), unexpected, false /* tokenSpent */)
			assert.ErrorIs(t, err, devicetrustpublicv1.ErrEnrollDeviceUnavailable)
			assert.NotContains(t, err.Error(), "not registered")
			assert.NotContains(t, err.Error(), "shard")

			// Once the token is spent a retry cannot succeed, so the erased error
			// turns terminal.
			err = devicetrustpublicv1.RedactEnrollError(t.Context(), unexpected, true /* tokenSpent */)
			assert.ErrorIs(t, err, devicetrustpublicv1.ErrEnrollDeviceFailed)
			assert.NotContains(t, err.Error(), "not registered")
			assert.NotContains(t, err.Error(), "shard")
		}
	})

	t.Run("cancellation surfaces as the context error", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		err := devicetrustpublicv1.RedactEnrollError(ctx, trace.Errorf("stream torn down"), true /* tokenSpent */)
		assert.ErrorIs(t, err, context.Canceled)
	})
}

// fakeAuthorizer stands in for both Authorize calls the handler makes: to
// verify the incoming Proxy identity and to verify the pairing user
// reconstructed from the pairing token.
type fakeAuthorizer struct {
	authorizedUsers []string
	// checker overrides the fakeChecker derived from authorizedUsers, so that a
	// test can assert which rule and verb the handler checks.
	checker services.AccessChecker
	// userAuthorizeErr, when set, fails the Authorize call for a reconstructed
	// user, simulating a transient authorization failure rather than a denial.
	userAuthorizeErr error
}

// Authorize returns either a context with the user (if the user was injected
// into the context by Service.authorizeUser) or the context with the proxy.
func (a *fakeAuthorizer) Authorize(ctx context.Context) (*authz.Context, error) {
	if u, err := authz.UserFromContext(ctx); err == nil {
		if localUser, ok := u.(authz.LocalUser); ok {
			if a.userAuthorizeErr != nil {
				return nil, a.userAuthorizeErr
			}
			user, err := types.NewUser(localUser.Username)
			if err != nil {
				return nil, err
			}
			checker := a.checker
			if localUser.Username == deviceRegistrar {
				// The registrar exists so that [registerDevice] can register devices
				// through the private service without widening what the tested users
				// are allowed to do.
				checker = &registrarChecker{}
			} else if checker == nil {
				checker = &fakeChecker{
					authorized: slices.Contains(a.authorizedUsers, localUser.Username),
				}
			}
			return &authz.Context{
				User:                 user,
				Identity:             localUser,
				Checker:              checker,
				AdminActionAuthState: authz.AdminActionAuthNotRequired,
			}, nil
		}
	}

	return authz.ContextForBuiltinRole(authz.BuiltinRole{
		Role:     types.RoleProxy,
		Username: string(types.RoleProxy),
	}, nil)
}

// fakeChecker allows the mobile_device.create_enroll_token permission for
// authorized users only.
type fakeChecker struct {
	testenv.NoopChecker
	authorized bool
}

func (c *fakeChecker) CheckAccessToRule(_ services.RuleContext, _, rule, verb string) error {
	if rule != types.KindMobileDevice || verb != types.VerbCreateEnrollToken {
		return trace.BadParameter("unexpected rule/verb: %s/%s", rule, verb)
	}
	if !c.authorized {
		return trace.AccessDenied("access denied")
	}
	return nil
}

// deviceRegistrar is the identity the device fixtures ([registerDevice],
// [deleteDevice], [createAdminEnrollToken]) use to call the private Device
// Trust service.
const deviceRegistrar = "device-registrar"

// registrarChecker authorizes the device fixtures, registration, removal and
// admin-issued enrollment tokens, and nothing else.
type registrarChecker struct {
	testenv.NoopChecker
}

func (registrarChecker) CheckAccessToRule(_ services.RuleContext, _, rule, verb string) error {
	if rule == types.KindDevice && (verb == types.VerbCreate || verb == types.VerbDelete || verb == types.VerbCreateEnrollToken) {
		return nil
	}
	return trace.AccessDenied("access denied to %v/%v", rule, verb)
}

type wantRuleVerb struct {
	rule, verb string
}

type ruleVerifyingChecker struct {
	testenv.NoopChecker
	want []wantRuleVerb
}

func (c *ruleVerifyingChecker) CheckAccessToRule(ruleCtx services.RuleContext, namespace string, rule string, verb string) error {
	if namespace != defaults.Namespace {
		return fmt.Errorf("unexpected namespace: %v", namespace)
	}

	for i, want := range c.want {
		if want.rule == rule && want.verb == verb {
			c.want = slices.Delete(c.want, i, i+1) // cut
			return nil
		}
	}

	return fmt.Errorf("CheckAccessToRule called with an unexpected rule+verb pair: %v %v", rule, verb)
}

// verifyMatches returns an error if any wanted matches are still unfulfilled.
func (c *ruleVerifyingChecker) verifyMatches() error {
	// CheckAccessToRule removes c.want entries on a positive match.
	// An empty slice means all wanted rules got a match.
	if len(c.want) == 0 {
		return nil
	}
	return fmt.Errorf("CheckAccessToRule not called for the following wanted matches: %v", c.want)
}

func makeRequest(token string, cd *devicepb.DeviceCollectedData) *devicetrustpublicv1pb.CreatePairedDeviceEnrollTokenRequest {
	return devicetrustpublicv1pb.CreatePairedDeviceEnrollTokenRequest_builder{
		EnrollPairingToken: token,
		DeviceData:         cd,
	}.Build()
}

func makeCollectedData() *devicepb.DeviceCollectedData {
	return devicepb.DeviceCollectedData_builder{
		OsType:       devicepb.OSType_OS_TYPE_IOS,
		SerialNumber: "llama",
		OsVersion:    "26.3.1",
		CollectTime:  timestamppb.Now(),
	}.Build()
}

// makeCollectedDataFor is makeCollectedData with a specific serial number, for
// subtests that register their own device and so need a serial number no other
// subtest has claimed.
func makeCollectedDataFor(serial string) *devicepb.DeviceCollectedData {
	cd := makeCollectedData()
	cd.SetSerialNumber(serial)
	return cd
}

// registerDevice adds the device described by cd to the inventory, which
// CreatePairedDeviceEnrollToken needs before it can mint an enrollment token.
// It goes through the private Device Trust service as the registrar identity,
// so the tests stay on the RPC surface.
// TODO(ravicious): Consider replacing this helper with something like this:
//
//	testenv.WithRegisteredDevices(device1, device2)
//
// See https://github.com/gravitational/core/pull/558#discussion_r3873731448.
func registerDevice(t *testing.T, env *testenv.E, cd *devicepb.DeviceCollectedData) *devicepb.Device {
	t.Helper()
	ctx := authz.ContextWithUser(t.Context(), authz.LocalUser{Username: deviceRegistrar})
	dev, err := env.DevicesService.CreateDevice(ctx, devicepb.CreateDeviceRequest_builder{
		Device: devicepb.Device_builder{
			OsType:   cd.GetOsType(),
			AssetTag: cd.GetSerialNumber(),
		}.Build(),
	}.Build())
	require.NoError(t, err)
	return dev
}

// createAdminEnrollToken mints an enrollment token for the device through the
// private Device Trust service. Admin-issued tokens carry no user and can't be
// used for enrollment through the public service.
func createAdminEnrollToken(t *testing.T, env *testenv.E, deviceID string) string {
	t.Helper()
	ctx := authz.ContextWithUser(t.Context(), authz.LocalUser{Username: deviceRegistrar})
	token, err := env.DevicesService.CreateDeviceEnrollToken(ctx, devicepb.CreateDeviceEnrollTokenRequest_builder{
		DeviceId: deviceID,
	}.Build())
	require.NoError(t, err)
	return token.GetToken()
}

// deleteDevice removes the device from the inventory through the private Device
// Trust service.
func deleteDevice(t *testing.T, env *testenv.E, deviceID string) {
	t.Helper()
	ctx := authz.ContextWithUser(t.Context(), authz.LocalUser{Username: deviceRegistrar})
	_, err := env.DevicesService.DeleteDevice(ctx, devicepb.DeleteDeviceRequest_builder{
		DeviceId: deviceID,
	}.Build())
	require.NoError(t, err)
}

// createUserBoundEnrollToken mints an enrollment token bound to user by running
// the full pairing flow: create, claim, approve, issue. The device described by
// cd must already be registered and user must exist and pass authz.
//
// The fixture uses an emitter key of its own so that its audit events don't
// pollute the calling test's assertions – its primary use case is for tests of
// the flow beyond CreatePairedDeviceEnrollToken.
func createUserBoundEnrollToken(t *testing.T, env *testenv.E, user string, cd *devicepb.DeviceCollectedData) string {
	t.Helper()
	token := createPairing(t, env, user)
	approvePairing(t, env, token, cd)
	ctx := testenv.WithOutgoingEmitterKey(t.Context(), user+"-token-fixture")
	resp, err := env.PublicDevicesClient.CreatePairedDeviceEnrollToken(ctx, makeRequest(token, cd))
	require.NoError(t, err)
	return resp.GetDeviceEnrollToken().GetToken()
}

// approvePairing drives the pairing behind token through a claim by the device
// described by cd and the user's approval, leaving it APPROVED with no handler
// waiting on it.
func approvePairing(t *testing.T, env *testenv.E, token string, cd *devicepb.DeviceCollectedData) {
	t.Helper()
	pairing, err := env.EnrollPairing.GetEnrollPairingByToken(t.Context(), token)
	require.NoError(t, err)

	claimed, err := env.EnrollPairing.RequestEnrollPairingApproval(t.Context(), pairing,
		devicepb.EnrollPairingDevice_builder{
			OsType:       cd.GetOsType(),
			SerialNumber: cd.GetSerialNumber(),
			OsVersion:    cd.GetOsVersion(),
		}.Build())
	require.NoError(t, err)

	_, err = env.EnrollPairing.ApproveEnrollPairing(t.Context(), claimed)
	require.NoError(t, err)
}

// createTokenAsync calls CreatePairedDeviceEnrollToken in a goroutine. The RPC
// blocks until the pairing is approved, denied or expires, so tests drive the
// pairing while the call is in flight.
func createTokenAsync(ctx context.Context, client devicetrustpublicv1pb.DeviceTrustServiceClient,
	token string, cd *devicepb.DeviceCollectedData) <-chan tokenResult {
	ch := make(chan tokenResult, 1)
	go func() {
		resp, err := client.CreatePairedDeviceEnrollToken(ctx, makeRequest(token, cd))
		ch <- tokenResult{resp: resp, err: err}
	}()
	return ch
}

// tokenResult is the value [createTokenAsync] delivers on its channel.
type tokenResult struct {
	resp *devicetrustpublicv1pb.CreatePairedDeviceEnrollTokenResponse
	err  error
}

// waitForPairingState waits for the pairing behind token to reach state, which
// is how a test knows the backgrounded handler got far enough to act on.
//
// It must run inside a synctest bubble: once every goroutine in it is durably
// blocked, the CreatePairedDeviceEnrollToken RPC handler is parked in its poll,
// past the writes the test wants to see, so a single read suffices.
func waitForPairingState(t *testing.T, env *testenv.E, token string, state devicepb.EnrollPairingState) *devicepb.EnrollPairing {
	t.Helper()
	synctest.Wait()
	pairing, err := env.EnrollPairing.GetEnrollPairingByToken(t.Context(), token)
	require.NoError(t, err)
	require.Equal(t, state, pairing.GetStatus().GetState())
	return pairing
}

// awaitResult waits for a [createTokenAsync] result. The 10s failsafe exists so
// that a handler that never returns fails the test rather than hanging it.
// Under synctest there's no cost to having it anyway.
func awaitResult(t *testing.T, ch <-chan tokenResult) tokenResult {
	t.Helper()
	select {
	case res := <-ch:
		return res
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for CreatePairedDeviceEnrollToken")
		return tokenResult{}
	}
}

// deviceEvents converts raw emitted events for subtests that pin more than one
// event, preserving emit order.
func deviceEvents(t *testing.T, raw []apievents.AuditEvent) []*apievents.DeviceEvent2 {
	t.Helper()
	var evts []*apievents.DeviceEvent2
	for _, e := range raw {
		evt, ok := e.(*apievents.DeviceEvent2)
		require.True(t, ok, "expected *apievents.DeviceEvent2, got %T", e)
		evts = append(evts, evt)
	}
	return evts
}

func lastDeviceEvent(t *testing.T, last apievents.AuditEvent) *apievents.DeviceEvent2 {
	t.Helper()
	require.NotNil(t, last, "expected the emitter to contain at least one event")
	evt, ok := last.(*apievents.DeviceEvent2)
	require.True(t, ok, "expected *apievents.DeviceEvent2, got %T", last)
	return evt
}

// assertErrorIsAndEqual asserts that err matches sentinel and that the entire
// user-visible message is the sentinel's, with nothing leaked around it.
// [assert.ErrorIs] alone stops pinning the exact message once the sentinel is
// wrapped with added context. [assert.EqualError] alone ignores the error type.
func assertErrorIsAndEqual(t *testing.T, err, sentinel error) {
	t.Helper()
	assert.ErrorIs(t, err, sentinel)
	assert.EqualError(t, err, sentinel.Error())
}

// createPairing seeds an AWAITING_DEVICE pairing for user and returns its token.
func createPairing(t *testing.T, env *testenv.E, user string) string {
	t.Helper()
	p, err := env.EnrollPairing.CreateEnrollPairing(t.Context(), user)
	require.NoError(t, err)
	return p.GetStatus().GetToken()
}

func createUser(t *testing.T, env *testenv.E, name string) {
	t.Helper()
	u, err := types.NewUser(name)
	require.NoError(t, err)
	_, err = env.IdentityService.CreateUser(t.Context(), u)
	require.NoError(t, err)
}

// builtinRoleAuthorizer authorizes the caller as a single builtin role.
type builtinRoleAuthorizer struct {
	role types.SystemRole
}

func (a builtinRoleAuthorizer) Authorize(ctx context.Context) (*authz.Context, error) {
	return authz.ContextForBuiltinRole(authz.BuiltinRole{
		Role:     a.role,
		Username: string(a.role),
	}, nil)
}

// racingEnrollPairing simulates a competing device claiming the pairing in the
// window between the handler's read and its compare-and-swap: the first
// GetEnrollPairingByToken advances the backend via a competing claim before
// handing back the still-AWAITING_DEVICE copy, so the handler's swap fails and
// it re-reads to find the winner.
type racingEnrollPairing struct {
	services.EnrollPairing
	competitor *devicepb.EnrollPairingDevice
	raced      bool
}

func (r *racingEnrollPairing) GetEnrollPairingByToken(ctx context.Context, token string) (*devicepb.EnrollPairing, error) {
	pairing, err := r.EnrollPairing.GetEnrollPairingByToken(ctx, token)
	if err != nil || r.raced {
		return pairing, err
	}
	r.raced = true

	competing, err := r.EnrollPairing.GetEnrollPairingByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if _, err := r.EnrollPairing.RequestEnrollPairingApproval(ctx, competing, r.competitor); err != nil {
		return nil, err
	}
	return pairing, nil
}

// newLookupFailureEnv builds an env whose enroll pairing lookups can be made to
// fail on demand, seeds the user and returns the token of a pairing it owns.
func newLookupFailureEnv(t *testing.T, user string) (*testenv.E, *failingEnrollPairing, string) {
	t.Helper()

	bk, err := memory.New(memory.Config{Context: t.Context()})
	require.NoError(t, err)
	t.Cleanup(func() { _ = bk.Close() })
	real, err := local.NewEnrollPairingService(bk)
	require.NoError(t, err)

	failing := &failingEnrollPairing{EnrollPairing: real}
	env := testenv.NewUsingT(t,
		testenv.WithAuthorizer(&fakeAuthorizer{authorizedUsers: []string{user}}),
		testenv.WithEnrollPairing(failing),
	)
	createUser(t, env, user)
	return env, failing, createPairing(t, env, user)
}

// failingEnrollPairing fails every pairing lookup, and separately the claim
// swap, once armed, standing in for a backend that goes bad partway through the
// flow.
type failingEnrollPairing struct {
	services.EnrollPairing

	mu      sync.Mutex
	err     error
	swapErr error
}

// failWith arms the failure. Lookups made before it, by the handler or by the
// test itself, hit the real service.
func (f *failingEnrollPairing) failWith(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

func (f *failingEnrollPairing) GetEnrollPairingByToken(ctx context.Context, token string) (*devicepb.EnrollPairing, error) {
	f.mu.Lock()
	err := f.err
	f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return f.EnrollPairing.GetEnrollPairingByToken(ctx, token)
}

// failSwapWith arms the claim swap failure alone, so the swap fails while
// lookups keep hitting the real service.
func (f *failingEnrollPairing) failSwapWith(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.swapErr = err
}

func (f *failingEnrollPairing) RequestEnrollPairingApproval(ctx context.Context, pairing *devicepb.EnrollPairing, device *devicepb.EnrollPairingDevice) (*devicepb.EnrollPairing, error) {
	f.mu.Lock()
	err := f.swapErr
	f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return f.EnrollPairing.RequestEnrollPairingApproval(ctx, pairing, device)
}

// staleEnrollPairing keeps returning the last pairing it saw after storage
// forgets it, standing in for a backend that missed the TTL expiry, which is
// what the approval wait timeout guards against.
type staleEnrollPairing struct {
	services.EnrollPairing

	mu   sync.Mutex
	last *devicepb.EnrollPairing
}

func (s *staleEnrollPairing) GetEnrollPairingByToken(ctx context.Context, token string) (*devicepb.EnrollPairing, error) {
	pairing, err := s.EnrollPairing.GetEnrollPairingByToken(ctx, token)
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case err == nil:
		s.last = pairing
		return pairing, nil
	case trace.IsNotFound(err) && s.last != nil:
		return s.last, nil
	default:
		return nil, err
	}
}

// blockingDeleteEnrollPairing blocks each DeleteEnrollPairing until all
// expected waiters arrive at theirs, so that concurrent waiters race the
// conditional delete itself rather than one of them consuming the pairing
// before the other finishes its post-approval read.
type blockingDeleteEnrollPairing struct {
	services.EnrollPairing
	arrivals sync.WaitGroup
}

func newBlockingDeleteEnrollPairing(real services.EnrollPairing, waiters int) *blockingDeleteEnrollPairing {
	b := &blockingDeleteEnrollPairing{EnrollPairing: real}
	b.arrivals.Add(waiters)
	return b
}

func (b *blockingDeleteEnrollPairing) DeleteEnrollPairing(ctx context.Context, pairing *devicepb.EnrollPairing) error {
	b.arrivals.Done()
	b.arrivals.Wait()
	return b.EnrollPairing.DeleteEnrollPairing(ctx, pairing)
}

// failingDeleteEnrollPairing fails every DeleteEnrollPairing with a backend
// error, standing in for storage trouble while the pairing is consumed.
type failingDeleteEnrollPairing struct {
	services.EnrollPairing
	err error
}

func (f *failingDeleteEnrollPairing) DeleteEnrollPairing(ctx context.Context, pairing *devicepb.EnrollPairing) error {
	return f.err
}

// runEnrollDevice drives the public enrollment ceremony: init, challenge,
// signed response. sign is called with the server challenge and its result
// rides in the IOSEnrollChallengeResponse.
func runEnrollDevice(ctx context.Context,
	client devicetrustpublicv1pb.DeviceTrustServiceClient,
	init *devicetrustpublicv1pb.EnrollDeviceInit,
	sign func(chal []byte) ([]byte, error),
) (*devicetrustpublicv1pb.EnrollDeviceResponse, error) {
	stream, err := client.EnrollDevice(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := stream.Send(devicetrustpublicv1pb.EnrollDeviceRequest_builder{
		Init: init,
	}.Build()); err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := stream.Recv()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	chal := resp.GetIosChallenge()
	if chal == nil {
		return resp, trace.BadParameter("expected IOSEnrollChallenge, got %v", resp)
	}

	sig, err := sign(chal.GetChallenge())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := stream.Send(devicetrustpublicv1pb.EnrollDeviceRequest_builder{
		IosChallengeResponse: devicetrustpublicv1pb.IOSEnrollChallengeResponse_builder{
			Signature: sig,
		}.Build(),
	}.Build()); err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err = stream.Recv()
	return resp, trace.Wrap(err)
}
