package identitycenter

import (
	"context"
	"fmt"
	"maps"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/gravitational/trace"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/common"
	"github.com/gravitational/teleport/e/lib/aws/identitycenter/equal"
	icsdk "github.com/gravitational/teleport/e/lib/aws/identitycenter/sdk"
	"github.com/gravitational/teleport/lib/services"
)

func getAccountID(acct *identitycenterv1.Account) services.IdentityCenterAccountID {
	return services.IdentityCenterAccountID(acct.GetMetadata().GetName())
}

type accountResourceMap map[services.IdentityCenterAccountID]*identitycenterv1.Account

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

	createAccount := func(ctx context.Context, acct *identitycenterv1.Account) error {
		createdAcct, err := svc.icSvc.CreateIdentityCenterAccount(ctx, acct)
		if err != nil {
			return trace.Wrap(err, "creating Identity Center Account record")
		}

		result[getAccountID(createdAcct)] = createdAcct
		return nil
	}

	updateAccount := func(ctx context.Context, newAcct, oldAcct *identitycenterv1.Account) error {
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

	deleteAccount := func(ctx context.Context, acct *identitycenterv1.Account) error {
		err := svc.icSvc.DeleteIdentityCenterAccount(ctx, getAccountID(acct))
		if err != nil {
			return trace.Wrap(err, "deleting Identity Center Account record")
		}
		delete(result, getAccountID(acct))
		return nil
	}

	r, err := services.NewGenericReconciler(services.GenericReconcilerConfig[services.IdentityCenterAccountID, *identitycenterv1.Account]{
		Matcher:             func(*identitycenterv1.Account) bool { return true },
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

func newIdentityCenterAccount(name string, id services.IdentityCenterAccountID, arn arn.ARN, idSource icsdk.IdentityStoreID, ssoRegion string) *identitycenterv1.Account {
	// https://docs.aws.amazon.com/signin/latest/userguide/sign-in-urls-defined.html#access-portal-url
	startURL := "https://%s.awsapps.com/start/#/console?account_id=%s"

	if arn.Partition == "aws-us-gov" {
		// https://docs.aws.amazon.com/govcloud-us/latest/UserGuide/govcloud-sso.html#govcloud-diffs-20
		startURL = "https://start.us-gov-home.awsapps.com/directory/%s#/console?account_id=%s"
	}
	// Note: startURL should also consider "aws-cn" partition but is
	// not handled above, mainly because lack of access to the AWS China
	// test instance.

	return &identitycenterv1.Account{
		Kind:    types.KindIdentityCenterAccount,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name:        string(id),
			Description: name,
			Labels: map[string]string{
				types.OriginLabel:         common.OriginAWSIdentityCenter,
				types.AWSAccountIDLabel:   string(id),
				types.AWSAccountNameLabel: name,
				types.AWSSSORegionLabel:   ssoRegion,
			},
		},
		Spec: &identitycenterv1.AccountSpec{
			Id:   string(id),
			Arn:  arn.String(),
			Name: name,
			// Web UI appends role_name query when launching users with specific permission set.
			StartUrl: fmt.Sprintf(startURL, idSource, id),
		},
		Status: &identitycenterv1.AccountStatus{},
	}
}

func compareAccounts(a, b *identitycenterv1.Account) int {
	return services.EqualFromBool(equal.AccountEqual(a, b))
}
