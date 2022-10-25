package storage_test

import (
	"context"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/teleport/e/lib/devicetrust/storage"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
)

func TestS_CreateDevice(t *testing.T) {
	s, closer := mustNewS()
	defer closer()

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
	s, closer := mustNewS()
	defer closer()

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
	s, closer := mustNewS()
	defer closer()

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

func TestS_GetDeviceByID_notFound(t *testing.T) {
	s, closer := mustNewS()
	defer closer()

	ctx := context.Background()
	if _, err := s.GetDeviceByID(ctx, "unknown"); !trace.IsNotFound(err) {
		t.Errorf("GetDeviceByID returned an unexpected error: %v", err)
	}
}

func TestS_GetDevicesByAssetTag_noDevices(t *testing.T) {
	s, closer := mustNewS()
	defer closer()

	ctx := context.Background()
	got, err := s.GetDevicesByAssetTag(ctx, "unknown")
	switch {
	case err != nil:
		t.Fatalf("GetDevicesByAssetTag failed: %v", err)
	case len(got) != 0:
		t.Fatalf("GetDevicesByAssetTag returned non-empty slice: %v", got)
	}
}

func mustNewS() (*storage.S, func() error) {
	s, closer, err := newS()
	if err != nil {
		panic(err)
	}
	return s, closer
}

func newS() (*storage.S, func() error, error) {
	mem, err := memory.New(memory.Config{})
	if err != nil {
		return nil, nil, err
	}

	s, err := storage.New(func() backend.Backend { return mem })
	if err != nil {
		mem.Close()
		return nil, nil, err
	}
	return s, mem.Close, err
}
