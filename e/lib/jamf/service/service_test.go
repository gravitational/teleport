package service_test

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/testing/protocmp"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/jamf"
	jamffake "github.com/gravitational/teleport/e/lib/jamf/fake"
	jamfservice "github.com/gravitational/teleport/e/lib/jamf/service"
	"github.com/gravitational/teleport/e/lib/jamf/testenv"
	"github.com/gravitational/teleport/e/lib/mdmsync"
	"github.com/gravitational/teleport/lib/service/servicecfg"
)

var devicesCmpOpts = []cmp.Option{
	cmpopts.SortSlices(func(a, b *devicepb.Device) bool {
		return a.AssetTag < b.AssetTag
	}),
	protocmp.Transform(),
	protocmp.IgnoreFields(&devicepb.Device{}, "api_version", "id", "create_time", "update_time"),
	protocmp.IgnoreFields(&devicepb.DeviceProfile{}, "update_time"),
}

func BenchmarkSyncInventory_jamf(b *testing.B) {
	b.StopTimer() // Don't count setup/warmup.

	// Benchmark the number of devices, instead of the number of method calls, as
	// each sync operation is effectively a loop over all devices.
	numDevs := b.N
	b.Logf("Benchmarking with an inventory of %v devices", numDevs)

	// Create test environment.
	clock := clockwork.NewRealClock()
	env := testenv.MustNew(&testenv.Opts{
		Clock:          clock,
		DeviceTrustEnv: true,
	})
	defer env.Close()

	api := env.API
	ctx := context.Background()

	// Prepare a fake device inventory.
	now := clock.Now()
	t0 := now.Add(-24 * time.Hour)
	t1 := now.Add(-23 * time.Hour)
	t2 := now.Add(-22 * time.Hour)
	inv := make([]*jamf.ComputerInventory, numDevs)
	nextComputerID := 1
	for i := range inv {
		id := strconv.Itoa(nextComputerID)
		nextComputerID++
		inv[i] = &jamf.ComputerInventory{
			ID:   id,
			UDID: "u" + id,
			General: &jamf.ComputerGeneralSection{
				Name:              "d" + id,
				JamfBinaryVersion: "10.47.0-t1685028359",
				Platform:          "Mac",
				ReportDate:        t2,
				LastContactTime:   t1,
				LastEnrolledDate:  t0,
			},
			Hardware: &jamf.ComputerHardwareSection{
				ModelIdentifier: "MacBookPro16,1",
				SerialNumber:    "CX" + id,
			},
			LocalUserAccounts: []*jamf.LocalUserAccount{
				{
					UID:      "501",
					Username: "alice",
					FullName: "Alice",
				},
			},
			OperatingSystem: &jamf.ComputerOperatingSystemSection{
				Name:    "Mac OS X",
				Version: "13.4.1",
				Build:   "22F82",
			},
		}
	}

	s := serviceFromEnv(b, env, nil /* modifyOpts */)

	// Run a quick sync to "warm up".
	api.SetInventory(inv[:1])
	if _, err := s.RunOnce(ctx, jamfservice.RunSpec{
		Mode:      mdmsync.SyncModeFull,
		OnMissing: jamfservice.DeviceActionDelete,
	}); err != nil {
		b.Fatalf("Warmup RunOnce failed: %v", err)
	}

	// Benchmark.
	b.StartTimer()
	api.SetInventory(inv)
	_, err := s.RunOnce(ctx, jamfservice.RunSpec{
		Mode:      mdmsync.SyncModeFull,
		OnMissing: jamfservice.DeviceActionDelete,
	})
	if err != nil {
		b.Fatalf("RunOnce failed: %v", err)
	}
}

func TestNew_errors(t *testing.T) {
	t.Parallel()
	env := testenv.NewUsingT(t, &testenv.Opts{
		DeviceTrustEnv: true,
	})

	ctx := context.Background()
	baseOpts := jamfservice.Opts{
		Clock:  env.Clock,
		Logger: env.Logger,
		Config: &servicecfg.JamfConfig{
			Spec: &types.JamfSpecV1{
				Enabled:     true,
				ApiEndpoint: env.APIEndpoint,
			},
			Credentials: &servicecfg.JamfCredentials{
				Username: testenv.DefaultUsers[0].Username,
				Password: testenv.DefaultUsers[0].Password,
			},
		},
		DevicesClient: env.DevicesClient,
		HTTPClient:    env.HTTPClient,
	}

	canceled, cancel := context.WithCancel(ctx)
	cancel()

	tests := []struct {
		name       string
		ctx        context.Context
		createOpts func() jamfservice.Opts
		wantErr    string
	}{
		{
			name:       "canceled context",
			ctx:        canceled,
			createOpts: func() jamfservice.Opts { return baseOpts },
			wantErr:    context.Canceled.Error(),
		},
		{
			name: "spec nil",
			ctx:  ctx,
			createOpts: func() jamfservice.Opts {
				opts := baseOpts
				opts.Config = &servicecfg.JamfConfig{}
				return opts
			},
			wantErr: "configuration required",
		},
		{
			name: "spec is validated",
			ctx:  ctx,
			createOpts: func() jamfservice.Opts {
				opts := baseOpts
				opts.Config = &servicecfg.JamfConfig{
					Spec: &types.JamfSpecV1{
						Enabled: true,
					},
					Credentials: &servicecfg.JamfCredentials{
						Username: baseOpts.Config.Credentials.Username,
						Password: baseOpts.Config.Credentials.Password,
					},
				}
				return opts
			},
			wantErr: "API endpoint",
		},
		{
			name: "empty schedule",
			ctx:  ctx,
			createOpts: func() jamfservice.Opts {
				opts := baseOpts
				opts.Config = &servicecfg.JamfConfig{
					Spec: &types.JamfSpecV1{
						Enabled:     true,
						ApiEndpoint: baseOpts.Config.Spec.ApiEndpoint,
						Inventory: []*types.JamfInventoryEntry{
							{
								SyncPeriodPartial: -1, // disabled
								SyncPeriodFull:    -1, // disabled
							},
						},
					},
					Credentials: &servicecfg.JamfCredentials{
						Username: baseOpts.Config.Credentials.Username,
						Password: baseOpts.Config.Credentials.Password,
					},
				}
				return opts
			},
			wantErr: "no active entries",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := jamfservice.New(test.ctx, test.createOpts())
			assert.ErrorContains(t, err, test.wantErr, "New() error mismatch")
		})
	}
}

func TestS_Run_stopsOnCancel(t *testing.T) {
	t.Parallel()
	env := testenv.NewUsingT(t, &testenv.Opts{
		DeviceTrustEnv: true,
	})

	s := serviceFromEnv(t, env, func(opts *jamfservice.Opts) {
		// Don't trigger a sync.
		opts.Config.Spec.SyncDelay = types.DurationStringForJamfSpecV1(1 * time.Hour)
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	started := make(chan struct{})
	exited := make(chan error)
	go func() {
		started <- struct{}{}
		exited <- s.Run(ctx)
	}()

	// Assert, to some extent, that the service started running.
	<-started
	select {
	case <-exited:
		t.Fatalf("Service exited before context cancellation")
	default:
		// OK, expected
	}

	// Service should stop on cancel
	cancel()
	if err := <-exited; !errors.Is(err, context.Canceled) {
		t.Errorf("Service exited with err=%q, wanted context.Canceled", err)
	}
}

// TestS_Run_syncDefaults runs the Jamf Service until a batch of devices is
// synced, using as many default service settings as possible.
func TestS_Run_syncDefaults(t *testing.T) {
	t.Parallel()
	clock := clockwork.NewFakeClockAt(time.Date(2023, 1, 2, 3, 4, 5, 0, time.UTC))
	env := testenv.NewUsingT(t, &testenv.Opts{
		Clock:          clock,
		DeviceTrustEnv: true,
	})

	api := env.API
	devicesClient := env.DevicesClient
	ctx := context.Background()

	t0 := clock.Now()
	clock.Advance(1 * time.Minute)
	t1 := clock.Now()
	clock.Advance(24 * time.Hour)
	t2 := clock.Now()
	clock.Advance(12 * time.Hour)
	// now > t2

	jamfDevs := []*jamf.ComputerInventory{
		// "Complete" device. This is a more realistic representation.
		{
			ID:   "1",
			UDID: "11",
			General: &jamf.ComputerGeneralSection{
				Name:              "llama's device",
				JamfBinaryVersion: "9.27",
				Platform:          "Mac",
				ReportDate:        t0,
				LastContactTime:   t1,
				LastEnrolledDate:  t0,
			},
			Hardware: &jamf.ComputerHardwareSection{
				ModelIdentifier: "MacBookPro9,2",
				SerialNumber:    "CXXXXXXXXX01",
			},
			LocalUserAccounts: []*jamf.LocalUserAccount{
				{
					UID:      "501",
					Username: "llama",
					FullName: "Llama",
				},
				{
					UID:      "502",
					Username: "admin",
					FullName: "admin",
				},
			},
			OperatingSystem: &jamf.ComputerOperatingSystemSection{
				Name:                     "Mac OS X",
				Version:                  "13.4.1",
				Build:                    "22F82",
				SupplementalBuildVersion: "22F770820d",
				RapidSecurityResponse:    "(c)",
			},
		},
		// "Minimal" device #1. Unexpected, but carries enough info to be created.
		{
			ID:   "2",
			UDID: "22",
			General: &jamf.ComputerGeneralSection{
				Platform:         "Mac",
				ReportDate:       t0,
				LastContactTime:  t2,
				LastEnrolledDate: t1,
			},
			Hardware: &jamf.ComputerHardwareSection{
				SerialNumber: "CXXXXXXXXX02",
			},
		},
		// "Minimal" device #2.
		{
			ID:   "3",
			UDID: "33",
			General: &jamf.ComputerGeneralSection{
				Platform:         "Mac",
				ReportDate:       t0,
				LastContactTime:  t2,
				LastEnrolledDate: t1,
			},
			Hardware: &jamf.ComputerHardwareSection{
				SerialNumber: "CXXXXXXXXX03",
			},
		},
		// "Minimal" with empty/nil accounts.
		{
			ID:   "4",
			UDID: "44",
			General: &jamf.ComputerGeneralSection{
				Platform:         "Mac",
				ReportDate:       t0,
				LastContactTime:  t2,
				LastEnrolledDate: t1,
			},
			Hardware: &jamf.ComputerHardwareSection{
				SerialNumber: "CXXXXXXXXX04",
			},
			LocalUserAccounts: []*jamf.LocalUserAccount{
				nil,
				{},
			},
		},
		// Invalid - doesn't have a platform.
		{
			ID:   "invalid1",
			UDID: "i11",
			General: &jamf.ComputerGeneralSection{
				ReportDate:       t0,
				LastContactTime:  t2,
				LastEnrolledDate: t1,
			},
			Hardware: &jamf.ComputerHardwareSection{
				SerialNumber: "invalid1",
			},
		},
		// Invalid - doesn't have a serial number.
		{
			ID:   "invalid2",
			UDID: "i22",
			General: &jamf.ComputerGeneralSection{
				Platform:         "Mac",
				ReportDate:       t0,
				LastContactTime:  t2,
				LastEnrolledDate: t1,
			},
		},
		// Invalid - General is nil.
		{
			ID:      "i33",
			UDID:    "i33",
			General: nil,
			Hardware: &jamf.ComputerHardwareSection{
				SerialNumber: "invalid2",
			},
		},
		// Invalid - Hardware is nil.
		{
			ID:   "i4",
			UDID: "i44",
			General: &jamf.ComputerGeneralSection{
				Platform:         "Mac",
				ReportDate:       t0,
				LastContactTime:  t2,
				LastEnrolledDate: t1,
			},
		},
		// Invalid - everything is empty or nil.
		{},
		// Invalid - it _is_ nil.
		nil,
		// Invalid - serial number too large.
		{
			ID:   "i5",
			UDID: "i55",
			General: &jamf.ComputerGeneralSection{
				Platform:         "Mac",
				ReportDate:       t0,
				LastContactTime:  t2,
				LastEnrolledDate: t1,
			},
			Hardware: &jamf.ComputerHardwareSection{
				SerialNumber: strings.Repeat("x", 121),
			},
		},
	}
	api.SetInventory(jamfDevs)

	source := &devicepb.DeviceSource{
		Name:   "jamf",
		Origin: devicepb.DeviceOrigin_DEVICE_ORIGIN_JAMF,
	}
	wantDevs := []*devicepb.Device{
		{
			OsType:       devicepb.OSType_OS_TYPE_MACOS,
			AssetTag:     jamfDevs[0].Hardware.SerialNumber,
			EnrollStatus: devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED,
			Source:       source,
			Profile: &devicepb.DeviceProfile{
				ModelIdentifier:     jamfDevs[0].Hardware.ModelIdentifier,
				OsVersion:           jamfDevs[0].OperatingSystem.Version,
				OsBuild:             jamfDevs[0].OperatingSystem.Build,
				OsBuildSupplemental: jamfDevs[0].OperatingSystem.SupplementalBuildVersion,
				OsUsernames: []string{
					jamfDevs[0].LocalUserAccounts[0].Username,
					jamfDevs[0].LocalUserAccounts[1].Username,
				},
				JamfBinaryVersion: jamfDevs[0].General.JamfBinaryVersion,
				ExternalId:        jamfDevs[0].ID,
			},
		},
		deviceFromMinimal(jamfDevs[1], source),
		deviceFromMinimal(jamfDevs[2], source),
		deviceFromMinimal(jamfDevs[3], source),
	}

	s := serviceFromEnv(t, env, nil /* modifyOpts */)

	// Let the sync run in the background, until we see the devices in Teleport.
	runCtx, runCancel := context.WithCancel(ctx)
	defer runCancel()
	go func() {
		if err := s.Run(runCtx); err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("Run returned an unexpected error: %v", err)
		}
	}()

	// Wait for devices to be synced.
	require.Eventually(t, func() bool {
		got := listAllDevices(t, devicesClient)
		return len(got) >= len(wantDevs)
	}, 1*time.Minute, 100*time.Millisecond)
	runCancel() // We can stop the service now.

	// Assert devices.
	got := listAllDevices(t, devicesClient)
	if diff := cmp.Diff(wantDevs, got, devicesCmpOpts...); diff != "" {
		t.Errorf("Run sync mismatch (-want +got)\n%s", diff)
	}
}

func TestS_Run_fullWithDeletions(t *testing.T) {
	t.Parallel()
	clock := clockwork.NewRealClock()
	env := testenv.NewUsingT(t, &testenv.Opts{
		Clock:          clock,
		DeviceTrustEnv: true,
	})

	t0 := clock.Now()
	t1 := clock.Now()

	api := env.API
	devicesClient := env.DevicesClient
	ctx := context.Background()

	jamfDevs := []*jamf.ComputerInventory{
		{
			ID:   "1",
			UDID: "1",
			General: &jamf.ComputerGeneralSection{
				Name:             "dev1",
				Platform:         "Mac",
				ReportDate:       t0,
				LastContactTime:  t1,
				LastEnrolledDate: t0,
			},
			Hardware: &jamf.ComputerHardwareSection{
				SerialNumber: "dev1",
			},
		},
		{
			ID:   "2",
			UDID: "2",
			General: &jamf.ComputerGeneralSection{
				Name:             "dev2",
				Platform:         "Mac",
				ReportDate:       t0,
				LastContactTime:  t1,
				LastEnrolledDate: t0,
			},
			Hardware: &jamf.ComputerHardwareSection{
				SerialNumber: "dev2",
			},
		},
		{
			ID:   "3",
			UDID: "3",
			General: &jamf.ComputerGeneralSection{
				Name:             "dev3",
				Platform:         "Mac",
				ReportDate:       t0,
				LastContactTime:  t1,
				LastEnrolledDate: t0,
			},
			Hardware: &jamf.ComputerHardwareSection{
				SerialNumber: "dev3",
			},
		},
	}
	api.SetInventory(jamfDevs)

	// Mobile devices.
	jamfMobileDevs := []*jamf.MobileDevice{
		{
			MobileDeviceID: "10",
			DeviceType:     "iOS",
			General: &jamf.MobileDeviceGeneralSection{
				LastInventoryUpdateDate: t0,
			},
			Hardware: &jamf.MobileDeviceHardwareSection{
				SerialNumber:    "mdev1",
				ModelIdentifier: "iPad15,7",
			},
		},
		{
			MobileDeviceID: "11",
			DeviceType:     "iOS",
			General: &jamf.MobileDeviceGeneralSection{
				LastInventoryUpdateDate: t0,
			},
			Hardware: &jamf.MobileDeviceHardwareSection{
				SerialNumber:    "mdev2",
				ModelIdentifier: "iPhone15,2",
			},
		},
	}
	api.SetMobileDeviceInventory(jamfMobileDevs)

	source := &devicepb.DeviceSource{
		Name:   "jamf",
		Origin: devicepb.DeviceOrigin_DEVICE_ORIGIN_JAMF,
	}
	s := serviceFromEnv(t, env, func(opts *jamfservice.Opts) {
		opts.Config.Spec.Name = source.Name
		opts.Config.Spec.Inventory = []*types.JamfInventoryEntry{
			{
				DeviceType:        types.JamfDeviceTypeComputers,
				SyncPeriodPartial: -1, // disabled
				SyncPeriodFull:    types.DurationStringForJamfSpecV1(100 * time.Millisecond),
				OnMissing:         "DELETE",
			},
			{
				DeviceType:        types.JamfDeviceTypeMobileDevices,
				SyncPeriodPartial: -1, // disabled
				SyncPeriodFull:    types.DurationStringForJamfSpecV1(100 * time.Millisecond),
				OnMissing:         "DELETE",
			},
		}
	})

	// Add devices to Teleport that have no match in Jamf.
	// These get removed in the first sync.
	if resp, err := devicesClient.BulkCreateDevices(ctx, &devicepb.BulkCreateDevicesRequest{
		Devices: []*devicepb.Device{
			// Computer: missing external_id.
			{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "deleteonsync1",
				Source:   source,
			},
			// Computer: mismatched Jamf ID.
			{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "deleteonsync2",
				Source:   source,
				Profile: &devicepb.DeviceProfile{
					ExternalId: jamfDevs[1].ID,
				},
			},
			// Mobile device: missing external_id.
			{
				OsType:   devicepb.OSType_OS_TYPE_IPADOS,
				AssetTag: "deleteonsync3",
				Source:   source,
			},
			// Mobile device: mismatched Jamf ID.
			{
				OsType:   devicepb.OSType_OS_TYPE_IOS,
				AssetTag: "deleteonsync4",
				Source:   source,
				Profile: &devicepb.DeviceProfile{
					ExternalId: jamfMobileDevs[1].MobileDeviceID,
				},
			},
		},
	}); err != nil {
		t.Fatalf("BulkCreateDevices failed: %v", err)
	} else {
		for i, s := range resp.Devices {
			if codes.Code(s.GetStatus().GetCode()) != codes.OK {
				t.Fatalf("BulkCreateDevices: device #%v has non-OK status: %+v", i, s)
			}
		}
	}

	// Run in the background.
	runCtx, runCancel := context.WithCancel(ctx)
	defer runCancel()
	go func() {
		if err := s.Run(runCtx); err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("Run returned an unexpected error: %v", err)
		}
	}()

	waitSynced := func(t *testing.T, num int) {
		t.Helper()

		require.Eventually(t, func() bool {
			return len(listAllDevices(t, devicesClient)) == num
		}, 1*time.Second, 100*time.Millisecond)
	}

	// Use the first sync to write the full list of devices (computers + mobile).
	totalDevs := len(jamfDevs) + len(jamfMobileDevs)
	waitSynced(t, totalDevs)

	// Remove a computer and a mobile device from Jamf.
	api.SetInventory(jamfDevs[1:])
	api.SetMobileDeviceInventory(jamfMobileDevs[1:])
	waitSynced(t, totalDevs-2)
	runCancel() // Stop syncs.

	// Verify deletions.
	got := listAllDevices(t, devicesClient)
	want := []*devicepb.Device{
		deviceFromMinimal(jamfDevs[1], nil /* source */),
		deviceFromMinimal(jamfDevs[2], nil /* source */),
		mobileDeviceFromMinimal(jamfMobileDevs[1], nil /* source */),
	}
	opts := append(devicesCmpOpts, protocmp.IgnoreFields(&devicepb.Device{}, "source"))
	if diff := cmp.Diff(want, got, opts...); diff != "" {
		t.Errorf("Run sync mismatch (-want +got)\n%s", diff)
	}
}

func TestS_Run_exitOnSync(t *testing.T) {
	t.Parallel()
	clock := clockwork.NewRealClock()
	env := testenv.NewUsingT(t, &testenv.Opts{
		Clock:          clock,
		DeviceTrustEnv: true,
	})

	api := env.API
	devicesClient := env.DevicesClient

	// Set a couple of devices. Specifics don't matter much.
	t0 := clock.Now()
	api.SetInventory([]*jamf.ComputerInventory{
		{
			ID:   "1",
			UDID: "11",
			General: &jamf.ComputerGeneralSection{
				Platform:         "Mac",
				ReportDate:       t0,
				LastContactTime:  t0,
				LastEnrolledDate: t0,
			},
			Hardware: &jamf.ComputerHardwareSection{
				SerialNumber: "llama1",
			},
		},
		{
			ID:   "2",
			UDID: "22",
			General: &jamf.ComputerGeneralSection{
				Platform:         "Mac",
				ReportDate:       t0,
				LastContactTime:  t0,
				LastEnrolledDate: t0,
			},
			Hardware: &jamf.ComputerHardwareSection{
				SerialNumber: "llama2",
			},
		},
	})
	const wantDevices = 2

	s := serviceFromEnv(t, env, func(opts *jamfservice.Opts) {
		opts.Config.ExitOnSync = true
	})

	// Run should exit after the first round of sync.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.Run(ctx); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if got := len(listAllDevices(t, devicesClient)); got != wantDevices {
		t.Errorf("Run synced %v devices, want %v", got, wantDevices)
	}
}

// TestS_Run_clientCredentials tests a sync using API client credentials instead
// of username+password.
func TestS_Run_clientCredentials(t *testing.T) {
	t.Parallel()
	clock := clockwork.NewRealClock()
	env := testenv.NewUsingT(t, &testenv.Opts{
		Clock:          clock,
		DeviceTrustEnv: true,
	})

	api := env.API
	devicesClient := env.DevicesClient

	// Configure API to use client credentials.
	const clientID = "llama-UUID"
	const clientSecret = "supersecretsecret!!1!"
	api.SetAPIClients([]*jamffake.APIClient{
		{ID: clientID, Secret: clientSecret},
	})

	// Create one device to sync. Specifics don't matter.
	t0 := clock.Now()
	api.SetInventory([]*jamf.ComputerInventory{
		{
			ID:   "1",
			UDID: "11",
			General: &jamf.ComputerGeneralSection{
				Platform:         "Mac",
				ReportDate:       t0,
				LastContactTime:  t0,
				LastEnrolledDate: t0,
			},
			Hardware: &jamf.ComputerHardwareSection{
				SerialNumber: "llama1",
			},
		},
	})

	s := serviceFromEnv(t, env, func(opts *jamfservice.Opts) {
		// Use API credentials instead of username+password.
		creds := opts.Config.Credentials
		creds.Username = ""
		creds.Password = ""
		creds.ClientID = clientID
		creds.ClientSecret = clientSecret

		// Stop after first sync.
		opts.Config.ExitOnSync = true
	})

	// Sync!
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.Run(ctx); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if devs := listAllDevices(t, devicesClient); len(devs) != 1 {
		t.Errorf("Run synced %d devices, want 1", len(devs))
	}
}

func TestS_RunOnce_partialSync(t *testing.T) {
	t.Parallel()
	clock := clockwork.NewFakeClock()
	env := testenv.NewUsingT(t, &testenv.Opts{
		Clock:          clock,
		DeviceTrustEnv: true,
	})

	api := env.API
	devicesClient := env.DevicesClient

	advanceNow := func() time.Time {
		clock.Advance(1 * time.Minute)
		return clock.Now()
	}

	// Create a few devices with varying timestamps.
	// We'll sync in the reverse order, new-to-old, to demonstrate that partial
	// cuts work.
	t0 := clock.Now()
	t1 := advanceNow()
	t2 := advanceNow()
	t3 := advanceNow()
	t4 := advanceNow()
	t5 := advanceNow()
	now := advanceNow()

	source := &devicepb.DeviceSource{
		Name:   "jamf2",
		Origin: devicepb.DeviceOrigin_DEVICE_ORIGIN_JAMF,
	}

	cases := []struct {
		deviceType string
		// setup populates the Jamf API for the case and returns the expected
		// Teleport devices in t5..t1 order (highest timestamp first).
		setup func() []*devicepb.Device
	}{
		{
			deviceType: types.JamfDeviceTypeComputers,
			setup: func() []*devicepb.Device {
				currentID := 0
				newJamfDev := func(reportDate time.Time) *jamf.ComputerInventory {
					currentID++
					return &jamf.ComputerInventory{
						ID:   strconv.Itoa(currentID),
						UDID: strconv.Itoa(currentID),
						General: &jamf.ComputerGeneralSection{
							Platform:         "Mac",
							ReportDate:       reportDate,
							LastContactTime:  t5,
							LastEnrolledDate: t0,
						},
						Hardware: &jamf.ComputerHardwareSection{
							SerialNumber: fmt.Sprintf("C%011d", currentID),
						},
					}
				}
				jamfDevs := []*jamf.ComputerInventory{
					newJamfDev(t5),
					newJamfDev(t4),
					newJamfDev(t3),
					newJamfDev(t2),
					newJamfDev(t1),
				}
				api.SetInventory(jamfDevs)
				devs := make([]*devicepb.Device, len(jamfDevs))
				for i, j := range jamfDevs {
					devs[i] = deviceFromMinimal(j, source)
				}
				return devs
			},
		},
		{
			deviceType: types.JamfDeviceTypeMobileDevices,
			setup: func() []*devicepb.Device {
				currentID := 100
				modelIDs := []string{"iPad15,7", "iPhone15,2", "iPad15,7", "iPhone15,2", "iPad15,7"}
				newMobileDev := func(lastUpdate time.Time, modelID string) *jamf.MobileDevice {
					currentID++
					return &jamf.MobileDevice{
						MobileDeviceID: strconv.Itoa(currentID),
						DeviceType:     "iOS",
						General: &jamf.MobileDeviceGeneralSection{
							LastInventoryUpdateDate: lastUpdate,
						},
						Hardware: &jamf.MobileDeviceHardwareSection{
							SerialNumber:    fmt.Sprintf("M%011d", currentID),
							ModelIdentifier: modelID,
						},
					}
				}
				jamfDevs := []*jamf.MobileDevice{
					newMobileDev(t5, modelIDs[0]),
					newMobileDev(t4, modelIDs[1]),
					newMobileDev(t3, modelIDs[2]),
					newMobileDev(t2, modelIDs[3]),
					newMobileDev(t1, modelIDs[4]),
				}
				api.SetMobileDeviceInventory(jamfDevs)
				devs := make([]*devicepb.Device, len(jamfDevs))
				for i, md := range jamfDevs {
					devs[i] = mobileDeviceFromMinimal(md, source)
				}
				return devs
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.deviceType, func(t *testing.T) {
			t.Cleanup(func() { clearDevices(t, devicesClient) })
			allDevs := tc.setup()

			s := serviceFromEnv(t, env, func(opts *jamfservice.Opts) {
				opts.Config.Spec.Name = source.Name
			})
			ctx := t.Context()

			// Sync with a too-high cut time to begin with.
			t.Run("t=now", func(t *testing.T) {
				gotTime, err := s.RunOnce(ctx, jamfservice.RunSpec{
					Mode:       mdmsync.SyncModePartial,
					DeviceType: tc.deviceType,
					CutTime:    now,
				})
				if err != nil {
					t.Fatalf("RunOnce failed: %v", err)
				}
				if !gotTime.IsZero() {
					t.Errorf("RunOnce returned non-zero time on an empty sync: %v", gotTime)
				}
				if got := len(listAllDevices(t, devicesClient)); got > 0 {
					t.Errorf("RunOnce synced %v devices, wanted none", got)
				}
			})

			// Sync new-to-old.
			for i, cutTime := range []time.Time{t5, t4, t3, t2, t1} {
				wantNextTime := t5 // Always the highest seen.
				wantDevs := allDevs[:i+1]

				t.Run(fmt.Sprintf("t=%v", cutTime), func(t *testing.T) {
					gotTime, err := s.RunOnce(ctx, jamfservice.RunSpec{
						Mode:       mdmsync.SyncModePartial,
						DeviceType: tc.deviceType,
						CutTime:    cutTime,
					})
					if err != nil {
						t.Fatalf("RunOnce failed: %v", err)
					}

					if !gotTime.Equal(wantNextTime) {
						t.Errorf("RunOnce = %v, want %v", gotTime, wantNextTime)
					}

					got := listAllDevices(t, devicesClient)
					if diff := cmp.Diff(wantDevs, got, devicesCmpOpts...); diff != "" {
						t.Errorf("RunOnce sync mismatch (-want +got)\n%s", diff)
					}
				})
			}
		})
	}
}

func TestS_RunOnce_pagingGaps(t *testing.T) {
	t.Parallel()
	env := testenv.NewUsingT(t, &testenv.Opts{
		DeviceTrustEnv: true,
	})

	api := env.API
	devicesClient := env.DevicesClient
	ctx := t.Context()

	// setup populates the Jamf API for the case and returns the expected
	// Teleport devices.
	cases := []struct {
		deviceType string
		setup      func() []*devicepb.Device
	}{
		{
			deviceType: types.JamfDeviceTypeComputers,
			setup: func() []*devicepb.Device {
				jamfDevs := []*jamf.ComputerInventory{
					{
						ID:       "1",
						General:  &jamf.ComputerGeneralSection{Platform: "Mac"},
						Hardware: &jamf.ComputerHardwareSection{SerialNumber: "dev1"},
					},
					{
						ID:       "2",
						General:  &jamf.ComputerGeneralSection{Platform: "Mac"},
						Hardware: &jamf.ComputerHardwareSection{SerialNumber: "dev2"},
					},
					{
						ID:       "3",
						General:  &jamf.ComputerGeneralSection{Platform: "Mac"},
						Hardware: &jamf.ComputerHardwareSection{SerialNumber: "dev3"},
					},
				}
				api.SetInventory(jamfDevs)
				devs := make([]*devicepb.Device, len(jamfDevs))
				for i, j := range jamfDevs {
					devs[i] = deviceFromMinimal(j, nil /* source */)
				}
				return devs
			},
		},
		{
			deviceType: types.JamfDeviceTypeMobileDevices,
			setup: func() []*devicepb.Device {
				jamfDevs := []*jamf.MobileDevice{
					{
						MobileDeviceID: "1",
						DeviceType:     "iOS",
						General:        &jamf.MobileDeviceGeneralSection{},
						Hardware: &jamf.MobileDeviceHardwareSection{
							SerialNumber:    "mdev1",
							ModelIdentifier: "iPad15,7",
						},
					},
					{
						MobileDeviceID: "2",
						DeviceType:     "iOS",
						General:        &jamf.MobileDeviceGeneralSection{},
						Hardware: &jamf.MobileDeviceHardwareSection{
							SerialNumber:    "mdev2",
							ModelIdentifier: "iPhone15,2",
						},
					},
					{
						MobileDeviceID: "3",
						DeviceType:     "iOS",
						General:        &jamf.MobileDeviceGeneralSection{},
						Hardware: &jamf.MobileDeviceHardwareSection{
							SerialNumber:    "mdev3",
							ModelIdentifier: "iPad15,7",
						},
					},
				}
				api.SetMobileDeviceInventory(jamfDevs)
				devs := make([]*devicepb.Device, len(jamfDevs))
				for i, md := range jamfDevs {
					devs[i] = mobileDeviceFromMinimal(md, nil /* source */)
				}
				return devs
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.deviceType, func(t *testing.T) {
			t.Cleanup(func() {
				// clearDevices needs ListDevices to work correctly, so toggle paging
				// gap simulation back to false.
				api.SetSimulatePagingGaps(false)
				clearDevices(t, devicesClient)
			})
			allDevs := tc.setup()

			s := serviceFromEnv(t, env, func(opts *jamfservice.Opts) {
				opts.Config.Spec.Inventory = []*types.JamfInventoryEntry{
					{
						DeviceType:        tc.deviceType,
						SyncPeriodPartial: -1, // disabled
						SyncPeriodFull:    types.DurationStringForJamfSpecV1(100 * time.Millisecond),
						OnMissing:         "DELETE",
					},
				}
			})

			assertStored := func(t *testing.T, want []*devicepb.Device) {
				got := listAllDevices(t, devicesClient)
				opts := append(devicesCmpOpts, protocmp.IgnoreFields(&devicepb.Device{}, "source"))
				if diff := cmp.Diff(want, got, opts...); diff != "" {
					t.Fatalf("RunOnce sync mismatch (-want +got)\n%s", diff)
				}
			}

			// Sync full inventory to Teleport.
			if _, err := s.RunOnce(ctx, jamfservice.RunSpec{
				Mode:       mdmsync.SyncModeFull,
				OnMissing:  jamfservice.DeviceActionDelete,
				DeviceType: tc.deviceType,
			}); err != nil {
				t.Fatalf("RunOnce failed: %v", err)
			}

			// Sanity check: all devices synced.
			assertStored(t, allDevs)

			// Sync with paging gaps.
			// We expect no deletions to happen in Teleport.
			api.SetSimulatePagingGaps(true)
			lookupsBefore := api.SingleDeviceLookups()
			if _, err := s.RunOnce(ctx, jamfservice.RunSpec{
				Mode:       mdmsync.SyncModeFull,
				OnMissing:  jamfservice.DeviceActionDelete,
				DeviceType: tc.deviceType,
			}); err != nil {
				t.Fatalf("RunOnce failed: %v", err)
			}

			// Sanity check the test setup: at least one missing device must
			// have been confirmed via a per-device lookup. Otherwise the
			// assertion below could pass trivially even if paging-gap
			// simulation was broken.
			if gained := api.SingleDeviceLookups() - lookupsBefore; gained == 0 {
				t.Fatal("Paging-gap protection was not exercised: no per-device lookups during the sync with a gap")
			}

			// Verify no deletions in Teleport.
			assertStored(t, allDevs)
		})
	}
}

func TestS_RunOnce_mobileDeviceSync(t *testing.T) {
	t.Parallel()
	env := testenv.NewUsingT(t, &testenv.Opts{
		DeviceTrustEnv: true,
	})

	api := env.API
	devicesClient := env.DevicesClient

	t0 := time.Unix(1685469902, 0) // 2023-05-30T18:05:02+00:00

	jamfMobileDevs := []*jamf.MobileDevice{
		// Complete iPad.
		{
			MobileDeviceID: "1",
			DeviceType:     "iOS",
			General: &jamf.MobileDeviceGeneralSection{
				OSVersion:                  "26.3.1",
				OSBuild:                    "23D8133",
				OSSupplementalBuildVersion: "23D771330a",
				LastInventoryUpdateDate:    t0,
				LastEnrolledDate:           t0,
			},
			Hardware: &jamf.MobileDeviceHardwareSection{
				SerialNumber:    "CXXXXXXXXX20",
				ModelIdentifier: "iPad15,7",
			},
		},
		// Minimal iPhone.
		{
			MobileDeviceID: "2",
			DeviceType:     "iOS",
			General: &jamf.MobileDeviceGeneralSection{
				LastInventoryUpdateDate: t0,
			},
			Hardware: &jamf.MobileDeviceHardwareSection{
				SerialNumber:    "CXXXXXXXXX21",
				ModelIdentifier: "iPhone15,2",
			},
		},
		// Invalid - tvOS device (unsupported).
		{
			MobileDeviceID: "3",
			DeviceType:     "tvOS",
			General: &jamf.MobileDeviceGeneralSection{
				LastInventoryUpdateDate: t0,
			},
			Hardware: &jamf.MobileDeviceHardwareSection{
				SerialNumber:    "CXXXXXXXXX22",
				ModelIdentifier: "AppleTV11,1",
			},
		},
		// Invalid - nil Hardware.
		{
			MobileDeviceID: "4",
			DeviceType:     "iOS",
			General: &jamf.MobileDeviceGeneralSection{
				LastInventoryUpdateDate: t0,
			},
		},
		// Invalid - iOS with unknown model.
		{
			MobileDeviceID: "5",
			DeviceType:     "iOS",
			General: &jamf.MobileDeviceGeneralSection{
				LastInventoryUpdateDate: t0,
			},
			Hardware: &jamf.MobileDeviceHardwareSection{
				SerialNumber:    "CXXXXXXXXX23",
				ModelIdentifier: "AppleTV11,1",
			},
		},
		// Invalid - everything is empty or nil.
		{},
		// Invalid - it _is_ nil.
		nil,
	}
	api.SetMobileDeviceInventory(jamfMobileDevs)

	source := &devicepb.DeviceSource{
		Name:   "jamf",
		Origin: devicepb.DeviceOrigin_DEVICE_ORIGIN_JAMF,
	}
	wantDevs := []*devicepb.Device{
		{
			OsType:       devicepb.OSType_OS_TYPE_IPADOS,
			AssetTag:     "CXXXXXXXXX20",
			EnrollStatus: devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED,
			Source:       source,
			Profile: &devicepb.DeviceProfile{
				ModelIdentifier:     "iPad15,7",
				ExternalId:          "1",
				OsVersion:           "26.3.1",
				OsBuild:             "23D8133",
				OsBuildSupplemental: "23D771330a",
			},
		},
		mobileDeviceFromMinimal(jamfMobileDevs[1], source),
	}

	s := serviceFromEnv(t, env, func(opts *jamfservice.Opts) {
		opts.Config.Spec.Name = source.Name
	})

	if _, err := s.RunOnce(t.Context(), jamfservice.RunSpec{
		Mode:       mdmsync.SyncModeFull,
		OnMissing:  jamfservice.DeviceActionDelete,
		DeviceType: types.JamfDeviceTypeMobileDevices,
	}); err != nil {
		t.Fatalf("RunOnce failed: %v", err)
	}

	got := listAllDevices(t, devicesClient)
	if diff := cmp.Diff(wantDevs, got, devicesCmpOpts...); diff != "" {
		t.Errorf("RunOnce sync mismatch (-want +got)\n%s", diff)
	}
}

func TestS_RunOnce_deviceTypeFiltering(t *testing.T) {
	t.Parallel()
	env := testenv.NewUsingT(t, &testenv.Opts{
		DeviceTrustEnv: true,
	})

	api := env.API
	devicesClient := env.DevicesClient

	// Both computers and mobile devices exist in Jamf.
	api.SetInventory([]*jamf.ComputerInventory{
		{
			ID:       "1",
			General:  &jamf.ComputerGeneralSection{Platform: "Mac"},
			Hardware: &jamf.ComputerHardwareSection{SerialNumber: "CXXXXXXXXX01"},
		},
	})
	api.SetMobileDeviceInventory([]*jamf.MobileDevice{
		{
			MobileDeviceID: "1",
			DeviceType:     "iOS",
			General:        &jamf.MobileDeviceGeneralSection{},
			Hardware: &jamf.MobileDeviceHardwareSection{
				SerialNumber:    "CXXXXXXXXX10",
				ModelIdentifier: "iPad15,7",
			},
		},
	})

	tests := []struct {
		name          string
		deviceType    string
		wantComputers int
		wantMobile    int
		wantErr       require.ErrorAssertionFunc
	}{
		{
			name:          "empty DeviceType defaults to computers",
			deviceType:    "",
			wantComputers: 1,
			wantMobile:    0,
		},
		{
			name:          "computers",
			deviceType:    types.JamfDeviceTypeComputers,
			wantComputers: 1,
			wantMobile:    0,
		},
		{
			name:          "mobile_devices",
			deviceType:    types.JamfDeviceTypeMobileDevices,
			wantComputers: 0,
			wantMobile:    1,
		},
		{
			name:       "unknown device type",
			deviceType: "unknown",
			wantErr: func(tt require.TestingT, err error, i ...any) {
				assert.True(tt, trace.IsBadParameter(err))
				assert.ErrorContains(tt, err, "unknown device type")
				assert.ErrorContains(tt, err, `"unknown"`)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Cleanup(func() { clearDevices(t, devicesClient) })

			s := serviceFromEnv(t, env, nil /* modifyOpts */)
			_, err := s.RunOnce(t.Context(), jamfservice.RunSpec{
				Mode:       mdmsync.SyncModeFull,
				DeviceType: test.deviceType,
			})
			if test.wantErr != nil {
				test.wantErr(t, err)
				return
			}
			require.NoError(t, err)

			var gotComputers, gotMobile int
			for _, d := range listAllDevices(t, devicesClient) {
				switch d.OsType {
				case devicepb.OSType_OS_TYPE_MACOS:
					gotComputers++
				case devicepb.OSType_OS_TYPE_IOS, devicepb.OSType_OS_TYPE_IPADOS:
					gotMobile++
				}
			}
			assert.Equal(t, test.wantComputers, gotComputers, "computers count")
			assert.Equal(t, test.wantMobile, gotMobile, "mobile devices count")
		})
	}
}

// TestS_RunOnce_continuesPastUnconvertiblePage is a regression test for a
// previous bug where readInventory would terminate the sync when a page
// produced zero Teleport devices. mobileDeviceToDevice intentionally drops
// non-iOS records (tvOS), so a page that contains only unconvertible devices
// yields zero upserts even though later pages may have valid iOS devices. The
// sync must keep paging until Jamf reports the inventory is exhausted.
func TestS_RunOnce_continuesPastUnconvertiblePage(t *testing.T) {
	t.Parallel()
	env := testenv.NewUsingT(t, &testenv.Opts{
		DeviceTrustEnv: true,
	})

	api := env.API
	devicesClient := env.DevicesClient

	// The service requests sort by mobileDeviceId:asc. Place the unconvertible
	// devices (tvOS) on the first page and the valid iOS devices on the second
	// page. With PageSize=2 this guarantees the first page yields zero Teleport
	// devices.
	jamfDevs := []*jamf.MobileDevice{
		{
			MobileDeviceID: "1",
			DeviceType:     "tvOS",
			General:        &jamf.MobileDeviceGeneralSection{},
			Hardware: &jamf.MobileDeviceHardwareSection{
				SerialNumber:    "tvdev1",
				ModelIdentifier: "AppleTV11,1",
			},
		},
		{
			MobileDeviceID: "2",
			DeviceType:     "tvOS",
			General:        &jamf.MobileDeviceGeneralSection{},
			Hardware: &jamf.MobileDeviceHardwareSection{
				SerialNumber:    "tvdev2",
				ModelIdentifier: "AppleTV11,1",
			},
		},
		{
			MobileDeviceID: "3",
			DeviceType:     "iOS",
			General:        &jamf.MobileDeviceGeneralSection{},
			Hardware: &jamf.MobileDeviceHardwareSection{
				SerialNumber:    "mdev3",
				ModelIdentifier: "iPad15,7",
			},
		},
		{
			MobileDeviceID: "4",
			DeviceType:     "iOS",
			General:        &jamf.MobileDeviceGeneralSection{},
			Hardware: &jamf.MobileDeviceHardwareSection{
				SerialNumber:    "mdev4",
				ModelIdentifier: "iPhone15,2",
			},
		},
	}
	api.SetMobileDeviceInventory(jamfDevs)

	wantDevs := []*devicepb.Device{
		mobileDeviceFromMinimal(jamfDevs[2], nil /* source */),
		mobileDeviceFromMinimal(jamfDevs[3], nil /* source */),
	}

	s := serviceFromEnv(t, env, nil /* modifyOpts */)

	if _, err := s.RunOnce(t.Context(), jamfservice.RunSpec{
		Mode:       mdmsync.SyncModeFull,
		OnMissing:  jamfservice.DeviceActionDelete,
		DeviceType: types.JamfDeviceTypeMobileDevices,
		PageSize:   2,
	}); err != nil {
		t.Fatalf("RunOnce failed: %v", err)
	}

	got := listAllDevices(t, devicesClient)
	opts := append(devicesCmpOpts, protocmp.IgnoreFields(&devicepb.Device{}, "source"))
	if diff := cmp.Diff(wantDevs, got, opts...); diff != "" {
		t.Errorf("RunOnce sync mismatch (-want +got)\n%s", diff)
	}
}

// TestS_RunOnce_noGoroutineLeakOnStreamError verifies that when the consumer
// returns early on a stream error, the producer goroutine reading from Jamf
// exits too.
func TestS_RunOnce_noGoroutineLeakOnStreamError(t *testing.T) {
	t.Parallel()

	env := testenv.NewUsingT(t, &testenv.Opts{
		DeviceTrustEnv: true,
	})

	// The consumer reads a single page before the mocked stream error, so the
	// producer only has to outrun the channel buffer to end up parked on a send
	// forever (the leak this test guards against). Syncing one device per page,
	// ten devices is comfortably more than devicesC can buffer.
	const numDevices = 10
	jamfDevs := make([]*jamf.ComputerInventory, 0, numDevices)
	for i := 1; i <= numDevices; i++ {
		jamfDevs = append(jamfDevs, &jamf.ComputerInventory{
			ID:   strconv.Itoa(i),
			UDID: strconv.Itoa(i),
			General: &jamf.ComputerGeneralSection{
				Platform:   "Mac",
				ReportDate: time.Unix(int64(i), 0),
			},
			Hardware: &jamf.ComputerHardwareSection{
				SerialNumber: fmt.Sprintf("C%011d", i),
			},
		})
	}
	env.API.SetInventory(jamfDevs)

	s := serviceFromEnv(t, env, func(opts *jamfservice.Opts) {
		opts.DevicesClient = failingDevicesClient{DeviceTrustServiceClient: opts.DevicesClient}
	})

	// Snapshot goroutines after the env is up, so its servers aren't flagged.
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	_, err := s.RunOnce(t.Context(), jamfservice.RunSpec{
		Mode:       mdmsync.SyncModeFull,
		DeviceType: types.JamfDeviceTypeComputers,
		PageSize:   1,
	})
	require.Error(t, err)
}

func clearDevices(t *testing.T, devicesClient devicepb.DeviceTrustServiceClient) {
	t.Helper()
	for _, d := range listAllDevices(t, devicesClient) {
		// Cannot use t.Context here as clearDevices is typically run from t.Cleanup
		// so t.Context is already canceled.
		_, err := devicesClient.DeleteDevice(context.Background(), &devicepb.DeleteDeviceRequest{
			DeviceId: d.Id,
		})
		require.NoError(t, err, "DeleteDevice failed for %v", d.Id)
	}
}

func deviceFromMinimal(c *jamf.ComputerInventory, source *devicepb.DeviceSource) *devicepb.Device {
	return &devicepb.Device{
		OsType:       devicepb.OSType_OS_TYPE_MACOS,
		AssetTag:     c.Hardware.SerialNumber,
		EnrollStatus: devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED,
		Source:       source,
		Profile: &devicepb.DeviceProfile{
			ExternalId: c.ID,
		},
	}
}

func mobileDeviceFromMinimal(md *jamf.MobileDevice, source *devicepb.DeviceSource) *devicepb.Device {
	osType := devicepb.OSType_OS_TYPE_UNSPECIFIED
	if strings.HasPrefix(md.Hardware.ModelIdentifier, "iPad") {
		osType = devicepb.OSType_OS_TYPE_IPADOS
	} else if strings.HasPrefix(md.Hardware.ModelIdentifier, "iPhone") {
		osType = devicepb.OSType_OS_TYPE_IOS
	}
	return &devicepb.Device{
		OsType:       osType,
		AssetTag:     md.Hardware.SerialNumber,
		EnrollStatus: devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED,
		Source:       source,
		Profile: &devicepb.DeviceProfile{
			ModelIdentifier: md.Hardware.ModelIdentifier,
			ExternalId:      md.MobileDeviceID,
		},
	}
}

func listAllDevices(t *testing.T, devicesClient devicepb.DeviceTrustServiceClient) []*devicepb.Device {
	ctx := context.Background()
	var devs []*devicepb.Device
	var pageToken string
	for {
		resp, err := devicesClient.ListDevices(ctx, &devicepb.ListDevicesRequest{
			PageToken: pageToken,
			View:      devicepb.DeviceView_DEVICE_VIEW_RESOURCE,
		})
		if err != nil {
			t.Fatalf("ListDevices failed: %v", err)
		}
		devs = append(devs, resp.Devices...)
		if resp.NextPageToken == "" {
			return devs
		}
		pageToken = resp.NextPageToken
	}
}

func serviceFromEnv(t testing.TB, env *testenv.E, modifyOpts func(opts *jamfservice.Opts)) *jamfservice.S {
	t.Helper()

	opts := jamfservice.Opts{
		Clock:  env.Clock,
		Logger: env.Logger,
		Config: &servicecfg.JamfConfig{
			Spec: &types.JamfSpecV1{
				Enabled:     true,
				SyncDelay:   -1, // always sync immediately
				ApiEndpoint: env.APIEndpoint,
			},
			Credentials: &servicecfg.JamfCredentials{
				Username: testenv.DefaultUsers[0].Username,
				Password: testenv.DefaultUsers[0].Password,
			},
		},
		DevicesClient: env.DevicesClient,
		HTTPClient:    env.HTTPClient,
	}
	if modifyOpts != nil {
		modifyOpts(&opts)
	}

	ctx := context.Background()
	s, err := jamfservice.New(ctx, opts)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	return s
}

// failingSyncStream acks the initial sync request, then fails every subsequent
// Recv. This drives RunOnce into its "devices: Recv" error return while the
// producer goroutine is still reading pages from Jamf.
type failingSyncStream struct {
	grpc.BidiStreamingClient[devicepb.SyncInventoryRequest, devicepb.SyncInventoryResponse]
	recvCount int
}

func (s *failingSyncStream) Send(*devicepb.SyncInventoryRequest) error { return nil }

func (s *failingSyncStream) CloseSend() error { return nil }

func (s *failingSyncStream) Recv() (*devicepb.SyncInventoryResponse, error) {
	s.recvCount++
	if s.recvCount == 1 {
		return &devicepb.SyncInventoryResponse{
			Payload: &devicepb.SyncInventoryResponse_Ack{Ack: &devicepb.SyncInventoryAck{}},
		}, nil
	}
	return nil, errors.New("simulated stream failure")
}

// failingDevicesClient hands out a [failingSyncStream] for SyncInventory and
// delegates everything else to the embedded client.
type failingDevicesClient struct {
	devicepb.DeviceTrustServiceClient
}

func (c failingDevicesClient) SyncInventory(ctx context.Context, opts ...grpc.CallOption) (grpc.BidiStreamingClient[devicepb.SyncInventoryRequest, devicepb.SyncInventoryResponse], error) {
	return &failingSyncStream{}, nil
}
