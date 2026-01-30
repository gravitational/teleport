import { useState } from 'react';

import { AppSubKind } from 'shared/services';

import { App, CloudInstance } from 'teleport/services/apps';
import { Labels, Role } from 'teleport/services/resources';

import {
  defaultStandardRoleConditions,
  extractAllowRoleConditionsFromRole,
  extractRequiredAppIdentitiesFromRole,
  StandardRoleConditions,
} from '../role/conditions';
import {
  labelBasedResourceAccessFields,
  LabelBasedResourceAccessFields,
  listResourceAccessFields,
  ListResourceAccessFields,
} from '../role/listaccess';
import {
  AppIdentities,
  appIdentityFieldNames,
  emptyAppIdentities,
  emptyRequiredAppIdentitiesWithFetchResult,
  RequiredAppIdentitiesWithFetchResult,
} from '../role/resources/app';
import { DbIdentities, emptyDbIdentities } from '../role/resources/db';
import {
  DesktopIdentities,
  emptyDesktopIdentities,
} from '../role/resources/desktop';
import { emptyKubeIdentities, KubeIdentities } from '../role/resources/kube';
import {
  emptyServerIdentities,
  ServerIdentities,
} from '../role/resources/server';
import {
  newAccessRole,
  RoleEditState,
  standardRoleAccessKind,
  wildcard,
} from '../role/role';

export type StandardRoleState = {
  /**
   * Tracks the editing state when modifying an existing role. Contains the
   * original role for comparison and an isDirty flag indicating unsaved changes.
   * Null when creating a new role from scratch.
   */
  roleEditState: RoleEditState;
  /**
   * Initializes editing state. When a role is provided, extracts its
   * conditions for editing. When omitted, resets to defaults for new
   * role creation.
   */
  initRoleEditState(role?: Role): void;
  /**
   * Returns the role to be saved based on the current edit state.
   *
   * Behavior:
   * - If unchanged (not dirty) with existing role: returns original role as-is
   * - If modified (dirty) with existing role:
   *   - Returns undefined if all access was removed (signals backend to remove
   *     access from grants and requester roles)
   *   - Returns modified role with updated conditions otherwise
   * - If modified (dirty) without existing role: returns a new role
   * - If unchanged without existing role: returns undefined (nothing to save)
   */
  getRoleToSave(): Role | undefined;

  /**
   * Defines access to standard (non-specialized) resources like apps, dbs,
   * servers, desktops, etc. Specialized resources like AWS IC applications
   * have their own role state (see useAwsIcRoleState).
   */
  roleConditions: StandardRoleConditions;

  /**
   * Resets roleConditions and requiredAppIdentities back to their
   * default empty states.
   */
  reset(): void;

  /**
   * Sets the entire role conditions state. Useful for initializing state
   * in tests/stories.
   */
  setRoleConditions(conditions: StandardRoleConditions): void;

  /**
   * Tracks which app identity fields require user input based on the types
   * of apps matched by the user's `app_labels` filter. An undefined/null
   * field means that identity type is not required.
   *
   * Apps have multiple identity types (Azure, GCP, MCP, AWS Console) and
   * only the relevant ones need to be shown. For example, if `app_labels`
   * only matches Azure and GCP apps, we only prompt for `azure_identities`
   * and `gcp_service_accounts`, hiding irrelevant fields for better UX.
   *
   * This state caches identity requirements discovered during app preview
   * to minimize redundant queries. If all matching apps fit in a single
   * page response, no additional queries are needed.
   */
  requiredAppIdentities: RequiredAppIdentitiesWithFetchResult;
  /**
   * Resets requiredAppIdentities back to empty (no identities required).
   */
  clearRequiredAppIdentities(): void;
  /**
   * Returns true if any app identity field is marked as required.
   */
  hasRequiredAppIdentities(): boolean;
  /**
   * Scans fetched apps to determine which identity fields are required.
   * Sets the identity field to an empty array (required) when a matching
   * app type is found. The allPagesFetched flag indicates whether all
   * apps in the cluster matching `app_labels` have been fetched.
   */
  markRequiredAppIdentities(fetchedApps: App[], allPagesFetched: boolean): void;
  /**
   * Marks specific app identity fields as required.
   */
  markAppIdentityFieldsAsRequired(
    fields: (keyof AppIdentities)[],
    allPagesFetched: boolean
  ): void;

  /**
   * Clears identity fields (e.g., db_users, kubernetes_groups) associated
   * with the given resource label field. Returns the updated role conditions.
   */
  clearIdentities(
    field: LabelBasedResourceAccessFields
  ): StandardRoleConditions;

  /**
   * Returns true if the identity definition step can be skipped. This is
   * can be the case when no access is defined, or certain types of resources
   * do not require identities (e.g. certain application kinds do not require
   * identities)
   */
  canSkipDefiningIdentities(): boolean;

  /**
   * Clears identity fields (e.g., db_users, kubernetes_groups) associated
   * with the given resource label field. Returns the updated role conditions.
   */
  clearIdentities(
    field: LabelBasedResourceAccessFields
  ): StandardRoleConditions;

  /**
   * Updates labels for specified resource field in the role condition.
   * When updating labels, it's related identities will also be updated.
   *
   * If labels are empty, related identities are cleared (empty state) since
   * empty labels is "removing access".
   *
   * If labels are defined, related identities are set to its default value
   * (if applicable) if no other values were present.
   *
   * E.g. if updating "db_labels", it's related identities like "db_names"
   * and "db_users" will default to value "wild card" if undefined. The values
   * could've been modified if user defined identities and then went back
   * to modify "db_labels".
   */
  updateLabels(
    field: LabelBasedResourceAccessFields,
    labels: Labels
  ): StandardRoleConditions;

  /**
   * Updates github permissions in the role condition.
   */
  updateGitHubPermissions(orgs: string[]): void;

  /**
   * Returns true if the given resource field has any access defined.
   */
  definedAccess(field: ListResourceAccessFields): boolean;
  /**
   * Returns true if access is defined for any resource types
   */
  hasAnyAccessDefined(): boolean;

  /**
   * Merges the given identity fields into roleConditions. Use this to
   * update identity values (e.g., db_users, logins) after user input.
   */
  updateIdentity(
    identities:
      | DbIdentities
      | AppIdentities
      | KubeIdentities
      | ServerIdentities
      | DesktopIdentities
  ): void;
};

/**
 * Manages role condition state for standard resources.
 */
export function useStandardRoleState(): StandardRoleState {
  const [roleEditState, setRoleEditState] = useState<RoleEditState | null>();

  const [roleConditions, setRoleConditions] = useState(() =>
    defaultStandardRoleConditions()
  );

  const [requiredAppIdentities, setRequiredAppIdentities] =
    useState<RequiredAppIdentitiesWithFetchResult>(() =>
      emptyRequiredAppIdentitiesWithFetchResult()
    );

  function reset() {
    setRoleConditions(defaultStandardRoleConditions());
    setRequiredAppIdentities(emptyRequiredAppIdentitiesWithFetchResult());
    setRoleEditState(null);
  }

  function initRoleEditState(role?: Role) {
    setRoleEditState({
      original: role,
      isDirty: false,
    });
    if (role) {
      setRoleConditions(extractAllowRoleConditionsFromRole(role));
      extractRequiredAppIdentitiesFromRole(role);
    } else {
      setRoleConditions(defaultStandardRoleConditions());
      setRequiredAppIdentities(emptyRequiredAppIdentitiesWithFetchResult());
    }
  }

  function getRoleToSave(): Role {
    // Send unchanged role, otherwise backend will interpret
    // missing role for an existing role as "remove access".
    if (!roleEditState.isDirty && roleEditState.original) {
      return roleEditState.original;
    }

    // Modifying existing role.
    if (roleEditState.isDirty && roleEditState.original) {
      // Not sending an existing role for update is interpreted
      // as "remove access". Backend will remove this access
      // from member grants and requester roles.
      if (!hasAnyAccessDefined()) {
        return undefined;
      }

      // Send modification.
      return {
        ...roleEditState.original,
        spec: {
          ...roleEditState.original.spec,
          allow: { ...roleConditions },
        },
      };
    }

    // Creating a new role b/c:
    // - user removed access but is adding it back
    // - wasn't defined in the first place (users can skip defining access)
    if (roleEditState.isDirty && !roleEditState.original) {
      return newAccessRole({
        kind: standardRoleAccessKind,
        roleConditions,
      });
    }
  }

  function updateRoleConditions(newConditions: StandardRoleConditions) {
    setRoleConditions(newConditions);
    if (roleEditState) {
      setRoleEditState({ ...roleEditState, isDirty: true });
    }
  }

  function markRequiredAppIdentities(
    fetchedApps: App[],
    fetchedAllAppsInCluster: boolean
  ) {
    const newIdentities: RequiredAppIdentitiesWithFetchResult = {
      ...emptyRequiredAppIdentitiesWithFetchResult(),
      allPagesFetched: fetchedAllAppsInCluster,
    };

    for (const app of fetchedApps) {
      for (const field of appIdentityFieldNames) {
        // Setting value other than "undefined/null" is interpreted
        // as "required".
        switch (field) {
          case 'aws_role_arns':
            if (app.awsConsole) {
              newIdentities.aws_role_arns = [];
            }
            break;
          case 'azure_identities':
            if (app.cloudInstance === CloudInstance.Azure) {
              newIdentities.azure_identities = [];
            }
            break;
          case 'gcp_service_accounts':
            if (app.cloudInstance === CloudInstance.Gcp) {
              newIdentities.gcp_service_accounts = [];
            }
            break;
          case 'mcp':
            if (app.subKind === AppSubKind.MCP) {
              newIdentities.mcp = { tools: [] };
            }
            break;
          default:
            field satisfies never;
        }
      }
    }

    setRequiredAppIdentities(newIdentities);
  }

  function clearRequiredAppIdentities() {
    setRequiredAppIdentities(emptyRequiredAppIdentitiesWithFetchResult());
  }

  function markAppIdentityFieldsAsRequired(
    fields: (keyof AppIdentities)[],
    allPagesFetched: boolean
  ) {
    const newIdentities: RequiredAppIdentitiesWithFetchResult = {
      ...requiredAppIdentities,
      allPagesFetched,
    };

    for (const field of fields) {
      switch (field) {
        case 'aws_role_arns':
        case 'azure_identities':
        case 'gcp_service_accounts':
          newIdentities[field] = [];
          break;
        case 'mcp':
          newIdentities.mcp = { tools: [] };
          break;
        default:
          field satisfies never;
      }
    }

    setRequiredAppIdentities(newIdentities);
  }

  function hasRequiredAppIdentities() {
    return appIdentityFieldNames.some(field => {
      switch (field) {
        case 'aws_role_arns':
        case 'azure_identities':
        case 'gcp_service_accounts':
          return requiredAppIdentities[field] != null;
        case 'mcp':
          return requiredAppIdentities['mcp'].tools != null;
        default:
          field satisfies never;
      }
    });
  }

  function updateIdentity(
    identities:
      | DbIdentities
      | AppIdentities
      | KubeIdentities
      | ServerIdentities
      | DesktopIdentities
  ) {
    updateRoleConditions({ ...roleConditions, ...identities });
  }

  function getEmptyIdentities(field: LabelBasedResourceAccessFields) {
    switch (field) {
      case 'app_labels':
        return emptyAppIdentities();
      case 'db_labels':
        return emptyDbIdentities();
      case 'kubernetes_labels':
        return emptyKubeIdentities();
      case 'node_labels':
        return emptyServerIdentities();
      case 'windows_desktop_labels':
        return emptyDesktopIdentities();
      default:
        field satisfies never;
    }
  }

  function clearIdentities(field: LabelBasedResourceAccessFields) {
    const emptyIdentities = getEmptyIdentities(field);
    const updatedConditions = { ...roleConditions, ...emptyIdentities };
    updateRoleConditions(updatedConditions);

    return updatedConditions;
  }

  function getDefaultIdentities(
    field: LabelBasedResourceAccessFields
  ): DbIdentities | null {
    switch (field) {
      case 'db_labels':
        const newDbIdentities: DbIdentities = {
          db_names: roleConditions['db_names'] ?? [wildcard],
          db_users: roleConditions['db_users'] ?? [wildcard],
        };
        return newDbIdentities;
      case 'app_labels':
      case 'kubernetes_labels':
      case 'node_labels':
      case 'windows_desktop_labels':
        // There is no default identities for these kinds yet.
        return null;
      default:
        field satisfies never;
    }
  }

  function updateLabels(
    field: LabelBasedResourceAccessFields,
    labels: Labels
  ): StandardRoleConditions {
    let updatedCondition = { ...roleConditions };
    updatedCondition[field] = labels;

    // Since empty labels is like removing access to a resource,
    // any previously set related identities are also removed as well.
    if (!labels || !Object.keys(labels).length) {
      const emptyIdentities = getEmptyIdentities(field);
      updatedCondition = { ...updatedCondition, ...emptyIdentities };
    } else {
      // Set default values for identities.
      const identities = getDefaultIdentities(field);
      if (identities) {
        updatedCondition = { ...updatedCondition, ...identities };
      }
    }

    updateRoleConditions(updatedCondition);
    return updatedCondition;
  }

  function updateGitHubPermissions(orgs: string[]) {
    updateRoleConditions({
      ...roleConditions,
      github_permissions: orgs.length ? [{ orgs }] : [],
    });
  }

  function definedAccess(field: ListResourceAccessFields) {
    switch (field) {
      case 'github_permissions':
        return roleConditions.github_permissions.length > 0;

      case 'app_labels':
      case 'db_labels':
      case 'kubernetes_labels':
      case 'node_labels':
      case 'windows_desktop_labels':
        const labels = roleConditions[field];
        return Object.keys(labels).length > 0;

      default:
        field satisfies never;
    }
  }

  function hasAnyAccessDefined() {
    return listResourceAccessFields.some(resourceField =>
      definedAccess(resourceField)
    );
  }

  function canSkipDefiningIdentities() {
    for (let i = 0; i < labelBasedResourceAccessFields.length; i++) {
      const field = labelBasedResourceAccessFields[i];
      if (definedAccess(field)) {
        switch (field) {
          // Not all app kinds will require app identities.
          case 'app_labels':
            if (hasRequiredAppIdentities()) {
              return false;
            }
            continue;

          case 'db_labels':
          case 'kubernetes_labels':
          case 'node_labels':
          case 'windows_desktop_labels':
            return false;

          default:
            field satisfies never;
        }
      }
    }

    // There was no access defined.
    return true;
  }

  return {
    roleEditState,
    initRoleEditState,
    getRoleToSave,

    roleConditions,
    setRoleConditions,
    requiredAppIdentities,
    reset,
    definedAccess,
    hasAnyAccessDefined,

    hasRequiredAppIdentities,
    clearRequiredAppIdentities,
    markRequiredAppIdentities,
    markAppIdentityFieldsAsRequired,

    canSkipDefiningIdentities,

    clearIdentities,

    updateLabels,

    updateGitHubPermissions,

    updateIdentity,
  };
}
