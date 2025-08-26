package identitycenter

import (
	"context"
	"log/slog"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	provisioningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/provisioning/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/e/lib/aws/identitycenter/calculator"
	"github.com/gravitational/teleport/e/lib/aws/identitycenter/monitor"
	icprov "github.com/gravitational/teleport/e/lib/aws/identitycenter/provisioning"
	icsdk "github.com/gravitational/teleport/e/lib/aws/identitycenter/sdk"
	"github.com/gravitational/teleport/e/lib/provisioning"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

const (
	// IdentityCenterDownstreamID indicates the downstream ID to be used by the
	// Identity Center integration when storing provisioning records.
	IdentityCenterDownstreamID = services.DownstreamID("identitycenter")
)

// Service is the configuration for the Identity Center service
type Service struct {
	accessListSvc              services.AccessLists
	accessListMatchesPredicate provisioning.AccessListPredicate
	accessListSvcCache         provisioning.AccessListsService
	accessRequestSvc           services.AccessRequestGetter
	icClient                   icsdk.Client
	clock                      clockwork.Clock
	icSvc                      services.IdentityCenter
	log                        *slog.Logger
	provisioner                *provisioning.Service
	rolesSvc                   RolesService
	usersSvc                   UsersService
	userMatchesPredicate       func(types.User) bool
	assignmentSyncInterval     time.Duration
	awsSyncInterval            time.Duration
	importConfig               ImportConfig
	pluginStatusSink           common.StatusSink
	pluginsService             pluginsService
	resourceMonitor            *monitor.ResourceMonitor
	principalEventCh           chan *monitor.PrincipalEvent
	assignmentCalculator       *calculator.AssignmentCalculator
	assignmentProvisioner      *icprov.AssignmentProvisioner
	emitter                    apievents.Emitter
	rolesSyncMode              RolesSyncMode
	eventBatchDuration         time.Duration
}

// NewService creates a new Identity Center Service instance from the supplied
// config.
func NewService(config ServiceConfig) (svc *Service, err error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	aclPredicate := makeAccessListAssignmentPredicate(config.RolesSvc)

	assignmentProvisioner, err := icprov.NewAssignmentProvisioner(icprov.ProvisionerConfig{
		Assignment: config.IdentityCenterDataSvc,
		Log:        config.Log.With(teleport.ComponentKey, eteleport.ComponentAWSICAssignmentProvisioner),
		SDKClient:  config.ICClient,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Events from the resource monitor will end up being queued for handling in
	// this buffered channel. The channel is buffered because recalculating and
	// provisioning a principal can take some time (especially while creating or
	// deleting the downstream Identity Center account assignments, which can
	// sometimes take *minutes*), and using a buffered channel gives us a bit of
	// breathing space to keep queueing up PrincipalEvents for later processing
	// without unduly blocking the Resource Monitor.
	principalEventCh := make(chan *monitor.PrincipalEvent, config.EventBufferSize)
	defer func() {
		// Ensure the channel is closed if we return with error after this point
		if err != nil {
			close(principalEventCh)
		}
	}()

	svc = &Service{
		accessListSvc:              config.AccessListsSvc,
		accessListSvcCache:         config.Provisioning.AccessListsSvcCache,
		accessRequestSvc:           config.AccessRequestsSvc,
		clock:                      config.Clock,
		icSvc:                      config.IdentityCenterDataSvc,
		icClient:                   config.ICClient,
		log:                        config.Log,
		rolesSvc:                   config.RolesSvc,
		usersSvc:                   config.UsersSvc,
		userMatchesPredicate:       config.UserPredicate,
		accessListMatchesPredicate: aclPredicate,
		awsSyncInterval:            config.AWSSyncInterval,
		assignmentSyncInterval:     config.AssignmentSyncInterval,
		importConfig:               config.ImportConfig,
		pluginStatusSink:           config.PluginStatusSink,
		pluginsService:             config.PluginsService,
		principalEventCh:           principalEventCh,
		assignmentProvisioner:      assignmentProvisioner,
		emitter:                    config.Emitter,
		rolesSyncMode:              config.RolesSyncMode,
		eventBatchDuration:         config.EventBatchDuration,
	}

	svc.provisioner, err = provisioning.NewService(provisioning.ServiceConfig{
		DownstreamID:              IdentityCenterDownstreamID,
		SCIMClient:                config.Provisioning.SCIMClient,
		StateSvc:                  config.Provisioning.StateSvc,
		StateSvcCache:             config.Provisioning.StateSvcCache,
		UsersCache:                config.Provisioning.UsersSvcCache,
		AccessListsCache:          config.Provisioning.AccessListsSvcCache,
		Locks:                     config.Provisioning.LocksSvc,
		EventsClient:              config.EventsClient,
		UserPredicate:             config.UserPredicate,
		AccessListPredicate:       aclPredicate,
		OnPrincipalProvisioning:   svc.onPrincipalProvisioning,
		OnPrincipalProvisioned:    svc.onPrincipalProvisioned,
		OnPrincipalDeprovisioning: svc.onPrincipalDeprovisioning,
		Logger:                    config.Log.With(teleport.ComponentKey, eteleport.ComponentAWSICPrincipalProvisioner),
		StateRefreshInterval:      config.Provisioning.StateRefreshInterval,
	})
	if err != nil {
		return nil, trace.Wrap(err, "creating provisioner")
	}

	svc.assignmentCalculator, err = calculator.New(calculator.Config{
		AccessRequestsSvc:       config.AccessRequestsSvc,
		Clock:                   config.Clock,
		ExternalIDGetter:        svc.provisioner,
		PrincipalAssignmentsSvc: config.IdentityCenterDataSvc,
		AccountAssignmentCache:  config.IdentityCenterDataSvcCache,
		RolesGetter:             config.RolesSvc,
		Logger:                  config.Log.With(teleport.ComponentKey, eteleport.ComponentAWSICAssignmentCalculator),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	svc.resourceMonitor, err = monitor.New(monitor.Config{
		AccessListsSvcCache: config.Provisioning.AccessListsSvcCache,
		UsersSvcCache:       config.Provisioning.UsersSvcCache,
		Events:              config.EventsClient,
		Clock:               config.Clock,
		Logger:              config.Log.With(teleport.ComponentKey, eteleport.ComponentAWSICResourceMonitor),
		OnEvent:             svc.onResourceMonitorEvent,
	})
	if err != nil {
		return nil, trace.Wrap(err, "creating resource monitor")
	}

	return svc, nil
}

// Run the Identity Center service, blocking until the supplied context is
// canceled
func (svc *Service) Run(ctx context.Context) error {
	if err := svc.pluginStatusSink.Emit(ctx, &types.PluginStatusV1{
		Code: types.PluginStatusCode_RUNNING,
	}); err != nil {
		return trace.Wrap(err)
	}
	if err := svc.maybeImportGroupAndGroupMembers(ctx); err != nil {
		svc.log.ErrorContext(ctx,
			"Group import service exited with error",
			"error", err)
		return trace.Wrap(err)
	}

	go svc.runProvisioner(ctx)
	go svc.runAWSSyncService(ctx)
	go svc.runResourceMonitor(ctx)

	svc.runResourceEventHandler(ctx)

	return nil
}

func (svc *Service) runProvisioner(ctx context.Context) {
	svc.log.InfoContext(ctx, "Starting SCIM provisioning service")
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

// onPrincipalProvisioned is invoked by the user & group provisioning subsystem
// when it has (re-)provisioned a principal.
func (svc *Service) onPrincipalProvisioned(ctx context.Context, principal *provisioningv1.PrincipalState) error {
	log := svc.log.With(
		"principal_id", principal.GetMetadata().GetName())
	log.Log(ctx, logutils.TraceLevel, "Handling SCIM provisioning event")

	event := &monitor.PrincipalEvent{
		Verb: monitor.VerbCalculate,
	}
	switch principal.GetSpec().GetPrincipalType() {
	case provisioningv1.PrincipalType_PRINCIPAL_TYPE_USER:
		username := principal.GetSpec().GetPrincipalId()
		user, err := svc.usersSvc.GetUser(ctx, username, false)
		if err != nil {
			log.ErrorContext(ctx,
				"Failed looking up provisioned user",
				"user", username,
				"error", err.Error())
			return nil
		}
		event.Principal = user

	case provisioningv1.PrincipalType_PRINCIPAL_TYPE_ACCESS_LIST:
		aclName := principal.GetSpec().GetPrincipalId()
		acl, err := svc.accessListSvc.GetAccessList(ctx, aclName)
		if err != nil {
			log.ErrorContext(ctx,
				"Failed looking up provisioned access list",
				"access_list", aclName,
				"error", err.Error())
			return nil
		}
		event.Principal = acl

	default:
		log.ErrorContext(ctx,
			"Unexpected principal type",
			"principal_type", principal.GetSpec().GetPrincipalType())
		return nil
	}
	svc.queueResourceEvent(ctx, event)
	return nil
}

func (svc *Service) onPrincipalProvisioning(ctx context.Context, state *provisioningv1.PrincipalState) error {
	svc.log.DebugContext(ctx, "onPrincipalProvisioning invoked", "principal", state.Metadata.GetName())
	if state.Metadata.Labels[principalDeleteLabel] == principalDeleteModeTeleportOnly {
		return trace.Wrap(provisioning.ErrDoNotProvision)
	}
	return nil
}

func (svc *Service) onPrincipalDeprovisioning(ctx context.Context, state *provisioningv1.PrincipalState) error {
	svc.log.DebugContext(ctx, "onPrincipalDeprovisioning invoked", "principal", state.Metadata.GetName())
	if state.Metadata.Labels[principalDeleteLabel] == principalDeleteModeTeleportOnly {
		return trace.Wrap(provisioning.ErrDoNotProvision)
	}
	return nil
}

func (svc *Service) isTargetedResource(ctx context.Context, resource types.Resource) (bool, error) {
	switch r := resource.(type) {
	case *types.UserV2:
		return svc.userMatchesPredicate(r), nil
	case *accesslist.AccessList:
		return svc.accessListMatchesPredicate(ctx, r)
	default:
		return false, nil
	}
}

// emitSyncEvent emits resource sync audit event.
func (svc *Service) emitSyncEvent(ctx context.Context, in *apievents.AWSICResourceSync, success bool) {
	in.Metadata.Type = events.AWSICResourceSyncSuccessEvent
	in.Metadata.Code = events.AWSICResourceSyncSuccessCode
	in.Status.Success = true
	if !success {
		in.Metadata.Type = events.AWSICResourceSyncFailureEvent
		in.Metadata.Code = events.AWSICResourceSyncFailureCode
		in.Status.Success = false
	}
	if err := svc.emitter.EmitAuditEvent(ctx, in); err != nil {
		svc.log.ErrorContext(ctx, "Failed to emit resource sync event", "error", err)
	}
}

// syncEventUserMessage
func syncEventUserMessage(inMessage string, err error) string {
	const successMessage = "Periodic account, permission set and account assignment sync"
	if err != nil {
		// inMessage will be empty if the sync process erred out during
		// upstream resource fetch or fetched data processing step.
		if inMessage == "" {
			inMessage = successMessage + " failed"
		}
		return inMessage
	}
	return successMessage
}
