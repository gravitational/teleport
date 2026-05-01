import {
  AccessList,
  AccessListDescriptor,
} from 'e-teleport/services/accessmanagement';
import { TerraformAcessRoleRequest } from 'e-teleport/services/accessmanagement/preset';
import { Role } from 'teleport/services/resources';

import {
  newAccessRole,
  awsIcRoleAccessKind,
  standardRoleAccessKind,
} from '../Preset/role/role';

export function makeTerraformRoleBlock(
  roleParams: Parameters<typeof newAccessRole>[0],
  original: Role | undefined,
  hasAccessDefined: boolean
): ReturnType<typeof getTerraformBlockCommentWithRole> | undefined {
  if (original) {
    const shallowCopy = {
      ...original,
      spec: {
        ...original.spec,
        // If the role was originally created and user wants to remove
        // access, the allow spec is left empty since Terraform auto
        // clean up is a bit unpredictable and can cause errors if
        // Terraform tries to delete roles automatically (e.g. that role
        // could've been assigned to other access list or users)
        allow: hasAccessDefined ? newAccessRole(roleParams).spec.allow : {},
      },
    };
    return getTerraformBlockCommentWithRole(roleParams.kind, shallowCopy);
  }
  // New role has to be created.
  if (hasAccessDefined) {
    return getTerraformBlockCommentWithRole(
      roleParams.kind,
      newAccessRole(roleParams)
    );
  }
}

export function hasTerraformLabel(accessList: AccessListDescriptor) {
  return accessList.metadata?.labels?.['teleport.dev/iac-tool'] === 'terraform';
}

export function makeAccessListMembersForTerraformUpdate(original: AccessList) {
  return original.members.map(m => ({
    name: m.name,
    membership_kind: m.membershipKind,
    reason: m.reason || undefined,
    expires: m.expires || undefined,
  }));
}

export function getTerraformBlockCommentWithRole(
  accessKind: typeof awsIcRoleAccessKind | typeof standardRoleAccessKind,
  role: Role
): TerraformAcessRoleRequest {
  switch (accessKind) {
    case awsIcRoleAccessKind:
      return {
        role,
        blockComment:
          'A role that defines access specific to AWS Identity Center resources.',
      };
    case standardRoleAccessKind:
      return {
        role,
        blockComment: 'A role that defines access to standard resources.',
      };
    default:
      accessKind satisfies never;
  }
}
