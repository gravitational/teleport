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
	"github.com/gravitational/oxy/ratelimit"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
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
	prehogv1alpha "github.com/gravitational/teleport/gen/proto/go/prehog/v1alpha"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
)

func TestService_authz(t *testing.T) {
	authorizer := &fakeAuthorizer{}
	env := testenv.NewUsingT(t, testenv.WithAuthorizer(authorizer))

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

	// Safe because NewUsingT sets a modules.TestModule.
	m := modules.GetModules().(*modules.TestModules)
	m.TestFeatures.DeviceTrust.Enabled = false

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
		User:    user,
		Checker: a.Checker,
	}, nil
}

type wantRuleVerb struct {
	rule, verb string
}

type ruleVerifyingChecker struct {
	testenv.NoopChecker
	want []wantRuleVerb
}

func (c *ruleVerifyingChecker) CheckAccessToRule(ruleCtx services.RuleContext, namespace string, rule string, verb string, silent bool) error {
	if namespace != defaults.Namespace {
		return fmt.Errorf("unexpected namespace: %v", namespace)
	}

	for i, want := range c.want {
		if want.rule == rule && want.verb == verb {
			c.want = append(c.want[:i], c.want[i+1:]...) // cut
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

func (l *alternatingLimiter) RegisterRequest(token string, customRate *ratelimit.RateSet) error {
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
	withUser := func(ctx context.Context, user string) context.Context {
		return metadata.AppendToOutgoingContext(ctx, authorizerUserKey, user)
	}

	// Register a device for testing.
	dev, err := devices.CreateDevice(withUser(ctx, adminUser), &devicepb.CreateDeviceRequest{
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

				token, err := devices.CreateDeviceEnrollToken(withUser(ctx, test.user), test.req)
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
			assertErr: trace.IsAccessDenied, // redacted
		},
		{
			name:      "user empty request fails",
			user:      endUser,
			req:       &devicepb.CreateDeviceEnrollTokenRequest{},
			assertErr: trace.IsAccessDenied, // redacted
		},
		{
			name: "user invalid cd fails",
			user: endUser,
			req: &devicepb.CreateDeviceEnrollTokenRequest{
				DeviceData: cdBad,
			},
			assertErr: trace.IsAccessDenied, // redacted
		},
		{
			name: "user unknown device fails",
			user: endUser,
			req: &devicepb.CreateDeviceEnrollTokenRequest{
				DeviceData: cdUnknown,
			},
			assertErr: trace.IsAccessDenied, // redacted
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

// authorizerUserKey is used by [userAwareAuthorizer].
const authorizerUserKey = "user"

// userAwareAuthorizer allows access based on the context user. See [metadata]
// and [authorizerUserKey]
// Used by CreateDeviceEnrollToken/auto-enroll tests.
type userAwareAuthorizer struct {
	services.AccessChecker // double as an AccessChecker.

	knownUsers      []string
	authorizedUsers []string
}

func (a *userAwareAuthorizer) Authorize(ctx context.Context) (*authz.Context, error) {
	// Fetch the user from the "user" metadata key.
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, errors.New("ctx lacks metadata")
	}
	users := md.Get(authorizerUserKey)
	if len(users) == 0 || len(users[0]) == 0 {
		return nil, errors.New("ctx lacks user")
	}
	username := users[0]

	// Fail Authorize for unknown users.
	found := false
	for _, known := range a.knownUsers {
		if username == known {
			found = true
			break
		}
	}
	if !found {
		return nil, trace.AccessDenied("unknown user")
	}

	// Proceed.
	user, err := types.NewUser(username)
	if err != nil {
		return nil, fmt.Errorf("creating user: %v", err)
	}
	return &authz.Context{
		User:    user,
		Checker: a,
	}, nil
}

func (a *userAwareAuthorizer) CheckAccessToRule(ruleCtx services.RuleContext, namespace, rule, verb string, silent bool) error {
	user, err := ruleCtx.GetIdentifier([]string{"user", "metadata", "name"})
	if err != nil {
		return err
	}

	for _, authz := range a.authorizedUsers {
		if user == authz {
			return nil
		}
	}
	return trace.AccessDenied("access denied")
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
	env := testenv.NewUsingT(t)

	devices := env.DevicesClient
	service := env.DevicesService
	ctx := context.Background()

	// Safe because of NewUsingT.
	m := modules.GetModules().(*modules.TestModules)

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
				f.DeviceTrust.DevicesUsageLimit = 5
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
	var emittedEvents []usagereporter.Anonymizable
	fakeAnonymizeAndSubmit := func(events ...usagereporter.Anonymizable) {
		emittedEvents = append(emittedEvents, events...)
	}
	env := testenv.NewUsingT(
		t,
		testenv.WithAnonymizeAndSubmitFunc(fakeAnonymizeAndSubmit),
	)
	devicesClient := env.DevicesClient
	ctx := context.Background()

	// Safe because of NewUsingT.
	m := modules.GetModules().(*modules.TestModules)
	m.TestFeatures.IsUsageBasedBilling = true
	m.TestFeatures.DeviceTrust.DevicesUsageLimit = 1

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
