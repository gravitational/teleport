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
type accountAssignmentMap map[services.IdentityCenterAccountAssignmentID]services.IdentityCenterAccountAssignment

// getAccountAssignmentID provides a shorthand way to get an account assignment
// ID from an Account Assignment record, as fetching it directly is tediously
// verbose.
func getAccountAssignmentID(asmt services.IdentityCenterAccountAssignment) services.IdentityCenterAccountAssignmentID {
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

	createAssignment := func(ctx context.Context, asmt services.IdentityCenterAccountAssignment) error {
		createdAsmt, err := svc.icSvc.CreateAccountAssignment(ctx, asmt)
		if err != nil {
			return trace.Wrap(err, "creating Identity Center Account Assignment record")
		}

		result[getAccountAssignmentID(createdAsmt)] = createdAsmt
		return nil
	}

	updateAssignment := func(ctx context.Context, asmt, old services.IdentityCenterAccountAssignment) error {
		updatedAsmt, err := svc.icSvc.UpdateAccountAssignment(ctx, asmt)
		if err != nil {
			return trace.Wrap(err, "updating Identity Center Account Assignment record")
		}
		result[getAccountAssignmentID(updatedAsmt)] = updatedAsmt
		return nil
	}

	deleteAssignment := func(ctx context.Context, asmt services.IdentityCenterAccountAssignment) error {
		err := svc.icSvc.DeleteAccountAssignment(ctx, getAccountAssignmentID(asmt))
		if err != nil {
			return trace.Wrap(err, "updating Identity Center Account record")
		}
		delete(result, getAccountAssignmentID(asmt))
		return nil
	}

	r, err := services.NewGenericReconciler(services.GenericReconcilerConfig[services.IdentityCenterAccountAssignmentID, services.IdentityCenterAccountAssignment]{
		Matcher:             func(services.IdentityCenterAccountAssignment) bool { return true },
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

func newAccountAssignment(acct services.IdentityCenterAccount, ps *identitycenterv1.PermissionSetInfo) (services.IdentityCenterAccountAssignment, error) {
	asmt := services.IdentityCenterAccountAssignment{
		AccountAssignment: &identitycenterv1.AccountAssignment{
			Kind:    types.KindIdentityCenterAccountAssignment,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: normalizeResourceName(fmt.Sprintf("%s--%s", acct.Spec.Name, ps.Name)),
				Labels: map[string]string{
					types.OriginLabel: common.OriginAWSIdentityCenter,
				},
			},
			Spec: &identitycenterv1.AccountAssignmentSpec{
				Display: fmt.Sprintf("%s on %s", ps.Name, acct.Spec.Name),
				PermissionSet: &identitycenterv1.PermissionSetInfo{
					Arn:  ps.Arn,
					Name: ps.Name,
					Role: ps.Role,
				},
				AccountName: acct.Spec.Name,
				AccountId:   acct.Spec.Id,
			},
		},
	}

	return asmt, nil
}

func compareAccountAssignments(a, b services.IdentityCenterAccountAssignment) int {
	if equal.AccountAssignmentEqual(a.AccountAssignment, b.AccountAssignment) {
		return services.Equal
	}
	return services.Different
}
