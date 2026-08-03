import { StandardRoleConditions } from './conditions/standard';

/**
 * Fields that let users list/see resources in their cluster.
 *
 * Controls visibility (what shows up in UI), not to be confused
 * with "identity" access like logins and db_names etc.
 *
 * Examples:
 * - db_labels: {"env": "prod"} -> user sees prod databases
 * - github_permissions: [...] -> user sees matching git servers
 *
 * Derived from StandardRoleConditions with identity fields omitted.
 *
 * Base type for deriving UI-configurable types below this.
 */
type ListResourceConditions = Required<
  Omit<
    StandardRoleConditions,
    // Removes all identity related fields
    | 'aws_role_arns'
    | 'azure_identities'
    | 'gcp_service_accounts'
    | 'mcp'
    | 'db_service_labels'
    | 'db_names'
    | 'db_users'
    | 'db_roles'
    | 'kubernetes_groups'
    | 'kubernetes_resources'
    | 'kubernetes_users'
    | 'logins'
    | 'windows_desktop_logins'
    | 'linux_desktop_logins'
  >
>;

export type ListResourceAccessFields = keyof ListResourceConditions;

/**
 * Object to derive runtime array + type.
 */
const emptyListResourceAccessConditions = (): ListResourceConditions => ({
  app_labels: {},
  db_labels: {},
  windows_desktop_labels: {},
  linux_desktop_labels: {},
  kubernetes_labels: {},
  node_labels: {},
  github_permissions: [],
});

/**
 * Array for iteration
 */
export const listResourceAccessFields = Object.keys(
  emptyListResourceAccessConditions()
) as ListResourceAccessFields[];

/**
 * Resource "types" definable by the guide editor.
 *
 * Extends ListResourceConditions with "awsIc" which is a UI
 * concept, not a real role field. See AwsIcRoleConditions for details.
 */
type DefinableResourceConditions = ListResourceConditions & {
  // Value type is not important, only key existence to derive
  // a type from it.
  awsIc: any;
};

export type DefinableResourceAccessFields = keyof DefinableResourceConditions;

/**
 * Object to derive runtime array + type.
 */
const emptyDefinableAccess = (): DefinableResourceConditions => ({
  // Order matters - determines tab rendering order in ResourceTabs.tsx.
  // Object.keys() preserves string key order
  app_labels: {},
  awsIc: {},
  db_labels: {},
  windows_desktop_labels: {},
  linux_desktop_labels: {},
  github_permissions: [],
  kubernetes_labels: {},
  node_labels: {},
});

/**
 * Array for iteration in switch statements.
 */
export const definableResourceAccessFields = Object.keys(
  emptyDefinableAccess()
) as Array<DefinableResourceAccessFields>;

/**
 * Defines which fields are label based access.
 */
type LabelBasedResourceAccess = Omit<
  ListResourceConditions,
  'github_permissions'
>;

export type LabelBasedResourceAccessFields = keyof LabelBasedResourceAccess;

/**
 * Object to derive runtime array + type.
 */
const emptyLabelBasedResourceAccess = (): LabelBasedResourceAccess => ({
  app_labels: {},
  db_labels: {},
  windows_desktop_labels: {},
  linux_desktop_labels: {},
  kubernetes_labels: {},
  node_labels: {},
});

/**
 * Array for iteration in switch statements.
 */
export const labelBasedResourceAccessFields = Object.keys(
  emptyLabelBasedResourceAccess()
) as Array<LabelBasedResourceAccessFields>;
