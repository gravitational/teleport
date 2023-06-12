package service_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/testing/protocmp"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/jamf"
	jamfservice "github.com/gravitational/teleport/e/lib/jamf/service"
	"github.com/gravitational/teleport/e/lib/jamf/testenv"
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

	// Create a few devices with varying LastContactTimes.
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
	newJamfDev := func(lastContactTime time.Time) *jamf.ComputerInventory {
		currentID++
		return &jamf.ComputerInventory{
			ID:   strconv.Itoa(currentID),
			UDID: strconv.Itoa(currentID),
			General: &jamf.ComputerGeneralSection{
				Platform:         "Mac",
				ReportDate:       t0,
				LastContactTime:  lastContactTime,
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
		allDevs[i] = deviceFromMinimal(j, devicepb.OSType_OS_TYPE_MACOS, source)
	}

	s := serviceFromEnv(t, env, func(opts *jamfservice.Opts) {
		opts.Config.Spec.Name = source.Name
	})
	ctx := context.Background()

	// Sync with a too-high cut time to begin with.
	t.Run("t=now", func(t *testing.T) {
		gotTime, err := s.RunOnce(ctx, jamfservice.RunSpec{
			Mode:    devicepb.SyncInventoryMode_SYNC_INVENTORY_MODE_PARTIAL,
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
				Mode:    devicepb.SyncInventoryMode_SYNC_INVENTORY_MODE_PARTIAL,
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

func deviceFromMinimal(c *jamf.ComputerInventory, osType devicepb.OSType, source *devicepb.DeviceSource) *devicepb.Device {
	return &devicepb.Device{
		OsType:       osType,
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

func serviceFromEnv(t *testing.T, env *testenv.E, modifyOpts func(*jamfservice.Opts)) *jamfservice.S {
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
