package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
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
	"github.com/gravitational/teleport/e/lib/mdm"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/service/servicecfg"
)

const (
	// Avoid page sizes that are too large, it can cause "the read limit is
	// reached" errors.
	inventoryReadDefaultPageSize = 100

	sortByID             = "id:asc"
	sortByReportDateDesc = "general.reportDate:desc"
)

// jamfSubsystem is the metric subsystem for Jamf.
const jamfSubsystem = "jamf"

var defaultInventory = []*types.JamfInventoryEntry{
	// https://github.com/gravitational/teleport.e/blob/master/rfd/0007e-device-trust-mdm-integration.md#jamf-inventory-sync
	{
		FilterRsql:        `general.remoteManagement.managed==true`,
		SyncPeriodPartial: types.Duration(6 * time.Hour),
		SyncPeriodFull:    types.Duration(24 * time.Hour),
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
	scheduler *mdm.SyncScheduler[*scheduleEntry]
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
	if errors.Is(err, mdm.ErrScheduleEmpty) {
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

	onMissing mdm.DeviceAction
	cutTime   time.Time
}

func newJamfScheduler(spec *types.JamfSpecV1) (*mdm.SyncScheduler[*scheduleEntry], error) {
	parsedInv := make([]*scheduleEntry, len(spec.Inventory))
	for i, e := range spec.Inventory {
		onMissing := mdm.DeviceActionNoop
		if e.OnMissing == types.JamfOnMissingDelete {
			onMissing = mdm.DeviceActionDelete
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

	return mdm.NewSyncScheduler(
		parsedInv, delayFn, func(e *scheduleEntry) mdm.ScheduleEntryInfo {
			if e == nil || e.JamfInventoryEntry == nil {
				return mdm.ScheduleEntryInfo{}
			}
			return mdm.ScheduleEntryInfo{
				SyncPeriodPartial: time.Duration(e.SyncPeriodPartial),
				SyncPeriodFull:    time.Duration(e.SyncPeriodFull),
			}
		})
}

// verifyInventoryFilters verifies, during service startup, that the Jamf
// queries we intend to run are valid.
func (s *S) verifyInventoryFilters(ctx context.Context) error {
	for _, entry := range s.config.Spec.Inventory {
		if entry.FilterRsql == "" {
			continue
		}
		if _, err := s.jamf.GetComputersInventory(ctx, &jamf.GetComputersInventoryRequest{
			Page:     0,
			PageSize: 1,
			Sort:     []string{sortByReportDateDesc},
			Filter:   entry.FilterRsql,
		}); err != nil {
			return trace.BadParameter("computer inventory query, filter=%q: %v", entry.FilterRsql, err)
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

	// Create the timer and immediately stop/drain it, we'll reset it at the start
	// of every loop below.
	timer := time.NewTimer(999 * time.Hour)
	if !timer.Stop() {
		<-timer.C // Drain
	}

	for {
		offset := s.scheduler.NextOffset()
		if exitOnSync && offset > 0 {
			s.logger.InfoContext(ctx, "All immediate syncs are done, exiting [exit_on_sync=true]")
			return nil
		}
		// timer is always drained when we get here.
		timer.Reset(offset)

		select {
		case <-timer.C:
			// TODO(codingllama): "Downgrade" initial FULL sync to PARTIAL depending
			//  on device counts?
			e := s.scheduler.Next()

			nextCutTime, err := s.RunOnce(ctx, RunSpec{
				Mode:       e.Mode,
				OnMissing:  e.Entry.onMissing,
				FilterRSQL: e.Entry.FilterRsql,
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
			timer.Stop()
			s.logger.InfoContext(ctx, "Exited")
			return ctx.Err()
		}
	}
}

// RunSpec holds the parameters for an [S.RunOnce] invocation.
type RunSpec struct {
	Mode       mdm.SyncMode
	OnMissing  mdm.DeviceAction
	FilterRSQL string
	// CutTime is the cut time for partial syncs. The sync stops as soon as the
	// first computer modified before `CutTime` is found.
	CutTime time.Time
	// PageSize is the page size to use when querying the Jamf inventory.
	// If zero or negative a default value is used.
	PageSize int
}

// RunOnce attempts to perform a single sync operation with the given [RunSpec].
// Returns the cut time for the next sync, acquired from the first computer read
// from Jamf.
func (s *S) RunOnce(ctx context.Context, spec RunSpec) (nextCutTime time.Time, err error) {
	pageSize := spec.PageSize
	if pageSize <= 0 {
		pageSize = inventoryReadDefaultPageSize
	}

	s.logger.InfoContext(ctx,
		"Starting sync",
		"mode", spec.Mode,
		"filter_rsql", spec.FilterRSQL,
		"on_missing", spec.OnMissing,
		"cut_time", spec.CutTime,
		"page_size", pageSize,
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
	if err := stream.Send(&devicepb.SyncInventoryRequest{
		Payload: &devicepb.SyncInventoryRequest_Start{
			Start: &devicepb.SyncInventoryStart{
				Source: &devicepb.DeviceSource{
					Name:   sourceName,
					Origin: devicepb.DeviceOrigin_DEVICE_ORIGIN_JAMF,
				},
				TrackMissingDevices: spec.Mode == mdm.SyncModeFull && spec.OnMissing == mdm.DeviceActionDelete,
			},
		},
	}); err != nil {
		return time.Time{}, trace.Wrap(err, "init: Send")
	}
	resp, err := stream.Recv()
	if err != nil {
		return time.Time{}, trace.Wrap(err, "init: Recv")
	}
	if resp.GetAck() == nil {
		return time.Time{}, trace.BadParameter("unexpected payload %T, expecting ack", resp.Payload)
	}

	type devicesResp struct {
		jamfDevs     []*jamf.ComputerInventory
		teleportDevs []*devicepb.Device
		page         int
		err          error
	}
	const devicesBuffer = 4 // Allow for a small backlog to build up.
	devicesC := make(chan devicesResp, devicesBuffer)

	// Read and convert inventory from Jamf.
	go func() {
		req := &jamf.GetComputersInventoryRequest{
			Section: []string{
				jamf.SectionGeneral,
				jamf.SectionHardware,
				jamf.SectionLocalUserAccounts,
				jamf.SectionOperatingSystem,
			},
			PageSize: pageSize,
			Sort:     []string{sortByID}, // expected to be more "stable" than timestamps
			Filter:   spec.FilterRSQL,
		}

		// Sort by recent use on PARTIAL syncs.
		// Alternatively we could use an RSQL filter.
		if spec.Mode == mdm.SyncModePartial {
			req.Sort = []string{sortByReportDateDesc}
		}

		jamfDevsCount := 0
		for {
			page, err := s.getDevicesPage(ctx, spec, req)
			if err != nil {
				devicesC <- devicesResp{err: trace.Wrap(err)}
				return
			}

			jamfDevsCount += len(page.jamfDevs)
			if len(page.teleportDevs) > 0 {
				devicesC <- devicesResp{
					jamfDevs:     page.jamfDevs,
					teleportDevs: page.teleportDevs,
					page:         req.Page,
					err:          err,
				}
			}

			// Update cut time.
			if page.highCutTime.After(nextCutTime) {
				nextCutTime = page.highCutTime
			}

			// Stop?
			if page.partialStop ||
				jamfDevsCount >= page.jamfTotalCount ||
				len(page.teleportDevs) == 0 {
				close(devicesC)
				return
			}

			req.Page++
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

		if err := stream.Send(&devicepb.SyncInventoryRequest{
			Payload: &devicepb.SyncInventoryRequest_DevicesToUpsert{
				DevicesToUpsert: &devicepb.SyncInventoryDevices{
					Devices: devsResp.teleportDevs,
				},
			},
		}); err != nil {
			return time.Time{}, trace.Wrap(err, "devices: Send")
		}
		resp, err = stream.Recv()
		if err != nil {
			return time.Time{}, trace.Wrap(err, "devices: Recv")
		}
		s.logSyncResult(resp.GetResult(), syncState{
			jamfDevices: devsResp.jamfDevs,
			page:        devsResp.page,
		})
		syncCount += len(resp.GetResult().GetDevices())
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
		if err := stream.Send(&devicepb.SyncInventoryRequest{
			Payload: &devicepb.SyncInventoryRequest_DevicesToRemove{
				DevicesToRemove: &devicepb.SyncInventoryDevices{
					Devices: devicesToRemove,
				},
			},
		}); err != nil {
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

type devicesPage struct {
	jamfTotalCount int       // aka GetComputersInventoryResponse.TotalCount
	highCutTime    time.Time // highest observed cut date in the page
	partialStop    bool      // true if a partial stop was trigerred

	jamfDevs     []*jamf.ComputerInventory
	teleportDevs []*devicepb.Device
}

// getDevicesPage reads a single page of devices from Jamf and converts them
// to Teleport devices.
func (s *S) getDevicesPage(
	ctx context.Context, spec RunSpec, req *jamf.GetComputersInventoryRequest) (*devicesPage, error) {
	resp, err := s.jamf.GetComputersInventory(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err, "jamf read failed")
	}
	jamfDevs := resp.Results

	partialStop := false
	var highCutTime time.Time

	// Convert Jamf devices to Teleport.
	devs := make([]*devicepb.Device, 0, len(jamfDevs))
	for _, inv := range jamfDevs {
		// Stop partial sync?
		if spec.Mode == mdm.SyncModePartial &&
			inv.General != nil &&
			inv.General.ReportDate.Before(spec.CutTime) {
			//nolint:sloglint // Keys mimic JSON object.
			s.logger.DebugContext(ctx,
				"Stopping partial sync",
				"general.reportDate", inv.General.ReportDate,
			)
			// Signal stop after this round of upserts.
			partialStop = true
			break
		}

		dev, err := computerInventoryToDevice(inv)
		if err != nil {
			s.logger.WarnContext(ctx,
				"Failed to convert Jamf ComputerInventory to Teleport Device",
				"error", err,
			)
			continue
		}
		devs = append(devs, dev)

		// Log device information, but redact sensitive data first.
		if s.logger.Enabled(ctx, slog.LevelDebug) {
			osUsernames := dev.Profile.OsUsernames
			dev.Profile.OsUsernames = []string{"<REDACTED>"}
			//nolint:sloglint // Keys mimic JSON object.
			s.logger.DebugContext(ctx,
				"Syncing Jamf device",
				"general.platform", inv.General.Platform,
				"hardware.serialNumber", inv.Hardware.SerialNumber,
				"id", inv.ID,
				"general.reportDate", inv.General.ReportDate,
				"general.lastContactTime", inv.General.LastContactTime,
				"general.lastEnrolledDate", inv.General.LastEnrolledDate,
				"profile", dev.Profile,
			)
			dev.Profile.OsUsernames = osUsernames
		}

		// Keep tabs of highest-seen reportDate.
		if inv.General != nil && inv.General.ReportDate.After(highCutTime) {
			highCutTime = inv.General.ReportDate
		}
	}

	return &devicesPage{
		jamfTotalCount: resp.TotalCount,
		jamfDevs:       jamfDevs,
		highCutTime:    highCutTime,
		teleportDevs:   devs,
		partialStop:    partialStop,
	}, nil
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
		dev := dev
		id := dev.Profile.GetExternalId()
		if id == "" {
			s.logger.DebugContext(ctx,
				"Marking device without external_id for removal",
				"device", dev,
			)
			markForRemoval(dev)
			continue
		}

		group.Go(func() error {
			// Note: this query is rather conservative and, because of it, we might
			// keep around devices that would otherwise not show up in queries (for
			// example, if further RSQL filters are applied).
			// This seems OK for the moment, as devices can be removed by other means
			// (such as `tctl devices rm`), but it is a point of attention.
			computer, err := s.jamf.GetComputersInventoryByID(groupCtx, &jamf.GetComputersInventoryByIDRequest{
				ID: id,
				Section: []string{
					jamf.SectionGeneral,  // for Platform
					jamf.SectionHardware, // for SerialNumber
				},
			})

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
			case computer.General != nil &&
				platformToOSType(computer.General.Platform) == dev.OsType &&
				computer.Hardware != nil &&
				computer.Hardware.SerialNumber == dev.AssetTag:
				s.logger.DebugContext(ctx,
					"Skipping removal, device found on Jamf",
					"device", dev,
				)
			default:
				// ID matches the wrong device. A leftover from other times?
				s.logger.DebugContext(ctx,
					"Marking mismatched device for removal",
					"computer", computer,
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

func computerInventoryToDevice(c *jamf.ComputerInventory) (*devicepb.Device, error) {
	if c == nil {
		// This is rather unexpected, but let's guard against it anyway.
		return nil, trace.BadParameter("computer inventory is nil")
	}

	// General has the Platform.
	// Hardware has the SerialNumber.
	// That's the bare minimum we need, everything else is DeviceProfile info.
	if c.General == nil || c.Hardware == nil {
		return nil, trace.BadParameter("computer inventory has no general or hardware section")
	}

	osType := platformToOSType(c.General.Platform)
	if osType == devicepb.OSType_OS_TYPE_UNSPECIFIED {
		return nil, trace.BadParameter("unexpected general.platform=%q", c.General.Platform)
	}

	usernames := make([]string, 0, len(c.LocalUserAccounts))
	for _, account := range c.LocalUserAccounts {
		if account != nil && account.Username != "" {
			usernames = append(usernames, account.Username)
		}
	}

	profile := &devicepb.DeviceProfile{
		ModelIdentifier:   c.Hardware.ModelIdentifier,
		OsUsernames:       usernames,
		JamfBinaryVersion: c.General.JamfBinaryVersion,
		ExternalId:        c.ID,
	}
	if c.OperatingSystem != nil {
		profile.OsVersion = c.OperatingSystem.Version
		profile.OsBuild = c.OperatingSystem.Build
		profile.OsBuildSupplemental = c.OperatingSystem.SupplementalBuildVersion
	}

	return &devicepb.Device{
		OsType:   osType,
		AssetTag: c.Hardware.SerialNumber,
		Profile:  profile,
	}, nil
}

func platformToOSType(platform string) devicepb.OSType {
	if strings.EqualFold("Mac", platform) {
		return devicepb.OSType_OS_TYPE_MACOS
	}
	return devicepb.OSType_OS_TYPE_UNSPECIFIED
}

type syncState struct {
	jamfDevices  []*jamf.ComputerInventory
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
			if len(state.jamfDevices) > i {
				d := state.jamfDevices[i]
				if d.General != nil {
					platform = d.General.Platform
				}
				if d.Hardware != nil {
					serialNumber = d.Hardware.SerialNumber
				}
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
