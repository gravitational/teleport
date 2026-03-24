package devicetrustv1_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	gogoproto "github.com/gogo/protobuf/proto"
	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/defaults"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	resourceusagepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/resourceusage/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/e/lib/devicetrust/devicetrustv1"
	"github.com/gravitational/teleport/e/lib/devicetrust/testenv"
	"github.com/gravitational/teleport/entitlements"
	prehogv1alpha "github.com/gravitational/teleport/gen/proto/go/prehog/v1alpha"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/services"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
)

const (
	sampleUserAgentLinux   = "Mozilla/5.0 (X11; Ubuntu; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36"
	sampleUserAgentMacOS   = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"
	sampleUserAgentWindows = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36"
	sampleIP               = "40.89.244.232"
)

func TestService_authz(t *testing.T) {
	t.Parallel()
	authorizer := &fakeAuthorizer{}
	testModules := &modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.DeviceTrust:            {Enabled: true},
				entitlements.MobileDeviceManagement: {Enabled: true},
			},
		},
	}
	env := testenv.NewUsingT(t, testenv.WithAuthorizer(authorizer), testenv.WithModules(testModules))
	devices := env.DevicesClient
	ctx := context.Background()

	tests := []struct {
		name      string
		checker   *ruleVerifyingChecker
		rpc       func() error
		assertErr func(error) bool
	}{
		{
			name: "BulkCreateDevice",
			checker: &ruleVerifyingChecker{
				want: []wantRuleVerb{
					{rule: types.KindDevice, verb: types.VerbCreate},
				},
			},
			rpc: func() error {
				_, err := devices.BulkCreateDevices(ctx, &devicepb.BulkCreateDevicesRequest{})
				return err
			},
			assertErr: trace.IsBadParameter,
		},
		{
			name: "CreateDevice",
			checker: &ruleVerifyingChecker{
				want: []wantRuleVerb{
					{rule: types.KindDevice, verb: types.VerbCreate},
				},
			},
			rpc: func() error {
				_, err := devices.CreateDevice(ctx, &devicepb.CreateDeviceRequest{})
				return err
			},
			assertErr: trace.IsBadParameter,
		},
		{
			name: "CreateDevice checks for create_enroll_token",
			checker: &ruleVerifyingChecker{
				want: []wantRuleVerb{
					{rule: types.KindDevice, verb: types.VerbCreate},
					{rule: types.KindDevice, verb: types.VerbCreateEnrollToken},
				},
			},
			rpc: func() error {
				_, err := devices.CreateDevice(ctx, &devicepb.CreateDeviceRequest{
					CreateEnrollToken: true, // requires create_enroll_token
				})
				return err
			},
			assertErr: trace.IsBadParameter,
		},
		{
			name: "CreateDeviceEnrollToken",
			checker: &ruleVerifyingChecker{
				want: []wantRuleVerb{
					{rule: types.KindDevice, verb: types.VerbCreateEnrollToken},
				},
			},
			rpc: func() error {
				_, err := devices.CreateDeviceEnrollToken(ctx, &devicepb.CreateDeviceEnrollTokenRequest{
					DeviceId: "unknown",
				})
				return err
			},
			assertErr: trace.IsNotFound,
		},
		{
			name: "DeleteDevice",
			checker: &ruleVerifyingChecker{
				want: []wantRuleVerb{
					{rule: types.KindDevice, verb: types.VerbDelete},
				},
			},
			rpc: func() error {
				_, err := devices.DeleteDevice(ctx, &devicepb.DeleteDeviceRequest{
					DeviceId: "unknown",
				})
				return err
			},
			assertErr: trace.IsNotFound,
		},
		{
			name: "EnrollDevice",
			checker: &ruleVerifyingChecker{
				want: []wantRuleVerb{
					{rule: types.KindDevice, verb: types.VerbEnroll},
				},
			},
			rpc: func() error {
				stream, err := devices.EnrollDevice(ctx)
				if err != nil {
					return err
				}
				err = stream.Send(&devicepb.EnrollDeviceRequest{})
				if err != nil && !errors.Is(err, io.EOF) {
					return err
				}
				// Validation errors from Send typically arrive at Recv.
				_, err = stream.Recv()
				return err
			},
			assertErr: trace.IsBadParameter,
		},
		{
			name: "FindDevices",
			checker: &ruleVerifyingChecker{
				want: []wantRuleVerb{
					{rule: types.KindDevice, verb: types.VerbList},
					{rule: types.KindDevice, verb: types.VerbRead},
				},
			},
			rpc: func() error {
				_, err := devices.FindDevices(ctx, &devicepb.FindDevicesRequest{
					IdOrTag: "unknown",
				})
				return err
			},
			assertErr: func(err error) bool { return err == nil },
		},
		{
			name: "GetDevice",
			checker: &ruleVerifyingChecker{
				want: []wantRuleVerb{
					{rule: types.KindDevice, verb: types.VerbRead},
				},
			},
			rpc: func() error {
				_, err := devices.GetDevice(ctx, &devicepb.GetDeviceRequest{
					DeviceId: "unknown",
				})
				return err
			},
			assertErr: trace.IsNotFound,
		},
		{
			name: "ListDevices",
			checker: &ruleVerifyingChecker{
				want: []wantRuleVerb{
					{rule: types.KindDevice, verb: types.VerbList},
					{rule: types.KindDevice, verb: types.VerbRead},
				},
			},
			rpc: func() error {
				_, err := devices.ListDevices(ctx, &devicepb.ListDevicesRequest{})
				return err
			},
			assertErr: func(err error) bool { return err == nil },
		},
		{
			name: "UpdateDevice",
			checker: &ruleVerifyingChecker{
				want: []wantRuleVerb{
					{rule: types.KindDevice, verb: types.VerbUpdate},
				},
			},
			rpc: func() error {
				_, err := devices.UpdateDevice(ctx, &devicepb.UpdateDeviceRequest{})
				return err
			},
			assertErr: trace.IsBadParameter,
		},
		{
			name: "UpsertDevice",
			checker: &ruleVerifyingChecker{
				want: []wantRuleVerb{
					{rule: types.KindDevice, verb: types.VerbCreate},
					{rule: types.KindDevice, verb: types.VerbUpdate},
				},
			},
			rpc: func() error {
				_, err := devices.UpsertDevice(ctx, &devicepb.UpsertDeviceRequest{})
				return err
			},
			assertErr: trace.IsBadParameter,
		},
		{
			name: "SyncInventory",
			checker: &ruleVerifyingChecker{
				want: []wantRuleVerb{
					{rule: types.KindDevice, verb: types.VerbCreate},
					{rule: types.KindDevice, verb: types.VerbUpdate},
					{rule: types.KindDevice, verb: types.VerbList},
					{rule: types.KindDevice, verb: types.VerbDelete},
				},
			},
			rpc: func() error {
				stream, err := devices.SyncInventory(ctx)
				if err != nil {
					return err
				}
				if err := stream.Send(&devicepb.SyncInventoryRequest{
					Payload: nil, // missing start payload
				}); err != nil && !errors.Is(err, io.EOF) {
					return err
				}
				// Validation errors from Send typically arrive at Recv.
				_, err = stream.Recv()
				return err
			},
			assertErr: trace.IsBadParameter,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			authorizer.Checker = test.checker
			authorizer.authorizeCount = 0

			// Any "blessed" error is OK, we expect the RPCs to fail after
			// authorization. It's simpler to test this way.
			if err := test.rpc(); !test.assertErr(err) {
				t.Fatalf("RPC assertErr failed, err=%v", err)
			}

			if got, want := authorizer.authorizeCount, 1; got != want {
				t.Errorf("Authorize count mismatch: got=%v, want=%v", got, want)
			}
			if err := test.checker.verifyMatches(); err != nil {
				t.Errorf("Authorize: %v", err)
			}
		})
	}

	testModules.TestFeatures.Entitlements[entitlements.DeviceTrust] = modules.EntitlementInfo{Enabled: false}

	// Test system behavior when the feature is disabled.
	// The check is bundled with user authz, so it's easy to test it here.
	authorizer.Checker = &testenv.NoopChecker{}
	for _, test := range tests {
		t.Run(test.name+"-feature disabled", func(t *testing.T) {
			err := test.rpc()
			if !trace.IsAccessDenied(err) {
				t.Fatalf("RPC returned err=%v, want AccessDenied/feature disabled error", err)
			}
			assert.ErrorContains(t, err, "not licensed for device trust")
		})
	}
}

type fakeAuthorizer struct {
	authorizeCount int
	Checker        services.AccessChecker
}

func (a *fakeAuthorizer) Authorize(ctx context.Context) (*authz.Context, error) {
	a.authorizeCount++

	user, err := types.NewUser("llama")
	if err != nil {
		return nil, err
	}

	return &authz.Context{
		User:                 user,
		Checker:              a.Checker,
		AdminActionAuthState: authz.AdminActionAuthNotRequired,
	}, nil
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

type alternatingLimiter struct {
	mu   sync.Mutex
	keys map[string]struct{}
}

func (l *alternatingLimiter) RegisterRequestWithCustomRate(token string, customRate *limiter.RateSet) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.keys == nil {
		l.keys = make(map[string]struct{})
	}

	if _, ok := l.keys[token]; ok {
		delete(l.keys, token)
		return trace.LimitExceeded("limit exceeded")
	} else {
		l.keys[token] = struct{}{}
		return nil
	}
}

func (l *alternatingLimiter) reset() {
	l.mu.Lock()
	l.keys = nil
	l.mu.Unlock()
}

func TestService_rateLimiting(t *testing.T) {
	limiter := &alternatingLimiter{}
	env := testenv.NewUsingT(t,
		testenv.WithAuthPreferenceSpec(types.AuthPreferenceSpecV2{
			DeviceTrust: &types.DeviceTrust{
				AutoEnroll: true,
			},
		}),
		// Note: all testenv calls are made, by default, using a fake "llama" user.
		testenv.WithLimiter(limiter),
	)

	devices := env.DevicesClient
	ctx := context.Background()

	// Create a couple of test devices.
	createdDev, err := devices.CreateDevice(ctx, &devicepb.CreateDeviceRequest{
		Device: &devicepb.Device{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: "llama",
		},
	})
	if err != nil {
		t.Fatalf("CreateDevice failed: %v", err)
	}
	enrolledDev, enrolledKey, err := createAndEnroll(ctx, devices, &devicepb.Device{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "alpaca",
	})
	if err != nil {
		t.Fatalf("createAndEnroll failed: %v", err)
	}

	tests := []struct {
		name string
		rpc  func() error
	}{
		{
			name: "CreateDeviceEnrollToken auto-enroll",
			rpc: func() error {
				_, err := devices.CreateDeviceEnrollToken(ctx, &devicepb.CreateDeviceEnrollTokenRequest{
					DeviceData: defaultCollectData(createdDev),
				})
				return err
			},
		},
		{
			name: "AuthenticateDevice",
			rpc: func() error {
				_, err := authenticateSimulator(ctx, devices, enrolledKey.simulator(), enrolledDev, nil /* initCerts */)
				return err
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			limiter.reset()

			// First call succeeds.
			if err := test.rpc(); err != nil {
				t.Fatalf("First rpc call returned err=%v, want nil", err)
			}

			// Second call is rate limited.
			if err := test.rpc(); !trace.IsLimitExceeded(err) {
				t.Fatalf("Second rpc call returned err=%v, want LimitExceeded", err)
			}
		})
	}
}

func TestService_CreateDevice(t *testing.T) {
	emitter := &eventstest.MockRecorderEmitter{}
	env := testenv.NewUsingT(t, testenv.WithEmitter(emitter))
	devices := env.DevicesClient

	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}
	pubKeyDER, err := x509.MarshalPKIXPublicKey(privKey.Public())
	if err != nil {
		t.Fatalf("MarshalPKIXPublicKey failed: %v", err)
	}

	const resourceDeviceTag = "A00AA0AAAA0A"

	ctx := context.Background()
	tests := []struct {
		name string
		req  *devicepb.CreateDeviceRequest
	}{
		{
			name: "ok",
			req: &devicepb.CreateDeviceRequest{
				Device: &devicepb.Device{
					OsType:   devicepb.OSType_OS_TYPE_MACOS,
					AssetTag: "llama",
				},
			},
		},
		{
			name: "device and enroll token",
			req: &devicepb.CreateDeviceRequest{
				Device: &devicepb.Device{
					OsType:   devicepb.OSType_OS_TYPE_MACOS,
					AssetTag: "alpaca",
				},
				CreateEnrollToken: true,
			},
		},
		{
			name: "resource-like write",
			req: &devicepb.CreateDeviceRequest{
				Device: &devicepb.Device{
					ApiVersion: "v1",
					Id:         "a6f76866-a9eb-4a23-9bb1-7980347a1bee",
					OsType:     devicepb.OSType_OS_TYPE_MACOS,
					AssetTag:   resourceDeviceTag,
					CreateTime: timestamppb.New(time.Date(2023, 2, 15, 15, 28, 21, 0, time.UTC)),
					UpdateTime: timestamppb.New(time.Date(2023, 2, 23, 22, 9, 36, 0, time.UTC)),
					EnrollToken: &devicepb.DeviceEnrollToken{
						Token: "i-am-ignored",
					},
					EnrollStatus: devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_ENROLLED,
					Credential: &devicepb.DeviceCredential{
						Id:           "ae2d978c-fee8-419d-a2e6-a5dd0a00c4b8",
						PublicKeyDer: pubKeyDER,
					},
					// CollectedData is ignored by CreateDevice - it's only written in
					// paths where a device challenge is cleared.
					CollectedData: []*devicepb.DeviceCollectedData{
						{
							CollectTime:  timestamppb.New(time.Date(2023, 2, 15, 15, 28, 35, 402554, time.UTC)),
							RecordTime:   timestamppb.New(time.Date(2023, 2, 15, 15, 28, 35, 438274, time.UTC)),
							OsType:       devicepb.OSType_OS_TYPE_MACOS,
							SerialNumber: resourceDeviceTag,
						},
						{
							CollectTime:  timestamppb.New(time.Date(2023, 2, 23, 22, 9, 12, 9985, time.UTC)),
							RecordTime:   timestamppb.New(time.Date(2023, 2, 23, 22, 9, 12, 136163, time.UTC)),
							OsType:       devicepb.OSType_OS_TYPE_MACOS,
							SerialNumber: resourceDeviceTag,
						},
						{
							CollectTime:  timestamppb.New(time.Date(2023, 2, 23, 22, 9, 36, 904036, time.UTC)),
							RecordTime:   timestamppb.New(time.Date(2023, 2, 23, 22, 9, 36, 984874, time.UTC)),
							OsType:       devicepb.OSType_OS_TYPE_MACOS,
							SerialNumber: resourceDeviceTag,
						},
					},
				},
				CreateAsResource: true,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			emitter.Reset()

			got, err := devices.CreateDevice(ctx, test.req)
			if err != nil {
				t.Fatalf("CreateDevice failed: %v", err)
			}

			// Assert that expected fields are present.
			if got.Id == "" {
				t.Fatal("CreateDevice returned device without ID")
			}
			if got.ApiVersion == "" {
				t.Error("CreateDevice returned device without ApiVersion")
			}
			if got.CreateTime == nil {
				t.Error("CreateDevice returned device without CreateTime")
			}
			if got.UpdateTime == nil {
				t.Error("CreateDevice returned device without CreateTime")
			}

			// Verify CreateDevice response.
			want := proto.Clone(test.req.Device).(*devicepb.Device)
			want.ApiVersion = got.ApiVersion
			want.Id = got.Id
			want.CreateTime = got.CreateTime
			want.UpdateTime = got.UpdateTime
			want.EnrollToken = got.EnrollToken
			want.EnrollStatus = got.EnrollStatus
			want.CollectedData = nil // ignored by CreateDevice
			if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
				t.Fatalf("CreateDevice mismatch (-want +got):\n%s", diff)
			}

			// Verify enrollment token.
			switch {
			case test.req.CreateEnrollToken:
				if got.EnrollToken.GetToken() == "" {
					t.Error("CreateDevice returned nil or empty enroll token, expected a non-empty token present")
				}
			case got.EnrollToken != nil:
				t.Errorf("CreateDevice returned an unexpected enroll token: %#v", got.EnrollToken)
			}
			// No other endpoints return the token, so blank it to make subsequent
			// comparisons easier.
			got.EnrollToken = nil

			// Verify that device is stored.
			stored, err := devices.GetDevice(ctx, &devicepb.GetDeviceRequest{
				DeviceId: got.Id,
			})
			if err != nil {
				t.Fatalf("GetDevice failed: %v", err)
			}
			if diff := cmp.Diff(got, stored, protocmp.Transform()); diff != "" {
				t.Errorf("GetDevice mismatch (-want +got):\n%s", diff)
			}

			// Verify audit log.
			wantEvents := []wantEvent{
				{
					Type: events.DeviceCreateEvent,
					Code: events.DeviceCreateCode,
				},
			}
			if test.req.CreateEnrollToken {
				wantEvents = append(wantEvents, wantEvent{
					Type: events.DeviceEnrollTokenCreateEvent,
					Code: events.DeviceEnrollTokenCreateCode,
				})
			}
			assertEvents(t, emitter.Events(), wantEvents)
		})
	}
}

func TestService_CreateDevice_asResource(t *testing.T) {
	env := testenv.NewUsingT(t)
	defer env.Close()

	devices := env.DevicesClient
	ctx := context.Background()

	// Create a few devices, read them in full view, delete and then re-create as
	// a resource. This simulates an interaction similar to
	// `tctl rm device/X | tctl create`.
	// The final state should match the initial state, exactly.

	// Create a couple of registered-only devices.
	var allDevices []*devicepb.Device
	for _, tag := range []string{"dev1", "dev2"} {
		dev, err := devices.CreateDevice(ctx, &devicepb.CreateDeviceRequest{
			Device: &devicepb.Device{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: tag,
			},
		})
		if err != nil {
			t.Fatalf("CreateDevice(%q) failed: %v", tag, err)
		}
		allDevices = append(allDevices, dev)
	}

	// Create a couple of enrolled devices.
	llamaDev, llamaKey, err := createAndEnroll(ctx, devices, &devicepb.Device{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "llama",
	})
	if err != nil {
		t.Fatalf("createAndEnroll failed: %v", err)
	}
	allDevices = append(allDevices, llamaDev)

	alpacaDev, alpacaKey, err := createAndEnroll(ctx, devices, &devicepb.Device{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "alpaca",
	})
	if err != nil {
		t.Fatalf("createAndEnroll failed: %v", err)
	}
	allDevices = append(allDevices, alpacaDev)

	authenticate := func(dev *devicepb.Device, key *fakeEnclaveKey) error {
		_, err := authenticateSimulator(ctx, devices, key.simulator(), dev, nil /* initCerts */)
		return err
	}

	// Authenticate a few times to generate additional collected data.
	if err := authenticate(llamaDev, llamaKey); err != nil {
		t.Fatalf("authenticateDevice failed: %v", err)
	}
	if err := authenticate(llamaDev, llamaKey); err != nil {
		t.Fatalf("authenticateDevice failed: %v", err)
	}
	if err := authenticate(alpacaDev, alpacaKey); err != nil {
		t.Fatalf("authenticateDevice failed: %v", err)
	}

	// Read fresh, complete copies of all devices.
	for i, dev := range allDevices {
		stored, err := devices.GetDevice(ctx, &devicepb.GetDeviceRequest{
			DeviceId: dev.Id,
		})
		if err != nil {
			t.Fatalf("GetDevices failed: %v", err)
		}
		allDevices[i] = stored
	}

	// Sanity checks: make sure state is as we expect.
	switch {
	case allDevices[0].EnrollStatus != devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED: // dev1
		t.Fatalf("dev1 has unexpected enrollment status: %v", allDevices[0].EnrollStatus)
	case allDevices[1].EnrollStatus != devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED: // dev2
		t.Fatalf("dev2 has unexpected enrollment status: %v", allDevices[1].EnrollStatus)
	case allDevices[2].EnrollStatus != devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_ENROLLED: // llama
		t.Fatalf("llama has unexpected enrollment status: %v", allDevices[2].EnrollStatus)
	case allDevices[2].Credential == nil:
		t.Fatal("llama has nil credential")
	case len(allDevices[2].CollectedData) < 3: // 1 enroll + 2 authn
		t.Fatalf("llama has unexpected number of collected data: %v", len(allDevices[2].CollectedData))
	case allDevices[3].EnrollStatus != devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_ENROLLED: // alpaca
		t.Fatalf("alpaca has unexpected enrollment status: %v", allDevices[3].EnrollStatus)
	case allDevices[3].Credential == nil:
		t.Fatal("alpaca has nil credential")
	case len(allDevices[3].CollectedData) < 2: // 1 enroll + 1 authn
		t.Fatalf("alpaca has unexpected number of collected data: %v", len(allDevices[3].CollectedData))
	}

	assertDevices := func(t *testing.T, allDevices []*devicepb.Device) {
		t.Helper()

		listReq := &devicepb.ListDevicesRequest{
			View: devicepb.DeviceView_DEVICE_VIEW_RESOURCE,
		}

		var got []*devicepb.Device
		for {
			listResp, err := devices.ListDevices(ctx, listReq)
			if err != nil {
				t.Fatalf("ListDevices failed: %v", err)
			}
			got = append(got, listResp.Devices...)
			if listResp.NextPageToken == "" {
				break
			}
			listReq.PageToken = listResp.NextPageToken
		}

		// Preserve the order of `allDevices`, it's best if we avoid writing
		// devices in a specific order for the test.
		want := make([]*devicepb.Device, len(allDevices))
		copy(want, allDevices)

		slices.SortFunc(want, func(a, b *devicepb.Device) int {
			return strings.Compare(a.AssetTag, b.AssetTag)
		})
		slices.SortFunc(got, func(a, b *devicepb.Device) int {
			return strings.Compare(a.AssetTag, b.AssetTag)
		})
		if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
			t.Fatalf("ListDevices mismatch (-want +got):\n%s", diff)
		}
	}
	// Sanity check: storage matches allDevices.
	assertDevices(t, allDevices)

	deleteAll := func(t *testing.T) {
		t.Helper()

		listReq := &devicepb.ListDevicesRequest{}
		var deviceIDs []string
		for {
			listResp, err := devices.ListDevices(ctx, listReq)
			if err != nil {
				t.Fatalf("ListDevices failed: %v", err)
			}

			for _, dev := range listResp.Devices {
				deviceIDs = append(deviceIDs, dev.Id)
			}

			if listResp.NextPageToken == "" {
				break
			}
			listReq.PageToken = listResp.NextPageToken
		}

		for _, id := range deviceIDs {
			_, err := devices.DeleteDevice(ctx, &devicepb.DeleteDeviceRequest{
				DeviceId: id,
			})
			if err != nil {
				t.Fatalf("DeleteDevice(%q) failed: %v", id, err)
			}
		}

		// Sanity check deletion.
		switch listResp, err := devices.ListDevices(ctx, &devicepb.ListDevicesRequest{}); {
		case err != nil:
			t.Fatalf("ListDevices failed: %v", err)
		case len(listResp.Devices) > 0:
			t.Fatalf("ListDevices returned devices, wanted none: %v", listResp.Devices)
		}
	}

	t.Run("CreateDevice", func(t *testing.T) {
		deleteAll(t)

		wantAll := make([]*devicepb.Device, 0, len(allDevices))
		for _, dev := range allDevices {
			created, err := devices.CreateDevice(ctx, &devicepb.CreateDeviceRequest{
				Device:           dev,
				CreateAsResource: true,
			})
			if err != nil {
				t.Fatalf("CreateDevice(%q, createAsResource=true) failed: %v", dev.AssetTag, err)
			}

			want := proto.Clone(dev).(*devicepb.Device)
			want.CollectedData = nil // ignored by CreateDevice
			wantAll = append(wantAll, want)

			// Assert CreateDevice's response.
			if diff := cmp.Diff(want, created, protocmp.Transform()); diff != "" {
				t.Errorf("CreateDevice mismatch (-want +got):\n%s", diff)
			}
		}

		// Assert storage.
		assertDevices(t, wantAll)
	})

	t.Run("BulkCreateDevices", func(t *testing.T) {
		deleteAll(t)

		resp, err := devices.BulkCreateDevices(ctx, &devicepb.BulkCreateDevicesRequest{
			Devices:          allDevices,
			CreateAsResource: true,
		})
		if err != nil {
			t.Fatalf("BulkCreateDevices failed: %v", err)
		}

		// Assert BulkCreateDevices' response.
		want := make([]*devicepb.DeviceOrStatus, len(allDevices))
		for i, dev := range allDevices {
			want[i] = &devicepb.DeviceOrStatus{
				Id: dev.Id,
			}
		}
		if diff := cmp.Diff(want, resp.Devices, protocmp.Transform()); diff != "" {
			t.Errorf("BulkCreateDevices mismatch (-want +got):\n%s", diff)
		}

		wantAll := make([]*devicepb.Device, len(allDevices))
		for i, dev := range allDevices {
			wantAll[i] = proto.Clone(dev).(*devicepb.Device)
			wantAll[i].CollectedData = nil // ignored by BulkCreateDevices
		}

		// Assert storage.
		assertDevices(t, wantAll)
	})
}

func TestService_UpdateDevice(t *testing.T) {
	emitter := &eventstest.MockRecorderEmitter{}
	env := testenv.NewUsingT(t, testenv.WithEmitter(emitter))

	devices := env.DevicesClient
	ctx := context.Background()

	enrolled, _, err := createAndEnroll(ctx, devices, &devicepb.Device{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "llama",
	})
	if err != nil {
		t.Fatalf("createAndEnroll failed: %v", err)
	}

	profileBase, err := devices.CreateDevice(ctx, &devicepb.CreateDeviceRequest{
		Device: &devicepb.Device{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: "profile1",
		},
	})
	if err != nil {
		t.Fatalf("CreateDevice failed: %v", err)
	}
	profileSource := &devicepb.DeviceSource{
		Name:   "myscript",
		Origin: devicepb.DeviceOrigin_DEVICE_ORIGIN_API,
	}
	profileProfile := &devicepb.DeviceProfile{
		ModelIdentifier: "MacBookPro9,2",
		OsVersion:       "13.2.1",
		OsUsernames:     []string{"admin", "llama"},
	}

	tests := []struct {
		name      string
		req       *devicepb.UpdateDeviceRequest
		assertDev func(t *testing.T, updated *devicepb.Device)
	}{
		{
			name: "unenroll",
			req: &devicepb.UpdateDeviceRequest{
				Device: &devicepb.Device{
					Id:           enrolled.Id,
					EnrollStatus: devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED,
				},
				UpdateMask: &fieldmaskpb.FieldMask{
					Paths: []string{"enroll_status"},
				},
			},
			assertDev: func(t *testing.T, updated *devicepb.Device) {
				want := proto.Clone(enrolled).(*devicepb.Device)
				want.UpdateTime = updated.UpdateTime
				want.EnrollStatus = devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED
				want.Credential = nil // Automatically cleared.
				want.Owner = ""       // Automatically cleared.
				if diff := cmp.Diff(want, updated, protocmp.Transform()); diff != "" {
					t.Errorf("UpdateDevice mismatch (-want +got)\n%s", diff)
				}
			},
		},
		{
			name: "source and profile",
			req: &devicepb.UpdateDeviceRequest{
				Device: &devicepb.Device{
					Id:      profileBase.Id,
					Source:  profileSource,
					Profile: profileProfile,
				},
				UpdateMask: &fieldmaskpb.FieldMask{
					Paths: []string{"source", "profile"},
				},
			},
			assertDev: func(t *testing.T, updated *devicepb.Device) {
				// want the base device with source and profile set, plus new
				// timestamps.
				want := proto.Clone(profileBase).(*devicepb.Device)
				want.UpdateTime = updated.UpdateTime
				want.Source = profileSource
				want.Profile = profileProfile

				// Sanity check, then copy the UpdateTime to `want`.
				if updated.Profile == nil || updated.Profile.UpdateTime == nil {
					t.Errorf("UpdateDevice returned nil Profile.UpdateTime: %v", updated)
				} else {
					want.Profile.UpdateTime = updated.Profile.UpdateTime
				}

				if diff := cmp.Diff(want, updated, protocmp.Transform()); diff != "" {
					t.Errorf("UpdateDevice mismatch (-want +got)\n%s", diff)
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			emitter.Reset()

			updated, err := devices.UpdateDevice(ctx, test.req)
			if err != nil {
				t.Fatalf("UpdateDevice failed: %v", err)
			}
			test.assertDev(t, updated)

			// Verify stored device.
			stored, err := devices.GetDevice(ctx, &devicepb.GetDeviceRequest{
				DeviceId: updated.Id,
			})
			if err != nil {
				t.Fatalf("GetDevice failed: %v", err)
			}
			stored.CollectedData = nil // not returned by UpdateDevice
			if diff := cmp.Diff(updated, stored, protocmp.Transform()); diff != "" {
				t.Errorf("GetDevice mismatch (-want +got)\n%s", diff)
			}

			// Verify audit log.
			assertEvents(t, emitter.Events(), []wantEvent{
				{
					Type: events.DeviceUpdateEvent,
					Code: events.DeviceUpdateCode,
				},
			})
		})
	}
}

func TestService_UpdateDevice_errors(t *testing.T) {
	env := testenv.NewUsingT(t)

	devices := env.DevicesClient
	ctx := context.Background()

	enrolled, _, err := createAndEnroll(ctx, devices, &devicepb.Device{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "llama",
	})
	if err != nil {
		t.Fatalf("createAndEnroll failed: %v", err)
	}

	validUpdateDev := &devicepb.Device{
		Id:           enrolled.Id,
		EnrollStatus: devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED,
	}

	validUpdateMask := &fieldmaskpb.FieldMask{
		Paths: []string{"enroll_status"},
	}

	tests := []struct {
		name      string
		req       *devicepb.UpdateDeviceRequest
		assertErr func(err error) bool
		wantErr   string
	}{
		{
			name: "UpdateMask nil",
			req: &devicepb.UpdateDeviceRequest{
				Device: validUpdateDev,
			},
			assertErr: trace.IsBadParameter,
			wantErr:   "update mask required",
		},
		{
			name: "UpdateMask empty",
			req: &devicepb.UpdateDeviceRequest{
				Device:     validUpdateDev,
				UpdateMask: &fieldmaskpb.FieldMask{},
			},
			assertErr: trace.IsBadParameter,
			wantErr:   "update mask path",
		},
		{
			name: "UpdateMask invalid",
			req: &devicepb.UpdateDeviceRequest{
				Device: validUpdateDev,
				UpdateMask: &fieldmaskpb.FieldMask{
					Paths: []string{
						"enroll_status", // OK
						"id",            // NOK
					},
				},
			},
			assertErr: trace.IsBadParameter,
			wantErr:   "unsupported update mask",
		},
		{
			name: "Device nil",
			req: &devicepb.UpdateDeviceRequest{
				UpdateMask: validUpdateMask,
			},
			assertErr: trace.IsBadParameter,
			wantErr:   "device required",
		},
		{
			name: "Device.Id empty",
			req: &devicepb.UpdateDeviceRequest{
				Device: &devicepb.Device{
					Id:           "",
					EnrollStatus: devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED,
				},
				UpdateMask: validUpdateMask,
			},
			assertErr: trace.IsBadParameter,
			wantErr:   "ID required",
		},
		{
			name: "unknown device",
			req: &devicepb.UpdateDeviceRequest{
				Device: &devicepb.Device{
					Id:           "unknown",
					EnrollStatus: devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED,
				},
				UpdateMask: validUpdateMask,
			},
			assertErr: trace.IsNotFound,
			wantErr:   "not found",
		},
		{
			name: "invalid update",
			req: &devicepb.UpdateDeviceRequest{
				Device: &devicepb.Device{
					Id:           enrolled.Id,
					EnrollStatus: devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_UNSPECIFIED, // invalid
				},
				UpdateMask: validUpdateMask,
			},
			assertErr: trace.IsBadParameter,
			wantErr:   "enroll_status",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := devices.UpdateDevice(ctx, test.req)
			if err == nil {
				t.Fatal("UpdateDevice returned err=nil, want non-nil")
			}

			if !test.assertErr(err) {
				t.Errorf("UpdateDevice: assertErr failed, err=%v (%T)", err, err)
			}

			assert.ErrorContains(t, err, test.wantErr, "UpdateDevice error mismatch")
		})
	}
}

func TestService_UpsertDevice(t *testing.T) {
	emitter := &eventstest.MockRecorderEmitter{}
	env := testenv.NewUsingT(t, testenv.WithEmitter(emitter))

	devices := env.DevicesClient
	ctx := context.Background()

	enrolled, _, err := createAndEnroll(ctx, devices, &devicepb.Device{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "llama",
	})
	if err != nil {
		t.Fatalf("createAndEnroll failed: %v", err)
	}

	// enrolledUpdate is to used to unenroll `enrolled`.
	enrolledUpdate := proto.Clone(enrolled).(*devicepb.Device)
	enrolledUpdate.EnrollStatus = devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED

	terraformDev, _, err := createAndEnroll(ctx, devices, &devicepb.Device{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "terraform1",
	})
	if err != nil {
		t.Fatalf("terraformDev createAndEnroll failed: %v", err)
	}

	tests := []struct {
		name       string
		req        *devicepb.UpsertDeviceRequest
		assertDev  func(t *testing.T, base, upserted *devicepb.Device)
		wantEvents []wantEvent
	}{
		{
			name: "create",
			req: &devicepb.UpsertDeviceRequest{
				Device: &devicepb.Device{
					OsType:   devicepb.OSType_OS_TYPE_MACOS,
					AssetTag: "llama1",
				},
			},
			assertDev: func(t *testing.T, base, upserted *devicepb.Device) {
				// Only the request parameters have to match.
				want := proto.Clone(upserted).(*devicepb.Device)
				want.OsType = base.OsType
				want.AssetTag = base.AssetTag
				if diff := cmp.Diff(want, upserted, protocmp.Transform()); diff != "" {
					t.Errorf("UpsertDevice mismatch (-want +got)\n%s", diff)
				}
			},
			wantEvents: []wantEvent{
				{Type: events.DeviceCreateEvent, Code: events.DeviceCreateCode},
			},
		},
		{
			name: "create as resource",
			req: &devicepb.UpsertDeviceRequest{
				Device: &devicepb.Device{
					ApiVersion:   "v1",
					Id:           "71c51595-dfbc-4cae-b511-bce96a632a7e",
					OsType:       devicepb.OSType_OS_TYPE_MACOS,
					AssetTag:     "llama2",
					CreateTime:   timestamppb.New(time.UnixMilli(1680038979000)), // Tue, 28 Mar 2023 21:29:25 GMT
					UpdateTime:   timestamppb.New(time.UnixMilli(1680038979000)),
					EnrollStatus: devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED,
				},
				CreateAsResource: true,
			},
			assertDev: func(t *testing.T, base, upserted *devicepb.Device) {
				// Devices are exactly the same in this case.
				if diff := cmp.Diff(base, upserted, protocmp.Transform()); diff != "" {
					t.Errorf("UpsertDevice mismatch (-want +got)\n%s", diff)
				}
			},
			wantEvents: []wantEvent{
				{Type: events.DeviceCreateEvent, Code: events.DeviceCreateCode},
			},
		},
		{
			name: "update",
			req: &devicepb.UpsertDeviceRequest{
				Device: enrolledUpdate,
			},
			assertDev: func(t *testing.T, base *devicepb.Device, upserted *devicepb.Device) {
				want := base
				want.UpdateTime = upserted.UpdateTime // updated
				want.Credential = nil                 // removed on unenroll
				want.Owner = ""                       // removed on unenroll
				if diff := cmp.Diff(want, upserted, protocmp.Transform()); diff != "" {
					t.Errorf("UpsertDevice mismatch (-want +got)\n%s", diff)
				}
			},
			wantEvents: []wantEvent{
				{Type: events.DeviceUpdateEvent, Code: events.DeviceUpdateCode},
			},
		},
		{
			name: "upsert with altered CreateTime, UpdateTime and Credential(device from terraform)",
			req: &devicepb.UpsertDeviceRequest{
				Device: deviceFromTerraform(t, proto.Clone(terraformDev).(*devicepb.Device)),
			},
			assertDev: func(t *testing.T, base, upserted *devicepb.Device) {
				// verify modified credential did not end up in the storage
				assert.NotEqual(t, base.Credential, upserted.Credential)

				// revert modified timestamps and credential to original value
				base.CreateTime = terraformDev.CreateTime
				base.UpdateTime = terraformDev.UpdateTime
				base.Credential = terraformDev.Credential

				if diff := cmp.Diff(base, upserted, protocmp.Transform()); diff != "" {
					t.Errorf("UpsertDevice mismatch (-want +got)\n%s", diff)
				}
			},
			wantEvents: []wantEvent{
				{Type: events.DeviceUpdateEvent, Code: events.DeviceUpdateCode},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			emitter.Reset()

			upserted, err := devices.UpsertDevice(ctx, test.req)
			if err != nil {
				t.Fatalf("UpsertDevice failed: %v", err)
			}
			test.assertDev(t, test.req.Device, upserted)

			// Verify stored device.
			stored, err := devices.GetDevice(ctx, &devicepb.GetDeviceRequest{
				DeviceId: upserted.Id,
			})
			if err != nil {
				t.Fatalf("GetDevice failed: %v", err)
			}
			stored.CollectedData = nil // not returned by UpsertDevice
			if diff := cmp.Diff(upserted, stored, protocmp.Transform()); diff != "" {
				t.Errorf("GetDevice mismatch (-want +got)\n%s", diff)
			}

			// Verify audit log.
			assertEvents(t, emitter.Events(), test.wantEvents)
		})
	}
}

func deviceFromTerraform(t *testing.T, dev *devicepb.Device) *devicepb.Device {
	// Altered CreateTime and UpdateTime should be ignored.
	// Terraform represents dates using RFC3999 and, as a result, loses sub-second
	// precision on the timestamps. This simulates that, but in a simpler way.
	dev.CreateTime = timestamppb.New(dev.CreateTime.AsTime().Add(1 * time.Second))
	dev.UpdateTime = timestamppb.New(dev.UpdateTime.AsTime().Add(1 * time.Second))

	// Altered Credential should be ignored. Credential is not managed in terraform.
	dev.Credential = &devicepb.DeviceCredential{
		Id:           "deviceFromTerraform",
		PublicKeyDer: []byte("deviceFromTerraform test public key"),
	}

	return dev
}

func TestService_UpsertDevice_errors(t *testing.T) {
	env := testenv.NewUsingT(t)

	devices := env.DevicesClient
	ctx := context.Background()

	created, err := devices.CreateDevice(ctx, &devicepb.CreateDeviceRequest{
		Device: &devicepb.Device{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: "llama",
		},
	})
	if err != nil {
		t.Fatalf("CreateDevice failed: %v", err)
	}

	invalidUpdate := proto.Clone(created).(*devicepb.Device)
	invalidUpdate.AssetTag = "invalid" // cannot change

	tests := []struct {
		name      string
		req       *devicepb.UpsertDeviceRequest
		assertErr func(err error) bool
		wantErr   string
	}{
		{
			name:      "device nil",
			req:       &devicepb.UpsertDeviceRequest{},
			assertErr: trace.IsBadParameter,
			wantErr:   "device required",
		},
		{
			name: "create invalid device",
			req: &devicepb.UpsertDeviceRequest{
				Device: &devicepb.Device{
					OsType:   devicepb.OSType_OS_TYPE_MACOS,
					AssetTag: "", // required
				},
			},
			assertErr: trace.IsBadParameter,
			wantErr:   "asset_tag required",
		},
		{
			name: "update invalid device",
			req: &devicepb.UpsertDeviceRequest{
				Device: invalidUpdate,
			},
			assertErr: trace.IsBadParameter,
			wantErr:   "asset_tag is readonly",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := devices.UpsertDevice(ctx, test.req)
			if err == nil {
				t.Fatal("UpsertDevice returned err=nil, want non-nil")
			}

			if !test.assertErr(err) {
				t.Errorf("UpsertDevice: assertErr failed, err=%v (%T)", err, err)
			}

			assert.ErrorContains(t, err, test.wantErr, "UpsertDevice error mismatch")
		})
	}
}

func TestService_DeleteDevice(t *testing.T) {
	emitter := &eventstest.MockRecorderEmitter{}
	env := testenv.NewUsingT(t, testenv.WithEmitter(emitter))

	devices := env.DevicesClient
	ctx := context.Background()

	// Create a device so we can delete it below.
	dev, err := devices.CreateDevice(ctx, &devicepb.CreateDeviceRequest{
		Device: &devicepb.Device{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: "llama",
		},
	})
	if err != nil {
		t.Fatalf("CreateDevice failed: %v", err)
	}

	tests := []struct {
		name      string
		deviceID  string
		assertErr func(error) bool
	}{
		{
			name:      "device ID required",
			deviceID:  "",
			assertErr: trace.IsBadParameter,
		},
		{
			name:      "unknown device fails",
			deviceID:  "unknown",
			assertErr: trace.IsNotFound,
		},
		{
			name:      "ok",
			deviceID:  dev.Id,
			assertErr: func(err error) bool { return err == nil },
		},
		{
			name:      "double deletion fails",
			deviceID:  dev.Id,
			assertErr: trace.IsNotFound,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			emitter.Reset()

			_, err := devices.DeleteDevice(ctx, &devicepb.DeleteDeviceRequest{
				DeviceId: test.deviceID,
			})
			if !test.assertErr(err) {
				t.Fatalf("DeleteDevice: assertErr failed, err=%v", err)
			}
			if err != nil {
				return
			}

			// Verify deletion via read.
			if _, err := devices.GetDevice(ctx, &devicepb.GetDeviceRequest{
				DeviceId: test.deviceID,
			}); !trace.IsNotFound(err) {
				t.Errorf("GetDevice returned an unexpected error (want not found): %v", err)
			}

			// Verify audit log.
			assertEvents(t, emitter.Events(), []wantEvent{
				{
					Type: events.DeviceDeleteEvent,
					Code: events.DeviceDeleteCode,
				},
			})
		})
	}
}

func TestService_ListDevicesByUser(t *testing.T) {
	const tag = "llama-mac"

	env := testenv.NewUsingT(t)

	devices := env.DevicesClient
	ctx := context.Background()

	// create some device and don't enroll
	_, err := devices.CreateDevice(ctx, &devicepb.CreateDeviceRequest{
		Device: &devicepb.Device{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: "cool-laptop",
		},
	})
	require.NoError(t, err)

	var wantDevs []*devicepb.Device
	// create another device and enroll
	dev, _, err := createAndEnroll(ctx, devices, &devicepb.Device{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: tag,
	})
	require.NoError(t, err)
	wantDevs = append(wantDevs, dev)

	resp, err := devices.ListDevicesByUser(ctx, &devicepb.ListDevicesByUserRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp.Devices)

	// make sure collected data exists, but don't check specifics
	for i, dev := range resp.Devices {
		require.NotNil(t, dev.CollectedData, "resp.Devices[%d].CollectedData is nil", i)
		dev.CollectedData = nil
	}

	require.Equal(t, wantDevs, resp.Devices)
}

func TestService_ListDevices(t *testing.T) {
	env := testenv.NewUsingT(t)

	devices := env.DevicesClient
	ctx := context.Background()

	t.Run("no devices", func(t *testing.T) {
		resp, err := devices.ListDevices(ctx, &devicepb.ListDevicesRequest{})
		if err != nil {
			t.Fatalf("ListDevices failed: %v", err)
		}
		if devs := resp.Devices; len(devs) > 0 {
			t.Errorf("ListDevices returned %v devices, wanted zero: %v", len(devs), devs)
		}
		if resp.NextPageToken != "" {
			t.Error("ListDevices returned a non-empty nextPageToken")
		}
	})

	// Add a few devices to test with.
	var fullDevs []*devicepb.Device
	for _, assetTag := range []string{"llama", "alpaca", "camel"} {
		dev, err := devices.CreateDevice(ctx, &devicepb.CreateDeviceRequest{
			Device: &devicepb.Device{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: assetTag,
			},
		})
		if err != nil {
			t.Fatalf("CreateDevice failed: %v", err)
		}
		fullDevs = append(fullDevs, dev)
	}

	// Transform "fullDevs" into its "list" view equivalent.
	listDevs := make([]*devicepb.Device, len(fullDevs))
	for i, dev := range fullDevs {
		listDevs[i] = &devicepb.Device{
			ApiVersion:   dev.ApiVersion,
			Id:           dev.Id,
			OsType:       dev.OsType,
			AssetTag:     dev.AssetTag,
			CreateTime:   dev.CreateTime,
			UpdateTime:   dev.UpdateTime,
			EnrollStatus: dev.EnrollStatus,
		}
	}

	tests := []struct {
		name        string
		initialReq  *devicepb.ListDevicesRequest
		wantDevices []*devicepb.Device
	}{
		{
			name:        "default view",
			initialReq:  &devicepb.ListDevicesRequest{},
			wantDevices: listDevs,
		},
		{
			name: "list view",
			initialReq: &devicepb.ListDevicesRequest{
				View: devicepb.DeviceView_DEVICE_VIEW_LIST,
			},
			wantDevices: listDevs,
		},
		{
			name: "resource view",
			initialReq: &devicepb.ListDevicesRequest{
				View: devicepb.DeviceView_DEVICE_VIEW_RESOURCE,
			},
			wantDevices: fullDevs,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var got []*devicepb.Device
			req := proto.Clone(test.initialReq).(*devicepb.ListDevicesRequest)
			for {
				resp, err := devices.ListDevices(ctx, req)
				if err != nil {
					t.Fatalf("ListDevices failed: %v", err)
				}

				got = append(got, resp.Devices...)

				if resp.NextPageToken == "" {
					break
				}
				req.PageToken = resp.NextPageToken
			}

			want := test.wantDevices
			slices.SortFunc(want, func(a, b *devicepb.Device) int {
				return strings.Compare(a.Id, b.Id)
			})
			slices.SortFunc(got, func(a, b *devicepb.Device) int {
				return strings.Compare(a.Id, b.Id)
			})
			if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
				t.Errorf("ListDevices mismatch (-want +got)\n%s", diff)
			}
		})
	}
}

func TestService_FindDevices(t *testing.T) {
	env := testenv.NewUsingT(t)

	devices := env.DevicesClient
	ctx := context.Background()

	llamaDev := &devicepb.Device{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "llama",
	}
	alpacaDev := &devicepb.Device{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "alpaca",
	}
	camelDev := &devicepb.Device{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "camel",
	}
	llamaLinux := proto.Clone(llamaDev).(*devicepb.Device)
	llamaLinux.OsType = devicepb.OSType_OS_TYPE_LINUX
	llamaWin := proto.Clone(llamaDev).(*devicepb.Device)
	llamaWin.OsType = devicepb.OSType_OS_TYPE_WINDOWS

	// Create test devices.
	for _, dev := range []**devicepb.Device{&llamaDev, &alpacaDev, &camelDev, &llamaLinux, &llamaWin} {
		created, err := devices.CreateDevice(ctx, &devicepb.CreateDeviceRequest{
			Device: *dev,
		})
		if err != nil {
			t.Fatalf("CreateDevice(%q) failed: %v", (*dev).AssetTag, err)
		}
		*dev = created
	}

	// Create a device whose asset tag matches another's ID.
	// Incredibly unlikely, but let's test anyway.
	camelAssetTag, err := devices.CreateDevice(ctx, &devicepb.CreateDeviceRequest{
		Device: &devicepb.Device{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: camelDev.Id,
		},
	})
	if err != nil {
		t.Fatalf("CreateDevice failed: %v", err)
	}

	tests := []struct {
		name        string
		idOrTag     string
		assertErr   func(error) bool
		wantDevices []*devicepb.Device
	}{
		{
			name:      "id_or_tag required",
			idOrTag:   "",
			assertErr: trace.IsBadParameter,
		},
		{
			name:    "no results",
			idOrTag: "I won't ever match anything",
		},
		{
			name:        "match by ID",
			idOrTag:     llamaDev.Id,
			wantDevices: []*devicepb.Device{llamaDev},
		},
		{
			name:        "match by asset tag (single)",
			idOrTag:     alpacaDev.AssetTag,
			wantDevices: []*devicepb.Device{alpacaDev},
		},
		{
			name:        "match by asset tag (multiple)",
			idOrTag:     llamaDev.AssetTag,
			wantDevices: []*devicepb.Device{llamaDev, llamaLinux, llamaWin},
		},
		{
			name:        "match by ID and asset tag",
			idOrTag:     camelDev.Id, // matches the asset_tag of camelAssetTag
			wantDevices: []*devicepb.Device{camelDev, camelAssetTag},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resp, err := devices.FindDevices(ctx, &devicepb.FindDevicesRequest{
				IdOrTag: test.idOrTag,
			})
			switch {
			case test.assertErr == nil && err == nil: // OK
			case test.assertErr == nil && err != nil:
				t.Fatalf("FindDevices failed: %v", err)
			case !test.assertErr(err):
				t.Fatalf("FindDevices: assertErr failed, err=%v", err)
			}
			if err != nil {
				return
			}

			got := resp.Devices
			want := test.wantDevices
			slices.SortFunc(got, func(a, b *devicepb.Device) int {
				return strings.Compare(a.Id, b.Id)
			})
			slices.SortFunc(want, func(a, b *devicepb.Device) int {
				return strings.Compare(a.Id, b.Id)
			})
			if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
				t.Errorf("FindDevices mismatch (-want +got)\n%s", diff)
			}
		})
	}
}

func TestService_BulkCreateDevices(t *testing.T) {
	emitter := &eventstest.MockRecorderEmitter{}
	env := testenv.NewUsingT(t, testenv.WithEmitter(emitter))

	devices := env.DevicesClient
	ctx := context.Background()

	resp, err := devices.BulkCreateDevices(ctx, &devicepb.BulkCreateDevicesRequest{
		Devices: []*devicepb.Device{
			// Valid.
			{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "llama",
			},
			// Valid.
			{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "alpaca",
			},
			// Invalid: missing asset tag.
			{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "",
			},
			// Invalid: duplicate asset tag for macOS.
			{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "llama",
			},
			// Valid.
			{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "camel",
			},
			// Invalid: nil.
			nil,
		},
	})
	if err != nil {
		t.Fatalf("BulkCreateDevice failed: %v", err)
	}

	// Verify response codes.
	wantCodes := []codes.Code{
		codes.OK,              // llama
		codes.OK,              // alpaca
		codes.InvalidArgument, // missing asset tag
		codes.AlreadyExists,   // duplicate asset tag
		codes.OK,              // camel
		codes.InvalidArgument, // nil
	}
	var gotCodes []codes.Code
	for i, dev := range resp.Devices {
		// Using GetCode() because Status can be nil for successes.
		code := codes.Code(dev.Status.GetCode())
		gotCodes = append(gotCodes, code)

		// Sanity check details about the responses.
		switch {
		case code == codes.OK && dev.Id == "":
			t.Errorf("BulkCreateDevice: resp.Devices[%v].Id is empty, want non-empty", i)
		case code != codes.OK && dev.Status.GetMessage() == "":
			t.Errorf("BulkCreateDevice: resp.Devices[%v].Message is empty, want non-empty for code %s", i, code)
		}
	}
	if diff := cmp.Diff(wantCodes, gotCodes); diff != "" {
		t.Fatalf("BulkCreateDevice codes mismatch (-want +got)\n%s", diff)
	}

	// Verify that IDs match the expected asset tags.
	// BulkCreate response order matches the request order.
	wantDevices := map[string]string{
		resp.Devices[0].Id: "llama",
		resp.Devices[1].Id: "alpaca",
		resp.Devices[4].Id: "camel",
	}
	listResp, err := devices.ListDevices(ctx, &devicepb.ListDevicesRequest{})
	switch {
	case err != nil:
		t.Fatalf("ListDevices failed: %v", err)
	case listResp.NextPageToken != "":
		t.Fatal("ListDevices returned a non-empty nextPageToken")
	}
	gotDevices := make(map[string]string)
	for _, got := range listResp.Devices {
		gotDevices[got.Id] = got.AssetTag
	}
	if diff := cmp.Diff(wantDevices, gotDevices); diff != "" {
		t.Errorf("Created devices mismatch (-want +got)\n%s", diff)
	}

	// Verify audit log.
	wantEvents := []wantEvent{
		// llama
		{
			Type: events.DeviceCreateEvent,
			Code: events.DeviceCreateCode,
		},
		// alpaca
		{
			Type: events.DeviceCreateEvent,
			Code: events.DeviceCreateCode,
		},
		// camel
		{
			Type: events.DeviceCreateEvent,
			Code: events.DeviceCreateCode,
		},
	}
	assertEvents(t, emitter.Events(), wantEvents)
}

func TestService_CreateDeviceEnrollToken(t *testing.T) {
	emitter := &eventstest.MockRecorderEmitter{}
	env := testenv.NewUsingT(t, testenv.WithEmitter(emitter))

	devices := env.DevicesClient
	ctx := context.Background()

	dev, err := devices.CreateDevice(ctx, &devicepb.CreateDeviceRequest{
		Device: &devicepb.Device{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: "llama",
		},
	})
	if err != nil {
		t.Fatalf("CreateDevice failed: %v", err)
	}

	tests := []struct {
		name      string
		deviceID  string
		assertErr func(err error) bool
	}{
		{
			name:      "ok",
			deviceID:  dev.Id,
			assertErr: func(err error) bool { return err == nil },
		},
		{
			name:      "override ok",
			deviceID:  dev.Id,
			assertErr: func(err error) bool { return err == nil },
		},
		{
			name:      "empty device ID fails",
			deviceID:  "",
			assertErr: trace.IsBadParameter,
		},
		{
			name:      "unknown device fails",
			deviceID:  "unknown",
			assertErr: trace.IsNotFound,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			emitter.Reset()

			token, err := devices.CreateDeviceEnrollToken(ctx, &devicepb.CreateDeviceEnrollTokenRequest{
				DeviceId: test.deviceID,
			})
			if !test.assertErr(err) {
				t.Fatalf("CreateDeviceEnrollToken: assertErr failed, err=%v", err)
			}
			if err != nil {
				return
			}

			// Verify that the token is not empty.
			if token.GetToken() == "" {
				t.Error("CreateDeviceEnrollToken returned a nil or empty token")
			}

			// Verify audit log.
			assertEvents(t, emitter.Events(), []wantEvent{
				{
					Type: events.DeviceEnrollTokenCreateEvent,
					Code: events.DeviceEnrollTokenCreateCode,
				},
			})
		})
	}
}

func TestService_CreateDeviceEnrollToken_autoEnroll(t *testing.T) {
	// Prepare an authorizer and a set of users with the following powers:
	// - adminUser: logged in and has all necessary verbs
	// - endUser: logged in but has no device verbs
	// - unknownUser: not logged in
	const adminUser = "llama"
	const endUser = "alpaca"
	const unknownUser = "eve"
	authorizer := &userAwareAuthorizer{
		knownUsers:      []string{adminUser, endUser},
		authorizedUsers: []string{adminUser},
	}

	emitter := &eventstest.MockRecorderEmitter{}
	dt := &types.DeviceTrust{
		Mode: constants.DeviceTrustModeRequired,
	}
	env := testenv.NewUsingT(
		t,
		testenv.WithAuthorizer(authorizer),
		testenv.WithEmitter(emitter),
		testenv.WithAuthPreferenceSpec(types.AuthPreferenceSpecV2{
			DeviceTrust: dt,
		}),
	)
	defer env.Close()
	devices := env.DevicesClient

	ctx := context.Background()

	// Register a device for testing.
	dev, err := devices.CreateDevice(contextWithUser(ctx, adminUser), &devicepb.CreateDeviceRequest{
		Device: &devicepb.Device{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: "llama1",
			// Have a profile so we can check that validation happens, but we don't
			// need an extensive profile here.
			Profile: &devicepb.DeviceProfile{
				ModelIdentifier: "MacBookPro9,2",
				OsVersion:       "13.3.1",
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateDevice failed: %v", err)
	}

	cdValid := defaultCollectData(dev)
	cdBad := proto.Clone(cdValid).(*devicepb.DeviceCollectedData)
	cdBad.ModelIdentifier = "MacBookPro9,3" // doesn't match
	cdUnknown := &devicepb.DeviceCollectedData{
		CollectTime:  timestamppb.Now(),
		OsType:       devicepb.OSType_OS_TYPE_MACOS,
		SerialNumber: "unknown",
	}

	type testCase struct {
		name      string
		user      string
		req       *devicepb.CreateDeviceEnrollTokenRequest
		assertErr func(err error) bool
		wantErr   string // optional, used to disambiguate errors.
	}
	runTests := func(t *testing.T, tests []testCase) {
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				emitter.Reset()

				token, err := devices.CreateDeviceEnrollToken(contextWithUser(ctx, test.user), test.req)
				if !test.assertErr(err) {
					t.Errorf("CreateDeviceEnrollToken: assertErr failed, err=%v (%T)", err, err)
				}
				if err != nil {
					assert.ErrorContains(t, err, test.wantErr, "CreateDeviceEnrollToken error mismatch")
					return
				}

				// Verify that token is returned.
				if token.GetToken() == "" {
					t.Errorf("CreateDeviceEnrollToken got=%v, want non-empty", token)
				}

				// Verify audit events.
				assertEvents(t, emitter.Events(), []wantEvent{
					{
						Type: events.DeviceEnrollTokenCreateEvent,
						Code: events.DeviceEnrollTokenCreateCode,
					},
				})
			})
		}
	}

	assertNoErr := func(err error) bool { return err == nil }

	// We run 2 batches of scenarios below, first with auto-enroll disabled and
	// later with enabled.
	// The same device is used for the majority of tests. This works because the
	// device is never enrolled, so multiple token creation is allowed.

	runTests(t, []testCase{
		{
			name: "admin auto-enroll not allowed",
			user: adminUser,
			req: &devicepb.CreateDeviceEnrollTokenRequest{
				DeviceData: cdValid,
			},
			assertErr: trace.IsBadParameter,
			wantErr:   "device ID",
		},
		{
			name: "admin with both DeviceId and cd favors DeviceId",
			user: adminUser,
			req: &devicepb.CreateDeviceEnrollTokenRequest{
				DeviceId:   dev.Id,
				DeviceData: cdUnknown,
			},
			assertErr: assertNoErr,
		},
		{
			name:      "admin empty request fails",
			user:      adminUser,
			req:       &devicepb.CreateDeviceEnrollTokenRequest{},
			assertErr: trace.IsBadParameter,
			wantErr:   "device ID",
		},
		{
			name: "user auto-enroll not allowed",
			user: endUser,
			req: &devicepb.CreateDeviceEnrollTokenRequest{
				DeviceData: cdValid,
			},
			assertErr: trace.IsAccessDenied,
		},
		{
			name: "user device ID not allowed",
			user: endUser,
			req: &devicepb.CreateDeviceEnrollTokenRequest{
				DeviceId: dev.Id,
			},
			assertErr: trace.IsAccessDenied,
		},
		{
			name:      "user empty request fails",
			user:      endUser,
			req:       &devicepb.CreateDeviceEnrollTokenRequest{},
			assertErr: trace.IsAccessDenied,
		},
		{
			name: "unknown user auto-enroll not allowed",
			user: unknownUser,
			req: &devicepb.CreateDeviceEnrollTokenRequest{
				DeviceData: cdValid,
			},
			assertErr: trace.IsAccessDenied,
		},
		{
			name: "unknown user device ID not allowed",
			user: unknownUser,
			req: &devicepb.CreateDeviceEnrollTokenRequest{
				DeviceId: dev.Id,
			},
			assertErr: trace.IsAccessDenied,
		},
	})

	assertAutoEnrollError := func(err error) bool {
		return err != nil &&
			trace.IsBadParameter(err) &&
			// Note: errors is redacted and original error is written to audit.
			strings.Contains(err.Error(), "auto-enroll verifications")
	}

	dt.AutoEnroll = true
	runTests(t, []testCase{
		{
			name: "admin auto-enroll",
			user: adminUser,
			req: &devicepb.CreateDeviceEnrollTokenRequest{
				DeviceData: cdValid,
			},
			assertErr: assertNoErr,
		},
		{
			name: "admin with both DeviceId and cd favors DeviceId",
			user: adminUser,
			req: &devicepb.CreateDeviceEnrollTokenRequest{
				// This is a tad unrealistic, but should still work without issue.
				// The DeviceId is favored if both are present.
				DeviceId: dev.Id,
				// DeviceData ignored.
				DeviceData: cdBad,
			},
			assertErr: assertNoErr,
		},
		{
			name: "admin invalid cd fails",
			user: adminUser,
			req: &devicepb.CreateDeviceEnrollTokenRequest{
				DeviceData: cdBad,
			},
			assertErr: trace.IsBadParameter,
			wantErr:   "model drift",
		},
		{
			name: "admin unknown device fails",
			user: adminUser,
			req: &devicepb.CreateDeviceEnrollTokenRequest{
				DeviceData: cdUnknown,
			},
			assertErr: trace.IsNotFound,
		},
		{
			name:      "admin empty request fails",
			user:      adminUser,
			req:       &devicepb.CreateDeviceEnrollTokenRequest{},
			assertErr: trace.IsBadParameter,
			wantErr:   "device ID",
		},
		{
			name: "user auto-enroll",
			user: endUser,
			req: &devicepb.CreateDeviceEnrollTokenRequest{
				DeviceId:   "unknown", // ignored
				DeviceData: cdValid,
			},
			assertErr: assertNoErr,
		},
		{
			name: "user device ID not allowed",
			user: endUser,
			req: &devicepb.CreateDeviceEnrollTokenRequest{
				DeviceId: dev.Id,
			},
			assertErr: assertAutoEnrollError, // redacted
		},
		{
			name:      "user empty request fails",
			user:      endUser,
			req:       &devicepb.CreateDeviceEnrollTokenRequest{},
			assertErr: assertAutoEnrollError, // redacted
		},
		{
			name: "user invalid cd fails",
			user: endUser,
			req: &devicepb.CreateDeviceEnrollTokenRequest{
				DeviceData: cdBad,
			},
			assertErr: assertAutoEnrollError, // redacted
		},
		{
			name: "user unknown device fails",
			user: endUser,
			req: &devicepb.CreateDeviceEnrollTokenRequest{
				DeviceData: cdUnknown,
			},
			assertErr: assertAutoEnrollError, // redacted
		},
		{
			name: "unknown user auto-enroll not allowed",
			user: unknownUser,
			req: &devicepb.CreateDeviceEnrollTokenRequest{
				DeviceData: cdValid,
			},
			assertErr: trace.IsAccessDenied,
		},
		{
			name: "unknown user device ID not allowed",
			user: unknownUser,
			req: &devicepb.CreateDeviceEnrollTokenRequest{
				DeviceId: dev.Id,
			},
			assertErr: trace.IsAccessDenied,
		},
	})
}

func TestService_CreateDeviceEnrollToken_autoEnrollAudit(t *testing.T) {
	// Test that auto-enroll failures are written to audit.

	const adminUser = "llama"
	const endUser = "alpaca"
	authorizer := &userAwareAuthorizer{
		knownUsers:      []string{adminUser, endUser}, // Passes Authorize() calls.
		authorizedUsers: []string{adminUser},          // Passes CheckAccessToRule() calls.
	}

	dt := &types.DeviceTrust{
		Mode:       constants.DeviceTrustModeOptional,
		AutoEnroll: true,
	}
	emitter := &eventstest.MockRecorderEmitter{}
	env := testenv.NewUsingT(
		t,
		testenv.WithAuthorizer(authorizer),
		testenv.WithAuthPreferenceSpec(types.AuthPreferenceSpecV2{
			DeviceTrust: dt,
		}),
		testenv.WithEmitter(emitter),
	)
	defer env.Close()

	devices := env.DevicesClient
	ctx := context.Background()

	// Enroll device for "adminUser". This stops other users from auto-enrolling
	// the device.
	dev, _, err := createAndEnroll(contextWithUser(ctx, adminUser), devices, &devicepb.Device{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "device1",
	})
	if err != nil {
		t.Fatalf("createAndEnroll failed: %v", err)
	}
	emitter.Reset()

	t.Run("write failure to audit", func(t *testing.T) {
		endUserCtx := contextWithUser(ctx, endUser)

		// Attempt to enroll an already-enrolled device. This should cause the
		// ceremony to fail and a success=false audit event.
		_, err := devices.CreateDeviceEnrollToken(endUserCtx, &devicepb.CreateDeviceEnrollTokenRequest{
			DeviceData: defaultCollectData(dev),
		})
		require.ErrorContains(t, err, "auto-enroll verifications", "CreateDeviceEnrollToken error mismatch")

		// Assert audit events.
		gotEvents := emitter.Events()
		assertEvents(t, gotEvents, []wantEvent{
			{
				Type:     events.DeviceEnrollTokenCreateEvent,
				Code:     events.DeviceEnrollTokenCreateCode,
				WantFail: true,
			},
		})
		// We rely on assertEvents to check the basics of the layout, so just don't
		// panic here.
		if len(gotEvents) == 0 {
			return
		}
		event, ok := gotEvents[0].(*apievents.DeviceEvent2)
		if !ok {
			t.Fatalf("event = %T, want %T", event, &apievents.DeviceEvent2{})
		}
		if got, want := event.Status.UserMessage, "already enrolled"; !strings.Contains(got, want) {
			t.Errorf("event.Status.UserMessage = %q, want %q", got, want)
		}
	})
}

func TestService_EnrollDevice_autoEnrollE2E(t *testing.T) {
	// Test that auto-enrollment is possible, end-to-end, even for a user that has
	// no special device verbs.

	const adminUser = "llama"
	const endUser = "alpaca"
	authorizer := &userAwareAuthorizer{
		knownUsers:      []string{adminUser, endUser}, // Passes Authorize() calls.
		authorizedUsers: []string{adminUser},          // Passes CheckAccessToRule() calls.
	}

	dt := &types.DeviceTrust{
		Mode:       constants.DeviceTrustModeRequired,
		AutoEnroll: true,
	}
	env := testenv.NewUsingT(
		t,
		testenv.WithAuthorizer(authorizer),
		testenv.WithAuthPreferenceSpec(types.AuthPreferenceSpecV2{
			DeviceTrust: dt,
		}),
	)
	defer env.Close()

	devices := env.DevicesClient
	ctx := context.Background()

	// Register device.
	dev, err := devices.CreateDevice(contextWithUser(ctx, adminUser), &devicepb.CreateDeviceRequest{
		Device: &devicepb.Device{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: "alpaca-dev-1",
		},
	})
	if err != nil {
		t.Fatalf("CreateDevice failed: %v", err)
	}

	type createTokenParams struct {
		user string
		req  *devicepb.CreateDeviceEnrollTokenRequest
	}

	autoEnrollToken := func() createTokenParams {
		return createTokenParams{
			user: endUser,
			req: &devicepb.CreateDeviceEnrollTokenRequest{
				DeviceData: defaultCollectData(dev), // Auto-enroll token
			},
		}
	}

	adminToken := func() createTokenParams {
		return createTokenParams{
			user: adminUser,
			req: &devicepb.CreateDeviceEnrollTokenRequest{
				DeviceId: dev.Id, // "Regular" token.
			},
		}
	}

	// Note: tests run in sequence and may alter system state.
	tests := []struct {
		name            string
		createToken     createTokenParams
		enrollUser      string
		beforeEnroll    func(t *testing.T) // Optional.
		assertEnrollErr func(error) bool   // Optional. nil means no error wanted.
	}{
		{
			name:        "ok",
			createToken: autoEnrollToken(),
			enrollUser:  endUser,
		},
		{
			name:            "exemption only applies to auto-enroll tokens",
			createToken:     adminToken(),
			enrollUser:      endUser,
			assertEnrollErr: trace.IsAccessDenied, // needs device/enroll permissions
		},

		// Keep this last, it changes system settings.
		{
			name:        "exemption only applies if auto-enroll is enabled",
			createToken: adminToken(),
			enrollUser:  endUser,
			beforeEnroll: func(_ *testing.T) {
				// Disable system auto-enroll.
				dt.AutoEnroll = false
			},
			assertEnrollErr: trace.IsAccessDenied,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Create DeviceEnrollToken. The request decides whether it's an auto-enroll
			// token or not.
			enrollToken, err := devices.CreateDeviceEnrollToken(
				contextWithUser(ctx, test.createToken.user),
				test.createToken.req,
			)
			if err != nil {
				t.Fatalf("CreateDeviceEnrollToken failed: %v", err)
			}
			dev.EnrollToken = enrollToken

			// Run the before EnrollDevice hook.
			if test.beforeEnroll != nil {
				test.beforeEnroll(t)
			}

			// Attempt to enroll the device.
			// User + enrollToken will decide the outcome.
			_, _, err = enrollDevice(
				contextWithUser(ctx, test.enrollUser),
				devices, dev, defaultCollectData,
			)
			switch {
			case test.assertEnrollErr != nil:
				if !test.assertEnrollErr(err) {
					t.Errorf("EnrollDevice assertErr failed: err=%v (%T)", err, trace.Unwrap(err))
				}
			case err != nil:
				t.Errorf("EnrollDevice returned err=%v, want nil", err)
			}
		})
	}
}

func TestService_DeviceEnrollToken_expireTime(t *testing.T) {
	env := testenv.NewUsingT(t, testenv.WithAuthPreferenceSpec(types.AuthPreferenceSpecV2{
		DeviceTrust: &types.DeviceTrust{
			AutoEnroll: true,
		},
	}))

	devices := env.DevicesClient
	ctx := context.Background()

	createdDev, err := devices.CreateDevice(ctx, &devicepb.CreateDeviceRequest{
		Device: &devicepb.Device{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: "llama",
		},
	})
	if err != nil {
		t.Fatalf("CreateDevice failed: %v", err)
	}

	expireTime := time.Now().Add(244 * time.Minute) // arbitrary, "weird" time
	expirePB := timestamppb.New(expireTime)

	tests := []struct {
		name           string
		rpc            func() (*devicepb.DeviceEnrollToken, error)
		wantExpireTime time.Time
	}{
		{
			name: "CreateDevice",
			rpc: func() (*devicepb.DeviceEnrollToken, error) {
				dev, err := devices.CreateDevice(ctx, &devicepb.CreateDeviceRequest{
					Device: &devicepb.Device{
						OsType:   devicepb.OSType_OS_TYPE_MACOS,
						AssetTag: "create-device-test",
					},
					CreateEnrollToken:     true,
					EnrollTokenExpireTime: expirePB,
				})
				return dev.GetEnrollToken(), err
			},
			wantExpireTime: expireTime,
		},
		{
			name: "CreateDeviceEnrollToken",
			rpc: func() (*devicepb.DeviceEnrollToken, error) {
				return devices.CreateDeviceEnrollToken(ctx, &devicepb.CreateDeviceEnrollTokenRequest{
					DeviceId:   createdDev.Id,
					ExpireTime: expirePB,
				})
			},
			wantExpireTime: expireTime,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			token, err := test.rpc()
			if err != nil {
				t.Fatalf("rpc failed: %v", err)
			}
			if token.ExpireTime.AsTime().Unix() != test.wantExpireTime.Unix() {
				t.Errorf("rpc returned ExpireTime=%v, want %v", token.ExpireTime, test.wantExpireTime)
			}
		})
	}

	t.Run("CreateDeviceEnrollToken: auto-enroll ignores custom expire time", func(t *testing.T) {
		token, err := devices.CreateDeviceEnrollToken(ctx, &devicepb.CreateDeviceEnrollTokenRequest{
			DeviceData: defaultCollectData(createdDev),
			ExpireTime: expirePB,
		})
		if err != nil {
			t.Fatalf("CreateDeviceEnrollToken failed: %v", err)
		}

		if token.ExpireTime.AsTime().Unix() == expireTime.Unix() {
			t.Error("CreateDeviceEnrollToken: auto-enroll should ignore custom ExpireTime")
		}
	})
}

func TestService_dataDriftErrorsRedacted(t *testing.T) {
	env := testenv.NewUsingT(t)
	defer env.Close()

	devices := env.DevicesClient
	ctx := context.Background()

	// Create a device with a profile for testing.
	// This raises the bar that collected data has to meet.
	dev, key, err := createAndEnroll(ctx, devices, &devicepb.Device{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "llama1",
		Profile: &devicepb.DeviceProfile{
			ModelIdentifier: "MacBookPro9,2",
		},
	})
	if err != nil {
		t.Fatalf("createAndEnroll failed: %v", err)
	}

	// Authenticate adding some "organic" collected data, beyond what is in the
	// profile.
	organicCollectData := func(dev *devicepb.Device) *devicepb.DeviceCollectedData {
		cd := defaultCollectData(dev)
		// Add some organic data, beyond the profile
		cd.OsVersion = "13.2.1"
		cd.OsBuild = "22D68"
		cd.OsUsername = "llama"
		cd.OsLoginUser = "llama"
		return cd
	}

	authenticate := func(fn func(dev *devicepb.Device) *devicepb.DeviceCollectedData) error {
		sim := key.simulator(withCollectFn(dev, fn))
		_, err := authenticateSimulator(ctx, devices, sim, dev, nil /* initCerts */)
		return err
	}

	if err := authenticate(organicCollectData); err != nil {
		t.Fatalf("authenticate failed: %v", err)
	}

	badCollectData := func(dev *devicepb.Device) *devicepb.DeviceCollectedData {
		cd := organicCollectData(dev)
		cd.ModelIdentifier = "MacBookPro9,3" // model can't change
		return cd
	}

	tests := []struct {
		name string
		rpc  func() error // RPC is supposed to fail with a drift error.
	}{
		{
			name: "authenticate",
			rpc: func() error {
				return authenticate(badCollectData)
			},
		},
		{
			name: "enroll",
			rpc: func() error {
				_, _, err := enrollDevice(ctx, devices, dev, badCollectData)
				return err
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotErr := test.rpc()
			if gotErr == nil {
				t.Fatal("Got err=nil, wanted non-nil")
			}
			if !trace.IsAccessDenied(gotErr) {
				t.Errorf("Got err=%v (%T), want trace.AccessDeniedError", gotErr, gotErr)
			}
			if !strings.Contains(gotErr.Error(), devicetrustv1.DataDriftDetectedMessage) {
				t.Errorf("Got err=%v, want %q (redacted data drift message)", gotErr, devicetrustv1.DataDriftDetectedMessage)
			}
		})
	}
}

func TestService_GetResourceDevicesUsage(t *testing.T) {
	t.Parallel()
	m := &modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.DeviceTrust:            {Enabled: true},
				entitlements.MobileDeviceManagement: {Enabled: true},
			},
		},
	}
	env := testenv.NewUsingT(t, testenv.WithModules(m))

	devices := env.DevicesClient
	service := env.DevicesService
	ctx := context.Background()

	// Enroll a device so the count is not zero.
	if _, _, err := createAndEnroll(ctx, devices, &devicepb.Device{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "llama",
	}); err != nil {
		t.Fatalf("createAndEnroll failed: %v", err)
	}

	tests := []struct {
		name           string
		modifyFeatures func(f *modules.Features)
		want           *resourceusagepb.DevicesUsage
	}{
		{
			name: "unlimited account",
			want: &resourceusagepb.DevicesUsage{},
		},
		{
			name: "usage-based account",
			modifyFeatures: func(f *modules.Features) {
				f.IsUsageBasedBilling = true
				f.Entitlements[entitlements.DeviceTrust] = modules.EntitlementInfo{Limit: 5}
			},
			want: &resourceusagepb.DevicesUsage{
				DevicesUsageLimit: 5,
				DevicesInUse:      1,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.modifyFeatures != nil {
				test.modifyFeatures(&m.TestFeatures)
			}

			f := m.Features()
			got, err := service.GetResourceDevicesUsage(ctx, &f)
			if err != nil {
				t.Fatalf("GetResourceDevicesUsage failed: %v", err)
			}

			if diff := cmp.Diff(test.want, got, protocmp.Transform()); diff != "" {
				t.Errorf("GetResourceDevicesUsage mismatch (-want +got)\n%s", diff)
			}
		})
	}
}

type wantEvent struct {
	Type, Code string
	WantFail   bool
}

func assertEvents(t *testing.T, got []apievents.AuditEvent, want []wantEvent) {
	t.Helper()

	if len(got) != len(want) {
		t.Errorf("Audit: found an unexpected number events: got %v, want %v", len(got), len(want))
		return
	}

	for i, g := range got {
		w := want[i]

		// Sanity check type/code.
		switch {
		case g.GetType() == "device":
			t.Errorf("Audit: got[%v].Type = %v is the legacy, catch-all event type", i, g.GetType())
		case !strings.HasPrefix(g.GetType(), "device."):
			t.Errorf(`Audit: got[%v].Type = %v does not begin with "device.", it could be an event code instead`, i, g.GetType())
		}
		if !strings.HasPrefix(g.GetCode(), "TV") {
			t.Errorf(`Audit: got[%v].Code = %v does not begin with "TV", is it a device event?`, i, g.GetType())
		}

		if g.GetType() != w.Type {
			t.Errorf("Audit: event mismatch: got[%v].Type = %v, want %v", i, g.GetType(), w.Type)
		}
		if g.GetCode() != w.Code {
			t.Errorf("Audit: event mismatch: got[%v].Code = %v, want %v", i, g.GetCode(), w.Code)
		}

		devEvent, ok := g.(*apievents.DeviceEvent2)
		switch {
		case !ok:
			t.Errorf("Audit: event mismatch: got[%v] is not a DeviceEvent: %T", i, devEvent)
		case devEvent.Success == w.WantFail:
			t.Errorf("Audit: event mismatch: got[%v].Status.Success = %v, want %v", i, devEvent.Status.Success, !w.WantFail)
		case gogoproto.Equal(&devEvent.UserMetadata, &apievents.UserMetadata{}):
			t.Errorf("Audit: event mismatch: got[%v].User has no fields set", i)
		case !devEvent.Success:
			// Abort here, failures can't always inform the device.
			return
		case devEvent.Device == nil:
			t.Errorf("Audit: event mismatch: got[%v].Device=nil, want non-nil", i)
		case devEvent.Device.DeviceId == "":
			t.Errorf(`Audit: event mismatch: got[%v].Device.DeviceId="", want non-empty`, i)
		}
	}
}

func TestService_EnrollDevice_issuesDevicesLimitEvent(t *testing.T) {
	t.Parallel()

	var emittedEvents []usagereporter.Anonymizable
	fakeAnonymizeAndSubmit := func(events ...usagereporter.Anonymizable) {
		emittedEvents = append(emittedEvents, events...)
	}
	env := testenv.NewUsingT(
		t,
		testenv.WithAnonymizeAndSubmitFunc(fakeAnonymizeAndSubmit),
		testenv.WithModules(&modulestest.Modules{
			TestBuildType: modules.BuildEnterprise,
			TestFeatures: modules.Features{
				IsUsageBasedBilling: true,
				Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
					entitlements.DeviceTrust:            {Enabled: true, Limit: 1},
					entitlements.MobileDeviceManagement: {Enabled: true},
				},
			},
		}),
	)
	devicesClient := env.DevicesClient
	ctx := context.Background()

	if _, _, err := createAndEnroll(ctx, devicesClient, &devicepb.Device{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "llama",
	}); err != nil {
		t.Fatalf("createAndEnroll failed: %v", err)
	}

	wantLimitEvent := []usagereporter.Anonymizable{&usagereporter.LicenseLimitEvent{
		LicenseLimit: prehogv1alpha.LicenseLimit_LICENSE_LIMIT_DEVICE_TRUST_TEAM_USAGE,
	}}

	_, _, err := createAndEnroll(ctx, devicesClient, &devicepb.Device{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "exceeded-llama",
	})
	assert.ErrorContains(t, err, "device limit")
	assert.Equal(t, wantLimitEvent, emittedEvents)
}

func TestService_deviceModeOff(t *testing.T) {
	const user = "llama"
	allUsers := []string{user}

	env := testenv.NewUsingT(t,
		testenv.WithAuthPreferenceSpec(types.AuthPreferenceSpecV2{
			Type:         constants.Local,
			SecondFactor: constants.SecondFactorOTP, // unimportant
			DeviceTrust: &types.DeviceTrust{
				Mode: constants.DeviceTrustModeOff, // device authn disabled by default
			},
		}),
		testenv.WithAuthorizer(&userAwareAuthorizer{
			knownUsers:      allUsers,
			authorizedUsers: allUsers,
		}),
	)

	devicesClient := env.DevicesClient
	identity := env.IdentityService
	service := env.DevicesService
	ctx := context.Background()

	// Create user.
	u, err := types.NewUser(user)
	if err != nil {
		t.Fatalf("NewUser() failed: %v", err)
	}
	if _, err := identity.CreateUser(ctx, u); err != nil {
		t.Fatalf("CreateUser() failed: %v", err)
	}

	// Create and enroll a device for the user.
	userCtx := contextWithUser(ctx, user)
	dev, key, err := createAndEnroll(userCtx, devicesClient, &devicepb.Device{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "llama-mac1",
	})
	if err != nil {
		t.Fatalf("createAndEnroll failed: %v", err)
	}

	t.Run("AuthenticateDevice", func(t *testing.T) {
		t.Parallel()

		_, err := authenticateSimulator(userCtx, devicesClient, key.simulator(), dev, nil /* initCerts */)
		assert.ErrorContains(t, err, "device trust disabled", "AuthenticateDevice error mismatch")
	})

	t.Run("CreateDeviceWebToken", func(t *testing.T) {
		t.Parallel()

		token, err := service.CreateDeviceWebToken(ctx, &devicepb.DeviceWebToken{
			WebSessionId:     "my-web-session-id",
			BrowserUserAgent: sampleUserAgentMacOS,
			BrowserIp:        sampleIP,
			User:             user,
		})
		if err != nil {
			t.Fatalf("CreateDeviceWebToken failed unexpectedly: %v", err)
		}
		if token != nil {
			t.Errorf("CreateDeviceWebToken=%v, want nil", token)
		}
	})
}

func TestService_CreateDeviceWebToken(t *testing.T) {
	const userLlama = "llama"
	const userAlpaca = "alpaca"
	allUsers := []string{userLlama, userAlpaca}

	emitter := &eventstest.MockRecorderEmitter{}
	env := testenv.NewUsingT(t,
		testenv.WithAuthorizer(&userAwareAuthorizer{
			knownUsers:      allUsers,
			authorizedUsers: allUsers,
		}),
		testenv.WithEmitter(emitter),
	)

	devicesClient := env.DevicesClient
	identity := env.IdentityService
	service := env.DevicesService
	ctx := context.Background()

	// Create the users above.
	for _, user := range allUsers {
		u, err := types.NewUser(user)
		if err != nil {
			t.Fatalf("NewUser(%q) failed: %v", user, err)
		}
		if _, err := identity.CreateUser(ctx, u); err != nil {
			t.Fatalf("CreateUser(%q) failed: %v", user, err)
		}
	}

	createAndEnrollForUser := func(t *testing.T, user string, dev *devicepb.Device) (*devicepb.Device, *fakeEnclaveKey) {
		userCtx := contextWithUser(ctx, user)
		dev, key, err := createAndEnroll(userCtx, devicesClient, dev)
		if err != nil {
			t.Fatalf("createAndEnroll failed: %v", err)
		}
		return dev, key
	}

	// Llama has 2 macOS trusted devices:
	_, _ = createAndEnrollForUser(t, userLlama, &devicepb.Device{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "llama-mac1",
	})
	_, _ = createAndEnrollForUser(t, userLlama, &devicepb.Device{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "llama-mac2",
	})

	// Alpaca has no trusted devices.

	makeToken := func(owner, ua string) *devicepb.DeviceWebToken {
		return &devicepb.DeviceWebToken{
			WebSessionId:     "my-web-session-id", // OK to fake, not looked up at this stage.
			BrowserUserAgent: ua,
			BrowserIp:        sampleIP,
			User:             owner,
		}
	}

	tests := []struct {
		name      string
		token     *devicepb.DeviceWebToken
		assertErr func(error) bool
		wantErr   string
		wantToken bool // true if a non-nil token is expected
	}{
		{
			name:      "success",
			token:     makeToken(userLlama, sampleUserAgentMacOS),
			wantToken: true,
		},
		{
			name:  "user has no suitable trusted device (1)",
			token: makeToken(userLlama, sampleUserAgentLinux),
			// want `nil, nil`
		},
		{
			name:  "user has no suitable trusted device (2)",
			token: makeToken(userLlama, sampleUserAgentWindows),
			// want `nil, nil`
		},
		{
			name:  "user has no trusted devices",
			token: makeToken(userAlpaca, sampleUserAgentMacOS),
			// want `nil, nil`
		},
		{
			name:      "nil token",
			token:     nil,
			assertErr: trace.IsBadParameter,
			wantErr:   "token required",
		},
		{
			name:      "BrowserUserAgent empty",
			token:     makeToken(userLlama, ""),
			assertErr: trace.IsBadParameter,
			wantErr:   "user agent required",
		},
		{
			name:  "BrowserUserAgent invalid",
			token: makeToken(userLlama, "ceci n'est pas a user agent"),
			// want `nil, nil`
		},
		{
			name:      "User empty",
			token:     makeToken("", sampleUserAgentMacOS),
			assertErr: trace.IsBadParameter,
			wantErr:   "user required",
		},
		{
			name:      "User unknown",
			token:     makeToken("unknown", sampleUserAgentMacOS),
			assertErr: trace.IsNotFound,
		},
		{
			name: "token is validated",
			token: func() *devicepb.DeviceWebToken {
				token := makeToken(userLlama, sampleUserAgentMacOS)
				token.BrowserIp = "" // Not used directly, validated by storage.
				return token
			}(),
			assertErr: trace.IsBadParameter,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			emitter.Reset()
			got, err := service.CreateDeviceWebToken(ctx, test.token)

			// Assert error type and contents.
			if test.assertErr != nil && !test.assertErr(err) {
				t.Errorf("CreateDeviceWebToken assertErr failed: err=%v (%T)", err, trace.Unwrap(err))
			}
			if test.wantErr != "" {
				assert.ErrorContains(t, err, test.wantErr, "CreateDeviceWebToken error mismatch")
			}
			if err != nil {
				return
			}

			// `nil, nil` is a valid return, test for those scenarios.
			if !test.wantToken {
				if got != nil {
					t.Errorf("CreateDeviceWebToken returned token=%#v, want nil token", got)
				}
				return
			}
			if got == nil {
				t.Fatal("CreateDeviceWebToken returned a nil token, want non-nil")
				return // return to make golangci-lint happy
			}

			// Do some light assertions in the returned token.
			// We trust storage to assert it in depth.
			if got.Id == "" {
				t.Errorf("CreateDeviceWebToken returned token without ID: %#v", got)
			}
			if got.Token == "" {
				t.Errorf("CreateDeviceWebToken returned token without the token itself: %#v", got)
			}

			allEvents := emitter.Events()
			assertEvents(t, allEvents, []wantEvent{
				{
					Type: events.DeviceWebTokenCreateEvent,
					Code: events.DeviceWebTokenCreateCode,
				},
			})

			// Assert that the audit event user is the token.User, the context user
			// here is the Auth process.
			//
			// Do some rudimentary checks below to avoid panics, but otherwise we rely
			// on the assertEvents call above to verify the number and type of events.
			if len(allEvents) == 1 {
				if event, ok := allEvents[0].(*apievents.DeviceEvent2); ok {
					if event.User != test.token.User {
						t.Errorf("Audit event user mismatch: got=%q, want %q", event.User, test.token.User)
					}
				}
			}
		})
	}
}

func TestService_CreateDeviceWebToken_unknownDevices(t *testing.T) {
	const userTrustedDevices = "llama"
	const userDevicesIndex = "alpaca"
	allUsers := []string{userTrustedDevices, userDevicesIndex}

	augmentWebFunc := &fakeAugmentWebFunc{}
	emitter := &keyedEmitter{}
	env := testenv.NewUsingT(t,
		testenv.WithAugmentWebFunc(augmentWebFunc.function),
		testenv.WithAuthorizer(&userAwareAuthorizer{
			knownUsers:      allUsers,
			authorizedUsers: allUsers,
		}),
		testenv.WithEmitter(emitter),
	)

	devicesService := env.DevicesService
	identityService := env.IdentityService
	ctx := context.Background()

	// userTrustedDevices has:
	// * 1 valid device (needed for a successful token)
	// * 1 unknown device in its User.TrustedDeviceIDs list.
	setupUserForDeviceWebAuthn(t, env, setupUserWebAuthnOpts{
		user: userTrustedDevices,
		devices: []*devicepb.Device{
			{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "llama-1",
			},
		},
	})
	const unknownDeviceID = "unknown-device-ID"
	if _, err := identityService.UpdateAndSwapUser(ctx, userTrustedDevices, false /* withSecrets */, func(u types.User) (changed bool, err error) {
		u.SetTrustedDeviceIDs(append(u.GetTrustedDeviceIDs(), unknownDeviceID))
		return true, nil
	}); err != nil {
		t.Fatalf("UpdateAndSwapUser failed: %v", err)
	}

	// userDevicesIndex has:
	// * 1 valid device (needed for a successful token)
	// * 1 unknown device in its /devices/by_user index.
	userDevicesIndexData := setupUserForDeviceWebAuthn(t, env, setupUserWebAuthnOpts{
		user: userDevicesIndex,
		devices: []*devicepb.Device{
			{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "alpaca-1",
			},
			{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "alpaca-2",
			},
		},
	})
	// A direct delete doesn't update the "/devices/by_user" index.
	// High-level operations are well behaved, so we must resort to direct backend
	// access.
	key := backend.NewKey("devices", "id", userDevicesIndexData.devices[1].dev.Id)
	be := identityService.Backend
	if err := be.Delete(ctx, key); err != nil {
		t.Fatalf("be.Delete(%q) failed: %v", key, err)
	}

	tests := []struct {
		name string
		user string
	}{
		{
			name: "User.TrustedDeviceIDs has unknown device",
			user: userTrustedDevices,
		},
		{
			name: "/devices/by_user index has unknown device",
			user: userDevicesIndex,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if _, err := devicesService.CreateDeviceWebToken(ctx, &devicepb.DeviceWebToken{
				Id:               unknownDeviceID,
				WebSessionId:     "mysessionid", // unimportant
				BrowserUserAgent: sampleUserAgentMacOS,
				BrowserIp:        sampleIP,
				User:             test.user,
			}); err != nil {
				t.Fatalf("CreateDeviceWebToken failed, want success: %v", err)
			}
		})
	}
}

func TestService_ConfirmDeviceWebAuthentication(t *testing.T) {
	const userLlama = "llama"
	const userProxy = "proxy"
	allUsers := []string{userLlama, userProxy}

	augmentWebFunc := &fakeAugmentWebFunc{}
	emitter := &keyedEmitter{}
	env := testenv.NewUsingT(t,
		testenv.WithAugmentWebFunc(augmentWebFunc.function),
		testenv.WithAuthorizer(&userAwareAuthorizer{
			knownUsers:      allUsers,
			authorizedUsers: allUsers,
			userToSystemRoles: map[string][]types.SystemRole{
				userProxy: []types.SystemRole{types.RoleProxy},
			},
		}),
		testenv.WithEmitter(emitter),
	)

	devicesClient := env.DevicesClient
	ctx := context.Background()

	userData := setupUserForDeviceWebAuthn(t, env, setupUserWebAuthnOpts{
		user: userLlama,
		devices: []*devicepb.Device{
			{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "llama-1",
			},
		},
	})

	makeConfirmParams := func() createConfirmationTokenParams {
		return createConfirmationTokenParams{
			device:       userData.device.dev,
			sim:          userData.device.sim,
			user:         userData.user,
			sourceIP:     sampleIP,
			webSessionID: uuid.NewString(),
		}
	}

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		params := makeConfirmParams()
		confirmToken := createConfirmationToken(t, env, params)

		outCtx := configureOutgoingContext(ctx, outgoingContextParams{
			User:       userProxy,
			EmitterKey: params.webSessionID,
		})

		if _, err := devicesClient.ConfirmDeviceWebAuthentication(outCtx, &devicepb.ConfirmDeviceWebAuthenticationRequest{
			ConfirmationToken:   confirmToken,
			CurrentWebSessionId: params.webSessionID,
		}); err != nil {
			t.Errorf("ConfirmDeviceWebAuthentication failed: %v", err)
		}

		// Verify that Auth was called to augment the session.
		if got := augmentWebFunc.getNumCalls(params.webSessionID); got != 1 {
			t.Errorf("ConfirmDeviceWebAuthentication: unexpected number of augmentWebFunc calls, got=%v, want 1", got)
		}

		// Verify audit events.
		assertEvents(t, emitter.Events(params.webSessionID), []wantEvent{
			{
				Type: events.DeviceAuthenticateConfirmEvent,
				Code: events.DeviceAuthenticateConfirmCode,
			},
		})
	})

	makeSuccessRequest := func(t *testing.T) *devicepb.ConfirmDeviceWebAuthenticationRequest {
		p := makeConfirmParams()
		confirmToken := createConfirmationToken(t, env, p)
		return &devicepb.ConfirmDeviceWebAuthenticationRequest{
			ConfirmationToken:   confirmToken,
			CurrentWebSessionId: p.webSessionID,
		}
	}

	// Failure scenarios.
	const invalidTokenError = "invalid device confirmation token"
	tests := []struct {
		name               string
		currentUser        string // defaults to userProxy
		failAugmentWebFunc bool
		makeRequest        func(*testing.T) *devicepb.ConfirmDeviceWebAuthenticationRequest
		assertErr          func(error) bool
		wantErr            string
		wantAuditErr       string // audit UserMessage string
		skipAudit          bool
	}{
		{
			name: "nil token",
			makeRequest: func(_ *testing.T) *devicepb.ConfirmDeviceWebAuthenticationRequest {
				return &devicepb.ConfirmDeviceWebAuthenticationRequest{
					CurrentWebSessionId: uuid.NewString(),
				}
			},
			assertErr: trace.IsBadParameter,
			wantErr:   "token required",
			skipAudit: true,
		},
		{
			name: "empty session ID",
			makeRequest: func(_ *testing.T) *devicepb.ConfirmDeviceWebAuthenticationRequest {
				return &devicepb.ConfirmDeviceWebAuthenticationRequest{
					ConfirmationToken: &devicepb.DeviceConfirmationToken{
						Id:    "real-looking-id",
						Token: "base64token",
					},
				}
			},
			assertErr: trace.IsBadParameter,
			wantErr:   "session ID required",
			skipAudit: true,
		},
		{
			name:        "non-proxy caller",
			currentUser: userLlama,
			makeRequest: func(t *testing.T) *devicepb.ConfirmDeviceWebAuthenticationRequest {
				return &devicepb.ConfirmDeviceWebAuthenticationRequest{
					ConfirmationToken: &devicepb.DeviceConfirmationToken{
						Id:    "real-looking-id-2",
						Token: "base64token2",
					},
					CurrentWebSessionId: uuid.NewString(),
				}
			},
			assertErr: trace.IsAccessDenied,
			wantErr:   "access denied",
			skipAudit: true,
		},
		{
			name: "invalid token",
			makeRequest: func(t *testing.T) *devicepb.ConfirmDeviceWebAuthenticationRequest {
				req := makeSuccessRequest(t)
				req.ConfirmationToken.Token += "invalid"
				return req
			},
			assertErr:    trace.IsAccessDenied,
			wantErr:      invalidTokenError,
			wantAuditErr: invalidTokenError, // This is the actual failure reason in this case
		},
		{
			name: "invalid session ID",
			makeRequest: func(t *testing.T) *devicepb.ConfirmDeviceWebAuthenticationRequest {
				req := makeSuccessRequest(t)
				req.CurrentWebSessionId = "bad-session-id"
				return req
			},
			assertErr:    trace.IsAccessDenied,
			wantErr:      invalidTokenError,
			wantAuditErr: "token move",
		},
		{
			name:               "fails to issue certificates",
			failAugmentWebFunc: true,
			makeRequest:        makeSuccessRequest,
			wantErr:            errFakeAugmentWebFuncFailed.Error(),
			wantAuditErr:       "failed to issue",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Prepare outCtx user.
			user := userProxy
			if test.currentUser != "" {
				user = test.currentUser
			}

			req := test.makeRequest(t)

			// Use an unique emitter key for every test.
			emitterKey := req.CurrentWebSessionId
			if emitterKey == "" {
				emitterKey = uuid.NewString()
			}

			outCtx := configureOutgoingContext(ctx, outgoingContextParams{
				User:       user,
				EmitterKey: emitterKey,
			})

			if test.failAugmentWebFunc {
				augmentWebFunc.setFailNext(emitterKey)
			}

			_, err := devicesClient.ConfirmDeviceWebAuthentication(outCtx, req)
			if err == nil {
				t.Fatal("ConfirmDeviceWebAuthentication returned err=nil, want non-nil")
			}
			if test.assertErr != nil && !test.assertErr(err) {
				t.Errorf("ConfirmDeviceWebAuthentication: assertErr failed, err=%v (%T)", err, trace.Unwrap(err))
			}
			if test.wantErr != "" {
				assert.ErrorContains(t, err, test.wantErr, "ConfirmDeviceWebAuthentication error mismatch")
			}

			// Verify certs not augmented.
			wantCalls := 0
			if test.failAugmentWebFunc {
				wantCalls++
			}
			if got := augmentWebFunc.getNumCalls(emitterKey); got != wantCalls {
				t.Errorf("ConfirmDeviceWebAuthentication: unexpected number of augmentWebFunc calls, got=%v, want %v", got, wantCalls)
			}

			if test.skipAudit {
				return
			}

			// Verify audit.
			allEvents := emitter.Events(emitterKey)
			assertEvents(t, allEvents, []wantEvent{
				{
					Type:     events.DeviceAuthenticateConfirmEvent,
					Code:     events.DeviceAuthenticateConfirmCode,
					WantFail: true,
				},
			})

			var auditUserMessage string
			if len(allEvents) > 0 {
				if event, ok := allEvents[0].(*apievents.DeviceEvent2); ok {
					auditUserMessage = event.Status.UserMessage
				}
			}
			if !strings.Contains(auditUserMessage, test.wantAuditErr) {
				t.Errorf("ConfirmDeviceWebAuthentication: event.Status.UserMessage=%q, want %q", auditUserMessage, test.wantAuditErr)
			}
		})
	}
}

type createConfirmationTokenParams struct {
	device       *devicepb.Device
	sim          simulator
	user         string
	sourceIP     string
	webSessionID string
}

func createConfirmationToken(t *testing.T, env *testenv.E, p createConfirmationTokenParams) *devicepb.DeviceConfirmationToken {
	ctx := context.Background()

	service := env.DevicesService
	webToken, err := service.CreateDeviceWebToken(ctx, &devicepb.DeviceWebToken{
		WebSessionId:     p.webSessionID,
		BrowserUserAgent: sampleUserAgentMacOS,
		BrowserIp:        p.sourceIP,
		User:             p.user,
		ExpectedDeviceIds: []string{
			p.device.Id,
		},
	})
	if err != nil {
		t.Fatalf("CreateDeviceWebToken failed: %v", err)
	}

	outCtx := configureOutgoingContext(ctx, outgoingContextParams{
		User:     p.user,
		SourceIP: p.sourceIP,
	})

	devicesClient := env.DevicesClient
	confirmToken, err := authenticateDeviceWeb(outCtx, devicesClient, p.device, p.sim, webToken)
	if err != nil {
		t.Fatalf("AuthenticateDevice failed: %v", err)
	}

	return confirmToken
}
