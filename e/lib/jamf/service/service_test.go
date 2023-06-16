package service_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/jamf"
	jamfservice "github.com/gravitational/teleport/e/lib/jamf/service"
	"github.com/gravitational/teleport/e/lib/jamf/testenv"
	"github.com/gravitational/teleport/e/lib/mdm"
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
		HTTPClient:    http.DefaultClient,
		NewJamfClient: func(ctx context.Context, opts jamf.ClientOpts) (*jamf.Client, error) {
			opts.AllowPlainHTTP = true
			return jamf.NewClient(ctx, opts)
		},
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

	logger := log.New()
	logger.SetLevel(log.PanicLevel) // mostly silent logger

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
				Name:    "Mac OS X",
				Version: "10.9.5",
				Build:   "13A603",
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
				SerialNumber: strings.Repeat("x", 100),
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
				ModelIdentifier: jamfDevs[0].Hardware.ModelIdentifier,
				OsVersion:       jamfDevs[0].OperatingSystem.Version,
				OsBuild:         jamfDevs[0].OperatingSystem.Build,
				OsUsernames: []string{
					jamfDevs[0].LocalUserAccounts[0].Username,
					jamfDevs[0].LocalUserAccounts[1].Username,
				},
				JamfBinaryVersion: jamfDevs[0].General.JamfBinaryVersion,
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
		DeviceTrustEnv: true,
	})

	t0 := clock.Now()
	t1 := clock.Now()

	api := env.API
	devicesClient := env.DevicesClient

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

	s := serviceFromEnv(t, env, func(opts *jamfservice.Opts) {
		opts.Config.Spec.Inventory = []*types.JamfInventoryEntry{
			{
				SyncPeriodPartial: -1, // disabled
				SyncPeriodFull:    types.Duration(100 * time.Millisecond),
				OnMissing:         "DELETE",
			},
		}
	})

	// Run in the background.
	runCtx, runCancel := context.WithCancel(context.Background())
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

func deviceFromMinimal(c *jamf.ComputerInventory, source *devicepb.DeviceSource) *devicepb.Device {
	return &devicepb.Device{
		OsType:       devicepb.OSType_OS_TYPE_MACOS,
		AssetTag:     c.Hardware.SerialNumber,
		EnrollStatus: devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED,
		Source:       source,
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

func serviceFromEnv(t *testing.T, env *testenv.E, modifyOpts func(opts *jamfservice.Opts)) *jamfservice.S {
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
		HTTPClient: &http.Client{
			Timeout: 1 * time.Minute,
		},
		NewJamfClient: func(ctx context.Context, opts jamf.ClientOpts) (*jamf.Client, error) {
			opts.AllowPlainHTTP = true
			return jamf.NewClient(ctx, opts)
		},
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
