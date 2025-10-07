package identitycenter

import (
	"context"
	"maps"
	"slices"
	"time"

	"github.com/gravitational/trace"

	apievents "github.com/gravitational/teleport/api/types/events"
	icsdk "github.com/gravitational/teleport/e/lib/aws/identitycenter/sdk"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

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

type localData struct {
	awsAccounts            accountResourceMap
	permissionSets         psResourceMap
	accountAssignments     accountAssignmentMap
	accountAssignmentRoles accountAssignmentRolesMap
}

func (svc *Service) loadLocalData(ctx context.Context) (*localData, error) {
	svc.log.DebugContext(ctx, "Loading Teleport Identity Center resources")

	svc.log.DebugContext(ctx, "Loading existing accounts")
	awsAccounts, err := svc.loadAccountResources(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "loading existing account records")
	}

	svc.log.DebugContext(ctx, "Loading existing permission sets")
	permissionSets, err := svc.loadPermissionSets(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "loading existing permission set records")
	}

	svc.log.DebugContext(ctx, "Loading existing account assignment resources")
	accountAssignments, err := svc.loadAccountAssignmentResources(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "loading existing account assignment roles")
	}

	svc.log.DebugContext(ctx, "Loading existing account assignment roles")
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

	var syncEvent apievents.AWSICResourceSync
	var err error
	defer func() {
		syncEvent.UserMessage = syncEventUserMessage(syncEvent.UserMessage, err)
		svc.emitSyncEvent(ctx, &syncEvent, err == nil)
	}()

	awsData, err := svc.refreshExternalData(ctx)
	if err != nil {
		return trace.Wrap(err, "fetching AWS resources")
	}

	// Make sure that the assignment provisioner knows which accounts it is
	// allowed to touch.
	svc.assignmentProvisioner.SetKnownAccounts(slices.Collect(maps.Keys(awsData.accounts))...)

	awsResources, err := svc.preProcessExternalData(ctx, awsData)
	if err != nil {
		return trace.Wrap(err, "preprocessing raw aws data")
	}

	teleportResources, err := svc.loadLocalData(ctx)
	if err != nil {
		return trace.Wrap(err, "loading Teleport Identity Center data")
	}

	_, err = svc.reconcileAccounts(ctx, teleportResources.awsAccounts, awsResources.accounts)
	if err != nil {
		syncEvent.UserMessage = "Periodic account sync failed"
		return trace.Wrap(err, "reconciling initial account list")
	}
	syncEvent.TotalAccounts = int32(len(awsResources.accounts))

	_, err = svc.reconcilePermissionSets(ctx, teleportResources.permissionSets, awsResources.permissionSets)
	if err != nil {
		syncEvent.UserMessage = "Periodic permission set sync failed"
		return trace.Wrap(err, "reconciling initial account list")
	}
	syncEvent.TotalPermissionSets = int32(len(awsResources.permissionSets))

	_, err = svc.reconcileAccountAssignments(ctx, teleportResources.accountAssignments, awsResources.accountAssignments)
	if err != nil {
		syncEvent.UserMessage = "Periodic account assignment sync failed"
		return trace.Wrap(err, "reconciling permission set records")
	}
	syncEvent.TotalAccountAssignments = int32(len(awsResources.accountAssignments))

	if svc.rolesSyncMode == RolesSyncModeAll {
		_, err = svc.reconcileAccountAssignmentRoles(ctx, teleportResources.accountAssignmentRoles, awsResources.accountAssignmentRoles)
		if err != nil {
			syncEvent.UserMessage = "Periodic account assignment role reconciliation failed"
			return trace.Wrap(err, "reconciling account assignment roles")
		}
	}

	return nil
}

// preProcessedExternalData holds AWS Identity Center *after* we have re-indexes
// and cross-referenced it to make it easier to search.
type preProcessedExternalData struct {
	icInstance             *icsdk.InstanceInfo
	accounts               accountResourceMap
	accountAssignments     accountAssignmentMap
	permissionSets         psResourceMap
	accountAssignmentRoles accountAssignmentRolesMap
}

// preProcessExternalData cross-references the fetched data
//   - ensures Accounts have updated permission set lists
func (svc *Service) preProcessExternalData(ctx context.Context, data *externalData) (*preProcessedExternalData, error) {
	svc.log.DebugContext(ctx, "Normalizing Identity Center data")

	// generate all of the the possible permission set bindings
	assignmentCount := len(data.accounts) * len(data.permissionSets)
	roles := make(accountAssignmentRolesMap, assignmentCount)
	accountAssignments := make(accountAssignmentMap, assignmentCount)
	for _, acct := range data.accounts {
		for _, ps := range acct.Spec.PermissionSetInfo {
			if svc.rolesSyncMode == RolesSyncModeAll {
				role, err := NewAccountAssignmentRole(acct, ps)
				if err != nil {
					return nil, trace.Wrap(err, "creating account assignment role")
				}
				roles[mkRoleKey(getAccountID(acct), ps.Arn, acct.GetSpec().GetId())] = role
			}

			asmt := newAccountAssignment(acct, ps)
			accountAssignments[getAccountAssignmentID(asmt)] = asmt
			ps.AssignmentId = asmt.GetMetadata().GetName()
		}
	}

	// mark the owning account
	ownerID := services.IdentityCenterAccountID(data.icInstance.OwnerAccountID)
	if acct, ok := data.accounts[ownerID]; ok {
		acct.GetSpec().IsOrganizationOwner = true
	}

	return &preProcessedExternalData{
		icInstance:             data.icInstance,
		accounts:               data.accounts,
		permissionSets:         data.permissionSets,
		accountAssignments:     accountAssignments,
		accountAssignmentRoles: roles,
	}, nil
}
