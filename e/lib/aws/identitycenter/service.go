package identitycenter

import (
	"context"
	"log/slog"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/e/lib/aws/identitycenter/monitor"
	icSDK "github.com/gravitational/teleport/e/lib/aws/identitycenter/sdk"
	"github.com/gravitational/teleport/e/lib/provisioning"
	"github.com/gravitational/teleport/lib/services"
)

const (
	Component = "AWS:IC"

	// identityCenterDownstreamID indicates the downstream ID to be used by the
	// Identity Center integration when storing provisioning records
	identityCenterDownstreamID = services.DownstreamID("identitycenter")
)

// Service is the configuration for the Identity Center service
type Service struct {
	accessListSvc       services.AccessLists
	accessListPredicate func(*accesslist.AccessList) bool
	accessListSvcCache  provisioning.AccessListsService
	accessRequestSvc    services.AccessRequestGetter
	icClient            icSDK.Client
	clock               clockwork.Clock
	icSvc               services.IdentityCenter
	log                 *slog.Logger
	provisioner         *provisioning.Service
	rolesSvc            RolesService
	usersSvc            UsersService
	userPredicate       func(types.User) bool
	awsSyncInterval     time.Duration
	resourceMonitor     *monitor.ResourceMonitor
	principalEventCh    chan *monitor.PrincipalEvent
}

// NewService creates a new Identity Center Service instance from the supplied
// config.
func NewService(config ServiceConfig) (svc *Service, err error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	provisioner, err := provisioning.NewService(provisioning.ServiceConfig{
		DownstreamID:        identityCenterDownstreamID,
		SCIMClient:          config.Provisioning.SCIMClient,
		StateSvc:            config.Provisioning.StateSvc,
		StateSvcCache:       config.Provisioning.StateSvcCache,
		UsersCache:          config.Provisioning.UsersSvcCache,
		AccessListsCache:    config.Provisioning.AccessListsSvcCache,
		Locks:               config.Provisioning.LocksSvc,
		EventsClient:        config.EventsClient,
		UserPredicate:       config.UserPredicate,
		AccessListPredicate: config.AccessListPredicate,
		Logger:              config.Log.With(teleport.ComponentKey, Component+":PR"),
	})
	if err != nil {
		return nil, trace.Wrap(err, "creating provisioner")
	}

	resourceMonitor, err := monitor.New(monitor.Config{
		AccessListsSvcCache: config.Provisioning.AccessListsSvcCache,
		Events:              config.EventsClient,
		Clock:               config.Clock,
		Logger:              config.Log.With(teleport.ComponentKey, Component+":RM"),
	})
	if err != nil {
		return nil, trace.Wrap(err, "creating resource monitor")
	}

	// Events from the resource monitor will end up being queued for handling in
	// this buffered channel. The channel is buffered because recalculating and
	// provisioning a principal can take some time (especially while creating or
	// deleting the downstream Identity Center account assignments, which can
	// sometimes take *minutes*), and using a buffered channel gives us a bit of
	// breathing space to keep queueing up PrincipalEvents for later processing
	// without unduly blocking the Resource Monitor.
	principalEventCh := make(chan *monitor.PrincipalEvent, config.EventBufferSize)

	svc = &Service{
		accessListSvc:       config.AccessListsSvc,
		accessListSvcCache:  config.Provisioning.AccessListsSvcCache,
		accessRequestSvc:    config.AccessRequestsSvc,
		clock:               config.Clock,
		icSvc:               config.IdentityCenterDataSvc,
		icClient:            config.IdentityCenterClient,
		log:                 config.Log,
		provisioner:         provisioner,
		rolesSvc:            config.RolesSvc,
		usersSvc:            config.UsersSvc,
		userPredicate:       config.UserPredicate,
		accessListPredicate: config.AccessListPredicate,
		awsSyncInterval:     config.SyncInterval,
		resourceMonitor:     resourceMonitor,
		principalEventCh:    principalEventCh,
	}
	resourceMonitor.SetEventHandler(svc.onResourceMonitorEvent)

	return svc, nil
}

// Run the Identity Center service, blocking until the supplied context is
// canceled
func (svc *Service) Run(ctx context.Context) error {
	go svc.runProvisioner(ctx)
	go svc.runAWSSyncService(ctx)
	go svc.runResourceMonitor(ctx)

	svc.runResourceEventHandler(ctx)

	return nil
}

func (svc *Service) runProvisioner(ctx context.Context) {
	svc.log.DebugContext(ctx, "Starting provisioning service...")
	if err := svc.provisioner.Run(ctx); err != nil {
		svc.log.ErrorContext(ctx, "Provisioning service exited with error",
			"error", err)
	}
}

func (svc *Service) runAWSSyncService(ctx context.Context) {
	svc.log.DebugContext(ctx, "Starting Identity Center sync service...")
	if err := svc.awsSyncService(ctx); err != nil {
		svc.log.ErrorContext(ctx, "Identity Center sync service exited with error",
			"error", err)
	}
}

func (svc *Service) runResourceMonitor(ctx context.Context) {
	svc.resourceMonitor.Watch(ctx)
	// the resource monitor event handler is the only thing that writes to this
	// channel, so it's safe to close it once the monitor has exited
	close(svc.principalEventCh)
}

func (svc *Service) runResourceEventHandler(ctx context.Context) {
	svc.log.DebugContext(ctx, "Starting resource event handler...")
	if err := svc.resourceEventLoop(ctx); err != nil {
		svc.log.ErrorContext(ctx, "Identity Center resource event handler exited with error",
			"error", err)
	}
}

func (svc *Service) queueResourceEvent(ctx context.Context, event *monitor.PrincipalEvent) error {
	select {
	case svc.principalEventCh <- event:
		return nil

	case <-ctx.Done():
		return ctx.Err()
	}
}

func (svc *Service) onResourceMonitorEvent(ctx context.Context, event *monitor.PrincipalEvent) {
	if err := svc.queueResourceEvent(ctx, event); err != nil {
		svc.log.ErrorContext(ctx,
			"Unable to queue resource event. event dropped")
	}
}
