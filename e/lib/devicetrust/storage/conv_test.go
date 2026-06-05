package storage_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/devicetrust"
	"github.com/gravitational/teleport/lib/services"
)

func TestDeviceFromBackendItem(t *testing.T) {
	t.Parallel()

	env := mustNewEnv()
	defer env.Close()

	s := env.S
	ctx := context.Background()

	const resource1Tag = "AAA000000000"

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

	// Create a device without collected data as a resource.
	createResp, err := s.CreateDevice(ctx, resource1Dev, true /* createAsResource */)
	if err != nil {
		t.Fatalf("CreateDevice failed: %v", err)
	}

	// Get the device from the backend.
	key := backend.NewKey(append(devicetrust.DevicesIDPrefix, resource1Dev.GetId())...)
	item, err := env.mem.Get(ctx, key)
	if err != nil {
		t.Fatalf("backend.Get failed: %v", err)
	}

	// Unmarshal the device from the backend item and ensures that we previously called [services.SetUnmarshalDeviceFromBackendItemConv].
	got, err := services.UnmarshalDeviceFromBackendItem(*item)
	if err != nil {
		t.Fatalf("UnmarshalDeviceFromBackendItem failed: %v", err)
	}

	// Compare the device we got from the backend and unmarshaled using [services.UnmarshalDeviceFromBackendItem]
	// with the device we created.
	if diff := cmp.Diff(createResp, got, protocmp.Transform()); diff != "" {
		t.Errorf("UnmarshalDeviceFromBackendItem mismatch (-want +got)\n%s", diff)
	}
}
