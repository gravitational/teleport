package service

import (
	"cmp"
	"context"
	"errors"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/codes"

	"github.com/gravitational/teleport"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/jamf"
	"github.com/gravitational/teleport/e/lib/mdmsync"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/service/servicecfg"
)

const (
	// Avoid page sizes that are too large, it can cause "the read limit is
	// reached" errors.
	inventoryReadDefaultPageSize = 100
	// jamfSubsystem is the metric subsystem for Jamf.
	jamfSubsystem = "jamf"
)

var defaultInventory = []*types.JamfInventoryEntry{
	// https://github.com/gravitational/teleport.e/blob/master/rfd/0007e-device-trust-mdm-integration.md#jamf-inventory-sync
	{
		FilterRsql:        `general.remoteManagement.managed==true`,
		SyncPeriodPartial: types.DurationStringForJamfSpecV1(6 * time.Hour),
		SyncPeriodFull:    types.DurationStringForJamfSpecV1(24 * time.Hour),
		OnMissing:         "DELETE",
	},
}

var (
	syncsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   teleport.MetricNamespace,
		Subsystem:   jamfSubsystem,
		Name:        "syncs_total",
		Help:        "Number of inventory sync runs, labeled by mode and outcome",
		ConstLabels: map[string]string{},
	}, []string{"mode", "success"})

	allMetrics = []prometheus.Collector{
		syncsTotal,
	}
)

// S is the Jamf service implementation.
//
// The Jamf service is an MDM service specialization that syncs device inventory
// from Jamf to the Auth Server/DeviceTrustService.
type S struct {
	logger    *slog.Logger
	clock     clockwork.Clock
	config    *servicecfg.JamfConfig
	devices   devicepb.DeviceTrustServiceClient
	jamf      *jamf.Client
	scheduler *mdmsync.Scheduler[*scheduleEntry]
	// pluginStatusSink is only used when Jamf service is run as a hosted plugin in cloud.
	pluginStatusSink common.StatusSink
}

// Opts are creation options from [S].
type Opts struct {
	Clock            clockwork.Clock
	Logger           *slog.Logger
	Config           *servicecfg.JamfConfig
	DevicesClient    devicepb.DeviceTrustServiceClient
	HTTPClient       *http.Client
	PluginStatusSink common.StatusSink
}

// New creates a new [S] instance.
// `ctx` is used to perform initial validations against the Jamf API.
func New(ctx context.Context, opts Opts) (*S, error) {
	// Register service metrics. Expected to always work.
	if err := metrics.RegisterPrometheusCollectors(allMetrics...); err != nil {
		return nil, trace.Wrap(err)
	}

	switch {
	case opts.Logger == nil:
		return nil, trace.BadParameter("parameter Logger required")
	case opts.Config == nil:
		return nil, trace.BadParameter("parameter Config required")
	case opts.Config.Spec == nil:
		return nil, trace.BadParameter("jamf configuration required")
	case opts.DevicesClient == nil:
		return nil, trace.BadParameter("parameter DevicesClient required")
	case opts.HTTPClient == nil:
		return nil, trace.BadParameter("parameter HTTPClient required")
	}

	// Take defensive copies of the spec and config, so we can freely modify them.
	spec := *opts.Config.Spec
	if len(spec.Inventory) == 0 {
		spec.Inventory = defaultInventory
	}
	cfg := *opts.Config
	cfg.Spec = &spec

	// Make sure the (possibly modified) config is valid.
	if err := types.ValidateJamfSpecV1(cfg.Spec); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := servicecfg.ValidateJamfCredentials(cfg.Credentials); err != nil {
		return nil, trace.Wrap(err, "invalid Jamf credentials")
	}

	// Create scheduler (and early detect empty schedules).
	logger := opts.Logger
	scheduler, err := newJamfScheduler(cfg.Spec)
	if errors.Is(err, mdmsync.ErrScheduleEmpty) {
		logger.ErrorContext(ctx, "Jamf service has an empty sync schedule, aborting")
		return nil, trace.Wrap(err)
	} else if err != nil {
		return nil, trace.Wrap(err)
	}

	clock := opts.Clock
	if clock == nil {
		clock = clockwork.NewRealClock()
	}

	// Connect to the Jamf API and verify credentials.
	jamfClient, err := jamf.NewClient(ctx, jamf.ClientOpts{
		Clock:        clock,
		Logger:       logger,
		HTTPClient:   opts.HTTPClient,
		APIURL:       spec.ApiEndpoint,
		Username:     cfg.Credentials.Username,
		Password:     cfg.Credentials.Password,
		ClientID:     cfg.Credentials.ClientID,
		ClientSecret: cfg.Credentials.ClientSecret,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s := &S{
		logger:           logger,
		clock:            clock,
		config:           &cfg,
		devices:          opts.DevicesClient,
		jamf:             jamfClient,
		scheduler:        scheduler,
		pluginStatusSink: opts.PluginStatusSink,
	}
	if err := s.verifyInventoryFilters(ctx); err != nil {
		return nil, trace.Wrap(err)
	}
	return s, nil
}

type scheduleEntry struct {
	*types.JamfInventoryEntry

	onMissing DeviceAction
	cutTime   time.Time
}

func newJamfScheduler(spec *types.JamfSpecV1) (*mdmsync.Scheduler[*scheduleEntry], error) {
	parsedInv := make([]*scheduleEntry, len(spec.Inventory))
	for i, e := range spec.Inventory {
		onMissing := DeviceActionNoop
		if e.OnMissing == types.JamfOnMissingDelete {
			onMissing = DeviceActionDelete
		}
		parsedInv[i] = &scheduleEntry{
			JamfInventoryEntry: e,
			onMissing:          onMissing,
		}
	}

	var delayFn func() time.Duration
	switch {
	case spec.SyncDelay < 0: // immediate
		delayFn = func() time.Duration { return 0 }
	case spec.SyncDelay > 0: // as specified
		delayFn = func() time.Duration { return time.Duration(spec.SyncDelay) }
	default: // random
		delayFn = func() time.Duration { return rand.N(2 * time.Minute) }
	}

	return mdmsync.New(
		parsedInv, delayFn, func(e *scheduleEntry) mdmsync.EntryInfo {
			if e == nil || e.JamfInventoryEntry == nil {
				return mdmsync.EntryInfo{}
			}
			return mdmsync.EntryInfo{
				SyncPeriodPartial: time.Duration(e.SyncPeriodPartial),
				SyncPeriodFull:    time.Duration(e.SyncPeriodFull),
			}
		})
}

// verifyInventoryFilters verifies, during service startup, that the Jamf
// queries we intend to run are valid.
func (s *S) verifyInventoryFilters(ctx context.Context) error {
	for _, entry := range s.config.Spec.Inventory {
		switch entry.DeviceType {
		case "", types.JamfDeviceTypeComputers:
			if err := s.verifyComputersInventoryFilters(ctx, entry); err != nil {
				return trace.Wrap(err)
			}
		case types.JamfDeviceTypeMobileDevices:
			if err := s.verifyMobileInventoryFilters(ctx, entry); err != nil {
				return trace.Wrap(err)
			}
		}
	}
	return nil
}

// Run starts the Jamf service, blocking until the context is closed or a fatal
// error occurs.
func (s *S) Run(ctx context.Context) error {
	s.logger.InfoContext(ctx, "Jamf service successfully started")

	exitOnSync := s.config.ExitOnSync
	if exitOnSync {
		s.config.Spec.SyncDelay = -1
	}

	for {
		offset := s.scheduler.NextOffset()
		if exitOnSync && offset > 0 {
			s.logger.InfoContext(ctx, "All immediate syncs are done, exiting [exit_on_sync=true]")
			return nil
		}

		select {
		case <-s.clock.After(offset):
			// TODO(codingllama): "Downgrade" initial FULL sync to PARTIAL depending
			//  on device counts?
			e := s.scheduler.Next()

			nextCutTime, err := s.RunOnce(ctx, RunSpec{
				Mode:       e.Mode,
				OnMissing:  e.Entry.onMissing,
				FilterRSQL: e.Entry.FilterRsql,
				DeviceType: e.Entry.DeviceType,
				CutTime:    e.Entry.cutTime,
				PageSize:   int(e.Entry.PageSize),
			})
			syncsTotal.WithLabelValues(
				strconv.Itoa(int(e.Mode)),
				strconv.FormatBool(err == nil),
			).Inc()
			if err != nil {
				s.logger.WarnContext(ctx,
					"Jamf inventory sync attempt failed",
					"error", err,
				)
				if s.pluginStatusSink != nil {
					s.pluginStatusSink.Emit(ctx, &types.PluginStatusV1{Code: types.PluginStatusCode_OTHER_ERROR})
				}
				continue
			}

			// Emit PluginStatusCode_RUNNING after each successful sync.
			if s.pluginStatusSink != nil {
				s.pluginStatusSink.Emit(ctx, &types.PluginStatusV1{Code: types.PluginStatusCode_RUNNING})
			}

			// Update cut time.
			if nextCutTime.After(e.Entry.cutTime) {
				e.Entry.cutTime = nextCutTime
			}

		case <-ctx.Done():
			s.logger.InfoContext(ctx, "Exited")
			return ctx.Err()
		}
	}
}

// RunSpec holds the parameters for an [S.RunOnce] invocation.
type RunSpec struct {
	Mode       mdmsync.SyncMode
	OnMissing  DeviceAction
	FilterRSQL string
	// DeviceType is the device type to sync.
	// If empty, defaults to "computers".
	DeviceType string
	// CutTime is the cut time for partial syncs. The sync stops as soon as the
	// first device modified before `CutTime` is found.
	CutTime time.Time
	// PageSize is the page size to use when querying the Jamf inventory.
	// If zero or negative a default value is used.
	PageSize int
}

// RunOnce attempts to perform a single sync operation with the given [RunSpec].
// Returns the highest cut time observed during the sync.
func (s *S) RunOnce(ctx context.Context, spec RunSpec) (nextCutTime time.Time, err error) {
	if spec.PageSize <= 0 {
		spec.PageSize = inventoryReadDefaultPageSize
	}

	deviceType := cmp.Or(spec.DeviceType, types.JamfDeviceTypeComputers)
	if !slices.Contains(types.JamfDeviceTypes, deviceType) {
		return time.Time{}, trace.BadParameter("unknown device type %q", deviceType)
	}

	s.logger.InfoContext(ctx,
		"Starting sync",
		"mode", spec.Mode,
		"device_type", deviceType,
		"filter_rsql", spec.FilterRSQL,
		"on_missing", spec.OnMissing,
		"cut_time", spec.CutTime,
		"page_size", spec.PageSize,
	)
	start := s.clock.Now()

	stream, err := s.devices.SyncInventory(ctx)
	if err != nil {
		return time.Time{}, trace.Wrap(err)
	}
	defer stream.CloseSend()

	// Init.
	sourceName := "jamf" // default
	if s.config.Spec.Name != "" {
		sourceName = s.config.Spec.Name
	}
	osTypes := []devicepb.OSType{devicepb.OSType_OS_TYPE_MACOS}
	if deviceType == types.JamfDeviceTypeMobileDevices {
		osTypes = []devicepb.OSType{devicepb.OSType_OS_TYPE_IOS, devicepb.OSType_OS_TYPE_IPADOS}
	}
	if err := stream.Send(devicepb.SyncInventoryRequest_builder{
		Start: devicepb.SyncInventoryStart_builder{
			OsTypes: osTypes,
			Source: devicepb.DeviceSource_builder{
				Name:   sourceName,
				Origin: devicepb.DeviceOrigin_DEVICE_ORIGIN_JAMF,
			}.Build(),
			TrackMissingDevices: spec.Mode == mdmsync.SyncModeFull && spec.OnMissing == DeviceActionDelete,
		}.Build(),
	}.Build()); err != nil {
		return time.Time{}, trace.Wrap(err, "init: Send")
	}
	resp, err := stream.Recv()
	if err != nil {
		return time.Time{}, trace.Wrap(err, "init: Recv")
	}
	if resp.GetAck() == nil {
		return time.Time{}, trace.BadParameter("unexpected payload %T, expecting ack", resp.Payload)
	}

	const devicesBuffer = 4 // Allow for a small backlog to build up.
	devicesC := make(chan devicesResp, devicesBuffer)

	// Cancel the producer goroutine when RunOnce returns, so that an early return
	// (for example, a stream error) doesn't leave it blocked forever on a send to
	// devicesC.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Read and convert inventory from Jamf.
	go func() {
		defer close(devicesC)

		var readPage func(ctx context.Context, spec RunSpec, pageNum int) (rawJamfPage, error)
		switch deviceType {
		case types.JamfDeviceTypeComputers:
			readPage = s.getComputersPage
		case types.JamfDeviceTypeMobileDevices:
			readPage = s.getMobilePage
		}

		jamfDevsCount := 0
		for pageNum := 0; ; pageNum++ {
			rawPage, err := readPage(ctx, spec, pageNum)
			if err != nil {
				select {
				case devicesC <- devicesResp{err: trace.Wrap(err)}:
				case <-ctx.Done():
				}
				return
			}

			jamfDevsCount += len(rawPage.devices)
			page := s.processDevicesPage(ctx, spec, deviceType, rawPage.devices)

			// Send every page, even ones that produced no Teleport devices, so
			// that the consumer can still advance the cut time past records that
			// were intentionally dropped during conversion.
			select {
			case devicesC <- devicesResp{
				teleportDevs: page.teleportDevs,
				jamfDevs:     page.jamfDevs,
				page:         pageNum,
				highCutTime:  page.highCutTime,
			}:
			case <-ctx.Done():
				return
			}

			// Stop?
			if page.partialStop ||
				jamfDevsCount >= rawPage.totalCount ||
				len(rawPage.devices) == 0 {
				return
			}
		}
	}()

	// Write devices to Teleport.
	syncCount := 0
	for {
		devsResp, ok := <-devicesC
		if !ok {
			break // No more devices.
		}
		if err := devsResp.err; err != nil {
			return time.Time{}, trace.Wrap(err)
		}

		if devsResp.highCutTime.After(nextCutTime) {
			nextCutTime = devsResp.highCutTime
		}

		if len(devsResp.teleportDevs) == 0 {
			continue // Empty page: cut time advanced, nothing to upsert.
		}

		if err := stream.Send(devicepb.SyncInventoryRequest_builder{
			DevicesToUpsert: devicepb.SyncInventoryDevices_builder{
				Devices: devsResp.teleportDevs,
			}.Build(),
		}.Build()); err != nil {
			return time.Time{}, trace.Wrap(err, "devices: Send")
		}
		resp, err = stream.Recv()
		if err != nil {
			return time.Time{}, trace.Wrap(err, "devices: Recv")
		}
		s.logSyncResult(resp.GetResult(), syncState{
			jamfDevs: devsResp.jamfDevs,
			page:     devsResp.page,
		})
		syncCount += len(resp.GetResult().GetDevices())
	}

	// End.
	if err := stream.Send(devicepb.SyncInventoryRequest_builder{
		End: &devicepb.SyncInventoryEnd{},
	}.Build()); err != nil {
		return time.Time{}, trace.Wrap(err, "end: Send")
	}

	// Handle missing devices until EOF.
	for page := 0; true; page++ {
		resp, err = stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return time.Time{}, trace.Wrap(err, "end: Recv missing devices")
		}

		devicesToRemove, err := s.confirmMissingDevices(ctx, resp.GetMissingDevices().GetDevices())
		if err != nil {
			return time.Time{}, trace.Wrap(err, "confirming missing devices in Jamf")
		}

		// Echo missing devices for deletion.
		if err := stream.Send(devicepb.SyncInventoryRequest_builder{
			DevicesToRemove: devicepb.SyncInventoryDevices_builder{
				Devices: devicesToRemove,
			}.Build(),
		}.Build()); err != nil {
			return time.Time{}, trace.Wrap(err, "end: Send devices to remove")
		}

		resp, err = stream.Recv()
		if err != nil {
			return time.Time{}, trace.Wrap(err, "end: Recv results")
		}
		s.logSyncResult(resp.GetResult(), syncState{
			page:         page,
			expectDelete: true,
		})
	}

	s.logger.InfoContext(ctx,
		"Synced devices",
		"count", syncCount,
		"elapsed", s.clock.Since(start),
	)
	return nextCutTime, nil
}

type devicesResp struct {
	teleportDevs []*devicepb.Device
	jamfDevs     []jamfDevice
	page         int
	highCutTime  time.Time
	err          error
}

// rawJamfPage is a single page of devices as fetched from Jamf, before
// [S.processDevicesPage] turns it into a [devicesPage].
type rawJamfPage struct {
	devices    []jamfDevice
	totalCount int // total count reported by the API
}

type devicesPage struct {
	highCutTime time.Time // highest observed cut date in the page
	partialStop bool      // true if a partial stop was triggered

	teleportDevs []*devicepb.Device
	// jamfDevs contains matching raw Jamf devices, each corresponding to an entry
	// with the same index in teleportDevs. It includes only Jamf devices that
	// could be converted to a Teleport device. For a full list of devices from
	// the API use [rawJamfPage.devices].
	jamfDevs []jamfDevice
}

// jamfDevice abstracts over different Jamf device types (computers, mobile
// devices) for the shared page-processing logic in processDevicesPage.
type jamfDevice interface {
	toDevice() (*devicepb.Device, error)
	cutTime() time.Time
	// platform returns a raw value from Jamf, e.g. "Mac", "iOS".
	platform() string
	serialNumber() string
	logSync(ctx context.Context, log logFunc, dev *devicepb.Device)
}

type logFunc func(context.Context, string, ...any)

// processDevicesPage processes a slice of jamfDevice values, converting them to
// Teleport devices and collecting metadata for the sync.
func (s *S) processDevicesPage(
	ctx context.Context, spec RunSpec, deviceType string, devices []jamfDevice,
) *devicesPage {
	partialStop := false
	var highCutTime time.Time

	// Any append to devs should have a matching append in jamfDevs.
	// This way when [S.logSyncResult] iterates over statuses for devices (based
	// on devs), each index always has a matching entry in jamfDevs.
	devs := make([]*devicepb.Device, 0, len(devices))
	jamfDevs := make([]jamfDevice, 0, len(devices))
	for _, d := range devices {
		deviceCutTime := d.cutTime()
		// Stop partial sync?
		if spec.Mode == mdmsync.SyncModePartial && !deviceCutTime.IsZero() && deviceCutTime.Before(spec.CutTime) {
			s.logger.DebugContext(ctx,
				"Stopping partial sync",
				"device_type", deviceType,
				"cut_time", deviceCutTime,
			)
			// Signal stop after this round of upserts.
			partialStop = true
			break
		}

		// Make sure to update highCutTime for every observed Jamf device, not only
		// those that can be successfully converted with d.toDevice.
		if deviceCutTime.After(highCutTime) {
			highCutTime = deviceCutTime
		}

		dev, err := d.toDevice()
		if err != nil {
			s.logger.WarnContext(ctx,
				"Failed to convert Jamf device to Teleport Device",
				"error", err,
			)
			continue
		}
		devs = append(devs, dev)
		jamfDevs = append(jamfDevs, d)

		if s.logger.Enabled(ctx, slog.LevelDebug) {
			d.logSync(ctx, s.logger.DebugContext, dev)
		}
	}

	return &devicesPage{
		highCutTime:  highCutTime,
		teleportDevs: devs,
		jamfDevs:     jamfDevs,
		partialStop:  partialStop,
	}
}

func (s *S) confirmMissingDevices(ctx context.Context, missingDevs []*devicepb.Device) ([]*devicepb.Device, error) {
	group, groupCtx := errgroup.WithContext(ctx)
	const jamfGroupLimit = 8 // Arbitrary. Not too many, not too few.
	group.SetLimit(jamfGroupLimit)

	missingLen := len(missingDevs)
	var devicesMux sync.Mutex // guards devicesToRemove
	devicesToRemove := make([]*devicepb.Device, 0, missingLen)
	markForRemoval := func(d *devicepb.Device) {
		devicesMux.Lock()
		devicesToRemove = append(devicesToRemove, d)
		devicesMux.Unlock()
	}

	// Concurrently query devices on Jamf.
	// We are looking for either confirmation that the device doesn't exist, or an
	// existing but mismatched device.
	for _, dev := range missingDevs {
		if dev.GetProfile().GetExternalId() == "" {
			s.logger.DebugContext(ctx,
				"Marking device without external_id for removal",
				"device", dev,
			)
			markForRemoval(dev)
			continue
		}

		group.Go(func() error {
			// Note: the queries below are rather conservative and, because of that,
			// we might keep around devices that would not appear otherwise (for
			// example, if further RSQL filters are applied).
			// This seems OK for the moment, as devices can be removed by other means
			// (such as `tctl devices rm`), but it is a point of attention.
			var matches bool
			var jamfDevForLogging any
			var err error
			switch dev.GetOsType() {
			case devicepb.OSType_OS_TYPE_IOS, devicepb.OSType_OS_TYPE_IPADOS:
				matches, jamfDevForLogging, err = s.confirmMobile(groupCtx, dev)
			default:
				matches, jamfDevForLogging, err = s.confirmComputer(groupCtx, dev)
			}

			apiErr := &jamf.APIError{}
			switch {
			case errors.As(err, &apiErr) && apiErr.StatusCode == 404:
				s.logger.DebugContext(ctx,
					"Marking unknown device for removal",
					"device", dev,
				)
				markForRemoval(dev)
			case err != nil: // Unexpected error
				s.logger.DebugContext(ctx,
					"Skipping removal of device, query failed",
					"error", err,
					"device", dev,
				)
			case matches:
				s.logger.DebugContext(ctx,
					"Skipping removal, device found in Jamf",
					"device", dev,
				)
			default:
				// ID matches the wrong device. A leftover from other times?
				s.logger.DebugContext(ctx,
					"Marking mismatched device for removal",
					"jamf_device", jamfDevForLogging,
					"device", dev,
				)
				markForRemoval(dev)
			}

			return nil
		})
	}

	if err := group.Wait(); err != nil {
		return nil, trace.Wrap(err)
	}

	return devicesToRemove, nil
}

type syncState struct {
	jamfDevs     []jamfDevice
	page         int
	expectDelete bool
}

func (s *S) logSyncResult(result *devicepb.SyncInventoryResult, state syncState) {
	var upserts, deletes, failures int
	for i, status := range result.GetDevices() {
		ok := codes.Code(status.GetStatus().GetCode()) == codes.OK
		switch {
		case !ok:
			failures++

			var platform, serialNumber string
			if len(state.jamfDevs) > i {
				platform = state.jamfDevs[i].platform()
				serialNumber = state.jamfDevs[i].serialNumber()
			}

			s.logger.WarnContext(context.Background(),
				"Failed to sync device",
				"code", status.GetStatus().GetCode(),
				"message", status.GetStatus().GetMessage(),
				"device_id", status.GetId(),
				"platform", platform,
				"serial_number", serialNumber,
				"expect_delete", state.expectDelete,
			)

		case status.GetDeleted():
			deletes++

		default:
			upserts++
		}
	}

	s.logger.InfoContext(context.Background(),
		"Device sync page report",
		"page", state.page,
		"upserts", upserts,
		"deletes", deletes,
		"failures", failures,
	)
}
