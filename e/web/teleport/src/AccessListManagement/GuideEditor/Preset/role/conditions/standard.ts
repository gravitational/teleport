import { defaultRoleVersion } from 'teleport/Roles/RoleEditor/StandardEditor/standardmodel';
import { RoleConditions } from 'teleport/services/resources';

import { emptyAppIdentities } from '../resources/app';
import { emptyDbIdentities } from '../resources/db';
import { emptyDesktopIdentities } from '../resources/desktop';
import { emptyGitHubIdentities } from '../resources/github';
import { emptyKubeIdentities } from '../resources/kube';
import { emptyServerIdentities } from '../resources/server';
import { WithRoleVersion } from '../role';

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
export type StandardRoleConditions = Required<
  Omit<
    RoleConditions,
    // account_assignments is omitted here because it is AWS IC specific
    // which has its own role condition - see AwsIcRoleConditions
    'rules' | 'db_service_labels' | 'db_roles' | 'account_assignments'
  >
> &
  WithRoleVersion;

export const defaultStandardRoleConditions = (): StandardRoleConditions => ({
  roleVersion: defaultRoleVersion,

  app_labels: {},
  db_labels: {},
  windows_desktop_labels: {},
  kubernetes_labels: {},
  node_labels: {},

  ...emptyGitHubIdentities(),
  ...emptyAppIdentities(),
  ...emptyDbIdentities(),
  ...emptyKubeIdentities(),
  ...emptyServerIdentities(),
  ...emptyDesktopIdentities(),
});
