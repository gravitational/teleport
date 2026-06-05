package web

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport"
	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	conv "github.com/gravitational/teleport/api/types/accesslist/convert/v1"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/e/lib/accesslist/preset"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tfgen"
	"github.com/gravitational/teleport/lib/web"
)

// createAccessListWithPreset is the handler for POST /enterprise/accesslistpreset.
// It creates a new access list with preset configuration.
func (p *Plugin) createAccessListWithPreset(_ http.ResponseWriter, r *http.Request, params httprouter.Params, sctx *web.SessionContext) (any, error) {
	var req ui.AccessListWithPresetRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	aclClient := getAccessListServiceClient(sctx)

	resp, err := aclClient.CreateAccessListWithPreset(r.Context(), accesslistv1.CreateAccessListWithPresetRequest_builder{
		PresetType: req.PresetType,
		AccessList: conv.ToProto(req.AccessList.AccessList),
		Roles:      req.AccessRoles,
	}.Build())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	acl, err := conv.FromProto(resp.GetAccessList())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	members := make([]*accesslist.AccessListMember, 0, len(req.AccessList.Members))
	for _, member := range req.AccessList.Members {
		members = append(members, memberToAccessListMember(acl.GetName(), member))
	}
	acl, updatedMembers, err := upsertAccessListWithMembers(r.Context(), sctx, acl, members)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := sctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, member := range updatedMembers {
		enrichAccessListMemberTitle(r.Context(), clt.AccessListClient(), member)
	}
	for i := range acl.Spec.Owners {
		enrichAccessListOwnerTitle(r.Context(), clt.AccessListClient(), &acl.Spec.Owners[i])
	}

	return &ui.AccessListWithPresetResponse{
		AccessList: &ui.AccessList{
			AccessList: acl,
			Members:    membersToMembersSpec(updatedMembers),
		},
		AccessRoles: resp.GetRoles(),
	}, nil
}

// updateAccessListWithPreset is the handler for PUT /enterprise/accesslistpreset/:accessListId.
// It updates an existing access list that was created with a preset.
func (p *Plugin) updateAccessListWithPreset(_ http.ResponseWriter, r *http.Request, params httprouter.Params, sctx *web.SessionContext) (any, error) {
	accessListID, err := extractAccessListID(params)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var req ui.AccessListWithPresetRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	if accessListID != req.AccessList.Metadata.Name {
		return nil, trace.BadParameter("accessList.metadata.name must match the accessListName from request")
	}
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	aclClient := getAccessListServiceClient(sctx)
	resp, err := aclClient.UpdateAccessListWithPreset(r.Context(), accesslistv1.UpdateAccessListWithPresetRequest_builder{
		AccessList: conv.ToProto(req.AccessList.AccessList),
		Roles:      req.AccessRoles,
	}.Build())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	acl, err := conv.FromProto(resp.GetAccessList())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	members := make([]*accesslist.AccessListMember, 0, len(req.AccessList.Members))
	for _, member := range req.AccessList.Members {
		members = append(members, memberToAccessListMember(accessListID, member))
	}
	acl, updatedMembers, err := upsertAccessListWithMembers(r.Context(), sctx, acl, members)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clt, err := sctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, member := range updatedMembers {
		enrichAccessListMemberTitle(r.Context(), clt.AccessListClient(), member)
	}
	for i := range acl.Spec.Owners {
		enrichAccessListOwnerTitle(r.Context(), clt.AccessListClient(), &acl.Spec.Owners[i])
	}

	return &ui.AccessListWithPresetResponse{
		AccessList: &ui.AccessList{
			AccessList: acl,
			Members:    membersToMembersSpec(updatedMembers),
		},
		AccessRoles:      resp.GetRoles(),
		RolesToBeDeleted: resp.GetRolesToBeDeleted(),
	}, nil
}

func membersToMembersSpec(members []*accesslist.AccessListMember) []accesslist.AccessListMemberSpec {
	specs := make([]accesslist.AccessListMemberSpec, 0, len(members))
	for _, member := range members {
		specs = append(specs, member.Spec)
	}
	return specs
}

// deleteAccessListWithPreset is the handler for DELETE /enterprise/accesslistpreset/:accessListId
// It deletes an access list that was created with a preset, including all associated preset roles.
func (p *Plugin) deleteAccessListWithPreset(_ http.ResponseWriter, r *http.Request, params httprouter.Params, sctx *web.SessionContext) (any, error) {
	accessListID, err := extractAccessListID(params)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	client, err := sctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	acl, err := client.AccessListClient().GetAccessList(r.Context(), accessListID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := client.AccessListClient().DeleteAccessList(r.Context(), acl.GetName()); err != nil {
		return nil, trace.Wrap(err)
	}
	return &ui.DeleteAccessListWithPresetResponse{
		Roles: preset.ExtractRolesFromLabels(acl.GetAllLabels()),
	}, nil
}

// extractAccessListID extracts and validates the accessListId parameter.
func extractAccessListID(params httprouter.Params) (string, error) {
	accessListID := params.ByName("accessListId")
	if accessListID == "" {
		return "", trace.BadParameter("accessListID is required")
	}
	return accessListID, nil
}

// getAccessListServiceClient creates a new AccessListServiceClient from the session context.
func getAccessListServiceClient(sctx *web.SessionContext) accesslistv1.AccessListServiceClient {
	return accesslistv1.NewAccessListServiceClient(sctx.GetClientConnection())
}

// upsertAccessListWithMembers upserts an access list along with its members.
func upsertAccessListWithMembers(ctx context.Context, sctx *web.SessionContext, acl *accesslist.AccessList, members []*accesslist.AccessListMember) (*accesslist.AccessList, []*accesslist.AccessListMember, error) {
	client, err := sctx.GetClient()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return client.AccessListClient().UpsertAccessListWithMembers(ctx, acl, members)
}

// enrichAccessListMemberTitle enriches the member's title if the membership kind is MembershipKindList.
func enrichAccessListMemberTitle(ctx context.Context, client services.AccessLists, member *accesslist.AccessListMember) {
	if member.Spec.MembershipKind == accesslist.MembershipKindList {
		if al, err := getAccessListNoMFACtx(ctx, client, member.GetName()); err == nil {
			member.Spec.Title = al.Spec.Title
		}
	}
}

// enrichAccessListOwnerTitle enriches the owner's title if the membership kind is MembershipKindList.
func enrichAccessListOwnerTitle(ctx context.Context, client services.AccessLists, owner *accesslist.Owner) {
	if owner.MembershipKind == accesslist.MembershipKindList {
		if al, err := getAccessListNoMFACtx(ctx, client, owner.Name); err == nil {
			owner.Title = al.Spec.Title
		}
	}
}

// generateAccessListTerraformConfig generates Terraform configuration for an access list preset based on the provided request parameters.
func (p *Plugin) generateAccessListTerraformConfig(_ http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (any, error) {
	var req ui.GenerateAccessListTerraformConfigRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	if req.AccessListID == "" {
		return nil, trace.BadParameter("accessListId is required")
	}

	var newAccessList *accesslist.AccessList
	var err error
	if req.AccessList != nil {
		if req.AccessList.AccessList == nil {
			return nil, trace.BadParameter("accessList is required")
		}
		newAccessList, err = accesslist.NewAccessList(req.AccessList.Metadata, req.AccessList.Spec)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if newAccessList.GetName() != req.AccessListID {
			return nil, trace.BadParameter("accessList.metadata.name must match accessListId from request")
		}
	}

	accessRoles := make([]types.Role, 0, len(req.AccessRoles))
	for _, roleReq := range req.AccessRoles {
		if roleReq.Role == nil {
			return nil, trace.BadParameter("access role is nil")
		}
		accessRoles = append(accessRoles, roleReq.Role)
	}

	reqPreset := preset.PresetType(req.PresetType)
	presetBuilder, err := preset.NewPresetAccessListRolesBuilderForTerraform(preset.AccessListRolesBuilderConfig{
		PresetName:     req.AccessListID,
		AccessListSpec: newAccessList,
		PresetType:     reqPreset,
		AccessRoles:    accessRoles,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	builtRoles, err := presetBuilder.BuildRoles()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var roleCfgs []string
	roleCfgs, err = makeAccessRoleCfgs(req, builtRoles)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(roleCfgs) > 0 {
		supportingRoleCfgs, err := makeSupportingRoleCfgs(reqPreset, builtRoles)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		roleCfgs = append(roleCfgs, supportingRoleCfgs...)
	}

	var accessListCfg string
	terraformAccessListName := ""
	terraformAccessListType := "teleport_access_list"

	if req.AccessList != nil {
		// Only pass builtRoles when access roles were provided, so
		// the access list doesn't reference roles absent from the config.
		var rolesForAccessList *preset.RolesBuildResult
		if len(req.AccessRoles) > 0 {
			rolesForAccessList = builtRoles
		}
		builtAl, err := presetBuilder.BuildAccessList(rolesForAccessList)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		terraformAccessListName = fmt.Sprintf("acl-%s", tfgen.SanitizeResourceName(builtAl.AccessList.GetName()))
		accessList := conv.ToProto(builtAl.AccessList)
		accessListCfgBytes, err := tfgen.Generate(
			tfgen.WrapHeaderResource(accessList),
			tfgen.WithResourceType(terraformAccessListType),
			tfgen.WithResourceName(terraformAccessListName),
			tfgen.WithOmitField("spec.owners.ineligible_status"),
			tfgen.WithOmitField("spec.audit"),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		accessListCfg = string(accessListCfgBytes)
	}

	var accessListMembersCfgs []string
	if req.AccessList != nil && len(req.AccessList.Members) > 0 {
		accessListMembersCfgs, err = makeAccessListMemberCfgs(req, terraformAccessListType, terraformAccessListName)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	var configDescription string
	switch reqPreset {
	case preset.ShortTermPresetType:
		configDescription = "# Terraform config for creating an Access List with just-in-time (JIT) access.\n# Members must submit an access request for temporary access to Teleport resources,\n# subject to approval by the owners.\n"
	case preset.LongTermPresetType:
		configDescription = "# Terraform config for creating an Access List that grants members direct\n# access to Teleport resources defined in the associated roles.\n"
	}

	cfgBlocks := []string{
		configDescription,
		providerBlock(teleport.SemVer().Major, p.h.PublicProxyAddr()),
	}
	cfgBlocks = append(cfgBlocks, roleCfgs...)
	if accessListCfg != "" {
		cfgBlocks = append(cfgBlocks, accessListCfg)
	}
	cfgBlocks = append(cfgBlocks, accessListMembersCfgs...)

	return ui.GenerateAccessListTerraformConfigResponse{Terraform: strings.Join(cfgBlocks, "\n")}, nil
}

func makeAccessRoleCfgs(req ui.GenerateAccessListTerraformConfigRequest, builtRoles *preset.RolesBuildResult) ([]string, error) {
	findRoleComment := func(builtRoleName string) string {
		for _, roleReq := range req.AccessRoles {
			if roleReq.BlockComment != "" && strings.HasPrefix(builtRoleName, roleReq.Role.Metadata.Name) {
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

func makeAccessListMemberCfgs(req ui.GenerateAccessListTerraformConfigRequest, terraformAccessListType, terraformAccessListName string) ([]string, error) {
	members := make([]*accesslist.AccessListMember, 0, len(req.AccessList.Members))
	for _, reqMember := range req.AccessList.Members {
		reqMember.AccessList = req.AccessListID
		member, err := accesslist.NewAccessListMember(header.Metadata{Name: reqMember.Name}, reqMember)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		members = append(members, member)
	}

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

func providerBlock(majorVersion int64, proxyAddr string) string {
	return fmt.Sprintf(`terraform {
  required_providers {
    teleport = {
      source  = "terraform.releases.teleport.dev/gravitational/teleport"
      version = "~> %d.0"
    }
  }
}

provider "teleport" {
  addr = %q
}
`, majorVersion, proxyAddr)
}
