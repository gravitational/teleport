package storage_test

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/e/lib/devicetrust/storage"
	"github.com/gravitational/teleport/lib/backend/memory"
)

func TestS_BulkCreateDevices(t *testing.T) {
	env := mustNewEnv()
	defer env.Close()

	s := env.S
	ctx := context.Background()

	// Make sure "alpaca" already exists before we attempt the bulk creation.
	_, err := s.CreateDevice(ctx, &devicepb.Device{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "alpaca",
	}, false /* createAsResource */)
	if err != nil {
		t.Fatalf("CreateDevice failed: %v", err)
	}

	const resource1Tag = "AAA000000000"
	const resource2Tag = "BBB000000000"
	_, pubKeyDER := newKeyPair(t)

	// resource1 is a complete, resource-like device.
	resource1 := &devicepb.Device{
		ApiVersion: "v1",
		Id:         "a6f76866-a9eb-4a23-9bb1-7980347a1bee",
		OsType:     devicepb.OSType_OS_TYPE_MACOS,
		AssetTag:   resource1Tag,
		CreateTime: timestamppb.New(time.Date(2023, 2, 24, 19, 0, 0, 0, time.UTC)),
		UpdateTime: timestamppb.New(time.Date(2023, 2, 24, 19, 15, 0, 500, time.UTC)),
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
				CollectTime:  timestamppb.New(time.Date(2023, 2, 24, 19, 0, 5, 0, time.UTC)),
				RecordTime:   timestamppb.New(time.Date(2023, 2, 24, 19, 0, 5, 500, time.UTC)),
				OsType:       devicepb.OSType_OS_TYPE_MACOS,
				SerialNumber: resource1Tag,
			},
			{
				CollectTime:  timestamppb.New(time.Date(2023, 2, 24, 19, 0, 15, 0, time.UTC)),
				RecordTime:   timestamppb.New(time.Date(2023, 2, 24, 19, 0, 15, 500, time.UTC)),
				OsType:       devicepb.OSType_OS_TYPE_MACOS,
				SerialNumber: resource1Tag,
			},
		},
	}

	// listAll is a helper that lists all devices in storage.
	listAll := func(t *testing.T) []*devicepb.Device {
		t.Helper()

		var allDevices []*devicepb.Device
		var pageToken string
		for {
			devs, nextPageToken, err := s.ListDevices(ctx, 0 /* pageSize */, pageToken, devicepb.DeviceView_DEVICE_VIEW_RESOURCE)
			if err != nil {
				t.Fatalf("ListDevices failed: %v", err)
			}
			allDevices = append(allDevices, devs...)
			if nextPageToken == "" {
				break
			}
			pageToken = nextPageToken
		}

		return allDevices
	}

	// Tests build on each other: this is desirable so can tests errors such as
	// AlreadyExists, but it does making testing a bit more complex as a whole.
	tests := []struct {
		name             string
		createAsResource bool
		devices          []*devicepb.Device
		wantCodes        []codes.Code
		wantDevices      func(t *testing.T, got []*devicepb.DeviceOrStatus) map[string]string // ID->AssetTag
	}{
		{
			name: "ok",
			devices: []*devicepb.Device{
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
			},
			wantCodes: []codes.Code{
				codes.OK,              // llama
				codes.AlreadyExists,   // llama dupe
				codes.AlreadyExists,   // alpaca dupe
				codes.AlreadyExists,   // alpaca dupe
				codes.OK,              // camel
				codes.InvalidArgument, // cat, missing OsType
			},
			wantDevices: func(t *testing.T, got []*devicepb.DeviceOrStatus) map[string]string {
				llamaDev := got[0]
				camelDev := got[4]
				return map[string]string{
					llamaDev.Id: "llama",
					camelDev.Id: "camel",
				}
			},
		},
		{
			name:             "createAsResource",
			createAsResource: true,
			devices: []*devicepb.Device{
				// OK: Complete resource.
				resource1,
				// OK: Partial resource.
				{
					OsType:   devicepb.OSType_OS_TYPE_MACOS,
					AssetTag: resource2Tag,
				},
				// NOK: Invalid resource: missing mandatory field.
				{
					OsType:   devicepb.OSType_OS_TYPE_MACOS,
					AssetTag: "",
				},
				// NOK: Invalid resource: failed resource-like validation.
				{
					ApiVersion: "v99", // invalid
					OsType:     devicepb.OSType_OS_TYPE_MACOS,
					AssetTag:   "XXXXXXXXXXXX",
				},
			},
			wantCodes: []codes.Code{
				codes.OK, // resource1
				codes.OK, // resource2
				codes.InvalidArgument,
				codes.InvalidArgument,
			},
			wantDevices: func(t *testing.T, got []*devicepb.DeviceOrStatus) map[string]string {
				// ID in `got` matches requested ID.
				resOrStatus1 := got[0]
				if resOrStatus1.Id != resource1.Id {
					t.Errorf("BulkCreateDevices: got[0].Id = %v, want %v", resOrStatus1.Id, resource1.Id)
				}

				resource2 := got[1]
				return map[string]string{
					resource1.Id: resource1.AssetTag, // ID is the same as requested
					resource2.Id: resource2Tag,
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			devsBefore := listAll(t)

			got := s.BulkCreateDevices(ctx, test.devices, test.createAsResource)

			devsAfter := listAll(t)

			// Verify response codes from `got`.
			gotCodes := make([]codes.Code, len(got))
			for i, dev := range got {
				c := codes.Code(dev.GetStatus().GetCode())
				gotCodes[i] = c

				// Sanity check IDs.
				if c == codes.OK && dev.GetId() == "" {
					t.Errorf("BulkCreateDevices: device #%v has code %s but an empty ID", i, c)
				}
			}
			if diff := cmp.Diff(test.wantCodes, gotCodes); diff != "" {
				t.Fatalf("BulkCreateDevices codes mismatch (-want +got):\n%s", diff)
			}

			// Verify storage state.
			gotDevs := make(map[string]string)
			for _, d := range devsAfter {
				gotDevs[d.Id] = d.AssetTag
			}
			wantDevs := make(map[string]string)
			for _, d := range devsBefore { // Fill want with existing devs...
				wantDevs[d.Id] = d.AssetTag
			}
			for k, v := range test.wantDevices(t, got) { //...then add the expected devs
				wantDevs[k] = v
			}
			if diff := cmp.Diff(wantDevs, gotDevs); diff != "" {
				t.Errorf("ListDevices mismatch (-want +got)\n%s", diff)
			}
		})
	}
}

func TestS_CreateDevice(t *testing.T) {
	env := mustNewEnv()
	defer env.Close()

	s := env.S
	ctx := context.Background()

	clock := env.Clock
	clockAdvance := func() {
		clock.Advance(1 * time.Second)
	}

	const dev1Tag = "A00AA0AAAA0A"
	const resource1Tag = "AAA000000000"
	const resource2Tag = "BBB000000000"
	const resource3Tag = "CCC000000000"

	_, pubKeyDER := newKeyPair(t)

	// excessiveCD is used by resource-like write tests.
	excessiveCD := make([]*devicepb.DeviceCollectedData, storage.MaxCollectedDataPerDevice+2)
	for i := range excessiveCD {
		now := clock.Now().UTC()
		excessiveCD[i] = &devicepb.DeviceCollectedData{
			CollectTime:  timestamppb.New(now),
			RecordTime:   timestamppb.New(now.Add(50 * time.Millisecond)),
			OsType:       devicepb.OSType_OS_TYPE_MACOS,
			SerialNumber: resource3Tag,
		}
		clockAdvance()
	}

	tests := []struct {
		name             string
		dev              *devicepb.Device
		createAsResource bool
		modifyWant       func(dev, want *devicepb.Device) // adjust want for `createAsResource` tests.
	}{
		{
			name: "ok",
			dev: &devicepb.Device{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: dev1Tag,
			},
		},
		{
			name: "create as a resource",
			dev: &devicepb.Device{
				ApiVersion: "v1",
				Id:         "a6f76866-a9eb-4a23-9bb1-7980347a1bee",
				OsType:     devicepb.OSType_OS_TYPE_MACOS,
				AssetTag:   resource1Tag,
				CreateTime: timestamppb.New(time.Date(2023, 2, 24, 19, 0, 0, 0, time.UTC)),
				UpdateTime: timestamppb.New(time.Date(2023, 2, 24, 19, 15, 0, 500, time.UTC)),
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
						CollectTime:  timestamppb.New(time.Date(2023, 2, 24, 19, 0, 5, 0, time.UTC)),
						RecordTime:   timestamppb.New(time.Date(2023, 2, 24, 19, 0, 5, 500, time.UTC)),
						OsType:       devicepb.OSType_OS_TYPE_MACOS,
						SerialNumber: resource1Tag,
					},
					{
						CollectTime:  timestamppb.New(time.Date(2023, 2, 24, 19, 0, 15, 0, time.UTC)),
						RecordTime:   timestamppb.New(time.Date(2023, 2, 24, 19, 0, 15, 500, time.UTC)),
						OsType:       devicepb.OSType_OS_TYPE_MACOS,
						SerialNumber: resource1Tag,
					},
				},
			},
			createAsResource: true,
			modifyWant: func(dev, want *devicepb.Device) {
				// All fields that are typically system-generated are copied from `dev`.
				want.Id = dev.Id
				want.CreateTime = dev.CreateTime
				want.UpdateTime = dev.UpdateTime
				want.EnrollStatus = dev.EnrollStatus
				want.Credential = dev.Credential
				want.CollectedData = dev.CollectedData
			},
		},
		{
			name: "partial create as a resource", // Only required fields set.
			dev: &devicepb.Device{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: resource2Tag,
			},
			createAsResource: true,
		},
		{
			name: "excessive collected data discarded on write",
			dev: &devicepb.Device{
				OsType:        devicepb.OSType_OS_TYPE_MACOS,
				AssetTag:      resource3Tag,
				CollectedData: excessiveCD,
			},
			createAsResource: true,
			modifyWant: func(_ *devicepb.Device, want *devicepb.Device) {
				// Keep the oldest CD as enrollment data, skip the excessive entry
				// (index 1) and retain the rest.
				want.CollectedData = append(excessiveCD[:1], excessiveCD[2:]...)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := s.CreateDevice(ctx, test.dev, test.createAsResource)
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
			if m := storage.MaxCollectedDataPerDevice + 1; len(got.CollectedData) > m {
				t.Errorf("CreateDevice: got %v CollectedData entries, want <= %v", len(got.CollectedData), m)
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
			if test.modifyWant != nil {
				test.modifyWant(test.dev, want)
			}
			// Sanity check: wanted collected data is within the limits of
			// MaxCollectedDataPerDevice.
			if m := storage.MaxCollectedDataPerDevice + 1; len(want.CollectedData) > m {
				t.Errorf("want.CollectedData has %v entries, it should have at most %v entries", len(got.CollectedData), m)
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

func newKeyPair(t *testing.T) (priv *ecdsa.PrivateKey, pubKeyDER []byte) {
	t.Helper()

	var err error
	priv, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	pubKeyDER, err = x509.MarshalPKIXPublicKey(priv.Public())
	if err != nil {
		t.Fatalf("MarshalPKIXPublicKey failed: %v", err)
	}
	return priv, pubKeyDER
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
		created, err := s.CreateDevice(ctx, *dev, false /* createAsResource */)
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
			}, false /* createAsResource */); err != nil && !trace.IsAlreadyExists(err) {
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
	if _, err := s.CreateDevice(ctx, validDev, false /* createAsResource */); err != nil {
		t.Fatalf("CreateDevice failed: %v", err)
	}

	// Switch tags so the device would be perfectly valid for creation.
	const otherTag = "alpaca"
	validDev.AssetTag = otherTag

	tests := []struct {
		name             string
		createDev        func() *devicepb.Device
		createAsResource bool
		wantErr          string
		assertErr        func(error) bool
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
			name: "asset_tag length",
			createDev: func() *devicepb.Device {
				d := proto.Clone(validDev).(*devicepb.Device)
				d.AssetTag = strings.Repeat("A", 41)
				return d
			},
			wantErr:   "asset_tag exceeds",
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
		{
			name: "resource: invalid api_version",
			createDev: func() *devicepb.Device {
				d := proto.Clone(validDev).(*devicepb.Device)
				d.ApiVersion = "v99"
				return d
			},
			createAsResource: true,
			wantErr:          "api_version",
			assertErr:        trace.IsBadParameter,
		},
		{
			name: "resource: ID not an UUID",
			createDev: func() *devicepb.Device {
				d := proto.Clone(validDev).(*devicepb.Device)
				d.Id = "banana"
				return d
			},
			createAsResource: true,
			wantErr:          "device ID",
			assertErr:        trace.IsBadParameter,
		},
		{
			name: "resource: asset_tag empty",
			createDev: func() *devicepb.Device {
				d := proto.Clone(validDev).(*devicepb.Device)
				d.AssetTag = ""
				return d
			},
			createAsResource: true,
			wantErr:          "asset_tag required",
			assertErr:        trace.IsBadParameter,
		},
		{
			name: "resource: os_type unspecified",
			createDev: func() *devicepb.Device {
				d := proto.Clone(validDev).(*devicepb.Device)
				d.OsType = devicepb.OSType_OS_TYPE_UNSPECIFIED
				return d
			},
			createAsResource: true,
			wantErr:          "invalid os_type",
			assertErr:        trace.IsBadParameter,
		},
		{
			name: "resource: create_time invalid",
			createDev: func() *devicepb.Device {
				d := proto.Clone(validDev).(*devicepb.Device)
				d.CreateTime = &timestamppb.Timestamp{
					Nanos: -1,
				}
				d.UpdateTime = timestamppb.Now() // both timestamps must be set
				return d
			},
			createAsResource: true,
			wantErr:          "invalid create_time",
			assertErr:        trace.IsBadParameter,
		},
		{
			name: "resource: update_time invalid",
			createDev: func() *devicepb.Device {
				d := proto.Clone(validDev).(*devicepb.Device)
				d.CreateTime = timestamppb.Now() // both timestamps must be set
				d.UpdateTime = &timestamppb.Timestamp{
					Nanos: -1,
				}
				return d
			},
			createAsResource: true,
			wantErr:          "invalid update_time",
			assertErr:        trace.IsBadParameter,
		},
		{
			name: "resource: create_time without update_time",
			createDev: func() *devicepb.Device {
				d := proto.Clone(validDev).(*devicepb.Device)
				d.CreateTime = timestamppb.Now()
				return d
			},
			createAsResource: true,
			wantErr:          "create_time and update_time",
			assertErr:        trace.IsBadParameter,
		},
		{
			name: "resource: update_time without create_time",
			createDev: func() *devicepb.Device {
				d := proto.Clone(validDev).(*devicepb.Device)
				d.UpdateTime = timestamppb.Now()
				return d
			},
			createAsResource: true,
			wantErr:          "create_time and update_time",
			assertErr:        trace.IsBadParameter,
		},
		{
			name: "resource: create_time after update_time",
			createDev: func() *devicepb.Device {
				t := time.Now()
				t1 := t.Add(-2 * time.Minute)
				t2 := t.Add(-1 * time.Minute)

				d := proto.Clone(validDev).(*devicepb.Device)
				d.CreateTime = timestamppb.New(t2)
				d.UpdateTime = timestamppb.New(t1)
				return d
			},
			createAsResource: true,
			wantErr:          "create_time cannot be more recent",
			assertErr:        trace.IsBadParameter,
		},
		{
			name: "resource: credential invalid",
			createDev: func() *devicepb.Device {
				d := proto.Clone(validDev).(*devicepb.Device)
				d.Credential = &devicepb.DeviceCredential{
					// Missing all fields.
				}
				return d
			},
			createAsResource: true,
			wantErr:          "credential",
			assertErr:        trace.IsBadParameter,
		},
		{
			name: "resource: collected data invalid",
			createDev: func() *devicepb.Device {
				d := proto.Clone(validDev).(*devicepb.Device)
				d.CollectedData = []*devicepb.DeviceCollectedData{
					// valid
					{
						CollectTime:  timestamppb.Now(),
						RecordTime:   timestamppb.Now(),
						OsType:       d.OsType,
						SerialNumber: d.AssetTag,
					},
					// missing collect_time.
					{
						RecordTime:   timestamppb.Now(),
						OsType:       d.OsType,
						SerialNumber: d.AssetTag,
					},
				}
				return d
			},
			createAsResource: true,
			wantErr:          "collected_data[1]",
			assertErr:        trace.IsBadParameter,
		},
		{
			name: "resource: collected data missing record_time",
			createDev: func() *devicepb.Device {
				d := proto.Clone(validDev).(*devicepb.Device)
				d.CollectedData = []*devicepb.DeviceCollectedData{
					{
						CollectTime: timestamppb.Now(),
						// RecordTime required for resource-like writes.
						OsType:       d.OsType,
						SerialNumber: d.AssetTag,
					},
				}
				return d
			},
			createAsResource: true,
			wantErr:          "collected_data[0]",
			assertErr:        trace.IsBadParameter,
		},
		{
			name: "resource: collected data mismatched",
			createDev: func() *devicepb.Device {
				d := proto.Clone(validDev).(*devicepb.Device)
				d.CollectedData = []*devicepb.DeviceCollectedData{
					{
						CollectTime:  timestamppb.Now(),
						RecordTime:   timestamppb.Now(),
						OsType:       d.OsType,
						SerialNumber: "incorrect-asset-tag",
					},
				}
				return d
			},
			createAsResource: true,
			wantErr:          "serial number mismatch",
			assertErr:        trace.IsBadParameter,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := s.CreateDevice(ctx, test.createDev(), test.createAsResource)
			assert.ErrorContains(t, err, test.wantErr, "CreateDevice error mismatch")
			if !test.assertErr(err) {
				t.Errorf("CreateDevice: assertErr failed: %#v", err)
			}
		})
	}
}

func TestS_UpdateDevice(t *testing.T) {
	env := mustNewEnv()
	defer env.Close()

	s := env.S
	clock := env.Clock
	ctx := context.Background()

	enrolledDev, _, err := createAndEnroll(ctx, s, &devicepb.Device{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "llama",
	})
	if err != nil {
		t.Fatalf("createAndEnroll failed: %v", err)
	}

	assertNoop := func(t *testing.T, base, updated *devicepb.Device) {
		if diff := cmp.Diff(base, updated, protocmp.Transform()); diff != "" {
			t.Errorf("UpdateDevice returned a changed device, want no changes (-want +got)\n%s", diff)
		}
	}

	tests := []struct {
		name         string
		baseDev      *devicepb.Device // baseDevice is created if its ID is empty
		update       func(stored *devicepb.Device) *devicepb.Device
		assertUpdate func(t *testing.T, base, updated *devicepb.Device)
	}{
		{
			name: "noop",
			baseDev: &devicepb.Device{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "llama1",
			},
			update: func(stored *devicepb.Device) *devicepb.Device {
				// No changes.
				return stored
			},
			assertUpdate: assertNoop,
		},
		{
			name: "transient fields ignored",
			baseDev: &devicepb.Device{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "llama2",
			},
			update: func(stored *devicepb.Device) *devicepb.Device {
				now := timestamppb.Now()
				stored.EnrollToken = &devicepb.DeviceEnrollToken{
					Token: "insert enrollment token here",
				}
				stored.CollectedData = []*devicepb.DeviceCollectedData{
					{
						CollectTime:  now,
						RecordTime:   now,
						OsType:       stored.OsType,
						SerialNumber: stored.AssetTag,
					},
				}
				return stored
			},
			assertUpdate: assertNoop,
		},
		{
			name:    "unenroll",
			baseDev: enrolledDev,
			update: func(stored *devicepb.Device) *devicepb.Device {
				stored.EnrollStatus = devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED
				return stored
			},
			assertUpdate: func(t *testing.T, base, updated *devicepb.Device) {
				if proto.Equal(base.UpdateTime, updated.UpdateTime) {
					t.Error("UpdateDevice: update time didn't change")
				}

				want := proto.Clone(base).(*devicepb.Device)
				want.UpdateTime = updated.UpdateTime
				want.EnrollStatus = devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED
				want.Credential = nil // credential automatically cleared
				if diff := cmp.Diff(want, updated, protocmp.Transform()); diff != "" {
					t.Errorf("UpdateDevice mismatch (-want +got)\n%s", diff)
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Create baseDev, if necessary.
			baseDev := test.baseDev
			if baseDev.Id == "" {
				var err error
				baseDev, err = s.CreateDevice(ctx, baseDev, false /* createAsResource */)
				if err != nil {
					t.Fatalf("CreateDevice failed: %v", err)
				}
			}

			// Allow update time to change.
			clock.Advance(1 * time.Second)

			updated, err := s.UpdateDevice(ctx, baseDev.Id, test.update)
			if err != nil {
				t.Fatalf("UpdateDevice failed: %v", err)
			}
			// Has the update time regressed?
			if got, want := updated.UpdateTime.AsTime(), baseDev.UpdateTime.AsTime(); want.After(got) {
				t.Errorf("UpdateDevice: got updated.UpdateTime = %v, want >= %v", got, want)
			}
			test.assertUpdate(t, baseDev, updated)

			// Verify stored device.
			stored, err := s.GetDeviceByID(ctx, updated.Id)
			if err != nil {
				t.Fatalf("GetDeviceByID failed: %v", err)
			}
			stored.CollectedData = nil // not returned by UpdateDevice
			if diff := cmp.Diff(updated, stored, protocmp.Transform()); diff != "" {
				t.Errorf("GetDeviceByID mismatch (-want +got)\n%s", diff)
			}
		})
	}
}

func TestS_UpdateDevice_errors(t *testing.T) {
	env := mustNewEnv()
	defer env.Close()

	s := env.S
	ctx := context.Background()

	baseDev, err := s.CreateDevice(ctx, &devicepb.Device{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "llama",
	}, false /* createAsResource */)
	if err != nil {
		t.Fatalf("CreateDevice failed: %v", err)
	}

	tests := []struct {
		name      string
		deviceID  string
		update    func(stored *devicepb.Device) *devicepb.Device
		assertErr func(err error) bool // defaults to trace.IsBadParameter
		wantErr   string
	}{
		{
			name:    "deviceID is empty",
			update:  func(stored *devicepb.Device) *devicepb.Device { return stored },
			wantErr: "device ID required",
		},
		{
			name:     "updateFunc is nil",
			deviceID: baseDev.Id,
			update:   nil,
			wantErr:  "updateFunc required",
		},
		{
			name:      "not found",
			deviceID:  "unknown",
			update:    func(stored *devicepb.Device) *devicepb.Device { return stored },
			assertErr: trace.IsNotFound,
			wantErr:   "not found",
		},

		{
			name:     "ApiVersion readonly",
			deviceID: baseDev.Id,
			update: func(stored *devicepb.Device) *devicepb.Device {
				stored.ApiVersion = "v9999"
				return stored
			},
			wantErr: "api_version",
		},
		{
			name:     "Id readonly",
			deviceID: baseDev.Id,
			update: func(stored *devicepb.Device) *devicepb.Device {
				stored.Id = "another Id"
				return stored
			},
			wantErr: "id is readonly",
		},
		{
			name:     "OsType readonly",
			deviceID: baseDev.Id,
			update: func(stored *devicepb.Device) *devicepb.Device {
				stored.OsType = devicepb.OSType_OS_TYPE_WINDOWS
				return stored
			},
			wantErr: "os_type",
		},
		{
			name:     "AssetTag readonly",
			deviceID: baseDev.Id,
			update: func(stored *devicepb.Device) *devicepb.Device {
				stored.AssetTag = "another tag"
				return stored
			},
			wantErr: "asset_tag",
		},
		{
			name:     "CreateTime readonly",
			deviceID: baseDev.Id,
			update: func(stored *devicepb.Device) *devicepb.Device {
				stored.CreateTime = &timestamppb.Timestamp{
					Seconds: stored.CreateTime.Seconds - 2, // move to the past, so CreateTime <= UpdateTime
					Nanos:   stored.CreateTime.Nanos,
				}
				return stored
			},
			wantErr: "create_time",
		},
		{
			name:     "UpdateTime readonly",
			deviceID: baseDev.Id,
			update: func(stored *devicepb.Device) *devicepb.Device {
				stored.UpdateTime = &timestamppb.Timestamp{
					Seconds: stored.UpdateTime.Seconds + 2, // move to the future, so CreateTime <= UpdateTime
					Nanos:   stored.UpdateTime.Nanos,
				}
				return stored
			},
			wantErr: "update_time",
		},
		{
			name:     "Credential readonly",
			deviceID: baseDev.Id,
			update: func(stored *devicepb.Device) *devicepb.Device {
				stored.Credential = &devicepb.DeviceCredential{
					Id:           uuid.NewString(),
					PublicKeyDer: []byte("insert public key here"),
				}
				return stored
			},
			wantErr: "credential",
		},
		{
			name:     "EnrollStatus can't transition to UNSPECIFIED",
			deviceID: baseDev.Id,
			update: func(stored *devicepb.Device) *devicepb.Device {
				stored.EnrollStatus = devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_UNSPECIFIED
				return stored
			},
			wantErr: "enroll_status",
		},
		{
			name:     "EnrollStatus can't transition to ENROLLED",
			deviceID: baseDev.Id,
			update: func(stored *devicepb.Device) *devicepb.Device {
				stored.EnrollStatus = devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_ENROLLED
				return stored
			},
			wantErr: "enroll_status",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := s.UpdateDevice(ctx, test.deviceID, test.update)
			if err == nil {
				t.Fatal("UpdateDevice returned err=nil, want non=nil")
			}

			assertErr := test.assertErr
			if assertErr == nil {
				assertErr = trace.IsBadParameter
			}
			if !assertErr(err) {
				t.Errorf("UpdateDevice: assertErr failed, err=%v (%T)", err, err)
			}

			assert.ErrorContains(t, err, test.wantErr, "UpdateDevice error mismatch")
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
		}, false /* createAsResource */)
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
// have undesired side effects in devices with similar asset tags.
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
		created, err := s.CreateDevice(ctx, *dev, false /* createAsResource */)
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
		}, false /* createAsResource */)
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
	}, false /* createAsResource */)
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
	}, false /* createAsResource */)
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
			name:     "credential ID length",
			deviceID: dev.Id,
			createCred: func() *devicepb.DeviceCredential {
				cp := proto.Clone(validCred).(*devicepb.DeviceCredential)
				cp.Id = strings.Repeat("A", 41)
				return cp
			},
			createCD:  func() *devicepb.DeviceCollectedData { return validCD },
			assertErr: trace.IsBadParameter,
			wantErr:   "credential ID exceeds",
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
			name:       "collectedData SerialNumber empty",
			deviceID:   dev.Id,
			createCred: func() *devicepb.DeviceCredential { return validCred },
			createCD: func() *devicepb.DeviceCollectedData {
				cp := proto.Clone(validCD).(*devicepb.DeviceCollectedData)
				cp.SerialNumber = ""
				return cp
			},
			assertErr: trace.IsBadParameter,
			wantErr:   "serial number required",
		},
		{
			name:       "collectedData SerialNumber length",
			deviceID:   dev.Id,
			createCred: func() *devicepb.DeviceCredential { return validCred },
			createCD: func() *devicepb.DeviceCollectedData {
				cp := proto.Clone(validCD).(*devicepb.DeviceCollectedData)
				cp.SerialNumber = strings.Repeat("A", 41)
				return cp
			},
			assertErr: trace.IsBadParameter,
			wantErr:   "serial number exceeds",
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

func TestS_DeviceCollectedData_crud(t *testing.T) {
	env := mustNewEnv()
	defer env.Close()

	s := env.S
	ctx := context.Background()

	clock := env.Clock
	clockAdvance := func() {
		clock.Advance(1 * time.Second)
	}

	// Use a couple of distinct enrolled devices.
	dev1, _, err := createAndEnroll(ctx, s, &devicepb.Device{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "llama",
	})
	if err != nil {
		t.Fatalf("createAndEnroll failed: %v", err)
	}
	clockAdvance()
	dev2, _, err := createAndEnroll(ctx, s, &devicepb.Device{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "alpaca",
	})
	if err != nil {
		t.Fatalf("createAndEnroll failed: %v", err)
	}

	// Write additional collected data for each device.
	// dev1 has a total of 4 data: 1 for enroll and 3 for authn.
	// dev2 has a total of 6 data: 1 for enroll and 5 for authn.
	const dev1WantData = 4
	const dev2WantData = 6
	for _, item := range []struct {
		dev *devicepb.Device
		num int
	}{
		{dev: dev1, num: dev1WantData - 1},
		{dev: dev2, num: dev2WantData - 1},
	} {
		for i := 0; i < item.num; i++ {
			clockAdvance()
			if err := s.RecordDeviceAuthnData(ctx, item.dev.Id, collectedDataForDevice(item.dev)); err != nil {
				t.Fatalf("RecordDeviceAuthnData failed: %v", err)
			}
		}
	}
	devToWantData := map[*devicepb.Device]int{
		dev1: dev1WantData,
		dev2: dev2WantData,
	}

	// Verify Get* reads.
	for dev, wantData := range devToWantData {
		// GetDeviceByID returns collected data.
		t.Run(fmt.Sprintf("GetByID(%v)", dev.AssetTag), func(t *testing.T) {
			got, err := s.GetDeviceByID(ctx, dev.Id)
			if err != nil {
				t.Fatalf("GetDeviceByID failed: %v", err)
			}
			if gotData := len(got.CollectedData); gotData != wantData {
				t.Fatalf("Got %v collected data instances, want %v", gotData, wantData)
			}

			// Verify collected data instances.
			for _, cd := range got.CollectedData {
				if cd.CollectTime == nil {
					t.Error("Got cd.CollectTime = nil, want non=nil")
				}
				if cd.RecordTime == nil {
					t.Error("Got cd.RecordTime = nil, want non=nil")
				}
				wantCD := &devicepb.DeviceCollectedData{
					CollectTime:  cd.CollectTime,
					RecordTime:   cd.RecordTime,
					OsType:       dev.OsType,
					SerialNumber: dev.AssetTag,
				}
				if diff := cmp.Diff(wantCD, cd, protocmp.Transform()); diff != "" {
					t.Errorf("CollectedData mismatch (-want +got):\n%s", diff)
				}
			}

			// Verify that timestamps are in order.
			prev := got.CollectedData[0].RecordTime.AsTime()
			for _, cd := range got.CollectedData[1:] {
				curr := cd.RecordTime.AsTime()
				if prev.After(curr) {
					t.Errorf("Got out-of-order collected data instances, want from oldest to newest: %v", got.CollectedData)
					break
				}
				prev = curr
			}
		})

		// GetByAssetTag returns collected data.
		t.Run(fmt.Sprintf("GetByAssetTag(%v)", dev.AssetTag), func(t *testing.T) {
			devs, err := s.GetDevicesByAssetTag(ctx, dev.AssetTag)
			if err != nil {
				t.Fatalf("GetDevicesByAssetTag failed: %v", err)
			}
			got := devs[0]
			if gotData := len(got.CollectedData); gotData != wantData {
				t.Errorf("Got %v collected data instances, want %v", gotData, wantData)
			}
		})
	}

	// LIST view returns no collected data.
	t.Run("List with LIST view", func(t *testing.T) {
		devs, _, err := s.ListDevices(ctx, 100 /* pageSize */, "" /* pageToken */, devicepb.DeviceView_DEVICE_VIEW_LIST)
		if err != nil {
			t.Fatalf("ListDevices failed: %v", err)
		}
		for _, got := range devs {
			if gotData := len(got.CollectedData); gotData != 0 {
				t.Errorf("Device %v: got %v collected data instances, want zero", got.AssetTag, gotData)
			}
		}
	})

	// RESOURCE view returns collected data.
	t.Run("List with RESOURCE view", func(t *testing.T) {
		devs, _, err := s.ListDevices(ctx, 100 /* pageSize */, "" /* pageToken */, devicepb.DeviceView_DEVICE_VIEW_RESOURCE)
		if err != nil {
			t.Fatalf("ListDevices failed: %v", err)
		}
		for _, got := range devs {
			var want int
			for dev, wantData := range devToWantData {
				if dev.Id == got.Id {
					want = wantData
					break
				}
			}
			if gotData := len(got.CollectedData); gotData != want {
				t.Errorf("Device %v: got %v collected data instances, want %v", got.AssetTag, gotData, want)
			}
		}
	})

	// Writing too many collected data instances deletes the older entries.
	t.Run("RecordDeviceAuthnData replaces older entries", func(t *testing.T) {
		// Find out the latest timestamp.
		got, err := s.GetDeviceByID(ctx, dev1.Id)
		if err != nil {
			t.Fatalf("GetDeviceByID failed: %v", err)
		}
		first := got.CollectedData[0].RecordTime.AsTime()
		last := got.CollectedData[len(got.CollectedData)-1].RecordTime.AsTime()

		// Write up to MaxCollectedDataPerDevice and then a bit more, so we know the
		// cap keeps working.
		for i := 0; i < storage.MaxCollectedDataPerDevice+2; i++ {
			clockAdvance()
			if err := s.RecordDeviceAuthnData(ctx, dev1.Id, collectedDataForDevice(dev1)); err != nil {
				t.Fatalf("RecordDeviceAuthnData failed: %v", err)
			}
		}

		// Collected data number got capped?
		got, err = s.GetDeviceByID(ctx, dev1.Id)
		if err != nil {
			t.Fatalf("GetDeviceByID failed: %v", err)
		}
		if gotData, want := len(got.CollectedData), storage.MaxCollectedDataPerDevice+1; gotData != want {
			t.Fatalf("Got %v collected data instances, want %v", gotData, want)
		}

		// Enrollment entry preserved (it is now the oldest).
		if ts := got.CollectedData[0].RecordTime.AsTime(); ts != first {
			t.Error("Enrollment data not preserved during collected data trim")
		}
		// All older entries deleted.
		if ts := got.CollectedData[1].RecordTime.AsTime(); !ts.After(last) {
			t.Errorf("Unexpected RecordTime after collected data trim, got %v, want > %v", ts, last)
		}
	})

	// Finally, delete a device that has collected data to verify that it works.
	t.Run("DeleteDevice", func(t *testing.T) {
		if err := s.DeleteDevice(ctx, dev1.Id); err != nil {
			t.Fatalf("DeleteDevice failed: %v", err)
		}

		// Collected data cannot be found in storage.
		storedCD, err := s.GetDeviceCollecteDataForTests(ctx, dev1.Id)
		switch {
		case err != nil:
			t.Errorf("GetDeviceCollecteDataForTests failed: %v", err)
		case len(storedCD) > 0:
			t.Errorf("GetDeviceCollecteDataForTests return %v collected data instances, want zero", len(storedCD))
		}

		// Unrelated device is intact.
		got, err := s.GetDeviceByID(ctx, dev2.Id)
		if err != nil {
			t.Fatalf("GetDeviceByID failed: %v", err)
		}
		if gotData, want := len(got.CollectedData), dev2WantData; gotData != want {
			t.Errorf("Got %v collected data instances, want %v", gotData, want)
		}
	})
}

func createAndEnroll(ctx context.Context, s *storage.S, dev *devicepb.Device) (*devicepb.Device, crypto.PrivateKey, error) {
	dev, err := s.CreateDevice(ctx, dev, false /* createAsResource */)
	if err != nil {
		return nil, nil, fmt.Errorf("calling CreateDevice: %v", err)
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("calling GenerateKey: %v", err)
	}
	pubKeyDER, err := x509.MarshalPKIXPublicKey(key.Public())
	if err != nil {
		return nil, nil, fmt.Errorf("calling MarshalPKIXPublicKey: %v", err)
	}
	cred := &devicepb.DeviceCredential{
		Id:           uuid.NewString(),
		PublicKeyDer: pubKeyDER,
	}

	dev, err = s.EnrollDevice(ctx, dev.Id, cred, collectedDataForDevice(dev))
	if err != nil {
		return nil, nil, fmt.Errorf("calling EnrollDevice: %v", err)
	}
	return dev, key, nil
}

func collectedDataForDevice(dev *devicepb.Device) *devicepb.DeviceCollectedData {
	return &devicepb.DeviceCollectedData{
		CollectTime:  timestamppb.Now(),
		OsType:       dev.OsType,
		SerialNumber: dev.AssetTag,
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
		}, false /* createAsResource */)
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

	s, err := storage.New(mem)
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
