import {
  AccessListOrigin,
  isReadOnly,
  isScim,
  type AccessList,
} from 'e-teleport/services/accessmanagement';

import { isGuideEditorSupported } from '../GuideEditor/useGuideEditor';
import { Perms } from './Shared';

type AccessProps = {
  accessList: Pick<AccessList, 'origin' | 'type' | 'preset'>;
  isReadOnlyOktaList?: boolean;
  action: Action;
  perms: Perms;
  missingRolePerms?: string[];
};

export enum Action {
  EditMembers,
  EditOwners,
  EditTitleOrDescription,
  EditMembersEligibility,
  EditOwnersEligibility,
  EditMembersGrants,
  EditOwnersGrants,
  EditAudit,
  Delete,
}

export function isActionForbidden(props: AccessProps): boolean {
  return getEditAccess(props) !== EditAccess.Allowed;
}

export function getActionForbiddenInfo(props: AccessProps): string | undefined {
  const access = getEditAccess(props);
  const { action } = props;
  switch (access) {
    case EditAccess.Allowed:
      return undefined;
    case EditAccess.ForbiddenRbac:
      if (action === Action.Delete) {
        return noDeleteAccessMsg;
      } else {
        return noEditAccessMsg;
      }
    case EditAccess.ForbiddenOkta:
      return readOnlyMsg({
        accessKind: action,
        extraInfo: oktaExtraInfo,
      });
    case EditAccess.ForbiddenOktaReadOnly:
      return readOnlyMsg({
        accessKind: action,
        extraInfo: oktaReadOnlyExtraInfo,
      });
    case EditAccess.ForbiddenReadOnlyType:
      return readOnlyMsg({
        accessKind: action,
        extraInfo: readOnlyTypeExtraInfo,
      });
    case EditAccess.ForbiddenScim:
      return readOnlyMsg({
        accessKind: action,
        extraInfo: scimExtraInfo,
      });
    case EditAccess.ForbiddenPreset:
      return 'Go to "Access Definition" tab to edit member access';
    case EditAccess.ForbiddenPresetDelete:
      return `Insufficient permissions to delete this access list created with a guide. Missing role
        permissions: ${props.missingRolePerms.join(', ')}`;
    default:
      access satisfies never;
  }
}

enum EditAccess {
  Allowed,
  ForbiddenRbac,
  ForbiddenOkta,
  ForbiddenReadOnlyType,
  ForbiddenOktaReadOnly,
  ForbiddenScim,
  ForbiddenPreset,
  ForbiddenPresetDelete,
}

function getEditAccess({
  accessList,
  isReadOnlyOktaList,
  action,
  perms,
  missingRolePerms = [],
}: AccessProps): EditAccess {
  const isReadOnlyTypeList = isReadOnly(accessList.type);
  const isOktaList = accessList.origin === AccessListOrigin.Okta;

  // Editing members is a special case, because access_list_member is a separate resource.
  if (action === Action.EditMembers) {
    const canEditMembers = perms.isOwner || perms.adminWhoCanEdit;
    if (!canEditMembers) return EditAccess.ForbiddenRbac;
    if (isReadOnlyOktaList) return EditAccess.ForbiddenOktaReadOnly;
    if (isReadOnlyTypeList) return EditAccess.ForbiddenReadOnlyType;
    if (isScim(accessList.type)) return EditAccess.ForbiddenScim;
    return EditAccess.Allowed;
  }

  // RBAC check should go first to avoid leaking info about access list properties.
  if (action === Action.Delete) {
    if (!perms.adminWhoCanDelete) return EditAccess.ForbiddenRbac;

    if (
      isGuideEditorSupported(accessList.preset) &&
      missingRolePerms.length > 0
    ) {
      return EditAccess.ForbiddenPresetDelete;
    }
  } else {
    if (!perms.adminWhoCanEdit) return EditAccess.ForbiddenRbac;
  }

  // For Okta lists title and grants can be defined only in Okta.
  if (isOktaList) {
    switch (action) {
      case Action.EditTitleOrDescription:
      case Action.EditMembersGrants:
      case Action.EditOwnersGrants:
        return EditAccess.ForbiddenOkta;
    }
  }

  if (isReadOnlyTypeList) return EditAccess.ForbiddenReadOnlyType;
  if (isReadOnlyOktaList) return EditAccess.ForbiddenOktaReadOnly;

  if (
    action === Action.EditMembersGrants &&
    isGuideEditorSupported(accessList.preset)
  ) {
    return EditAccess.ForbiddenPreset;
  }

  return EditAccess.Allowed;
}

const noEditAccessMsg = 'You do not have permission to edit this Access List';

const noDeleteAccessMsg =
  'You do not have permission to delete this Access List';

const oktaExtraInfo = 'this Access List is managed by Okta';

const readOnlyTypeExtraInfo =
  'this Access List type is managed by IaC tools or an integration and is read-only in the web UI';

const oktaReadOnlyExtraInfo =
  'this Access List is managed by Okta and is read-only in Teleport';

const scimExtraInfo =
  'this Access List membership is managed by your SCIM provider';

function readOnlyMsg({
  accessKind,
  extraInfo,
}: {
  accessKind: Action;
  extraInfo: string;
}): string {
  switch (accessKind) {
    case Action.EditMembers:
      return `Editing members is disabled; ${extraInfo}`;
    case Action.EditOwners:
      return `Editing owners is disabled; ${extraInfo}`;
    case Action.EditTitleOrDescription:
      return `Editing title or description is disabled; ${extraInfo}`;
    case Action.EditMembersEligibility:
      return `Editing members is disabled; ${extraInfo}`;
    case Action.EditOwnersEligibility:
      return `Editing owners is disabled; ${extraInfo}`;
    case Action.EditMembersGrants:
      return `Editing members is disabled; ${extraInfo}`;
    case Action.EditOwnersGrants:
      return `Editing owners is disabled; ${extraInfo}`;
    case Action.EditAudit:
      return `Editing audit is disabled; ${extraInfo}`;
    case Action.Delete:
      return `Deleting this Access List is disabled; ${extraInfo}`;
    default:
      accessKind satisfies never;
  }
}
