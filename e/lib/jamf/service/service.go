package service

import (
	"context"
	"errors"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/jamf"
	"github.com/gravitational/teleport/e/lib/mdm"
	"github.com/gravitational/teleport/lib/service/servicecfg"
)

const lastContactTimeDesc = "general.lastContactTime:desc"

var defaultInventory = []*types.JamfInventoryEntry{
	// https://github.com/gravitational/teleport.e/blob/master/rfd/0007e-device-trust-mdm-integration.md#jamf-inventory-sync
	{
		FilterRsql:        `general.remoteManagement.managed==true`,
		SyncPeriodPartial: types.Duration(6 * time.Hour),
		SyncPeriodFull:    types.Duration(24 * time.Hour),
		OnMissing:         "DELETE",
	},
}

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

	// NewJamfClient is the function used to create a new [jamf.Client].
	// Defaults to [jamf.NewClient].
	NewJamfClient func(context.Context, jamf.ClientOpts) (*jamf.Client, error)
}

// New creates a new [S] instance.
// `ctx` is used to perform initial validations against the Jamf API.
func New(ctx context.Context, opts Opts) (*S, error) {
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
	newJamfClient := opts.NewJamfClient
	if newJamfClient == nil {
		newJamfClient = jamf.NewClient
	}
	jamfClient, err := newJamfClient(ctx, jamf.ClientOpts{
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

	OnMissingAction devicepb.SyncInventoryDeviceAction
	CutTime         time.Time
}

func newJamfScheduler(spec *types.JamfSpecV1) (*mdm.SyncScheduler[*scheduleEntry], error) {
	parsedInv := make([]*scheduleEntry, len(spec.Inventory))
	for i, e := range spec.Inventory {
		onMissing := devicepb.SyncInventoryDeviceAction_SYNC_INVENTORY_DEVICE_ACTION_NOOP
		if e.OnMissing == types.JamfOnMissingDelete {
			onMissing = devicepb.SyncInventoryDeviceAction_SYNC_INVENTORY_DEVICE_ACTION_DELETE
		}
		parsedInv[i] = &scheduleEntry{
			JamfInventoryEntry: e,
			OnMissingAction:    onMissing,
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
			Sort:     []string{lastContactTimeDesc}, // might as well check
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
			s.logger.Debug("All immediate syncs are done, exiting [exit_on_sync=true]")
			return nil
		}
		// timer is always drained when we get here.
		timer.Reset(offset)

		select {
		case <-timer.C:
			e := s.scheduler.Next()
			nextCutTime, err := s.RunOnce(ctx, RunSpec{
				Mode:            e.Mode,
				OnMissingAction: e.Entry.OnMissingAction,
				FilterRSQL:      e.Entry.FilterRsql,
				CutTime:         e.Entry.CutTime,
			})
			if err != nil {
				s.logger.WithError(err).Warn("Jamf inventory sync attempt failed")
				continue
			}
			// Update cut time.
			if nextCutTime.After(e.Entry.CutTime) {
				e.Entry.CutTime = nextCutTime
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
	Mode            devicepb.SyncInventoryMode
	OnMissingAction devicepb.SyncInventoryDeviceAction
	FilterRSQL      string
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
				Mode:            spec.Mode,
				OnMissingAction: spec.OnMissingAction,
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

	// TODO(codingllama): Add a delete-confirmation step to the SyncInventory
	//  stream and apply it here.
	//  Trusting the external pagination to never have gaps could lead to deletion
	//  of legitimate devices.

	// Devices.
	failedMDMRead := false
	seenCount := 0
	getReq := &jamf.GetComputersInventoryRequest{
		Section: []string{
			jamf.SectionGeneral,
			jamf.SectionHardware,
			jamf.SectionLocalUserAccounts,
			jamf.SectionOperatingSystem,
		},
		PageSize: 200, // arbitrary
		// TODO(codingllama): Change filters according to sync type.
		//  IDs are likely less liable to have gaps in FULL syncs.
		Sort:   []string{lastContactTimeDesc}, // required for partial syncs
		Filter: spec.FilterRSQL,
	}
	for {
		// Read inventory from Jamf.
		inventoryResp, err := s.jamf.GetComputersInventory(ctx, getReq)
		if err != nil {
			s.logger.WithError(err).Warn("Jamf read failed, aborting current sync")
			failedMDMRead = true
			break
		}
		jamfDevs := inventoryResp.Results
		jamfDevsLen := len(jamfDevs)

		// Convert Jamf devices to Teleport.
		devs := make([]*devicepb.Device, 0, jamfDevsLen)
		partialStop := false
		for _, inv := range jamfDevs {
			// Stop partial sync?
			if spec.Mode == devicepb.SyncInventoryMode_SYNC_INVENTORY_MODE_PARTIAL &&
				inv.General != nil &&
				inv.General.LastContactTime.Before(spec.CutTime) {
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
			End: &devicepb.SyncInventoryEnd{
				ExternalSyncSuccessful: !failedMDMRead,
			},
		},
	}); err != nil {
		return time.Time{}, trace.Wrap(err, "end: Send")
	}
	// Receive deletion reports until EOF.
	for page := 0; true; page++ {
		resp, err = stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return time.Time{}, trace.Wrap(err, "end: Recv")
		}
		s.logSyncResult(resp.GetResult(), syncState{
			page:      page,
			isEndPage: true,
		})
	}

	// Did we complete the entire sync successfully?
	if failedMDMRead {
		return time.Time{}, errors.New("sync partially successful, MDM reads failed")
	}
	return nextCutTime, nil
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

	var osType devicepb.OSType
	switch {
	case strings.EqualFold("mac", c.General.Platform):
		osType = devicepb.OSType_OS_TYPE_MACOS
	// TODO(codingllama): Confirm linux and windows platform string.
	case strings.EqualFold("linux", c.General.Platform):
		osType = devicepb.OSType_OS_TYPE_LINUX
	case strings.EqualFold("windows", c.General.Platform):
		osType = devicepb.OSType_OS_TYPE_WINDOWS
	default:
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

type syncState struct {
	jamfDevices []*jamf.ComputerInventory
	page        int
	isEndPage   bool
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
				"ExpectDelete": state.isEndPage,
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
