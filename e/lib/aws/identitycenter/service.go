package identitycenter

import (
	"context"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
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
	accessListSvc      services.AccessLists
	accessListSvcCache provisioning.AccessListsService
	accessRequestSvc   services.AccessRequestGetter
	clock              clockwork.Clock
	icSvc              services.IdentityCenter
	instanceARN        arn.ARN
	instanceRegion     string
	log                *slog.Logger
	provisioner        *provisioning.Service
	rolesSvc           RolesService
	usersSvc           UsersService
}

// NewService creates a new Identity Center Service instance from the supplied
// config.
func NewService(config ServiceConfig) (svc *Service, err error) {
	// TODO(tcsc): add predicates to select which users and access lists should
	//             be provisioned and/or managed by this integration.
	provisioner, err := provisioning.NewService(provisioning.ServiceConfig{
		SCIMClient:       config.Provisioning.SCIMClient,
		DownstreamID:     identityCenterDownstreamID,
		StateSvc:         config.Provisioning.StateSvc,
		UsersCache:       config.Provisioning.UsersSvcCache,
		AccessListsCache: config.Provisioning.AccessListsSvcCache,
		Locks:            config.Provisioning.LocksSvc,
		EventsClient:     config.EventsClient,
		Logger:           config.Log.With(teleport.ComponentKey, Component+":PR"),
	})
	if err != nil {
		return nil, trace.Wrap(err, "creating provisioner")
	}

	svc = &Service{
		accessListSvc:      config.AccessListsSvc,
		accessListSvcCache: config.Provisioning.AccessListsSvcCache,
		accessRequestSvc:   config.AccessRequestsSvc,
		clock:              config.Clock,
		icSvc:              config.IdentityCenterDataSvc,
		instanceARN:        config.AWS.InstanceARN,
		instanceRegion:     config.AWS.Region,
		log:                config.Log,
		provisioner:        provisioner,
		rolesSvc:           config.RolesSvc,
		usersSvc:           config.UsersSvc,
	}

	return svc, nil
}

// Run the Identity Center service, blocking until the supplied context is
// canceled
func (svc *Service) Run(ctx context.Context) error {
	svc.log.DebugContext(ctx, "Entering service main loop")
	defer svc.log.DebugContext(ctx, "Exiting service main loop")

	// TODO(tcsc): Have some way to negotiate which instance of this service
	//             is in control of the AWS Identity Center if there are multiple
	//             instances of auth running. See the the `okta/leader` package
	//             for inspiration

	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	svc.log.InfoContext(ctx, "Starting provisioning service...")
	go func() {
		if err := svc.provisioner.Run(cancelCtx); err != nil {
			svc.log.ErrorContext(ctx, "Provisioning service exited with error", "error", err)
		}
	}()

	// For now, this will just wait to be killed. Actual work will be added in
	// later patches.
	<-ctx.Done()
	return nil
}
