package storage_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"errors"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/e/lib/devicetrust/storage"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
)

func TestS_BulkCreateDevices(t *testing.T) {
	env := mustNewEnv()
	defer env.Close()

	s := env.S
	ctx := context.Background()

	// Make sure "alpaca" already exists before we attempt the bulk creation.
	alpacaDev, err := s.CreateDevice(ctx, &devicepb.Device{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "alpaca",
	})
	if err != nil {
		t.Fatalf("CreateDevice failed: %v", err)
	}

	// Bulk create a few devices, mixing successes and failures.
	devs := s.BulkCreateDevices(ctx, []*devicepb.Device{
		// OK.
		{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: "llama",
		},
		// NOK, duplicate within devs.
		{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: "llama",
		},
		// NOK, duplicate in storage.
		{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: "alpaca",
		},
		// NOK, duplicate within devs (and in storage).
		{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: "alpaca",
		},
		// OK.
		{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: "camel",
		},
		// NOK, invalid OsType.
		{
			OsType:   devicepb.OSType_OS_TYPE_UNSPECIFIED,
			AssetTag: "cat",
		},
	})

	// Verify response codes.
	wantCodes := []codes.Code{
		codes.OK,              // llama
		codes.AlreadyExists,   // llama dupe
		codes.AlreadyExists,   // alpaca dupe
		codes.AlreadyExists,   // alpaca dupe
		codes.OK,              // camel
		codes.InvalidArgument, // cat, missing OsType
	}
	gotCodes := make([]codes.Code, len(devs))
	for i, dev := range devs {
		c := codes.Code(dev.GetStatus().GetCode())
		gotCodes[i] = c

		// Sanity check IDs.
		if c == codes.OK && dev.GetId() == "" {
			t.Errorf("BulkCreateDevices: device #%v has code %s but an empty ID", i, c)
		}
	}
	if diff := cmp.Diff(wantCodes, gotCodes); diff != "" {
		t.Fatalf("BulkCreateDevices codes mismatch (-want +got):\n%s", diff)
	}
	llamaDev := devs[0]
	camelDev := devs[4]

	// Verify stored devices.
	pageSize := len(devs) + 2 // devs+alpaca+1, so we can detect unwanted devices
	stored, _, err := s.ListDevices(ctx, pageSize, "" /* pageToken */, devicepb.DeviceView_DEVICE_VIEW_LIST)
	if err != nil {
		t.Fatalf("ListDevices failed: %v", err)
	}
	wantDevs := map[string]string{
		alpacaDev.Id: "alpaca",
		llamaDev.Id:  "llama", // AssetTag not present in DeviceOrStatus.
		camelDev.Id:  "camel",
	}
	gotDevs := make(map[string]string)
	for _, dev := range stored {
		gotDevs[dev.Id] = dev.AssetTag
	}
	if diff := cmp.Diff(wantDevs, gotDevs); diff != "" {
		t.Errorf("ListDevices mismatch (-want +got):\n%s", diff)
	}
}

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
				EnrollStatus: devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED,
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

	want := []*devicepb.Device{devMac, devLinux, devWin}
	if diff := diffDevices(want, got); diff != "" {
		t.Errorf("GetDevicesByAssetTag: mismatch (-want +got):\n%s", diff)
	}
}

// TestS_CreateDevice_concurrentAssetTags tests a CreateDevice race condition
// where neither CreateDevice calls can determine if an asset tag is legitimate
// or hanging. In this scenario, both devices are registered for the same asset
// tag.
func TestS_CreateDevice_concurrentAssetTags(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping potential long-running test")
	}

	timeout := time.After(1 * time.Second)

	// Run until we hit the desired scenario or time out.
	for i := 0; true; i++ {
		select {
		case <-timeout:
			t.Log("Stopping test before desired scenario was achieved")
			return
		default:
		}

		env := mustNewEnv()
		defer env.Close()

		s := env.S
		ctx := context.Background()

		var wg sync.WaitGroup
		createLlama := func() {
			if _, err := s.CreateDevice(ctx, &devicepb.Device{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "llama",
			}); err != nil && !trace.IsAlreadyExists(err) {
				t.Errorf("CreateDevice returned an unexpected error: %v (want nil or already exists)", err)
			}
			wg.Done()
		}

		wg.Add(2)
		go createLlama()
		go createLlama()
		wg.Wait()

		stored, err := s.GetDevicesByAssetTag(ctx, "llama")
		if err != nil {
			t.Fatalf("GetDevicesByAssetTag failed: %v", err)
		}
		if len(stored) < 1 {
			t.Errorf("GetDevicesByAssetTag returned %v devices, want at least 1", len(stored))
		}
		if len(stored) == 2 {
			t.Logf("Got tag override scenario on i=%v, stopping the test", i)
			return
		}
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
		got, _, err := s.ListDevices(ctx, 3 /* pageSize */, "" /* pageToken */, devicepb.DeviceView_DEVICE_VIEW_RESOURCE)
		if err != nil {
			t.Fatalf("ListDevices failed: %v", err)
		}

		want := []*devicepb.Device{d2, d3}
		if diff := diffDevices(want, got); diff != "" {
			t.Errorf("ListDevices mismatch (-want +got)\n%s", diff)
		}
	})
}

// TestS_DeleteDevice_assetTagMappings verifies that device deletion doesn't
// have undesired side-effects in devices with similar asset tags.
func TestS_DeleteDevice_assetTagMappings(t *testing.T) {
	env := mustNewEnv()
	defer env.Close()

	s := env.S
	ctx := context.Background()

	llamaMac := &devicepb.Device{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "llama",
	}
	llamaLinux := &devicepb.Device{
		OsType:   devicepb.OSType_OS_TYPE_LINUX,
		AssetTag: "llama",
	}
	llamaWin := &devicepb.Device{
		OsType:   devicepb.OSType_OS_TYPE_WINDOWS,
		AssetTag: "llama",
	}
	unrelatedDev := &devicepb.Device{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "unrelated",
	}
	for _, dev := range []**devicepb.Device{
		&llamaMac,
		&llamaLinux,
		&llamaWin,
		&unrelatedDev,
	} {
		created, err := s.CreateDevice(ctx, *dev)
		if err != nil {
			t.Fatalf("CreateDevice failed: %v", err)
		}
		*dev = created
	}

	// Delete device.
	if err := s.DeleteDevice(ctx, llamaMac.Id); err != nil {
		t.Fatalf("DeleteDevice failed: %v", err)
	}

	// Sanity check: device is not in storage anymore.
	if _, err := s.GetDeviceByID(ctx, llamaMac.Id); !trace.IsNotFound(err) {
		t.Fatalf("GetDeviceByID returned an unexpected error: %v (want not found)", err)
	}

	// Verify asset tag read.
	gotTagDevs, err := s.GetDevicesByAssetTag(ctx, llamaMac.AssetTag)
	if err != nil {
		t.Fatalf("GetDevicesByAssetTag failed: %v", err)
	}
	want := []*devicepb.Device{llamaLinux, llamaWin}
	if diff := diffDevices(want, gotTagDevs); diff != "" {
		t.Errorf("GetDevicesByAssetTag mismatch (-want +got):\n%s", diff)
	}

	// Sanity check: unrelated devices are OK.
	gotAllDevs, _, err := s.ListDevices(ctx, 100 /* pageSize */, "" /* pageToken */, devicepb.DeviceView_DEVICE_VIEW_RESOURCE)
	if err != nil {
		t.Fatalf("ListDevices failed: %v", err)
	}
	want = []*devicepb.Device{llamaLinux, llamaWin, unrelatedDev}
	if diff := diffDevices(want, gotAllDevs); diff != "" {
		t.Errorf("ListDevices mismatch (-want +got):\n%s", diff)
	}
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
		devs, nextPageToken, err := s.ListDevices(ctx, defaultPageSize, defaultPageToken, devicepb.DeviceView_DEVICE_VIEW_LIST)
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
	var fullDevs []*devicepb.Device
	for _, assetTag := range []string{"llama", "alpaca", "camel", "horse", "duck"} {
		dev, err := s.CreateDevice(ctx, &devicepb.Device{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: assetTag,
		})
		if err != nil {
			t.Fatalf("CreateDevice(%q) failed: %v", assetTag, err)
		}
		fullDevs = append(fullDevs, dev)
	}

	// Create a couple of enrollment tokens, so we can make sure they don't
	// pollute the results.
	for _, deviceID := range []string{fullDevs[0].Id, fullDevs[1].Id} {
		if _, err := s.CreateDeviceEnrollToken(ctx, deviceID); err != nil {
			t.Fatalf("CreateDeviceEnrollToken failed: %v", err)
		}
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

	// TODO(codingllama): Test listing in resource view with a full-data device,
	//  once we have one. (Ie, credential data, collected data, etc.)

	tests := []struct {
		name             string
		pageSize         int
		view             devicepb.DeviceView
		wantDevices      []*devicepb.Device
		wantTrimmedPages bool
	}{
		{
			name:        "default page size",
			view:        devicepb.DeviceView_DEVICE_VIEW_RESOURCE,
			wantDevices: fullDevs,
		},
		{
			name:        "page size 1",
			pageSize:    1,
			view:        devicepb.DeviceView_DEVICE_VIEW_RESOURCE,
			wantDevices: fullDevs,
		},
		{
			name:        "page size odd",
			pageSize:    2,
			view:        devicepb.DeviceView_DEVICE_VIEW_RESOURCE,
			wantDevices: fullDevs,
		},
		{
			name:        "page size even",
			pageSize:    3,
			view:        devicepb.DeviceView_DEVICE_VIEW_RESOURCE,
			wantDevices: fullDevs,
		},
		{
			name:        "empty last page",
			pageSize:    len(fullDevs),
			view:        devicepb.DeviceView_DEVICE_VIEW_RESOURCE,
			wantDevices: fullDevs,
		},
		{
			name:        "all results in first page",
			pageSize:    len(fullDevs) + 1,
			view:        devicepb.DeviceView_DEVICE_VIEW_RESOURCE,
			wantDevices: fullDevs,
		},
		{
			name:             "large page sizes get trimmed",
			pageSize:         100_000,
			view:             devicepb.DeviceView_DEVICE_VIEW_RESOURCE,
			wantDevices:      fullDevs,
			wantTrimmedPages: true,
		},
		{
			name:        "negative page size ignored",
			pageSize:    -1,
			view:        devicepb.DeviceView_DEVICE_VIEW_RESOURCE,
			wantDevices: fullDevs,
		},
		{
			name:        "list view",
			view:        devicepb.DeviceView_DEVICE_VIEW_LIST,
			wantDevices: listDevs,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// We expect to eventually get all devices in all scenarios, so we'll
			// collect them here and later compare against the complete list.
			var pageToken string
			var got []*devicepb.Device
			for {
				devs, nextPageToken, err := s.ListDevices(ctx, test.pageSize, pageToken, test.view)
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

			want := test.wantDevices
			if diff := diffDevices(want, got); diff != "" {
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

	tests := []struct {
		name      string
		pageToken string
		view      devicepb.DeviceView
		wantErr   string
	}{
		{
			name:      "pageToken invalid",
			pageToken: "abadpagetoken",
			view:      devicepb.DeviceView_DEVICE_VIEW_LIST,
			wantErr:   "page token",
		},
		{
			name:    "view invalid",
			view:    devicepb.DeviceView_DEVICE_VIEW_UNSPECIFIED,
			wantErr: "view required",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, _, err := s.ListDevices(ctx, 0 /* pageSize */, test.pageToken, test.view)
			if !trace.IsBadParameter(err) {
				t.Fatalf("ListDevices returned an unexpected error: %v", err)
			}
			assert.ErrorContains(t, err, test.wantErr, "ListDevices error mismatch")
		})
	}
}

func TestS_EnrollDevice(t *testing.T) {
	env := mustNewEnv()
	defer env.Close()

	s := env.S
	ctx := context.Background()

	dev, err := s.CreateDevice(ctx, &devicepb.Device{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "llama",
	})
	if err != nil {
		t.Fatalf("CreateDevice failed: %v", err)
	}

	key1, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}
	key1DER, err := x509.MarshalPKIXPublicKey(key1.Public())
	if err != nil {
		t.Fatalf("MarshalPKIXPublicKey failed: %v", err)
	}
	key2, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}
	key2DER, err := x509.MarshalPKIXPublicKey(key2.Public())
	if err != nil {
		t.Fatalf("MarshalPKIXPublicKey failed: %v", err)
	}

	tests := []struct {
		name    string
		baseDev *devicepb.Device
		cred    *devicepb.DeviceCredential
		cd      *devicepb.DeviceCollectedData
	}{
		{
			name:    "ok",
			baseDev: dev,
			cred: &devicepb.DeviceCredential{
				Id:           "cred1",
				PublicKeyDer: key1DER,
			},
			cd: &devicepb.DeviceCollectedData{
				CollectTime:  timestamppb.Now(),
				OsType:       dev.OsType,
				SerialNumber: dev.AssetTag,
			},
		},
		{
			// Note: this test case depends on the device being successfully enrolled
			// above.
			name:    "re-enroll",
			baseDev: dev,
			cred: &devicepb.DeviceCredential{
				Id:           "cred2",
				PublicKeyDer: key2DER,
			},
			cd: &devicepb.DeviceCollectedData{
				CollectTime:  timestamppb.Now(),
				OsType:       dev.OsType,
				SerialNumber: dev.AssetTag,
			},
		},
	}
	for _, test := range tests {
		deviceID := test.baseDev.Id
		got, err := s.EnrollDevice(ctx, deviceID, test.cred, test.cd)
		if err != nil {
			t.Fatalf("EnrollDevice failed: %v", err)
		}

		if got.UpdateTime.AsTime().Before(test.baseDev.UpdateTime.AsTime()) {
			t.Errorf("got.UpdateTime = %v, want >= %v", got.UpdateTime, test.baseDev.UpdateTime)
		}

		want := proto.Clone(test.baseDev).(*devicepb.Device)
		want.UpdateTime = got.UpdateTime
		want.EnrollStatus = devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_ENROLLED
		want.Credential = test.cred
		if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
			t.Errorf("EnrollDevice mismatch (-want +got):\n%s", diff)
		}

		// Are changes reflected in storage?
		stored, err := s.GetDeviceByID(ctx, deviceID)
		if err != nil {
			t.Fatalf("GetDeviceByID failed: %v", err)
		}
		// TODO(codingllama): Assert collected data on tests.
		stored.CollectedData = nil
		if diff := cmp.Diff(got, stored, protocmp.Transform()); diff != "" {
			t.Errorf("GetDeviceByID mismatch (-want +got):\n%s", diff)
		}
	}
}

func TestS_EnrollDevice_errors(t *testing.T) {
	env := mustNewEnv()
	defer env.Close()

	s := env.S
	ctx := context.Background()

	dev, err := s.CreateDevice(ctx, &devicepb.Device{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "llama",
	})
	if err != nil {
		t.Fatalf("CreateDevice failed: %v", err)
	}
	key1, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}
	key1DER, err := x509.MarshalPKIXPublicKey(key1.Public())
	if err != nil {
		t.Fatalf("MarshalPKIXPublicKey failed: %v", err)
	}

	validCred := &devicepb.DeviceCredential{
		Id:           "cred1",
		PublicKeyDer: key1DER,
	}
	validCD := &devicepb.DeviceCollectedData{
		CollectTime:  timestamppb.Now(),
		OsType:       dev.OsType,
		SerialNumber: dev.AssetTag,
	}

	tests := []struct {
		name       string
		deviceID   string
		createCred func() *devicepb.DeviceCredential
		createCD   func() *devicepb.DeviceCollectedData
		assertErr  func(error) bool
		wantErr    string
	}{
		{
			name:       "unknown device",
			deviceID:   "unknown",
			createCred: func() *devicepb.DeviceCredential { return validCred },
			createCD:   func() *devicepb.DeviceCollectedData { return validCD },
			assertErr:  trace.IsNotFound,
		},
		{
			name:       "credential nil",
			deviceID:   dev.Id,
			createCred: func() *devicepb.DeviceCredential { return nil },
			createCD:   func() *devicepb.DeviceCollectedData { return validCD },
			assertErr:  trace.IsBadParameter,
			wantErr:    "credential required",
		},
		{
			name:     "credential ID empty",
			deviceID: dev.Id,
			createCred: func() *devicepb.DeviceCredential {
				cp := proto.Clone(validCred).(*devicepb.DeviceCredential)
				cp.Id = ""
				return cp
			},
			createCD:  func() *devicepb.DeviceCollectedData { return validCD },
			assertErr: trace.IsBadParameter,
			wantErr:   "credential ID",
		},
		{
			name:     "credential PublicKeyDer empty",
			deviceID: dev.Id,
			createCred: func() *devicepb.DeviceCredential {
				cp := proto.Clone(validCred).(*devicepb.DeviceCredential)
				cp.PublicKeyDer = nil
				return cp
			},
			createCD:  func() *devicepb.DeviceCollectedData { return validCD },
			assertErr: trace.IsBadParameter,
			wantErr:   "public key required",
		},
		{
			name:     "credential PublicKeyDer invalid",
			deviceID: dev.Id,
			createCred: func() *devicepb.DeviceCredential {
				cp := proto.Clone(validCred).(*devicepb.DeviceCredential)
				cp.PublicKeyDer = []byte("not a DER")
				return cp
			},
			createCD:  func() *devicepb.DeviceCollectedData { return validCD },
			assertErr: trace.IsBadParameter,
			wantErr:   "invalid credential public key",
		},
		{
			name:       "collectedData nil",
			deviceID:   dev.Id,
			createCred: func() *devicepb.DeviceCredential { return validCred },
			createCD:   func() *devicepb.DeviceCollectedData { return nil },
			assertErr:  trace.IsBadParameter,
			wantErr:    "collected data required",
		},
		{
			name:       "collectedData CollectTime nil",
			deviceID:   dev.Id,
			createCred: func() *devicepb.DeviceCredential { return validCred },
			createCD: func() *devicepb.DeviceCollectedData {
				cp := proto.Clone(validCD).(*devicepb.DeviceCollectedData)
				cp.CollectTime = nil
				return cp

			},
			assertErr: trace.IsBadParameter,
			wantErr:   "collect time missing",
		},
		{
			name:       "collectedData OSType mismatch",
			deviceID:   dev.Id,
			createCred: func() *devicepb.DeviceCredential { return validCred },
			createCD: func() *devicepb.DeviceCollectedData {
				cp := proto.Clone(validCD).(*devicepb.DeviceCollectedData)
				cp.OsType = devicepb.OSType_OS_TYPE_LINUX
				return cp

			},
			assertErr: trace.IsBadParameter,
			wantErr:   "OS type mismatch",
		},
		{
			name:       "collectedData SerialNumber mismatch",
			deviceID:   dev.Id,
			createCred: func() *devicepb.DeviceCredential { return validCred },
			createCD: func() *devicepb.DeviceCollectedData {
				cp := proto.Clone(validCD).(*devicepb.DeviceCollectedData)
				cp.SerialNumber = "not the same as the other"
				return cp

			},
			assertErr: trace.IsBadParameter,
			wantErr:   "serial number mismatch",
		},
	}
	for _, test := range tests {
		_, err := s.EnrollDevice(ctx, test.deviceID, test.createCred(), test.createCD())
		if !test.assertErr(err) {
			t.Errorf("EnrollDevice: assertErr failed, err=%v", err)
		}
		assert.ErrorContains(t, err, test.wantErr, "EnrollDevice error mismatch")
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

// diffDevices diffs two slices of devices, sorting both by ID first.
func diffDevices(want, got []*devicepb.Device) string {
	sort.Slice(want, func(i, j int) bool { return want[i].Id < want[j].Id })
	sort.Slice(got, func(i, j int) bool { return got[i].Id < got[j].Id })
	return cmp.Diff(want, got, protocmp.Transform())
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
