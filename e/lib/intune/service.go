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
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/codes"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/intune/api"
	"github.com/gravitational/teleport/e/lib/mdmsync"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/msgraph"
	"github.com/gravitational/teleport/lib/observability/metrics"
)

// Config contains parameters needed by [Service].
type Config struct {
	APIConfig     APIConfig
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

// APIConfig are parameters required by the Intune API itself.
type APIConfig struct {
	// AppCredentials are credentials used to authenticate with the API.
	AppCredentials api.AppCredentials
	// LoginEndpoint points to one of the national deployments of Microsoft Entra ID.
	// Optional, defaults to "https://login.microsoftonline.com".
	//
	// https://learn.microsoft.com/en-us/graph/deployments
	LoginEndpoint string
	// GraphEndpoint points to one of the national deployments of Microsoft Graph.
	// Optional, defaults to "https://graph.microsoft.com".
	//
	// https://learn.microsoft.com/en-us/graph/deployments
	GraphEndpoint string
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
	case config.HTTPClient == nil:
		// Token provider blows up if it receives a nil Transport in [policy.ClientOptions].
		httpClient, err := defaults.HTTPClient()
		if err != nil {
			return nil, trace.Wrap(err, "getting default HTTP client")
		}
		httpClient.Timeout = apidefaults.DefaultIOTimeout
		config.HTTPClient = httpClient
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
	scheduler, err := mdmsync.New(schedulerEntries, delayFn, func(e *scheduleEntry) mdmsync.EntryInfo {
		return mdmsync.EntryInfo{SyncPeriodPartial: e.SyncPeriodPartial, SyncPeriodFull: e.SyncPeriodFull}
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Connect to the Graph API and verify credentials.
	if err := config.APIConfig.AppCredentials.Validate(); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := types.ValidateMSGraphAndLoginEndpoints(config.APIConfig.LoginEndpoint, config.APIConfig.GraphEndpoint); err != nil {
		return nil, trace.Wrap(err)
	}
	creds := config.APIConfig.AppCredentials
	tokenProvider, err := azidentity.NewClientSecretCredential(creds.Tenant, creds.ClientID, creds.ClientSecret,
		&azidentity.ClientSecretCredentialOptions{
			ClientOptions: policy.ClientOptions{
				Transport: config.HTTPClient,
				Cloud: cloud.Configuration{
					ActiveDirectoryAuthorityHost: config.APIConfig.LoginEndpoint,
				},
			},
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	client, err := msgraph.NewClient(msgraph.Config{
		TokenProvider: tokenProvider,
		HTTPClient:    config.HTTPClient,
		Clock:         config.Clock,
		GraphEndpoint: config.APIConfig.GraphEndpoint,
		Logger:        config.Logger,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := VerifyCredentials(ctx, client); err != nil {
		return nil, trace.Wrap(err)
	}

	s := &Service{
		cfg:       config,
		msgraph:   client,
		scheduler: scheduler,
	}
	return s, nil
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
	msgraph   *msgraph.Client
	scheduler *mdmsync.Scheduler[*scheduleEntry]
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

			s.cfg.StatusSink.Emit(ctx, &types.PluginStatusV1{
				Code: types.PluginStatusCode_RUNNING, LastSyncTime: s.cfg.Clock.Now()})

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
	mode                   mdmsync.SyncMode
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
	if err := stream.Send(devicepb.SyncInventoryRequest_builder{
		Start: devicepb.SyncInventoryStart_builder{
			Source: devicepb.DeviceSource_builder{
				Name:   "intune",
				Origin: devicepb.DeviceOrigin_DEVICE_ORIGIN_INTUNE,
			}.Build(),
			TrackMissingDevices: spec.mode == mdmsync.SyncModeFull,
			OsTypes: []devicepb.OSType{
				devicepb.OSType_OS_TYPE_MACOS,
				devicepb.OSType_OS_TYPE_WINDOWS,
				devicepb.OSType_OS_TYPE_LINUX,
				devicepb.OSType_OS_TYPE_IOS,
				devicepb.OSType_OS_TYPE_IPADOS,
			},
		}.Build(),
	}.Build()); err != nil {
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
		var iterateOpts []msgraph.IterateOpt
		if spec.mode == mdmsync.SyncModePartial {
			iterateOpts = append(iterateOpts, msgraph.WithLastSyncDateTimeGt(spec.deviceLastSyncDateTime))
		}

		err := s.msgraph.IterateManagedDevicePages(ctx, func(mds []*msgraph.ManagedDevice) bool {
			page, err := s.processDevicesPage(ctx, mds)
			if err != nil {
				select {
				case <-ctx.Done():
				case devicesC <- devicesResp{err: trace.Wrap(err)}:
				}
				return false
			}

			if len(page.teleportDevices) > 0 {
				select {
				case <-ctx.Done():
					return false
				case devicesC <- devicesResp{page: page}:
				}
			}

			if page.deviceLastSyncDateTime.After(nextDeviceLastSyncDateTime) {
				nextDeviceLastSyncDateTime = page.deviceLastSyncDateTime
			}

			return true
		}, iterateOpts...)
		if err != nil {
			select {
			case <-ctx.Done():
			case devicesC <- devicesResp{err: trace.Wrap(err)}:
			}
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

		if err := stream.Send(devicepb.SyncInventoryRequest_builder{
			DevicesToUpsert: devicepb.SyncInventoryDevices_builder{
				Devices: devicesResp.page.teleportDevices,
			}.Build(),
		}.Build()); err != nil {
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
	if err := stream.Send(devicepb.SyncInventoryRequest_builder{
		End: &devicepb.SyncInventoryEnd{},
	}.Build()); err != nil {
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

		// At the end of a full sync, the auth server reports to the Intune integration all devices from
		// Intune that are in the Teleport inventory but weren't observed during this sync. The job of
		// the Intune integration is to verify that those devices are indeed not present in Intune.
		devicesToRemove, err := s.confirmMissingDevices(ctx, missingDevicesResp.GetMissingDevices().GetDevices())
		if err != nil {
			return time.Time{}, trace.Wrap(err, "confirming missing devices in Intune")
		}

		if err := stream.Send(devicepb.SyncInventoryRequest_builder{
			DevicesToRemove: devicepb.SyncInventoryDevices_builder{
				Devices: devicesToRemove,
			}.Build(),
		}.Build()); err != nil {
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
	intuneDevices []*msgraph.ManagedDevice
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

const (
	// deviceRegistrationStateRegistered is DeviceRegistrationState value of [msgraph.ManagedDevice]
	// set after the device is fully enrolled into Intune.
	deviceRegistrationStateRegistered = "registered"
)

func (s *Service) processDevicesPage(ctx context.Context, intuneDevices []*msgraph.ManagedDevice) (*devicesPage, error) {
	var highLastSyncDateTime time.Time
	teleportToIntuneIdx := make(map[int]int)
	devices := make([]*devicepb.Device, 0, len(intuneDevices))
	for intuneIdx, intuneDevice := range intuneDevices {
		if intuneDevice == nil {
			s.cfg.Logger.DebugContext(ctx, "Skipping nil device")
			continue
		}

		// Purposefully not using slog.Logger.With as it's not free and the logger is always called just
		// once per device.
		logGroup := slog.Group("intune_device", "id", intuneDevice.ID, "serial_number", intuneDevice.SerialNumber)

		if intuneDevice.DeviceRegistrationState != deviceRegistrationStateRegistered {
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
		if device.GetOsType() == devicepb.OSType_OS_TYPE_UNSPECIFIED {
			// Emit a debug message instead of a warning. This scenario is expected to happen often enough
			// that we don't want to spam logs with warnings. A customer might have many devices in their
			// inventory that we don't support, e.g., Android devices. Intune API offers no way to filter
			// by the OS.
			s.cfg.Logger.DebugContext(ctx, "Skipping device due to unsupported operating system",
				logGroup,
				slog.String("operating_system", intuneDevice.OperatingSystem),
				slog.String("model", intuneDevice.Model),
			)
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
			slog.String("operating_system", intuneDevice.OperatingSystem),
			slog.String("model", intuneDevice.Model),
			slog.String("os_version", intuneDevice.OSVersion),
			slog.Any("profile", device.GetProfile()),
		)
	}

	return &devicesPage{
		intuneDevices:          intuneDevices,
		teleportDevices:        devices,
		teleportToIntuneIdx:    teleportToIntuneIdx,
		deviceLastSyncDateTime: highLastSyncDateTime,
	}, nil
}

// confirmMissingDevices takes missing Teleport devices as reported by the auth server and queries
// Intune for them one by one to confirm whether a Teleport device indeed doesn't have an Intune
// counterpart.
func (s *Service) confirmMissingDevices(ctx context.Context, missingDevices []*devicepb.Device) ([]*devicepb.Device, error) {
	group, groupCtx := errgroup.WithContext(ctx)
	const intuneGroupLimit = 8 // Arbitrary. Not too many, not too few.
	group.SetLimit(intuneGroupLimit)

	var devicesMux sync.Mutex // guards devicesToRemove
	devicesToRemove := make([]*devicepb.Device, 0, len(missingDevices))
	markForRemoval := func(d *devicepb.Device) {
		devicesMux.Lock()
		devicesToRemove = append(devicesToRemove, d)
		devicesMux.Unlock()
	}

	// Concurrently query devices in Intune.
	// We are looking for either confirmation that the device doesn't exist, or an existing but
	// mismatched device.
	for _, missingDevice := range missingDevices {
		id := missingDevice.GetProfile().GetExternalId()
		if id == "" {
			s.cfg.Logger.DebugContext(ctx,
				"Marking device without external_id for removal",
				"device", missingDevice,
			)
			markForRemoval(missingDevice)
			continue
		}

		group.Go(func() error {
			graphError := &msgraph.GraphError{}
			// Note: this gets a device by ID, but during syncs we skip devices where device registration
			// state is different than "registered". This means we might keep around devices which
			// deviceRegistrationState in Intune has changed since they were initially added to Teleport,
			// e.g., because they were reset in Intune and are yet to be assigned to a new user. This
			// seems OK for the moment, as devices can be removed by other means (such as `tctl devices
			// rm`), but it is a point of attention.
			intuneDevice, err := s.msgraph.GetManagedDevice(groupCtx, id)
			switch {
			case errors.As(err, &graphError) && graphError.StatusCode == http.StatusNotFound:
				s.cfg.Logger.DebugContext(ctx, "Device not found in Intune, marking for removal", "device", missingDevice)
				markForRemoval(missingDevice)

			case err != nil:
				// Unexpected error
				s.cfg.Logger.DebugContext(ctx, "Skipping removal of device, query failed",
					"error", err, "device", missingDevice)

			case operatingSystemToOSType(intuneDevice.OperatingSystem, intuneDevice.Model) == missingDevice.GetOsType() &&
				intuneDevice.SerialNumber == missingDevice.GetAssetTag():
				s.cfg.Logger.DebugContext(ctx, "Skipping removal, device found in Intune", "device", missingDevice)

			default:
				// ID matches the wrong device.
				s.cfg.Logger.DebugContext(ctx, "Marking mismatched device for removal",
					"intune_device", intuneDevice, "device", missingDevice)
				markForRemoval(missingDevice)
			}

			return nil
		})
	}

	if err := group.Wait(); err != nil {
		return nil, trace.Wrap(err)
	}

	return devicesToRemove, nil
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
			var operatingSystem, model, serialNumber, intuneID string
			if page != nil {
				syncType = "upsert"
				if intuneIdx, found := page.teleportToIntuneIdx[teleportIdx]; found && intuneIdx < len(page.intuneDevices) {
					intuneDevice := page.intuneDevices[intuneIdx]
					intuneID = intuneDevice.ID
					operatingSystem = intuneDevice.OperatingSystem
					model = intuneDevice.Model
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
				"model", model,
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

// VerifyCredentials checks if the given credentials can be used to authenticate to the Graph API
// and if they're authorized to list managed devices.
func VerifyCredentials(ctx context.Context, client *msgraph.Client) error {
	err := client.VerifyCredentials(ctx, func(ctx context.Context, client *msgraph.Client) error {
		return client.IterateManagedDevicePages(ctx, func(_ []*msgraph.ManagedDevice) bool {
			return false
		}, msgraph.WithTop(1))
	})

	if errors.Is(err, msgraph.ErrClientUnauthorized) {
		return trace.Wrap(err,
			"does the application have the DeviceManagementManagedDevices.Read.All permission "+
				"and has it been granted by an administrator?")
	}
	return trace.Wrap(err)
}
