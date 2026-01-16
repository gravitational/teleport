package preset

import (
	"fmt"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
)

const (
	// TeleportAccessListPreset marks an access list resource (with a label)
	// that it was created using a preset.
	TeleportAccessListPreset = types.TeleportInternalLabelPrefix + "access-list-preset"
	// TeleportAccessListPresetRoles stores the comma-separated list of access role names
	// managed by this preset access list. This is used to track which roles should be
	// deleted when they're removed from the configuration.
	TeleportAccessListPresetRoles = types.TeleportInternalLabelPrefix + "access-list-preset-roles"
	// roleInfixPreset is used to easily identify roles by name that it was
	// created for an access list that used a preset.
	roleInfixPreset = "acl-preset"

	// roleRequesterPrefix describes a role that allows to make access requests
	// to some resources.
	roleRequesterPrefix = "requester"
	// roleReviewerPrefix describes a role that allows reviewing access requests.
	roleReviewerPrefix = "reviewer"

	// RoleDesc indicating that resources was created by internal flow and should not be manage by users.
	RoleDesc = "Role created by Teleport. Do not edit."
)

// PresetType defines the type of access list preset.
type PresetType string

// LongTermPresetType grants members access roles directly.
// Owners receive the reviewer role.
const LongTermPresetType PresetType = "long-term"

// ShortTermPresetType grants members a requester role for on-demand access.
// Members receive the requester role to request access, owners receive the reviewer role.
const ShortTermPresetType PresetType = "short-term"

// AccessListRolesBuilderConfig contains the configuration for building
// access list and roles. The config is validated during builder creation.
type AccessListRolesBuilderConfig struct {
	// PresetName is the name of the preset access list.
	PresetName string
	// AccessListSpec is the access list specification to build from.
	AccessListSpec accesslist.AccessList
	// PresetType determines the grant behavior (long-term or short-term).
	PresetType PresetType
	// AccessRoles are the roles that will be granted based on the preset type.
	AccessRoles []types.Role
}

// CheckAndSetDefaults validates the config and sets default values.
// It ensures that the access list name, preset name, and preset type are valid.
func (c *AccessListRolesBuilderConfig) CheckAndSetDefaults() error {
	aclName := c.AccessListSpec.GetName()
	if aclName == "" {
		return trace.BadParameter("access list name is required")
	}
	if c.PresetName != "" && c.PresetName != aclName {
		return trace.BadParameter("access list name is invalid")
	}
	if c.PresetType != LongTermPresetType && c.PresetType != ShortTermPresetType {
		return trace.BadParameter("preset type is required")
	}
	return nil
}

// AccessListRolesBuilder is a standalone builder that constructs access lists
// and roles in-memory WITHOUT any backend operations.
// It is completely decoupled from persistence and can be used independently.
type AccessListRolesBuilder struct {
	cfg AccessListRolesBuilderConfig
}

// BuildResult contains all constructed objects (access list + roles).
type BuildResult struct {
	// AccessList is the constructed access list with grants configured based on preset type.
	AccessList *accesslist.AccessList
	// ReviewerRole allows reviewing access requests for the access roles.
	ReviewerRole types.Role
	// RequesterRole allows requesting access to the access roles.
	RequesterRole types.Role
	// AccessRoles are the roles that grant actual permissions (e.g., app access, database access).
	AccessRoles []types.Role
	// RolesToBeDeleted are role names that should be deleted (removed from previous access list configuration).
	RolesToBeDeleted []string
}

// NewPresetAccessListRolesBuilder creates a new standalone builder
// that constructs access list and roles in-memory (no backend operations).
// The config is validated during builder creation.
func NewPresetAccessListRolesBuilder(cfg AccessListRolesBuilderConfig) (*AccessListRolesBuilder, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &AccessListRolesBuilder{cfg: cfg}, nil
}

func collectRolesName(roles []types.Role) []string {
	out := make([]string, 0, len(roles))
	for _, role := range roles {
		out = append(out, role.GetName())
	}
	return out
}

// Build constructs the access list and all roles in-memory.
// It creates the access list, reviewer role, requester role, and access roles
// based on the preset type. Returns BuildResult with all constructed objects.
func (b *AccessListRolesBuilder) Build() (*BuildResult, error) {
	accessRoles, err := b.constructAccessRoles()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	accessRoleNames := collectRolesName(accessRoles)
	reviewerRole, err := b.constructReviewerRole(accessRoleNames)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	requesterRole, err := b.constructRequesterRole(accessRoleNames)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	accessList, err := b.constructAccessList(reviewerRole.GetName(), requesterRole.GetName(), accessRoleNames)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Compute roles to be deleted by comparing old and new access lists
	rolesToBeDeleted := b.computeRolesToBeDeleted(&b.cfg.AccessListSpec, accessList)

	return &BuildResult{
		AccessList:       accessList,
		AccessRoles:      accessRoles,
		ReviewerRole:     reviewerRole,
		RequesterRole:    requesterRole,
		RolesToBeDeleted: rolesToBeDeleted,
	}, nil
}

// constructAccessRoles constructs the access roles from the provided role specs
//
// Note: This function modifies the input roles in cfg.AccessRoles
// (renames them and adds labels). Callers should not reuse these role objects.
func (b *AccessListRolesBuilder) constructAccessRoles() ([]types.Role, error) {
	roles := make([]types.Role, 0, len(b.cfg.AccessRoles))
	for _, roleSpec := range b.cfg.AccessRoles {
		role := b.prepareAccessRole(roleSpec)
		roles = append(roles, role)
	}
	return roles, nil
}

// prepareAccessRole clones and configures a role for the preset access list.
// It ensures the role has the correct preset name (without double-suffixing),
// labels, and description.
func (b *AccessListRolesBuilder) prepareAccessRole(roleSpec types.Role) types.Role {
	role := roleSpec.Clone()

	if _, ok := role.GetLabel(TeleportAccessListPreset); !ok {
		role.SetName(b.generateRoleName(roleSpec.GetName()))
		role.SetStaticLabels(map[string]string{
			TeleportAccessListPreset: b.cfg.PresetName,
		})
	}
	setRoleDesc(role, RoleDesc)

	return role
}

// constructReviewerRole constructs the reviewer role that allows reviewing access requests
func (b *AccessListRolesBuilder) constructReviewerRole(accessRoleNames []string) (types.Role, error) {
	spec := types.RoleSpecV6{
		Allow: types.RoleConditions{
			ReviewRequests: &types.AccessReviewConditions{
				PreviewAsRoles: accessRoleNames,
				Roles:          accessRoleNames,
			},
		},
	}
	labels := map[string]string{
		TeleportAccessListPreset: b.cfg.PresetName,
	}

	role, err := types.NewRole(b.generateRoleName(roleReviewerPrefix), spec)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	setRoleDesc(role, RoleDesc)
	role.SetStaticLabels(labels)
	return role, nil
}

// constructRequesterRole constructs the requester role that allows requesting access
func (b *AccessListRolesBuilder) constructRequesterRole(accessRoleNames []string) (types.Role, error) {
	roleName := b.generateRoleName(roleRequesterPrefix)

	spec := types.RoleSpecV6{
		Allow: types.RoleConditions{
			Request: &types.AccessRequestConditions{
				SearchAsRoles: accessRoleNames,
			},
		},
	}
	labels := map[string]string{
		TeleportAccessListPreset: b.cfg.PresetName,
	}

	role, err := types.NewRole(roleName, spec)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	setRoleDesc(role, RoleDesc)
	role.SetStaticLabels(labels)
	return role, nil
}

func setRoleDesc(r types.Role, desc string) {
	meta := r.GetMetadata()
	meta.Description = desc
	r.SetMetadata(meta)
}

// constructAccessList constructs the access list with the appropriate grants based on preset type
func (b *AccessListRolesBuilder) constructAccessList(reviewerRoleName, requesterRoleName string, accessRoleNames []string) (*accesslist.AccessList, error) {
	al, err := accesslist.NewAccessList(b.cfg.AccessListSpec.Metadata, b.cfg.AccessListSpec.Spec)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	m := header.Metadata{
		Name: b.cfg.PresetName,
		Labels: map[string]string{
			TeleportAccessListPreset:      string(b.cfg.PresetType),
			TeleportAccessListPresetRoles: strings.Join(append([]string{reviewerRoleName, requesterRoleName}, accessRoleNames...), ","),
		},
	}
	al.Metadata = m

	// Apply grants based on preset type
	switch b.cfg.PresetType {
	case LongTermPresetType:
		// long-term: Members get access roles directly, owners get reviewer role
		al.Spec.Grants = accesslist.Grants{
			Roles: accessRoleNames,
		}
		al.Spec.OwnerGrants = accesslist.Grants{
			Roles: []string{reviewerRoleName},
		}

	case ShortTermPresetType:
		// Short-term: Members get requester role, owners get reviewer role
		al.Spec.Grants = accesslist.Grants{
			Roles: []string{requesterRoleName},
		}
		al.Spec.OwnerGrants = accesslist.Grants{
			Roles: []string{reviewerRoleName},
		}
	default:
		return nil, trace.BadParameter("unknown preset type %v", b.cfg.PresetType)
	}

	return al, nil
}

// computeRolesToBeDeleted identifies roles that were in the previous access list
// but are not in the new configuration and should be deleted.
func (b *AccessListRolesBuilder) computeRolesToBeDeleted(old, new *accesslist.AccessList) []string {
	oldRoles := ExtractRolesFromLabels(old.GetStaticLabels())
	newRoles := ExtractRolesFromLabels(new.GetStaticLabels())
	return b.findRolesToDelete(oldRoles, newRoles)
}

// ExtractRolesFromLabels extracts the list of managed roles from an access list.
// It checks the TeleportAccessListPresetRoles label first, then falls back to
// extracting from Grants.Roles for backward compatibility.
func ExtractRolesFromLabels(labels map[string]string) []string {
	// First, try to get roles from the metadata label (preferred method)
	rolesStr, ok := labels[TeleportAccessListPresetRoles]
	if !ok {
		return nil
	}
	return strings.Split(rolesStr, ",")
}

// findRolesToDelete compares old roles with new roles and returns
// a list of roles that should be deleted.
func (b *AccessListRolesBuilder) findRolesToDelete(existingRoles, newRoleNames []string) []string {
	newRoleSet := make(map[string]struct{}, len(newRoleNames))
	for _, name := range newRoleNames {
		newRoleSet[name] = struct{}{}
	}

	var rolesToDelete []string
	for _, existingRole := range existingRoles {
		if _, exists := newRoleSet[existingRole]; !exists {
			rolesToDelete = append(rolesToDelete, existingRole)
		}
	}
	return rolesToDelete
}

func (b *AccessListRolesBuilder) generateRoleName(prefix string) string {
	return RoleName(prefix, b.cfg.PresetName)
}

// GetAllRoles returns access, access request, reviewer roles.
func (r BuildResult) GetAllRoles() []types.Role {
	return append([]types.Role{r.ReviewerRole, r.RequesterRole}, r.AccessRoles...)
}

// RoleName generates a role name for preset access list roles.
// The format is: {prefix}-acl-preset-{accessListName}.
// For example: "reviewer-acl-preset-my-access-list".
func RoleName(prefix, accessListName string) string {
	return fmt.Sprintf("%s-%s-%s", prefix, roleInfixPreset, accessListName)
}
