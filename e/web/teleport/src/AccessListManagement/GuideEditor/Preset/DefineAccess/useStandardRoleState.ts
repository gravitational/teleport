import { useState } from 'react';

import { AppSubKind } from 'shared/services';

import { App, CloudInstance } from 'teleport/services/apps';
import { Labels } from 'teleport/services/resources';

import {
  defaultStandardRoleConditions,
  StandardRoleConditions,
} from '../role/conditions';
import {
  LabelBasedResourceAccessFields,
  ListResourceAccessFields,
} from '../role/listaccess';
import {
  appIdentityFieldNames,
  emptyAppIdentities,
  emptyRequiredAppIdentitiesWithFetchResult,
  RequiredAppIdentitiesWithFetchResult,
} from '../role/resources/app';
import { emptyDbIdentities } from '../role/resources/db';
import { emptyDesktopIdentities } from '../role/resources/desktop';
import { emptyKubeIdentities } from '../role/resources/kube';
import { emptyServerIdentities } from '../role/resources/server';

export type StandardRoleState = {
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
   * Scans fetched apps to determine which identity fields are required.
   * Sets the identity field to an empty array (required) when a matching
   * app type is found. The allPagesFetched flag indicates whether all
   * apps in the cluster matching `app_labels` have been fetched.
   */
  markRequiredAppIdentities(fetchedApps: App[], allPagesFetched: boolean): void;

  /**
   * Clears identity fields (e.g., db_users, kubernetes_groups) associated
   * with the given resource label field. Returns the updated role conditions.
   */
  clearIdentities(
    field: LabelBasedResourceAccessFields
  ): StandardRoleConditions;

  /**
   * Updates labels for specified resource field in the role condition.
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
};

/**
 * Manages role condition state for standard resources.
 */
export function useStandardRoleState(): StandardRoleState {
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

  function clearIdentities(field: LabelBasedResourceAccessFields) {
    let emptyIdentities;
    switch (field) {
      case 'app_labels':
        emptyIdentities = emptyAppIdentities();
        break;
      case 'db_labels':
        emptyIdentities = emptyDbIdentities();
        break;
      case 'kubernetes_labels':
        emptyIdentities = emptyKubeIdentities();
        break;
      case 'node_labels':
        emptyIdentities = emptyServerIdentities();
        break;
      case 'windows_desktop_labels':
        emptyIdentities = emptyDesktopIdentities();
        break;
      default:
        field satisfies never;
    }

    const updatedConditions = { ...roleConditions, ...emptyIdentities };
    setRoleConditions(updatedConditions);

    return updatedConditions;
  }

  function updateLabels(
    field: LabelBasedResourceAccessFields,
    labels: Labels
  ): StandardRoleConditions {
    const updatedCondition = { ...roleConditions };
    updatedCondition[field] = labels;
    setRoleConditions(updatedCondition);
    return updatedCondition;
  }

  function updateGitHubPermissions(orgs: string[]) {
    setRoleConditions({
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

  return {
    roleConditions,
    requiredAppIdentities,
    reset,
    definedAccess,

    clearRequiredAppIdentities,
    markRequiredAppIdentities,

    clearIdentities,

    updateLabels,

    updateGitHubPermissions,
  };
}
