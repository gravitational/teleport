package intune

import (
	"context"
	"slices"
	"testing"
	"testing/synctest"
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
	"github.com/gravitational/teleport/lib/msgraph"
)

var devicesCmpOpts = []cmp.Option{
	cmpopts.SortSlices(func(a, b *devicepb.Device) bool {
		return a.GetAssetTag() < b.GetAssetTag()
	}),
	protocmp.Transform(),
	protocmp.IgnoreFields(&devicepb.Device{}, "api_version", "id", "create_time", "update_time"),
	protocmp.IgnoreFields(&devicepb.DeviceProfile{}, "update_time"),
}

var source = devicepb.DeviceSource_builder{Name: "intune", Origin: devicepb.DeviceOrigin_DEVICE_ORIGIN_INTUNE}.Build()

// TestRun_fullSync runs a full sync once and verifies that only valid devices are pushed to
// Teleport.
func TestRun_fullSync(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		env := testenv.MustNew(t, &testenv.Config{
			DeviceTrustEnv: true,
			InMemory:       true,
		})
		fakeAPI := env.API
		devicesClient := env.DevicesClient
		ctx := t.Context()

		t0 := time.Now()
		t1 := t0.Add(1 * time.Minute)

		intuneDevices := []*msgraph.ManagedDevice{
			// A complete device.
			{
				ID:                      "1",
				LastSyncDateTime:        t0,
				DeviceRegistrationState: "registered",
				SerialNumber:            "CXXXXXXXXX01",
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
				OperatingSystem:         "macOS",
				OSVersion:               "",
			},
			// An iPhone.
			{
				ID:                      "3",
				LastSyncDateTime:        t1,
				DeviceRegistrationState: "registered",
				SerialNumber:            "FYXXXXXXXX03",
				OperatingSystem:         "iOS",
				Model:                   "iPhone 16e",
				OSVersion:               "26.3.1",
			},
			// An iPad.
			{
				ID:                      "4",
				LastSyncDateTime:        t1,
				DeviceRegistrationState: "registered",
				SerialNumber:            "GYXXXXXXXX04",
				OperatingSystem:         "iOS",
				Model:                   "iPad (10th generation)",
				OSVersion:               "18.6",
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
				OSVersion:               "13.4.1 (22F82)",
			},
		}
		fakeAPI.SetManagedDevices(intuneDevices)

		// Create a device that is going to get removed during a full sync since it comes from Intune but
		// has no matching external ID.
		mustBulkCreateDevices(t, devicesClient, []*devicepb.Device{
			devicepb.Device_builder{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "deleteonfullsync1",
				Source:   source,
			}.Build(),
		})

		s := serviceFromEnv(t, env, syncPeriods{})

		errCh := make(chan error, 1)
		go func() { errCh <- s.Run(ctx) }()
		t.Cleanup(func() {
			assert.ErrorIs(t, <-errCh, context.Canceled, "Run returned an unexpected error")
		})

		// The startup full sync runs immediately. Wait for it to settle.
		synctest.Wait()
		wantDevs := []*devicepb.Device{
			devicepb.Device_builder{
				OsType:       devicepb.OSType_OS_TYPE_MACOS,
				AssetTag:     intuneDevices[0].SerialNumber,
				EnrollStatus: devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED,
				Source:       source,
				Profile: devicepb.DeviceProfile_builder{
					OsVersion:  "13.4.1",
					OsBuild:    "22F82",
					ExternalId: intuneDevices[0].ID,
				}.Build(),
			}.Build(),
			devicepb.Device_builder{
				OsType:       devicepb.OSType_OS_TYPE_MACOS,
				AssetTag:     intuneDevices[1].SerialNumber,
				EnrollStatus: devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED,
				Source:       source,
				Profile: devicepb.DeviceProfile_builder{
					ExternalId: intuneDevices[1].ID,
				}.Build(),
			}.Build(),
			devicepb.Device_builder{
				OsType:       devicepb.OSType_OS_TYPE_IOS,
				AssetTag:     intuneDevices[2].SerialNumber,
				EnrollStatus: devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED,
				Source:       source,
				Profile: devicepb.DeviceProfile_builder{
					OsVersion:  "26.3.1",
					ExternalId: intuneDevices[2].ID,
				}.Build(),
			}.Build(),
			devicepb.Device_builder{
				OsType:       devicepb.OSType_OS_TYPE_IPADOS,
				AssetTag:     intuneDevices[3].SerialNumber,
				EnrollStatus: devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED,
				Source:       source,
				Profile: devicepb.DeviceProfile_builder{
					OsVersion:  "18.6",
					ExternalId: intuneDevices[3].ID,
				}.Build(),
			}.Build(),
		}
		if diff := cmp.Diff(wantDevs, listAllDevices(ctx, t, devicesClient), devicesCmpOpts...); diff != "" {
			t.Errorf("Device mismatch (-want +got)\n%s", diff)
		}
	})
}

// TestRun_partialSync runs two partial syncs. It verifies that partial syncs use the highest
// observed LastSyncDateTime to fetch devices during syncs and that they don't delete devices.
func TestRun_partialSync(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		env := testenv.MustNew(t, &testenv.Config{
			DeviceTrustEnv: true,
			InMemory:       true,
		})
		fakeAPI := env.API
		devicesClient := env.DevicesClient
		ctx := t.Context()

		t0 := time.Now()
		t1 := t0.Add(1 * time.Minute)

		intuneDevices := []*msgraph.ManagedDevice{
			{
				ID:                      "1",
				LastSyncDateTime:        t0,
				DeviceRegistrationState: "registered",
				SerialNumber:            "CXXXXXXXXX01",
				OperatingSystem:         "macOS",
				OSVersion:               "13.4.1 (22F82)",
			},
			{
				ID:                      "2",
				LastSyncDateTime:        t1,
				DeviceRegistrationState: "registered",
				SerialNumber:            "DYXXXXXXXX02",
				OperatingSystem:         "macOS",
				OSVersion:               "13.4.1 (22F82)",
			},
		}
		fakeAPI.SetManagedDevices(intuneDevices)

		// Create a device that would get removed during a full sync. Later on verify that it wasn't
		// removed by a partial sync.
		mustBulkCreateDevices(t, devicesClient, []*devicepb.Device{
			devicepb.Device_builder{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "deleteonfullsync1",
				Source:   source,
			}.Build(),
		})

		const syncPeriodPartial = 1 * time.Second
		s := serviceFromEnv(t, env, syncPeriods{
			syncPeriodFull:    -1,
			syncPeriodPartial: syncPeriodPartial,
		})

		errCh := make(chan error, 1)
		go func() { errCh <- s.Run(ctx) }()
		t.Cleanup(func() {
			assert.ErrorIs(t, <-errCh, context.Canceled, "Run returned an unexpected error")
		})

		// The first partial sync runs immediately on startup.
		synctest.Wait()
		requireDeviceCount(t, devicesClient, 3, "Expected a partial sync to add two new devices next to one which would be deleted during a full sync")

		// Update OS version of both devices in Intune, but update lastSyncDateTime in only one of them,
		// ensuring that only this device is going to be updated during a partial sync.
		// Also, add a new device.
		t2 := t1.Add(1 * time.Minute)
		intuneDevices[0].OSVersion = "14.0.0 (22F83)"
		intuneDevices[0].LastSyncDateTime = t2
		intuneDevices[1].OSVersion = "14.0.0 (22F83)"
		intuneDevices = append(intuneDevices, &msgraph.ManagedDevice{
			ID:                      "3",
			LastSyncDateTime:        t2,
			DeviceRegistrationState: "registered",
			SerialNumber:            "EZXXXXXXXX03",
			OperatingSystem:         "macOS",
			OSVersion:               "",
		})
		fakeAPI.SetManagedDevices(intuneDevices)

		// Advance one period to trigger the next partial sync.
		time.Sleep(syncPeriodPartial)
		synctest.Wait()
		requireDeviceCount(t, devicesClient, 4, "Expected a partial sync to add just one new device")

		// Verify that the right device had its OS version updated.
		got := listAllDevices(ctx, t, devicesClient)
		updatedDeviceIdx := slices.IndexFunc(got, func(d *devicepb.Device) bool {
			return d.GetAssetTag() == intuneDevices[0].SerialNumber
		})
		notUpdatedDeviceIdx := slices.IndexFunc(got, func(d *devicepb.Device) bool {
			return d.GetAssetTag() == intuneDevices[1].SerialNumber
		})
		updatedDevice := got[updatedDeviceIdx]
		notUpdatedDevice := got[notUpdatedDeviceIdx]
		require.Equal(t, "14.0.0", updatedDevice.GetProfile().GetOsVersion(), "device with asset tag %s was not updated", updatedDevice.GetAssetTag())
		require.Equal(t, "13.4.1", notUpdatedDevice.GetProfile().GetOsVersion(), "device with asset tag %s was updated", notUpdatedDevice.GetAssetTag())
	})
}

// TestRun_fullSyncThenPartialSync verifies that the partial sync uses the highest observed
// lastSyncDateTime from the full sync to fetch only a small subset of devices from Intune.
func TestRun_fullSyncThenPartialSync(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		env := testenv.MustNew(t, &testenv.Config{
			DeviceTrustEnv: true,
			InMemory:       true,
		})
		fakeAPI := env.API
		devicesClient := env.DevicesClient
		ctx := t.Context()
		t0 := time.Now()

		// Create only a single device in Intune.
		intuneDevices := []*msgraph.ManagedDevice{
			{
				ID:                      "1",
				LastSyncDateTime:        t0,
				DeviceRegistrationState: "registered",
				SerialNumber:            "CXXXXXXXXX01",
				OperatingSystem:         "macOS",
				OSVersion:               "13.4.1 (22F82)",
			},
		}
		fakeAPI.SetManagedDevices(intuneDevices)

		// With these periods the schedule between full syncs is two partial syncs
		// (at 1s and 2s) followed by a full sync (at 3s). Every subsequent sync is
		// then one period away, so time.Sleep(syncPeriodPartial) fires exactly one.
		const (
			syncPeriodFull    = 3 * time.Second
			syncPeriodPartial = 1 * time.Second
		)
		s := serviceFromEnv(t, env, syncPeriods{
			syncPeriodFull:    syncPeriodFull,
			syncPeriodPartial: syncPeriodPartial,
		})

		errCh := make(chan error, 1)
		go func() { errCh <- s.Run(ctx) }()
		t.Cleanup(func() {
			assert.ErrorIs(t, <-errCh, context.Canceled, "Run returned an unexpected error")
		})

		// The full sync runs immediately on startup.
		synctest.Wait()
		requireDeviceCount(t, devicesClient, 1)

		// Update OS version of the existing device in Intune, but don't update lastSyncDateTime – the OS
		// version in Teleport shouldn't get updated. Add a new device.
		t1 := t0.Add(1 * time.Minute)
		intuneDevices[0].OSVersion = "14.0.0 (22F83)"
		intuneDevices = append(intuneDevices, &msgraph.ManagedDevice{
			ID:                      "2",
			LastSyncDateTime:        t1,
			DeviceRegistrationState: "registered",
			SerialNumber:            "DYXXXXXXXX02",
			OperatingSystem:         "macOS",
			OSVersion:               "13.4.1 (22F82)",
		})
		fakeAPI.SetManagedDevices(intuneDevices)
		// Create a device with missing details which would get removed during a full sync. Its existence
		// confirms that only a partial sync took place.
		mustBulkCreateDevices(t, devicesClient, []*devicepb.Device{
			devicepb.Device_builder{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "deleteonfullsync1",
				Source:   source,
			}.Build(),
		})

		// Advance one period to trigger the partial sync.
		time.Sleep(syncPeriodPartial)
		synctest.Wait()
		requireDeviceCount(t, devicesClient, 3, "Expected the partial sync to add one new device and keep the one that's supposed to be deleted on full sync")
		// Verify that the OS version wasn't updated.
		got := listAllDevices(ctx, t, devicesClient)
		ogDeviceIdx := slices.IndexFunc(got, func(d *devicepb.Device) bool {
			return d.GetAssetTag() == intuneDevices[0].SerialNumber
		})
		ogDevice := got[ogDeviceIdx]
		require.Equal(t, "13.4.1", ogDevice.GetProfile().GetOsVersion(), "OS version has changed, indicating that the service didn't fetch a subset of devices")

		// Update lastSyncDateTime of the og device and add another device.
		t2 := t1.Add(1 * time.Minute)
		intuneDevices[0].LastSyncDateTime = t2
		intuneDevices = append(intuneDevices, &msgraph.ManagedDevice{
			ID:                      "3",
			LastSyncDateTime:        t2,
			DeviceRegistrationState: "registered",
			SerialNumber:            "EZXXXXXXXX03",
			OperatingSystem:         "macOS",
			OSVersion:               "",
		})
		fakeAPI.SetManagedDevices(intuneDevices)

		// Advance one period to trigger the next partial sync.
		time.Sleep(syncPeriodPartial)
		synctest.Wait()
		requireDeviceCount(t, devicesClient, 4, "Expected the partial sync to add just one new device")
		// Verify that the OS version was updated.
		got = listAllDevices(ctx, t, devicesClient)
		ogDeviceIdx = slices.IndexFunc(got, func(d *devicepb.Device) bool {
			return d.GetAssetTag() == intuneDevices[0].SerialNumber
		})
		ogDevice = got[ogDeviceIdx]
		require.Equal(t, "14.0.0", ogDevice.GetProfile().GetOsVersion(), "OS version has not changed, indicating that the OG device wasn't updated during a partial sync")

		// Advance one more period to reach the full sync, which removes the device
		// that only ever existed in Teleport.
		time.Sleep(syncPeriodPartial)
		synctest.Wait()
		requireDeviceCount(t, devicesClient, 3, "Expected a full sync to happen and remove one device")
	})
}

// TestRun_deviceConfirmation simulates a situation where, for whatever reason, the Intune client
// didn't receive a device from the API despite the device still being present in the Intune
// inventory. The service is supposed to check if the device indeed doesn't exist in Intune before
// removing it from Teleport's inventory.
func TestRun_deviceConfirmation(t *testing.T) {
	t.Parallel()
	clock := clockwork.NewFakeClockAt(time.Date(2023, 1, 2, 3, 4, 5, 0, time.UTC))
	env := testenv.MustNew(t, &testenv.Config{
		Clock:          clock,
		DeviceTrustEnv: true,
	})
	fakeAPI := env.API
	devicesClient := env.DevicesClient

	t0 := clock.Now()

	intuneDevices := []*msgraph.ManagedDevice{
		{
			ID:                      "1",
			LastSyncDateTime:        t0,
			DeviceRegistrationState: "registered",
			SerialNumber:            "CXXXXXXXXX01",
			OperatingSystem:         "macOS",
			OSVersion:               "13.4.1 (22F82)",
		},
		{
			ID:                      "2",
			LastSyncDateTime:        t0,
			DeviceRegistrationState: "registered",
			SerialNumber:            "DYXXXXXXXX02",
			OperatingSystem:         "macOS",
		},
		{
			ID:                      "3",
			LastSyncDateTime:        t0,
			DeviceRegistrationState: "registered",
			SerialNumber:            "EZXXXXXXXX03",
			OperatingSystem:         "macOS",
		},
	}
	fakeAPI.SetManagedDevices(intuneDevices)

	s := serviceFromEnv(t, env, syncPeriods{})
	ctx := t.Context()

	// Perform a full sync and verify that all devices got saved.
	_, err := s.RunFullSync(ctx)
	require.NoError(t, err)
	waitSynced(t, devicesClient, 3)

	// Simulate a gap in Intune response.
	fakeAPI.SetSimulatePagingGaps(true)
	// Remove the third device from Intune.
	fakeAPI.SetManagedDevices(intuneDevices[:2])

	// Verify that a full sync doesn't remove the first device despite the list endpoint not returning
	// the first device (because of paging gaps) and that a full sync does remove the third device
	// since it's no longer in Intune.
	_, err = s.RunFullSync(ctx)
	require.NoError(t, err)
	waitForDevices(
		t, devicesClient, []*devicepb.Device{
			devicepb.Device_builder{
				OsType:       devicepb.OSType_OS_TYPE_MACOS,
				AssetTag:     intuneDevices[0].SerialNumber,
				EnrollStatus: devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED,
				Source:       source,
				Profile: devicepb.DeviceProfile_builder{
					OsVersion:       "13.4.1",
					OsBuild:         "22F82",
					ExternalId:      intuneDevices[0].ID,
					ModelIdentifier: "",
				}.Build(),
			}.Build(),
			devicepb.Device_builder{
				OsType:       devicepb.OSType_OS_TYPE_MACOS,
				AssetTag:     intuneDevices[1].SerialNumber,
				EnrollStatus: devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED,
				Source:       source,
				Profile: devicepb.DeviceProfile_builder{
					ExternalId: intuneDevices[1].ID,
				}.Build(),
			}.Build(),
		})
}

func listAllDevices(ctx context.Context, t require.TestingT, devicesClient devicepb.DeviceTrustServiceClient) []*devicepb.Device {
	var devs []*devicepb.Device
	var pageToken string
	for {
		resp, err := devicesClient.ListDevices(ctx, devicepb.ListDevicesRequest_builder{
			PageToken: pageToken,
			View:      devicepb.DeviceView_DEVICE_VIEW_RESOURCE,
		}.Build())
		require.NoError(t, err)
		devs = append(devs, resp.GetDevices()...)
		if resp.GetNextPageToken() == "" {
			return devs
		}
		pageToken = resp.GetNextPageToken()
	}
}

// requireDeviceCount asserts that the device inventory currently holds num devices.
func requireDeviceCount(t *testing.T, devicesClient devicepb.DeviceTrustServiceClient, num int, msgAndArgs ...any) {
	t.Helper()
	require.Len(t, listAllDevices(t.Context(), t, devicesClient), num, msgAndArgs...)
}

func waitSynced(t *testing.T, devicesClient devicepb.DeviceTrustServiceClient, num int, msgAndArgs ...any) {
	t.Helper()
	ctx := t.Context()
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		require.Len(t, listAllDevices(ctx, t, devicesClient), num, "Unexpected number of devices")
	}, 1*time.Second, 100*time.Millisecond, msgAndArgs...)
}

func waitForDevices(t *testing.T, devicesClient devicepb.DeviceTrustServiceClient, wantDevices []*devicepb.Device) {
	t.Helper()
	const (
		waitForDeviceSyncTimeout = 5 * time.Second
		waitForDeviceSyncTick    = 100 * time.Millisecond
	)
	ctx := t.Context()

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		got := listAllDevices(ctx, t, devicesClient)
		if diff := cmp.Diff(wantDevices, got, devicesCmpOpts...); diff != "" {
			t.Errorf("Device mismatch (-want +got)\n%s", diff)
		}
	}, waitForDeviceSyncTimeout, waitForDeviceSyncTick)
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
		APIConfig:         APIConfig{AppCredentials: *testenv.DefaultApps[0]},
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
	resp, err := devicesClient.BulkCreateDevices(t.Context(), devicepb.BulkCreateDevicesRequest_builder{
		Devices: devices,
	}.Build())
	require.NoError(t, err)
	for i, s := range resp.GetDevices() {
		require.Equal(t, codes.OK, codes.Code(s.GetStatus().GetCode()), "device #%v has non-OK status: %+v", i, s)
	}
}

func TestAuthFailures(t *testing.T) {
	clock := clockwork.NewFakeClock()
	env := testenv.MustNew(t, &testenv.Config{
		Clock:          clock,
		DeviceTrustEnv: true,
	})
	statusSink := &integration.FakeStatusSink{}
	serviceConfig := &Config{
		Logger:        env.Logger,
		HTTPClient:    env.HTTPClient,
		Clock:         env.Clock,
		DevicesClient: env.DevicesClient,
		StatusSink:    statusSink,
	}

	t.Run("invalid tenant", func(t *testing.T) {
		config := *serviceConfig
		config.APIConfig = APIConfig{
			AppCredentials: api.AppCredentials{Tenant: "foo", ClientID: "invalid", ClientSecret: "not a secret"},
		}
		_, err := NewService(t.Context(), config)
		require.ErrorIs(t, err, msgraph.ErrTenantNotFound)
	})

	t.Run("invalid client ID", func(t *testing.T) {
		config := *serviceConfig
		config.APIConfig = APIConfig{
			AppCredentials: api.AppCredentials{Tenant: testenv.DefaultApps[0].Tenant, ClientID: "invalid", ClientSecret: "not a secret"},
		}
		_, err := NewService(t.Context(), config)
		require.ErrorIs(t, err, msgraph.ErrInvalidCredentials)
	})

	t.Run("invalid client secret", func(t *testing.T) {
		config := *serviceConfig
		config.APIConfig = APIConfig{
			AppCredentials: api.AppCredentials{Tenant: testenv.DefaultApps[0].Tenant, ClientID: testenv.DefaultApps[0].ClientID, ClientSecret: "not a secret"},
		}
		_, err := NewService(t.Context(), config)
		require.ErrorIs(t, err, msgraph.ErrInvalidCredentials)
	})

	t.Run("app lacking permissions", func(t *testing.T) {
		env.API.SetUnauthorizedClientIDs([]string{testenv.DefaultApps[0].ClientID})
		defer env.API.SetUnauthorizedClientIDs([]string{})

		config := *serviceConfig
		config.APIConfig = APIConfig{
			AppCredentials: *testenv.DefaultApps[0],
		}
		_, err := NewService(t.Context(), config)
		require.ErrorIs(t, err, msgraph.ErrClientUnauthorized)
	})
}
