import { Role, RoleConditions } from 'teleport/services/resources';

import {
  emptyAppIdentities,
  RequiredAppIdentitiesWithFetchResult,
} from '../resources/app';
import { emptyDbIdentities } from '../resources/db';
import { emptyDesktopIdentities } from '../resources/desktop';
import { emptyGitHubIdentities } from '../resources/github';
import { emptyKubeIdentities } from '../resources/kube';
import { emptyLinuxDesktopIdentities } from '../resources/linux_desktop';
import { emptyServerIdentities } from '../resources/server';

export type RequiredRoleConditions = Required<RoleConditions>;

/**
 * Role conditions for standard resources (apps, databases, servers,
 * desktops, kubernetes, git).
 *
 * Standard refers to "non-specialized" resources. For example,
 * AWS Identity Center applications are considered "specialized" resources
 * despite it being part of the apps family due to unique constraints and
 * is handled by using a separate role (see AwsIcRoleConditions).
 *
 * Built from RoleConditions with irrelevant fields omitted and
 * relevant fields required. This forces new fields to be addressed.
 */
export type StandardRoleConditions = Omit<
  RequiredRoleConditions,
  // account_assignments is omitted here because it is AWS IC specific
  // which has its own role condition - see AwsIcRoleConditions
  'rules' | 'db_service_labels' | 'db_roles' | 'account_assignments'
>;

export const defaultStandardRoleConditions = (): StandardRoleConditions => ({
  app_labels: {},
  db_labels: {},
  windows_desktop_labels: {},
  linux_desktop_labels: {},
  kubernetes_labels: {},
  node_labels: {},

  ...emptyGitHubIdentities(),
  ...emptyAppIdentities(),
  ...emptyDbIdentities(),
  ...emptyKubeIdentities(),
  ...emptyServerIdentities(),
  ...emptyDesktopIdentities(),
  ...emptyLinuxDesktopIdentities(),
});

export const requiredRoleConditions = (): RequiredRoleConditions => ({
  app_labels: {},
  db_labels: {},
  windows_desktop_labels: {},
  linux_desktop_labels: {},
  kubernetes_labels: {},
  node_labels: {},

  rules: [],
  db_service_labels: {},
  db_roles: [],
  account_assignments: [],

  github_permissions: [],

  aws_role_arns: [],
  azure_identities: [],
  gcp_service_accounts: [],
  mcp: {},
  windows_desktop_logins: [],
  linux_desktop_logins: [],
  logins: [],
  db_names: [],
  db_users: [],
  kubernetes_groups: [],
  kubernetes_resources: [],
  kubernetes_users: [],
});

export const requiredRoleConditionFieldNames = Object.keys(
  requiredRoleConditions()
) as Array<keyof RequiredRoleConditions>;

export function extractAllowRoleConditionsFromRole(
  role: Role
): StandardRoleConditions {
  const allow = role.spec.allow ?? {};
  return {
    // app access + identities
    app_labels: allow.app_labels || {},
    aws_role_arns: allow.aws_role_arns || [],
    azure_identities: allow.azure_identities || [],
    gcp_service_accounts: allow.gcp_service_accounts || [],
    mcp: allow.mcp?.tools?.length ? allow.mcp : { tools: [] },

    // db access + identities
    db_labels: allow.db_labels || {},
    db_names: allow.db_names || [],
    db_users: allow.db_users || [],

    // desktop access + identities
    windows_desktop_labels: allow.windows_desktop_labels || {},
    windows_desktop_logins: allow.windows_desktop_logins || [],

    // linux desktop access + identities
    linux_desktop_labels: allow.linux_desktop_labels || {},
    linux_desktop_logins: allow.linux_desktop_logins || [],

    // kube access + identities
    kubernetes_labels: allow.kubernetes_labels || {},
    kubernetes_groups: allow.kubernetes_groups || [],
    kubernetes_users: allow.kubernetes_users || [],
    kubernetes_resources: allow.kubernetes_resources || [],

    // server access + identities
    node_labels: allow.node_labels || {},
    logins: allow.logins || [],

    // git server access
    github_permissions: allow.github_permissions || [],
  };
}

export function extractRequiredAppIdentitiesFromRole(
  role: Role
): RequiredAppIdentitiesWithFetchResult {
  const allow = role.spec.allow ?? {};

  return {
    aws_role_arns: allow.aws_role_arns?.length ? [] : undefined,
    azure_identities: allow.azure_identities?.length ? [] : undefined,
    gcp_service_accounts: allow.gcp_service_accounts?.length ? [] : undefined,
    mcp: allow.mcp?.tools?.length ? { tools: [] } : undefined,
    allPagesFetched: false,
  };
}
