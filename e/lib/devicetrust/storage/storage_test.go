package storage_test

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/devicetrust/storage"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services/local"
)

const (
	sampleUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36"
	sampleIP        = "40.89.244.232"
)

func TestS_BulkCreateDevices(t *testing.T) {
	t.Parallel()

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
		Owner: "llama",
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
	t.Parallel()

	env := mustNewEnv()
	defer env.Close()

	s := env.S
	ctx := context.Background()

	const resource1Tag = "AAA000000000"
	const resource2Tag = "BBB000000000"

	_, pubKeyDER := newKeyPair(t)

	resource1Dev := &devicepb.Device{
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
		// CollectedData is ignored on Create.
		CollectedData: []*devicepb.DeviceCollectedData{
			{
				CollectTime:  timestamppb.New(time.Date(2023, 2, 24, 19, 0, 5, 0, time.UTC)),
				RecordTime:   timestamppb.New(time.Date(2023, 2, 24, 19, 0, 5, 500, time.UTC)),
				OsType:       devicepb.OSType_OS_TYPE_MACOS,
				SerialNumber: resource1Tag,
			},
			{
				CollectTime:  nil,                           // invalid
				RecordTime:   nil,                           // invalid for a resource
				OsType:       devicepb.OSType_OS_TYPE_LINUX, // invalid
				SerialNumber: "ignored",                     // invalid
			},
		},
		Source: &devicepb.DeviceSource{
			Name:   "myscript",
			Origin: devicepb.DeviceOrigin_DEVICE_ORIGIN_API,
		},
		Profile: &devicepb.DeviceProfile{
			UpdateTime:          timestamppb.Now(),
			ModelIdentifier:     "MacBookPro9,2",
			OsVersion:           "13.4.1",
			OsBuild:             "22F82",
			OsBuildSupplemental: "22F770820d",
			OsUsernames:         []string{"admin", "codingllama", "alpaca"},
			JamfBinaryVersion:   "10.44.1-t1677509507",
			ExternalId:          "99",
		},
		Owner: "llama",
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
				AssetTag: "dev1",
			},
		},
		{
			name: "ok with source and profile",
			dev: &devicepb.Device{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "dev2",
				Source: &devicepb.DeviceSource{
					Name:   "mysource",
					Origin: devicepb.DeviceOrigin_DEVICE_ORIGIN_API,
				},
				Profile: &devicepb.DeviceProfile{
					ModelIdentifier:   "MacBookPro9,2",
					OsVersion:         "13.2.1",
					OsBuild:           "22D68",
					OsUsernames:       []string{"admin", "alpaca"},
					JamfBinaryVersion: "10.44.1",
					ExternalId:        "99",
				},
			},
		},
		{
			name: "ok with supplemental build",
			dev: &devicepb.Device{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "devSup1",
				Profile: &devicepb.DeviceProfile{
					OsVersion:           "13.4.1",
					OsBuild:             "22F82",
					OsBuildSupplemental: "22F770820d",
				},
			},
		},
		{
			name:             "create as a resource",
			dev:              resource1Dev,
			createAsResource: true,
			modifyWant: func(dev, want *devicepb.Device) {
				// All fields that are typically system-generated are copied from `dev`.
				want.Id = dev.Id
				want.CreateTime = dev.CreateTime
				want.UpdateTime = dev.UpdateTime
				want.EnrollStatus = dev.EnrollStatus
				want.Credential = dev.Credential
				want.Owner = dev.Owner

				// Ignored on pure Create/Update methods.
				want.CollectedData = nil
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
				Source:       test.dev.Source,
				Profile:      test.dev.Profile,
			}
			if want.Profile != nil {
				want.Profile.UpdateTime = got.Profile.GetUpdateTime() // System-managed
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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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

	validProfile := &devicepb.DeviceProfile{
		ModelIdentifier:   "MacBookPro9,3",
		OsVersion:         "13.2.1",
		OsBuild:           "22D68",
		OsUsernames:       []string{"admin", "llama"},
		JamfBinaryVersion: "9.27",
	}

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
				d.AssetTag = strings.Repeat("A", 121)
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
			name: "source.name empty",
			createDev: func() *devicepb.Device {
				d := proto.Clone(validDev).(*devicepb.Device)
				d.Source = &devicepb.DeviceSource{
					Origin: devicepb.DeviceOrigin_DEVICE_ORIGIN_JAMF,
				}
				return d
			},
			wantErr:   "source name",
			assertErr: trace.IsBadParameter,
		},
		{
			name: "source.origin unspecified",
			createDev: func() *devicepb.Device {
				d := proto.Clone(validDev).(*devicepb.Device)
				d.Source = &devicepb.DeviceSource{
					Name: "mysource",
				}
				return d
			},
			wantErr:   "source origin",
			assertErr: trace.IsBadParameter,
		},
		{
			name: "profile.os_version macOS not a semver",
			createDev: func() *devicepb.Device {
				p := proto.Clone(validProfile).(*devicepb.DeviceProfile)
				p.OsVersion = "NOT A SEMVER"

				d := proto.Clone(validDev).(*devicepb.Device)
				d.OsType = devicepb.OSType_OS_TYPE_MACOS // important for this test
				d.Profile = p
				return d
			},
			wantErr:   "not a valid semver",
			assertErr: trace.IsBadParameter,
		},
		{
			name: "profile.os_usernames empty value",
			createDev: func() *devicepb.Device {
				p := proto.Clone(validProfile).(*devicepb.DeviceProfile)
				p.OsUsernames = []string{
					"admin",
					"", // invalid
					"alpaca",
				}

				d := proto.Clone(validDev).(*devicepb.Device)
				d.Profile = p
				return d
			},
			wantErr:   "username",
			assertErr: trace.IsBadParameter,
		},
		{
			name: "profile.jamf_binary_version not a semver",
			createDev: func() *devicepb.Device {
				p := proto.Clone(validProfile).(*devicepb.DeviceProfile)
				p.JamfBinaryVersion = "NOT A SEMVER"

				d := proto.Clone(validDev).(*devicepb.Device)
				d.Profile = p
				return d
			},
			wantErr:   "not a valid semver",
			assertErr: trace.IsBadParameter,
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
	t.Parallel()

	env := mustNewEnv()
	defer env.Close()

	s := env.S
	clock := env.Clock
	ctx := context.Background()

	const owner = "llama"
	enrolledDev, _, err := createAndEnroll(ctx, s, &devicepb.Device{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "llama",
	}, owner)
	if err != nil {
		t.Fatalf("createAndEnroll failed: %v", err)
	}

	validSource := &devicepb.DeviceSource{
		Name:   "jamf",
		Origin: devicepb.DeviceOrigin_DEVICE_ORIGIN_JAMF,
	}
	validProfile := &devicepb.DeviceProfile{
		ModelIdentifier:   "MacBookPro9,2",
		OsVersion:         "13.2.1",
		OsBuild:           "22D68",
		OsUsernames:       []string{"alpaca"},
		JamfBinaryVersion: "10.44.1-t1677509507",
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
				want.Owner = ""       // owner automatically cleared
				if diff := cmp.Diff(want, updated, protocmp.Transform()); diff != "" {
					t.Errorf("UpdateDevice mismatch (-want +got)\n%s", diff)
				}
			},
		},
		{
			name: "set source and profile",
			baseDev: &devicepb.Device{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "mdmfields1",
			},
			update: func(stored *devicepb.Device) *devicepb.Device {
				stored.Source = validSource
				stored.Profile = validProfile
				return stored
			},
			assertUpdate: func(t *testing.T, base, updated *devicepb.Device) {
				p := proto.Clone(validProfile).(*devicepb.DeviceProfile)

				// Copy system-assigned profile update time.
				if updated.Profile == nil || updated.Profile.UpdateTime == nil {
					t.Error("UpdateDevice: profile UpdateTime not assigned")
				} else {
					p.UpdateTime = updated.Profile.UpdateTime
				}

				want := proto.Clone(updated).(*devicepb.Device)
				want.Source = validSource
				want.Profile = p

				if diff := cmp.Diff(want, updated, protocmp.Transform()); diff != "" {
					t.Errorf("UpdateDevice mismatch (-want +got)\n%s", diff)
				}
			},
		},
		{
			name: "DeviceProfile.UpdateTime ignored for noop",
			baseDev: &devicepb.Device{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "mdmfields2",
				Profile: &devicepb.DeviceProfile{
					ModelIdentifier: "MacBookPro9,2",
				},
			},
			update: func(stored *devicepb.Device) *devicepb.Device {
				// nil UpdateTime is allowed.
				// Everything else is the same, so this shouldn't cause an update.
				stored.Profile.UpdateTime = nil
				return stored
			},
			assertUpdate: assertNoop,
		},
		{
			name: "Added empty profile ignored",
			baseDev: &devicepb.Device{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "mdmfields3",
			},
			update: func(stored *devicepb.Device) *devicepb.Device {
				stored.Profile = &devicepb.DeviceProfile{
					// UpdateTime, by itself, doesn't qualify the profile as non-empty.
					UpdateTime: timestamppb.Now(),
				}
				return stored
			},
			assertUpdate: assertNoop,
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
			cd := stored.CollectedData
			stored.CollectedData = nil // not returned by UpdateDevice
			if diff := cmp.Diff(updated, stored, protocmp.Transform()); diff != "" {
				t.Errorf("GetDeviceByID mismatch (-want +got)\n%s", diff)
			}

			// Verify collected data deletion on unenroll.
			if updated.EnrollStatus == devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED && len(cd) > 0 {
				t.Errorf("Unenrolled device has collected data=%v, want none", cd)
			}
		})
	}
}

func TestS_UpdateDevice_errors(t *testing.T) {
	t.Parallel()

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
			name:      "deviceID is empty",
			update:    func(stored *devicepb.Device) *devicepb.Device { return stored },
			wantErr:   "device ID required",
			assertErr: trace.IsBadParameter,
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
		{
			name:     "source is validated",
			deviceID: baseDev.Id,
			update: func(stored *devicepb.Device) *devicepb.Device {
				stored.Source = &devicepb.DeviceSource{
					Origin: devicepb.DeviceOrigin_DEVICE_ORIGIN_JAMF,
				}
				return stored
			},
			wantErr: "source name",
		},
		{
			name:     "profile is validated",
			deviceID: baseDev.Id,
			update: func(stored *devicepb.Device) *devicepb.Device {
				stored.Profile = &devicepb.DeviceProfile{
					JamfBinaryVersion: "NOT A SEMVER",
				}
				return stored
			},
			wantErr: "jamf binary version",
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

func TestS_DeleteDevicePredicate(t *testing.T) {
	t.Parallel()

	env := mustNewEnv()
	defer env.Close()

	s := env.S
	ctx := context.Background()

	const createAsResource = false
	llama, err := s.CreateDevice(ctx, &devicepb.Device{OsType: devicepb.OSType_OS_TYPE_MACOS, AssetTag: "llama"}, createAsResource)
	if err != nil {
		t.Fatalf("CreateDevice failed: %v", err)
	}
	dev1, err := s.CreateDevice(ctx, &devicepb.Device{OsType: devicepb.OSType_OS_TYPE_MACOS, AssetTag: "dev1"}, createAsResource)
	if err != nil {
		t.Fatalf("CreateDevice failed: %v", err)
	}

	// See TestS_DeleteDevice for more extensive testing.
	tests := []struct {
		name      string
		deviceID  string
		predicate func(d *devicepb.Device) error
		wantErr   string
	}{
		{
			name:      "predicate matches",
			deviceID:  llama.Id,
			predicate: func(_ *devicepb.Device) error { return nil },
		},
		{
			name:      "predicate doesn't match",
			deviceID:  dev1.Id,
			predicate: func(_ *devicepb.Device) error { return errors.New("please don't delete me") },
			wantErr:   "please don't delete me",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := s.DeleteDevicePredicate(ctx, test.deviceID, test.predicate)
			if test.wantErr != "" {
				assert.ErrorContains(t, err, test.wantErr, "DeleteDevicePredicate error mismatch")
			} else if err != nil {
				t.Errorf("DeleteDevicePredicate returned err=%v, want nil", err)
			}

			// Verify deletion.
			_, err = s.GetDeviceByID(ctx, test.deviceID)
			if err != nil && !trace.IsNotFound(err) {
				t.Fatalf("GetDeviceByID returned unexpected error: %v", err)
			}

			gotDeleted := err != nil // NotFound means deleted
			wantDeleted := test.wantErr == ""
			if gotDeleted != wantDeleted {
				t.Errorf("DeleteDevicePredicate: device deleted=%v, want %v", gotDeleted, wantDeleted)
			}
		})
	}
}

func TestS_DeleteDevice(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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

func TestS_GetDeviceIDsByOSTag(t *testing.T) {
	t.Parallel()

	env := mustNewEnv()
	defer env.Close()

	s := env.S
	ctx := context.Background()

	// Create a few devices that we can query later.
	const createAsResource = false
	var allDevices []*devicepb.Device
	for _, dev := range []*devicepb.Device{
		{OsType: devicepb.OSType_OS_TYPE_MACOS, AssetTag: "llama"},
		{OsType: devicepb.OSType_OS_TYPE_WINDOWS, AssetTag: "llama"},
		{OsType: devicepb.OSType_OS_TYPE_MACOS, AssetTag: "alpaca"},
		{OsType: devicepb.OSType_OS_TYPE_MACOS, AssetTag: "dev1"},
		{OsType: devicepb.OSType_OS_TYPE_MACOS, AssetTag: "dev2"},
	} {
		created, err := s.CreateDevice(ctx, dev, createAsResource)
		if err != nil {
			t.Fatalf("CreateDevice failed: %v", err)
		}
		allDevices = append(allDevices, created)
	}
	llama := allDevices[0]
	llamaWin := allDevices[1]
	alpaca := allDevices[2]
	// dev1 unused
	dev2 := allDevices[4]

	// Simulate a bad asset tag mapping by deleting the device directly from
	// storage.
	// No normal storage.S interaction will get us into this state.
	if err := env.mem.Delete(ctx, backend.Key("devices", "id", dev2.Id)); err != nil {
		t.Fatalf("Direct deletion of %q failed: %v", dev2.AssetTag, err)
	}

	tests := []struct {
		name            string
		osType          devicepb.OSType
		assetTag        string
		verifyExistence bool
		wantID          string
		wantErr         string               // assert error message
		assertErr       func(err error) bool // assert error type
	}{
		{
			name:     "ok",
			osType:   alpaca.OsType,
			assetTag: alpaca.AssetTag,
			wantID:   alpaca.Id,
		},
		{
			name:            "ok with verifyExistence=true",
			osType:          alpaca.OsType,
			assetTag:        alpaca.AssetTag,
			verifyExistence: true,
			wantID:          alpaca.Id,
		},
		{
			name:     "macOS with conflicting asset tag",
			osType:   llama.OsType,
			assetTag: llama.AssetTag, // same AssetTag as llamaWin
			wantID:   llama.Id,
		},
		{
			name:     "Windows with conflicting asset tag",
			osType:   llamaWin.OsType,
			assetTag: llamaWin.AssetTag, // same AssetTag as llama
			wantID:   llamaWin.Id,
		},
		{
			name:      "unknown os_type not found",
			osType:    devicepb.OSType_OS_TYPE_LINUX,
			assetTag:  llama.AssetTag,
			wantErr:   "not found",
			assertErr: trace.IsNotFound,
		},
		{
			name:      "unknown tag not found",
			osType:    devicepb.OSType_OS_TYPE_MACOS,
			assetTag:  "unknown",
			wantErr:   "not found",
			assertErr: trace.IsNotFound,
		},
		{
			name:     "verifyExistence=false succeeds if mapping exists",
			osType:   dev2.OsType,
			assetTag: dev2.AssetTag,
			wantID:   dev2.Id,
		},
		{
			name:            "verifyExistence=true fails if only mapping exist",
			osType:          dev2.OsType,
			assetTag:        dev2.AssetTag,
			verifyExistence: true,
			wantErr:         "not found",
			assertErr:       trace.IsNotFound,
		},
		{
			name:      "OSType unspecified",
			osType:    0,
			assetTag:  "llama",
			wantErr:   "os type",
			assertErr: trace.IsBadParameter,
		},
		{
			name:      "AssetTag empty",
			osType:    devicepb.OSType_OS_TYPE_MACOS,
			assetTag:  "",
			wantErr:   "asset tag",
			assertErr: trace.IsBadParameter,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := s.GetDeviceIDByOSTag(ctx, test.osType, test.assetTag, test.verifyExistence)
			if test.wantErr != "" {
				assert.ErrorContains(t, err, test.wantErr, "GetDeviceIDByOSTag error mismatch")
			} else if err != nil {
				t.Errorf("GetDeviceIDByOSTag failed: %v", err)
			}

			if test.assertErr != nil && !test.assertErr(err) {
				t.Errorf("GetDeviceIDByOSTag: assertErr failed, err=%q (%T)", err, err)
			}

			// Safe to do even on failures, ID is supposed to be empty.
			if test.wantID != got {
				t.Errorf("GetDeviceIDByOSTag() = %v, want = %v", got, test.wantID)
			}
		})
	}
}

func TestS_GetDevicesByAssetTag_errors(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
		if _, err := s.CreateDeviceEnrollToken(ctx, deviceID, time.Time{} /* expiresAt */); err != nil {
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
	t.Parallel()

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
	t.Parallel()

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
		owner   string
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
			owner: "llama",
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
			owner: "alpaca",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			deviceID := test.baseDev.Id
			got, err := s.EnrollDevice(ctx, deviceID, test.cred, test.cd, test.owner)
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
			want.Owner = test.owner
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
		})
	}
}

func TestS_EnrollDevice_reEnroll(t *testing.T) {
	t.Parallel()

	env := mustNewEnv()
	defer env.Close()

	s := env.S
	clock := env.Clock
	ctx := context.Background()

	const user1 = "llama"
	const user2 = "llamaer"

	// Device is created and enrolled.
	dev, _, err := createAndEnroll(ctx, s, &devicepb.Device{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "llama1",
	}, user1)
	if err != nil {
		t.Fatalf("createAndEnroll failed: %v", err)
	}
	clock.Advance(1 * time.Second)

	// Record some additional data for good measure.
	if err := s.RecordDeviceAuthnData(ctx, dev.Id, collectedDataForDevice(dev)); err != nil {
		t.Fatalf("RecordDeviceAuthnData failed: %v", err)
	}
	clock.Advance(1 * time.Second)
	if err := s.RecordDeviceAuthnData(ctx, dev.Id, collectedDataForDevice(dev)); err != nil {
		t.Fatalf("RecordDeviceAuthnData failed: %v", err)
	}
	clock.Advance(1 * time.Second)

	assertCD := func(dev *devicepb.Device, want int) {
		dev, err = s.GetDeviceByID(ctx, dev.Id)
		if err != nil {
			t.Fatalf("GetDeviceByID failed: %v", err)
		}
		if got := len(dev.CollectedData); got != want {
			t.Errorf("GetDeviceByID: got %v instances of collected data, want %v", got, want)
		}
	}
	// Sanity check: we collected data 3 times.
	assertCD(dev, 3)

	// Re-enroll.
	dev, _, err = enroll(ctx, s, dev, user2)
	if err != nil {
		t.Fatalf("enroll failed: %v", err)
	}

	// Verify that collected data was "reset".
	assertCD(dev, 1) // enroll data only
}

func TestS_EnrollDevice_errors(t *testing.T) {
	t.Parallel()

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

	isDriftError := func(err error) bool {
		return errors.Is(err, &storage.CollectedDataDriftError{})
	}

	const owner = "llama"

	tests := []struct {
		name       string
		deviceID   string
		createCred func() *devicepb.DeviceCredential
		createCD   func() *devicepb.DeviceCollectedData
		owner      string
		assertErr  func(error) bool
		wantErr    string
	}{
		{
			name:       "unknown device",
			deviceID:   "unknown",
			createCred: func() *devicepb.DeviceCredential { return validCred },
			createCD:   func() *devicepb.DeviceCollectedData { return validCD },
			owner:      owner,
			assertErr:  trace.IsNotFound,
		},
		{
			name:       "credential nil",
			deviceID:   dev.Id,
			createCred: func() *devicepb.DeviceCredential { return nil },
			createCD:   func() *devicepb.DeviceCollectedData { return validCD },
			owner:      owner,
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
			owner:     owner,
			assertErr: trace.IsBadParameter,
			wantErr:   "credential ID",
		},
		{
			name:     "credential ID length",
			deviceID: dev.Id,
			createCred: func() *devicepb.DeviceCredential {
				cp := proto.Clone(validCred).(*devicepb.DeviceCredential)
				cp.Id = strings.Repeat("A", 70)
				return cp
			},
			createCD:  func() *devicepb.DeviceCollectedData { return validCD },
			owner:     owner,
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
			owner:     owner,
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
			owner:     owner,
			assertErr: trace.IsBadParameter,
			wantErr:   "invalid credential public key",
		},
		{
			name:       "collectedData nil",
			deviceID:   dev.Id,
			createCred: func() *devicepb.DeviceCredential { return validCred },
			createCD:   func() *devicepb.DeviceCollectedData { return nil },
			owner:      owner,
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
			owner:     owner,
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
			owner:     owner,
			assertErr: isDriftError,
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
			owner:     owner,
			assertErr: trace.IsBadParameter,
			wantErr:   "serial number required",
		},
		{
			name:       "collectedData SerialNumber length",
			deviceID:   dev.Id,
			createCred: func() *devicepb.DeviceCredential { return validCred },
			createCD: func() *devicepb.DeviceCollectedData {
				cp := proto.Clone(validCD).(*devicepb.DeviceCollectedData)
				cp.SerialNumber = strings.Repeat("A", 121)
				return cp
			},
			owner:     owner,
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
			owner:     owner,
			assertErr: isDriftError,
			wantErr:   "serial number mismatch",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := s.EnrollDevice(ctx, test.deviceID, test.createCred(), test.createCD(), test.owner)
			if !test.assertErr(err) {
				t.Errorf("EnrollDevice: assertErr failed, err=%v", err)
			}
			assert.ErrorContains(t, err, test.wantErr, "EnrollDevice error mismatch")
		})
	}
}

func TestS_DeviceCollectedData_crud(t *testing.T) {
	t.Parallel()

	env := mustNewEnv()
	defer env.Close()

	s := env.S
	ctx := context.Background()

	clock := env.Clock
	clockAdvance := func() {
		clock.Advance(1 * time.Second)
	}

	// Use a couple of distinct enrolled devices.
	const owner = "llama"
	dev1, _, err := createAndEnroll(ctx, s, &devicepb.Device{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "llama",
	}, owner)
	if err != nil {
		t.Fatalf("createAndEnroll failed: %v", err)
	}
	clockAdvance()
	dev2, _, err := createAndEnroll(ctx, s, &devicepb.Device{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "alpaca",
	}, owner)
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

func TestS_RecordDeviceAuthnData(t *testing.T) {
	t.Parallel()

	env := mustNewEnv()
	defer env.Close()

	s := env.S
	clock := env.Clock
	ctx := context.Background()

	nowAndAdvance := func() time.Time {
		now := clock.Now()
		clock.Advance(1 * time.Second)
		return now
	}

	// Create test devices.
	const owner = "llama"
	var allDevs []*devicepb.Device
	for _, dev := range []*devicepb.Device{
		{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: "mac",
		},
		{
			OsType:   devicepb.OSType_OS_TYPE_WINDOWS,
			AssetTag: "win",
		},
		{
			OsType:   devicepb.OSType_OS_TYPE_LINUX,
			AssetTag: "linux",
		},
	} {
		created, _, err := createAndEnroll(ctx, s, dev, owner)
		if err != nil {
			t.Fatalf("createAndEnroll(%q) failed: %v", dev.AssetTag, err)
		}
		allDevs = append(allDevs, created)
	}
	macDev := allDevs[0]
	winDev := allDevs[1]
	linuxDev := allDevs[2]

	tests := []struct {
		name     string
		deviceID string
		cd       *devicepb.DeviceCollectedData
	}{
		{
			name:     "minimal",
			deviceID: macDev.Id,
			cd: &devicepb.DeviceCollectedData{
				CollectTime:  timestamppb.New(nowAndAdvance()),
				OsType:       macDev.OsType,
				SerialNumber: macDev.AssetTag,
			},
		},
		{
			name:     "MDM fields",
			deviceID: macDev.Id,
			cd: &devicepb.DeviceCollectedData{
				CollectTime:       timestamppb.New(nowAndAdvance()),
				OsType:            macDev.OsType,
				SerialNumber:      macDev.AssetTag,
				ModelIdentifier:   "MacBookPro14,3",
				OsVersion:         "11.1",
				OsBuild:           "11D11",
				OsUsername:        "alpaca",
				JamfBinaryVersion: "1",
				MacosEnrollmentProfiles: `Enrolled via DEP: No
MDM enrollment: Yes (User Approved)
MDM server: https://example.com/mdm/ServerURL`,
			},
		},
		{
			name:     "TPM fields",
			deviceID: winDev.Id,
			cd: &devicepb.DeviceCollectedData{
				CollectTime:  timestamppb.New(nowAndAdvance()),
				OsType:       winDev.OsType,
				SerialNumber: winDev.AssetTag,
				TpmPlatformAttestation: &devicepb.TPMPlatformAttestation{
					Nonce: []byte("fake-nonce"),
					PlatformParameters: &devicepb.TPMPlatformParameters{
						EventLog: []byte("fake-event-log"),
						Quotes: []*devicepb.TPMQuote{
							{
								Quote:     []byte("fake-quote-0"),
								Signature: []byte("fake-signature-0"),
							},
							{
								Quote:     []byte("fake-quote-1"),
								Signature: []byte("fake-signature-1"),
							},
						},
						Pcrs: []*devicepb.TPMPCR{
							{
								Index:     0,
								Digest:    []byte("fake-sha1-digest"),
								DigestAlg: uint64(crypto.SHA1),
							},
							{
								Index:     1,
								Digest:    []byte("fake-sha256-digest"),
								DigestAlg: uint64(crypto.SHA256),
							},
						},
					},
				},
			},
		},
		{
			name:     "Linux",
			deviceID: linuxDev.Id,
			cd: &devicepb.DeviceCollectedData{
				CollectTime:           timestamppb.New(nowAndAdvance()),
				OsType:                devicepb.OSType_OS_TYPE_LINUX,
				SerialNumber:          linuxDev.AssetTag,
				ModelIdentifier:       "21J50013US",
				OsVersion:             "22.04",
				OsBuild:               "22.04 LTS (Jammy Jellyfish)",
				OsUsername:            "llama",
				ReportedAssetTag:      "No Asset Information",
				SystemSerialNumber:    linuxDev.AssetTag,
				BaseBoardSerialNumber: "L1AA00A00A0",
				OsId:                  "ubuntu",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cd := test.cd

			// Write a new collected data instance.
			clock.Advance(1 * time.Second)
			if err := s.RecordDeviceAuthnData(ctx, test.deviceID, cd); err != nil {
				t.Fatalf("RecordDeviceAuthnData failed: %v", err)
			}

			// Verify stored collected data.
			stored, err := s.GetDeviceByID(ctx, test.deviceID)
			if err != nil {
				t.Fatalf("GetDeviceByID failed: %v", err)
			}
			if got, want := len(stored.CollectedData), 1; got < want {
				t.Fatalf("GetDeviceByID: got %v collected data instances, want>=%v", got, want)
			}

			// Last recorded entry must be the one above
			got := stored.CollectedData[len(stored.CollectedData)-1]
			want := cd
			want.RecordTime = got.RecordTime // System-managed
			if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
				t.Errorf("Collected data mismatch (-want +got)\n%s", diff)
			}
		})
	}
}

func TestS_RecordDeviceAuthnData_errors(t *testing.T) {
	t.Parallel()

	env := mustNewEnv()
	defer env.Close()

	s := env.S
	clock := env.Clock
	ctx := context.Background()

	const owner = "llama"
	devWithProfile, _, err := createAndEnroll(ctx, s, &devicepb.Device{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "alpaca",
		Profile: &devicepb.DeviceProfile{
			ModelIdentifier:   "MacBookPro9,2",
			OsVersion:         "13.3.1",
			OsBuild:           "22E261",
			OsUsernames:       []string{"admin", "llama"},
			JamfBinaryVersion: "10.45.0-t1678116779",
		},
	}, owner)
	if err != nil {
		t.Fatalf("createAndEnroll failed: %v", err)
	}

	devWithData, err := s.CreateDevice(ctx, &devicepb.Device{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "llama",
	}, false /* createAsResource */)
	if err != nil {
		t.Fatalf("CreateDevice failed: %v", err)
	}

	// Record a couple of CD instances that evolve over time as the basis for
	// testing.
	cd1 := &devicepb.DeviceCollectedData{
		OsType:            devWithData.OsType,
		SerialNumber:      devWithData.AssetTag,
		ModelIdentifier:   "MacBookPro9,2",
		OsVersion:         "13.2.1",
		OsBuild:           "22D68",
		OsUsername:        "llama",
		JamfBinaryVersion: "9.27",
	}
	cd2 := &devicepb.DeviceCollectedData{
		OsType:            devWithData.OsType,
		SerialNumber:      devWithData.AssetTag,
		ModelIdentifier:   "MacBookPro9,2",
		OsVersion:         "13.3",
		OsBuild:           "22E260",
		OsUsername:        "llama",
		JamfBinaryVersion: "10.44.1-t1677509507",
	}
	for _, cd := range []*devicepb.DeviceCollectedData{cd1, cd2} {
		clock.Advance(1 * time.Second)
		cd.CollectTime = timestamppb.New(clock.Now())
		if err := s.RecordDeviceAuthnData(ctx, devWithData.Id, cd); err != nil {
			t.Fatalf("RecordDeviceAuthnData failed: %v", err)
		}
	}

	// Prepare devices for a few Linux-specific data drift scenarios.
	linuxWithData := &devicepb.Device{
		OsType:   devicepb.OSType_OS_TYPE_LINUX,
		AssetTag: "linux1",
	}
	linuxWithProfile := &devicepb.Device{
		OsType:   devicepb.OSType_OS_TYPE_LINUX,
		AssetTag: "linux2",
		Profile: &devicepb.DeviceProfile{
			OsId: "ubuntu",
		},
	}
	for _, d := range []**devicepb.Device{&linuxWithData, &linuxWithProfile} {
		var err error
		*d, err = s.CreateDevice(ctx, *d, false /* createAsResource */)
		if err != nil {
			t.Fatalf("CreateDevice(%q) failed: %v", (*d).AssetTag, err)
		}
	}
	cdLinux := collectedDataForDevice(linuxWithData)
	cdLinux.OsId = "ubuntu"
	if err := s.RecordDeviceAuthnData(ctx, linuxWithData.Id, cdLinux); err != nil {
		t.Fatalf("RecordDeviceAuthnData failed: %v", err)
	}

	isDriftError := func(err error) bool {
		return errors.Is(err, &storage.CollectedDataDriftError{})
	}

	tests := []struct {
		name      string
		deviceID  string
		createCD  func() *devicepb.DeviceCollectedData
		wantErr   string
		assertErr func(error) bool
	}{
		{
			name:     "CollectTime missing",
			deviceID: devWithProfile.Id,
			createCD: func() *devicepb.DeviceCollectedData {
				cd := collectedDataForDevice(devWithProfile)
				cd.CollectTime = nil
				return cd
			},
			wantErr:   "collect time",
			assertErr: trace.IsBadParameter,
		},
		{
			name:     "SerialNumber mismatch",
			deviceID: devWithProfile.Id,
			createCD: func() *devicepb.DeviceCollectedData {
				cd := collectedDataForDevice(devWithProfile)
				cd.SerialNumber = "unknown"
				return cd
			},
			wantErr:   "serial number mismatch",
			assertErr: isDriftError,
		},
		{
			name:     "OSVersion for macOS not a semver",
			deviceID: devWithProfile.Id,
			createCD: func() *devicepb.DeviceCollectedData {
				cd := collectedDataForDevice(devWithProfile)
				cd.OsType = devicepb.OSType_OS_TYPE_MACOS // to be 100% sure
				cd.OsVersion = "not a semver"
				return cd
			},
			wantErr:   "OS version",
			assertErr: trace.IsBadParameter,
		},
		{
			name:     "JamfBinaryVersion not a semver",
			deviceID: devWithProfile.Id,
			createCD: func() *devicepb.DeviceCollectedData {
				cd := collectedDataForDevice(devWithProfile)
				cd.JamfBinaryVersion = "not a semver"
				return cd
			},
			wantErr:   "jamf binary",
			assertErr: trace.IsBadParameter,
		},

		// Profile validation.
		{
			name:     "profile ModelIdentifier drift",
			deviceID: devWithProfile.Id,
			createCD: func() *devicepb.DeviceCollectedData {
				cd := collectedDataForDevice(devWithProfile)
				cd.ModelIdentifier = "MacBookPro9,3" // can't change
				return cd
			},
			wantErr:   "device model",
			assertErr: isDriftError,
		},
		{
			name:     "profile OsVersion missing",
			deviceID: devWithProfile.Id,
			createCD: func() *devicepb.DeviceCollectedData {
				cd := collectedDataForDevice(devWithProfile)
				cd.OsVersion = ""
				return cd
			},
			wantErr:   "OS version",
			assertErr: isDriftError,
		},
		{
			name:     "profile OsVersion backwards drift",
			deviceID: devWithProfile.Id,
			createCD: func() *devicepb.DeviceCollectedData {
				cd := collectedDataForDevice(devWithProfile)
				cd.OsVersion = "13.1"
				return cd
			},
			wantErr:   "OS version",
			assertErr: isDriftError,
		},
		{
			name:     "profile OsUsernames drift",
			deviceID: devWithProfile.Id,
			createCD: func() *devicepb.DeviceCollectedData {
				cd := collectedDataForDevice(devWithProfile)
				cd.OsUsername = "alpaca2" // not in profile
				return cd
			},
			wantErr:   "OS username",
			assertErr: isDriftError,
		},

		// Previous data validation.
		{
			name:     "old collected data replay drift",
			deviceID: devWithData.Id,
			createCD: func() *devicepb.DeviceCollectedData {
				return cd1 // rolls back OS version, among others
			},
			wantErr:   "OS version",
			assertErr: isDriftError,
		},
		{
			name:     "collected data ModelIdentifier drift",
			deviceID: devWithData.Id,
			createCD: func() *devicepb.DeviceCollectedData {
				cd := proto.Clone(cd2).(*devicepb.DeviceCollectedData)
				cd.ModelIdentifier = "MacBookPro9,1" // can't change
				return cd
			},
			wantErr:   "device model",
			assertErr: isDriftError,
		},
		{
			name:     "collected data OsVersion drift",
			deviceID: devWithData.Id,
			createCD: func() *devicepb.DeviceCollectedData {
				cd := proto.Clone(cd2).(*devicepb.DeviceCollectedData)
				cd.OsVersion = "13.2.2" // in-between cd1 and cd2
				return cd
			},
			wantErr:   "OS version",
			assertErr: isDriftError,
		},
		{
			name:     "collected data OsUsername drift",
			deviceID: devWithData.Id,
			createCD: func() *devicepb.DeviceCollectedData {
				cd := proto.Clone(cd2).(*devicepb.DeviceCollectedData)
				cd.OsUsername = "eve" // unexpected change
				return cd
			},
			wantErr:   "OS username",
			assertErr: isDriftError,
		},
		{
			name:     "collected data OsId drift (Linux)",
			deviceID: linuxWithData.Id,
			createCD: func() *devicepb.DeviceCollectedData {
				cd := proto.Clone(cdLinux).(*devicepb.DeviceCollectedData)
				cd.OsId = "" // want "ubuntu"
				return cd
			},
			wantErr:   "OS ID",
			assertErr: isDriftError,
		},
		{
			name:     "profile OsId drift (Linux)",
			deviceID: linuxWithProfile.Id,
			createCD: func() *devicepb.DeviceCollectedData {
				cd := collectedDataForDevice(linuxWithProfile)
				cd.OsId = "fedora" // want "ubuntu"
				return cd
			},
			wantErr:   "OS ID",
			assertErr: isDriftError,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clock.Advance(1 * time.Second) // Use a somewhat-realistic time sequence

			err := s.RecordDeviceAuthnData(ctx, test.deviceID, test.createCD())
			switch {
			case err == nil:
				t.Fatal("RecordDeviceAuthnData returned err=nil, want non-nil")
			case !test.assertErr(err):
				t.Errorf("RecordDeviceAuthnData: assertErr failed, got err=%v (%T)", err, err)
			}
			assert.ErrorContains(t, err, test.wantErr, "RecordDeviceAuthnData error message mismatch")
		})
	}
}

func createAndEnroll(ctx context.Context, s *storage.S, dev *devicepb.Device, owner string) (*devicepb.Device, crypto.PrivateKey, error) {
	dev, err := s.CreateDevice(ctx, dev, false /* createAsResource */)
	if err != nil {
		return nil, nil, fmt.Errorf("calling CreateDevice: %w", err)
	}
	return enroll(ctx, s, dev, owner)
}

func enroll(ctx context.Context, s *storage.S, dev *devicepb.Device, owner string) (*devicepb.Device, crypto.PrivateKey, error) {
	var cred *devicepb.DeviceCredential
	var key crypto.PublicKey
	switch dev.OsType {
	case devicepb.OSType_OS_TYPE_MACOS:
		ecdsaKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return nil, nil, fmt.Errorf("calling GenerateKey: %w", err)
		}
		pubKeyDER, err := x509.MarshalPKIXPublicKey(ecdsaKey.Public())
		if err != nil {
			return nil, nil, fmt.Errorf("calling MarshalPKIXPublicKey: %w", err)
		}
		cred = &devicepb.DeviceCredential{
			Id:           uuid.NewString(),
			PublicKeyDer: pubKeyDER,
		}
		key = ecdsaKey
	case devicepb.OSType_OS_TYPE_LINUX, devicepb.OSType_OS_TYPE_WINDOWS:
		validAKPublic, err := base64.StdEncoding.DecodeString(validAKPublic)
		if err != nil {
			return nil, nil, fmt.Errorf("parsing valid ak public: %w", err)
		}
		cred = &devicepb.DeviceCredential{
			Id:                    "fake-credential-id",
			DeviceAttestationType: devicepb.DeviceAttestationType_DEVICE_ATTESTATION_TYPE_TPM_EKPUB,
			TpmAkPublic:           validAKPublic,
		}
	default:
		return nil, nil, fmt.Errorf("unhandled OS Type: %s", dev.OsType)
	}

	dev, err := s.EnrollDevice(ctx, dev.Id, cred, collectedDataForDevice(dev), owner)
	if err != nil {
		return nil, nil, err // unwrapped for simpler comparisons
	}
	return dev, key, nil
}

func collectedDataForDevice(dev *devicepb.Device) *devicepb.DeviceCollectedData {
	cd := &devicepb.DeviceCollectedData{
		CollectTime:  timestamppb.Now(),
		OsType:       dev.OsType,
		SerialNumber: dev.AssetTag,
	}
	if p := dev.Profile; p != nil {
		cd.ModelIdentifier = p.ModelIdentifier
		cd.OsVersion = p.OsVersion
		cd.OsBuild = p.OsBuild
		if len(p.OsUsernames) > 0 {
			cd.OsUsername = p.OsUsernames[0]
		}
		cd.JamfBinaryVersion = p.JamfBinaryVersion
	}
	return cd
}

func TestS_CreateDeviceEnrollTokenUsingData(t *testing.T) {
	t.Parallel()

	env := mustNewEnv()
	defer env.Close()

	clock := env.Clock
	s := env.S
	ctx := context.Background()

	// Prepare a handful of devices with varying profiles to test.
	devFullProfile := &devicepb.Device{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "llama1",
		Profile: &devicepb.DeviceProfile{
			ModelIdentifier:   "MacBookPro9,2",
			OsVersion:         "13.3.1",
			OsBuild:           "22E261",
			OsUsernames:       []string{"llama", "admin"},
			JamfBinaryVersion: "10.45.0-t1678116779",
		},
	}
	devSmallProfile := &devicepb.Device{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "llama2",
		Profile: &devicepb.DeviceProfile{
			ModelIdentifier: "MacBookPro9,2",
			OsVersion:       "13.3.1",
		},
	}
	devNoProfile := &devicepb.Device{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "alpaca",
	}
	for _, dev := range []**devicepb.Device{
		&devFullProfile,
		&devSmallProfile,
		&devNoProfile,
	} {
		created, err := s.CreateDevice(ctx, *dev, false /* createAsResource */)
		if err != nil {
			t.Fatalf("CreateDevice(%q) failed: %v", (*dev).AssetTag, err)
		}
		*dev = created
		clock.Advance(1 * time.Second)
	}

	tests := []struct {
		name    string
		cd      *devicepb.DeviceCollectedData
		wantDev *devicepb.Device
	}{
		{
			name: "auto-enroll (full profile)",
			cd: &devicepb.DeviceCollectedData{
				CollectTime: timestamppb.New(clock.Now()),
				// Required fields.
				OsType:       devFullProfile.OsType,
				SerialNumber: devFullProfile.AssetTag,
				// Profile-informed fields.
				ModelIdentifier:   devFullProfile.Profile.ModelIdentifier,
				OsVersion:         devFullProfile.Profile.OsVersion,
				OsBuild:           devFullProfile.Profile.OsBuild,
				OsUsername:        devFullProfile.Profile.OsUsernames[0],
				JamfBinaryVersion: devFullProfile.Profile.JamfBinaryVersion,
			},
			wantDev: devFullProfile,
		},
		{
			name: "auto-enroll (small profile)",
			cd: &devicepb.DeviceCollectedData{
				CollectTime:     timestamppb.New(clock.Now()),
				OsType:          devSmallProfile.OsType,
				SerialNumber:    devSmallProfile.AssetTag,
				ModelIdentifier: devSmallProfile.Profile.ModelIdentifier,
				OsVersion:       devSmallProfile.Profile.OsVersion,
				// Nothing else required by the profile.
			},
			wantDev: devSmallProfile,
		},
		{
			name: "auto-enroll (no profile)",
			cd: &devicepb.DeviceCollectedData{
				CollectTime:  timestamppb.New(clock.Now()),
				OsType:       devNoProfile.OsType,
				SerialNumber: devNoProfile.AssetTag,
				// Nothing else required by the profile.
			},
			wantDev: devNoProfile,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := s.CreateDeviceEnrollTokenUsingData(ctx, test.cd)
			if err != nil {
				t.Fatalf("CreateDeviceEnrollTokenUsingData failed: %v", err)
			}
			clock.Advance(1 * time.Second)

			// Verify that we got the correct device.
			if want := test.wantDev; got.Id != want.Id {
				t.Errorf(
					"CreateDeviceEnrollTokenUsingData: got device %v/%v, want %v/%v",
					got.Id, got.AssetTag,
					want.Id, want.AssetTag,
				)
			}

			// Verify that we got a non-empty token.
			if got.EnrollToken.GetToken() == "" {
				t.Fatalf("CreateDeviceEnrollTokenUsingData: got token=%v, want non-empty token", got.EnrollToken)
			}

			// Verify that no collected data was stored (device is not enrolled yet!)
			stored, err := s.GetDeviceByID(ctx, got.Id)
			if err != nil {
				t.Fatalf("GetDeviceByID failed: %v", err)
			}
			if len(stored.CollectedData) > 0 {
				t.Errorf("GetDeviceByID: got %v instances of collected data, wanted zero: %v", len(stored.CollectedData), stored.CollectedData)
			}

			// Spend the token to verify that it works.
			if tokenData, err := s.SpendDeviceEnrollToken(ctx, got.Id, got.EnrollToken.Token); err != nil {
				t.Errorf("SpendDeviceEnrollToken failed: %v", err)
			} else if !tokenData.CreatedByAutoEnroll {
				t.Errorf("SpendDeviceEnrollToken returned tokenData=%#v, want tokenData.CreatedByAutoEnroll=true", tokenData)
			}
		})
	}
}

func TestS_CreateDeviceEnrollTokenUsingData_errors(t *testing.T) {
	t.Parallel()

	env := mustNewEnv()
	defer env.Close()

	s := env.S
	ctx := context.Background()

	dev, err := s.CreateDevice(ctx, &devicepb.Device{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "llama1",
		Profile: &devicepb.DeviceProfile{
			ModelIdentifier:   "MacBookPro9,2",
			OsVersion:         "13.3.1",
			OsBuild:           "22E261",
			OsUsernames:       []string{"llama", "admin"},
			JamfBinaryVersion: "10.45.0-t1678116779",
		},
	}, false /* createAsResource */)
	if err != nil {
		t.Fatalf("CreateDevice) failed: %v", err)
	}

	// Register an unrelated Windows device.
	// The purpose of this device is to not match collected data from its namesake
	// MacOS device.
	if _, err = s.CreateDevice(ctx, &devicepb.Device{
		OsType:   devicepb.OSType_OS_TYPE_WINDOWS,
		AssetTag: "llama1",
		Profile: &devicepb.DeviceProfile{
			ModelIdentifier:   "ThinkPad 9000",
			OsVersion:         "22H2",
			OsBuild:           "19045",
			OsUsernames:       []string{"not-a-llama", "admin"},
			JamfBinaryVersion: "10.44",
		},
	}, false /* createAsResource */); err != nil {
		t.Fatalf("CreateDevice failed: %v", err)
	}

	const owner = "llama"
	devEnrolled, _, err := createAndEnroll(ctx, s, &devicepb.Device{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "llama2",
		// A nil Profile makes this device very easy to auto-enroll, if not for the
		// fact that it already is enrolled.
		Profile: nil,
	}, owner)
	if err != nil {
		t.Fatalf("createAndEnroll failed: %v", err)
	}

	isDriftError := func(err error) bool {
		return errors.Is(err, &storage.CollectedDataDriftError{})
	}

	tests := []struct {
		name      string
		createCD  func() *devicepb.DeviceCollectedData
		assertErr func(err error) bool
		wantErr   string
	}{
		{
			name: "unknown device (OSType)",
			createCD: func() *devicepb.DeviceCollectedData {
				cd := collectedDataForDevice(dev)
				cd.OsType = devicepb.OSType_OS_TYPE_LINUX // unknown
				return cd
			},
			assertErr: trace.IsNotFound,
			wantErr:   "not found",
		},
		{
			name: "unknown device (SerialNumber)",
			createCD: func() *devicepb.DeviceCollectedData {
				cd := collectedDataForDevice(dev)
				cd.SerialNumber = "unknown"
				return cd
			},
			assertErr: trace.IsNotFound,
			wantErr:   "not found",
		},
		{
			name: "ModelIdentifier empty",
			createCD: func() *devicepb.DeviceCollectedData {
				cd := collectedDataForDevice(dev)
				cd.ModelIdentifier = ""
				return cd
			},
			assertErr: isDriftError,
			wantErr:   "device model",
		},
		{
			name: "ModelIdentifier invalid",
			createCD: func() *devicepb.DeviceCollectedData {
				cd := collectedDataForDevice(dev)
				cd.ModelIdentifier = "MacBookPro10,1"
				return cd
			},
			assertErr: isDriftError,
			wantErr:   "device model",
		},
		{
			name: "OsVersion invalid",
			createCD: func() *devicepb.DeviceCollectedData {
				cd := collectedDataForDevice(dev)
				cd.OsVersion = "13.4" // even a drift upwards is disallowed here
				return cd
			},
			assertErr: isDriftError,
			wantErr:   "OS version",
		},
		{
			name: "OsBuild invalid",
			createCD: func() *devicepb.DeviceCollectedData {
				cd := collectedDataForDevice(dev)
				cd.OsBuild = "22E262" // changed from 22E261
				return cd
			},
			assertErr: isDriftError,
			wantErr:   "OS build",
		},
		{
			name: "OsUsername invalid",
			createCD: func() *devicepb.DeviceCollectedData {
				cd := collectedDataForDevice(dev)
				cd.OsUsername = "llamaO" // wanted "llama" or "admin"
				return cd
			},
			assertErr: isDriftError,
			wantErr:   "OS username",
		},
		{
			name: "JamfBinaryVersion invalid",
			createCD: func() *devicepb.DeviceCollectedData {
				cd := collectedDataForDevice(dev)
				cd.JamfBinaryVersion = "10.46" // changed from 10.45.0-t1678116779
				return cd
			},
			assertErr: isDriftError,
			wantErr:   "jamf binary",
		},
		{
			name: "device already enrolled",
			createCD: func() *devicepb.DeviceCollectedData {
				return collectedDataForDevice(devEnrolled)
			},
			assertErr: trace.IsBadParameter,
			wantErr:   "already enrolled",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := s.CreateDeviceEnrollTokenUsingData(ctx, test.createCD())
			if err == nil {
				t.Fatal("CreateDeviceEnrollTokenUsingData returned err=nil, want non-nil", err)
			}
			if !test.assertErr(err) {
				t.Errorf("CreateDeviceEnrollTokenUsingData: assertErr failed, err=%v (%T)", err, err)
			}
			assert.ErrorContains(t, err, test.wantErr, "CreateDeviceEnrollTokenUsingData error mismatch")
		})
	}
}

func TestS_CreateDeviceEnrollToken_createAndSpend(t *testing.T) {
	t.Parallel()

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
		expiresAt       time.Time
		createToken     func(ctx context.Context, deviceID string, expiresAt time.Time) (*devicepb.DeviceEnrollToken, error)
		spendToken      func(ctx context.Context, deviceID, token string) (*storage.DeviceEnrollTokenData, error)
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
			createToken: func(ctx context.Context, deviceID string, expiresAt time.Time) (*devicepb.DeviceEnrollToken, error) {
				first, err := s.CreateDeviceEnrollToken(ctx, deviceID, expiresAt)
				if err != nil {
					return nil, err
				}

				// Immediately replace initial token.
				second, err := s.CreateDeviceEnrollToken(ctx, deviceID, expiresAt)
				if err != nil {
					return nil, err
				}
				// Sanity check that tokens are different.
				if first.Token == second.Token {
					return nil, errors.New("first and second tokens are equal")
				}

				// First token cannot be spent anymore.
				if _, err := s.SpendDeviceEnrollToken(ctx, deviceID, first.Token); !trace.IsBadParameter(err) {
					// Original error type erased on purpose (%v instead of %w)
					return nil, fmt.Errorf("unexpected error attempting to spend first token: %w", err)
				}

				return second, nil
			},
			spendToken: s.SpendDeviceEnrollToken,
		},
		{
			name:     "token expires (default time)",
			deviceID: deviceID,
			createToken: func(ctx context.Context, deviceID string, _ time.Time) (*devicepb.DeviceEnrollToken, error) {
				token, err := s.CreateDeviceEnrollToken(ctx, deviceID, time.Time{})
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
			name:      "token expires (custom time)",
			deviceID:  deviceID,
			expiresAt: clock.Now().Add(2 * time.Minute),
			createToken: func(ctx context.Context, deviceID string, expiresAt time.Time) (*devicepb.DeviceEnrollToken, error) {
				token, err := s.CreateDeviceEnrollToken(ctx, deviceID, expiresAt)
				if err != nil {
					return nil, err
				}

				// Fast-forward to after the token is expired.
				clock.Advance(expiresAt.Sub(clock.Now()) + 1)

				return token, nil
			},
			spendToken:     s.SpendDeviceEnrollToken,
			assertSpendErr: trace.IsNotFound,
		},
		{
			name:        "token is tied to device",
			deviceID:    deviceID,
			createToken: s.CreateDeviceEnrollToken,
			spendToken: func(ctx context.Context, _, token string) (*storage.DeviceEnrollTokenData, error) {
				return s.SpendDeviceEnrollToken(ctx, dev2.Id /* wrong device */, token)
			},
			assertSpendErr: trace.IsNotFound,
		},
		{
			name:        "token must match",
			deviceID:    deviceID,
			createToken: s.CreateDeviceEnrollToken,
			spendToken: func(ctx context.Context, deviceID, token string) (*storage.DeviceEnrollTokenData, error) {
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
			spendToken: func(ctx context.Context, _, token string) (*storage.DeviceEnrollTokenData, error) {
				return s.SpendDeviceEnrollToken(ctx, "unknown", token)
			},
			assertSpendErr: trace.IsNotFound,
		},
		{
			name:        "double spend fails",
			deviceID:    deviceID,
			createToken: s.CreateDeviceEnrollToken,
			spendToken: func(ctx context.Context, deviceID string, token string) (*storage.DeviceEnrollTokenData, error) {
				if _, err := s.SpendDeviceEnrollToken(ctx, deviceID, token); err != nil {
					return nil, errors.New("first spend failed")
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
			spendToken: func(ctx context.Context, _, token string) (*storage.DeviceEnrollTokenData, error) {
				return s.SpendDeviceEnrollToken(ctx, "" /* deviceID */, token)
			},
			assertSpendErr: trace.IsBadParameter,
		},
		{
			name:        "SpendDeviceEnrollToken requires token",
			deviceID:    deviceID,
			createToken: s.CreateDeviceEnrollToken,
			spendToken: func(ctx context.Context, deviceID, _ string) (*storage.DeviceEnrollTokenData, error) {
				return s.SpendDeviceEnrollToken(ctx, deviceID, "" /* token */)
			},
			assertSpendErr: trace.IsBadParameter,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			token, err := test.createToken(ctx, test.deviceID, test.expiresAt)
			switch {
			case test.assertCreateErr != nil:
				if !test.assertCreateErr(err) {
					t.Errorf("CreateDeviceEnrollToken: assert failed, err=%v", err)
				}
				return
			case err != nil:
				t.Fatalf("CreateDeviceEnrollToken failed: %v", err)
			}

			switch tokenData, err := test.spendToken(ctx, test.deviceID, token.GetToken()); {
			case test.assertSpendErr != nil:
				if !test.assertSpendErr(err) {
					t.Errorf("SpendDeviceEnrollmentToken: assert failed, err=%v", err)
				}
			case err != nil:
				t.Fatalf("SpendDeviceEnrollmentToken failed: %v", err)
			case tokenData.CreatedByAutoEnroll:
				t.Errorf("SpendDeviceEnrollmentToken returned tokenData=%#v, want tokenData.CreatedByAutoEnroll=false", tokenData)
			}
		})
	}
}

func TestS_DevicesUsageLimit(t *testing.T) {
	// Don't t.Parallel! Uses modules.SetTestModules.

	const devicesLimit = 3
	features := modules.GetModules().Features()
	features.IsUsageBasedBilling = true
	features.Entitlements[entitlements.DeviceTrust] = modules.EntitlementInfo{
		Enabled: true,
		Limit:   devicesLimit,
	}
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures:  features,
	})

	// Lock acquisition for usage-based enrollments requires a RealClock, the test
	// will deadlock otherwise.
	env := mustNewEnv(withClock(clockwork.NewRealClock()))
	defer env.Close()

	s := env.S
	ctx := context.Background()

	assertUsage := func(t *testing.T, wantEnrolled int) {
		got, err := s.GetDevicesUsage(ctx)
		if err != nil {
			t.Errorf("GetDevicesUsage failed: %v", err)
			return
		}
		want := &storage.DevicesUsage{
			NumEnrolled: wantEnrolled,
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("GetDevicesUsage mismatch (-want +got)\n%s", diff)
		}
	}

	t.Run("GetDevicesUsage/zero", func(t *testing.T) {
		assertUsage(t, 0 /* wantEnrolled */)
	})

	// Add a few devices.
	const allDevsNum = devicesLimit + 10
	allDevs := make([]*devicepb.Device, allDevsNum)
	for i := 0; i < allDevsNum; i++ {
		dev, err := s.CreateDevice(ctx, &devicepb.Device{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: fmt.Sprintf("dev-%v", i),
		}, false /* createAsResource */)
		if err != nil {
			t.Fatalf("CreateDevice failed: %v", err)
		}
		allDevs[i] = dev
	}

	// Usage is still zero.
	t.Run("GetDevicesUsage/zero", func(t *testing.T) {
		assertUsage(t, 0 /* wantEnrolled */)
	})

	// Enroll a few devices and verify the side effects.
	const owner = "llama"
	wantEnrolled := devicesLimit - 1
	for _, dev := range allDevs[:wantEnrolled] {
		if _, _, err := enroll(ctx, s, dev, owner); err != nil {
			t.Errorf("enroll returned err=%v, want success", err)
		}
	}
	t.Run(fmt.Sprintf("GetDevicesUsage/%v", wantEnrolled), func(t *testing.T) {
		assertUsage(t, wantEnrolled)
	})
	t.Run("VerifyEnrolledDevicesLimit/allowed", func(t *testing.T) {
		if err := s.VerifyEnrolledDevicesLimit(ctx); err != nil {
			t.Errorf("VerifyEnrolledDevicesLimit returned err=%v, want success", err)
		}
	})

	// Attempt to enroll past the limit.
	t.Run("Enroll beyond limit", func(t *testing.T) {
		var g errgroup.Group
		var successes atomic.Int32
		for _, dev := range allDevs[wantEnrolled:] {
			dev := dev
			g.Go(func() error {
				switch _, _, err := enroll(ctx, s, dev, owner); {
				case err == nil:
					successes.Add(1)
				case !trace.IsAccessDenied(err):
					t.Errorf("enroll returned unexpected error: %v, want either nil or AccessDenied", err)
				}
				return nil
			})
		}
		g.Wait()
		if got := successes.Load(); got != 1 {
			t.Errorf("Enrolled %v devices at the limit, want exactly 1", got)
		}
	})

	t.Run("VerifyEnrolledDevicesLimit/denied", func(t *testing.T) {
		if err := s.VerifyEnrolledDevicesLimit(ctx); !trace.IsAccessDenied(err) {
			t.Errorf("VerifyEnrolledDevicesLimit returned err=%v, want AccessDenied/devices limit failure", err)
		}
	})

	// Add limit back to Device Trust & disable Identity
	features.Entitlements[entitlements.Identity] = modules.EntitlementInfo{Enabled: false, Limit: 0}
	features.Entitlements[entitlements.DeviceTrust] = modules.EntitlementInfo{Enabled: true, Limit: 1}
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures:  features,
	})
	t.Run("VerifyEnrolledDevicesLimit/denied", func(t *testing.T) {
		if err := s.VerifyEnrolledDevicesLimit(ctx); !trace.IsAccessDenied(err) {
			t.Errorf("VerifyEnrolledDevicesLimit returned err=%v, want AccessDenied/devices limit failure", err)
		}
	})
}

func TestS_AssignDeviceOwner(t *testing.T) {
	t.Parallel()

	env := mustNewEnv()
	defer env.Close()

	identity := env.IdentityService
	s := env.S
	ctx := context.Background()

	const user1 = "llama"
	const user2 = "alpaca"

	var allDevs []*devicepb.Device
	for _, d := range []*devicepb.Device{
		{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: "emptyOwner",
		},
		{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: "onlyDevice",
			Owner:    user1,
		},
		{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: "onlyUser", // Assigned below.
		},
	} {
		created, err := s.CreateDevice(ctx, d, true /* createAsResource */)
		if err != nil {
			t.Fatalf("CreateDevice failed: %v", err)
		}
		allDevs = append(allDevs, created)
	}
	devEmptyOwner := allDevs[0]
	devOnlyDevice := allDevs[1]
	devOnlyUser := allDevs[2]

	// Create test users.
	u1, _ := types.NewUser(user1)
	u2, _ := types.NewUser(user2)
	u2.SetTrustedDeviceIDs([]string{devOnlyUser.Id})
	if _, err := identity.CreateUser(ctx, u1); err != nil {
		t.Fatalf("CreateUser(%q) failed: %v", u1.GetName(), err)
	}
	if _, err := identity.CreateUser(ctx, u2); err != nil {
		t.Fatalf("CreateUser(%q) failed: %v", u2.GetName(), err)
	}

	tests := []struct {
		name string
		dev  *devicepb.Device
		user string
	}{
		{
			name: "device without owner",
			dev:  devEmptyOwner,
			user: user1,
		},
		{
			name: "device assigned, user not assigned",
			dev:  devOnlyDevice,
			user: user1,
		},
		{
			name: "device not assigned, user assigned",
			dev:  devOnlyUser,
			user: user2,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			deviceID := test.dev.Id
			got, err := s.AssignDeviceOwner(ctx, deviceID, test.user)
			if err != nil {
				t.Fatalf("AssignDeviceOwner returned err=%v", err)
			}

			// Assert returned device.
			want := test.dev
			want.Owner = test.user
			want.UpdateTime = got.UpdateTime // System-managed.
			if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
				t.Errorf("AssignDeviceOwner mismatch (-want +got)\n%s", diff)
			}

			// Assert stored device.
			stored, err := s.GetDeviceByID(ctx, deviceID)
			if err != nil {
				t.Fatalf("GetDeviceByID failed: %v", err)
			}
			if diff := cmp.Diff(got, stored, protocmp.Transform()); diff != "" {
				t.Errorf("GetDeviceByID mismatch (-want +got)\n%s", diff)
			}

			// Assert user trusted devices.
			u, err := identity.GetUser(ctx, test.user, false /* withSecrets */)
			if err != nil {
				t.Fatalf("GetUser failed: %v", err)
			}
			if !slices.Contains(u.GetTrustedDeviceIDs(), deviceID) {
				t.Errorf(
					"User %q missing trusted device %q, u.TrustedDeviceIDs=%v",
					test.user, test.dev.Id, u.GetTrustedDeviceIDs())
			}
		})
	}
}

// TestS_UserTrustedDeviceIDs tests various methods that update the user's
// trusted device IDs as part of their side-effects.
// See [TestS_AssignDeviceOwner] for [storage.S.AssignDeviceOwner] tests.
func TestS_UserTrustedDeviceIDs(t *testing.T) {
	t.Parallel()

	env := mustNewEnv()
	defer env.Close()

	identity := env.IdentityService
	s := env.S
	ctx := context.Background()

	// Create test users.
	const user1 = "llama"
	const user2 = "alpaca"
	u1, _ := types.NewUser(user1)
	u2, _ := types.NewUser(user2)
	if _, err := identity.CreateUser(ctx, u1); err != nil {
		t.Fatalf("CreateUser(%q) failed: %v", u1.GetName(), err)
	}
	if _, err := identity.CreateUser(ctx, u2); err != nil {
		t.Fatalf("CreateUser(%q) failed: %v", u2.GetName(), err)
	}

	// Create a couple of previously-owned devices.
	var allDevs []*devicepb.Device
	for _, d := range []*devicepb.Device{
		{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: "re-enroll",
		},
		{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: "unenroll",
		},
		{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: "delete",
		},
	} {
		created, _, err := createAndEnroll(ctx, s, d, user1)
		if err != nil {
			t.Fatalf("createAndEnroll failed: %v", err)
		}
		allDevs = append(allDevs, created)
	}
	reEnrollDev := allDevs[0]
	unenrollDev := allDevs[1]
	deleteDev := allDevs[2]

	tests := []struct {
		name string
		// baseDev is created if it lacks an Id.
		baseDev *devicepb.Device
		// baseUser is the Teleport user to be assigned/unassigned the device.
		baseUser types.User
		// fn assigns/unassigns an owner to dev
		fn           func(dev *devicepb.Device, user string) (*devicepb.Device, error)
		wantOwner    string // If distinct from "" or baseUser.
		wantAssigned bool   // If true, then baseUser is the owner of baseDev.
	}{
		{
			name: "EnrollDevice assigns owner",
			baseDev: &devicepb.Device{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "enroll1",
			},
			baseUser: u1,
			fn: func(dev *devicepb.Device, user string) (*devicepb.Device, error) {
				dev, _, err := enroll(ctx, s, dev, user)
				return dev, err
			},
			wantAssigned: true,
		},
		{
			name:     "EnrollDevice re-enroll unassigns previous owner",
			baseDev:  reEnrollDev,
			baseUser: u1, // Previous owner, unassigned.
			fn: func(dev *devicepb.Device, _ string) (*devicepb.Device, error) {
				dev, _, err := enroll(ctx, s, dev, user2)
				return dev, err
			},
			wantOwner:    user2,
			wantAssigned: false,
		},
		{
			name:     "UpdateDevice unenroll removes owner",
			baseDev:  unenrollDev,
			baseUser: u1,
			fn: func(dev *devicepb.Device, _ string) (*devicepb.Device, error) {
				return s.UpdateDevice(ctx, dev.Id, func(stored *devicepb.Device) *devicepb.Device {
					stored.EnrollStatus = devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED
					return stored
				})
			},
			wantAssigned: false,
		},
		{
			name:     "DeleteDevice removes owner",
			baseDev:  deleteDev,
			baseUser: u1,
			fn: func(dev *devicepb.Device, _ string) (*devicepb.Device, error) {
				err := s.DeleteDevice(ctx, dev.Id)
				return nil, err
			},
			wantAssigned: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dev := test.baseDev
			user := test.baseUser.GetName()

			if dev.Id == "" {
				var err error
				dev, err = s.CreateDevice(ctx, test.baseDev, false /* createAsResource */)
				if err != nil {
					t.Fatalf("CreateDevice failed: %v", err)
				}
			}

			// Sanity check: device is assigned before `fn`.
			if !test.wantAssigned {
				storedUser, err := identity.GetUser(ctx, user, false /* withSecrets */)
				switch {
				case err != nil:
					t.Fatalf("GetUser failed: %v", err)
				case !slices.Contains(storedUser.GetTrustedDeviceIDs(), dev.Id):
					t.Fatalf(
						"User %q does not have trusted device %q, u.TrustedDeviceIDs=%v",
						user, dev.Id, storedUser.GetTrustedDeviceIDs())
				}
			}

			// Enroll, Update, Delete, etc.
			got, err := test.fn(dev, user)
			if err != nil {
				t.Fatalf("fn returned err=%v, want nil", err)
			}

			// Verify returned device.
			wantOwner := ""
			if test.wantOwner != "" {
				wantOwner = test.wantOwner
			} else if test.wantAssigned {
				wantOwner = user
			}
			if got.GetOwner() != wantOwner {
				t.Errorf("fn returned dev.Owner=%q, want %q", got.GetOwner(), wantOwner)
			}

			// Verify user trusted devices.
			storedUser, err := identity.GetUser(ctx, user, false /* withSecrets */)
			if err != nil {
				t.Fatalf("GetUser failed: %v", err)
			}
			trustedIDs := storedUser.GetTrustedDeviceIDs()
			if got, want := slices.Contains(trustedIDs, dev.Id), test.wantAssigned; got != want {
				var msg string
				if want {
					msg = "Device %q not assigned to user %q, User.TrustedDeviceIDs=%v"
				} else {
					msg = "Device %q not unassigned from user %q, User.TrustedDeviceIDs=%v"
				}
				t.Errorf(msg, dev.Id, user, trustedIDs)
			}
		})
	}
}

func TestS_CreateDeviceWebToken(t *testing.T) {
	t.Parallel()

	env := mustNewEnv()
	defer env.Close()

	s := env.S
	ctx := context.Background()

	validToken := &devicepb.DeviceWebToken{
		WebSessionId:      "llama-session-id-1234",
		BrowserUserAgent:  sampleUserAgent,
		BrowserIp:         sampleIP,
		User:              "llama",
		ExpectedDeviceIds: []string{"device-id-1"},
	}

	// Success scenario.
	t.Run("success", func(t *testing.T) {
		got, err := s.CreateDeviceWebToken(ctx, validToken)
		if err != nil {
			t.Fatalf("CreateDeviceWebToken failed: %v", err)
		}

		// Assert that "usage" field are returned.
		if got.Id == "" {
			t.Error("Created DeviceWebToken has an empty ID")
		}
		if got.Token == "" {
			t.Error("Created DeviceWebToken has an empty Token")
		}

		// Assert that no other fields are set.
		want := &devicepb.DeviceWebToken{
			Id:    got.Id,
			Token: got.Token,
		}
		if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
			t.Errorf("DeviceWebToken mismatch (-want +got)\n%s", diff)
		}
	})

	makeToken := func(fn func(*devicepb.DeviceWebToken)) *devicepb.DeviceWebToken {
		token := proto.Clone(validToken).(*devicepb.DeviceWebToken)
		fn(token)
		return token
	}

	// Failure scenarios.
	tests := []struct {
		name    string
		token   *devicepb.DeviceWebToken
		wantErr string
	}{
		{
			name:    "nil token",
			token:   nil,
			wantErr: "token required",
		},
		{
			name: "WebSessionId empty",
			token: makeToken(func(token *devicepb.DeviceWebToken) {
				token.WebSessionId = ""
			}),
			wantErr: "web session ID",
		},
		{
			name: "BrowserUserAgent empty",
			token: makeToken(func(token *devicepb.DeviceWebToken) {
				token.BrowserUserAgent = ""
			}),
			wantErr: "user agent",
		},
		{
			name: "BrowserIp empty",
			token: makeToken(func(token *devicepb.DeviceWebToken) {
				token.BrowserIp = ""
			}),
			wantErr: "IP",
		},
		{
			name: "User empty",
			token: makeToken(func(token *devicepb.DeviceWebToken) {
				token.User = ""
			}),
			wantErr: "user",
		},
		{
			name: "ExpectedDeviceIds nil",
			token: makeToken(func(token *devicepb.DeviceWebToken) {
				token.ExpectedDeviceIds = nil
			}),
			wantErr: "device IDs",
		},
		{
			name: "empty ExpectedDeviceIds item",
			token: makeToken(func(token *devicepb.DeviceWebToken) {
				token.ExpectedDeviceIds = []string{
					"device-id-1",
					"", // invalid!
					"device-id-3",
				}
			}),
			wantErr: "device ID ",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := s.CreateDeviceWebToken(ctx, test.token)
			if err == nil {
				t.Fatal("CreateDeviceWebToken returned err=nil, want non-nil")
			}
			if !trace.IsBadParameter(err) {
				t.Errorf("CreateDeviceWebToken returned err=%T, want BadParameter", trace.Unwrap(err))
			}
			assert.ErrorContains(t, err, test.wantErr, "CreateDeviceWebToken error mismatch")
		})
	}
}

func TestS_SpendDeviceWebToken(t *testing.T) {
	t.Parallel()

	env := mustNewEnv()
	t.Cleanup(func() { env.Close() })

	s := env.S
	ctx := context.Background()

	// Not exercised by this test.
	const authenticatedDeviceID = "llama-dev-1"

	sampleToken := &devicepb.DeviceWebToken{
		WebSessionId:      "llama-session-id-1234",
		BrowserUserAgent:  sampleUserAgent,
		BrowserIp:         sampleIP,
		User:              "llama",
		ExpectedDeviceIds: []string{authenticatedDeviceID, "llama-dev-2"},
	}

	createWebToken := func(t *testing.T) *devicepb.DeviceWebToken {
		token, err := s.CreateDeviceWebToken(ctx, sampleToken)
		if err != nil {
			t.Fatalf("CreateDeviceWebToken failed: %v", err)
		}
		return token
	}

	// Success scenario.
	t.Run("success", func(t *testing.T) {
		t.Parallel()

		token := createWebToken(t)

		gotWeb, gotConfirm, err := s.SpendDeviceWebToken(ctx, token, authenticatedDeviceID)
		if err != nil {
			t.Fatalf("SpendDeviceWebToken failed: %v", err)
		}

		// Verify DeviceWebToken.
		wantWeb := proto.Clone(sampleToken).(*devicepb.DeviceWebToken)
		wantWeb.Id = token.Id
		wantWeb.Token = ""
		if diff := cmp.Diff(wantWeb, gotWeb, protocmp.Transform()); diff != "" {
			t.Errorf("SpendDeviceWebToken webToken mismatch (-want +got)\n%s", diff)
		}

		// Verify DeviceConfirmationToken.
		if gotConfirm.GetToken() == "" {
			t.Errorf("SpendDeviceWebToken returned a nil or empty confirm token: %#v", gotConfirm)
		}
		wantConfirm := &devicepb.DeviceConfirmationToken{
			Id:    token.Id, // same underlying attempt ID
			Token: gotConfirm.Token,
		}
		if diff := cmp.Diff(wantConfirm, gotConfirm, protocmp.Transform()); diff != "" {
			t.Errorf("SpendDeviceWebToken confirmToken mismatch (-want +got)\n%s", diff)
		}

		t.Run("previously spent token", func(t *testing.T) {
			_, _, err := s.SpendDeviceWebToken(ctx, token, authenticatedDeviceID)
			if !trace.IsBadParameter(err) {
				t.Errorf("SpendDeviceWebToken error mismatch, err=%v (%T), want BadParameter", err, trace.Unwrap(err))
			}
			assert.ErrorContains(t, err, "attempt state", "SpendDeviceWebToken error mismatch")
		})
	})

	const invalidToken = "ceci n'est pas a valid token"
	invalidTokenB64 := base64.RawURLEncoding.EncodeToString([]byte(invalidToken))

	// Tested here to keep the failure table simpler.
	t.Run("empty authenticatedDeviceID", func(t *testing.T) {
		t.Parallel()

		_, _, err := s.SpendDeviceWebToken(ctx, &devicepb.DeviceWebToken{
			Id:    "valid-token-id",
			Token: "validtokenb64",
		}, "" /* authenticatedDeviceID */)
		if !trace.IsBadParameter(err) {
			t.Errorf("SpendDeviceWebToken returned err=%v (%T), want BadParameter", err, trace.Unwrap(err))
		}
		assert.ErrorContains(t, err, "device ID required", "SpendDeviceWebToken error mismatch")
	})

	// Failure scenarios.
	tests := []struct {
		name            string
		createToken     func(t *testing.T) *devicepb.DeviceWebToken // modified test token
		assertErrorType func(err error) bool                        // defaults to trace.IsBadParameter
		wantErr         string
		wantDeleted     bool // Verifies if a subsequent Spend returns NotFound.
	}{
		{
			name: "unknown token",
			createToken: func(_ *testing.T) *devicepb.DeviceWebToken {
				return &devicepb.DeviceWebToken{
					Id:    "unknown-token-ID",
					Token: invalidTokenB64,
				}
			},
			assertErrorType: trace.IsNotFound,
		},
		{
			name: "ID empty",
			createToken: func(_ *testing.T) *devicepb.DeviceWebToken {
				return &devicepb.DeviceWebToken{
					Token: invalidTokenB64,
				}
			},
			wantErr: "token ID required",
		},
		{
			name: "Token empty",
			createToken: func(t *testing.T) *devicepb.DeviceWebToken {
				token := createWebToken(t)
				token.Token = "" // Same as an invalid token.
				return token
			},
			wantErr:     "invalid device token",
			wantDeleted: true,
		},
		{
			name: "Token invalid",
			createToken: func(t *testing.T) *devicepb.DeviceWebToken {
				token := createWebToken(t)
				token.Token = invalidTokenB64
				return token
			},
			wantErr:     "invalid device token",
			wantDeleted: true,
		},
		{
			name: "Token not base64",
			createToken: func(t *testing.T) *devicepb.DeviceWebToken {
				token := createWebToken(t)
				token.Token = invalidToken // bad encoding
				return token
			},
			wantErr:     "base64",
			wantDeleted: true,
		},
	}
	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			token := test.createToken(t)

			_, _, err := s.SpendDeviceWebToken(ctx, token, authenticatedDeviceID)
			if err == nil {
				t.Fatal("SpendDeviceWebToken returned err=nil, want non-nil")
			}

			// Assert error type.
			if test.assertErrorType == nil {
				test.assertErrorType = trace.IsBadParameter
			}
			if !test.assertErrorType(err) {
				t.Errorf("SpendDeviceWebToken error type mismatch, got=%T", trace.Unwrap(err))
			}

			// Assert error type. Used to disambiguate errors.
			if test.wantErr != "" {
				assert.ErrorContains(t, err, test.wantErr, "SpendDeviceWebToken error mismatch")
			}

			// Assert deletion of existing token.
			if !test.wantDeleted {
				return
			}
			if _, _, err := s.SpendDeviceWebToken(ctx, token, authenticatedDeviceID); !trace.IsNotFound(err) {
				t.Errorf("SpendDeviceEnrollToken returned err=%q (%T), wanted NotFound (signifying a spent token)", err, trace.Unwrap(err))
			}
		})
	}
}

func TestS_SpendDeviceConfirmationToken(t *testing.T) {
	t.Parallel()

	env := mustNewEnv()
	defer env.Close()

	s := env.S
	ctx := context.Background()

	const authenticatedDeviceID = "llama1"
	sampleToken := &devicepb.DeviceWebToken{
		WebSessionId:      "llama-session-id",
		BrowserUserAgent:  sampleUserAgent,
		BrowserIp:         sampleIP,
		User:              "llama",
		ExpectedDeviceIds: []string{authenticatedDeviceID, "llama2"},
	}

	createConfirmToken := func(t *testing.T) (*devicepb.DeviceWebToken, *devicepb.DeviceConfirmationToken) {
		webToken, err := s.CreateDeviceWebToken(ctx, sampleToken)
		if err != nil {
			t.Fatalf("CreateDeviceWebToken failed: %v", err)
		}

		_, confirmToken, err := s.SpendDeviceWebToken(ctx, webToken, authenticatedDeviceID)
		if err != nil {
			t.Fatalf("SpendDeviceWebToken failed: %v", err)
		}

		return webToken, confirmToken
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		_, confirmToken := createConfirmToken(t)
		gotData, err := s.SpendDeviceConfirmationToken(ctx, confirmToken)
		if err != nil {
			t.Fatalf("SpendDeviceConfirmationToken failed: %v", err)
		}

		// Verify returned data.
		wantData := &storage.DeviceConfirmationTokenData{
			WebSessionID:          sampleToken.WebSessionId,
			User:                  sampleToken.User,
			BrowserIP:             sampleToken.BrowserIp,
			AuthenticatedDeviceID: authenticatedDeviceID,
		}
		if diff := cmp.Diff(wantData, gotData); diff != "" {
			t.Errorf("SpendDeviceConfirmationToken mismatch (-want +got)\n%s", diff)
		}

		t.Run("double spend", func(t *testing.T) {
			_, err := s.SpendDeviceConfirmationToken(ctx, confirmToken)
			if !trace.IsNotFound(err) {
				t.Errorf("SpendDeviceConfirmationToken returned err=%v (%T), want NotFound", err, trace.Unwrap(err))
			}
		})
	})

	// Failure scenarios.
	tests := []struct {
		name        string
		createToken func(*testing.T) *devicepb.DeviceConfirmationToken
		assertErr   func(error) bool
		wantErr     string
	}{
		{
			name: "nil token",
			createToken: func(*testing.T) *devicepb.DeviceConfirmationToken {
				return nil
			},
			assertErr: trace.IsBadParameter,
			wantErr:   "token ID required",
		},
		{
			name: "unknown token",
			createToken: func(*testing.T) *devicepb.DeviceConfirmationToken {
				return &devicepb.DeviceConfirmationToken{
					Id:    "not a token ID",
					Token: "ACBDEF0123456789", // valid b64
				}
			},
			assertErr: trace.IsNotFound,
		},
		{
			name: "invalid token",
			createToken: func(*testing.T) *devicepb.DeviceConfirmationToken {
				_, token := createConfirmToken(t)
				token.Token = "notavalidtoken" // valid b64
				return token
			},
			assertErr: trace.IsBadParameter,
			wantErr:   "invalid device token",
		},
		{
			name: "invalid attempt state",
			createToken: func(*testing.T) *devicepb.DeviceConfirmationToken {
				webToken, err := s.CreateDeviceWebToken(ctx, sampleToken)
				if err != nil {
					t.Fatalf("CreateDeviceWebToken failed: %v", err)
				}

				// Try to spend our webToken as if it was a confirmToken.
				// This shouldn't work for various reasons, the first being an incorrect
				// attempt state.
				return &devicepb.DeviceConfirmationToken{
					Id:    webToken.Id,
					Token: webToken.Token,
				}
			},
			assertErr: trace.IsBadParameter,
			wantErr:   "attempt state",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := s.SpendDeviceConfirmationToken(ctx, test.createToken(t))
			if !test.assertErr(err) {
				t.Errorf("SpendDeviceConfirmationToken: assertErr failed, err=%v (%T)", err, trace.Unwrap(err))
			}
			if test.wantErr != "" {
				assert.ErrorContains(t, err, test.wantErr, "SpendDeviceConfirmationToken error mismatch")
			}
		})
	}
}

func TestS_DeleteDeviceWebAuthenticationAttempt(t *testing.T) {
	t.Parallel()

	env := mustNewEnv()
	defer env.Close()

	s := env.S
	ctx := context.Background()

	const authenticatedDeviceID = "llama1"
	sampleToken := &devicepb.DeviceWebToken{
		WebSessionId:      "llama-session-id",
		BrowserUserAgent:  sampleUserAgent,
		BrowserIp:         sampleIP,
		User:              "llama",
		ExpectedDeviceIds: []string{authenticatedDeviceID, "llama2"},
	}

	createWebToken := func(t *testing.T) *devicepb.DeviceWebToken {
		webToken, err := s.CreateDeviceWebToken(ctx, sampleToken)
		if err != nil {
			t.Fatalf("CreateDeviceWebToken failed: %v", err)
		}
		return webToken
	}

	safeToken := base64.RawStdEncoding.EncodeToString([]byte(`value does not matter`))

	assertDeleted := func(t *testing.T, attemptID string) {
		if _, _, err := s.SpendDeviceWebToken(ctx, &devicepb.DeviceWebToken{
			Id:    attemptID,
			Token: safeToken,
		}, authenticatedDeviceID); !trace.IsNotFound(err) {
			t.Errorf("SpendDeviceWebToken returned err=%v (%T), want NotFound", err, trace.Unwrap(err))
		}
	}

	t.Run("delete web token", func(t *testing.T) {
		t.Parallel()

		webToken := createWebToken(t)
		if err := s.DeleteDeviceWebAuthenticationAttempt(ctx, webToken.Id); err != nil {
			t.Fatalf("DeleteDeviceWebAuthenticationAttempt failed: %v", err)
		}

		assertDeleted(t, webToken.Id)
	})

	t.Run("delete confirmation token", func(t *testing.T) {
		t.Parallel()

		// Create the DeviceConfirmationToken.
		webToken := createWebToken(t)
		_, confirmToken, err := s.SpendDeviceWebToken(ctx, webToken, authenticatedDeviceID)
		if err != nil {
			t.Fatalf("SpendDeviceWebToken failed: %v", err)
		}

		if err := s.DeleteDeviceWebAuthenticationAttempt(ctx, confirmToken.Id); err != nil {
			t.Fatalf("DeleteDeviceWebAuthenticationAttempt failed: %v", err)
		}

		assertDeleted(t, confirmToken.Id)
	})

	t.Run("not found", func(t *testing.T) {
		if err := s.DeleteDeviceWebAuthenticationAttempt(ctx, "unknown token ID"); !trace.IsNotFound(err) {
			t.Errorf("DeleteDeviceWebAuthenticationAttempt returned err=%v (%T), want NotFound", err, trace.Unwrap(err))
		}
	})
}

// diffDevices diffs two slices of devices, sorting both by ID first.
func diffDevices(want, got []*devicepb.Device) string {
	slices.SortFunc(want, func(a, b *devicepb.Device) int {
		return strings.Compare(a.Id, b.Id)
	})
	slices.SortFunc(got, func(a, b *devicepb.Device) int {
		return strings.Compare(a.Id, b.Id)
	})
	return cmp.Diff(want, got, protocmp.Transform())
}

// storageEnv groups the necessary components to test storage.
type storageEnv struct {
	// Clock is the underlying FakeClock.
	// nil if withClock() is used with a real clock.
	Clock           clockwork.FakeClock
	IdentityService *local.IdentityService
	S               *storage.S

	memClock clockwork.Clock // actual mem clock, always set.
	mem      *memory.Memory
}

func (e *storageEnv) Close() error {
	if e.mem != nil {
		return e.mem.Close()
	}
	return nil
}

type opt func(*storageEnv)

func withClock(clock clockwork.Clock) opt {
	return func(env *storageEnv) { env.memClock = clock }
}

func mustNewEnv(opts ...opt) *storageEnv {
	env, err := newEnv(opts...)
	if err != nil {
		panic(err)
	}
	return env
}

func newEnv(opts ...opt) (*storageEnv, error) {
	env := &storageEnv{}
	for _, opt := range opts {
		opt(env)
	}

	// Use a FakeClock if no clock was provided (via withClock), otherwise do our
	// best to honor the clock we got.
	// Initially storageEnv only allowed for a FakeClock, this was retrofited
	// later.
	if fakeClock, ok := env.memClock.(clockwork.FakeClock); ok {
		env.Clock = fakeClock
	} else if env.memClock == nil {
		env.Clock = clockwork.NewFakeClock()
		env.memClock = env.Clock
	}

	ok := false
	defer func() {
		if !ok {
			env.Close()
		}
	}()

	var err error
	env.mem, err = memory.New(memory.Config{
		Clock: env.memClock,
	})
	if err != nil {
		return nil, err
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError + 1, // Silence logging for tests.
	}))

	env.IdentityService = local.NewIdentityService(env.mem)
	env.S, err = storage.New(storage.Params{
		Logger:             logger,
		Backend:            env.mem,
		UsersService:       env.IdentityService,
		BCryptCostOverride: bcrypt.MinCost,
	})
	if err != nil {
		return nil, err
	}

	ok = true
	return env, nil
}
