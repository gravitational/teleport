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
	icSDK "github.com/gravitational/teleport/e/lib/aws/identitycenter/sdk"
	"github.com/gravitational/teleport/e/lib/provisioning"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
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
	}

	return svc, nil
}

// Run the Identity Center service, blocking until the supplied context is
// canceled
func (svc *Service) Run(ctx context.Context) error {
	svc.log.InfoContext(ctx, "Starting provisioning service...")
	go svc.runProvisioner(ctx)

	svc.log.InfoContext(ctx, "Starting identity center service...")
	if err := svc.awsSyncService(ctx); err != nil {
		svc.log.ErrorContext(ctx,
			"Identity center service exited with error",
			"error", err)
	}

	return nil
}

// awsSyncService is the main sync loop of the service. It periodically updates
// all of Teleports AWS resources (accounts, account assignments, permission
// sets, etc) with AWS.
func (svc *Service) awsSyncService(ctx context.Context) error {
	svc.log.DebugContext(ctx, "Entering AWS IAM IdentityCenter service")
	defer svc.log.DebugContext(ctx, "Exiting AWS IAM IdentityCenter service")

	svc.log.DebugContext(ctx, "Starting AWS sync loop", "sync_interval", svc.awsSyncInterval)
	timer := svc.clock.NewTimer(svc.awsSyncInterval + utils.RandomDuration(10*time.Second))
	defer timer.Stop()

	for {
		if err := svc.synchronize(ctx); err != nil {
			svc.log.ErrorContext(ctx, "Failed synchronizing", "error", err)
		}

		select {
		case <-timer.Chan():
			timer.Reset(svc.awsSyncInterval + utils.RandomDuration(10*time.Second))

		case <-ctx.Done():
			svc.log.InfoContext(ctx, "Exit signaled")
			return nil
		}
	}
}

func (svc *Service) runProvisioner(ctx context.Context) {
	if err := svc.provisioner.Run(ctx); err != nil {
		svc.log.ErrorContext(ctx, "Provisioning service exited with error", "error", err)
	}
}

type localData struct {
	awsAccounts            accountResourceMap
	permissionSets         psResourceMap
	accountAssignments     accountAssignmentMap
	accountAssignmentRoles rolesMap
}

func (svc *Service) loadLocalData(ctx context.Context) (*localData, error) {
	svc.log.DebugContext(ctx, "Loading Teleport Identity Center resources")

	svc.log.DebugContext(ctx, "loading existing accounts")
	awsAccounts, err := svc.loadAccountResources(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "loading existing account records")
	}

	svc.log.DebugContext(ctx, "loading existing permission sets")
	permissionSets, err := svc.loadPermissionSets(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "loading existing permission set records")
	}

	svc.log.DebugContext(ctx, "loading existing account assignment resources")
	accountAssignments, err := svc.loadAccountAssignmentResources(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "loading existing account assignment roles")
	}

	svc.log.DebugContext(ctx, "loading existing account assignment roles")
	accountAssignmentRoles, err := svc.loadAccountAssignmentRoles(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "loading existing account assignment roles")
	}

	data := &localData{
		awsAccounts:            awsAccounts,
		permissionSets:         permissionSets,
		accountAssignments:     accountAssignments,
		accountAssignmentRoles: accountAssignmentRoles,
	}

	return data, nil
}

func (svc *Service) synchronize(ctx context.Context) error {
	svc.log.InfoContext(ctx, "Entering synchronization")
	defer svc.log.InfoContext(ctx, "Exiting synchronization")

	rawData, err := svc.refreshExternalData(ctx)
	if err != nil {
		return trace.Wrap(err, "doing initial data fetch")
	}

	awsResources, err := svc.preProcessExternalData(ctx, rawData)
	if err != nil {
		return trace.Wrap(err, "preprocessing raw aws data")
	}

	teleportResources, err := svc.loadLocalData(ctx)
	if err != nil {
		return trace.Wrap(err, "loading Teleport Identity Center data")
	}

	svc.log.DebugContext(ctx, "reconciling accounts")
	_, err = svc.reconcileAccounts(ctx, teleportResources.awsAccounts, awsResources.accounts)
	if err != nil {
		return trace.Wrap(err, "reconciling initial account list")
	}

	svc.log.DebugContext(ctx, "reconciling permission sets")
	_, err = svc.reconcilePermissionSets(ctx, teleportResources.permissionSets, awsResources.permissionSets)
	if err != nil {
		return trace.Wrap(err, "reconciling initial account list")
	}

	svc.log.DebugContext(ctx, "reconciling account assignment records")
	_, err = svc.reconcileAccountAssignments(ctx, teleportResources.accountAssignments, awsResources.accountAssignments)
	if err != nil {
		return trace.Wrap(err, "reconciling permission set records")
	}

	svc.log.DebugContext(ctx, "reconciling account assignment roles")
	_, err = svc.reconcileAccountAssignmentRoles(ctx, teleportResources.accountAssignmentRoles, awsResources.accountAssignmentRoles)
	if err != nil {
		return trace.Wrap(err, "reconciling  account assignment roles")
	}

	return nil
}

// preProcessedExternalData holds AWS Identity Center *after* we have re-indexes
// and cross-referenced it to make it easier to search.
type preProcessedExternalData struct {
	icInstance             *icSDK.InstanceInfo
	accounts               accountResourceMap
	accountAssignments     accountAssignmentMap
	permissionSets         psResourceMap
	accountAssignmentRoles rolesMap
}

// preProcessExternalData cross-references the fetched data
//   - ensures Accounts have updated permission set lists
func (svc *Service) preProcessExternalData(ctx context.Context, data *externalData) (*preProcessedExternalData, error) {
	svc.log.DebugContext(ctx, "Normalizing Identity Center data")

	// generate all of the the possible permission set bindings
	roles := make(rolesMap, len(data.accounts)*len(data.permissionSets))
	accountAssignments := make(accountAssignmentMap, len(data.accounts)*len(data.permissionSets))
	for _, acct := range data.accounts {
		for _, ps := range acct.Spec.PermissionSetInfo {
			role, err := NewAccountAssignmentRole(acct, ps)
			if err != nil {
				return nil, trace.Wrap(err, "creating account assignment role")
			}
			roles.Store(getAccountID(acct), ps.Arn, role)

			asmt, err := newAccountAssignment(acct, ps)
			if err != nil {
				return nil, trace.Wrap(err, "creating account assignment record")
			}
			accountAssignments[getAccountAssignmentID(asmt)] = asmt
		}
	}

	return &preProcessedExternalData{
		icInstance:             data.icInstance,
		accounts:               data.accounts,
		permissionSets:         data.permissionSets,
		accountAssignments:     accountAssignments,
		accountAssignmentRoles: roles,
	}, nil
}
