package identitycenter

import (
	"context"
	"fmt"
	"log/slog"
	"maps"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	iciter "github.com/gravitational/teleport/e/lib/aws/identitycenter/iter"
	"github.com/gravitational/teleport/lib/services"
)

const (
	roleAccountLabel     = types.TeleportInternalLabelPrefix + "account_id"
	roleReplacementLabel = types.TeleportInternalLabelPrefix + "replaced_with"
	roleCreatedByLabel   = types.TeleportInternalLabelPrefix + "created_by"
)

// A note on role keys and roleAccountLabels.
//
// Originally we didn't include the AWS account ID in the role name, but it
// turns out that account names are not guaranteed to be unique, meaning that we
// need to include the unique Account ID in the Role name in order to prevent
// duplicate roles.
//
// We're using the normal reconciler to delete the old roles and re-create them
// with the new names. We use a label value only present in the migrated roles
// in the map key to differentiate the old and new roles.

// accountAssignmentKey is a key for the [accountAssignmentRolesMap], which is a
// triple containing the AccountID, the PermissionSet ARN and the content of the
// Role's [roleAccountLabel] label.
//
// The label value is included to mark values that have been migrated to use
// names that include the AWS Account ID.
type accountAssignmentRoleKey struct {
	account       services.IdentityCenterAccountID
	permissionSet string
	label         string
}

func (key accountAssignmentRoleKey) String() string {
	ps := key.permissionSet
	if psARN, err := arn.Parse(ps); err == nil {
		ps = psARN.Resource
	}
	return ps + "/" + string(key.account)
}

func (key *accountAssignmentRoleKey) LogValue() any {
	return slog.StringValue(key.String())
}

type accountAssignmentRolesMap map[accountAssignmentRoleKey]*types.RoleV6

func mkRoleKey(account services.IdentityCenterAccountID, permissionSetARN string, label string) accountAssignmentRoleKey {
	return accountAssignmentRoleKey{
		account:       account,
		permissionSet: permissionSetARN,
		label:         label,
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
	return mkRoleKey(
		services.IdentityCenterAccountID(asmt.Account),
		asmt.PermissionSet,
		role.GetMetadata().Labels[roleAccountLabel]), nil
}

func (svc *Service) loadAccountAssignmentRoles(ctx context.Context) (accountAssignmentRolesMap, error) {
	roles := accountAssignmentRolesMap{}
	for role, err := range iciter.AllAccountAssignmentRoles(ctx, svc.rolesSvc) {
		if err != nil {
			return nil, trace.Wrap(err)
		}
		roleKey, err := mkRoleKeyForRole(role)
		if err != nil {
			svc.log.WarnContext(ctx, "malformed account assignment role",
				"error", err.Error(),
				"role", role.GetName())
			continue
		}
		roles[roleKey] = role
	}
	return roles, nil
}

// getImportedRoleName returns role name based on permission set name and account name.
func getImportedRoleName(permissionSetName, accountName, accountID string) string {
	return normalizeResourceName(fmt.Sprintf("%s-on-%s-%s", permissionSetName, accountName, accountID))
}

func NewAccountAssignmentRole(acct *identitycenterv1.Account, ps *identitycenterv1.PermissionSetInfo) (*types.RoleV6, error) {
	roleName := getImportedRoleName(ps.GetName(), acct.GetSpec().GetName(), acct.GetSpec().GetId())

	// TODO(sshah): update role version to v8 in Teleport version v19.0.0.
	role, err := types.NewRoleWithVersion(roleName, types.V7, types.RoleSpecV6{
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

	// Set the role subkind to indicate that the role is under the control of
	// the Identity Center integration.
	role.SetSubKind(types.KindIdentityCenter)

	labels := role.GetStaticLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	// Mark the role as a new-style IC role that can handle multiple AWS accounts
	// with the same name.
	labels[roleAccountLabel] = acct.Spec.Id

	// Mark the role as being created by the identity center plugin (we can't
	// use origin due to backwards compatibility issues) so that we can target
	// it for deletion even when it gets deprecated (i.e. it's subkind gets reset.)
	labels[roleCreatedByLabel] = types.KindIdentityCenter
	role.SetStaticLabels(labels)

	return role.(*types.RoleV6), nil
}

func (svc *Service) reconcileAccountAssignmentRoles(ctx context.Context, oldRoles, newRoles accountAssignmentRolesMap) (accountAssignmentRolesMap, error) {
	result := maps.Clone(oldRoles)

	createRole := func(ctx context.Context, newRole *types.RoleV6) error {
		// if we can't make an appropriate key for the map then there is no
		// point polluting the role DB with something that will never work, so
		// try making the key first, even though we don't use it til later
		key, err := mkRoleKeyForRole(newRole)
		if err != nil {
			return trace.Wrap(err, "malformed Identity Center Account Assignment Role resource")
		}

		svc.log.DebugContext(ctx, "Creating new Identity Center Account Assignment Role", "rolename", newRole.GetName())
		created, err := svc.rolesSvc.CreateRole(ctx, newRole)
		if trace.IsAlreadyExists(err) {
			// The target role already existing is the most probable cause of
			// error when creating a new role, so emit an admin-friendly log
			// message to help diagnosis
			svc.log.ErrorContext(ctx, "Failed creating Identity Center Account Assignment Role; a role with than name already exists.",
				"role", newRole.GetName())
		}
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

	updateRole := func(ctx context.Context, newRole, oldRole *types.RoleV6) error {
		// A role has been renamed, but still refers to the same underlying PS
		// and AWS Account. Instead of simply updating the role we need to create
		// a new role to match the new name and deprecate the old one.
		if newRole.GetName() != oldRole.GetName() {
			svc.log.DebugContext(ctx, "Identity Center Account Assignment Role name change detected. A new role will be created and the existing role deprecated.",
				slog.String("from", oldRole.GetName()),
				slog.String("to", newRole.GetName()))

			if err := createRole(ctx, newRole); err != nil {
				return trace.Wrap(err, "handling renamed Identity Center Account Assignment Role")
			}
			if err := svc.deprecateRole(ctx, oldRole, withReplacement(newRole.GetName())); err != nil {
				return trace.Wrap(err, "handling renamed Identity Center Account Assignment Role")
			}
			return nil
		}

		// This is a straight update. Update the "new" role's revision to the
		// revision of the "old" role we compared against so that the optimistic
		// locking system can detect and block concurrent writes to the role.
		// If a write fails on this pass, it should be cleaned up on the next one

		refreshRole := func(ctx context.Context, isRetry bool) (*types.RoleV6, error) {
			if !isRetry {
				return oldRole, nil
			}
			freshRole, err := svc.rolesSvc.GetRole(ctx, newRole.GetName())
			if err != nil {
				return nil, trace.Wrap(err)
			}
			if r, ok := freshRole.(*types.RoleV6); ok {
				return r, nil
			}
			return nil, trace.BadParameter("Unexpected role type %T", freshRole)
		}

		updateRole := func(ctx context.Context, baseRole *types.RoleV6) error {
			newRole.Metadata.Revision = baseRole.Metadata.Revision
			updatedRole, err := svc.rolesSvc.UpdateRole(ctx, newRole)
			if err != nil {
				return trace.Wrap(err, "updating Identity Center Account Assignment Role resource")
			}
			rv6, ok := updatedRole.(*types.RoleV6)
			if !ok {
				return trace.BadParameter("expected RoleV6, got %T", rv6)
			}
			key, err := mkRoleKeyForRole(rv6)
			if err != nil {
				return trace.BadParameter("malformed Identity Center Account Assignment Role")
			}
			result[key] = rv6
			return nil
		}

		return trace.Wrap(retryutils.UpdateWithRetry(ctx, svc.clock, refreshRole, updateRole))
	}

	deleteRole := func(ctx context.Context, role *types.RoleV6) error {
		// Completely deleting the role will immediately break any Users, Access
		// Lists, SAML connectors, etc. To avoid this, we deprecate the role by
		// excluding it from the set of roles that the reconciler will consider
		// in future
		if err := svc.deprecateRole(ctx, role); err != nil {
			return trace.Wrap(err)
		}
		key, err := mkRoleKeyForRole(role)
		if err != nil {
			return trace.BadParameter("malformed Identity Center Account Assignment Role")
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

			// As part of addressing a backwards compatibility issue, we want the
			// reconciler to be able to remove the origin value on the IC-created
			// roles during an update. Previously the origin was set to
			// [types.OriginAWSIdentityCenter] which Teleports older than v17
			// reject as an unknown origin.
			//
			// See Also: https://github.com/gravitational/teleport/issues/50654
			AllowOriginChanges: true,
		})
	if err != nil {
		return nil, trace.Wrap(err, "creating Identity Center Account Assignment role reconciler")
	}

	if err := r.Reconcile(ctx); err != nil {
		return nil, trace.Wrap(err, "reconciling Identity Center Account Assignment Roles")
	}

	return result, nil
}

type deprecateRoleOptions struct {
	replacement string
	clock       clockwork.Clock
}

type deprecateRoleOption func(*deprecateRoleOptions)

func withReplacement(roleName string) deprecateRoleOption {
	return func(opts *deprecateRoleOptions) {
		opts.replacement = roleName
	}
}

func (svc *Service) deprecateRole(ctx context.Context, role *types.RoleV6, options ...deprecateRoleOption) error {
	opts := deprecateRoleOptions{
		clock: clockwork.NewRealClock(),
	}
	for _, applyOption := range options {
		applyOption(&opts)
	}

	svc.log.DebugContext(ctx, "Deprecating Account Assignment role",
		slog.String("rolename", role.GetName()),
		slog.String("replacement", opts.replacement))

	refreshRole := func(ctx context.Context, isRetry bool) (*types.RoleV6, error) {
		if !isRetry {
			return role, nil
		}
		freshRole, err := svc.rolesSvc.GetRole(ctx, role.GetName())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if r, ok := freshRole.(*types.RoleV6); ok {
			return r, nil
		}
		return nil, trace.BadParameter("unexpected role type %T", freshRole)
	}

	updateRole := func(ctx context.Context, role *types.RoleV6) error {
		role.SubKind = ""
		if role.Metadata.Labels == nil {
			role.Metadata.Labels = map[string]string{}
		}
		role.Metadata.Labels[roleReplacementLabel] = opts.replacement
		_, err := svc.rolesSvc.UpdateRole(ctx, role)
		return trace.Wrap(err)
	}

	return trace.Wrap(retryutils.UpdateWithRetry(ctx, svc.clock, refreshRole, updateRole))
}
