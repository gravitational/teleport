package identitycenter

import (
	"context"
	"maps"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/gravitational/trace"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/common"
	"github.com/gravitational/teleport/e/lib/aws/identitycenter/equal"
	"github.com/gravitational/teleport/lib/services"
)

func getAccountID(acct services.IdentityCenterAccount) services.IdentityCenterAccountID {
	return services.IdentityCenterAccountID(acct.GetMetadata().GetName())
}

type accountResourceMap map[services.IdentityCenterAccountID]services.IdentityCenterAccount

func (svc *Service) loadAccountResources(ctx context.Context) (accountResourceMap, error) {
	accounts := make(accountResourceMap)
	for acct, err := range allAccounts(ctx, svc.icSvc) {
		if err != nil {
			return nil, trace.Wrap(err, "loading Teleport Identity Center account resources")
		}
		accounts[services.IdentityCenterAccountID(acct.Spec.Id)] = acct
	}
	return accounts, nil
}

func (svc *Service) reconcileAccounts(ctx context.Context, oldAccounts, newAccounts accountResourceMap) (accountResourceMap, error) {
	result := maps.Clone(oldAccounts)

	createAccount := func(ctx context.Context, acct services.IdentityCenterAccount) error {
		createdAcct, err := svc.icSvc.CreateIdentityCenterAccount(ctx, acct)
		if err != nil {
			return trace.Wrap(err, "creating Identity Center Account record")
		}

		result[getAccountID(createdAcct)] = createdAcct
		return nil
	}

	updateAccount := func(ctx context.Context, newAcct, oldAcct services.IdentityCenterAccount) error {
		// Copy the revision from the old record to the new so as not to upset
		// the conditional update in the Identity Center data service
		newAcct.Metadata.Revision = oldAcct.Metadata.Revision

		updatedAcct, err := svc.icSvc.UpdateIdentityCenterAccount(ctx, newAcct)
		if err != nil {
			return trace.Wrap(err, "updating Identity Center Account record")
		}
		result[getAccountID(updatedAcct)] = updatedAcct
		return nil
	}

	deleteAccount := func(ctx context.Context, acct services.IdentityCenterAccount) error {
		err := svc.icSvc.DeleteIdentityCenterAccount(ctx, getAccountID(acct))
		if err != nil {
			return trace.Wrap(err, "updating Identity Center Account record")
		}
		delete(result, getAccountID(acct))
		return nil
	}

	r, err := services.NewGenericReconciler(services.GenericReconcilerConfig[services.IdentityCenterAccountID, services.IdentityCenterAccount]{
		Matcher:             func(services.IdentityCenterAccount) bool { return true },
		GetCurrentResources: passThrough(oldAccounts),
		GetNewResources:     passThrough(newAccounts),
		OnCreate:            createAccount,
		OnUpdate:            updateAccount,
		OnDelete:            deleteAccount,
		CompareResources:    compareAccounts,
		Logger:              svc.log.With("resource_type", types.KindIdentityCenterAccount),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := r.Reconcile(ctx); err != nil {
		return nil, trace.Wrap(err, "reconciling Identity Center Accounts")
	}

	return result, nil
}

func newIdentityCenterAccount(name string, id services.IdentityCenterAccountID, arn arn.ARN) services.IdentityCenterAccount {
	return services.IdentityCenterAccount{
		Account: &identitycenterv1.Account{
			Kind:    types.KindIdentityCenterAccount,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name:        string(id),
				Description: name,
				Labels: map[string]string{
					types.OriginLabel: common.OriginAWSIdentityCenter,
				},
			},
			Spec: &identitycenterv1.AccountSpec{
				Id:   string(id),
				Arn:  arn.String(),
				Name: name,
			},
			Status: &identitycenterv1.AccountStatus{},
		},
	}
}

func compareAccounts(a, b services.IdentityCenterAccount) int {
	return services.EqualFromBool(equal.AccountEqual(a.Account, b.Account))
}
