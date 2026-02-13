import {
  QueryFunctionContext,
  useQuery,
  UseQueryResult,
} from '@tanstack/react-query';
import { useState } from 'react';
import { useLocation } from 'react-router';

import { UnifiedResourceApp } from 'shared/components/UnifiedResources';

import { ResumableAwsIcRoleState } from 'e-teleport/AccessListManagement/CreateAccessList/route';
import cfg from 'teleport/config';
import { PermissionSet } from 'teleport/services/apps';
import ResourceService, { Role } from 'teleport/services/resources';

import {
  AwsIcRoleConditions,
  convertAwsAccountMapToRoleType,
  defaultAwsIcRoleConditions,
  extractAwsIcRoleConditionsFromRole,
} from '../../role/conditions';
import {
  awsIcRoleAccessKind,
  newAccessRole,
  RoleEditState,
  wildcard,
} from '../../role/role';
import { awsIcSubKindPredicate } from '../../role/unifiedResource';

export type AwsIcRoleState = {
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
   * Defines access specifically for AWS IC applications.
   */
  roleConditions: AwsIcRoleConditions;

  /**
   * Resets states.
   */
  reset(): void;

  /**
   * Result of fetching all aws ic applications upfront.
   */
  fetchedApps: UseQueryResult<
    {
      list: UnifiedResourceApp[];
      // lookup an application by its name
      lookup: Map<string, UnifiedResourceApp>;
      // collection of all permission sets seen.
      permissionSet: Map<string, string>;
    },
    Error
  >;

  /**
   * Removes a ARN from a AWS account from the role conditions.
   */
  removeArn(awsAccountId: string, arnId: string): void;
  /**
   * Removes a AWS account from the role conditions.
   */
  removeAccount(awsAccountId: string): void;
  /**
   * Adds new account/arn selections to the role conditions.
   */
  updateAccount(
    selectedAwsIcApps: UnifiedResourceApp[],
    selectedSharedArns: PermissionSet[]
  ): void;
  /**
   * Adding wildcard for AWS account removes all previously selected
   * AWS accounts with wildcard and sets new arns.
   */
  addAccountWildcard(selectedSharedArns: PermissionSet[]): void;
  /**
   * Returns true if there were any accounts defined.
   */
  definedAccess(): boolean;
};

/**
 * Defines states and functions related to defining access for
 * AWS IC applications.
 */
export function useAwsIcRoleState(): AwsIcRoleState {
  const loc = useLocation<ResumableAwsIcRoleState>();

  const [roleEditState, setRoleEditState] = useState<RoleEditState | null>();

  const [roleConditions, setRoleConditions] = useState(
    () => loc.state?.awsIcRoleConditions ?? defaultAwsIcRoleConditions()
  );

  const fetchedApps = useQuery({
    queryKey: ['unifiedResources', 'listAllAwsIcApps'],
    queryFn: fetchAllAwsIcApps,
  });

  function reset() {
    setRoleConditions(defaultAwsIcRoleConditions());
    setRoleEditState(null);
  }

  function initRoleEditState(role?: Role) {
    setRoleEditState({
      original: role,
      isDirty: false,
    });
    if (role) {
      setRoleConditions(extractAwsIcRoleConditionsFromRole(role));
    } else {
      setRoleConditions(defaultAwsIcRoleConditions());
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
      if (!definedAccess()) {
        return undefined;
      }

      // Send modification.
      return {
        ...roleEditState.original,
        spec: {
          ...roleEditState.original.spec,
          allow: {
            app_labels: roleConditions.labels,
            account_assignments: convertAwsAccountMapToRoleType(
              roleConditions.account
            ),
          },
        },
      };
    }

    // Creating a new role b/c:
    // - user removed access but is adding it back
    // - wasn't defined in the first place (users can skip defining access)
    if (roleEditState.isDirty && !roleEditState.original) {
      return newAccessRole({
        kind: awsIcRoleAccessKind,
        roleConditions,
      });
    }
  }

  function updateRoleConditions(newConditions: AwsIcRoleConditions) {
    setRoleConditions(newConditions);
    if (roleEditState) {
      setRoleEditState({ ...roleEditState, isDirty: true });
    }
  }

  function removeArn(awsAccount: string, arn: string) {
    const newAccountMap = new Map(roleConditions.account);

    // Remove the target arn.
    const arns = newAccountMap.get(awsAccount);
    arns.delete(arn);

    // If deleting the arn results in no arns, remove account.
    if (arns.size == 0) {
      newAccountMap.delete(awsAccount);
    }

    updateRoleConditions({
      ...roleConditions,
      account: newAccountMap,
    });
  }

  function removeAccount(awsAccount: string) {
    const newAccountMap = new Map(roleConditions.account);
    newAccountMap.delete(awsAccount);
    updateRoleConditions({
      ...roleConditions,
      account: newAccountMap,
    });
  }

  function updateAccount(
    selectedAwsAccounts: UnifiedResourceApp[],
    selectedSharedPermSets: PermissionSet[]
  ) {
    const newAccountMap = new Map();
    const seenAccountInNewMap: Record<string, boolean> = {};

    selectedAwsAccounts.forEach(app => {
      const arns = new Set(roleConditions.account.get(app.name));
      selectedSharedPermSets.forEach(perm => {
        arns.add(perm.arn);
      });

      newAccountMap.set(app.name, arns);
      seenAccountInNewMap[app.name] = true;
    });

    // To render new account insertion at the top of the list, re-insert
    // untouched accounts at the end of map (JS map keeps insertion order).
    roleConditions.account.forEach(function (arns, account) {
      if (!seenAccountInNewMap[account]) {
        newAccountMap.set(account, arns);
      }
    });

    updateRoleConditions({
      ...roleConditions,
      account: newAccountMap,
    });
  }

  function addAccountWildcard(selectedSharedPermSets: PermissionSet[]) {
    const newArns = new Set();
    selectedSharedPermSets.forEach(perm => {
      newArns.add(perm.arn);
    });

    // Replace existing map with only wildcard.
    const newAccountMap = new Map();
    newAccountMap.set(wildcard, newArns);
    updateRoleConditions({ ...roleConditions, account: newAccountMap });
  }

  function definedAccess() {
    return roleConditions.account.size > 0;
  }

  return {
    roleEditState,
    initRoleEditState,
    getRoleToSave,

    roleConditions,
    fetchedApps,
    removeArn,
    removeAccount,
    updateAccount,
    addAccountWildcard,
    reset,
    definedAccess,
  };
}

const collator = new Intl.Collator(undefined, {
  numeric: true,
  sensitivity: 'base',
});

/**
 * Fetches all AWS IC pages at once.
 * This is to support wildcard option where calculating shared permission sets
 * between all apps requires going through every app available. Also aids in
 * setting friendly names of app name and arn name.
 *
 * TODO(kimlisa): This is not scalable, future iteration could work
 * on moving both calculations to the back.
 */
const fetchAllAwsIcApps = async ({ signal }: QueryFunctionContext) => {
  let apps: UnifiedResourceApp[] = [];
  let nextKey = '';

  const resourceService = new ResourceService();

  while (nextKey !== undefined) {
    const data = await resourceService.fetchUnifiedResources(
      cfg.proxyCluster,
      {
        query: awsIcSubKindPredicate,
        kinds: ['app'],
        limit: 200,
        startKey: nextKey,
      },
      signal
    );

    // We know "data.agents" are all apps b/c that's how we requested it.
    apps.push(...(data.agents as UnifiedResourceApp[]));
    nextKey = data.startKey || undefined;
  }

  // Sorting is done manually because backend does not support sorting
  // by friendly names.
  const sortedApps = apps.sort((appA, appB) =>
    collator.compare(appA.friendlyName, appB.friendlyName)
  );

  const lookup: Map<string, UnifiedResourceApp> = new Map();
  const permissionSet: Map<string, string> = new Map();
  sortedApps.forEach(app => {
    lookup.set(app.name, app);
    app.permissionSets?.forEach(ps => {
      permissionSet.set(ps.arn, ps.name);
    });
  });

  return {
    list: sortedApps,
    lookup,
    permissionSet,
  };
};
