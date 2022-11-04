package devicetrustv1_test

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/gravitational/teleport/api/defaults"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/e/lib/devicetrust/testenv"
	"github.com/gravitational/teleport/lib/auth"
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
		checker   *fakeChecker
		rpc       func() error
		assertErr func(error) bool
	}{
		{
			name: "CreateDevice",
			checker: &fakeChecker{
				wantRule: types.KindDevice,
				wantVerb: types.VerbCreate,
			},
			rpc: func() error {
				_, err := devices.CreateDevice(ctx, &devicepb.CreateDeviceRequest{})
				return err
			},
			assertErr: trace.IsBadParameter,
		},
		{
			name: "DeleteDevice",
			checker: &fakeChecker{
				wantRule: types.KindDevice,
				wantVerb: types.VerbDelete,
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
			name: "FindDevices",
			checker: &fakeChecker{
				wantRule: types.KindDevice,
				wantVerb: types.VerbList,
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
			checker: &fakeChecker{
				wantRule: types.KindDevice,
				wantVerb: types.VerbRead,
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
			checker: &fakeChecker{
				wantRule: types.KindDevice,
				wantVerb: types.VerbList,
			},
			rpc: func() error {
				_, err := devices.ListDevices(ctx, &devicepb.ListDevicesRequest{})
				return err
			},
			assertErr: func(err error) bool { return err == nil },
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			authorizer.Checker = test.checker
			authorizer.authorizeCount = 0

			// Any "blessed" error OK, we expect the RPCs to fail after authorization.
			// It's simpler to test this way.
			if err := test.rpc(); !test.assertErr(err) {
				t.Fatalf("RPC assertErr failed, err=%v", err)
			}

			if got, want := authorizer.authorizeCount, 1; got != want {
				t.Errorf("Authorize count mismatch: got=%v, want=%v", got, want)
			}
			if got, want := test.checker.checkAccessToRuleCount, 1; got != want {
				t.Errorf("CheckAccessToRule count mismatch: got=%v, want=%v", got, want)
			}
		})
	}
}

type fakeAuthorizer struct {
	authorizeCount int
	Checker        services.AccessChecker
}

func (a *fakeAuthorizer) Authorize(ctx context.Context) (*auth.Context, error) {
	a.authorizeCount++

	user, err := types.NewUser("llama")
	if err != nil {
		return nil, err
	}

	return &auth.Context{
		User:    user,
		Checker: a.Checker,
	}, nil
}

type fakeChecker struct {
	services.AccessChecker

	checkAccessToRuleCount int
	wantRule, wantVerb     string
}

func (c *fakeChecker) CheckAccessToRule(ruleCtx services.RuleContext, namespace string, rule string, verb string, silent bool) error {
	c.checkAccessToRuleCount++
	switch {
	case namespace != defaults.Namespace:
		return fmt.Errorf("unexpected namespace: %v", namespace)
	case rule != c.wantRule:
		return fmt.Errorf("unexpected rule=%q, want %q", rule, c.wantRule)
	case verb != c.wantVerb:
		return fmt.Errorf("unexpected verb=%q, want %q", verb, c.wantVerb)
	}
	return nil
}

func TestService_CreateDevice(t *testing.T) {
	emitter := &eventstest.MockEmitter{}
	env := testenv.MustNew(testenv.WithEmitter(emitter))
	defer env.Close()
	devices := env.DevicesClient

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
	var allDevices []*devicepb.Device
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
		allDevices = append(allDevices, dev)
	}

	tests := []struct {
		name        string
		initialReq  *devicepb.ListDevicesRequest
		wantDevices []*devicepb.Device
	}{
		{
			name:        "ok",
			initialReq:  &devicepb.ListDevicesRequest{}, // default parameters
			wantDevices: allDevices,
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

type wantEvent struct {
	Type, Code string
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
	}
}
