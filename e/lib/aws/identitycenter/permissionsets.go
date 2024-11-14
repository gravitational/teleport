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

func getPermissionSetID(ps *identitycenterv1.PermissionSet) services.PermissionSetID {
	return services.PermissionSetID(ps.GetMetadata().GetName())
}

type psResourceMap map[services.PermissionSetID]*identitycenterv1.PermissionSet

func newPermissionSet(arn arn.ARN, name, description string) *identitycenterv1.PermissionSet {
	return &identitycenterv1.PermissionSet{
		Kind:    types.KindIdentityCenterPermissionSet,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: normalizeResourceName(arn.Resource),
			Labels: map[string]string{
				common.OriginLabel: common.OriginAWSIdentityCenter,
			},
		},
		Spec: &identitycenterv1.PermissionSetSpec{
			Arn:         arn.String(),
			Name:        name,
			Description: description,
		},
	}
}

func (svc *Service) loadPermissionSets(ctx context.Context) (psResourceMap, error) {
	permissionSets := psResourceMap{}
	for ps, err := range allPermissionSets(ctx, svc.icSvc) {
		if err != nil {
			return nil, trace.Wrap(err, "enumerating known permission sets")
		}
		permissionSets[getPermissionSetID(ps)] = ps
	}
	return permissionSets, nil
}

func (svc *Service) reconcilePermissionSets(ctx context.Context, oldPermissionSets, newPermissionSets psResourceMap) (psResourceMap, error) {
	result := maps.Clone(oldPermissionSets)

	for k, old := range oldPermissionSets {
		if new, present := newPermissionSets[k]; present {
			new.Metadata.Revision = old.Metadata.Revision
		}
	}

	createPS := func(ctx context.Context, ps *identitycenterv1.PermissionSet) error {
		createdPS, err := svc.icSvc.CreatePermissionSet(ctx, ps)
		if err != nil {
			return trace.Wrap(err, "creating Identity Center permission set")
		}

		result[getPermissionSetID(createdPS)] = createdPS
		return nil
	}

	updatePS := func(ctx context.Context, ps, _ *identitycenterv1.PermissionSet) error {
		updatedAcct, err := svc.icSvc.UpdatePermissionSet(ctx, ps)
		if err != nil {
			return trace.Wrap(err, "updating Identity Center permission set")
		}
		result[getPermissionSetID(ps)] = updatedAcct
		return nil
	}

	deletePS := func(ctx context.Context, ps *identitycenterv1.PermissionSet) error {
		err := svc.icSvc.DeletePermissionSet(ctx, getPermissionSetID(ps))
		if err != nil {
			return trace.Wrap(err, "updating Identity Center permission set")
		}
		delete(result, getPermissionSetID(ps))
		return nil
	}
	r, err := services.NewGenericReconciler(services.GenericReconcilerConfig[services.PermissionSetID, *identitycenterv1.PermissionSet]{
		Matcher:             func(*identitycenterv1.PermissionSet) bool { return true },
		GetCurrentResources: passThrough(oldPermissionSets),
		GetNewResources:     passThrough(newPermissionSets),
		OnCreate:            createPS,
		OnUpdate:            updatePS,
		OnDelete:            deletePS,
		CompareResources:    comparePermissionSets,
		Logger:              svc.log.With("resource_type", types.KindIdentityCenterPermissionSet),
	})
	if err != nil {
		return nil, trace.Wrap(err, "creating Identity Center permission set reconciler")
	}

	if err := r.Reconcile(ctx); err != nil {
		return nil, trace.Wrap(err, "reconciling Identity Center permission sets")
	}

	return result, nil
}

func comparePermissionSets(a, b *identitycenterv1.PermissionSet) int {
	return services.EqualFromBool(equal.PermissionSetEqual(a, b))
}
