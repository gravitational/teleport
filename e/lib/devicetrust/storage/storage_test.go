package storage_test

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/e/lib/devicetrust/storage"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
)

func TestS_CreateDevice(t *testing.T) {
	env := mustNewEnv()
	defer env.Close()
	s := env.S

	ctx := context.Background()

	tests := []struct {
		name string
		dev  *devicepb.Device
	}{
		{
			name: "ok",
			dev: &devicepb.Device{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "A00AA0AAAA0A",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := s.CreateDevice(ctx, test.dev)
			if err != nil {
				t.Fatalf("CreateDevice failed: %v", err)
			}

			// Verify returned device.
			if got.Id == "" {
				t.Fatal("CreateDevice: Id empty")
			}
			if got.ApiVersion == "" {
				t.Error("CreateDevice: ApiVersion empty")
			}
			if got.CreateTime == nil {
				t.Error("CreateDevice: CreateTime nil")
			}
			if got.UpdateTime == nil {
				t.Error("CreateDevice: UpdateTime nil")
			}
			if got.CreateTime != nil && got.UpdateTime != nil && got.CreateTime.AsTime().After(got.UpdateTime.AsTime()) {
				t.Errorf("CreateDevice: CreateTime (%v) is after UpdateTime (%v)", got.CreateTime, got.UpdateTime)
			}
			want := &devicepb.Device{
				ApiVersion:   got.ApiVersion,
				Id:           got.Id,
				OsType:       test.dev.OsType,
				AssetTag:     test.dev.AssetTag,
				CreateTime:   got.CreateTime,
				UpdateTime:   got.UpdateTime,
				EnrollStatus: got.EnrollStatus,
			}
			if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
				t.Errorf("CreateDevice: mismatch (-want +got):\n%s", diff)
			}

			// Verify that device is present is storage.
			stored, err := s.GetDeviceByID(ctx, got.Id)
			if err != nil {
				t.Fatalf("GetDeviceByID failed: %v", err)
			}
			if diff := cmp.Diff(got, stored, protocmp.Transform()); diff != "" {
				t.Errorf("GetDeviceByID: mismatch (-want +got):\n%s", diff)
			}

			// Verify that the asset tag index is up-to-date.
			gotDevs, err := s.GetDevicesByAssetTag(ctx, got.AssetTag)
			if err != nil {
				t.Fatalf("GetDevicesByAssetTag failed: %v", err)
			}
			// Assume no duplicates here for simplicity.
			// Complex scenarios exercised in other tests.
			wantDevs := []*devicepb.Device{got}
			if diff := cmp.Diff(wantDevs, gotDevs, protocmp.Transform()); diff != "" {
				t.Errorf("GetDevicesByAssetTag: mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestS_CreateDevice_reusedAssetTags(t *testing.T) {
	env := mustNewEnv()
	defer env.Close()
	s := env.S

	devMac := &devicepb.Device{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "llama",
	}
	devLinux := proto.Clone(devMac).(*devicepb.Device)
	devLinux.OsType = devicepb.OSType_OS_TYPE_LINUX
	devWin := proto.Clone(devMac).(*devicepb.Device)
	devWin.OsType = devicepb.OSType_OS_TYPE_WINDOWS

	// Add devices above to storage.
	// All devices have the same asset tag but distinct OSes.
	ctx := context.Background()
	for _, dev := range []**devicepb.Device{&devMac, &devLinux, &devWin} {
		created, err := s.CreateDevice(ctx, *dev)
		if err != nil {
			t.Fatalf("CreateDevice failed: %v", err)
		}
		*dev = created
	}

	// Verify that index reads are consistent.
	got, err := s.GetDevicesByAssetTag(ctx, devMac.AssetTag)
	if err != nil {
		t.Fatalf("GetDevicesByAssetTag failed: %v", err)
	}

	// want matches the elements in got. Both slices are sorted as the order
	// doesn't matter / isn't guaranteed.
	want := []*devicepb.Device{devMac, devLinux, devWin}
	sort.Slice(got, func(i, j int) bool { return got[i].Id < got[j].Id })
	sort.Slice(want, func(i, j int) bool { return want[i].Id < want[j].Id })

	if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
		t.Errorf("GetDevicesByAssetTag: mismatch (-want +got):\n%s", diff)
	}
}

func TestS_CreateDevice_errors(t *testing.T) {
	env := mustNewEnv()
	defer env.Close()
	s := env.S

	// Prepare and register a valid device to serve as the basis for tests.
	const knownTag = "llama"
	validDev := &devicepb.Device{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: knownTag,
	}
	ctx := context.Background()
	if _, err := s.CreateDevice(ctx, validDev); err != nil {
		t.Fatalf("CreateDevice failed: %v", err)
	}

	// Switch tags so the device would be perfectly valid for creation.
	const otherTag = "alpaca"
	validDev.AssetTag = otherTag

	tests := []struct {
		name      string
		createDev func() *devicepb.Device
		wantErr   string
		assertErr func(error) bool
	}{
		{
			name:      "device nil",
			createDev: func() *devicepb.Device { return nil },
			wantErr:   "device required",
			assertErr: trace.IsBadParameter,
		},
		{
			name: "os_type unspecified",
			createDev: func() *devicepb.Device {
				d := proto.Clone(validDev).(*devicepb.Device)
				d.OsType = devicepb.OSType_OS_TYPE_UNSPECIFIED
				return d
			},
			wantErr:   "os_type",
			assertErr: trace.IsBadParameter,
		},
		{
			name: "asset_tag empty",
			createDev: func() *devicepb.Device {
				d := proto.Clone(validDev).(*devicepb.Device)
				d.AssetTag = ""
				return d
			},
			wantErr:   "asset_tag",
			assertErr: trace.IsBadParameter,
		},
		{
			name: "asset_tag registered",
			createDev: func() *devicepb.Device {
				d := proto.Clone(validDev).(*devicepb.Device)
				d.AssetTag = knownTag
				return d
			},
			wantErr:   "already registered",
			assertErr: trace.IsAlreadyExists,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := s.CreateDevice(ctx, test.createDev())
			assert.ErrorContains(t, err, test.wantErr, "CreateDevice error mismatch")
			if !test.assertErr(err) {
				t.Errorf("CreateDevice: assertErr failed: %#v", err)
			}
		})
	}
}

func TestS_DeleteDevice(t *testing.T) {
	env := mustNewEnv()
	defer env.Close()

	s := env.S
	ctx := context.Background()

	// Register a few devices for us to test with.
	var devs []*devicepb.Device
	for _, assetTag := range []string{"llama", "alpaca", "camel"} {
		dev, err := s.CreateDevice(ctx, &devicepb.Device{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: assetTag,
		})
		if err != nil {
			t.Fatalf("CreateDevice failed: %v", err)
		}
		devs = append(devs, dev)
	}
	d1 := devs[0] // deleted by tests
	d2 := devs[1] // not deleted
	d3 := devs[2] // not deleted

	tests := []struct {
		name      string
		deviceID  string
		assertErr func(err error) bool
	}{
		{
			name:      "ok",
			deviceID:  d1.Id,
			assertErr: func(err error) bool { return err == nil },
		},
		{
			name:      "already deleted device fails with not found",
			deviceID:  d1.Id,
			assertErr: trace.IsNotFound,
		},
		{
			name:      "unknown device fails with not found",
			deviceID:  "unknown",
			assertErr: trace.IsNotFound,
		},
		{
			name:      "empty device ID fails",
			deviceID:  "",
			assertErr: trace.IsBadParameter,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := s.DeleteDevice(ctx, test.deviceID)
			if !test.assertErr(err) {
				t.Errorf("DeleteDevice asserErr failed, err=%v", err)
			}
			if err != nil {
				return
			}

			// Deleted device should really be deleted.
			if _, err := s.GetDeviceByID(ctx, test.deviceID); !trace.IsNotFound(err) {
				t.Errorf("GetDeviceByID returned an unexpected error: %v", err)
			}
		})
	}

	// Unrelated devices should still exist.
	t.Run("unrelated", func(t *testing.T) {
		got, _, err := s.ListDevices(ctx, 3 /* pageSize */, "" /* pageToken */)
		if err != nil {
			t.Fatalf("ListDevices failed: %v", err)
		}

		want := []*devicepb.Device{d2, d3}
		sort.Slice(got, func(i, j int) bool { return got[i].Id < got[j].Id })
		sort.Slice(want, func(i, j int) bool { return want[i].Id < want[j].Id })
		if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
			t.Errorf("ListDevices mismatch (-want +got)\n%s", diff)
		}
	})
}

func TestS_GetDeviceByID_errors(t *testing.T) {
	env := mustNewEnv()
	defer env.Close()

	s := env.S
	ctx := context.Background()

	tests := []struct {
		name      string
		deviceID  string
		assertErr func(err error) bool
	}{
		{
			name:      "device ID required",
			deviceID:  "",
			assertErr: trace.IsBadParameter,
		},
		{
			name:      "unknown device not found",
			deviceID:  "unknown",
			assertErr: trace.IsNotFound,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := s.GetDeviceByID(ctx, test.deviceID)
			if !test.assertErr(err) {
				t.Errorf("GetDeviceByID assertErr failed, err=%v", err)
			}
		})
	}
}

func TestS_GetDevicesByAssetTag_errors(t *testing.T) {
	env := mustNewEnv()
	defer env.Close()

	s := env.S
	ctx := context.Background()

	tests := []struct {
		name      string
		assetTag  string
		assertErr func(err error) bool
	}{
		{
			name:      "asset tag required",
			assetTag:  "",
			assertErr: trace.IsBadParameter,
		},
		{
			name:      "unknown tag returns empty",
			assetTag:  "unknown",
			assertErr: func(err error) bool { return err == nil },
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := s.GetDevicesByAssetTag(ctx, test.assetTag)
			if !test.assertErr(err) {
				t.Errorf("GetDevicesByAssetTag assertErr failed, err=%v", err)
			}
		})
	}
}

func TestS_ListDevices(t *testing.T) {
	env := mustNewEnv()
	defer env.Close()

	s := env.S
	ctx := context.Background()

	const defaultPageSize = 0
	const defaultPageToken = ""

	t.Run("empty", func(t *testing.T) {
		devs, nextPageToken, err := s.ListDevices(ctx, defaultPageSize, defaultPageToken)
		if err != nil {
			t.Fatalf("ListDevices failed: %v", err)
		}
		if len(devs) > 0 {
			t.Errorf("ListDevices returned unexpected devices: %v", devs)
		}
		if nextPageToken != "" {
			t.Errorf("ListDevices returned an unexpected nextPageToken: %q", nextPageToken)
		}
	})

	// Add a few devices to query.
	var allDevs []*devicepb.Device
	for _, assetTag := range []string{"llama", "alpaca", "camel", "horse", "duck"} {
		dev, err := s.CreateDevice(ctx, &devicepb.Device{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: assetTag,
		})
		if err != nil {
			t.Fatalf("CreateDevice(%q) failed: %v", assetTag, err)
		}
		allDevs = append(allDevs, dev)
	}

	tests := []struct {
		name             string
		pageSize         int
		wantTrimmedPages bool
	}{
		{
			name: "default page size",
		},
		{
			name:     "page size 1",
			pageSize: 1,
		},
		{
			name:     "page size odd",
			pageSize: 2,
		},
		{
			name:     "page size even",
			pageSize: 3,
		},
		{
			name:     "empty last page",
			pageSize: len(allDevs),
		},
		{
			name:     "all results in first page",
			pageSize: len(allDevs) + 1,
		},
		{
			name:             "large page sizes get trimmed",
			pageSize:         100_000,
			wantTrimmedPages: true,
		},
		{
			name:     "negative page size ignored",
			pageSize: -1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// We expect to eventually get all devices in all scenarios, so we'll
			// collect them here and later compare against the complete list.
			var pageToken string
			var got []*devicepb.Device
			for {
				devs, nextPageToken, err := s.ListDevices(ctx, test.pageSize, pageToken)
				if err != nil {
					t.Fatalf("ListDevices failed: %v", err)
				}
				if test.wantTrimmedPages && len(devs) >= test.pageSize {
					t.Errorf("ListDevices returned non-trimmed page: got %v, want <= %v", len(devs), test.pageSize)
				}
				got = append(got, devs...)

				if nextPageToken == "" {
					break
				}
				pageToken = nextPageToken
			}

			want := allDevs
			sort.Slice(got, func(i, j int) bool { return got[i].Id < got[j].Id })
			sort.Slice(want, func(i, j int) bool { return want[i].Id < want[j].Id })

			if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
				t.Errorf("ListDevices mismatch (-want +got)\n%s", diff)
			}
		})
	}
}

func TestS_ListDevices_errors(t *testing.T) {
	env := mustNewEnv()
	defer env.Close()

	s := env.S
	ctx := context.Background()

	if _, _, err := s.ListDevices(ctx, 0 /* pageSize */, "badpagetoken" /* pageToken */); !trace.IsBadParameter(err) {
		t.Errorf("ListDevices returned an unexpected error: %v", err)
	}
}

func TestS_CreateDeviceEnrollToken_createAndSpend(t *testing.T) {
	env := mustNewEnv()
	defer env.Close()
	clock := env.Clock
	s := env.S

	// Create a couple devices for the test.
	ctx := context.Background()
	var devs []*devicepb.Device
	for _, asset := range []string{"llama", "alpaca"} {
		dev, err := s.CreateDevice(ctx, &devicepb.Device{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: asset,
		})
		if err != nil {
			t.Fatalf("CreateDevice failed: %v", err)
		}
		devs = append(devs, dev)
	}
	dev1 := devs[0]
	dev2 := devs[1]
	deviceID := dev1.Id

	tests := []struct {
		name            string
		deviceID        string
		createToken     func(ctx context.Context, deviceID string) (*devicepb.DeviceEnrollToken, error)
		spendToken      func(ctx context.Context, deviceID, token string) error
		assertCreateErr func(err error) bool
		assertSpendErr  func(err error) bool
	}{
		{
			name:        "create and spend token",
			deviceID:    deviceID,
			createToken: s.CreateDeviceEnrollToken,
			spendToken:  s.SpendDeviceEnrollToken,
		},
		{
			name:     "replace token",
			deviceID: deviceID,
			createToken: func(ctx context.Context, deviceID string) (*devicepb.DeviceEnrollToken, error) {
				first, err := s.CreateDeviceEnrollToken(ctx, deviceID)
				if err != nil {
					return nil, err
				}

				// Immediately replace initial token.
				second, err := s.CreateDeviceEnrollToken(ctx, deviceID)
				if err != nil {
					return nil, err
				}
				// Sanity check that tokens are different.
				if first.Token == second.Token {
					return nil, errors.New("first and second tokens are equal")
				}

				// First token cannot be spent anymore.
				if err := s.SpendDeviceEnrollToken(ctx, deviceID, first.Token); !trace.IsBadParameter(err) {
					// Original error type erased on purpose (%v instead of %w)
					return nil, fmt.Errorf("unexpected error attempting to spend first token: %v", err)
				}

				return second, nil
			},
			spendToken: s.SpendDeviceEnrollToken,
		},
		{
			name:     "token expires",
			deviceID: deviceID,
			createToken: func(ctx context.Context, deviceID string) (*devicepb.DeviceEnrollToken, error) {
				token, err := s.CreateDeviceEnrollToken(ctx, deviceID)
				if err != nil {
					return nil, err
				}

				// Fast-forward to after the token is expired.
				clock.Advance(storage.DeviceEnrollTokenExpireDuration + 1)

				return token, nil
			},
			spendToken:     s.SpendDeviceEnrollToken,
			assertSpendErr: trace.IsNotFound,
		},
		{
			name:        "token is tied to device",
			deviceID:    deviceID,
			createToken: s.CreateDeviceEnrollToken,
			spendToken: func(ctx context.Context, _, token string) error {
				return s.SpendDeviceEnrollToken(ctx, dev2.Id /* wrong device */, token)
			},
			assertSpendErr: trace.IsNotFound,
		},
		{
			name:        "token must match",
			deviceID:    deviceID,
			createToken: s.CreateDeviceEnrollToken,
			spendToken: func(ctx context.Context, deviceID, token string) error {
				return s.SpendDeviceEnrollToken(ctx, deviceID, token+"bad")
			},
			assertSpendErr: trace.IsBadParameter,
		},
		{
			name:            "unknown device fails create",
			deviceID:        "unknown",
			createToken:     s.CreateDeviceEnrollToken,
			assertCreateErr: trace.IsNotFound,
		},
		{
			name:        "unknown device fails spend",
			deviceID:    deviceID,
			createToken: s.CreateDeviceEnrollToken,
			spendToken: func(ctx context.Context, _, token string) error {
				return s.SpendDeviceEnrollToken(ctx, "unknown", token)
			},
			assertSpendErr: trace.IsNotFound,
		},
		{
			name:        "double spend fails",
			deviceID:    deviceID,
			createToken: s.CreateDeviceEnrollToken,
			spendToken: func(ctx context.Context, deviceID string, token string) error {
				if err := s.SpendDeviceEnrollToken(ctx, deviceID, token); err != nil {
					return errors.New("first spend failed")
				}

				return s.SpendDeviceEnrollToken(ctx, deviceID, token)
			},
			assertSpendErr: trace.IsNotFound,
		},
		{
			name:            "CreateDeviceEnrollToken requires device ID",
			deviceID:        "",
			createToken:     s.CreateDeviceEnrollToken,
			assertCreateErr: trace.IsBadParameter,
		},
		{
			name:        "SpendDeviceEnrollToken requires device ID",
			deviceID:    deviceID,
			createToken: s.CreateDeviceEnrollToken,
			spendToken: func(ctx context.Context, _, token string) error {
				return s.SpendDeviceEnrollToken(ctx, "" /* deviceID */, token)
			},
			assertSpendErr: trace.IsBadParameter,
		},
		{
			name:        "SpendDeviceEnrollToken requires token",
			deviceID:    deviceID,
			createToken: s.CreateDeviceEnrollToken,
			spendToken: func(ctx context.Context, deviceID, _ string) error {
				return s.SpendDeviceEnrollToken(ctx, deviceID, "" /* token */)
			},
			assertSpendErr: trace.IsBadParameter,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			token, err := test.createToken(ctx, test.deviceID)
			switch {
			case test.assertCreateErr != nil:
				if !test.assertCreateErr(err) {
					t.Errorf("CreateDeviceEnrollToken: assert failed, err=%v", err)
				}
				return
			case err != nil:
				t.Fatalf("CreateDeviceEnrollToken failed: %v", err)
			}

			switch err := test.spendToken(ctx, test.deviceID, token.GetToken()); {
			case test.assertSpendErr != nil:
				if !test.assertSpendErr(err) {
					t.Errorf("SpendDeviceEnrollmentToken: assert failed, err=%v", err)
				}
			case err != nil:
				t.Fatalf("SpendDeviceEnrollmentToken failed: %v", err)
			}
		})
	}
}

// storageEnv groups the necessary components to test storage.
type storageEnv struct {
	Clock clockwork.FakeClock
	S     *storage.S
	mem   *memory.Memory
}

func (e *storageEnv) Close() error {
	if e.mem != nil {
		return e.mem.Close()
	}
	return nil
}

func mustNewEnv() *storageEnv {
	env, err := newEnv()
	if err != nil {
		panic(err)
	}
	return env
}

func newEnv() (*storageEnv, error) {
	clock := clockwork.NewFakeClock()
	mem, err := memory.New(memory.Config{
		Clock: clock,
	})
	if err != nil {
		return nil, err
	}

	s, err := storage.New(func() backend.Backend { return mem })
	if err != nil {
		mem.Close()
		return nil, err
	}
	return &storageEnv{
		Clock: clock,
		S:     s,
		mem:   mem,
	}, nil
}
