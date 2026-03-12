package identitycenter

import (
	"context"
	"fmt"
	"maps"

	"github.com/gravitational/trace"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/common"
	"github.com/gravitational/teleport/e/lib/aws/identitycenter/equal"
	"github.com/gravitational/teleport/lib/services"
)

// accountAssignmentMap defines a mapping of account assignment IDs to the
// corresponding account assignment. This is used when reconciling the resources
// Teleport already knows about with fresh values from AWS.
type accountAssignmentMap map[services.IdentityCenterAccountAssignmentID]*identitycenterv1.AccountAssignment

// getAccountAssignmentID provides a shorthand way to get an account assignment
// ID from an Account Assignment record, as fetching it directly is tediously
// verbose.
func getAccountAssignmentID(asmt *identitycenterv1.AccountAssignment) services.IdentityCenterAccountAssignmentID {
	return services.IdentityCenterAccountAssignmentID(asmt.Metadata.Name)
}

// loadAccountAssignmentResources loads all of the Account Assignment records
// known to Teleport from the cluster backend.
func (svc *Service) loadAccountAssignmentResources(ctx context.Context) (accountAssignmentMap, error) {
	accountAssignments := make(accountAssignmentMap)
	for asmt, err := range allAccountAssignments(ctx, svc.icSvc) {
		if err != nil {
			return nil, trace.Wrap(err, "loading Teleport Identity Center account assignment resources")
		}
		accountAssignments[getAccountAssignmentID(asmt)] = asmt

	}
	return accountAssignments, nil
}

func (svc *Service) reconcileAccountAssignments(ctx context.Context, oldAssignments, newAssignments accountAssignmentMap) (accountAssignmentMap, error) {
	result := maps.Clone(oldAssignments)

	for k, old := range oldAssignments {
		if new, present := newAssignments[k]; present {
			new.Metadata.Revision = old.Metadata.Revision
		}
	}

	createAssignment := func(ctx context.Context, asmt *identitycenterv1.AccountAssignment) error {
		createdAsmt, err := svc.icSvc.CreateIdentityCenterAccountAssignment(ctx, asmt)
		if err != nil {
			return trace.Wrap(err, "creating Identity Center Account Assignment record")
		}

		result[getAccountAssignmentID(createdAsmt)] = createdAsmt
		return nil
	}

	updateAssignment := func(ctx context.Context, asmt, old *identitycenterv1.AccountAssignment) error {
		updatedAsmt, err := svc.icSvc.UpdateIdentityCenterAccountAssignment(ctx, asmt)
		if err != nil {
			return trace.Wrap(err, "updating Identity Center Account Assignment record")
		}
		result[getAccountAssignmentID(updatedAsmt)] = updatedAsmt
		return nil
	}

	deleteAssignment := func(ctx context.Context, asmt *identitycenterv1.AccountAssignment) error {
		err := svc.icSvc.DeleteAccountAssignment(ctx, getAccountAssignmentID(asmt))
		if err != nil {
			return trace.Wrap(err, "updating Identity Center Account record")
		}
		delete(result, getAccountAssignmentID(asmt))
		return nil
	}

	r, err := services.NewGenericReconciler(services.GenericReconcilerConfig[services.IdentityCenterAccountAssignmentID, *identitycenterv1.AccountAssignment]{
		Matcher:             func(*identitycenterv1.AccountAssignment) bool { return true },
		GetCurrentResources: passThrough(oldAssignments),
		GetNewResources:     passThrough(newAssignments),
		OnCreate:            createAssignment,
		OnUpdate:            updateAssignment,
		OnDelete:            deleteAssignment,
		CompareResources:    compareAccountAssignments,
		Logger:              svc.log.With("resource_type", types.KindIdentityCenterAccountAssignment),
	})
	if err != nil {
		return nil, trace.Wrap(err, "creating Identity Center Account Assignment reconciler")
	}

	if err := r.Reconcile(ctx); err != nil {
		return nil, trace.Wrap(err, "reconciling Identity Center Assignment Accounts")
	}

	return result, nil
}

func newAccountAssignment(acct *identitycenterv1.Account, ps *identitycenterv1.PermissionSetInfo) *identitycenterv1.AccountAssignment {
	return &identitycenterv1.AccountAssignment{
		Kind:    types.KindIdentityCenterAccountAssignment,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: normalizeResourceName(fmt.Sprintf("%s--%s", acct.GetSpec().GetId(), ps.Name)),
			Labels: map[string]string{
				types.OriginLabel:         common.OriginAWSIdentityCenter,
				types.AWSAccountIDLabel:   acct.GetSpec().GetId(),
				types.AWSAccountNameLabel: acct.GetSpec().GetName(),
			},
		},
		Spec: &identitycenterv1.AccountAssignmentSpec{
			Display: fmt.Sprintf("%q on %q", ps.Name, acct.Spec.Name),
			PermissionSet: &identitycenterv1.PermissionSetInfo{
				Arn:  ps.Arn,
				Name: ps.Name,
				Role: ps.Role,
			},
			AccountName: acct.Spec.Name,
			AccountId:   acct.Spec.Id,
		},
	}
}

func compareAccountAssignments(a, b *identitycenterv1.AccountAssignment) int {
	return services.EqualFromBool(equal.AccountAssignmentEqual(a, b))
}
