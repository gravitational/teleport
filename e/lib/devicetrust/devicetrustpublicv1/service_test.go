package devicetrustpublicv1_test

import (
	"context"
	"fmt"
	"slices"
	"testing"

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
	"github.com/gravitational/teleport/e/lib/devicetrust/testenv"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
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
	emitter := &eventstest.MockRecorderEmitter{}
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

	// Unlike in the private Device Trust service, the rule check runs against the
	// identity reconstructed from the pairing token instead of the caller's, so
	// each case seeds a user and a pairing for the handler to resolve.
	tests := []struct {
		name string
		want []wantRuleVerb
		// rpc creates user, seeds a pairing owned by it and runs the RPC.
		rpc func(t *testing.T, user string) error
		// assertErr accepts the error the RPC returns once authz passed. Any
		// "blessed" error is OK, we expect the RPCs to fail after
		// authorization.
		assertErr func(t *testing.T, err error)
	}{
		{
			name: "CreatePairedDeviceEnrollToken",
			want: []wantRuleVerb{
				{rule: types.KindMobileDevice, verb: types.VerbCreateEnrollToken},
			},
			rpc: func(t *testing.T, user string) error {
				createUser(t, env, user)
				token := createPairing(t, env, user)
				_, err := env.PublicDevicesClient.CreatePairedDeviceEnrollToken(t.Context(),
					makeRequest(token, makeCollectedData()))
				return err
			},
			assertErr: func(t *testing.T, err error) {
				assert.ErrorIs(t, err, devicetrustpublicv1.ErrAwaitingApproval)
			},
		},
	}

	// Verify that the right permission is checked.
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			checker := &ruleVerifyingChecker{want: test.want}
			authorizer.checker = checker

			test.assertErr(t, test.rpc(t, test.name+"-allowed"))
			assert.NoError(t, checker.verifyMatches())
		})
	}

	// Verify that the audit event is attributed to the correct user if they don't
	// pass authz.
	for _, test := range tests {
		t.Run(test.name+"-denied", func(t *testing.T) {
			user := test.name + "-denied"
			authorizer.checker = &fakeChecker{authorized: false}
			emitter.Reset()

			err := test.rpc(t, user)
			assert.ErrorAs(t, err, new(*trace.AccessDeniedError))
			// Message from fakeChecker, which tells this apart from the other
			// AccessDenied paths.
			assert.ErrorContains(t, err, "access denied")

			last := lastDeviceEvent(t, emitter)
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
			emitter.Reset()

			err := test.rpc(t, test.name+"-feature-disabled")
			assert.ErrorAs(t, err, new(*trace.AccessDeniedError))
			assert.ErrorContains(t, err, "not licensed for device trust")

			assert.Empty(t, emitter.Events())
		})
	}
}

func TestService_CreatePairedDeviceEnrollToken(t *testing.T) {
	t.Parallel()

	emitter := &eventstest.MockRecorderEmitter{}
	env := testenv.NewUsingT(t,
		testenv.WithAuthorizer(&fakeAuthorizer{authorizedUsers: []string{"alice", "bob", "carol"}}),
		testenv.WithEmitter(emitter),
	)
	client := env.PublicDevicesClient

	t.Run("claims the pairing and moves it to awaiting approval state", func(t *testing.T) {
		createUser(t, env, "alice")
		token := createPairing(t, env, "alice")

		cd := makeCollectedData()
		_, err := client.CreatePairedDeviceEnrollToken(t.Context(), makeRequest(token, cd))
		// This is an initial version of the endpoint, so a successful claim
		// currently surfaces as CompareFailed.
		assert.ErrorIs(t, err, devicetrustpublicv1.ErrAwaitingApproval)

		// The pairing has transitioned and recorded the claiming device. The exact
		// fields are verified in tests of lib/services/local.EnrollPairingService.
		got, err := env.EnrollPairing.GetEnrollPairingByToken(t.Context(), token)
		assert.NoError(t, err)
		assert.Equal(t,
			devicepb.EnrollPairingState_ENROLL_PAIRING_STATE_AWAITING_APPROVAL,
			got.GetStatus().GetState())
		assert.Equal(t, cd.GetSerialNumber(), got.GetStatus().GetDevice().GetSerialNumber())

		last := lastDeviceEvent(t, emitter)
		assert.Equal(t, events.DeviceEnrollPairingRequestEvent, last.GetType())
		assert.True(t, last.Status.Success)
		assert.Equal(t, "alice", last.UserMetadata.User)
		assert.Equal(t, events.DeviceEnrollPairingRequestCode, last.Metadata.Code)
	})

	t.Run("allows a retry from the same device", func(t *testing.T) {
		createUser(t, env, "bob")
		token := createPairing(t, env, "bob")

		_, err := client.CreatePairedDeviceEnrollToken(t.Context(), makeRequest(token, makeCollectedData()))
		assert.ErrorIs(t, err, devicetrustpublicv1.ErrAwaitingApproval)

		// The same device polling again finds its own in-progress claim. A poll is
		// not a fresh request, so no additional audit event is emitted.
		emitter.Reset()
		_, err = client.CreatePairedDeviceEnrollToken(t.Context(), makeRequest(token, makeCollectedData()))
		assert.ErrorIs(t, err, devicetrustpublicv1.ErrAwaitingApproval)
		assert.Empty(t, emitter.Events())
	})

	t.Run("rejects a claim from a different device", func(t *testing.T) {
		createUser(t, env, "carol")
		token := createPairing(t, env, "carol")

		_, err := client.CreatePairedDeviceEnrollToken(t.Context(), makeRequest(token, makeCollectedData()))
		assert.ErrorIs(t, err, devicetrustpublicv1.ErrAwaitingApproval)

		hijack := makeCollectedData()
		hijack.SetSerialNumber("alpaca")
		emitter.Reset()
		_, err = client.CreatePairedDeviceEnrollToken(t.Context(), makeRequest(token, hijack))
		assert.ErrorIs(t, err, devicetrustpublicv1.ErrPairingClaimed)

		last := lastDeviceEvent(t, emitter)
		assert.False(t, last.Status.Success)
		assert.Equal(t, "carol", last.UserMetadata.User)
	})

	t.Run("returns NotFound for an unknown token", func(t *testing.T) {
		_, err := client.CreatePairedDeviceEnrollToken(t.Context(), makeRequest("does-not-exist", makeCollectedData()))
		assert.ErrorAs(t, err, new(*trace.NotFoundError))
		assert.ErrorContains(t, err, "read enroll pairing token")

		last := lastDeviceEvent(t, emitter)
		assert.False(t, last.Status.Success)
		assert.Empty(t, last.UserMetadata.User)
		assert.Equal(t, events.DeviceEnrollPairingRequestFailureCode, last.Metadata.Code)
	})

	t.Run("returns NotFound when the pairing user no longer exists", func(t *testing.T) {
		// "sso-user" owns a pairing but has no user record.
		token := createPairing(t, env, "sso-user")

		_, err := client.CreatePairedDeviceEnrollToken(t.Context(), makeRequest(token, makeCollectedData()))
		assert.ErrorAs(t, err, new(*trace.NotFoundError))
		assert.ErrorContains(t, err, "user not found")

		last := lastDeviceEvent(t, emitter)
		assert.False(t, last.Status.Success)
		assert.Equal(t, "sso-user", last.UserMetadata.User)
	})
}

func TestService_CreatePairedDeviceEnrollToken_badParameters(t *testing.T) {
	t.Parallel()

	emitter := &eventstest.MockRecorderEmitter{}
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
			emitter.Reset()

			_, err := env.PublicDevicesClient.CreatePairedDeviceEnrollToken(t.Context(),
				makeRequest(test.token, test.cd))
			assert.ErrorAs(t, err, new(*trace.BadParameterError))
			assert.ErrorContains(t, err, test.wantErr)

			assert.Empty(t, emitter.Events())
		})
	}
}

func TestService_CreatePairedDeviceEnrollToken_rejectsNonProxyCaller(t *testing.T) {
	t.Parallel()

	emitter := &eventstest.MockRecorderEmitter{}
	env := testenv.NewUsingT(t,
		testenv.WithAuthorizer(builtinRoleAuthorizer{role: types.RoleNode}),
		testenv.WithEmitter(emitter),
	)

	_, err := env.PublicDevicesClient.CreatePairedDeviceEnrollToken(t.Context(),
		makeRequest("some-token", makeCollectedData()))
	assert.ErrorAs(t, err, new(*trace.AccessDeniedError))
	assert.ErrorContains(t, err, "can only be executed by a proxy")
	// The Proxy check runs before the user is resolved, so nothing is attributed.
	assert.Empty(t, emitter.Events())
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

	last := lastDeviceEvent(t, emitter)
	assert.False(t, last.Status.Success)
	assert.Equal(t, "alice", last.UserMetadata.User)
}

// fakeAuthorizer stands in for both Authorize calls the handler makes: to
// verify the incoming Proxy identity and to verify the pairing user
// reconstructed from the pairing token.
type fakeAuthorizer struct {
	authorizedUsers []string
	// checker overrides the fakeChecker derived from authorizedUsers, so that a
	// test can assert which rule and verb the handler checks.
	checker services.AccessChecker
}

// Authorize returns either a context with the user (if the user was injected
// into the context by Service.authorizeUser) or the context with the proxy.
func (a *fakeAuthorizer) Authorize(ctx context.Context) (*authz.Context, error) {
	if u, err := authz.UserFromContext(ctx); err == nil {
		if localUser, ok := u.(authz.LocalUser); ok {
			user, err := types.NewUser(localUser.Username)
			if err != nil {
				return nil, err
			}
			checker := a.checker
			if checker == nil {
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

func lastDeviceEvent(t *testing.T, emitter *eventstest.MockRecorderEmitter) *apievents.DeviceEvent2 {
	t.Helper()
	last := emitter.LastEvent()
	require.NotNil(t, last)
	evt, ok := last.(*apievents.DeviceEvent2)
	require.True(t, ok, "expected *apievents.DeviceEvent2, got %T", last)
	return evt
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
