package storage_test

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/devicetrust/storage"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils/clocki"
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
	_, err := s.CreateDevice(ctx, devicepb.Device_builder{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "alpaca",
	}.Build(), false /* createAsResource */)
	if err != nil {
		t.Fatalf("CreateDevice failed: %v", err)
	}

	const resource1Tag = "AAA000000000"
	const resource2Tag = "BBB000000000"
	_, pubKeyDER := newKeyPair(t)

	// resource1 is a complete, resource-like device.
	resource1 := devicepb.Device_builder{
		ApiVersion: "v1",
		Id:         "a6f76866-a9eb-4a23-9bb1-7980347a1bee",
		OsType:     devicepb.OSType_OS_TYPE_MACOS,
		AssetTag:   resource1Tag,
		CreateTime: timestamppb.New(time.Date(2023, 2, 24, 19, 0, 0, 0, time.UTC)),
		UpdateTime: timestamppb.New(time.Date(2023, 2, 24, 19, 15, 0, 500, time.UTC)),
		EnrollToken: devicepb.DeviceEnrollToken_builder{
			Token: "i-am-ignored",
		}.Build(),
		EnrollStatus: devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_ENROLLED,
		Credential: devicepb.DeviceCredential_builder{
			Id:           "ae2d978c-fee8-419d-a2e6-a5dd0a00c4b8",
			PublicKeyDer: pubKeyDER,
		}.Build(),
		CollectedData: []*devicepb.DeviceCollectedData{
			devicepb.DeviceCollectedData_builder{
				CollectTime:  timestamppb.New(time.Date(2023, 2, 24, 19, 0, 5, 0, time.UTC)),
				RecordTime:   timestamppb.New(time.Date(2023, 2, 24, 19, 0, 5, 500, time.UTC)),
				OsType:       devicepb.OSType_OS_TYPE_MACOS,
				SerialNumber: resource1Tag,
			}.Build(),
			devicepb.DeviceCollectedData_builder{
				CollectTime:  timestamppb.New(time.Date(2023, 2, 24, 19, 0, 15, 0, time.UTC)),
				RecordTime:   timestamppb.New(time.Date(2023, 2, 24, 19, 0, 15, 500, time.UTC)),
				OsType:       devicepb.OSType_OS_TYPE_MACOS,
				SerialNumber: resource1Tag,
			}.Build(),
		},
		Owner: "llama",
	}.Build()

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
				devicepb.Device_builder{
					OsType:   devicepb.OSType_OS_TYPE_MACOS,
					AssetTag: "llama",
				}.Build(),
				// NOK, duplicate within devs.
				devicepb.Device_builder{
					OsType:   devicepb.OSType_OS_TYPE_MACOS,
					AssetTag: "llama",
				}.Build(),
				// NOK, duplicate in storage.
				devicepb.Device_builder{
					OsType:   devicepb.OSType_OS_TYPE_MACOS,
					AssetTag: "alpaca",
				}.Build(),
				// NOK, duplicate within devs (and in storage).
				devicepb.Device_builder{
					OsType:   devicepb.OSType_OS_TYPE_MACOS,
					AssetTag: "alpaca",
				}.Build(),
				// OK.
				devicepb.Device_builder{
					OsType:   devicepb.OSType_OS_TYPE_MACOS,
					AssetTag: "camel",
				}.Build(),
				// NOK, invalid OsType.
				devicepb.Device_builder{
					OsType:   devicepb.OSType_OS_TYPE_UNSPECIFIED,
					AssetTag: "cat",
				}.Build(),
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
					llamaDev.GetId(): "llama",
					camelDev.GetId(): "camel",
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
				devicepb.Device_builder{
					OsType:   devicepb.OSType_OS_TYPE_MACOS,
					AssetTag: resource2Tag,
				}.Build(),
				// NOK: Invalid resource: missing mandatory field.
				devicepb.Device_builder{
					OsType:   devicepb.OSType_OS_TYPE_MACOS,
					AssetTag: "",
				}.Build(),
				// NOK: Invalid resource: failed resource-like validation.
				devicepb.Device_builder{
					ApiVersion: "v99", // invalid
					OsType:     devicepb.OSType_OS_TYPE_MACOS,
					AssetTag:   "XXXXXXXXXXXX",
				}.Build(),
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
				if resOrStatus1.GetId() != resource1.GetId() {
					t.Errorf("BulkCreateDevices: got[0].Id = %v, want %v", resOrStatus1.GetId(), resource1.GetId())
				}

				resource2 := got[1]
				return map[string]string{
					resource1.GetId(): resource1.GetAssetTag(), // ID is the same as requested
					resource2.GetId(): resource2Tag,
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
				gotDevs[d.GetId()] = d.GetAssetTag()
			}
			wantDevs := make(map[string]string)
			for _, d := range devsBefore { // Fill want with existing devs...
				wantDevs[d.GetId()] = d.GetAssetTag()
			}
			maps.Copy(wantDevs, test.wantDevices(t, got))
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

	resource1Dev := devicepb.Device_builder{
		ApiVersion: "v1",
		Id:         "a6f76866-a9eb-4a23-9bb1-7980347a1bee",
		OsType:     devicepb.OSType_OS_TYPE_MACOS,
		AssetTag:   resource1Tag,
		CreateTime: timestamppb.New(time.Date(2023, 2, 24, 19, 0, 0, 0, time.UTC)),
		UpdateTime: timestamppb.New(time.Date(2023, 2, 24, 19, 15, 0, 500, time.UTC)),
		EnrollToken: devicepb.DeviceEnrollToken_builder{
			Token: "i-am-ignored",
		}.Build(),
		EnrollStatus: devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_ENROLLED,
		Credential: devicepb.DeviceCredential_builder{
			Id:           "ae2d978c-fee8-419d-a2e6-a5dd0a00c4b8",
			PublicKeyDer: pubKeyDER,
		}.Build(),
		// CollectedData is ignored on Create.
		CollectedData: []*devicepb.DeviceCollectedData{
			devicepb.DeviceCollectedData_builder{
				CollectTime:  timestamppb.New(time.Date(2023, 2, 24, 19, 0, 5, 0, time.UTC)),
				RecordTime:   timestamppb.New(time.Date(2023, 2, 24, 19, 0, 5, 500, time.UTC)),
				OsType:       devicepb.OSType_OS_TYPE_MACOS,
				SerialNumber: resource1Tag,
			}.Build(),
			devicepb.DeviceCollectedData_builder{
				CollectTime:  nil,                           // invalid
				RecordTime:   nil,                           // invalid for a resource
				OsType:       devicepb.OSType_OS_TYPE_LINUX, // invalid
				SerialNumber: "ignored",                     // invalid
			}.Build(),
		},
		Source: devicepb.DeviceSource_builder{
			Name:   "myscript",
			Origin: devicepb.DeviceOrigin_DEVICE_ORIGIN_API,
		}.Build(),
		Profile: devicepb.DeviceProfile_builder{
			UpdateTime:          timestamppb.Now(),
			ModelIdentifier:     "MacBookPro9,2",
			OsVersion:           "13.4.1",
			OsBuild:             "22F82",
			OsBuildSupplemental: "22F770820d",
			OsUsernames:         []string{"admin", "codingllama", "alpaca"},
			JamfBinaryVersion:   "10.44.1-t1677509507",
			ExternalId:          "99",
		}.Build(),
		Owner: "llama",
	}.Build()

	tests := []struct {
		name             string
		dev              *devicepb.Device
		createAsResource bool
		modifyWant       func(dev, want *devicepb.Device) // adjust want for `createAsResource` tests.
	}{
		{
			name: "ok",
			dev: devicepb.Device_builder{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "dev1",
			}.Build(),
		},
		{
			name: "ok with source and profile",
			dev: devicepb.Device_builder{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "dev2",
				Source: devicepb.DeviceSource_builder{
					Name:   "mysource",
					Origin: devicepb.DeviceOrigin_DEVICE_ORIGIN_API,
				}.Build(),
				Profile: devicepb.DeviceProfile_builder{
					ModelIdentifier:   "MacBookPro9,2",
					OsVersion:         "13.2.1",
					OsBuild:           "22D68",
					OsUsernames:       []string{"admin", "alpaca"},
					JamfBinaryVersion: "10.44.1",
					ExternalId:        "99",
				}.Build(),
			}.Build(),
		},
		{
			name: "ok with supplemental build",
			dev: devicepb.Device_builder{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "devSup1",
				Profile: devicepb.DeviceProfile_builder{
					OsVersion:           "13.4.1",
					OsBuild:             "22F82",
					OsBuildSupplemental: "22F770820d",
				}.Build(),
			}.Build(),
		},
		{
			name:             "create as a resource",
			dev:              resource1Dev,
			createAsResource: true,
			modifyWant: func(dev, want *devicepb.Device) {
				// All fields that are typically system-generated are copied from `dev`.
				want.SetId(dev.GetId())
				want.SetCreateTime(dev.GetCreateTime())
				want.SetUpdateTime(dev.GetUpdateTime())
				want.SetEnrollStatus(dev.GetEnrollStatus())
				want.SetCredential(dev.GetCredential())
				want.SetOwner(dev.GetOwner())

				// Ignored on pure Create/Update methods.
				want.SetCollectedData(nil)
			},
		},
		{
			name: "partial create as a resource", // Only required fields set.
			dev: devicepb.Device_builder{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: resource2Tag,
			}.Build(),
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
			if got.GetId() == "" {
				t.Fatal("CreateDevice: Id empty")
			}
			if got.GetApiVersion() == "" {
				t.Error("CreateDevice: ApiVersion empty")
			}
			if !got.HasCreateTime() {
				t.Error("CreateDevice: CreateTime nil")
			}
			if !got.HasUpdateTime() {
				t.Error("CreateDevice: UpdateTime nil")
			}
			if got.HasCreateTime() && got.HasUpdateTime() && got.GetCreateTime().AsTime().After(got.GetUpdateTime().AsTime()) {
				t.Errorf("CreateDevice: CreateTime (%v) is after UpdateTime (%v)", got.GetCreateTime(), got.GetUpdateTime())
			}
			if m := storage.MaxCollectedDataPerDevice + 1; len(got.GetCollectedData()) > m {
				t.Errorf("CreateDevice: got %v CollectedData entries, want <= %v", len(got.GetCollectedData()), m)
			}

			want := devicepb.Device_builder{
				ApiVersion:   got.GetApiVersion(),
				Id:           got.GetId(),
				OsType:       test.dev.GetOsType(),
				AssetTag:     test.dev.GetAssetTag(),
				CreateTime:   got.GetCreateTime(),
				UpdateTime:   got.GetUpdateTime(),
				EnrollStatus: devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED,
				Source:       test.dev.GetSource(),
				Profile:      test.dev.GetProfile(),
			}.Build()
			if want.HasProfile() {
				want.GetProfile().SetUpdateTime(got.GetProfile().GetUpdateTime()) // System-managed
			}
			if test.modifyWant != nil {
				test.modifyWant(test.dev, want)
			}
			// Sanity check: wanted collected data is within the limits of
			// MaxCollectedDataPerDevice.
			if m := storage.MaxCollectedDataPerDevice + 1; len(want.GetCollectedData()) > m {
				t.Errorf("want.CollectedData has %v entries, it should have at most %v entries", len(got.GetCollectedData()), m)
			}
			if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
				t.Errorf("CreateDevice: mismatch (-want +got):\n%s", diff)
			}

			// Verify that device is present is storage.
			stored, err := s.GetDeviceByID(ctx, got.GetId())
			if err != nil {
				t.Fatalf("GetDeviceByID failed: %v", err)
			}
			if diff := cmp.Diff(got, stored, protocmp.Transform()); diff != "" {
				t.Errorf("GetDeviceByID: mismatch (-want +got):\n%s", diff)
			}

			// Verify that the asset tag index is up-to-date.
			gotDevs, err := s.GetDevicesByAssetTag(ctx, got.GetAssetTag())
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

	devMac := devicepb.Device_builder{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "llama",
	}.Build()
	devLinux := proto.Clone(devMac).(*devicepb.Device)
	devLinux.SetOsType(devicepb.OSType_OS_TYPE_LINUX)
	devWin := proto.Clone(devMac).(*devicepb.Device)
	devWin.SetOsType(devicepb.OSType_OS_TYPE_WINDOWS)

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
	got, err := s.GetDevicesByAssetTag(ctx, devMac.GetAssetTag())
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
			if _, err := s.CreateDevice(ctx, devicepb.Device_builder{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "llama",
			}.Build(), false /* createAsResource */); err != nil && !trace.IsAlreadyExists(err) {
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
	validDev := devicepb.Device_builder{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: knownTag,
	}.Build()
	ctx := context.Background()
	if _, err := s.CreateDevice(ctx, validDev, false /* createAsResource */); err != nil {
		t.Fatalf("CreateDevice failed: %v", err)
	}

	// Switch tags so the device would be perfectly valid for creation.
	const otherTag = "alpaca"
	validDev.SetAssetTag(otherTag)

	validProfile := devicepb.DeviceProfile_builder{
		ModelIdentifier:   "MacBookPro9,3",
		OsVersion:         "13.2.1",
		OsBuild:           "22D68",
		OsUsernames:       []string{"admin", "llama"},
		JamfBinaryVersion: "9.27",
	}.Build()

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
				d.SetOsType(devicepb.OSType_OS_TYPE_UNSPECIFIED)
				return d
			},
			wantErr:   "os_type",
			assertErr: trace.IsBadParameter,
		},
		{
			name: "asset_tag empty",
			createDev: func() *devicepb.Device {
				d := proto.Clone(validDev).(*devicepb.Device)
				d.SetAssetTag("")
				return d
			},
			wantErr:   "asset_tag",
			assertErr: trace.IsBadParameter,
		},
		{
			name: "asset_tag length",
			createDev: func() *devicepb.Device {
				d := proto.Clone(validDev).(*devicepb.Device)
				d.SetAssetTag(strings.Repeat("A", 121))
				return d
			},
			wantErr:   "asset_tag exceeds",
			assertErr: trace.IsBadParameter,
		},
		{
			name: "asset_tag registered",
			createDev: func() *devicepb.Device {
				d := proto.Clone(validDev).(*devicepb.Device)
				d.SetAssetTag(knownTag)
				return d
			},
			wantErr:   "already registered",
			assertErr: trace.IsAlreadyExists,
		},
		{
			name: "resource: invalid api_version",
			createDev: func() *devicepb.Device {
				d := proto.Clone(validDev).(*devicepb.Device)
				d.SetApiVersion("v99")
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
				d.SetId("banana")
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
				d.SetAssetTag("")
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
				d.SetOsType(devicepb.OSType_OS_TYPE_UNSPECIFIED)
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
				d.SetCreateTime(&timestamppb.Timestamp{
					Nanos: -1,
				})
				d.SetUpdateTime(timestamppb.Now()) // both timestamps must be set
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
				d.SetCreateTime(timestamppb.Now()) // both timestamps must be set
				d.SetUpdateTime(&timestamppb.Timestamp{
					Nanos: -1,
				})
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
				d.SetCreateTime(timestamppb.Now())
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
				d.SetUpdateTime(timestamppb.Now())
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
				d.SetCreateTime(timestamppb.New(t2))
				d.SetUpdateTime(timestamppb.New(t1))
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
				d.SetCredential(&devicepb.DeviceCredential{
					// Missing all fields.
				})
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
				d.SetSource(devicepb.DeviceSource_builder{
					Origin: devicepb.DeviceOrigin_DEVICE_ORIGIN_JAMF,
				}.Build())
				return d
			},
			wantErr:   "source name",
			assertErr: trace.IsBadParameter,
		},
		{
			name: "source.origin unspecified",
			createDev: func() *devicepb.Device {
				d := proto.Clone(validDev).(*devicepb.Device)
				d.SetSource(devicepb.DeviceSource_builder{
					Name: "mysource",
				}.Build())
				return d
			},
			wantErr:   "source origin",
			assertErr: trace.IsBadParameter,
		},
		{
			name: "profile.os_version macOS not a semver",
			createDev: func() *devicepb.Device {
				p := proto.Clone(validProfile).(*devicepb.DeviceProfile)
				p.SetOsVersion("NOT A SEMVER")

				d := proto.Clone(validDev).(*devicepb.Device)
				d.SetOsType(devicepb.OSType_OS_TYPE_MACOS) // important for this test
				d.SetProfile(p)
				return d
			},
			wantErr:   "not a valid semver",
			assertErr: trace.IsBadParameter,
		},
		{
			name: "profile.os_usernames empty value",
			createDev: func() *devicepb.Device {
				p := proto.Clone(validProfile).(*devicepb.DeviceProfile)
				p.SetOsUsernames([]string{
					"admin",
					"", // invalid
					"alpaca",
				})

				d := proto.Clone(validDev).(*devicepb.Device)
				d.SetProfile(p)
				return d
			},
			wantErr:   "username",
			assertErr: trace.IsBadParameter,
		},
		{
			name: "profile.jamf_binary_version not a semver",
			createDev: func() *devicepb.Device {
				p := proto.Clone(validProfile).(*devicepb.DeviceProfile)
				p.SetJamfBinaryVersion("NOT A SEMVER")

				d := proto.Clone(validDev).(*devicepb.Device)
				d.SetProfile(p)
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
	enrolledDev, _, err := createAndEnroll(ctx, s, devicepb.Device_builder{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "llama",
	}.Build(), owner)
	if err != nil {
		t.Fatalf("createAndEnroll failed: %v", err)
	}

	validSource := devicepb.DeviceSource_builder{
		Name:   "jamf",
		Origin: devicepb.DeviceOrigin_DEVICE_ORIGIN_JAMF,
	}.Build()
	validProfile := devicepb.DeviceProfile_builder{
		ModelIdentifier:   "MacBookPro9,2",
		OsVersion:         "13.2.1",
		OsBuild:           "22D68",
		OsUsernames:       []string{"alpaca"},
		JamfBinaryVersion: "10.44.1-t1677509507",
	}.Build()

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
			baseDev: devicepb.Device_builder{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "llama1",
			}.Build(),
			update: func(stored *devicepb.Device) *devicepb.Device {
				// No changes.
				return stored
			},
			assertUpdate: assertNoop,
		},
		{
			name: "transient fields ignored",
			baseDev: devicepb.Device_builder{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "llama2",
			}.Build(),
			update: func(stored *devicepb.Device) *devicepb.Device {
				now := timestamppb.Now()
				stored.SetEnrollToken(devicepb.DeviceEnrollToken_builder{
					Token: "insert enrollment token here",
				}.Build())
				stored.SetCollectedData([]*devicepb.DeviceCollectedData{
					devicepb.DeviceCollectedData_builder{
						CollectTime:  now,
						RecordTime:   now,
						OsType:       stored.GetOsType(),
						SerialNumber: stored.GetAssetTag(),
					}.Build(),
				})
				return stored
			},
			assertUpdate: assertNoop,
		},
		{
			name:    "unenroll",
			baseDev: enrolledDev,
			update: func(stored *devicepb.Device) *devicepb.Device {
				stored.SetEnrollStatus(devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED)
				return stored
			},
			assertUpdate: func(t *testing.T, base, updated *devicepb.Device) {
				if proto.Equal(base.GetUpdateTime(), updated.GetUpdateTime()) {
					t.Error("UpdateDevice: update time didn't change")
				}

				want := proto.Clone(base).(*devicepb.Device)
				want.SetUpdateTime(updated.GetUpdateTime())
				want.SetEnrollStatus(devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED)
				want.ClearCredential() // credential automatically cleared
				want.SetOwner("")      // owner automatically cleared
				if diff := cmp.Diff(want, updated, protocmp.Transform()); diff != "" {
					t.Errorf("UpdateDevice mismatch (-want +got)\n%s", diff)
				}
			},
		},
		{
			name: "set source and profile",
			baseDev: devicepb.Device_builder{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "mdmfields1",
			}.Build(),
			update: func(stored *devicepb.Device) *devicepb.Device {
				stored.SetSource(validSource)
				stored.SetProfile(validProfile)
				return stored
			},
			assertUpdate: func(t *testing.T, base, updated *devicepb.Device) {
				p := proto.Clone(validProfile).(*devicepb.DeviceProfile)

				// Copy system-assigned profile update time.
				if !updated.HasProfile() || !updated.GetProfile().HasUpdateTime() {
					t.Error("UpdateDevice: profile UpdateTime not assigned")
				} else {
					p.SetUpdateTime(updated.GetProfile().GetUpdateTime())
				}

				want := proto.Clone(updated).(*devicepb.Device)
				want.SetSource(validSource)
				want.SetProfile(p)

				if diff := cmp.Diff(want, updated, protocmp.Transform()); diff != "" {
					t.Errorf("UpdateDevice mismatch (-want +got)\n%s", diff)
				}
			},
		},
		{
			name: "DeviceProfile.UpdateTime ignored for noop",
			baseDev: devicepb.Device_builder{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "mdmfields2",
				Profile: devicepb.DeviceProfile_builder{
					ModelIdentifier: "MacBookPro9,2",
				}.Build(),
			}.Build(),
			update: func(stored *devicepb.Device) *devicepb.Device {
				// nil UpdateTime is allowed.
				// Everything else is the same, so this shouldn't cause an update.
				stored.GetProfile().ClearUpdateTime()
				return stored
			},
			assertUpdate: assertNoop,
		},
		{
			name: "Added empty profile ignored",
			baseDev: devicepb.Device_builder{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "mdmfields3",
			}.Build(),
			update: func(stored *devicepb.Device) *devicepb.Device {
				stored.SetProfile(devicepb.DeviceProfile_builder{
					// UpdateTime, by itself, doesn't qualify the profile as non-empty.
					UpdateTime: timestamppb.Now(),
				}.Build())
				return stored
			},
			assertUpdate: assertNoop,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Create baseDev, if necessary.
			baseDev := test.baseDev
			if baseDev.GetId() == "" {
				var err error
				baseDev, err = s.CreateDevice(ctx, baseDev, false /* createAsResource */)
				if err != nil {
					t.Fatalf("CreateDevice failed: %v", err)
				}
			}

			// Allow update time to change.
			clock.Advance(1 * time.Second)

			updated, err := s.UpdateDevice(ctx, baseDev.GetId(), test.update)
			if err != nil {
				t.Fatalf("UpdateDevice failed: %v", err)
			}
			// Has the update time regressed?
			if got, want := updated.GetUpdateTime().AsTime(), baseDev.GetUpdateTime().AsTime(); want.After(got) {
				t.Errorf("UpdateDevice: got updated.UpdateTime = %v, want >= %v", got, want)
			}
			test.assertUpdate(t, baseDev, updated)

			// Verify stored device.
			stored, err := s.GetDeviceByID(ctx, updated.GetId())
			if err != nil {
				t.Fatalf("GetDeviceByID failed: %v", err)
			}
			cd := stored.GetCollectedData()
			stored.SetCollectedData(nil) // not returned by UpdateDevice
			if diff := cmp.Diff(updated, stored, protocmp.Transform()); diff != "" {
				t.Errorf("GetDeviceByID mismatch (-want +got)\n%s", diff)
			}

			// Verify collected data deletion on unenroll.
			if updated.GetEnrollStatus() == devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED && len(cd) > 0 {
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

	baseDev, err := s.CreateDevice(ctx, devicepb.Device_builder{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "llama",
	}.Build(), false /* createAsResource */)
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
			deviceID: baseDev.GetId(),
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
			deviceID: baseDev.GetId(),
			update: func(stored *devicepb.Device) *devicepb.Device {
				stored.SetApiVersion("v9999")
				return stored
			},
			wantErr: "api_version",
		},
		{
			name:     "Id readonly",
			deviceID: baseDev.GetId(),
			update: func(stored *devicepb.Device) *devicepb.Device {
				stored.SetId("another Id")
				return stored
			},
			wantErr: "id is readonly",
		},
		{
			name:     "OsType readonly",
			deviceID: baseDev.GetId(),
			update: func(stored *devicepb.Device) *devicepb.Device {
				stored.SetOsType(devicepb.OSType_OS_TYPE_WINDOWS)
				return stored
			},
			wantErr: "os_type",
		},
		{
			name:     "AssetTag readonly",
			deviceID: baseDev.GetId(),
			update: func(stored *devicepb.Device) *devicepb.Device {
				stored.SetAssetTag("another tag")
				return stored
			},
			wantErr: "asset_tag",
		},
		{
			name:     "CreateTime readonly",
			deviceID: baseDev.GetId(),
			update: func(stored *devicepb.Device) *devicepb.Device {
				stored.SetCreateTime(&timestamppb.Timestamp{
					Seconds: stored.GetCreateTime().Seconds - 2, // move to the past, so CreateTime <= UpdateTime
					Nanos:   stored.GetCreateTime().Nanos,
				})
				return stored
			},
			wantErr: "create_time",
		},
		{
			name:     "UpdateTime readonly",
			deviceID: baseDev.GetId(),
			update: func(stored *devicepb.Device) *devicepb.Device {
				stored.SetUpdateTime(&timestamppb.Timestamp{
					Seconds: stored.GetUpdateTime().Seconds + 2, // move to the future, so CreateTime <= UpdateTime
					Nanos:   stored.GetUpdateTime().Nanos,
				})
				return stored
			},
			wantErr: "update_time",
		},
		{
			name:     "Credential readonly",
			deviceID: baseDev.GetId(),
			update: func(stored *devicepb.Device) *devicepb.Device {
				stored.SetCredential(devicepb.DeviceCredential_builder{
					Id:           uuid.NewString(),
					PublicKeyDer: []byte("insert public key here"),
				}.Build())
				return stored
			},
			wantErr: "credential",
		},
		{
			name:     "EnrollStatus can't transition to UNSPECIFIED",
			deviceID: baseDev.GetId(),
			update: func(stored *devicepb.Device) *devicepb.Device {
				stored.SetEnrollStatus(devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_UNSPECIFIED)
				return stored
			},
			wantErr: "enroll_status",
		},
		{
			name:     "EnrollStatus can't transition to ENROLLED",
			deviceID: baseDev.GetId(),
			update: func(stored *devicepb.Device) *devicepb.Device {
				stored.SetEnrollStatus(devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_ENROLLED)
				return stored
			},
			wantErr: "enroll_status",
		},
		{
			name:     "source is validated",
			deviceID: baseDev.GetId(),
			update: func(stored *devicepb.Device) *devicepb.Device {
				stored.SetSource(devicepb.DeviceSource_builder{
					Origin: devicepb.DeviceOrigin_DEVICE_ORIGIN_JAMF,
				}.Build())
				return stored
			},
			wantErr: "source name",
		},
		{
			name:     "profile is validated",
			deviceID: baseDev.GetId(),
			update: func(stored *devicepb.Device) *devicepb.Device {
				stored.SetProfile(devicepb.DeviceProfile_builder{
					JamfBinaryVersion: "NOT A SEMVER",
				}.Build())
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
	llama, err := s.CreateDevice(ctx, devicepb.Device_builder{OsType: devicepb.OSType_OS_TYPE_MACOS, AssetTag: "llama"}.Build(), createAsResource)
	if err != nil {
		t.Fatalf("CreateDevice failed: %v", err)
	}
	dev1, err := s.CreateDevice(ctx, devicepb.Device_builder{OsType: devicepb.OSType_OS_TYPE_MACOS, AssetTag: "dev1"}.Build(), createAsResource)
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
			deviceID:  llama.GetId(),
			predicate: func(_ *devicepb.Device) error { return nil },
		},
		{
			name:      "predicate doesn't match",
			deviceID:  dev1.GetId(),
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
		dev, err := s.CreateDevice(ctx, devicepb.Device_builder{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: assetTag,
		}.Build(), false /* createAsResource */)
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
			deviceID:  d1.GetId(),
			assertErr: func(err error) bool { return err == nil },
		},
		{
			name:      "already deleted device fails with not found",
			deviceID:  d1.GetId(),
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

	llamaMac := devicepb.Device_builder{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "llama",
	}.Build()
	llamaLinux := devicepb.Device_builder{
		OsType:   devicepb.OSType_OS_TYPE_LINUX,
		AssetTag: "llama",
	}.Build()
	llamaWin := devicepb.Device_builder{
		OsType:   devicepb.OSType_OS_TYPE_WINDOWS,
		AssetTag: "llama",
	}.Build()
	unrelatedDev := devicepb.Device_builder{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "unrelated",
	}.Build()
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
	if err := s.DeleteDevice(ctx, llamaMac.GetId()); err != nil {
		t.Fatalf("DeleteDevice failed: %v", err)
	}

	// Sanity check: device is not in storage anymore.
	if _, err := s.GetDeviceByID(ctx, llamaMac.GetId()); !trace.IsNotFound(err) {
		t.Fatalf("GetDeviceByID returned an unexpected error: %v (want not found)", err)
	}

	// Verify asset tag read.
	gotTagDevs, err := s.GetDevicesByAssetTag(ctx, llamaMac.GetAssetTag())
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
		devicepb.Device_builder{OsType: devicepb.OSType_OS_TYPE_MACOS, AssetTag: "llama"}.Build(),
		devicepb.Device_builder{OsType: devicepb.OSType_OS_TYPE_WINDOWS, AssetTag: "llama"}.Build(),
		devicepb.Device_builder{OsType: devicepb.OSType_OS_TYPE_MACOS, AssetTag: "alpaca"}.Build(),
		devicepb.Device_builder{OsType: devicepb.OSType_OS_TYPE_MACOS, AssetTag: "dev1"}.Build(),
		devicepb.Device_builder{OsType: devicepb.OSType_OS_TYPE_MACOS, AssetTag: "dev2"}.Build(),
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
	if err := env.mem.Delete(ctx, backend.NewKey("devices", "id", dev2.GetId())); err != nil {
		t.Fatalf("Direct deletion of %q failed: %v", dev2.GetAssetTag(), err)
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
			osType:   alpaca.GetOsType(),
			assetTag: alpaca.GetAssetTag(),
			wantID:   alpaca.GetId(),
		},
		{
			name:            "ok with verifyExistence=true",
			osType:          alpaca.GetOsType(),
			assetTag:        alpaca.GetAssetTag(),
			verifyExistence: true,
			wantID:          alpaca.GetId(),
		},
		{
			name:     "macOS with conflicting asset tag",
			osType:   llama.GetOsType(),
			assetTag: llama.GetAssetTag(), // same AssetTag as llamaWin
			wantID:   llama.GetId(),
		},
		{
			name:     "Windows with conflicting asset tag",
			osType:   llamaWin.GetOsType(),
			assetTag: llamaWin.GetAssetTag(), // same AssetTag as llama
			wantID:   llamaWin.GetId(),
		},
		{
			name:      "unknown os_type not found",
			osType:    devicepb.OSType_OS_TYPE_LINUX,
			assetTag:  llama.GetAssetTag(),
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
			osType:   dev2.GetOsType(),
			assetTag: dev2.GetAssetTag(),
			wantID:   dev2.GetId(),
		},
		{
			name:            "verifyExistence=true fails if only mapping exist",
			osType:          dev2.GetOsType(),
			assetTag:        dev2.GetAssetTag(),
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

func TestS_GetUserTrustedDeviceIDs(t *testing.T) {
	t.Parallel()

	env := mustNewEnv()
	defer env.Close()

	s := env.S
	ctx := context.Background()

	const userLlama = "llama"
	const userAlpaca = "alpaca"
	const userNoDevices = "noDevices"

	for _, user := range []string{userLlama, userAlpaca, userNoDevices} {
		u, err := types.NewUser(user)
		if err != nil {
			t.Fatalf("NewUser failed: %v", err)
		}
		if _, err := env.IdentityService.CreateUser(ctx, u); err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}
	}

	// Assign via enrollment.
	llama1, _, err := createAndEnroll(ctx, s, devicepb.Device_builder{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "llama1",
	}.Build(), userLlama)
	if err != nil {
		t.Fatalf("createAndEnroll failed: %v", err)
	}

	// Assign via AssignDeviceOwner.
	var devices []*devicepb.Device
	for _, spec := range []struct {
		osType   devicepb.OSType
		assetTag string
		owner    string
	}{
		{
			osType:   devicepb.OSType_OS_TYPE_MACOS,
			assetTag: "llama2",
			owner:    userLlama,
		},
		{
			osType:   devicepb.OSType_OS_TYPE_WINDOWS,
			assetTag: "llama3",
			owner:    userLlama,
		},
		{
			osType:   devicepb.OSType_OS_TYPE_MACOS,
			assetTag: "alpaca1",
			owner:    userAlpaca,
		},
	} {
		dev, err := s.CreateDevice(ctx, devicepb.Device_builder{
			OsType:   spec.osType,
			AssetTag: spec.assetTag,
		}.Build(), false /* createAsResource */)
		if err != nil {
			t.Fatalf("CreateDevice failed: %v", err)
		}
		dev, err = s.AssignDeviceOwner(ctx, dev.GetId(), spec.owner)
		if err != nil {
			t.Fatalf("AssignDeviceOwner failed: %v", err)
		}
		devices = append(devices, dev)
	}
	llama2 := devices[0]
	llama3 := devices[1]
	alpaca1 := devices[2]

	// Assign and unassign noDevices.
	noDevices1, _, err := createAndEnroll(ctx, s, devicepb.Device_builder{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "noDevices1",
		Owner:    userNoDevices,
	}.Build(), userNoDevices)
	if err != nil {
		t.Fatalf("createAndEnroll failed: %v", err)
	}
	if _, err := s.UpdateDevice(ctx, noDevices1.GetId(), func(stored *devicepb.Device) *devicepb.Device {
		stored.SetEnrollStatus(devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED)
		return stored
	}); err != nil {
		t.Fatalf("UpdateDevice failed: %v", err)
	}

	tests := []struct {
		name          string
		user          string
		wantDeviceIDs []string
	}{
		{
			name: "multiple devices",
			user: userLlama,
			wantDeviceIDs: []string{
				llama1.GetId(),
				llama2.GetId(),
				llama3.GetId(),
			},
		},
		{
			name: "single device",
			user: userAlpaca,
			wantDeviceIDs: []string{
				alpaca1.GetId(),
			},
		},
		{
			name: "all devices unassigned",
			user: userNoDevices,
		},
		{
			name: "unknown user",
			user: "unknown",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := s.GetUserTrustedDeviceIDs(ctx, test.user)
			if err != nil {
				t.Fatalf("GetUserTrustedDeviceIDs failed: %v", err)
			}

			want := test.wantDeviceIDs
			slices.Sort(want)
			slices.Sort(got)
			if diff := cmp.Diff(want, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("GetUserTrustedDeviceIDs mismatch (-want +got)\n%s", diff)
			}
		})
	}
}

func TestS_ListDevicesByUser(t *testing.T) {
	t.Parallel()

	env := mustNewEnv()
	defer env.Close()

	s := env.S
	ctx := context.Background()
	const user = "llama"
	const otherUser = "otherUser"

	var userDevices []*devicepb.Device
	for i, assetTag := range []string{"phone", "laptop", "tablet"} {
		dev, _, err := createAndEnroll(ctx, s, devicepb.Device_builder{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: fmt.Sprintf("%s-%d", user, i),
		}.Build(), user)
		if err != nil {
			t.Fatalf("CreateDevice(%q) failed: %v", assetTag, err)
		}
		userDevices = append(userDevices, dev)
	}

	// Add devices owned by other users to ensure they are not returned.
	for i, assetTag := range []string{"otheruser-device1", "otheruser-device2"} {
		_, _, err := createAndEnroll(ctx, s, devicepb.Device_builder{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: fmt.Sprintf("%s-%d", otherUser, i),
		}.Build(), otherUser)
		if err != nil {
			t.Fatalf("CreateDevice(%q) failed: %v", assetTag, err)
		}
	}

	slices.SortFunc(userDevices, func(a, b *devicepb.Device) int {
		return strings.Compare(a.GetId(), b.GetId())
	})

	t.Run("iterate until no nextPageToken", func(t *testing.T) {
		const defaultPageSize = 10

		nextPageToken := ""
		var gotDevices []*devicepb.Device

		for {
			devs, gotNextToken, err := s.ListDevicesByUser(ctx, defaultPageSize, nextPageToken, user)
			if err != nil {
				t.Fatalf("ListDevicesByUser failed: %v", err)
			}

			gotDevices = append(gotDevices, devs...)

			nextPageToken = gotNextToken

			if nextPageToken == "" {
				break
			}
		}

		// make sure collected data exists, but don't check specifics
		for i, dev := range gotDevices {
			if dev.GetCollectedData() == nil {
				t.Errorf("ListDevicesByUser device response missing collected data\nindex: %d\ndevice: %v", i, dev)
			}
			dev.SetCollectedData(nil)
		}

		diff := cmp.Diff(userDevices, gotDevices, protocmp.Transform())
		if diff != "" {
			t.Errorf("ListDevicesByUser mismatch (-want +got):\n%s", diff)
		}
	})
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
		dev, err := s.CreateDevice(ctx, devicepb.Device_builder{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: assetTag,
		}.Build(), false /* createAsResource */)
		if err != nil {
			t.Fatalf("CreateDevice(%q) failed: %v", assetTag, err)
		}
		fullDevs = append(fullDevs, dev)
	}

	// Create a couple of enrollment tokens, so we can make sure they don't
	// pollute the results.
	for _, deviceID := range []string{fullDevs[0].GetId(), fullDevs[1].GetId()} {
		if _, err := s.CreateDeviceEnrollToken(ctx, deviceID, time.Time{} /* expiresAt */); err != nil {
			t.Fatalf("CreateDeviceEnrollToken failed: %v", err)
		}
	}

	// Transform "fullDevs" into its "list" view equivalent.
	listDevs := make([]*devicepb.Device, len(fullDevs))
	for i, dev := range fullDevs {
		listDevs[i] = devicepb.Device_builder{
			ApiVersion:   dev.GetApiVersion(),
			Id:           dev.GetId(),
			OsType:       dev.GetOsType(),
			AssetTag:     dev.GetAssetTag(),
			CreateTime:   dev.GetCreateTime(),
			UpdateTime:   dev.GetUpdateTime(),
			EnrollStatus: dev.GetEnrollStatus(),
		}.Build()
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

	dev, err := s.CreateDevice(ctx, devicepb.Device_builder{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "llama",
	}.Build(), false /* createAsResource */)
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
			cred: devicepb.DeviceCredential_builder{
				Id:           "cred1",
				PublicKeyDer: key1DER,
			}.Build(),
			cd: devicepb.DeviceCollectedData_builder{
				CollectTime:  timestamppb.Now(),
				OsType:       dev.GetOsType(),
				SerialNumber: dev.GetAssetTag(),
			}.Build(),
			owner: "llama",
		},
		{
			// Note: this test case depends on the device being successfully enrolled
			// above.
			name:    "re-enroll",
			baseDev: dev,
			cred: devicepb.DeviceCredential_builder{
				Id:           "cred2",
				PublicKeyDer: key2DER,
			}.Build(),
			cd: devicepb.DeviceCollectedData_builder{
				CollectTime:  timestamppb.Now(),
				OsType:       dev.GetOsType(),
				SerialNumber: dev.GetAssetTag(),
			}.Build(),
			owner: "alpaca",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			deviceID := test.baseDev.GetId()
			got, err := s.EnrollDevice(ctx, deviceID, test.cred, test.cd, test.owner)
			if err != nil {
				t.Fatalf("EnrollDevice failed: %v", err)
			}

			if got.GetUpdateTime().AsTime().Before(test.baseDev.GetUpdateTime().AsTime()) {
				t.Errorf("got.UpdateTime = %v, want >= %v", got.GetUpdateTime(), test.baseDev.GetUpdateTime())
			}

			want := proto.Clone(test.baseDev).(*devicepb.Device)
			want.SetUpdateTime(got.GetUpdateTime())
			want.SetEnrollStatus(devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_ENROLLED)
			want.SetCredential(test.cred)
			want.SetOwner(test.owner)
			if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
				t.Errorf("EnrollDevice mismatch (-want +got):\n%s", diff)
			}

			// Are changes reflected in storage?
			stored, err := s.GetDeviceByID(ctx, deviceID)
			if err != nil {
				t.Fatalf("GetDeviceByID failed: %v", err)
			}
			// TODO(codingllama): Assert collected data on tests.
			stored.SetCollectedData(nil)
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
	dev, _, err := createAndEnroll(ctx, s, devicepb.Device_builder{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "llama1",
	}.Build(), user1)
	if err != nil {
		t.Fatalf("createAndEnroll failed: %v", err)
	}
	clock.Advance(1 * time.Second)

	// Record some additional data for good measure.
	if err := s.RecordDeviceAuthnData(ctx, dev.GetId(), collectedDataForDevice(dev)); err != nil {
		t.Fatalf("RecordDeviceAuthnData failed: %v", err)
	}
	clock.Advance(1 * time.Second)
	if err := s.RecordDeviceAuthnData(ctx, dev.GetId(), collectedDataForDevice(dev)); err != nil {
		t.Fatalf("RecordDeviceAuthnData failed: %v", err)
	}
	clock.Advance(1 * time.Second)

	assertCD := func(dev *devicepb.Device, want int) {
		dev, err = s.GetDeviceByID(ctx, dev.GetId())
		if err != nil {
			t.Fatalf("GetDeviceByID failed: %v", err)
		}
		if got := len(dev.GetCollectedData()); got != want {
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

	dev, err := s.CreateDevice(ctx, devicepb.Device_builder{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "llama",
	}.Build(), false /* createAsResource */)
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

	validCred := devicepb.DeviceCredential_builder{
		Id:           "cred1",
		PublicKeyDer: key1DER,
	}.Build()
	validCD := devicepb.DeviceCollectedData_builder{
		CollectTime:  timestamppb.Now(),
		OsType:       dev.GetOsType(),
		SerialNumber: dev.GetAssetTag(),
	}.Build()

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
			deviceID:   dev.GetId(),
			createCred: func() *devicepb.DeviceCredential { return nil },
			createCD:   func() *devicepb.DeviceCollectedData { return validCD },
			owner:      owner,
			assertErr:  trace.IsBadParameter,
			wantErr:    "credential required",
		},
		{
			name:     "credential ID empty",
			deviceID: dev.GetId(),
			createCred: func() *devicepb.DeviceCredential {
				cp := proto.Clone(validCred).(*devicepb.DeviceCredential)
				cp.SetId("")
				return cp
			},
			createCD:  func() *devicepb.DeviceCollectedData { return validCD },
			owner:     owner,
			assertErr: trace.IsBadParameter,
			wantErr:   "credential ID",
		},
		{
			name:     "credential ID length",
			deviceID: dev.GetId(),
			createCred: func() *devicepb.DeviceCredential {
				cp := proto.Clone(validCred).(*devicepb.DeviceCredential)
				cp.SetId(strings.Repeat("A", 70))
				return cp
			},
			createCD:  func() *devicepb.DeviceCollectedData { return validCD },
			owner:     owner,
			assertErr: trace.IsBadParameter,
			wantErr:   "credential ID exceeds",
		},
		{
			name:     "credential PublicKeyDer empty",
			deviceID: dev.GetId(),
			createCred: func() *devicepb.DeviceCredential {
				cp := proto.Clone(validCred).(*devicepb.DeviceCredential)
				cp.SetPublicKeyDer(nil)
				return cp
			},
			createCD:  func() *devicepb.DeviceCollectedData { return validCD },
			owner:     owner,
			assertErr: trace.IsBadParameter,
			wantErr:   "public key required",
		},
		{
			name:     "credential PublicKeyDer invalid",
			deviceID: dev.GetId(),
			createCred: func() *devicepb.DeviceCredential {
				cp := proto.Clone(validCred).(*devicepb.DeviceCredential)
				cp.SetPublicKeyDer([]byte("not a DER"))
				return cp
			},
			createCD:  func() *devicepb.DeviceCollectedData { return validCD },
			owner:     owner,
			assertErr: trace.IsBadParameter,
			wantErr:   "invalid credential public key",
		},
		{
			name:       "collectedData nil",
			deviceID:   dev.GetId(),
			createCred: func() *devicepb.DeviceCredential { return validCred },
			createCD:   func() *devicepb.DeviceCollectedData { return nil },
			owner:      owner,
			assertErr:  trace.IsBadParameter,
			wantErr:    "collected data required",
		},
		{
			name:       "collectedData CollectTime nil",
			deviceID:   dev.GetId(),
			createCred: func() *devicepb.DeviceCredential { return validCred },
			createCD: func() *devicepb.DeviceCollectedData {
				cp := proto.Clone(validCD).(*devicepb.DeviceCollectedData)
				cp.ClearCollectTime()
				return cp

			},
			owner:     owner,
			assertErr: trace.IsBadParameter,
			wantErr:   "collect time missing",
		},
		{
			name:       "collectedData OSType mismatch",
			deviceID:   dev.GetId(),
			createCred: func() *devicepb.DeviceCredential { return validCred },
			createCD: func() *devicepb.DeviceCollectedData {
				cp := proto.Clone(validCD).(*devicepb.DeviceCollectedData)
				cp.SetOsType(devicepb.OSType_OS_TYPE_LINUX)
				return cp

			},
			owner:     owner,
			assertErr: isDriftError,
			wantErr:   "OS type mismatch",
		},
		{
			name:       "collectedData SerialNumber empty",
			deviceID:   dev.GetId(),
			createCred: func() *devicepb.DeviceCredential { return validCred },
			createCD: func() *devicepb.DeviceCollectedData {
				cp := proto.Clone(validCD).(*devicepb.DeviceCollectedData)
				cp.SetSerialNumber("")
				return cp
			},
			owner:     owner,
			assertErr: trace.IsBadParameter,
			wantErr:   "serial number required",
		},
		{
			name:       "collectedData SerialNumber length",
			deviceID:   dev.GetId(),
			createCred: func() *devicepb.DeviceCredential { return validCred },
			createCD: func() *devicepb.DeviceCollectedData {
				cp := proto.Clone(validCD).(*devicepb.DeviceCollectedData)
				cp.SetSerialNumber(strings.Repeat("A", 121))
				return cp
			},
			owner:     owner,
			assertErr: trace.IsBadParameter,
			wantErr:   "serial number exceeds",
		},
		{
			name:       "collectedData SerialNumber mismatch",
			deviceID:   dev.GetId(),
			createCred: func() *devicepb.DeviceCredential { return validCred },
			createCD: func() *devicepb.DeviceCollectedData {
				cp := proto.Clone(validCD).(*devicepb.DeviceCollectedData)
				cp.SetSerialNumber("not the same as the other")
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
	dev1, _, err := createAndEnroll(ctx, s, devicepb.Device_builder{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "llama",
	}.Build(), owner)
	if err != nil {
		t.Fatalf("createAndEnroll failed: %v", err)
	}
	clockAdvance()
	dev2, _, err := createAndEnroll(ctx, s, devicepb.Device_builder{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "alpaca",
	}.Build(), owner)
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
		for range item.num {
			clockAdvance()
			if err := s.RecordDeviceAuthnData(ctx, item.dev.GetId(), collectedDataForDevice(item.dev)); err != nil {
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
		t.Run(fmt.Sprintf("GetByID(%v)", dev.GetAssetTag()), func(t *testing.T) {
			got, err := s.GetDeviceByID(ctx, dev.GetId())
			if err != nil {
				t.Fatalf("GetDeviceByID failed: %v", err)
			}
			if gotData := len(got.GetCollectedData()); gotData != wantData {
				t.Fatalf("Got %v collected data instances, want %v", gotData, wantData)
			}

			// Verify collected data instances.
			for _, cd := range got.GetCollectedData() {
				if !cd.HasCollectTime() {
					t.Error("Got cd.CollectTime = nil, want non=nil")
				}
				if !cd.HasRecordTime() {
					t.Error("Got cd.RecordTime = nil, want non=nil")
				}
				wantCD := devicepb.DeviceCollectedData_builder{
					CollectTime:  cd.GetCollectTime(),
					RecordTime:   cd.GetRecordTime(),
					OsType:       dev.GetOsType(),
					SerialNumber: dev.GetAssetTag(),
				}.Build()
				if diff := cmp.Diff(wantCD, cd, protocmp.Transform()); diff != "" {
					t.Errorf("CollectedData mismatch (-want +got):\n%s", diff)
				}
			}

			// Verify that timestamps are in order.
			prev := got.GetCollectedData()[0].GetRecordTime().AsTime()
			for _, cd := range got.GetCollectedData()[1:] {
				curr := cd.GetRecordTime().AsTime()
				if prev.After(curr) {
					t.Errorf("Got out-of-order collected data instances, want from oldest to newest: %v", got.GetCollectedData())
					break
				}
				prev = curr
			}
		})

		// GetByAssetTag returns collected data.
		t.Run(fmt.Sprintf("GetByAssetTag(%v)", dev.GetAssetTag()), func(t *testing.T) {
			devs, err := s.GetDevicesByAssetTag(ctx, dev.GetAssetTag())
			if err != nil {
				t.Fatalf("GetDevicesByAssetTag failed: %v", err)
			}
			got := devs[0]
			if gotData := len(got.GetCollectedData()); gotData != wantData {
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
			if gotData := len(got.GetCollectedData()); gotData != 0 {
				t.Errorf("Device %v: got %v collected data instances, want zero", got.GetAssetTag(), gotData)
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
				if dev.GetId() == got.GetId() {
					want = wantData
					break
				}
			}
			if gotData := len(got.GetCollectedData()); gotData != want {
				t.Errorf("Device %v: got %v collected data instances, want %v", got.GetAssetTag(), gotData, want)
			}
		}
	})

	// Writing too many collected data instances deletes the older entries.
	t.Run("RecordDeviceAuthnData replaces older entries", func(t *testing.T) {
		// Find out the latest timestamp.
		got, err := s.GetDeviceByID(ctx, dev1.GetId())
		if err != nil {
			t.Fatalf("GetDeviceByID failed: %v", err)
		}
		first := got.GetCollectedData()[0].GetRecordTime().AsTime()
		last := got.GetCollectedData()[len(got.GetCollectedData())-1].GetRecordTime().AsTime()

		// Write up to MaxCollectedDataPerDevice and then a bit more, so we know the
		// cap keeps working.
		for range storage.MaxCollectedDataPerDevice + 2 {
			clockAdvance()
			if err := s.RecordDeviceAuthnData(ctx, dev1.GetId(), collectedDataForDevice(dev1)); err != nil {
				t.Fatalf("RecordDeviceAuthnData failed: %v", err)
			}
		}

		// Collected data number got capped?
		got, err = s.GetDeviceByID(ctx, dev1.GetId())
		if err != nil {
			t.Fatalf("GetDeviceByID failed: %v", err)
		}
		if gotData, want := len(got.GetCollectedData()), storage.MaxCollectedDataPerDevice+1; gotData != want {
			t.Fatalf("Got %v collected data instances, want %v", gotData, want)
		}

		// Enrollment entry preserved (it is now the oldest).
		if ts := got.GetCollectedData()[0].GetRecordTime().AsTime(); ts != first {
			t.Error("Enrollment data not preserved during collected data trim")
		}
		// All older entries deleted.
		if ts := got.GetCollectedData()[1].GetRecordTime().AsTime(); !ts.After(last) {
			t.Errorf("Unexpected RecordTime after collected data trim, got %v, want > %v", ts, last)
		}
	})

	// Finally, delete a device that has collected data to verify that it works.
	t.Run("DeleteDevice", func(t *testing.T) {
		if err := s.DeleteDevice(ctx, dev1.GetId()); err != nil {
			t.Fatalf("DeleteDevice failed: %v", err)
		}

		// Collected data cannot be found in storage.
		storedCD, err := s.GetDeviceCollecteDataForTests(ctx, dev1.GetId())
		switch {
		case err != nil:
			t.Errorf("GetDeviceCollecteDataForTests failed: %v", err)
		case len(storedCD) > 0:
			t.Errorf("GetDeviceCollecteDataForTests return %v collected data instances, want zero", len(storedCD))
		}

		// Unrelated device is intact.
		got, err := s.GetDeviceByID(ctx, dev2.GetId())
		if err != nil {
			t.Fatalf("GetDeviceByID failed: %v", err)
		}
		if gotData, want := len(got.GetCollectedData()), dev2WantData; gotData != want {
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
		devicepb.Device_builder{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: "mac",
		}.Build(),
		devicepb.Device_builder{
			OsType:   devicepb.OSType_OS_TYPE_WINDOWS,
			AssetTag: "win",
		}.Build(),
		devicepb.Device_builder{
			OsType:   devicepb.OSType_OS_TYPE_LINUX,
			AssetTag: "linux",
		}.Build(),
	} {
		created, _, err := createAndEnroll(ctx, s, dev, owner)
		if err != nil {
			t.Fatalf("createAndEnroll(%q) failed: %v", dev.GetAssetTag(), err)
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
			deviceID: macDev.GetId(),
			cd: devicepb.DeviceCollectedData_builder{
				CollectTime:  timestamppb.New(nowAndAdvance()),
				OsType:       macDev.GetOsType(),
				SerialNumber: macDev.GetAssetTag(),
			}.Build(),
		},
		{
			name:     "MDM fields",
			deviceID: macDev.GetId(),
			cd: devicepb.DeviceCollectedData_builder{
				CollectTime:       timestamppb.New(nowAndAdvance()),
				OsType:            macDev.GetOsType(),
				SerialNumber:      macDev.GetAssetTag(),
				ModelIdentifier:   "MacBookPro14,3",
				OsVersion:         "11.1",
				OsBuild:           "11D11",
				OsUsername:        "alpaca",
				OsLoginUser:       "alpaca",
				JamfBinaryVersion: "1",
				MacosEnrollmentProfiles: `Enrolled via DEP: No
MDM enrollment: Yes (User Approved)
MDM server: https://example.com/mdm/ServerURL`,
			}.Build(),
		},
		{
			name:     "TPM fields",
			deviceID: winDev.GetId(),
			cd: devicepb.DeviceCollectedData_builder{
				CollectTime:  timestamppb.New(nowAndAdvance()),
				OsType:       winDev.GetOsType(),
				SerialNumber: winDev.GetAssetTag(),
				TpmPlatformAttestation: devicepb.TPMPlatformAttestation_builder{
					Nonce: []byte("fake-nonce"),
					PlatformParameters: devicepb.TPMPlatformParameters_builder{
						EventLog: []byte("fake-event-log"),
						Quotes: []*devicepb.TPMQuote{
							devicepb.TPMQuote_builder{
								Quote:     []byte("fake-quote-0"),
								Signature: []byte("fake-signature-0"),
							}.Build(),
							devicepb.TPMQuote_builder{
								Quote:     []byte("fake-quote-1"),
								Signature: []byte("fake-signature-1"),
							}.Build(),
						},
						Pcrs: []*devicepb.TPMPCR{
							devicepb.TPMPCR_builder{
								Index:     0,
								Digest:    []byte("fake-sha1-digest"),
								DigestAlg: uint64(crypto.SHA1),
							}.Build(),
							devicepb.TPMPCR_builder{
								Index:     1,
								Digest:    []byte("fake-sha256-digest"),
								DigestAlg: uint64(crypto.SHA256),
							}.Build(),
						},
					}.Build(),
				}.Build(),
			}.Build(),
		},
		{
			name:     "Linux",
			deviceID: linuxDev.GetId(),
			cd: devicepb.DeviceCollectedData_builder{
				CollectTime:           timestamppb.New(nowAndAdvance()),
				OsType:                devicepb.OSType_OS_TYPE_LINUX,
				SerialNumber:          linuxDev.GetAssetTag(),
				ModelIdentifier:       "21J50013US",
				OsVersion:             "22.04",
				OsBuild:               "22.04 LTS (Jammy Jellyfish)",
				OsUsername:            "Llama Teleport",
				OsLoginUser:           "llama",
				ReportedAssetTag:      "No Asset Information",
				SystemSerialNumber:    linuxDev.GetAssetTag(),
				BaseBoardSerialNumber: "L1AA00A00A0",
				OsId:                  "ubuntu",
			}.Build(),
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
			if got, want := len(stored.GetCollectedData()), 1; got < want {
				t.Fatalf("GetDeviceByID: got %v collected data instances, want>=%v", got, want)
			}

			// Last recorded entry must be the one above
			got := stored.GetCollectedData()[len(stored.GetCollectedData())-1]
			want := cd
			want.SetRecordTime(got.GetRecordTime()) // System-managed
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
	ctx := t.Context()

	const owner = "llama"
	devWithProfile, _, err := createAndEnroll(ctx, s, devicepb.Device_builder{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "alpaca",
		Profile: devicepb.DeviceProfile_builder{
			ModelIdentifier:   "MacBookPro9,2",
			OsVersion:         "13.3.1",
			OsBuild:           "22E261",
			OsUsernames:       []string{"admin", "llama"},
			JamfBinaryVersion: "10.45.0-t1678116779",
		}.Build(),
	}.Build(), owner)
	if err != nil {
		t.Fatalf("createAndEnroll failed: %v", err)
	}

	devWithData, err := s.CreateDevice(ctx, devicepb.Device_builder{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "llama",
	}.Build(), false /* createAsResource */)
	if err != nil {
		t.Fatalf("CreateDevice failed: %v", err)
	}

	// Record a couple of CD instances that evolve over time as the basis for
	// testing.
	cd1 := devicepb.DeviceCollectedData_builder{
		OsType:            devWithData.GetOsType(),
		SerialNumber:      devWithData.GetAssetTag(),
		ModelIdentifier:   "MacBookPro9,2",
		OsVersion:         "13.2.1",
		OsBuild:           "22D68",
		OsUsername:        "llama",
		JamfBinaryVersion: "9.27",
	}.Build()
	cd2 := devicepb.DeviceCollectedData_builder{
		OsType:            devWithData.GetOsType(),
		SerialNumber:      devWithData.GetAssetTag(),
		ModelIdentifier:   "MacBookPro9,2",
		OsVersion:         "13.3",
		OsBuild:           "22E260",
		OsUsername:        "llama",
		OsLoginUser:       "llama", // added
		JamfBinaryVersion: "10.44.1-t1677509507",
	}.Build()
	for _, cd := range []*devicepb.DeviceCollectedData{cd1, cd2} {
		clock.Advance(1 * time.Second)
		cd.SetCollectTime(timestamppb.New(clock.Now()))
		if err := s.RecordDeviceAuthnData(ctx, devWithData.GetId(), cd); err != nil {
			t.Fatalf("RecordDeviceAuthnData failed: %v", err)
		}
	}

	// Prepare devices for a few Linux-specific data drift scenarios.
	linuxWithData := devicepb.Device_builder{
		OsType:   devicepb.OSType_OS_TYPE_LINUX,
		AssetTag: "linux1",
	}.Build()
	linuxWithProfile := devicepb.Device_builder{
		OsType:   devicepb.OSType_OS_TYPE_LINUX,
		AssetTag: "linux2",
		Profile: devicepb.DeviceProfile_builder{
			OsId: "ubuntu",
		}.Build(),
	}.Build()
	for _, d := range []**devicepb.Device{&linuxWithData, &linuxWithProfile} {
		var err error
		*d, err = s.CreateDevice(ctx, *d, false /* createAsResource */)
		if err != nil {
			t.Fatalf("CreateDevice(%q) failed: %v", (*d).GetAssetTag(), err)
		}
	}
	cdLinux := collectedDataForDevice(linuxWithData)
	cdLinux.SetOsId("ubuntu")
	if err := s.RecordDeviceAuthnData(ctx, linuxWithData.GetId(), cdLinux); err != nil {
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
			deviceID: devWithProfile.GetId(),
			createCD: func() *devicepb.DeviceCollectedData {
				cd := collectedDataForDevice(devWithProfile)
				cd.ClearCollectTime()
				return cd
			},
			wantErr:   "collect time",
			assertErr: trace.IsBadParameter,
		},
		{
			name:     "SerialNumber mismatch",
			deviceID: devWithProfile.GetId(),
			createCD: func() *devicepb.DeviceCollectedData {
				cd := collectedDataForDevice(devWithProfile)
				cd.SetSerialNumber("unknown")
				return cd
			},
			wantErr:   "serial number mismatch",
			assertErr: isDriftError,
		},
		{
			name:     "OSVersion for macOS not a semver",
			deviceID: devWithProfile.GetId(),
			createCD: func() *devicepb.DeviceCollectedData {
				cd := collectedDataForDevice(devWithProfile)
				cd.SetOsType(devicepb.OSType_OS_TYPE_MACOS) // to be 100% sure
				cd.SetOsVersion("not a semver")
				return cd
			},
			wantErr:   "OS version",
			assertErr: trace.IsBadParameter,
		},
		{
			name:     "JamfBinaryVersion not a semver",
			deviceID: devWithProfile.GetId(),
			createCD: func() *devicepb.DeviceCollectedData {
				cd := collectedDataForDevice(devWithProfile)
				cd.SetJamfBinaryVersion("not a semver")
				return cd
			},
			wantErr:   "jamf binary",
			assertErr: trace.IsBadParameter,
		},

		// Profile validation.
		{
			name:     "profile ModelIdentifier drift",
			deviceID: devWithProfile.GetId(),
			createCD: func() *devicepb.DeviceCollectedData {
				cd := collectedDataForDevice(devWithProfile)
				cd.SetModelIdentifier("MacBookPro9,3") // can't change
				return cd
			},
			wantErr:   "device model",
			assertErr: isDriftError,
		},
		{
			name:     "profile OsVersion missing",
			deviceID: devWithProfile.GetId(),
			createCD: func() *devicepb.DeviceCollectedData {
				cd := collectedDataForDevice(devWithProfile)
				cd.SetOsVersion("")
				return cd
			},
			wantErr:   "OS version",
			assertErr: isDriftError,
		},
		{
			name:     "profile OsVersion backwards drift",
			deviceID: devWithProfile.GetId(),
			createCD: func() *devicepb.DeviceCollectedData {
				cd := collectedDataForDevice(devWithProfile)
				cd.SetOsVersion("13.1")
				return cd
			},
			wantErr:   "OS version",
			assertErr: isDriftError,
		},
		{
			name:     "profile OsUsernames drift",
			deviceID: devWithProfile.GetId(),
			createCD: func() *devicepb.DeviceCollectedData {
				cd := collectedDataForDevice(devWithProfile)
				cd.SetOsUsername("alpaca2") // not in profile
				cd.SetOsLoginUser("alpaca2")
				return cd
			},
			wantErr:   "OS username",
			assertErr: isDriftError,
		},

		// Previous data validation.
		{
			name:     "old collected data replay drift",
			deviceID: devWithData.GetId(),
			createCD: func() *devicepb.DeviceCollectedData {
				return cd1 // rolls back OsLoginUser, among others
			},
			wantErr:   "drift detected",
			assertErr: isDriftError,
		},
		{
			name:     "collected data ModelIdentifier drift",
			deviceID: devWithData.GetId(),
			createCD: func() *devicepb.DeviceCollectedData {
				cd := proto.Clone(cd2).(*devicepb.DeviceCollectedData)
				cd.SetModelIdentifier("MacBookPro9,1") // can't change
				return cd
			},
			wantErr:   "device model",
			assertErr: isDriftError,
		},
		{
			name:     "collected data OsVersion drift",
			deviceID: devWithData.GetId(),
			createCD: func() *devicepb.DeviceCollectedData {
				cd := proto.Clone(cd2).(*devicepb.DeviceCollectedData)
				cd.SetOsVersion("13.2.2") // in-between cd1 and cd2
				return cd
			},
			wantErr:   "OS version",
			assertErr: isDriftError,
		},
		{
			name:     "collected data OsLoginUser drift",
			deviceID: devWithData.GetId(),
			createCD: func() *devicepb.DeviceCollectedData {
				cd := proto.Clone(cd2).(*devicepb.DeviceCollectedData)
				cd.SetOsUsername("eve")  // unexpected change
				cd.SetOsLoginUser("eve") // unexpected change
				return cd
			},
			wantErr:   "OS login user",
			assertErr: isDriftError,
		},
		{
			name:     "collected data OsId drift (Linux)",
			deviceID: linuxWithData.GetId(),
			createCD: func() *devicepb.DeviceCollectedData {
				cd := proto.Clone(cdLinux).(*devicepb.DeviceCollectedData)
				cd.SetOsId("") // want "ubuntu"
				return cd
			},
			wantErr:   "OS ID",
			assertErr: isDriftError,
		},
		{
			name:     "profile OsId drift (Linux)",
			deviceID: linuxWithProfile.GetId(),
			createCD: func() *devicepb.DeviceCollectedData {
				cd := collectedDataForDevice(linuxWithProfile)
				cd.SetOsId("fedora") // want "ubuntu"
				return cd
			},
			wantErr:   "OS ID",
			assertErr: isDriftError,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := t.Context()
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

// Tests corner cases of OsUsername and OsLoginUser validation.
func TestS_RecordDeviceAuthnData_osUsername(t *testing.T) {
	t.Parallel()

	env := mustNewEnv()
	defer env.Close()

	s := env.S
	clock := env.Clock

	mustRecordData := func(
		t *testing.T, devID string, cd *devicepb.DeviceCollectedData) {
		t.Helper()
		clock.Advance(1 * time.Second)
		assert.NoError(t, s.RecordDeviceAuthnData(t.Context(), devID, cd))
	}

	mustFailRecordData := func(
		t *testing.T, devID string, cd *devicepb.DeviceCollectedData, wantErr string) {
		t.Helper()
		clock.Advance(1 * time.Second)
		assert.ErrorContains(t,
			s.RecordDeviceAuthnData(t.Context(), devID, cd),
			wantErr,
		)
	}

	const user = "llama"

	t.Run("macOS device", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		dev, err := s.CreateDevice(ctx, devicepb.Device_builder{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: "macos",
		}.Build(), false /* createAsResource */)
		if err != nil {
			t.Fatalf("CreateDevice failed: %v", err)
		}

		// Initial collected data.
		cd := collectedDataForDevice(dev)
		cd.SetOsUsername(user)
		mustRecordData(t, dev.GetId(), cd)

		// OsUsername is validated.
		cd.SetOsUsername("alpaca")
		mustFailRecordData(t, dev.GetId(), cd, "OS username drift")

		// OsUsername and OsLoginUser must match.
		cd.SetOsUsername(user)
		cd.SetOsLoginUser("alpaca") // bad
		mustFailRecordData(t, dev.GetId(), cd, "login user mismatch")

		// OsLoginUser recorded.
		cd.SetOsUsername(user)
		cd.SetOsLoginUser(user)
		mustRecordData(t, dev.GetId(), cd)

		// OsLoginUser is validated.
		cd.SetOsUsername("alpaca")  // bad
		cd.SetOsLoginUser("alpaca") // bad
		mustFailRecordData(t, dev.GetId(), cd, "OS login user drift")

		// One last correct recording after all failures.
		cd.SetOsUsername(user)
		cd.SetOsLoginUser(user)
		mustRecordData(t, dev.GetId(), cd)
	})

	t.Run("macOS profile validation", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		dev, err := s.CreateDevice(ctx, devicepb.Device_builder{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: "macos2",
			Profile: devicepb.DeviceProfile_builder{
				OsUsernames: []string{user},
			}.Build(),
		}.Build(), false /* createAsResource */)
		if err != nil {
			t.Fatalf("CreateDevice failed: %v", err)
		}

		// Profile requires username.
		cd := collectedDataForDevice(dev)
		cd.SetOsUsername("")
		cd.SetOsLoginUser("")
		mustFailRecordData(t, dev.GetId(), cd, "username required")

		// OsUsername does not match profile.
		cd.SetOsUsername("alpaca")
		mustFailRecordData(t, dev.GetId(), cd, "username not present")

		// OsUsername matches profile.
		cd.SetOsUsername(user)
		mustRecordData(t, dev.GetId(), cd)

		// OsLoginUser matches profile.
		cd.SetOsUsername(user)
		cd.SetOsLoginUser(user)
		mustRecordData(t, dev.GetId(), cd)
	})

	t.Run("Linux device", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		dev, err := s.CreateDevice(ctx, devicepb.Device_builder{
			OsType:   devicepb.OSType_OS_TYPE_LINUX,
			AssetTag: "linux",
		}.Build(), false /* createAsResource */)
		if err != nil {
			t.Fatalf("CreateDevice failed: %v", err)
		}

		const displayName = "Llama Teleport"

		// Initial collected data.
		cd := collectedDataForDevice(dev)
		cd.SetOsUsername(displayName)
		mustRecordData(t, dev.GetId(), cd)

		// OsUsername drift allowed.
		cd.SetOsUsername("Just Call Me Llama")
		mustRecordData(t, dev.GetId(), cd)

		// OsLoginUser recorded.
		cd.SetOsLoginUser(user)
		mustRecordData(t, dev.GetId(), cd)

		// OsUsername drift still allowed.
		cd.SetOsUsername(displayName) // changed back
		cd.SetOsLoginUser(user)
		mustRecordData(t, dev.GetId(), cd)
		// OsLoginUser is validated.
		cd.SetOsUsername(displayName)
		cd.SetOsLoginUser("alpaca") // bad
		mustFailRecordData(t, dev.GetId(), cd, "OS login user drift")

		// One last correct recording after all failures.
		cd.SetOsUsername(displayName)
		cd.SetOsLoginUser(user)
		mustRecordData(t, dev.GetId(), cd)
	})

	t.Run("Linux profile validation", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		dev, err := s.CreateDevice(ctx, devicepb.Device_builder{
			OsType:   devicepb.OSType_OS_TYPE_LINUX,
			AssetTag: "linux2",
			Profile: devicepb.DeviceProfile_builder{
				OsUsernames: []string{user},
			}.Build(),
		}.Build(), false /* createAsResource */)
		if err != nil {
			t.Fatalf("CreateDevice failed: %v", err)
		}

		// Profile requires username.
		cd := collectedDataForDevice(dev)
		cd.SetOsUsername("")
		cd.SetOsLoginUser("")
		mustFailRecordData(t, dev.GetId(), cd, "username required")

		// OsUsername ignored for profile matching.
		cd.SetOsUsername(user)
		mustFailRecordData(t, dev.GetId(), cd, "username required")

		// OsLoginUser does not match profile.
		cd.SetOsUsername("Llama")
		cd.SetOsLoginUser("alpaca") // bad
		mustFailRecordData(t, dev.GetId(), cd, "username not present")

		// OsLoginUser matches profile.
		cd.SetOsUsername("Llama")
		cd.SetOsLoginUser(user)
		mustRecordData(t, dev.GetId(), cd)
	})
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
	switch dev.GetOsType() {
	case devicepb.OSType_OS_TYPE_MACOS:
		ecdsaKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return nil, nil, fmt.Errorf("calling GenerateKey: %w", err)
		}
		pubKeyDER, err := x509.MarshalPKIXPublicKey(ecdsaKey.Public())
		if err != nil {
			return nil, nil, fmt.Errorf("calling MarshalPKIXPublicKey: %w", err)
		}
		cred = devicepb.DeviceCredential_builder{
			Id:           uuid.NewString(),
			PublicKeyDer: pubKeyDER,
		}.Build()
		key = ecdsaKey
	case devicepb.OSType_OS_TYPE_LINUX, devicepb.OSType_OS_TYPE_WINDOWS:
		validAKPublic, err := base64.StdEncoding.DecodeString(validAKPublic)
		if err != nil {
			return nil, nil, fmt.Errorf("parsing valid ak public: %w", err)
		}
		cred = devicepb.DeviceCredential_builder{
			Id:                    "fake-credential-id",
			DeviceAttestationType: devicepb.DeviceAttestationType_DEVICE_ATTESTATION_TYPE_TPM_EKPUB,
			TpmAkPublic:           validAKPublic,
		}.Build()
	default:
		return nil, nil, fmt.Errorf("unhandled OS Type: %s", dev.GetOsType())
	}

	dev, err := s.EnrollDevice(ctx, dev.GetId(), cred, collectedDataForDevice(dev), owner)
	if err != nil {
		return nil, nil, err // unwrapped for simpler comparisons
	}
	return dev, key, nil
}

func collectedDataForDevice(dev *devicepb.Device) *devicepb.DeviceCollectedData {
	cd := devicepb.DeviceCollectedData_builder{
		CollectTime:  timestamppb.Now(),
		OsType:       dev.GetOsType(),
		SerialNumber: dev.GetAssetTag(),
	}.Build()
	if p := dev.GetProfile(); p != nil {
		cd.SetModelIdentifier(p.GetModelIdentifier())
		cd.SetOsVersion(p.GetOsVersion())
		cd.SetOsBuild(p.GetOsBuild())
		if len(p.GetOsUsernames()) > 0 {
			cd.SetOsUsername(p.GetOsUsernames()[0])
			cd.SetOsLoginUser(p.GetOsUsernames()[0])
		}
		cd.SetJamfBinaryVersion(p.GetJamfBinaryVersion())
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
	devFullProfile := devicepb.Device_builder{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "llama1",
		Profile: devicepb.DeviceProfile_builder{
			ModelIdentifier:   "MacBookPro9,2",
			OsVersion:         "13.3.1",
			OsBuild:           "22E261",
			OsUsernames:       []string{"llama", "admin"},
			JamfBinaryVersion: "10.45.0-t1678116779",
		}.Build(),
	}.Build()
	devSmallProfile := devicepb.Device_builder{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "llama2",
		Profile: devicepb.DeviceProfile_builder{
			ModelIdentifier: "MacBookPro9,2",
			OsVersion:       "13.3.1",
		}.Build(),
	}.Build()
	devNoProfile := devicepb.Device_builder{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "alpaca",
	}.Build()
	for _, dev := range []**devicepb.Device{
		&devFullProfile,
		&devSmallProfile,
		&devNoProfile,
	} {
		created, err := s.CreateDevice(ctx, *dev, false /* createAsResource */)
		if err != nil {
			t.Fatalf("CreateDevice(%q) failed: %v", (*dev).GetAssetTag(), err)
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
			cd: devicepb.DeviceCollectedData_builder{
				CollectTime: timestamppb.New(clock.Now()),
				// Required fields.
				OsType:       devFullProfile.GetOsType(),
				SerialNumber: devFullProfile.GetAssetTag(),
				// Profile-informed fields.
				ModelIdentifier:   devFullProfile.GetProfile().GetModelIdentifier(),
				OsVersion:         devFullProfile.GetProfile().GetOsVersion(),
				OsBuild:           devFullProfile.GetProfile().GetOsBuild(),
				OsUsername:        devFullProfile.GetProfile().GetOsUsernames()[0],
				OsLoginUser:       devFullProfile.GetProfile().GetOsUsernames()[0],
				JamfBinaryVersion: devFullProfile.GetProfile().GetJamfBinaryVersion(),
			}.Build(),
			wantDev: devFullProfile,
		},
		{
			name: "auto-enroll (small profile)",
			cd: devicepb.DeviceCollectedData_builder{
				CollectTime:     timestamppb.New(clock.Now()),
				OsType:          devSmallProfile.GetOsType(),
				SerialNumber:    devSmallProfile.GetAssetTag(),
				ModelIdentifier: devSmallProfile.GetProfile().GetModelIdentifier(),
				OsVersion:       devSmallProfile.GetProfile().GetOsVersion(),
				// Nothing else required by the profile.
			}.Build(),
			wantDev: devSmallProfile,
		},
		{
			name: "auto-enroll (no profile)",
			cd: devicepb.DeviceCollectedData_builder{
				CollectTime:  timestamppb.New(clock.Now()),
				OsType:       devNoProfile.GetOsType(),
				SerialNumber: devNoProfile.GetAssetTag(),
				// Nothing else required by the profile.
			}.Build(),
			wantDev: devNoProfile,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			const user = "llama"
			got, err := s.CreateDeviceEnrollTokenUsingData(ctx, test.cd, user)
			if err != nil {
				t.Fatalf("CreateDeviceEnrollTokenUsingData failed: %v", err)
			}
			clock.Advance(1 * time.Second)

			// Verify that we got the correct device.
			if want := test.wantDev; got.GetId() != want.GetId() {
				t.Errorf(
					"CreateDeviceEnrollTokenUsingData: got device %v/%v, want %v/%v",
					got.GetId(), got.GetAssetTag(),
					want.GetId(), want.GetAssetTag(),
				)
			}

			// Verify that we got a non-empty token.
			if got.GetEnrollToken().GetToken() == "" {
				t.Fatalf("CreateDeviceEnrollTokenUsingData: got token=%v, want non-empty token", got.GetEnrollToken())
			}

			// Verify that no collected data was stored (device is not enrolled yet!)
			stored, err := s.GetDeviceByID(ctx, got.GetId())
			if err != nil {
				t.Fatalf("GetDeviceByID failed: %v", err)
			}
			if len(stored.GetCollectedData()) > 0 {
				t.Errorf("GetDeviceByID: got %v instances of collected data, wanted zero: %v", len(stored.GetCollectedData()), stored.GetCollectedData())
			}

			// Spend the token to verify that it works.
			if tokenData, err := s.SpendDeviceEnrollToken(ctx, got.GetId(), got.GetEnrollToken().GetToken()); err != nil {
				t.Errorf("SpendDeviceEnrollToken failed: %v", err)
			} else {
				if !tokenData.CreatedByAutoEnroll {
					t.Errorf("SpendDeviceEnrollToken returned tokenData=%#v, want tokenData.CreatedByAutoEnroll=true", tokenData)
				}
				if tokenData.User != user {
					t.Errorf("SpendDeviceEnrollToken returned tokenData.User=%q, want %q", tokenData.User, user)
				}
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

	dev, err := s.CreateDevice(ctx, devicepb.Device_builder{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "llama1",
		Profile: devicepb.DeviceProfile_builder{
			ModelIdentifier:   "MacBookPro9,2",
			OsVersion:         "13.3.1",
			OsBuild:           "22E261",
			OsUsernames:       []string{"llama", "admin"},
			JamfBinaryVersion: "10.45.0-t1678116779",
		}.Build(),
	}.Build(), false /* createAsResource */)
	require.NoError(t, err, "CreateDevice failed")

	if err != nil {
		t.Fatalf("CreateDevice) failed: %v", err)
	}

	// Register an unrelated Windows device.
	// The purpose of this device is to not match collected data from its namesake
	// MacOS device.
	if _, err = s.CreateDevice(ctx, devicepb.Device_builder{
		OsType:   devicepb.OSType_OS_TYPE_WINDOWS,
		AssetTag: "llama1",
		Profile: devicepb.DeviceProfile_builder{
			ModelIdentifier:   "ThinkPad 9000",
			OsVersion:         "22H2",
			OsBuild:           "19045",
			OsUsernames:       []string{"not-a-llama", "admin"},
			JamfBinaryVersion: "10.44",
		}.Build(),
	}.Build(), false /* createAsResource */); err != nil {
		t.Fatalf("CreateDevice failed: %v", err)
	}
	require.NoError(t, err, "CreateDevice failed")

	const owner = "llama"
	devEnrolled, _, err := createAndEnroll(ctx, s, devicepb.Device_builder{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "llama2",
		// A nil Profile makes this device very easy to auto-enroll, if not for the
		// fact that it already is enrolled.
		Profile: nil,
	}.Build(), owner)
	require.NoError(t, err, "createAndEnroll failed")

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
				cd.SetOsType(devicepb.OSType_OS_TYPE_LINUX) // unknown
				return cd
			},
			assertErr: trace.IsNotFound,
			wantErr:   "not found",
		},
		{
			name: "unknown device (SerialNumber)",
			createCD: func() *devicepb.DeviceCollectedData {
				cd := collectedDataForDevice(dev)
				cd.SetSerialNumber("unknown")
				return cd
			},
			assertErr: trace.IsNotFound,
			wantErr:   "not found",
		},
		{
			name: "ModelIdentifier empty",
			createCD: func() *devicepb.DeviceCollectedData {
				cd := collectedDataForDevice(dev)
				cd.SetModelIdentifier("")
				return cd
			},
			assertErr: isDriftError,
			wantErr:   "device model",
		},
		{
			name: "ModelIdentifier invalid",
			createCD: func() *devicepb.DeviceCollectedData {
				cd := collectedDataForDevice(dev)
				cd.SetModelIdentifier("MacBookPro10,1")
				return cd
			},
			assertErr: isDriftError,
			wantErr:   "device model",
		},
		{
			name: "OsVersion invalid",
			createCD: func() *devicepb.DeviceCollectedData {
				cd := collectedDataForDevice(dev)
				cd.SetOsVersion("13.4") // even a drift upwards is disallowed here
				return cd
			},
			assertErr: isDriftError,
			wantErr:   "OS version",
		},
		{
			name: "OsBuild invalid",
			createCD: func() *devicepb.DeviceCollectedData {
				cd := collectedDataForDevice(dev)
				cd.SetOsBuild("22E262") // changed from 22E261
				return cd
			},
			assertErr: isDriftError,
			wantErr:   "OS build",
		},
		{
			name: "OsUsername not in profile",
			createCD: func() *devicepb.DeviceCollectedData {
				cd := collectedDataForDevice(dev)
				cd.SetOsUsername("llamaO") // wanted "llama" or "admin"
				cd.SetOsLoginUser("")
				return cd
			},
			assertErr: isDriftError,
			wantErr:   "username not present",
		},
		{
			name: "OsLoginUser not in profile",
			createCD: func() *devicepb.DeviceCollectedData {
				cd := collectedDataForDevice(dev)
				cd.SetOsUsername("")
				cd.SetOsLoginUser("llamaO") // wanted "llama" or "admin"
				return cd
			},
			assertErr: isDriftError,
			wantErr:   "username not present",
		},
		{
			name: "JamfBinaryVersion invalid",
			createCD: func() *devicepb.DeviceCollectedData {
				cd := collectedDataForDevice(dev)
				cd.SetJamfBinaryVersion("10.46") // changed from 10.45.0-t1678116779
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
			_, err := s.CreateDeviceEnrollTokenUsingData(ctx, test.createCD(), "llama")
			require.Error(t, err, "CreateDeviceEnrollTokenUsingData returned err=nil, want non-nil")
			assert.True(t, test.assertErr(err), "CreateDeviceEnrollTokenUsingData: assertErr failed, err=%v (%T)", err, err)
			assert.ErrorContains(t, err, test.wantErr, "CreateDeviceEnrollTokenUsingData error mismatch")
		})
	}
}

// TestS_CreateDeviceEnrollTokenUsingData_DuplicateTagAndOSType tests a (largely
// theoretical) scenario where distinct devices have the same (OsType,AssetTag)
// pair.
func TestS_CreateDeviceEnrollTokenUsingData_DuplicateTagAndOSType(t *testing.T) {
	t.Parallel()

	env := mustNewEnv()
	defer env.Close()

	s := env.S
	ctx := context.Background()

	device1, err := s.CreateDevice(ctx, devicepb.Device_builder{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "device1",
	}.Build(), false /* createAsResource */)
	require.NoError(t, err, "CreateDevice failed")
	// Create a second device in the backend with the same AssetTag and OSType.
	// If two devices have the same AssetTag and same OsType, then the test should
	// fail. We insert directly into the backend to bypass validation in
	// CreateDevice, which would reject this.
	device2 := devicepb.Device_builder{
		Id:       uuid.NewString(),
		OsType:   device1.GetOsType(),
		AssetTag: device1.GetAssetTag(),
	}.Build()
	device2JSON, err := json.Marshal(device2)
	require.NoError(t, err)
	_, err = env.mem.Create(ctx, backend.Item{
		Key:   backend.NewKey("devices", "id", device2.GetId()),
		Value: device2JSON,
	})
	require.NoError(t, err, "Create device directly in backend failed")

	newIndex := map[string]any{
		"devices": []map[string]any{
			{
				"device_id": device1.GetId(),
				"os_type":   int32(device1.GetOsType()),
			},
			{
				"device_id": device2.GetId(),
				"os_type":   int32(device2.GetOsType()),
			},
		},
	}
	indexJSON, err := json.Marshal(newIndex)
	require.NoError(t, err)
	_, err = env.mem.Put(ctx, backend.Item{
		Key:   backend.NewKey("devices", "byTag", "device1"),
		Value: indexJSON,
	})
	require.NoError(t, err)

	_, err = s.CreateDeviceEnrollTokenUsingData(ctx, collectedDataForDevice(device1), "llama")
	require.Error(t, err, "CreateDeviceEnrollTokenUsingData returned err=nil, want non-nil")
	assert.True(t, trace.IsBadParameter(err), "CreateDeviceEnrollTokenUsingData: assertErr failed, err=%v (%T)", err, err)
	assert.ErrorContains(t, err, "collected data matches more than one device, aborting", "CreateDeviceEnrollTokenUsingData error mismatch")
}

func TestS_GetDeviceEnrollTokenDataUsingData(t *testing.T) {
	t.Parallel()

	env := mustNewEnv()
	defer env.Close()
	s := env.S
	ctx := t.Context()

	dev, err := s.CreateDevice(ctx, devicepb.Device_builder{
		OsType:   devicepb.OSType_OS_TYPE_IOS,
		AssetTag: "reader-llama",
	}.Build(), false /* createAsResource */)
	require.NoError(t, err)
	cd := collectedDataForDevice(dev)

	created, err := s.CreateDeviceEnrollTokenUsingData(ctx, cd, "llama")
	require.NoError(t, err)
	token := created.GetEnrollToken().GetToken()

	// Reads return the token data without consuming the token.
	for range 2 {
		gotDev, data, err := s.GetDeviceEnrollTokenDataUsingData(ctx, cd, token)
		require.NoError(t, err)
		assert.Equal(t, dev.GetId(), gotDev.GetId())
		assert.Equal(t, "llama", data.User)
		assert.True(t, data.CreatedByAutoEnroll)
	}

	// A token mismatch still names the device it was checked against.
	gotDev, _, err := s.GetDeviceEnrollTokenDataUsingData(ctx, cd, "not-the-token")
	assert.ErrorAs(t, err, new(*trace.BadParameterError))
	assert.Equal(t, dev.GetId(), gotDev.GetId())

	unknownCD := devicepb.DeviceCollectedData_builder{
		CollectTime:  timestamppb.Now(),
		OsType:       devicepb.OSType_OS_TYPE_IOS,
		SerialNumber: "no-such-device",
	}.Build()
	gotDev, _, err = s.GetDeviceEnrollTokenDataUsingData(ctx, unknownCD, token)
	assert.ErrorAs(t, err, new(*trace.NotFoundError))
	assert.Nil(t, gotDev)

	// The token survives the reads and is spent exactly once.
	data, err := s.SpendDeviceEnrollToken(ctx, dev.GetId(), token)
	require.NoError(t, err)
	assert.Equal(t, "llama", data.User)
	_, _, err = s.GetDeviceEnrollTokenDataUsingData(ctx, cd, token)
	assert.ErrorAs(t, err, new(*trace.NotFoundError))
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
		dev, err := s.CreateDevice(ctx, devicepb.Device_builder{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: asset,
		}.Build(), false /* createAsResource */)
		if err != nil {
			t.Fatalf("CreateDevice failed: %v", err)
		}
		devs = append(devs, dev)
	}
	dev1 := devs[0]
	dev2 := devs[1]
	deviceID := dev1.GetId()

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
				if first.GetToken() == second.GetToken() {
					return nil, errors.New("first and second tokens are equal")
				}

				// First token cannot be spent anymore.
				if _, err := s.SpendDeviceEnrollToken(ctx, deviceID, first.GetToken()); !trace.IsBadParameter(err) {
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
				return s.SpendDeviceEnrollToken(ctx, dev2.GetId() /* wrong device */, token)
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
			case tokenData.User != "":
				t.Errorf("SpendDeviceEnrollmentToken returned tokenData.User=%q, want empty", tokenData.User)
			}
		})
	}
}

func TestDefaultFakeEnrollTokenHash(t *testing.T) {
	t.Parallel()

	hash := []byte(storage.DefaultFakeEnrollTokenHash)

	// The precomputed hash must match the cost of the real hashes, otherwise fake
	// comparisons take a different time than real ones.
	cost, err := bcrypt.Cost(hash)
	if err != nil {
		t.Fatalf("Cost failed: %v", err)
	}
	if cost != bcrypt.DefaultCost {
		t.Errorf("Cost returned %v, want %v", cost, bcrypt.DefaultCost)
	}

	if err := bcrypt.CompareHashAndPassword(hash, []byte(storage.FakeEnrollTokenPassword)); err != nil {
		t.Errorf("CompareHashAndPassword failed, hash doesn't match its documented password: %v", err)
	}
}

func TestS_DeviceEnrollToken_rejectionTiming(t *testing.T) {
	t.Parallel()

	// Rejections must take at least as long as a bcrypt comparison, otherwise
	// their timing reveals whether the device and its token exist. Run at the
	// default cost so a comparison takes tens of milliseconds, far enough from
	// minElapsed that fast machines cannot dip below it.
	env := mustNewEnv(withBCryptCost(bcrypt.DefaultCost))
	t.Cleanup(func() { require.NoError(t, env.Close()) })
	s := env.S

	ctx := t.Context()

	// Create a device with an enrollment token and a device without one.
	var devs []*devicepb.Device
	for _, asset := range []string{"llama", "alpaca"} {
		dev, err := s.CreateDevice(ctx, devicepb.Device_builder{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: asset,
		}.Build(), false /* createAsResource */)
		if err != nil {
			t.Fatalf("CreateDevice failed: %v", err)
		}
		devs = append(devs, dev)
	}
	devWithToken := devs[0]
	devWithoutToken := devs[1]
	token, err := s.CreateDeviceEnrollToken(ctx, devWithToken.GetId(), time.Time{} /* expiresAt */)
	if err != nil {
		t.Fatalf("CreateDeviceEnrollToken failed: %v", err)
	}

	// minElapsed is well below a bcrypt comparison at the default cost, but
	// well above what the rejections do apart from the comparison.
	const minElapsed = 10 * time.Millisecond

	tests := []struct {
		name      string
		call      func(ctx context.Context) (*storage.DeviceEnrollTokenData, error)
		errTarget any
	}{
		{
			name: "SpendDeviceEnrollToken device not found",
			call: func(ctx context.Context) (*storage.DeviceEnrollTokenData, error) {
				return s.SpendDeviceEnrollToken(ctx, "unknown", token.GetToken())
			},
			errTarget: new(*trace.NotFoundError),
		},
		{
			name: "SpendDeviceEnrollToken device without token",
			call: func(ctx context.Context) (*storage.DeviceEnrollTokenData, error) {
				return s.SpendDeviceEnrollToken(ctx, devWithoutToken.GetId(), token.GetToken())
			},
			errTarget: new(*trace.NotFoundError),
		},
		{
			name: "SpendDeviceEnrollToken mismatched token",
			call: func(ctx context.Context) (*storage.DeviceEnrollTokenData, error) {
				return s.SpendDeviceEnrollToken(ctx, devWithToken.GetId(), token.GetToken()+"bad")
			},
			errTarget: new(*trace.BadParameterError),
		},
		{
			name: "GetDeviceEnrollTokenDataUsingData device not resolved",
			call: func(ctx context.Context) (*storage.DeviceEnrollTokenData, error) {
				unresolvedCD := devicepb.DeviceCollectedData_builder{
					CollectTime:  timestamppb.Now(),
					OsType:       devicepb.OSType_OS_TYPE_IOS,
					SerialNumber: "no-such-device",
				}.Build()
				_, data, err := s.GetDeviceEnrollTokenDataUsingData(ctx, unresolvedCD, token.GetToken())
				return data, err
			},
			errTarget: new(*trace.NotFoundError),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			start := time.Now()
			_, err := test.call(ctx)
			elapsed := time.Since(start)
			require.ErrorAs(t, err, test.errTarget)
			if elapsed < minElapsed {
				t.Errorf("rejected in %v, want at least %v (a bcrypt comparison)", elapsed, minElapsed)
			}
		})
	}
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
		devicepb.Device_builder{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: "emptyOwner",
		}.Build(),
		devicepb.Device_builder{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: "onlyDevice",
			Owner:    user1,
		}.Build(),
		devicepb.Device_builder{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: "onlyUser", // Assigned below.
		}.Build(),
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
	u2.SetTrustedDeviceIDs([]string{devOnlyUser.GetId()})
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
			deviceID := test.dev.GetId()
			got, err := s.AssignDeviceOwner(ctx, deviceID, test.user)
			if err != nil {
				t.Fatalf("AssignDeviceOwner returned err=%v", err)
			}

			// Assert returned device.
			want := test.dev
			want.SetOwner(test.user)
			want.SetUpdateTime(got.GetUpdateTime()) // System-managed.
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
					test.user, test.dev.GetId(), u.GetTrustedDeviceIDs())
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
		devicepb.Device_builder{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: "re-enroll",
		}.Build(),
		devicepb.Device_builder{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: "unenroll",
		}.Build(),
		devicepb.Device_builder{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: "delete",
		}.Build(),
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
			baseDev: devicepb.Device_builder{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "enroll1",
			}.Build(),
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
				return s.UpdateDevice(ctx, dev.GetId(), func(stored *devicepb.Device) *devicepb.Device {
					stored.SetEnrollStatus(devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED)
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
				err := s.DeleteDevice(ctx, dev.GetId())
				return nil, err
			},
			wantAssigned: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dev := test.baseDev
			user := test.baseUser.GetName()

			if dev.GetId() == "" {
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
				case !slices.Contains(storedUser.GetTrustedDeviceIDs(), dev.GetId()):
					t.Fatalf(
						"User %q does not have trusted device %q, u.TrustedDeviceIDs=%v",
						user, dev.GetId(), storedUser.GetTrustedDeviceIDs())
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
			if got, want := slices.Contains(trustedIDs, dev.GetId()), test.wantAssigned; got != want {
				var msg string
				if want {
					msg = "Device %q not assigned to user %q, User.TrustedDeviceIDs=%v"
				} else {
					msg = "Device %q not unassigned from user %q, User.TrustedDeviceIDs=%v"
				}
				t.Errorf(msg, dev.GetId(), user, trustedIDs)
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

	validToken := devicepb.DeviceWebToken_builder{
		WebSessionId:      "llama-session-id-1234",
		BrowserUserAgent:  sampleUserAgent,
		BrowserIp:         sampleIP,
		User:              "llama",
		ExpectedDeviceIds: []string{"device-id-1"},
	}.Build()

	// Success scenario.
	t.Run("success", func(t *testing.T) {
		got, err := s.CreateDeviceWebToken(ctx, validToken)
		if err != nil {
			t.Fatalf("CreateDeviceWebToken failed: %v", err)
		}

		// Assert that "usage" field are returned.
		if got.GetId() == "" {
			t.Error("Created DeviceWebToken has an empty ID")
		}
		if got.GetToken() == "" {
			t.Error("Created DeviceWebToken has an empty Token")
		}

		// Assert that no other fields are set.
		want := devicepb.DeviceWebToken_builder{
			Id:    got.GetId(),
			Token: got.GetToken(),
		}.Build()
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
				token.SetWebSessionId("")
			}),
			wantErr: "web session ID",
		},
		{
			name: "BrowserUserAgent empty",
			token: makeToken(func(token *devicepb.DeviceWebToken) {
				token.SetBrowserUserAgent("")
			}),
			wantErr: "user agent",
		},
		{
			name: "BrowserIp empty",
			token: makeToken(func(token *devicepb.DeviceWebToken) {
				token.SetBrowserIp("")
			}),
			wantErr: "IP",
		},
		{
			name: "User empty",
			token: makeToken(func(token *devicepb.DeviceWebToken) {
				token.SetUser("")
			}),
			wantErr: "user",
		},
		{
			name: "ExpectedDeviceIds nil",
			token: makeToken(func(token *devicepb.DeviceWebToken) {
				token.SetExpectedDeviceIds(nil)
			}),
			wantErr: "device IDs",
		},
		{
			name: "empty ExpectedDeviceIds item",
			token: makeToken(func(token *devicepb.DeviceWebToken) {
				token.SetExpectedDeviceIds([]string{
					"device-id-1",
					"", // invalid!
					"device-id-3",
				})
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

	sampleToken := devicepb.DeviceWebToken_builder{
		WebSessionId:      "llama-session-id-1234",
		BrowserUserAgent:  sampleUserAgent,
		BrowserIp:         sampleIP,
		User:              "llama",
		ExpectedDeviceIds: []string{authenticatedDeviceID, "llama-dev-2"},
	}.Build()

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
		wantWeb.SetId(token.GetId())
		wantWeb.SetToken("")
		if diff := cmp.Diff(wantWeb, gotWeb, protocmp.Transform()); diff != "" {
			t.Errorf("SpendDeviceWebToken webToken mismatch (-want +got)\n%s", diff)
		}

		// Verify DeviceConfirmationToken.
		if gotConfirm.GetToken() == "" {
			t.Errorf("SpendDeviceWebToken returned a nil or empty confirm token: %#v", gotConfirm)
		}
		wantConfirm := devicepb.DeviceConfirmationToken_builder{
			Id:    token.GetId(), // same underlying attempt ID
			Token: gotConfirm.GetToken(),
		}.Build()
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

		_, _, err := s.SpendDeviceWebToken(ctx, devicepb.DeviceWebToken_builder{
			Id:    "valid-token-id",
			Token: "validtokenb64",
		}.Build(), "" /* authenticatedDeviceID */)
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
				return devicepb.DeviceWebToken_builder{
					Id:    "unknown-token-ID",
					Token: invalidTokenB64,
				}.Build()
			},
			assertErrorType: trace.IsNotFound,
		},
		{
			name: "ID empty",
			createToken: func(_ *testing.T) *devicepb.DeviceWebToken {
				return devicepb.DeviceWebToken_builder{
					Token: invalidTokenB64,
				}.Build()
			},
			wantErr: "token ID required",
		},
		{
			name: "Token empty",
			createToken: func(t *testing.T) *devicepb.DeviceWebToken {
				token := createWebToken(t)
				token.SetToken("") // Same as an invalid token.
				return token
			},
			wantErr:     "invalid device token",
			wantDeleted: true,
		},
		{
			name: "Token invalid",
			createToken: func(t *testing.T) *devicepb.DeviceWebToken {
				token := createWebToken(t)
				token.SetToken(invalidTokenB64)
				return token
			},
			wantErr:     "invalid device token",
			wantDeleted: true,
		},
		{
			name: "Token not base64",
			createToken: func(t *testing.T) *devicepb.DeviceWebToken {
				token := createWebToken(t)
				token.SetToken(invalidToken) // bad encoding
				return token
			},
			wantErr:     "base64",
			wantDeleted: true,
		},
	}
	for _, test := range tests {

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
	sampleToken := devicepb.DeviceWebToken_builder{
		WebSessionId:      "llama-session-id",
		BrowserUserAgent:  sampleUserAgent,
		BrowserIp:         sampleIP,
		User:              "llama",
		ExpectedDeviceIds: []string{authenticatedDeviceID, "llama2"},
	}.Build()

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
			WebSessionID:          sampleToken.GetWebSessionId(),
			User:                  sampleToken.GetUser(),
			BrowserIP:             sampleToken.GetBrowserIp(),
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
				return devicepb.DeviceConfirmationToken_builder{
					Id:    "not a token ID",
					Token: "ACBDEF0123456789", // valid b64
				}.Build()
			},
			assertErr: trace.IsNotFound,
		},
		{
			name: "invalid token",
			createToken: func(*testing.T) *devicepb.DeviceConfirmationToken {
				_, token := createConfirmToken(t)
				token.SetToken("notavalidtoken") // valid b64
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
				return devicepb.DeviceConfirmationToken_builder{
					Id:    webToken.GetId(),
					Token: webToken.GetToken(),
				}.Build()
			},
			assertErr: trace.IsBadParameter,
			wantErr:   "attempt state",
		},
	}
	for _, test := range tests {
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
	sampleToken := devicepb.DeviceWebToken_builder{
		WebSessionId:      "llama-session-id",
		BrowserUserAgent:  sampleUserAgent,
		BrowserIp:         sampleIP,
		User:              "llama",
		ExpectedDeviceIds: []string{authenticatedDeviceID, "llama2"},
	}.Build()

	createWebToken := func(t *testing.T) *devicepb.DeviceWebToken {
		webToken, err := s.CreateDeviceWebToken(ctx, sampleToken)
		if err != nil {
			t.Fatalf("CreateDeviceWebToken failed: %v", err)
		}
		return webToken
	}

	safeToken := base64.RawStdEncoding.EncodeToString([]byte(`value does not matter`))

	assertDeleted := func(t *testing.T, attemptID string) {
		if _, _, err := s.SpendDeviceWebToken(ctx, devicepb.DeviceWebToken_builder{
			Id:    attemptID,
			Token: safeToken,
		}.Build(), authenticatedDeviceID); !trace.IsNotFound(err) {
			t.Errorf("SpendDeviceWebToken returned err=%v (%T), want NotFound", err, trace.Unwrap(err))
		}
	}

	t.Run("delete web token", func(t *testing.T) {
		t.Parallel()

		webToken := createWebToken(t)
		if err := s.DeleteDeviceWebAuthenticationAttempt(ctx, webToken.GetId()); err != nil {
			t.Fatalf("DeleteDeviceWebAuthenticationAttempt failed: %v", err)
		}

		assertDeleted(t, webToken.GetId())
	})

	t.Run("delete confirmation token", func(t *testing.T) {
		t.Parallel()

		// Create the DeviceConfirmationToken.
		webToken := createWebToken(t)
		_, confirmToken, err := s.SpendDeviceWebToken(ctx, webToken, authenticatedDeviceID)
		if err != nil {
			t.Fatalf("SpendDeviceWebToken failed: %v", err)
		}

		if err := s.DeleteDeviceWebAuthenticationAttempt(ctx, confirmToken.GetId()); err != nil {
			t.Fatalf("DeleteDeviceWebAuthenticationAttempt failed: %v", err)
		}

		assertDeleted(t, confirmToken.GetId())
	})

	t.Run("not found", func(t *testing.T) {
		if err := s.DeleteDeviceWebAuthenticationAttempt(ctx, "unknown token ID"); !trace.IsNotFound(err) {
			t.Errorf("DeleteDeviceWebAuthenticationAttempt returned err=%v (%T), want NotFound", err, trace.Unwrap(err))
		}
	})
}

func TestS_unassignDevice_missingByUserIndex(t *testing.T) {
	t.Parallel()

	env := mustNewEnv()
	defer env.Close()

	s := env.S
	ctx := context.Background()

	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}
	pubKeyDER, err := x509.MarshalPKIXPublicKey(privKey.Public())
	if err != nil {
		t.Fatalf("MarshalPKIXPublicKey failed: %v", err)
	}

	// Assign device to user, simulating a legacy user without the by_user index.
	const user = "llama"
	dev, err := s.CreateDevice(ctx, devicepb.Device_builder{
		Id:           "62005f63-7e12-451e-b836-586c07337e54",
		OsType:       devicepb.OSType_OS_TYPE_MACOS,
		AssetTag:     "llama1",
		CreateTime:   timestamppb.Now(),
		UpdateTime:   timestamppb.Now(),
		EnrollStatus: devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_ENROLLED,
		Credential: devicepb.DeviceCredential_builder{
			Id:           "fbcc498e-e773-40ea-a5a9-90fcb571b34a",
			PublicKeyDer: pubKeyDER,
		}.Build(),
		Owner: user,
	}.Build(), true /* createAsResource */)
	if err != nil {
		t.Fatalf("CreateDevice failed: %v", err)
	}

	// Unassign the device. Any action that causes the device to be unassigned
	// works here.
	if _, err := s.UpdateDevice(ctx, dev.GetId(), func(d *devicepb.Device) *devicepb.Device {
		d.SetEnrollStatus(devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED)
		return d
	}); err != nil {
		t.Fatalf("UpdateDevice errored, failed to uneroll the device: %v", err)
	}
}

// diffDevices diffs two slices of devices, sorting both by ID first.
func diffDevices(want, got []*devicepb.Device) string {
	slices.SortFunc(want, func(a, b *devicepb.Device) int {
		return strings.Compare(a.GetId(), b.GetId())
	})
	slices.SortFunc(got, func(a, b *devicepb.Device) int {
		return strings.Compare(a.GetId(), b.GetId())
	})
	return cmp.Diff(want, got, protocmp.Transform())
}

// storageEnv groups the necessary components to test storage.
type storageEnv struct {
	// Clock is the underlying FakeClock.
	// nil if withClock() is used with a real clock.
	Clock           clocki.FakeClock
	IdentityService *local.IdentityService
	S               *storage.S
	modules         *modulestest.Modules
	memClock        clockwork.Clock // actual mem clock, always set.
	mem             *memory.Memory

	bcryptCostOverride int
}

func (e *storageEnv) Close() error {
	if e.mem != nil {
		return e.mem.Close()
	}
	return nil
}

type opt func(*storageEnv)

// withBCryptCost makes the storage hash enrollment tokens with the given
// bcrypt cost instead of bcrypt.MinCost.
func withBCryptCost(cost int) opt {
	return func(e *storageEnv) {
		e.bcryptCostOverride = cost
	}
}

func mustNewEnv(opts ...opt) *storageEnv {
	env, err := newEnv(opts...)
	if err != nil {
		panic(err)
	}
	return env
}

func newEnv(opts ...opt) (*storageEnv, error) {
	env := &storageEnv{
		modules:            modulestest.EnterpriseModules(),
		bcryptCostOverride: bcrypt.MinCost,
	}
	for _, opt := range opts {
		opt(env)
	}

	// Use a FakeClock if no clock was provided (via withClock), otherwise do our
	// best to honor the clock we got.
	// Initially storageEnv only allowed for a FakeClock, this was retrofited
	// later.
	if fakeClock, ok := env.memClock.(clocki.FakeClock); ok {
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

	env.IdentityService, err = local.NewIdentityService(env.mem)
	if err != nil {
		return nil, err
	}
	env.S, err = storage.New(storage.Params{
		Logger:             logger,
		Backend:            env.mem,
		UsersService:       env.IdentityService,
		BCryptCostOverride: env.bcryptCostOverride,
		Modules:            env.modules,
	})
	if err != nil {
		return nil, err
	}

	ok = true
	return env, nil
}
