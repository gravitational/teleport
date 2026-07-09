// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package preset

import (
	"fmt"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
)

const (
	// AccessListPresetRoleInfix is the infix used in the names of roles
	// auto-created for a preset-backed access list. The full role name
	// format is "{prefix}-{AccessListPresetRoleInfix}-{accessListUUID}".
	AccessListPresetRoleInfix = "acl-preset"

	// RoleRequesterPrefix describes a role that allows to make access requests
	// to some resources.
	RoleRequesterPrefix = "requester"
	// RoleReviewerPrefix describes a role that allows reviewing access requests.
	RoleReviewerPrefix = "reviewer"

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
	AccessListSpec *accesslist.AccessList
	// PresetType determines the grant behavior (long-term or short-term).
	PresetType PresetType
	// AccessRoles are the roles that will be granted based on the preset type.
	AccessRoles []types.Role

	// SkipRoleDescriptions when true, omits adding role descriptions
	SkipRoleDescriptions bool
	// ManagedByIAC when not empty, adds a label to roles and access list that
	// the resource is being managed by a IAC tool (e.g. terraform)
	ManagedByIAC string
}

// CheckAndSetDefaults validates the config and sets default values.
// It ensures that the access list name, preset name, and preset type are valid.
//
// Note: this function is not called by NewPresetAccessListRolesBuilderForTerraform.
func (c *AccessListRolesBuilderConfig) CheckAndSetDefaults() error {
	if c.AccessListSpec == nil {
		return trace.BadParameter("access list is required")
	}
	if err := c.validateAccessListName(); err != nil {
		return trace.Wrap(err)
	}
	if err := c.validatePreset(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// validatePreset validates the preset type in the config.
func (c *AccessListRolesBuilderConfig) validatePreset() error {
	if c.PresetType != LongTermPresetType && c.PresetType != ShortTermPresetType {
		return trace.BadParameter("preset type is required")
	}
	return nil
}

// validateAccessListName validates the access list name in the config
// and ensures name matches if both are provided.
func (c *AccessListRolesBuilderConfig) validateAccessListName() error {
	aclName := c.AccessListSpec.GetName()
	if aclName == "" {
		return trace.BadParameter("access list name is required")
	}
	if c.PresetName != "" && c.PresetName != aclName {
		return trace.BadParameter("access list name is invalid")
	}
	return nil
}

// AccessListRolesBuilder is a standalone builder that constructs access lists
// and roles in-memory WITHOUT any backend operations.
// It is completely decoupled from persistence and can be used independently.
type AccessListRolesBuilder struct {
	cfg AccessListRolesBuilderConfig
}

type RolesBuildResult struct {
	// ReviewerRole allows reviewing access requests for the access roles.
	ReviewerRole types.Role
	// RequesterRole allows requesting access to the access roles.
	RequesterRole types.Role
	// AccessRoles are the roles that grant actual permissions (e.g., app access, database access).
	AccessRoles []types.Role
}

type AccessListBuildResult struct {
	// AccessList is the constructed access list with grants configured based on preset type.
	AccessList *accesslist.AccessList
	// RolesToBeDeleted are role names that should be deleted (removed from previous access list configuration).
	RolesToBeDeleted []string
}

// BuildResult contains all constructed objects (access list + roles).
type BuildResult struct {
	RolesBuildResult
	AccessListBuildResult
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

// NewPresetAccessListRolesBuilderForTerraform is similar to NewPresetAccessListRolesBuilder but allows for more
// flexible validation rules to accommodate Terraform's workflow where certain resources (e.g access list) may
// not exist yet.
//
// Note: this constructor does not call CheckAndSetDefaults.
func NewPresetAccessListRolesBuilderForTerraform(cfg AccessListRolesBuilderConfig) (*AccessListRolesBuilder, error) {
	if err := cfg.validatePreset(); err != nil {
		return nil, trace.Wrap(err)
	}
	if cfg.AccessListSpec != nil {
		if err := cfg.validateAccessListName(); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	cfg.ManagedByIAC = types.IACToolTerraform
	cfg.SkipRoleDescriptions = true
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
	builtRoles, err := b.BuildRoles()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	builtAccessList, err := b.BuildAccessList(builtRoles)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &BuildResult{
		RolesBuildResult:      *builtRoles,
		AccessListBuildResult: *builtAccessList,
	}, nil
}

// BuildRoles only constructs the roles: access roles, reviewer role, and requester role.
// Reviewer and requester roles are still constructed even if access roles are not provided.
func (b *AccessListRolesBuilder) BuildRoles() (*RolesBuildResult, error) {
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

	return &RolesBuildResult{
		AccessRoles:   accessRoles,
		ReviewerRole:  reviewerRole,
		RequesterRole: requesterRole,
	}, nil
}

// BuildAccessList only constructs the access list with the appropriate grants based on preset type.
// If builtRoles is nil, access list grants will be empty.
func (b *AccessListRolesBuilder) BuildAccessList(builtRoles *RolesBuildResult) (*AccessListBuildResult, error) {
	if b.cfg.AccessListSpec == nil {
		return nil, trace.BadParameter("access list spec is required to build access list")
	}

	reviewerRoleName := ""
	requesterRoleName := ""
	accessRoleNames := []string{}
	if builtRoles != nil {
		if builtRoles.ReviewerRole != nil {
			reviewerRoleName = builtRoles.ReviewerRole.GetName()
		}
		if builtRoles.RequesterRole != nil {
			requesterRoleName = builtRoles.RequesterRole.GetName()
		}
		accessRoleNames = collectRolesName(builtRoles.AccessRoles)
	}

	accessList, err := b.constructAccessList(reviewerRoleName, requesterRoleName, accessRoleNames)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Compute roles to be deleted by comparing old and new access lists
	rolesToBeDeleted := b.computeRolesToBeDeleted(b.cfg.AccessListSpec, accessList)

	return &AccessListBuildResult{
		AccessList:       accessList,
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

	labels := map[string]string{
		accesslist.AccessListPresetLabel: b.cfg.PresetName,
	}
	if b.cfg.ManagedByIAC != "" {
		labels[types.IACToolLabel] = b.cfg.ManagedByIAC
	}

	if _, ok := role.GetLabel(accesslist.AccessListPresetLabel); !ok {
		role.SetName(b.generateRoleName(roleSpec.GetName()))
		role.SetStaticLabels(labels)
	} else if b.cfg.ManagedByIAC != "" {
		existing := role.GetStaticLabels()
		if existing == nil {
			existing = map[string]string{}
		}
		existing[types.IACToolLabel] = b.cfg.ManagedByIAC
		role.SetStaticLabels(existing)
	}
	if !b.cfg.SkipRoleDescriptions {
		setRoleDesc(role, RoleDesc)
	}

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
		accesslist.AccessListPresetLabel: b.cfg.PresetName,
	}
	if b.cfg.ManagedByIAC != "" {
		labels[types.IACToolLabel] = b.cfg.ManagedByIAC
	}

	role, err := types.NewRole(b.generateRoleName(RoleReviewerPrefix), spec)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !b.cfg.SkipRoleDescriptions {
		setRoleDesc(role, RoleDesc)
	}
	role.SetStaticLabels(labels)
	return role, nil
}

// constructRequesterRole constructs the requester role that allows requesting access
func (b *AccessListRolesBuilder) constructRequesterRole(accessRoleNames []string) (types.Role, error) {
	roleName := b.generateRoleName(RoleRequesterPrefix)

	spec := types.RoleSpecV6{
		Allow: types.RoleConditions{
			Request: &types.AccessRequestConditions{
				SearchAsRoles: accessRoleNames,
			},
		},
	}
	labels := map[string]string{
		accesslist.AccessListPresetLabel: b.cfg.PresetName,
	}
	if b.cfg.ManagedByIAC != "" {
		labels[types.IACToolLabel] = b.cfg.ManagedByIAC
	}

	role, err := types.NewRole(roleName, spec)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !b.cfg.SkipRoleDescriptions {
		setRoleDesc(role, RoleDesc)
	}
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
	labels := map[string]string{
		accesslist.AccessListPresetLabel: string(b.cfg.PresetType),
	}

	presetRoles := []string{}
	if reviewerRoleName != "" {
		presetRoles = append(presetRoles, reviewerRoleName)
	}
	if requesterRoleName != "" {
		presetRoles = append(presetRoles, requesterRoleName)
	}
	presetRoles = append(presetRoles, accessRoleNames...)

	if len(presetRoles) > 0 {
		labels[accesslist.AccessListPresetRolesLabel] = strings.Join(presetRoles, ",")
	}

	if b.cfg.ManagedByIAC != "" {
		labels[types.IACToolLabel] = b.cfg.ManagedByIAC
	}

	al, err := accesslist.NewAccessList(b.cfg.AccessListSpec.Metadata, b.cfg.AccessListSpec.Spec)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	al.Metadata.Name = b.cfg.PresetName
	al.Metadata.Labels = labels

	// Apply grants based on preset type
	switch b.cfg.PresetType {
	case LongTermPresetType:
		memberGrants := accesslist.Grants{}
		ownerGrants := accesslist.Grants{}

		// long-term: Members get access roles directly, owners get reviewer role
		if len(accessRoleNames) > 0 {
			memberGrants = accesslist.Grants{
				Roles: accessRoleNames,
			}
		}
		if reviewerRoleName != "" {
			ownerGrants = accesslist.Grants{
				Roles: []string{reviewerRoleName},
			}
		}

		al.Spec.Grants = memberGrants
		al.Spec.OwnerGrants = ownerGrants

	case ShortTermPresetType:
		memberGrants := accesslist.Grants{}
		ownerGrants := accesslist.Grants{}

		// Short-term: Members get requester role, owners get reviewer role
		if requesterRoleName != "" {
			memberGrants = accesslist.Grants{
				Roles: []string{requesterRoleName},
			}
		}
		if reviewerRoleName != "" {
			ownerGrants = accesslist.Grants{
				Roles: []string{reviewerRoleName},
			}
		}

		al.Spec.Grants = memberGrants
		al.Spec.OwnerGrants = ownerGrants

	default:
		return nil, trace.BadParameter("unknown preset type %v", b.cfg.PresetType)
	}

	return al, nil
}

// computeRolesToBeDeleted identifies roles that were in the previous access list
// but are not in the new configuration and should be deleted.
func (b *AccessListRolesBuilder) computeRolesToBeDeleted(old, new *accesslist.AccessList) []string {
	return b.findRolesToDelete(old.PresetRoleNames(), new.PresetRoleNames())
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
	return fmt.Sprintf("%s-%s-%s", prefix, AccessListPresetRoleInfix, accessListName)
}
