package identitycenter

import (
	"context"
	"fmt"
	"maps"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/common"
	"github.com/gravitational/teleport/lib/services"
)

const (
	RoleSubKindIdentityCenter = "aws_identity_center"
)

type accountAssignmentRoleKey struct {
	account       services.IdentityCenterAccountID
	permissionSet string
}

type accountAssignmentRolesMap map[accountAssignmentRoleKey]*types.RoleV6

func mkRoleKey(account services.IdentityCenterAccountID, permissionSetARN string) accountAssignmentRoleKey {
	return accountAssignmentRoleKey{
		account:       account,
		permissionSet: permissionSetARN,
	}
}

func mkRoleKeyForRole(role *types.RoleV6) (accountAssignmentRoleKey, error) {
	if role == nil {
		return accountAssignmentRoleKey{}, trace.BadParameter("role may not be nil")
	}

	if len(role.Spec.Allow.AccountAssignments) != 1 {
		return accountAssignmentRoleKey{}, trace.BadParameter("role must have a single account assignment")
	}

	asmt := &role.Spec.Allow.AccountAssignments[0]
	return mkRoleKey(services.IdentityCenterAccountID(asmt.Account), asmt.PermissionSet), nil
}

func (svc *Service) loadAccountAssignmentRoles(ctx context.Context) (accountAssignmentRolesMap, error) {
	var pageKey string
	roles := accountAssignmentRolesMap{}
	for {
		response, err := svc.rolesSvc.ListRoles(ctx, &proto.ListRolesRequest{
			StartKey: pageKey,
			Limit:    200,
			Filter:   &types.RoleFilter{SkipSystemRoles: true},
		})
		if err != nil {
			return nil, trace.Wrap(err, "enumerating known AWS accounts")
		}

		for _, role := range response.Roles {
			if role.Origin() != common.OriginAWSIdentityCenter ||
				role.GetSubKind() != RoleSubKindIdentityCenter {
				continue
			}

			roleKey, err := mkRoleKeyForRole(role)
			if err != nil {
				svc.log.WarnContext(ctx, "malformed account assignment role",
					"error", err.Error(),
					"role", role.GetName())
			}
			roles[roleKey] = role
		}

		if response.NextKey == "" {
			break
		}
		pageKey = response.NextKey
	}
	return roles, nil
}

// getImportedRoleName returns role name based on permission set name and account name.
func getImportedRoleName(permissionSetName, accountName string) string {
	return normalizeResourceName(fmt.Sprintf("%s-on-%s", permissionSetName, accountName))
}

func NewAccountAssignmentRole(acct services.IdentityCenterAccount, ps *identitycenterv1.PermissionSetInfo) (*types.RoleV6, error) {
	roleName := getImportedRoleName(ps.GetName(), acct.Spec.Name)

	role, err := types.NewRole(roleName, types.RoleSpecV6{
		Allow: types.RoleConditions{
			AccountAssignments: []types.IdentityCenterAccountAssignment{
				{
					Account:       acct.Spec.Id,
					PermissionSet: ps.Arn,
				},
			},
		},
	})
	if err != nil {
		return nil, trace.Wrap(err, "creating account assignment role %q", roleName)
	}
	role.SetSubKind(RoleSubKindIdentityCenter)
	role.SetOrigin(common.OriginAWSIdentityCenter)

	return role.(*types.RoleV6), nil
}

func (svc *Service) reconcileAccountAssignmentRoles(ctx context.Context, oldRoles, newRoles accountAssignmentRolesMap) (accountAssignmentRolesMap, error) {
	result := maps.Clone(oldRoles)

	for k, old := range oldRoles {
		if new, present := newRoles[k]; present {
			new.Metadata.Revision = old.Metadata.Revision
		}
	}

	createRole := func(ctx context.Context, newRole *types.RoleV6) error {
		// if we can't make an appropriate key for the map then there is no
		// point polluting the role DB with something that will never work, so
		// try making the key first, even though we don't use it til later
		key, err := mkRoleKeyForRole(newRole)
		if err != nil {
			return trace.Wrap(err, "malformed Identity Center Account Assignment Role resource")
		}

		svc.log.DebugContext(ctx, "Creating new role", "rolename", newRole.GetName())
		created, err := svc.rolesSvc.CreateRole(ctx, newRole)
		if err != nil {
			return trace.Wrap(err, "creating Identity Center Account Assignment Role resource")
		}

		rv6, ok := created.(*types.RoleV6)
		if !ok {
			return trace.BadParameter("Expected RoleV6, got %T", created)
		}

		result[key] = rv6
		return nil
	}

	updateRole := func(ctx context.Context, newRole, _ *types.RoleV6) error {
		updated, err := svc.rolesSvc.UpdateRole(ctx, newRole)
		if err != nil {
			return trace.Wrap(err, "updating Identity Center Account record")
		}
		rv6, ok := updated.(*types.RoleV6)
		if !ok {
			return trace.BadParameter("Expected RoleV6, got %T", rv6)
		}

		key, err := mkRoleKeyForRole(newRole)
		if err != nil {
			return trace.Wrap(err, "malformed Identity Center Account Assignment Role resource")
		}
		result[key] = rv6
		return nil
	}

	deleteRole := func(ctx context.Context, role *types.RoleV6) error {
		// TODO(tcsc): make sure all users and access lists have this role
		// removed before deleting

		if err := svc.rolesSvc.DeleteRole(ctx, role.GetName()); err != nil {
			return trace.Wrap(err, "updating Identity Center Account record")
		}

		key, err := mkRoleKeyForRole(role)
		if err != nil {
			return trace.Wrap(err, "malformed Identity Center Account Assignment Role resource")
		}

		delete(result, key)
		return nil
	}

	r, err := services.NewGenericReconciler(
		services.GenericReconcilerConfig[accountAssignmentRoleKey, *types.RoleV6]{
			Matcher:             func(*types.RoleV6) bool { return true },
			GetCurrentResources: passThrough(oldRoles),
			GetNewResources:     passThrough(newRoles),
			OnCreate:            createRole,
			OnUpdate:            updateRole,
			OnDelete:            deleteRole,
			Logger:              svc.log.With("resource_type", types.KindRole),
		})
	if err != nil {
		return nil, trace.Wrap(err, "creating Identity Center Account Assignment role reconciler")
	}

	if err := r.Reconcile(ctx); err != nil {
		return nil, trace.Wrap(err, "reconciling Identity Center Account Assignment Roles")
	}

	return result, nil
}
