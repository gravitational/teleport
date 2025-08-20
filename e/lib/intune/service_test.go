package intune

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/testing/protocmp"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/e/lib/intune/api"
	"github.com/gravitational/teleport/e/lib/intune/testenv"
	"github.com/gravitational/teleport/integrations/lib/testing/integration"
)

const (
	waitForDeviceSyncTimeout = 5 * time.Second
	waitForDeviceSyncTick    = 100 * time.Millisecond
)

var devicesCmpOpts = []cmp.Option{
	cmpopts.SortSlices(func(a, b *devicepb.Device) bool {
		return a.AssetTag < b.AssetTag
	}),
	protocmp.Transform(),
	protocmp.IgnoreFields(&devicepb.Device{}, "api_version", "id", "create_time", "update_time"),
	protocmp.IgnoreFields(&devicepb.DeviceProfile{}, "update_time"),
}

// TestRun_fullSync runs a full sync once and verifies that only valid devices are pushed to
// Teleport.
func TestRun_fullSync(t *testing.T) {
	clock := clockwork.NewFakeClockAt(time.Date(2023, 1, 2, 3, 4, 5, 0, time.UTC))
	env := testenv.MustNew(t, &testenv.Config{
		Clock:          clock,
		DeviceTrustEnv: true,
	})
	fakeAPI := env.API
	devicesClient := env.DevicesClient

	t0 := clock.Now()
	clock.Advance(1 * time.Minute)
	t1 := clock.Now()
	clock.Advance(24 * time.Hour)

	intuneDevices := []*api.ManagedDevice{
		// A complete device.
		{
			ID:                      "1",
			LastSyncDateTime:        t0,
			DeviceRegistrationState: "registered",
			SerialNumber:            "CXXXXXXXXX01",
			Model:                   "MacBookPro9,2",
			OperatingSystem:         "macOS",
			OSVersion:               "13.4.1 (22F82)",
		},
		// A "minimal" device. Unexpected, but carries enough info to be created (and later let the user
		// pass auto-enrollment).
		{
			ID:                      "2",
			LastSyncDateTime:        t1,
			DeviceRegistrationState: "registered",
			SerialNumber:            "DYXXXXXXXX02",
			Model:                   "",
			OperatingSystem:         "macOS",
			OSVersion:               "",
		},
		// Invalid – unsupported OS.
		{
			ID:                      "invalid1",
			LastSyncDateTime:        t1,
			DeviceRegistrationState: "registered",
			SerialNumber:            "1234",
			OperatingSystem:         "Android",
		},
		// Invalid – no serial number.
		{
			ID:                      "invalid2",
			LastSyncDateTime:        t1,
			DeviceRegistrationState: "registered",
			SerialNumber:            "",
			OperatingSystem:         "macOS",
		},
		// Invalid nil device.
		nil,
		// Invalid empty device.
		{},
		// Skipped because of device registration state.
		{
			ID:                      "skipped",
			LastSyncDateTime:        t1,
			DeviceRegistrationState: "notRegistered",
			SerialNumber:            "4321",
			OperatingSystem:         "macOS",
			Model:                   "MacBookPro9,2",
			OSVersion:               "13.4.1 (22F82)",
		},
	}
	fakeAPI.SetManagedDevices(intuneDevices)

	source := &devicepb.DeviceSource{Name: "intune", Origin: devicepb.DeviceOrigin_DEVICE_ORIGIN_INTUNE}
	wantDevices := []*devicepb.Device{
		{
			OsType:       devicepb.OSType_OS_TYPE_MACOS,
			AssetTag:     intuneDevices[0].SerialNumber,
			EnrollStatus: devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED,
			Source:       source,
			Profile: &devicepb.DeviceProfile{
				ModelIdentifier: intuneDevices[0].Model,
				OsVersion:       "13.4.1",
				OsBuild:         "22F82",
				ExternalId:      intuneDevices[0].ID,
			},
		},
		{
			OsType:       devicepb.OSType_OS_TYPE_MACOS,
			AssetTag:     intuneDevices[1].SerialNumber,
			EnrollStatus: devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED,
			Source:       source,
			Profile: &devicepb.DeviceProfile{
				ExternalId: intuneDevices[1].ID,
			},
		},
	}

	s := serviceFromEnv(t, env, syncPeriods{})

	go func() {
		if err := s.Run(t.Context()); err != nil {
			assert.ErrorIs(t, err, context.Canceled, "Run returned an unexpected error")
		}
	}()

	// Wait for devices to by synced.
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		got := listAllDevices(t, devicesClient)
		if diff := cmp.Diff(wantDevices, got, devicesCmpOpts...); diff != "" {
			c.Errorf("Run sync mismatch (-want +got)\n%s", diff)
		}
	}, waitForDeviceSyncTimeout, waitForDeviceSyncTick)
}

// TestRun_partialSync runs two partial syncs. It verifies that partial syncs use the highest
// observed LastSyncDateTime to fetch devices during syncs and that they don't delete devices.
func TestRun_partialSync(t *testing.T) {
	clock := clockwork.NewFakeClockAt(time.Date(2023, 1, 2, 3, 4, 5, 0, time.UTC))
	env := testenv.MustNew(t, &testenv.Config{
		Clock:          clock,
		DeviceTrustEnv: true,
	})
	fakeAPI := env.API
	devicesClient := env.DevicesClient

	advanceNow := func() time.Time {
		clock.Advance(1 * time.Minute)
		return clock.Now()
	}

	t0 := clock.Now()
	t1 := advanceNow()

	intuneDevices := []*api.ManagedDevice{
		{
			ID:                      "1",
			LastSyncDateTime:        t0,
			DeviceRegistrationState: "registered",
			SerialNumber:            "CXXXXXXXXX01",
			Model:                   "MacBookPro9,2",
			OperatingSystem:         "macOS",
			OSVersion:               "13.4.1 (22F82)",
		},
		{
			ID:                      "2",
			LastSyncDateTime:        t1,
			DeviceRegistrationState: "registered",
			SerialNumber:            "DYXXXXXXXX02",
			Model:                   "",
			OperatingSystem:         "macOS",
			OSVersion:               "13.4.1 (22F82)",
		},
	}
	fakeAPI.SetManagedDevices(intuneDevices)

	source := &devicepb.DeviceSource{Name: "intune", Origin: devicepb.DeviceOrigin_DEVICE_ORIGIN_INTUNE}

	// Create a device that would get removed during a full sync. Later on verify that it wasn't
	// removed by a partial sync.
	mustBulkCreateDevices(t, devicesClient, []*devicepb.Device{
		{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: "deleteonfullsync1",
			Source:   source,
		},
	})

	s := serviceFromEnv(t, env, syncPeriods{
		syncPeriodFull:    -1,
		syncPeriodPartial: 100 * time.Millisecond,
	})

	go func() {
		if err := s.Run(t.Context()); err != nil {
			assert.ErrorIs(t, err, context.Canceled, "Run returned an unexpected error")
		}
	}()

	// Wait for the first partial sync.
	waitSynced(t, devicesClient, 3, "Expected a partial sync to add two new devices next to one which would be deleted during a full sync")

	// Update OS version of both devices in Intune, but update lastSyncDateTime in only one of them,
	// ensuring that only this device is going to be updated during a partial sync.
	// Also, add a new device.
	t2 := advanceNow()
	intuneDevices[0].OSVersion = "14.0.0 (22F83)"
	intuneDevices[0].LastSyncDateTime = t2
	intuneDevices[1].OSVersion = "14.0.0 (22F83)"
	intuneDevices = append(intuneDevices, &api.ManagedDevice{
		ID:                      "3",
		LastSyncDateTime:        t2,
		DeviceRegistrationState: "registered",
		SerialNumber:            "EZXXXXXXXX03",
		Model:                   "",
		OperatingSystem:         "macOS",
		OSVersion:               "",
	})
	fakeAPI.SetManagedDevices(intuneDevices)

	// Wait for the next sync.
	waitSynced(t, devicesClient, 4, "Expected a partial sync to add just one new device")

	// Verify that the right device had its OS version updated.
	got := listAllDevices(t, devicesClient)
	updatedDeviceIdx := slices.IndexFunc(got, func(d *devicepb.Device) bool {
		return d.AssetTag == intuneDevices[0].SerialNumber
	})
	notUpdatedDeviceIdx := slices.IndexFunc(got, func(d *devicepb.Device) bool {
		return d.AssetTag == intuneDevices[1].SerialNumber
	})
	updatedDevice := got[updatedDeviceIdx]
	notUpdatedDevice := got[notUpdatedDeviceIdx]
	require.Equal(t, "14.0.0", updatedDevice.Profile.OsVersion, "device with asset tag %s was not updated", updatedDevice.AssetTag)
	require.Equal(t, "13.4.1", notUpdatedDevice.Profile.OsVersion, "device with asset tag %s was updated", notUpdatedDevice.AssetTag)
}

// TestRun_fullSyncThenPartialSync verifies that the partial sync uses the highest observed
// lastSyncDateTime from the full sync to fetch only a small subset of devices from Intune.
func TestRun_fullSyncThenPartialSync(t *testing.T) {
	clock := clockwork.NewFakeClockAt(time.Date(2023, 1, 2, 3, 4, 5, 0, time.UTC))
	env := testenv.MustNew(t, &testenv.Config{
		Clock:          clock,
		DeviceTrustEnv: true,
	})
	fakeAPI := env.API
	devicesClient := env.DevicesClient

	advanceNow := func() time.Time {
		clock.Advance(1 * time.Minute)
		return clock.Now()
	}

	t0 := clock.Now()

	// Create only a single device in Intune.
	intuneDevices := []*api.ManagedDevice{
		{
			ID:                      "1",
			LastSyncDateTime:        t0,
			DeviceRegistrationState: "registered",
			SerialNumber:            "CXXXXXXXXX01",
			Model:                   "MacBookPro9,2",
			OperatingSystem:         "macOS",
			OSVersion:               "13.4.1 (22F82)",
		},
	}
	fakeAPI.SetManagedDevices(intuneDevices)

	const syncPeriodFull = 3 * time.Second
	s := serviceFromEnv(t, env, syncPeriods{
		syncPeriodFull:    syncPeriodFull,
		syncPeriodPartial: 100 * time.Millisecond,
	})

	go func() {
		if err := s.Run(t.Context()); err != nil {
			assert.ErrorIs(t, err, context.Canceled, "Run returned an unexpected error")
		}
	}()

	// Wait for the full sync.
	waitSynced(t, devicesClient, 1)

	// Update OS version of the existing device in Intune, but don't update lastSyncDateTime – the OS
	// version in Teleport shouldn't get updated. Add a new device.
	t1 := advanceNow()
	intuneDevices[0].OSVersion = "14.0.0 (22F83)"
	intuneDevices = append(intuneDevices, &api.ManagedDevice{
		ID:                      "2",
		LastSyncDateTime:        t1,
		DeviceRegistrationState: "registered",
		SerialNumber:            "DYXXXXXXXX02",
		Model:                   "",
		OperatingSystem:         "macOS",
		OSVersion:               "13.4.1 (22F82)",
	})
	fakeAPI.SetManagedDevices(intuneDevices)
	// Create a device with missing details which would get removed during a full sync. Its existence
	// confirms that only a partial sync took place.
	source := &devicepb.DeviceSource{Name: "intune", Origin: devicepb.DeviceOrigin_DEVICE_ORIGIN_INTUNE}
	mustBulkCreateDevices(t, devicesClient, []*devicepb.Device{
		{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: "deleteonfullsync1",
			Source:   source,
		},
	})

	// Wait for the partial sync.
	waitSynced(t, devicesClient, 3, "Expected the partial sync to add one new device and keep the one that's supposed to be deleted on full sync")
	// Verify that the OS version wasn't updated.
	got := listAllDevices(t, devicesClient)
	ogDeviceIdx := slices.IndexFunc(got, func(d *devicepb.Device) bool {
		return d.AssetTag == intuneDevices[0].SerialNumber
	})
	ogDevice := got[ogDeviceIdx]
	require.Equal(t, "13.4.1", ogDevice.Profile.OsVersion, "OS version has changed, indicating that the service didn't fetch a subset of devices")

	// Update lastSyncDateTime of the og device and add another device.
	t2 := advanceNow()
	intuneDevices[0].LastSyncDateTime = t2
	intuneDevices = append(intuneDevices, &api.ManagedDevice{
		ID:                      "3",
		LastSyncDateTime:        t2,
		DeviceRegistrationState: "registered",
		SerialNumber:            "EZXXXXXXXX03",
		Model:                   "",
		OperatingSystem:         "macOS",
		OSVersion:               "",
	})
	fakeAPI.SetManagedDevices(intuneDevices)

	// Wait for the next partial sync.
	waitSynced(t, devicesClient, 4, "Expected the partial sync to add just one new device")
	// Verify that the OS version was updated.
	got = listAllDevices(t, devicesClient)
	ogDeviceIdx = slices.IndexFunc(got, func(d *devicepb.Device) bool {
		return d.AssetTag == intuneDevices[0].SerialNumber
	})
	ogDevice = got[ogDeviceIdx]
	require.Equal(t, "14.0.0", ogDevice.Profile.OsVersion, "OS version has not changed, indicating that the OG device wasn't updated during a partial sync")

	// Wait for a full sync.
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Len(c, listAllDevices(t, devicesClient), 3, "Unexpected number of devices")
	}, syncPeriodFull*2, 100*time.Millisecond, "Expected a full sync to happen and remove one device")
}

func listAllDevices(t *testing.T, devicesClient devicepb.DeviceTrustServiceClient) []*devicepb.Device {
	t.Helper()
	var devs []*devicepb.Device
	var pageToken string
	for {
		resp, err := devicesClient.ListDevices(t.Context(), &devicepb.ListDevicesRequest{
			PageToken: pageToken,
			View:      devicepb.DeviceView_DEVICE_VIEW_RESOURCE,
		})
		require.NoError(t, err)
		devs = append(devs, resp.Devices...)
		if resp.NextPageToken == "" {
			return devs
		}
		pageToken = resp.NextPageToken
	}
}

func waitSynced(t *testing.T, devicesClient devicepb.DeviceTrustServiceClient, num int, msgAndArgs ...any) {
	t.Helper()
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Len(c, listAllDevices(t, devicesClient), num, "Unexpected number of devices")
	}, 1*time.Second, 100*time.Millisecond, msgAndArgs...)
}

type syncPeriods struct {
	syncPeriodPartial time.Duration
	syncPeriodFull    time.Duration
}

func serviceFromEnv(t *testing.T, env *testenv.Env, syncPeriods syncPeriods) *Service {
	t.Helper()
	s, err := NewService(t.Context(), Config{
		syncImmediately:   true,
		syncPeriodPartial: syncPeriods.syncPeriodPartial,
		syncPeriodFull:    syncPeriods.syncPeriodFull,
		APIConfig:         api.Config{AppCredentials: *testenv.DefaultApps[0]},
		DevicesClient:     env.DevicesClient,
		HTTPClient:        env.HTTPClient,
		StatusSink:        &integration.FakeStatusSink{},
		Clock:             env.Clock,
		Logger:            env.Logger,
	})
	require.NoError(t, err)
	return s
}

func mustBulkCreateDevices(t *testing.T, devicesClient devicepb.DeviceTrustServiceClient, devices []*devicepb.Device) {
	t.Helper()
	resp, err := devicesClient.BulkCreateDevices(t.Context(), &devicepb.BulkCreateDevicesRequest{
		Devices: devices,
	})
	require.NoError(t, err)
	for i, s := range resp.Devices {
		require.Equal(t, codes.OK, codes.Code(s.GetStatus().GetCode()), "device #%v has non-OK status: %+v", i, s)
	}
}
