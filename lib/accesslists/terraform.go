package accesslists

import (
	"fmt"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	conv "github.com/gravitational/teleport/api/types/accesslist/convert/v1"
	"github.com/gravitational/teleport/lib/accesslists/preset"
	"github.com/gravitational/teleport/lib/tfgen"
)

const terraformAccessListType = "teleport_access_list"

type TerraformConfigParams struct {
	// PresetType specifies the type of preset configuration to apply.
	PresetType preset.PresetType
	// AccessList contains the full access list configuration.
	// If nil, no access list and its members will be generated
	// as part of the terraform cfg.
	AccessList *accesslist.AccessList
	// Members are the access lists members. If access list is nil
	// no members are generated.
	Members []*accesslist.AccessListMember
	// AccessListID should be the same as AccessList.Metadata.Name but is separated
	// here to use the ID as part of the role name and role labels. Roles can be
	// generated first before the access list resource. (web UI flow)
	AccessListID string
	// AccessRoles related to defining access to resources.
	// If empty, no roles will be generated as part of the terraform cfg.
	AccessRoles []AccessRole
	// If empty, provider block will not be part of the terraform cfg.
	ProviderBlock *ProviderBlock
}

type AccessRole struct {
	Role types.Role
	// BlockComment is an optional comment that will be generated
	// on top of the role resource definition in the Terraform cfg.
	// This can be used to provide additional context to user.
	BlockComment string
}

type ProviderBlock struct {
	ProxyAddr       string
	TeleportVersion int64
}

// GenerateTerraformConfig returns a terraform text for basic access list and its members.
func GenerateTerraformConfig(al *accesslist.AccessList, members []*accesslist.AccessListMember, providerBlock *ProviderBlock) (string, error) {
	if al == nil {
		// Nothing to generate
		return "", nil
	}

	accessListCfg, terraformAccessListName, err := makeAccessListCfg(al)
	if err != nil {
		return "", trace.Wrap(err)
	}

	var memberCfgs []string
	if len(members) > 0 {
		memberCfgs, err = makeAccessListMemberCfgs(members, terraformAccessListName)
		if err != nil {
			return "", trace.Wrap(err)
		}
	}

	cfgBlocks := []string{"# Terraform config for creating an Access List.\n"}
	if providerBlock != nil {
		cfgBlocks = append(cfgBlocks, tfgen.ProviderBlock(providerBlock.TeleportVersion, providerBlock.ProxyAddr))
	}
	cfgBlocks = append(cfgBlocks, accessListCfg)
	cfgBlocks = append(cfgBlocks, memberCfgs...)

	return strings.Join(cfgBlocks, "\n"), nil
}

// GenerateTerraformConfigWithPresetBuilder returns a terraform text for access list created
// with a preset builder which constructs all the supporting roles required and auto sets these
// roles as access list grants.
func GenerateTerraformConfigWithPresetBuilder(params TerraformConfigParams) (string, error) {
	accessRoles := make([]types.Role, 0, len(params.AccessRoles))
	for _, ar := range params.AccessRoles {
		accessRoles = append(accessRoles, ar.Role)
	}

	presetBuilder, err := preset.NewPresetAccessListRolesBuilderForTerraform(preset.AccessListRolesBuilderConfig{
		PresetName:     params.AccessListID,
		AccessListSpec: params.AccessList,
		PresetType:     params.PresetType,
		AccessRoles:    accessRoles,
	})
	if err != nil {
		return "", trace.Wrap(err)
	}

	builtRoles, err := presetBuilder.BuildRoles()
	if err != nil {
		return "", trace.Wrap(err)
	}

	roleCfgs, err := makeAccessRoleCfgs(params.AccessRoles, builtRoles)
	if err != nil {
		return "", trace.Wrap(err)
	}

	if len(roleCfgs) > 0 {
		supportingRoleCfgs, err := makeSupportingRoleCfgs(params.PresetType, builtRoles)
		if err != nil {
			return "", trace.Wrap(err)
		}
		roleCfgs = append(roleCfgs, supportingRoleCfgs...)
	}

	var accessListCfg string
	terraformAccessListName := ""
	if params.AccessList != nil {
		// Only pass builtRoles when access roles were provided, so
		// the access list doesn't reference roles absent from the config.
		var rolesForAccessList *preset.RolesBuildResult
		if len(params.AccessRoles) > 0 {
			rolesForAccessList = builtRoles
		}
		builtAl, err := presetBuilder.BuildAccessList(rolesForAccessList)
		if err != nil {
			return "", trace.Wrap(err)
		}

		accessListCfg, terraformAccessListName, err = makeAccessListCfg(builtAl.AccessList)
		if err != nil {
			return "", trace.Wrap(err)
		}
	}

	var accessListMembersCfgs []string
	if params.AccessList != nil && len(params.Members) > 0 {
		accessListMembersCfgs, err = makeAccessListMemberCfgs(params.Members, terraformAccessListName)
		if err != nil {
			return "", trace.Wrap(err)
		}
	}

	if len(roleCfgs) == 0 && accessListCfg == "" {
		// Nothing to generate.
		return "", nil
	}

	var configDescription string
	switch params.PresetType {
	case preset.ShortTermPresetType:
		configDescription = "# Terraform config for creating an Access List with just-in-time (JIT) access.\n# Members must submit an access request for temporary access to Teleport resources,\n# subject to approval by the owners.\n"
	case preset.LongTermPresetType:
		configDescription = "# Terraform config for creating an Access List that grants members direct\n# access to Teleport resources defined in the associated roles.\n"
	}

	cfgBlocks := []string{configDescription}
	if params.ProviderBlock != nil {
		cfgBlocks = append(cfgBlocks, tfgen.ProviderBlock(params.ProviderBlock.TeleportVersion, params.ProviderBlock.ProxyAddr))
	}
	cfgBlocks = append(cfgBlocks, roleCfgs...)
	if accessListCfg != "" {
		cfgBlocks = append(cfgBlocks, accessListCfg)
	}
	cfgBlocks = append(cfgBlocks, accessListMembersCfgs...)

	return strings.Join(cfgBlocks, "\n"), nil
}

func makeAccessRoleCfgs(reqAccessRoles []AccessRole, builtRoles *preset.RolesBuildResult) ([]string, error) {
	findRoleComment := func(builtRoleName string) string {
		for _, roleReq := range reqAccessRoles {
			if roleReq.BlockComment != "" && strings.HasPrefix(builtRoleName, roleReq.Role.GetName()) {
				return roleReq.BlockComment
			}
		}
		return ""
	}

	var roleCfgs []string
	for _, builtAccessRole := range builtRoles.AccessRoles {
		var tfGenOpts []tfgen.GenerateOpt
		if comment := findRoleComment(builtAccessRole.GetName()); comment != "" {
			tfGenOpts = append(tfGenOpts, tfgen.WithResourceBlockComment(comment))
		}

		role, err := tfgen.Generate(builtAccessRole, tfGenOpts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		roleCfgs = append(roleCfgs, string(role))
	}
	return roleCfgs, nil
}

func makeSupportingRoleCfgs(requestedPreset preset.PresetType, builtRoles *preset.RolesBuildResult) ([]string, error) {
	requesterComment := ""
	switch requestedPreset {
	case preset.ShortTermPresetType:
		requesterComment = "A role that requires requesting access to resources (assigned to members)."
	case preset.LongTermPresetType:
		requesterComment = "A role that requires requesting access to resources. Can optionally assign to any users outside of this access list."
	}

	var roleCfgs []string

	requesterRoleCfg, err := tfgen.Generate(builtRoles.RequesterRole,
		tfgen.WithResourceBlockComment(requesterComment),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	roleCfgs = append(roleCfgs, string(requesterRoleCfg))

	reviewerRoleCfg, err := tfgen.Generate(builtRoles.ReviewerRole,
		tfgen.WithResourceBlockComment("A role that allows reviewing access requests (assigned to owners)."),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	roleCfgs = append(roleCfgs, string(reviewerRoleCfg))

	return roleCfgs, nil
}

func makeAccessListMemberCfgs(members []*accesslist.AccessListMember, terraformAccessListName string) ([]string, error) {
	membersProto := conv.ToMembersProto(members)
	var memberCfgs []string
	for _, memberProto := range membersProto {
		memberCfg, err := tfgen.Generate(
			tfgen.WrapHeaderResource(memberProto),
			tfgen.WithResourceType("teleport_access_list_member"),
			tfgen.WithResourceName(fmt.Sprintf("acl-member-%s", tfgen.UniqueSanitizedResourceName(memberProto.GetHeader().GetMetadata().GetName()))),
			tfgen.WithDependsOn(fmt.Sprintf("%s.%s", terraformAccessListType, terraformAccessListName)),
			tfgen.WithOmitField("spec.ineligible_status"),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		memberCfgs = append(memberCfgs, string(memberCfg))
	}

	return memberCfgs, nil
}

func makeAccessListCfg(al *accesslist.AccessList) (cfg string, terraformName string, err error) {
	terraformName = fmt.Sprintf("acl-%s", tfgen.SanitizeResourceName(al.GetName()))

	// Access lists managed with terraform requires type to be "static"
	// so members can also be managed by terraform.
	alStatic := *al
	alStatic.Spec.Type = accesslist.Static

	accessListProto := conv.ToProto(&alStatic)
	cfgBytes, err := tfgen.Generate(
		tfgen.WrapHeaderResource(accessListProto),
		tfgen.WithResourceType(terraformAccessListType),
		tfgen.WithResourceName(terraformName),
		tfgen.WithOmitField("spec.owners.ineligible_status"),
		tfgen.WithOmitField("spec.audit"),
	)
	if err != nil {
		return "", "", trace.Wrap(err)
	}

	return string(cfgBytes), terraformName, nil
}
