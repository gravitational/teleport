package devicetrustv1_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/defaults"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	dtent "github.com/gravitational/teleport/e/lib/devicetrust"
	"github.com/gravitational/teleport/e/lib/devicetrust/testenv"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/services"
)

func TestService_authz(t *testing.T) {
	authorizer := &fakeAuthorizer{}
	env := testenv.MustNew(testenv.WithAuthorizer(authorizer))
	defer env.Close()

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
				if err := stream.Send(&devicepb.EnrollDeviceRequest{}); err != nil {
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

func TestService_CreateDevice(t *testing.T) {
	emitter := &eventstest.MockEmitter{}
	env := testenv.MustNew(testenv.WithEmitter(emitter))
	defer env.Close()
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
					Type: events.DeviceEvent,
					Code: events.DeviceCreateCode,
				},
			}
			if test.req.CreateEnrollToken {
				wantEvents = append(wantEvents, wantEvent{
					Type: events.DeviceEvent,
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

	// Authenticate a few times to generate additional collected data.
	if err := authenticateDevice(ctx, devices, llamaDev, llamaKey); err != nil {
		t.Fatalf("authenticateDevice failed: %v", err)
	}
	if err := authenticateDevice(ctx, devices, llamaDev, llamaKey); err != nil {
		t.Fatalf("authenticateDevice failed: %v", err)
	}
	if err := authenticateDevice(ctx, devices, alpacaDev, alpacaKey); err != nil {
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

		sort.Slice(want, func(i, j int) bool { return want[i].AssetTag < want[j].AssetTag })
		sort.Slice(got, func(i, j int) bool { return got[i].AssetTag < got[j].AssetTag })
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

		for _, dev := range allDevices {
			created, err := devices.CreateDevice(ctx, &devicepb.CreateDeviceRequest{
				Device:           dev,
				CreateAsResource: true,
			})
			if err != nil {
				t.Fatalf("CreateDevice(%q, createAsResource=true) failed: %v", dev.AssetTag, err)
			}

			// Assert CreatteDevice's response.
			if diff := cmp.Diff(dev, created, protocmp.Transform()); diff != "" {
				t.Errorf("CreateDevice mismatch (-want +got):\n%s", diff)
			}
		}

		// Assert storage.
		assertDevices(t, allDevices)
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

		// Assert storage.
		assertDevices(t, allDevices)
	})
}

func authenticateDevice(ctx context.Context, devices devicepb.DeviceTrustServiceClient, dev *devicepb.Device, devKey *fakeEnclaveKey) error {
	stream, err := devices.AuthenticateDevice(ctx)
	if err != nil {
		return err
	}

	// 1. Init.
	if err := stream.Send(&devicepb.AuthenticateDeviceRequest{
		Payload: &devicepb.AuthenticateDeviceRequest_Init{
			Init: &devicepb.AuthenticateDeviceInit{
				CredentialId: devKey.id,
				DeviceData: &devicepb.DeviceCollectedData{
					CollectTime:  timestamppb.Now(),
					OsType:       dev.OsType,
					SerialNumber: dev.AssetTag,
				},
			},
		},
	}); err != nil {
		return err
	}
	resp, err := stream.Recv()
	if err != nil {
		return err
	}

	// 2. Challenge.
	sig, err := devKey.signChallenge(resp.GetChallenge().Challenge)
	if err != nil {
		return err
	}
	if err := stream.Send(&devicepb.AuthenticateDeviceRequest{
		Payload: &devicepb.AuthenticateDeviceRequest_ChallengeResponse{
			ChallengeResponse: &devicepb.AuthenticateDeviceChallengeResponse{
				Signature: sig,
			},
		},
	}); err != nil {
		return err
	}
	resp, err = stream.Recv()
	if err != nil {
		return err
	}

	// 3. Success.
	if resp.GetUserCertificates() == nil {
		return fmt.Errorf("got payload type %T, wanted UserCertificates", resp.Payload)
	}
	return nil
}

func TestService_UpdateDevice(t *testing.T) {
	emitter := &eventstest.MockEmitter{}
	env := testenv.MustNew(testenv.WithEmitter(emitter))
	defer env.Close()

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
		name             string
		mdmFeatureActive bool
		req              *devicepb.UpdateDeviceRequest
		assertDev        func(t *testing.T, updated *devicepb.Device)
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
				want.Credential = nil
				if diff := cmp.Diff(want, updated, protocmp.Transform()); diff != "" {
					t.Errorf("UpdateDevice mismatch (-want +got)\n%s", diff)
				}
			},
		},
		{
			name:             "source and profile",
			mdmFeatureActive: true,
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
			setMDMFeatureActive(t, test.mdmFeatureActive)
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
					Type: events.DeviceEvent,
					Code: events.DeviceUpdateCode,
				},
			})
		})
	}
}

func TestService_UpdateDevice_errors(t *testing.T) {
	env := testenv.MustNew()
	defer env.Close()

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
	emitter := &eventstest.MockEmitter{}
	env := testenv.MustNew(testenv.WithEmitter(emitter))
	defer env.Close()

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
				{Type: events.DeviceEvent, Code: events.DeviceCreateCode},
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
				{Type: events.DeviceEvent, Code: events.DeviceCreateCode},
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
				if diff := cmp.Diff(want, upserted, protocmp.Transform()); diff != "" {
					t.Errorf("UpsertDevice mismatch (-want +got)\n%s", diff)
				}
			},
			wantEvents: []wantEvent{
				{Type: events.DeviceEvent, Code: events.DeviceUpdateCode},
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

func TestService_UpsertDevice_errors(t *testing.T) {
	env := testenv.MustNew()
	defer env.Close()

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
	emitter := &eventstest.MockEmitter{}
	env := testenv.MustNew(testenv.WithEmitter(emitter))
	defer env.Close()

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
					Type: events.DeviceEvent,
					Code: events.DeviceDeleteCode,
				},
			})
		})
	}
}

func TestService_ListDevices(t *testing.T) {
	env := testenv.MustNew()
	defer env.Close()

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
			sort.Slice(want, func(i, j int) bool { return want[i].Id < want[j].Id })
			sort.Slice(got, func(i, j int) bool { return got[i].Id < got[j].Id })
			if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
				t.Errorf("ListDevices mismatch (-want +got)\n%s", diff)
			}
		})
	}
}

func TestService_FindDevices(t *testing.T) {
	env := testenv.MustNew()
	defer env.Close()

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
			sort.Slice(got, func(i, j int) bool { return got[i].Id < got[j].Id })
			sort.Slice(want, func(i, j int) bool { return want[i].Id < want[j].Id })
			if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
				t.Errorf("FindDevices mismatch (-want +got)\n%s", diff)
			}
		})
	}
}

func TestService_BulkCreateDevices(t *testing.T) {
	emitter := &eventstest.MockEmitter{}
	env := testenv.MustNew(testenv.WithEmitter(emitter))
	defer env.Close()

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
			Type: events.DeviceEvent,
			Code: events.DeviceCreateCode,
		},
		// alpaca
		{
			Type: events.DeviceEvent,
			Code: events.DeviceCreateCode,
		},
		// camel
		{
			Type: events.DeviceEvent,
			Code: events.DeviceCreateCode,
		},
	}
	assertEvents(t, emitter.Events(), wantEvents)
}

func TestService_CreateDeviceEnrollToken(t *testing.T) {
	emitter := &eventstest.MockEmitter{}
	env := testenv.MustNew(testenv.WithEmitter(emitter))
	defer env.Close()

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
					Type: events.DeviceEvent,
					Code: events.DeviceEnrollTokenCreateCode,
				},
			})
		})
	}
}

func setMDMFeatureActive(t *testing.T, active bool) {
	prev := dtent.MDMFeatureActive
	t.Cleanup(func() { dtent.MDMFeatureActive = prev })
	dtent.MDMFeatureActive = active
}

type wantEvent struct {
	Type, Code string
	WantFail   bool
}

type mockEmitter interface {
	Events() []apievents.AuditEvent
}

// assertEventsEmitter waits for the emitter to have at least the number of
// wanted events, then calls assertEvents.
func assertEventsEmitter(t *testing.T, emitter mockEmitter, want []wantEvent) {
	t.Helper()
	assert.Eventually(
		t,
		func() bool { return len(emitter.Events()) >= len(want) },
		2*time.Second,
		10*time.Millisecond,
		"Timed out waiting for emitter events")
	assertEvents(t, emitter.Events(), want)
}

func assertEvents(t *testing.T, got []apievents.AuditEvent, want []wantEvent) {
	t.Helper()

	if len(got) != len(want) {
		t.Errorf("Audit: found an unexpected number events: got %v, want %v", len(got), len(want))
		return
	}

	for i, g := range got {
		w := want[i]
		if g.GetType() != w.Type {
			t.Errorf("Audit: event mismatch: got[%v].Type = %v, want %v", i, g.GetType(), w.Type)
		}
		if g.GetCode() != w.Code {
			t.Errorf("Audit: event mismatch: got[%v].Code = %v, want %v", i, g.GetCode(), w.Code)
		}
		if g.GetType() != events.DeviceEvent {
			continue
		}

		devEvent, ok := g.(*apievents.DeviceEvent)
		switch {
		case !ok:
			t.Errorf("Audit: event mismatch: got[%v] is not a DeviceEvent: %T", i, devEvent)
		case devEvent.Status == nil:
			t.Errorf("Audit: event mismatch: got[%v].Status is nil, want non-nil", i)
		case devEvent.Status.Success == w.WantFail:
			t.Errorf("Audit: event mismatch: got[%v].Status.Success = %v, want %v", i, devEvent.Status.Success, !w.WantFail)
		}
	}
}
