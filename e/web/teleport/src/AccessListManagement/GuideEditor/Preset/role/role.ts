import { AccessListModified } from 'e-teleport/AccessListManagement/ViewEditAccessList/Shared';
import { defaultRoleVersion } from 'teleport/Roles/RoleEditor/StandardEditor/standardmodel';
import { optionsWithDefaults } from 'teleport/Roles/RoleEditor/StandardEditor/withDefaults';
import {
  Labels,
  Role,
  RoleConditions,
  RoleVersion,
} from 'teleport/services/resources';
import { Access } from 'teleport/services/user';

import { MinimumRoleVersionSupported } from '../../useGuideEditor';
import {
  AwsIcRoleConditions,
  convertAwsAccountMapToRoleType,
  PluginTypeAwsIdentityCenter,
  requiredRoleConditionFieldNames,
  StandardRoleConditions,
} from './conditions';
import { hasLabels, TeleportOriginLabelKey } from './label';
import { definableResourceAccessFields } from './listaccess';

export const wildcard = '*';

/**
 * Prefix of roles that are related to giving access to resources.
 *
 * e.g: {access}-awsic-acl-preset-AccessListID
 */
export const roleForAccessPrefix = 'access';

/**
 * Prefix of roles that require users to request for access.
 *
 * e.g: {requester}-acl-preset-AccessListID
 */
export const roleForRequesterPrefix = 'requester';

/**
 * Middle part of the role name that identifies the role was created
 * for an access list using a preset (guide editor).
 *
 * e.g: access-awsic{-acl-preset-}AccessListID
 */
export const roleInfix = '-acl-preset-';

/**
 * Part of a role name that identifies the role defines access specifically
 * for AWS identity center
 *
 * e.g: access-{awsic}-acl-preset-AccessListID
 */
export const awsIcRoleAccessKind = 'awsic';

/**
 * Part of a role name that identifies the role defines access to standard
 * resources (non specialized resources unlike AWS IC).
 *
 * e.g: access-{standard}-acl-preset-AccessListID
 */
export const standardRoleAccessKind = 'standard';

/**
 * Types of roles supported by the guide editor.
 */
export type AccessRoleKind =
  | typeof awsIcRoleAccessKind
  | typeof standardRoleAccessKind;

/**
 * States for when resource access is being edited by the guide editor.
 */
export type RoleEditState = {
  /**
   * The original untouched reference to a fetched role.
   * Should not be modified. It is used as a reference
   * to determine if changes are made.
   *
   * This field will be null if there was no role created.
   */
  original: Role | null;
  /**
   * Becomes true once states are mutated by the editor.
   */
  isDirty: boolean;
};

type RoleAccess = {
  hasAccess: boolean;
  name: string;
};

/**
 * Returns a list of missing role verbs depending on
 * what type of role action is requested.
 *
 * Returning an empty list means user met all requirements
 * for the given action (has access).
 *
 * Using the guide editor requires role access since
 * Teleport will perform role operations under the hood.
 */
export function getMissingRoleAccess(
  roleAccess: Access,
  action: 'crud' | 'write' | 'read'
): string[] {
  const readAccess: RoleAccess[] = [
    { hasAccess: roleAccess.list, name: 'role.list' },
    { hasAccess: roleAccess.read, name: 'role.read' },
  ];

  const writeAccess: RoleAccess[] = [
    { hasAccess: roleAccess.create, name: 'role.create' },
    { hasAccess: roleAccess.edit, name: 'role.update' },
    { hasAccess: roleAccess.remove, name: 'role.delete' },
  ];

  let access: RoleAccess[] = [];
  switch (action) {
    case 'crud':
      access = [...readAccess, ...writeAccess];
      break;
    case 'read':
      access = [...readAccess];
      break;
    case 'write':
      access = [...writeAccess];
      break;
    default:
      action satisfies never;
  }
  return access.filter(rule => !rule.hasAccess).map(rule => rule.name);
}

export function getRoleSuffix(accessListId: string) {
  return `-acl-preset-${accessListId}`;
}

export function getRolePrefixForAccess(kind: AccessRoleKind) {
  switch (kind) {
    case 'awsic':
      return `${roleForAccessPrefix}-${awsIcRoleAccessKind}`;
    case 'standard':
      return `${roleForAccessPrefix}-${standardRoleAccessKind}`;
    default:
      kind satisfies never;
  }
}

/**
 * Determines role access type based on role name pattern.
 */
export function getAccessRoleKind(
  role: Role,
  accessListId: string
): AccessRoleKind {
  const idSuffix = getRoleSuffix(accessListId);
  if (
    role.metadata.name === `${getRolePrefixForAccess('standard')}${idSuffix}`
  ) {
    return 'standard';
  }
  if (role.metadata.name === `${getRolePrefixForAccess('awsic')}${idSuffix}`) {
    return 'awsic';
  }
}

/**
 * Checks if a known RoleConditions field for AWS IC role has an unsupported
 * value. Returns true if the field is unsupported, false if supported.
 */
function isAwsIcRoleKnownFieldUnsupported(
  fieldName: keyof RoleConditions,
  allow: RoleConditions
): boolean {
  switch (fieldName) {
    // Supported fields - these are handled by the guide editor
    case 'app_labels':
      const labels = allow[fieldName] ?? {};
      return Object.keys(labels).length > 1 || !hasAwsIcAppLabel(labels);

    case 'account_assignments':
      return false;

    // Unsupported fields.
    case 'db_labels':
    case 'kubernetes_labels':
    case 'node_labels':
    case 'windows_desktop_labels':
    case 'db_service_labels':
      return hasLabels(allow[fieldName]);

    case 'windows_desktop_logins':
    case 'db_names':
    case 'db_users':
    case 'kubernetes_groups':
    case 'kubernetes_users':
    case 'kubernetes_resources':
    case 'logins':
    case 'github_permissions':
    case 'azure_identities':
    case 'aws_role_arns':
    case 'gcp_service_accounts':
    case 'db_roles':
    case 'rules':
      return (allow[fieldName]?.length ?? 0) > 0;

    case 'mcp':
      return (allow[fieldName]?.tools?.length ?? 0) > 0;

    default:
      fieldName satisfies never;
  }
  return true; // unsupported by default
}

function hasAwsIcAppLabel(labels?: Labels) {
  if (!labels) {
    return false;
  }
  return (
    labels[TeleportOriginLabelKey] &&
    labels[TeleportOriginLabelKey] === PluginTypeAwsIdentityCenter
  );
}

/**
 * Checks if a known RoleConditions field for a standard role has an
 * unsupported value. Returns true if the field is unsupported, false
 * if supported.
 */
function isStandardRoleKnownFieldUnsupported(
  fieldName: keyof RoleConditions,
  allow: RoleConditions
): boolean {
  // Interpolation can be used to refer to user traits.
  const hasInterpolation = labelVal =>
    labelVal.startsWith('{{') && labelVal.endsWith('}}');

  switch (fieldName) {
    // Supported fields - these are handled by the guide editor
    case 'app_labels':
    case 'db_labels':
    case 'kubernetes_labels':
    case 'node_labels':
    case 'windows_desktop_labels':
      // Check for interpolation which is not supported.
      const labels = allow[fieldName] ?? {};
      const labelKeys = Object.keys(labels);
      if (!labelKeys.length) {
        return false;
      }

      for (const key in labels) {
        let labelVals = labels[key];
        if (!Array.isArray(labelVals)) {
          labelVals = [labelVals];
        }
        for (const index in labelVals) {
          if (hasInterpolation(labelVals[index])) {
            return true;
          }
        }
      }

      // AWS IC support has it's own role and should not
      // be defined in the standard role (it won't get parsed by the editor)
      if (fieldName === 'app_labels') {
        return hasAwsIcAppLabel(labels);
      }

      return false;

    case 'windows_desktop_logins':
    case 'db_names':
    case 'db_users':
    case 'kubernetes_groups':
    case 'kubernetes_users':
    case 'kubernetes_resources':
    case 'logins':
    case 'mcp':
    case 'azure_identities':
    case 'aws_role_arns':
    case 'gcp_service_accounts':
    case 'github_permissions':
      return false;

    case 'account_assignments':
      return (allow[fieldName]?.length ?? 0) > 0;

    // Unsupported "known" fields - we know about these but don't support them
    case 'db_roles':
    case 'rules':
      return (allow[fieldName]?.length ?? 0) > 0;

    case 'db_service_labels':
      return hasLabels(allow[fieldName]);

    default:
      fieldName satisfies never;
  }
  return true; // unsupported by default
}

/**
 * Returns true if any deny field is present.
 * Deny rules are not supported by the guide editor and should
 * always be empty.
 */
export function roleHasDenyFields(role: Role) {
  const denyConditions = role.spec?.deny ?? {};
  return Object.keys(denyConditions).length > 0;
}

/**
 * Valiates requester role only has field "allow.request.search_as_roles"
 * defined.
 *
 * Returns true if an unsupported field has a value.
 */
export function roleHasUnsupportedRequesterFields(role: Role) {
  const allow = role.spec?.allow ?? {};
  const allowFieldNames = Object.keys(allow);
  if (!allowFieldNames.length) {
    return false;
  }

  const requestFieldName = 'request';

  return allowFieldNames.some(allowFieldName => {
    if (allowFieldName !== requestFieldName) {
      return unknownFieldHasValue(allow[allowFieldName]);
    }

    const requestField = allow[requestFieldName] ?? {};
    const requestFieldNames = Object.keys(requestField);

    if (!requestFieldNames.length) {
      return false;
    }

    for (const requestFieldName of requestFieldNames) {
      if (requestFieldName !== 'search_as_roles') {
        return unknownFieldHasValue(allow[allowFieldName]);
      }
    }
  });
}

export function roleHasUnsupportedAllowFieldsDefined(
  role: Role,
  accessKind: AccessRoleKind
) {
  const allow = role.spec?.allow ?? {};
  const allowFieldNames = Object.keys(allow);
  if (!allowFieldNames.length) {
    return false;
  }

  switch (accessKind) {
    case 'awsic':
      if (!hasAwsIcAppLabel(allow['app_labels'] ?? {})) {
        return true;
      }
      const accountAssignments = allow['account_assignments'];
      if (!accountAssignments || accountAssignments.length === 0) {
        return true;
      }
      break;
    case 'standard':
      break;
    default:
      accessKind satisfies never;
  }

  return allowFieldNames.some(allowFieldName => {
    // Check if this is a known field from RoleConditions
    const knownFieldName = allowFieldName as keyof RoleConditions;
    if (requiredRoleConditionFieldNames.includes(knownFieldName)) {
      switch (accessKind) {
        case 'awsic':
          return isAwsIcRoleKnownFieldUnsupported(knownFieldName, allow);
        case 'standard':
          return isStandardRoleKnownFieldUnsupported(knownFieldName, allow);
        default:
          accessKind satisfies never;
      }
    }

    // All unknown fields with values will be treated as unsupported.
    // Unknown fields can happen if the backend added new role fields
    // but the frontend RoleConditions type didn't get updated.
    return unknownFieldHasValue(allow[allowFieldName]);
  });
}

function unknownFieldHasValue(val: unknown) {
  if (!val) {
    return false;
  }
  if (Array.isArray(val)) {
    return val.length > 0;
  }
  if (typeof val === 'object') {
    return Object.keys(val).length > 0;
  }
  // The other types of data is non empty string or number
  return !!val;
}

export function roleHasAccessDefined(role: Role) {
  if (!role) {
    return false;
  }

  const allow = role.spec?.allow ?? {};

  return definableResourceAccessFields.some(field => {
    switch (field) {
      case 'app_labels':
      case 'db_labels':
      case 'kubernetes_labels':
      case 'node_labels':
      case 'windows_desktop_labels':
        return hasLabels(allow[field]);
      case 'github_permissions':
        return allow.github_permissions?.length > 0;
      case 'awsIc':
        return (
          hasAwsIcAppLabel(allow['app_labels']) &&
          allow.account_assignments?.length > 0
        );

      default:
        field satisfies never;
    }
  });
}

export function roleVersionSupported(role: Role) {
  if (role.version !== MinimumRoleVersionSupported) {
    const gotNumVersion = Number(role.version.slice(1));
    const minNumVersion = Number(MinimumRoleVersionSupported.slice(1));
    if (Number.isNaN(gotNumVersion) || Number.isNaN(minNumVersion)) {
      return false;
    }
    return gotNumVersion > minNumVersion;
  }
  return true;
}

export function isInvalidRole(role: Role, accessKind: AccessRoleKind) {
  return (
    // Nothing stops the user from editing these roles.
    !roleVersionSupported(role) ||
    roleHasDenyFields(role) ||
    roleHasUnsupportedAllowFieldsDefined(role, accessKind)
  );
}

export function hasInternalAccessListPresetLabel(
  accessListId: string,
  labels?: Record<string, string>
) {
  if (!labels) {
    return false;
  }
  return labels['teleport.internal/access-list-preset'] === accessListId;
}

export type QueriedRoleState = {
  status:
    | 'valid-roles'
    | 'no-access-defined'
    | 'unsupported-role-fields'
    | 'unknown-roles';
  awsIcRole?: Role;
  standardRole?: Role;
};

/**
 * Validates roles queried for an access list managed by the guide editor.
 *
 * Roles for guide editor access lists are auto-created by the backend.
 * While these roles should be edited via the guide editor, users can
 * modify them directly with tctl.
 *
 * Validation rules:
 * - Only known roles are accepted (validated by role name pattern and
 *   internal label).
 * - Only known/supported role fields are accepted.
 * - All granted roles must exist among the queried roles.
 *
 * If any anomaly is detected, validation fails because the guide editor
 * cannot accurately represent the access configuration. Users will be
 * directed to use the role editor instead.
 *
 * @returns
 * - `unsupported-role-fields`: Role has unsupported version or fields.
 * - `unknown-roles`: Granted roles contain unrecognized role names.
 * - `valid-roles`: Roles are valid and have access defined.
 * - `no-access-defined`: No access is defined.
 */
export function validateQueriedRoles({
  gotRoles = [],
  accessList,
}: {
  gotRoles: Role[];
  accessList: AccessListModified;
}): QueriedRoleState {
  const grantedRoleNames = accessList.grants.roles;

  // No grants assigned. This is valid when:
  // - User didn't define access when creating the access list, or
  // - User manually unassigned roles via tctl.
  // Editing via the guide editor will upsert and assign proper roles.
  if (!grantedRoleNames.length) {
    return { status: 'no-access-defined' };
  }

  // Filter queried roles by name prefix and internal label to identify
  // roles created by the guide editor for this access list.
  const gotAccessRoles = gotRoles
    .filter(role => role.metadata.name.startsWith(`${roleForAccessPrefix}-`))
    .filter(role =>
      hasInternalAccessListPresetLabel(accessList.id, role.metadata.labels)
    );

  const gotRequesterRoles = gotRoles
    .filter(role => role.metadata.name.startsWith(`${roleForRequesterPrefix}-`))
    .filter(role =>
      hasInternalAccessListPresetLabel(accessList.id, role.metadata.labels)
    );

  const gotAccessRoleNames = gotAccessRoles.map(role => role.metadata.name);

  // Validate that granted roles exist among queried roles.
  // Only proceed with access roles that are both known and granted.
  let grantValidatedAccessRoles: Role[] = [...gotAccessRoles];
  const preset = accessList.preset;
  switch (preset) {
    // Long-term: roles are assigned directly to members as standing access.
    // Granted roles should match the queried access roles.
    case 'long-term':
      // Reject if any granted role is not among known access roles.
      // This happens when user manually assigns unknown roles via tctl.
      const unknownRoles = grantedRoleNames.filter(
        memberRole => !gotAccessRoleNames.includes(memberRole)
      );
      if (unknownRoles.length > 0) {
        return { status: 'unknown-roles' };
      }
      // Keep only granted roles. User may have removed some via tctl.
      grantValidatedAccessRoles = gotAccessRoles.filter(gotRole =>
        grantedRoleNames.includes(gotRole.metadata.name)
      );

      break;

    // Short-term: roles are assigned indirectly via a "requester" role.
    // Users must request access, and the actual access roles are stored
    // in the requester role's `request.search_as_roles` field.
    case 'short-term':
      // Expect exactly one requester role that matches the grant.
      if (
        grantedRoleNames.length != 1 ||
        gotRequesterRoles.length != 1 ||
        grantedRoleNames[0] != gotRequesterRoles[0].metadata.name
      ) {
        return { status: 'unknown-roles' };
      }

      const requesterRole = gotRequesterRoles[0];

      // If a user manually edited this role like adding "resource access",
      // those fields won't be parsed, which will produce inaccurate previewing
      // of what resources members have access to.
      if (
        !roleVersionSupported(requesterRole) ||
        roleHasDenyFields(requesterRole) ||
        roleHasUnsupportedRequesterFields(requesterRole)
      ) {
        return { status: 'unsupported-role-fields' };
      }

      const allow = requesterRole.spec?.allow ?? {};
      const request = allow['request'];
      const searchAsRoles: string[] =
        request && request['search_as_roles'] ? request['search_as_roles'] : [];

      // Reject if search_as_roles contains unknown roles (likely added
      // manually via role editor).
      if (
        !searchAsRoles.every(searchAsRole =>
          gotAccessRoleNames.includes(searchAsRole)
        )
      ) {
        return { status: 'unknown-roles' };
      }

      // Keep only roles referenced in search_as_roles.
      // User may have removed some via tctl.
      grantValidatedAccessRoles = gotAccessRoles.filter(gotRole =>
        searchAsRoles.includes(gotRole.metadata.name)
      );

      break;

    case '':
      break;

    default:
      preset satisfies never;
  }

  // Categorize validated roles by access kind (standard vs AWS IC).
  // Track any roles that don't match a known kind.
  let standardRole: Role;
  let awsIcRole: Role;
  let unknownRoles = 0;

  grantValidatedAccessRoles.forEach(role => {
    const accessKind = getAccessRoleKind(role, accessList.id);
    switch (accessKind) {
      case 'standard':
        standardRole = role;
        return;
      case 'awsic':
        awsIcRole = role;
        return;
      default:
        unknownRoles += 1;
        accessKind satisfies never;
    }
  });

  // Fail if any role couldn't be categorized. The guide editor cannot
  // accurately represent access for unknown role kinds.
  if (unknownRoles > 0) {
    return { status: 'unsupported-role-fields' };
  }

  // Reject roles with unsupported fields (e.g., user added fields via tctl
  // or role editor that the guide editor doesn't support).
  if (standardRole && isInvalidRole(standardRole, 'standard')) {
    return { status: 'unsupported-role-fields' };
  }
  if (awsIcRole && isInvalidRole(awsIcRole, 'awsic')) {
    return { status: 'unsupported-role-fields' };
  }

  if (!standardRole && !awsIcRole) {
    return { status: 'no-access-defined' };
  }

  // Role may exist but define no actual resource access.
  if (!roleHasAccessDefined(standardRole) && !roleHasAccessDefined(awsIcRole)) {
    return { status: 'no-access-defined', awsIcRole, standardRole };
  }

  // Validation passed with access defined.
  return { status: 'valid-roles', awsIcRole, standardRole };
}

export function newAccessRole({
  kind,
  roleConditions,
  roleVersion = defaultRoleVersion,
}:
  | {
      kind: typeof awsIcRoleAccessKind;
      roleConditions: AwsIcRoleConditions;
      roleVersion?: RoleVersion;
    }
  | {
      kind: typeof standardRoleAccessKind;
      roleConditions: StandardRoleConditions;
      roleVersion?: RoleVersion;
    }): Role {
  if (kind === awsIcRoleAccessKind) {
    return {
      kind: 'role',
      version: roleVersion,
      metadata: { name: getRolePrefixForAccess('awsic') },
      spec: {
        deny: {},
        options: optionsWithDefaults(roleVersion),
        allow: {
          app_labels: roleConditions.labels,
          account_assignments: convertAwsAccountMapToRoleType(
            roleConditions.account
          ),
        },
      },
    };
  }

  return {
    kind: 'role',
    version: roleVersion,
    metadata: { name: getRolePrefixForAccess('standard') },
    spec: {
      deny: {},
      options: optionsWithDefaults(roleVersion),
      allow: { ...roleConditions },
    },
  };
}
