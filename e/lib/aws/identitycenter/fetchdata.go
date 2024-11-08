package identitycenter

import (
	"context"
	"maps"
	"slices"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/gravitational/trace"

	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	icsdk "github.com/gravitational/teleport/e/lib/aws/identitycenter/sdk"
	"github.com/gravitational/teleport/e/lib/provisioning"
	"github.com/gravitational/teleport/lib/services"
)

// ExternalID aliases provisioning.ExternalID to reduce clutter
type ExternalID = provisioning.ExternalID

type PrincipalMap map[ExternalID]*identitycenterv1.PrincipalAssignment

type externalData struct {
	icInstance     *icsdk.InstanceInfo
	accounts       accountResourceMap
	permissionSets psResourceMap
}

// refreshExternalData pulls the current state of users, groups, permission sets
// and so on from AWS.
//
// Many of the tasks it performs are orthogonal, so there is plenty of scope for
// speeding it up with parallel execution, but let's not optimize prematurely.
func (svc *Service) refreshExternalData(ctx context.Context) (*externalData, error) {

	icInstance, err := svc.icClient.DescribeInstance(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accounts, err := svc.fetchAccounts(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "fetching AWS Identity Center accounts")
	}

	permissionSets, err := svc.fetchPermissionSets(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "fetching AWS Identity Center permission sets")
	}

	result := &externalData{
		icInstance:     icInstance,
		accounts:       accounts,
		permissionSets: permissionSets,
	}

	// Ensure all accounts have the available permission sets listed. The
	// permission set info will be sorted by permission set id to ensure
	// deterministic ordering.
	sortedPermissionSetIDs := slices.Collect(maps.Keys(permissionSets))
	slices.Sort(sortedPermissionSetIDs)
	for _, acct := range accounts {
		pss := make([]*identitycenterv1.PermissionSetInfo, 0, len(permissionSets))

		for _, psID := range sortedPermissionSetIDs {
			ps := permissionSets[psID]
			pss = append(pss, &identitycenterv1.PermissionSetInfo{
				Name: ps.Spec.Name,
				Arn:  ps.Spec.Arn,
			})
		}
		acct.Account.Spec.PermissionSetInfo = pss
	}

	return result, nil
}

func (svc *Service) fetchAccounts(ctx context.Context) (accountResourceMap, error) {
	svc.log.DebugContext(ctx, "listing Accounts...")
	accountList, err := svc.icClient.ListAccounts(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "fetching accounts")
	}
	accounts := make(accountResourceMap, len(accountList))
	for _, src := range accountList {
		accountArn, err := arn.Parse(src.ARN)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		acct := newIdentityCenterAccount(src.Name, services.IdentityCenterAccountID(src.ID), accountArn)
		accounts[getAccountID(acct)] = acct
	}
	return accounts, err
}

func (svc *Service) fetchPermissionSets(ctx context.Context) (psResourceMap, error) {
	svc.log.DebugContext(ctx, "listing Permission Sets...")
	psList, err := svc.icClient.ListPermissionSets(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "listing permission sets")
	}
	permissionSets := make(psResourceMap, len(psList))
	for _, src := range psList {
		psARN, err := arn.Parse(src.ARN)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		ps := newPermissionSet(psARN, src.Name, src.Description)
		permissionSets[getPermissionSetID(ps)] = ps
	}
	return permissionSets, nil
}
