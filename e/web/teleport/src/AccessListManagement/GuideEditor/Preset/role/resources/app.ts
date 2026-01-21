import {
  ApplicationResourceAccess,
  MCPPermissions,
} from 'teleport/services/resources';

/**
 * Identities users can assume when connecting to an application.
 *
 * Derived from ApplicationResourceAccess with irrelevant fields
 * omitted and relevant fields required.
 */
export type AppIdentities = Required<
  // account_assignments is AWS IC specific - see AwsIcRoleConditions
  Omit<ApplicationResourceAccess, 'app_labels' | 'account_assignments'>
> & {
  // mcp is just re-typed to make all fields in MCPPermissions required
  mcp: Required<MCPPermissions>;
};

/**
 * Tracks which app identities are required based on queried apps.
 *
 * Not all application identities are required if the application access
 * definition does not require them.
 * E.g. if app_labels only match AWS console apps, only
 * aws_role_arns is required - other identities can be hidden.
 *
 * Helps determine which identity fields to show/hide in UI.
 */
export type RequiredAppIdentitiesWithFetchResult = AppIdentities & {
  /**
   * True when all app pages have been fetched.
   * Used to determine if more querying is needed.
   */
  allPagesFetched: boolean;
};

export const emptyAppIdentities = (): AppIdentities => ({
  aws_role_arns: [],
  azure_identities: [],
  gcp_service_accounts: [],
  mcp: { tools: [] },
});

export const emptyRequiredAppIdentitiesWithFetchResult =
  (): RequiredAppIdentitiesWithFetchResult => ({
    aws_role_arns: undefined,
    azure_identities: undefined,
    gcp_service_accounts: undefined,
    mcp: { tools: undefined },
    allPagesFetched: false,
  });

/**
 *  Array for iteration in switch statements.
 */
export const appIdentityFieldNames = Object.keys(emptyAppIdentities()) as Array<
  keyof AppIdentities
>;
