package intune

import (
	"cmp"
	"context"
	"errors"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc/codes"

	"github.com/gravitational/teleport"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/intune/api"
	"github.com/gravitational/teleport/e/lib/mdm"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/lib/observability/metrics"
)

// Config contains parameters needed by [Service].
type Config struct {
	APIConfig     api.Config
	Logger        *slog.Logger
	HTTPClient    *http.Client
	StatusSink    common.StatusSink
	Clock         clockwork.Clock
	DevicesClient devicepb.DeviceTrustServiceClient
	// syncImmediately allows syncing to start immediately. If false, the delay is randomized so that
	// the first sync happens within two minutes.
	syncImmediately bool
	// syncPeriodPartial is the PARTIAL sync period. Negative disables PARTIAL syncs.
	// Defaults to [defaultSyncPeriodPartial].
	syncPeriodPartial time.Duration
	// syncPeriodFull is the FULL sync period. Negative disables FULL syncs.
	// Defaults to [defaultSyncPeriodFull].
	syncPeriodFull time.Duration
}

const (
	defaultSyncPeriodPartial = 6 * time.Hour
	defaultSyncPeriodFull    = 24 * time.Hour
)

var (
	syncsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   teleport.MetricNamespace,
		Subsystem:   "intune",
		Name:        "syncs_total",
		Help:        "Number of inventory sync runs, labeled by mode and outcome",
		ConstLabels: map[string]string{},
	}, []string{"mode", "success"})

	allMetrics = []prometheus.Collector{
		syncsTotal,
	}
)

// NewService creates a new [Service].
// ctx is used to perform initial validations against the Intune API.
func NewService(ctx context.Context, config Config) (*Service, error) {
	// Register service metrics. Expected to always work.
	if err := metrics.RegisterPrometheusCollectors(allMetrics...); err != nil {
		return nil, trace.Wrap(err)
	}

	switch {
	case config.Logger == nil:
		return nil, trace.BadParameter("parameter Logger required")
	case config.StatusSink == nil:
		return nil, trace.BadParameter("parameter StatusSink required")
	case config.DevicesClient == nil:
		return nil, trace.BadParameter("parameter DevicesClient required")
	}
	config.Clock = cmp.Or(config.Clock, clockwork.NewRealClock())

	schedulerEntries := []*scheduleEntry{{
		SyncPeriodPartial: cmp.Or(config.syncPeriodPartial, defaultSyncPeriodPartial),
		SyncPeriodFull:    cmp.Or(config.syncPeriodFull, defaultSyncPeriodFull),
	}}
	delayFn := func() time.Duration { return rand.N(2 * time.Minute) }
	if config.syncImmediately {
		delayFn = func() time.Duration { return 0 }
	}
	scheduler, err := mdm.NewSyncScheduler(schedulerEntries, delayFn, func(e *scheduleEntry) mdm.ScheduleEntryInfo {
		return mdm.ScheduleEntryInfo{SyncPeriodPartial: e.SyncPeriodPartial, SyncPeriodFull: e.SyncPeriodFull}
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Connect to the Intune API and verify credentials.
	client, err := api.NewClient(ctx, api.ClientConfig{
		APIConfig:  config.APIConfig,
		Logger:     config.Logger,
		HTTPClient: config.HTTPClient,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Service{
		cfg:       config,
		intune:    client,
		scheduler: scheduler,
	}, nil
}

type scheduleEntry struct {
	SyncPeriodPartial time.Duration
	SyncPeriodFull    time.Duration
	// deviceLastSyncDateTime is the highest observed [api.ManagedDevice] LastSyncDateTime during
	// the previous partial/full sync. It is used during the next partial sync to fetch from Intune
	// only those devices that have since synchronized their state with Intune.
	deviceLastSyncDateTime time.Time
}

type Service struct {
	cfg       Config
	intune    *api.Client
	scheduler *mdm.SyncScheduler[*scheduleEntry]
}

// Run starts the Intune service, blocking until the context is closed.
func (s *Service) Run(ctx context.Context) error {
	s.cfg.Logger.InfoContext(ctx, "Intune service successfully started")

	for {
		offset := s.scheduler.NextOffset()
		select {
		case <-time.After(offset):
			e := s.scheduler.Next()

			nextDeviceLastSyncDateTime, err := s.runWithSpec(ctx, runSpec{
				mode:                   e.Mode,
				deviceLastSyncDateTime: e.Entry.deviceLastSyncDateTime,
			})
			syncsTotal.WithLabelValues(
				strconv.Itoa(int(e.Mode)),
				strconv.FormatBool(err == nil),
			).Inc()
			if err != nil {
				s.cfg.Logger.WarnContext(ctx, "Intune inventory sync attempt failed", "error", err)
				s.cfg.StatusSink.Emit(ctx, &types.PluginStatusV1{Code: types.PluginStatusCode_OTHER_ERROR})
				continue
			}

			s.cfg.StatusSink.Emit(ctx, &types.PluginStatusV1{Code: types.PluginStatusCode_RUNNING})

			if nextDeviceLastSyncDateTime.After(e.Entry.deviceLastSyncDateTime) {
				e.Entry.deviceLastSyncDateTime = nextDeviceLastSyncDateTime
			}
		case <-ctx.Done():
			s.cfg.Logger.InfoContext(ctx, "Intune service exited")
			return ctx.Err()
		}
	}
}

type runSpec struct {
	mode                   mdm.SyncMode
	deviceLastSyncDateTime time.Time
}

// runWithSpec carries out the actual logic of syncing with Intune. It supports partial and full
// syncs and the mode is controlled by spec.mode.
//
// During a full sync, it fetches all devices from Intune and asks the auth server to track missing
// devices which it later confirms and marks for removal. During a partial sync, it fetches only
// those devices that were updated in Intune after spec.deviceLastSyncDateTime.
//
// Returns the highest observed lastSyncDateTime among the processed devices.
func (s *Service) runWithSpec(ctx context.Context, spec runSpec) (nextDeviceLastSyncDateTime time.Time, err error) {
	s.cfg.Logger.InfoContext(ctx, "Starting sync",
		"mode", spec.mode, "last_sync_date_time", spec.deviceLastSyncDateTime)
	start := s.cfg.Clock.Now()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	stream, err := s.cfg.DevicesClient.SyncInventory(ctx)
	if err != nil {
		return time.Time{}, trace.Wrap(err)
	}
	defer stream.CloseSend()

	// Init.
	if err := stream.Send(&devicepb.SyncInventoryRequest{
		Payload: &devicepb.SyncInventoryRequest_Start{
			Start: &devicepb.SyncInventoryStart{
				Source: &devicepb.DeviceSource{
					Name:   "intune",
					Origin: devicepb.DeviceOrigin_DEVICE_ORIGIN_INTUNE,
				},
				TrackMissingDevices: spec.mode == mdm.SyncModeFull,
			},
		},
	}); err != nil {
		return time.Time{}, trace.Wrap(err, "init: Send")
	}
	ackResp, err := stream.Recv()
	if err != nil {
		return time.Time{}, trace.Wrap(err, "init: Recv")
	}
	if ackResp.GetAck() == nil {
		return time.Time{}, trace.BadParameter("unexpected payload %T, expecting ack", ackResp.Payload)
	}

	type devicesResp struct {
		page *devicesPage
		err  error
	}
	const devicesBuffer = 4 // Allow for a small backlog to build up.
	devicesC := make(chan devicesResp, devicesBuffer)

	// Read and convert inventory from Intune.
	go func() {
		defer close(devicesC)
		req := &api.ListManagedDevicesRequest{}
		if spec.mode == mdm.SyncModePartial {
			req.LastSyncDateTime = spec.deviceLastSyncDateTime
		}

		for {
			page, err := s.getDevicesPage(ctx, req)
			if err != nil {
				select {
				case <-ctx.Done():
				case devicesC <- devicesResp{err: trace.Wrap(err)}:
				}
				return
			}

			if len(page.teleportDevices) > 0 {
				select {
				case <-ctx.Done():
					return
				case devicesC <- devicesResp{page: page}:
				}
			}

			if page.deviceLastSyncDateTime.After(nextDeviceLastSyncDateTime) {
				nextDeviceLastSyncDateTime = page.deviceLastSyncDateTime
			}

			if page.resp.NextLink == "" {
				return
			}

			req.NextLink = page.resp.NextLink
		}
	}()

	// Write devices to Teleport.
	syncCount := 0
	for pageIdx := 0; true; pageIdx++ {
		devicesResp, ok := <-devicesC
		if !ok {
			break
		}
		if err := devicesResp.err; err != nil {
			return time.Time{}, trace.Wrap(err)
		}

		if err := stream.Send(&devicepb.SyncInventoryRequest{
			Payload: &devicepb.SyncInventoryRequest_DevicesToUpsert{
				DevicesToUpsert: &devicepb.SyncInventoryDevices{
					Devices: devicesResp.page.teleportDevices,
				},
			},
		}); err != nil {
			return time.Time{}, trace.Wrap(err, "devices: Send")
		}
		upsertResp, err := stream.Recv()
		if err != nil {
			return time.Time{}, trace.Wrap(err, "devices: Recv")
		}
		if upsertResp.GetResult() == nil {
			return time.Time{}, trace.BadParameter("unexpected payload %T, expecting result", upsertResp.Payload)
		}
		s.logSyncResult(ctx, upsertResp.GetResult(), pageIdx, devicesResp.page)
		syncCount += len(upsertResp.GetResult().GetDevices())
	}

	// End.
	if err := stream.Send(&devicepb.SyncInventoryRequest{
		Payload: &devicepb.SyncInventoryRequest_End{
			End: &devicepb.SyncInventoryEnd{},
		},
	}); err != nil {
		return time.Time{}, trace.Wrap(err, "end: Send")
	}

	// Handle missing devices until EOF.
	for pageIdx := 0; true; pageIdx++ {
		missingDevicesResp, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return time.Time{}, trace.Wrap(err, "end: Recv missing devices")
		}
		if missingDevicesResp.GetMissingDevices() == nil {
			return time.Time{}, trace.BadParameter("unexpected payload %T, expecting missing_devices", missingDevicesResp.Payload)
		}

		// TODO(ravicious): Confirm missing devices similar to how the Jamf service does it.
		// At the end of a full sync, the auth server reports to the Intune integration all devices from
		// Intune that are in the Teleport inventory but weren't observed during this sync. The job of
		// the Intune integration is to verify that those devices are indeed not present in Intune.
		// This will be implemented in a subsequent PR.
		devicesToRemove := missingDevicesResp.GetMissingDevices().GetDevices()

		if err := stream.Send(&devicepb.SyncInventoryRequest{
			Payload: &devicepb.SyncInventoryRequest_DevicesToRemove{
				DevicesToRemove: &devicepb.SyncInventoryDevices{
					Devices: devicesToRemove,
				},
			},
		}); err != nil {
			return time.Time{}, trace.Wrap(err, "end: Send devices to remove")
		}

		removeResp, err := stream.Recv()
		if err != nil {
			return time.Time{}, trace.Wrap(err, "end: Recv results")
		}
		if removeResp.GetResult() == nil {
			return time.Time{}, trace.BadParameter("unexpected payload %T, expecting result", removeResp.Payload)
		}
		s.logSyncResult(ctx, removeResp.GetResult(), pageIdx, nil /* page */)
	}

	s.cfg.Logger.InfoContext(ctx,
		"Synced devices",
		"count", syncCount,
		"elapsed", s.cfg.Clock.Since(start),
	)
	return nextDeviceLastSyncDateTime, nil
}

type devicesPage struct {
	resp *api.ListManagedDevicesResponse
	// teleportDevices is a filtered version of resp.ManagedDevices. It does not contain any device
	// which DeviceRegistrationState is different than "registered" and any device that could not be
	// converted to [devicepb.Device].
	teleportDevices []*devicepb.Device
	// teleportToIntuneIdx maps indices from teleportDevices to resp.ManagedDevices. When receiving
	// [devicepb.SyncInventoryResult], this lets us map a failure to sync a device to a specific
	// Intune device.
	teleportToIntuneIdx map[int]int
	// deviceLastSyncDateTime is the highest observed last sync date time of a device in that page.
	deviceLastSyncDateTime time.Time
}

func (s *Service) getDevicesPage(ctx context.Context, req *api.ListManagedDevicesRequest) (*devicesPage, error) {
	resp, err := s.intune.ListManagedDevices(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err, "Intune read failed")
	}

	var highLastSyncDateTime time.Time
	teleportToIntuneIdx := make(map[int]int)
	devices := make([]*devicepb.Device, 0, len(resp.ManagedDevices))
	for intuneIdx, intuneDevice := range resp.ManagedDevices {
		if intuneDevice == nil {
			s.cfg.Logger.DebugContext(ctx, "Skipping nil device")
			continue
		}

		// Purposefully not using slog.Logger.With as it's not free and the logger is always called just
		// once per device.
		logGroup := slog.Group("intune_device", "id", intuneDevice.ID, "serial_number", intuneDevice.SerialNumber)

		if intuneDevice.DeviceRegistrationState != api.DeviceRegistrationStateRegistered {
			s.cfg.Logger.DebugContext(ctx, "Skipping Intune device because it is not registered yet",
				logGroup, slog.String("device_registration_state", intuneDevice.DeviceRegistrationState),
			)
			continue
		}

		device, err := managedDeviceToDevice(intuneDevice)
		if err != nil {
			s.cfg.Logger.WarnContext(ctx, "Failed to convert Intune ManagedDevice to Teleport Device",
				logGroup, slog.Any("error", err))
			continue
		}
		if device.OsType == devicepb.OSType_OS_TYPE_UNSPECIFIED {
			// Emit a debug message instead of a warning. This scenario is expected to happen often enough
			// that we don't want to spam logs with warnings. A customer might have many devices in their
			// inventory that we don't support, e.g., Android devices. Intune API offers no way to filter
			// by the OS.
			s.cfg.Logger.DebugContext(ctx, "Skipping device due to unsupported operating system",
				logGroup, slog.String("operating_system", intuneDevice.OperatingSystem))
			continue
		}

		teleportToIntuneIdx[len(devices)] = intuneIdx
		devices = append(devices, device)

		// Keep tabs of highest-seen lastSyncDateTime.
		if intuneDevice.LastSyncDateTime.After(highLastSyncDateTime) {
			highLastSyncDateTime = intuneDevice.LastSyncDateTime
		}

		s.cfg.Logger.DebugContext(ctx, "Syncing Intune device",
			logGroup,
			slog.Time("last_sync_date_time", intuneDevice.LastSyncDateTime),
			slog.String("model", intuneDevice.Model),
			slog.String("operating_system", intuneDevice.OperatingSystem),
			slog.String("os_version", intuneDevice.OSVersion),
			slog.Any("profile", device.Profile),
		)
	}

	return &devicesPage{
		resp:                   resp,
		teleportDevices:        devices,
		teleportToIntuneIdx:    teleportToIntuneIdx,
		deviceLastSyncDateTime: highLastSyncDateTime,
	}, nil
}

// logSyncResult logs the result of submitting devices to the auth server to either be upserted or
// removed. page is present only when logging upsert results.
func (s *Service) logSyncResult(ctx context.Context, result *devicepb.SyncInventoryResult, pageIdx int, page *devicesPage) {
	var upserts, deletes, failures int
	for teleportIdx, status := range result.GetDevices() {
		ok := codes.Code(status.GetStatus().GetCode()) == codes.OK
		switch {
		case !ok:
			failures++

			syncType := "remove"
			var operatingSystem, serialNumber, intuneID string
			if page != nil {
				syncType = "upsert"
				if intuneIdx, found := page.teleportToIntuneIdx[teleportIdx]; found && intuneIdx < len(page.resp.ManagedDevices) {
					intuneDevice := page.resp.ManagedDevices[intuneIdx]
					intuneID = intuneDevice.ID
					operatingSystem = intuneDevice.OperatingSystem
					serialNumber = intuneDevice.SerialNumber
				}
			}

			s.cfg.Logger.WarnContext(ctx, "Failed to sync device",
				"sync_type", syncType,
				"code", status.GetStatus().GetCode(),
				"message", status.GetStatus().GetMessage(),
				"device_id", status.GetId(),
				"intune_id", intuneID,
				"operating_system", operatingSystem,
				"serial_number", serialNumber,
			)
		case status.GetDeleted():
			deletes++
		default:
			upserts++
		}
	}

	s.cfg.Logger.InfoContext(ctx, "Device sync page report",
		"page", pageIdx,
		"upserts", upserts,
		"deletes", deletes,
		"failures", failures,
	)
}
