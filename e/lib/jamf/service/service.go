package service

import (
	"context"
	"errors"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/codes"

	"github.com/gravitational/teleport"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/jamf"
	"github.com/gravitational/teleport/e/lib/mdm"
	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/service/servicecfg"
)

const (
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
	logger    log.FieldLogger
	config    *servicecfg.JamfConfig
	devices   devicepb.DeviceTrustServiceClient
	jamf      *jamf.Client
	scheduler *mdm.SyncScheduler[*scheduleEntry]
}

// Opts are creation options from [S].
type Opts struct {
	Clock         clockwork.Clock
	Logger        log.FieldLogger
	Config        *servicecfg.JamfConfig
	DevicesClient devicepb.DeviceTrustServiceClient
	HTTPClient    *http.Client
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

	// Create scheduler (and early detect empty schedules).
	logger := opts.Logger
	scheduler, err := newJamfScheduler(cfg.Spec)
	if errors.Is(err, mdm.ErrScheduleEmpty) {
		logger.Error("Jamf service has an empty sync schedule, aborting")
		return nil, trace.Wrap(err)
	} else if err != nil {
		return nil, trace.Wrap(err)
	}

	// Connect to the Jamf API and verify credentials.
	jamfClient, err := jamf.NewClient(ctx, jamf.ClientOpts{
		Clock:      opts.Clock,
		Logger:     opts.Logger,
		HTTPClient: opts.HTTPClient,
		APIURL:     spec.ApiEndpoint,
		Username:   spec.Username,
		Password:   spec.Password,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s := &S{
		config:    &cfg,
		logger:    logger,
		devices:   opts.DevicesClient,
		jamf:      jamfClient,
		scheduler: scheduler,
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
		delayFn = func() time.Duration {
			n := rand.Int63n(int64(2 * time.Minute))
			return time.Duration(n)
		}
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
	s.logger.Info("Jamf service successfully started")

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
			s.logger.Info("All immediate syncs are done, exiting [exit_on_sync=true]")
			return nil
		}
		// timer is always drained when we get here.
		timer.Reset(offset)

		select {
		case <-timer.C:
			// TODO(codingllama): "Downgrade" initial FULL sync to PARTIAL depending
			//  on device counts?
			e := s.scheduler.Next()
			s.logger.WithFields(log.Fields{
				"Mode":       e.Mode,
				"FilterRSQL": e.Entry.FilterRsql,
				"OnMissing":  e.Entry.OnMissing,
				"CutTime":    e.Entry.cutTime,
			}).Info("Starting sync")

			nextCutTime, err := s.RunOnce(ctx, RunSpec{
				Mode:       e.Mode,
				OnMissing:  e.Entry.onMissing,
				FilterRSQL: e.Entry.FilterRsql,
				CutTime:    e.Entry.cutTime,
			})
			syncsTotal.WithLabelValues(
				strconv.Itoa(int(e.Mode)),
				strconv.FormatBool(err == nil),
			).Inc()
			if err != nil {
				s.logger.WithError(err).Warn("Jamf inventory sync attempt failed")
				continue
			}
			s.logger.Info("Sync complete")

			// Update cut time.
			if nextCutTime.After(e.Entry.cutTime) {
				e.Entry.cutTime = nextCutTime
			}

		case <-ctx.Done():
			timer.Stop()
			s.logger.Info("Exited")
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
}

// RunOnce attempts to perform a single sync operation with the given [RunSpec].
// Returns the cut time for the next sync, acquired from the first computer read
// from Jamf.
func (s *S) RunOnce(ctx context.Context, spec RunSpec) (nextCutTime time.Time, err error) {
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

	// Devices.
	seenCount := 0
	getReq := &jamf.GetComputersInventoryRequest{
		Section: []string{
			jamf.SectionGeneral,
			jamf.SectionHardware,
			jamf.SectionLocalUserAccounts,
			jamf.SectionOperatingSystem,
		},
		PageSize: 200,                // arbitrary
		Sort:     []string{sortByID}, // expected to be more "stable" than timestamps
		Filter:   spec.FilterRSQL,
	}

	// Sort by recent use on PARTIAL syncs.
	// Alternatively we could use an RSQL filter.
	if spec.Mode == mdm.SyncModePartial {
		getReq.Sort = []string{sortByReportDateDesc}
	}

	for {
		// Read inventory from Jamf.
		inventoryResp, err := s.jamf.GetComputersInventory(ctx, getReq)
		if err != nil {
			return time.Time{}, trace.Wrap(err, "jamf read failed")
		}
		jamfDevs := inventoryResp.Results
		jamfDevsLen := len(jamfDevs)

		// Convert Jamf devices to Teleport.
		devs := make([]*devicepb.Device, 0, jamfDevsLen)
		partialStop := false
		for _, inv := range jamfDevs {
			// Stop partial sync?
			if spec.Mode == mdm.SyncModePartial &&
				inv.General != nil &&
				inv.General.ReportDate.Before(spec.CutTime) {
				s.logger.Debugf("Stopping partial sync, general.reportDate=%v", inv.General.ReportDate)
				// Signal stop after this round of upserts.
				partialStop = true
				break
			}

			dev, err := computerInventoryToDevice(inv)
			if err != nil {
				s.logger.WithError(err).Warn("Failed to convert Jamf ComputerInventory to Teleport Device")
				continue
			}
			devs = append(devs, dev)

			// Log device information, but redact sensitive data first.
			if log.IsLevelEnabled(log.DebugLevel) {
				osUsernames := dev.Profile.OsUsernames
				dev.Profile.OsUsernames = []string{"<REDACTED>"}
				s.logger.Debugf(""+
					"Syncing Jamf device %v/%v, "+
					"id=%v, "+
					"general.reportDate=%q, "+
					"general.lastContactTime=%q, "+
					"general.lastEnrolledDate=%q, "+
					"profile={%+v}",
					inv.General.Platform, inv.Hardware.SerialNumber,
					inv.ID,
					inv.General.ReportDate,
					inv.General.LastContactTime,
					inv.General.LastEnrolledDate,
					dev.Profile,
				)
				dev.Profile.OsUsernames = osUsernames
			}

			// Record last contact time of the first device to sync.
			if nextCutTime.IsZero() && inv.General != nil {
				nextCutTime = inv.General.LastContactTime
			}
		}

		// Write devices to Teleport.
		if len(devs) > 0 {
			if err := stream.Send(&devicepb.SyncInventoryRequest{
				Payload: &devicepb.SyncInventoryRequest_DevicesToUpsert{
					DevicesToUpsert: &devicepb.SyncInventoryDevices{
						Devices: devs,
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
				jamfDevices: jamfDevs,
				page:        getReq.Page,
			})
		}

		// Stop device upserts?
		seenCount += jamfDevsLen
		if partialStop || seenCount >= inventoryResp.TotalCount || jamfDevsLen == 0 {
			break
		}
		getReq.Page++
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

	return nextCutTime, nil
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
			s.logger.WithField("Device", dev).Debug("Marking device without external_id for removal")
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
				s.logger.WithField("Device", dev).Debug("Marking unknown device for removal")
				markForRemoval(dev)
			case err != nil: // Unexpected error
				s.logger.
					WithField("Device", dev).
					WithError(err).
					Debug("Skipping removal of device, query failed")
			case computer.General != nil &&
				platformToOSType(computer.General.Platform) == dev.OsType &&
				computer.Hardware != nil &&
				computer.Hardware.SerialNumber == dev.AssetTag:
				s.logger.
					WithField("Device", dev).
					Debug("Skipping removal, device found on Jamf")
			default:
				// ID matches the wrong device. A leftover from other times?
				s.logger.
					WithFields(log.Fields{
						"Computer": computer,
						"Device":   dev,
					}).Debug("Marking mismatched device for removal")
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

			s.logger.WithFields(log.Fields{
				"Code":         status.GetStatus().GetCode(),
				"Message":      status.GetStatus().GetMessage(),
				"DeviceID":     status.GetId(),
				"Platform":     platform,
				"SerialNumber": serialNumber,
				"ExpectDelete": state.expectDelete,
			}).Warn("Failed to sync device")

		case status.GetDeleted():
			deletes++

		default:
			upserts++
		}
	}

	s.logger.WithFields(log.Fields{
		"upserts":  upserts,
		"deletes":  deletes,
		"failures": failures,
	}).Infof("Device sync report, page #%v", state.page)
}
