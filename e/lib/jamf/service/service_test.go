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
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/testing/protocmp"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/jamf"
	jamffake "github.com/gravitational/teleport/e/lib/jamf/fake"
	jamfservice "github.com/gravitational/teleport/e/lib/jamf/service"
	"github.com/gravitational/teleport/e/lib/jamf/testenv"
	"github.com/gravitational/teleport/e/lib/mdm"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/modules"
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

	// Enable Enterprise build.
	beforeModules := modules.GetModules()
	modules.SetModules(&modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.DeviceTrust:            {Enabled: true},
				entitlements.MobileDeviceManagement: {Enabled: true},
			},
		},
	})
	b.Cleanup(func() { modules.SetModules(beforeModules) })

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
		Mode:      mdm.SyncModeFull,
		OnMissing: mdm.DeviceActionDelete,
	}); err != nil {
		b.Fatalf("Warmup RunOnce failed: %v", err)
	}

	// Benchmark.
	b.StartTimer()
	api.SetInventory(inv)
	_, err := s.RunOnce(ctx, jamfservice.RunSpec{
		Mode:      mdm.SyncModeFull,
		OnMissing: mdm.DeviceActionDelete,
	})
	if err != nil {
		b.Fatalf("RunOnce failed: %v", err)
	}
}

func TestNew_errors(t *testing.T) {
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
				Username:    testenv.DefaultUsers[0].Username,
				Password:    testenv.DefaultUsers[0].Password,
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
						Enabled:  true,
						Username: baseOpts.Config.Spec.Username,
						Password: baseOpts.Config.Spec.Password,
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
						Username:    baseOpts.Config.Spec.Username,
						Password:    baseOpts.Config.Spec.Password,
						Inventory: []*types.JamfInventoryEntry{
							{
								SyncPeriodPartial: -1, // disabled
								SyncPeriodFull:    -1, // disabled
							},
						},
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
	env := testenv.NewUsingT(t, &testenv.Opts{
		DeviceTrustEnv: true,
	})

	s := serviceFromEnv(t, env, func(opts *jamfservice.Opts) {
		// Don't trigger a sync.
		opts.Config.Spec.SyncDelay = types.Duration(1 * time.Hour)
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

	source := &devicepb.DeviceSource{
		Name:   "jamf",
		Origin: devicepb.DeviceOrigin_DEVICE_ORIGIN_JAMF,
	}
	s := serviceFromEnv(t, env, func(opts *jamfservice.Opts) {
		opts.Config.Spec.Name = source.Name
		opts.Config.Spec.Inventory = []*types.JamfInventoryEntry{
			{
				SyncPeriodPartial: -1, // disabled
				SyncPeriodFull:    types.Duration(100 * time.Millisecond),
				OnMissing:         "DELETE",
			},
		}
	})

	// Add a couple of devices to Teleport that have no match in Jamf.
	// These get removed in the first sync.
	if resp, err := devicesClient.BulkCreateDevices(ctx, &devicepb.BulkCreateDevicesRequest{
		Devices: []*devicepb.Device{
			{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "deleteonsync1",
				Source:   source, // Assign to Jamf.
				// Profile missing external_id.
			},
			{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "deleteonsync2",
				Source:   source, // Assign to Jamf.
				Profile: &devicepb.DeviceProfile{
					ExternalId: jamfDevs[1].ID, // Mismatched Jamf ID.
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

	// Use the first sync to write the full list of devices.
	waitSynced(t, len(jamfDevs))

	// Remove a device from Jamf and wait for the change to be reflected in
	// Teleport.
	api.SetInventory(jamfDevs[1:])
	waitSynced(t, len(jamfDevs)-1)
	runCancel() // Stop syncs.

	// Verify deletions.
	got := listAllDevices(t, devicesClient)
	want := []*devicepb.Device{
		deviceFromMinimal(jamfDevs[1], nil /* source */),
		deviceFromMinimal(jamfDevs[2], nil /* source */),
	}
	opts := append(devicesCmpOpts, protocmp.IgnoreFields(&devicepb.Device{}, "source"))
	if diff := cmp.Diff(want, got, opts...); diff != "" {
		t.Errorf("Run sync mismatch (-want +got)\n%s", diff)
	}
}

func TestS_Run_exitOnSync(t *testing.T) {
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
		spec := opts.Config.Spec
		spec.Username = ""
		spec.Password = ""
		spec.ClientId = clientID
		spec.ClientSecret = clientSecret

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

	source := &devicepb.DeviceSource{
		Name:   "jamf2",
		Origin: devicepb.DeviceOrigin_DEVICE_ORIGIN_JAMF,
	}
	allDevs := make([]*devicepb.Device, len(jamfDevs))
	for i, j := range jamfDevs {
		allDevs[i] = deviceFromMinimal(j, source)
	}

	s := serviceFromEnv(t, env, func(opts *jamfservice.Opts) {
		opts.Config.Spec.Name = source.Name
	})
	ctx := context.Background()

	// Sync with a too-high cut time to begin with.
	t.Run("t=now", func(t *testing.T) {
		gotTime, err := s.RunOnce(ctx, jamfservice.RunSpec{
			Mode:    mdm.SyncModePartial,
			CutTime: now,
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
				Mode:    mdm.SyncModePartial,
				CutTime: cutTime,
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
}

func TestS_RunOnce_pagingGaps(t *testing.T) {
	env := testenv.NewUsingT(t, &testenv.Opts{
		DeviceTrustEnv: true,
	})

	api := env.API
	devicesClient := env.DevicesClient
	ctx := context.Background()

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
	allDevs := []*devicepb.Device{
		deviceFromMinimal(jamfDevs[1], nil /* source */),
		deviceFromMinimal(jamfDevs[0], nil /* source */),
		deviceFromMinimal(jamfDevs[2], nil /* source */),
	}
	api.SetInventory(jamfDevs)

	s := serviceFromEnv(t, env, func(opts *jamfservice.Opts) {
		opts.Config.Spec.Inventory = []*types.JamfInventoryEntry{
			{
				SyncPeriodPartial: -1, // disabled
				SyncPeriodFull:    types.Duration(100 * time.Millisecond),
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
		Mode:      mdm.SyncModeFull,
		OnMissing: mdm.DeviceActionDelete,
	}); err != nil {
		t.Fatalf("RunOnce failed: %v", err)
	}

	// Sanity check: all devices synced.
	assertStored(t, allDevs)

	// Sync with paging gaps.
	// We expect no deletions to happen in Teleport.
	api.SetSimulatePagingGaps(true)
	if _, err := s.RunOnce(ctx, jamfservice.RunSpec{
		Mode:      mdm.SyncModeFull,
		OnMissing: mdm.DeviceActionDelete,
	}); err != nil {
		t.Fatalf("RunOnce failed: %v", err)
	}

	// Verify no deletions in Teleport.
	assertStored(t, allDevs)
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
				Username:    testenv.DefaultUsers[0].Username,
				Password:    testenv.DefaultUsers[0].Password,
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
