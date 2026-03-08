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
	newRole, err := types.NewRoleWithVersion(roleName, types.V7, types.RoleSpecV6{
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

	role, err := asRoleV6(newRole)
	if err != nil {
		return nil, trace.Wrap(err)
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

	return role, nil
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

		log := svc.log.With(
			slog.String("role_name", newRole.GetName()),
			slog.Group("for",
				slog.String("account", string(key.account)),
				slog.String("permission_set", key.permissionSet),
			))

		log.DebugContext(ctx, "Creating new Identity Center Account Assignment Role")
		created, err := createRoleV6(ctx, svc.rolesSvc, newRole)
		if err != nil {
			if !trace.IsAlreadyExists(err) {
				return trace.Wrap(err, "creating Identity Center Account Assignment Role resource")
			}

			log.WarnContext(ctx, "Detected Account Assignment Role name collision")
			created, err = svc.handleRoleNameClash(ctx, newRole)
			if err != nil {
				return trace.Wrap(err, "handling Account Assignment role name collision")
			}
		}

		result[key] = created
		return nil
	}

	updateRole := func(ctx context.Context, newRole, oldRole *types.RoleV6) error {
		// A role has been renamed, but still refers to the same underlying PS
		// and AWS Account. Instead of simply updating the role we need to create
		// a new role to match the new name and deprecate the old one.
		if newRole.GetName() != oldRole.GetName() {
			svc.log.InfoContext(ctx, "Identity Center Account Assignment Role name change detected. A new role will be created and the existing role deprecated.",
				slog.String("from", oldRole.GetName()),
				slog.String("to", newRole.GetName()))

			if err := createRole(ctx, newRole); err != nil {
				return trace.Wrap(err, "creating renamed Identity Center Account Assignment Role")
			}
			if err := svc.deprecateRole(ctx, oldRole, withReplacement(newRole.GetName())); err != nil {
				return trace.Wrap(err, "deprecating renamed Identity Center Account Assignment Role")
			}
			return nil
		}

		// This is a straight overwrite. We copy the existing role's revision to
		// the new role so that the optimistic locking system will allow us to
		// update the role, and then return the *new* role to be written to the
		// backend.
		// If a write fails on this pass, it should be cleaned up on the
		// synchronization pass.
		replaceRole := func(baseRole *types.RoleV6) *types.RoleV6 {
			newRole.Metadata.Revision = baseRole.Metadata.Revision
			return newRole
		}
		updated, err := updateRoleWithRetry(ctx, svc.rolesSvc, oldRole, replaceRole, svc.clock)
		if err != nil {
			return trace.Wrap(err)
		}
		key, err := mkRoleKeyForRole(updated)
		if err != nil {
			return trace.BadParameter("malformed Identity Center Account Assignment Role")
		}
		result[key] = updated
		return nil
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
		clock: svc.clock,
	}
	for _, applyOption := range options {
		applyOption(&opts)
	}

	svc.log.DebugContext(ctx, "Deprecating Account Assignment role",
		slog.String("rolename", role.GetName()),
		slog.String("replacement", opts.replacement))

	markAsDeprecated := func(r *types.RoleV6) *types.RoleV6 {
		r.SubKind = ""
		if r.Metadata.Labels == nil {
			r.Metadata.Labels = map[string]string{}
		}
		r.Metadata.Labels[roleReplacementLabel] = opts.replacement
		return r
	}

	_, err := updateRoleWithRetry(ctx, svc.rolesSvc, role, markAsDeprecated, opts.clock)
	return trace.Wrap(err)
}

// roleMutator defines the signature or role modifying functions for use with
// [updateRoleWithRetry]. The returned [*types.RoleV6] will be used when writing
// the role to the backend.
type roleMutator func(*types.RoleV6) *types.RoleV6

// updateRoleWithRetry attempts to mutate the supplied role by applying the
// supplied [modifyFn] and writing the result to the cluster backend. If the
// write fails due to an optimistic write conflict, the role will be re-loaded
// from the backend and the process will be retried with an exponential backoff.
func updateRoleWithRetry(ctx context.Context, svc RolesService, role *types.RoleV6, modifyFn roleMutator, clock clockwork.Clock) (*types.RoleV6, error) {
	refreshRole := func(ctx context.Context, isRetry bool) (*types.RoleV6, error) {
		if !isRetry {
			return role, nil
		}
		fresh, err := getRoleV6(ctx, svc, role.GetName())
		return fresh, trace.Wrap(err)
	}

	var updated *types.RoleV6
	updateRole := func(ctx context.Context, target *types.RoleV6) error {
		modified := modifyFn(target)
		var err error
		updated, err = updateRoleV6(ctx, svc, modified)
		return trace.Wrap(err)
	}

	if err := retryutils.UpdateWithRetry(ctx, clock, refreshRole, updateRole); err != nil {
		return nil, trace.Wrap(err)
	}

	return updated, nil
}

// handleRoleNameClash handles the situation where the Account Assignment Role
// reconciler cannot create a new role due to an existing role already having the
// target role's name.
//
// If the existing role was created by the Identity Center integration (for example,
// it could be an old Account Assignment role that was deprecated after an AWS
// Account was renamed), the IC integration will reclaim the existing role and
// configure it as the Account Assignment Role.
//
// If the role blocking the creation was not created by the Identity Center
// integration, the blocking role is left alone and an error is written to the
// log.
func (svc *Service) handleRoleNameClash(ctx context.Context, newRole *types.RoleV6) (*types.RoleV6, error) {
	log := svc.log.With("role_name", newRole.GetName())
	log.DebugContext(ctx, "Handling role name collision")

	existingRole, err := getRoleV6(ctx, svc.rolesSvc, newRole.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if l, ok := existingRole.GetLabel(roleCreatedByLabel); !ok || l != types.KindIdentityCenter {
		log.ErrorContext(ctx, "An existing role is blocking the creation of an Identity Center role. Please review this role and rename or delete it.")
		return nil, trace.BadParameter("Existing role blocks Account Assignment Role creation: %s", newRole.GetName())
	}

	log.DebugContext(ctx, "Reclaiming existing role")
	reclaimRole := func(r *types.RoleV6) *types.RoleV6 {
		r.SubKind = types.KindIdentityCenter
		if r.Metadata.Labels != nil {
			delete(r.Metadata.Labels, roleReplacementLabel)
		}
		r.Spec.Allow.AccountAssignments = newRole.Spec.Allow.AccountAssignments
		r.Spec.Deny.AccountAssignments = nil
		return r
	}
	reclaimed, err := updateRoleWithRetry(ctx, svc.rolesSvc, existingRole, reclaimRole, svc.clock)
	return reclaimed, trace.Wrap(err)
}

func createRoleV6(ctx context.Context, svc RolesService, role *types.RoleV6) (*types.RoleV6, error) {
	created, err := svc.CreateRole(ctx, role)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	rv6, err := asRoleV6(created)
	return rv6, trace.Wrap(err)
}

func getRoleV6(ctx context.Context, svc RolesService, name string) (*types.RoleV6, error) {
	r, err := svc.GetRole(ctx, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	rv6, err := asRoleV6(r)
	return rv6, trace.Wrap(err)
}

func updateRoleV6(ctx context.Context, svc RolesService, role *types.RoleV6) (*types.RoleV6, error) {
	updated, err := svc.UpdateRole(ctx, role)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	rv6, err := asRoleV6(updated)
	return rv6, trace.Wrap(err)
}

// asRoleV6 safely casts a [types.Role] into a [*types.RoleV6]. Use asRoleV6
// rather than manual type assertions to ensure uniform error messages in the
// event of failure.
func asRoleV6(role types.Role) (*types.RoleV6, error) {
	if rv6, ok := role.(*types.RoleV6); ok {
		return rv6, nil
	}
	return nil, trace.BadParameter("expected RoleV6, got %T", role)
}
