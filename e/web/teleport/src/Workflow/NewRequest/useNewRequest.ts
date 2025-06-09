import { useCallback, useEffect, useState } from 'react';

import { SortType } from 'design/DataTable/types';
import {
  getEmptyResourceState,
  PendingListItem,
  ResourceMap,
} from 'shared/components/AccessRequests/NewRequest';
import { RequestableResourceKind } from 'shared/components/AccessRequests/NewRequest/resource';
import { getNumAddedResources } from 'shared/components/AccessRequests/Shared/utils';
import { useUnifiedResourcesFetch } from 'shared/components/UnifiedResources';
import { SharedUnifiedResource } from 'shared/components/UnifiedResources/types';
import { useAsync } from 'shared/hooks/useAsync';
import useAttempt from 'shared/hooks/useAttemptNext';
import { AppSubKind } from 'shared/services';
import { isAbortError } from 'shared/utils/abortError';

import Ctx from 'e-teleport/teleportContextE';
import cfg from 'teleport/config';
import type { ResourceFilter } from 'teleport/services/agents';
import { PermissionSet } from 'teleport/services/apps';
import { KubeResource } from 'teleport/services/kube';
import useStickyClusterId from 'teleport/useStickyClusterId';

import {
  AccessRequestResourceIdParam,
  getResourceIdUri,
  parseResourceIdUri,
} from './kube';

export type AccessRequestKind = 'role' | 'resource';

export function useNewRequest(ctx: Ctx) {
  const { clusterId, isLeafCluster } = useStickyClusterId();
  const { attempt: dryRunAttempt, setAttempt: setDryRunAttempt } =
    useAttempt('processing');
  // Role-based access requests are only allowed in root cluster.
  const accessRequestKinds: AccessRequestKind[] = isLeafCluster
    ? ['resource']
    : ['role', 'resource'];
  const [selectedAccessRequestKind, setSelectedAccessRequestKind] =
    useState<AccessRequestKind>('resource');
  const {
    attempt: userGroupFetchAttempt,
    setAttempt: setUserGroupFetchAttempt,
  } = useAttempt('success');
  const [appsGrantedByUserGroup, setAppsGrantedByUserGroup] = useState<
    string[]
  >([]);
  const [resourceRequestsDisabled, setResourceRequestsDisabled] =
    useState(false);

  useEffect(() => {
    setResourceRequestsDisabled(false);
    const signal = new AbortController();

    async function createDryRunAccessRequest() {
      try {
        await ctx.workflowService.createAccessRequest(
          {
            resourceIds: [
              {
                kind: 'node',
                name: '',
                clusterName: clusterId,
              },
            ],
            reason: 'placeholder-reason',
            dryRun: true,
          },
          signal.signal
        );
        // we shouldn't hit this code as a user who can't search_as_roles will return a 403
        // and a user who _can_ search_as_roles will receive a 400 due to an "empty" request.
        // We only set success here to help with tests.
        setDryRunAttempt({ status: 'success' });
      } catch (err) {
        // ignore abort errors
        if (isAbortError(err)) {
          return;
        }

        // This request should _always_ fail due to sending an "empty" request
        // but that is ok because it is a dry run and we expect one of two failures. (403 and 400)
        // The 400 we can ignore because it is expected that an empty request will fail.
        // The 403 is the failure we are interested in as that means the user can't create
        // a resource access request due to not having an additional roles configured in their
        // "search_as_roles" setup.

        // Any subsequent attempts to create a request will be handled further along the path,
        // where errors are already managed. This notification is placed at the "start" of
        // an access request to prevent users from building up their cart only to encounter failure later.
        if (err?.response?.status === 403) {
          setDryRunAttempt({
            status: 'failed',
            statusText:
              'You cannot search for and request additional resources. This could be because you already have access to all resources or your roles do not include the requisite `search_as_roles` field.',
          });
          setResourceRequestsDisabled(true);
          return;
        }
        // Any other failure is acceptable at this point and we want to set this as "success" so the resources
        // list will still show
        setDryRunAttempt({ status: 'success' });
      }
    }

    createDryRunAccessRequest();

    return () => {
      signal.abort();
    };
  }, [clusterId]);

  const [agentFilter, setAgentFilter] = useState<ResourceFilter>({
    searchAsRoles: 'yes',
    sort: getDefaultSort(),
  });

  const [addedResources, setAddedResources] = useState<ResourceMap>(
    getEmptyResourceState()
  );

  const [fetchUsageAttempt, fetchUsage] = useAsync(
    useCallback(async () => {
      if (cfg.entitlements.AccessRequests.limit === 0) {
        // there are no limits
        return;
      }

      if (!ctx.storeUser.getBillingAccess().list) {
        return;
      }

      const { accessRequestUsage } =
        await ctx.cloudService.fetchNonBillableSummaryInformation();
      //  todo (michellescripts) we do have the option to not fetch the limit, as it's present on the entitlement
      return {
        limit: accessRequestUsage.monthlyLimit,
        used: accessRequestUsage.monthlyUsed,
      };
    }, [ctx.cloudService, ctx.storeUser])
  );

  useEffect(() => {
    if (fetchUsageAttempt.status === '') {
      void fetchUsage();
    }
  }, [fetchUsage, fetchUsageAttempt.status]);

  const {
    fetch: unifiedFetch,
    resources,
    attempt: unifiedFetchAttempt,
    clear,
  } = useUnifiedResourcesFetch({
    fetchFunc: useCallback(
      async (paginationParams, signal) => {
        const response = await ctx.resourceService.fetchUnifiedResources(
          clusterId,
          {
            search: agentFilter.search,
            query: agentFilter.query,
            sort: agentFilter.sort,
            kinds: agentFilter.kinds,
            searchAsRoles: 'yes',
            limit: paginationParams.limit,
            startKey: paginationParams.startKey,
          },
          signal
        );

        return {
          startKey: response.startKey,
          agents: response.agents,
          totalCount: response.agents.length,
        };
      },
      [clusterId, agentFilter, ctx.resourceService]
    ),
  });

  useEffect(() => {
    clear();
  }, [selectedAccessRequestKind, clear, clusterId, agentFilter]);

  useEffect(() => {
    // We cannot mix root and leaf cluster resources.
    // So we reset all states.
    // TODO(gzdunek): This is not true. Backend API allows mixing root and leaf
    // cluster resources (and it works in Teleport Connect).
    // To make it work here, we need to store clusterId with each added resource,
    // instead of relying on the "global" one that changes when the user switches the cluster.
    clearAddedResources();
    setAgentFilter({
      searchAsRoles: 'yes',
      sort: getDefaultSort(),
    });
  }, [clusterId]);

  // when the selected user_group changes, we need to fetch the
  // list of applications that the app grants access to display
  // in the checkout process
  useEffect(() => {
    const selectedUserGroup =
      Object.keys(addedResources.user_group).length > 0
        ? Object.keys(addedResources.user_group)[0]
        : null;

    async function fetchUserGroupApps(userGroupId: string) {
      setUserGroupFetchAttempt({ status: 'processing' });
      try {
        const ugs = await ctx.userGroupService.fetchUserGroups(clusterId, {
          limit: 1,
          searchAsRoles: 'yes',
          search: userGroupId,
        });

        if (ugs.agents.length > 0) {
          setAppsGrantedByUserGroup(
            ugs.agents[0].applications.map(app => app.friendlyName)
          );
        }
        setUserGroupFetchAttempt({ status: 'success' });
      } catch (err) {
        setUserGroupFetchAttempt({ status: 'failed', statusText: err.message });
      }
    }

    if (selectedUserGroup) {
      fetchUserGroupApps(selectedUserGroup);
    }
  }, [addedResources.user_group]);

  function clearAddedResources() {
    setAddedResources(getEmptyResourceState());
  }

  const updateAccessRequestKind = useCallback(
    (requestKind: AccessRequestKind) => {
      setSelectedAccessRequestKind(requestKind);
      setAgentFilter({
        searchAsRoles: 'yes',
        sort: getDefaultSort(),
      });
    },
    []
  );

  /**
   * addOrRemoveResources adds or removes a list of resource from
   * resource request cart. It adds item if it does not exist and deletes
   * item if it already exist in a ResourceMap. This behavior can be
   * overridden with a declarative action field.
   * @param items is a list of items to be added or removed
   * @param action defines add or remove action.
   */
  function addOrRemoveResources(
    items: RequestItem[],
    action?: 'add' | 'remove'
  ) {
    const newResources: ResourceMap = deepCopyResourceMap(addedResources);

    items.forEach(item => {
      const { id, val } = getResourceIdAndVal({
        resourceKind: item.kind,
        resourceName: item.resourceId,
        subResourceName: item.resourceName,
        teleportClusterName: clusterId,
      });

      switch (action) {
        case 'add':
          newResources[item.kind][id] = val;
          break;
        case 'remove':
          delete newResources[item.kind][id];
          break;
        default:
          if (newResources[item.kind][id]) {
            delete newResources[item.kind][id];
            // Delete all related namespaces as well.
            if (item.kind === 'kube_cluster') {
              const kubeNamespaceUris = Object.keys(newResources['namespace']);
              kubeNamespaceUris.forEach(uri => {
                const { resourceName } = parseResourceIdUri(uri).params;
                if (resourceName === id) {
                  delete newResources['namespace'][uri];
                }
              });
            }
          } else {
            newResources[item.kind][id] = val;
          }
      }
    });

    setAddedResources(newResources);
  }

  function updateNamespacesForKubeCluster(
    resources: PendingListItem[],
    kubeCluster: PendingListItem
  ) {
    const newResources: ResourceMap = deepCopyResourceMap(addedResources);

    // Validate each namespaces.
    const requestedResources = resources.map(resource => {
      if (resource.kind !== 'namespace') {
        throw new Error(
          `Only kube "namespace" kind can be updated, got kind ${resource.kind}`
        );
      }
      if (resource.id != kubeCluster.name) {
        throw new Error(
          'Only namespace belonging to the same requested kube cluster can be updated'
        );
      }
      return getResourceIdAndVal({
        resourceKind: resource.kind,
        resourceName: resource.id,
        subResourceName: resource.subResourceName,
        teleportClusterName: clusterId,
      });
    });

    const requestedNamespaceIds = requestedResources.map(r => r.id);

    // Delete existing namespace ids.
    Object.keys(newResources['namespace'] || []).forEach(id => {
      if (!requestedNamespaceIds.includes(id)) {
        delete newResources['namespace'][id];
      }
    });

    requestedResources.forEach(resource => {
      newResources['namespace'][resource.id] = resource.val;
    });

    setAddedResources(newResources);
  }

  const requestableRoles = ctx.storeUser.getRequestableRoles();

  /**
   * Used with bulk actions (eg: add/remove all)
   * Does not support bulk adding for:
   *  - apps with user groups
   *  - kubes with namespaces
   */
  const addSelectedResources = (
    resources: { unifiedResourceId: string; resource: ResourceDefinition }[]
  ) => {
    const allAdded = resources.every(
      ({ resource }) =>
        addedResources[resource.kind]?.[getResourceId(resource, clusterId)]
    );

    let newMap = { ...addedResources };
    if (allAdded) {
      resources.forEach(({ resource }) => {
        const key = getResourceId(resource, clusterId);
        const kind = resource.kind;
        delete newMap[kind][key];
      });
      setAddedResources(newMap);
      return;
    }

    resources.forEach(({ resource }) => {
      const key = getResourceId(resource, clusterId);
      const kind = resource.kind;
      const name = kind === 'node' ? resource.hostname : key;
      newMap[kind][key] = name;
    });

    resources.forEach(({ resource }) => {
      if (
        resource.kind === 'app' &&
        resource.subKind === AppSubKind.AwsIcAccount
      ) {
        addOrRemoveIdentityCenterAssignments(
          resource.name,
          resource.permissionSets,
          newMap
        );
        delete newMap['app'][resource.name];
      }
    });

    setAddedResources(newMap);
  };

  const numAddedResources = getNumAddedResources(addedResources);

  return {
    numAddedResources,
    agentFilter,
    setAgentFilter,
    clusterId,
    addSelectedResources,
    appsGrantedByUserGroup,
    userGroupFetchAttempt,
    resources,
    unifiedFetch,
    unifiedFetchAttempt,
    accessRequestKinds,
    selectedAccessRequestKind,
    updateAccessRequestKind,
    dryRunAttempt,
    addedResources,
    addOrRemoveResources,
    clearAddedResources,
    setAddedResources,
    requestableRoles,
    resourceRequestsDisabled,
    fetchUsage,
    fetchUsageAttempt,
    ctx,
    updateNamespacesForKubeCluster,
  };
}

const getResourceIdAndVal = (params: AccessRequestResourceIdParam) => {
  const { resourceKind, resourceName, subResourceName } = params;

  let id = resourceName;
  let val = subResourceName ? subResourceName : resourceName;
  if (resourceKind === 'namespace') {
    id = getResourceIdUri(params);
    val = resourceName;
  }

  return {
    id,
    val,
  };
};

function getDefaultSort(): SortType {
  return { fieldName: 'name', dir: 'ASC' };
}

export type State = ReturnType<typeof useNewRequest>;

export function getResourceId(resource: ResourceDefinition, clusterId: string) {
  if (resource.kind === 'node' || resource.kind === 'git_server') {
    return resource.id;
  }
  if (resource.kind === 'namespace') {
    return getResourceIdUri({
      resourceName: resource.cluster,
      subResourceName: resource.name,
      teleportClusterName: clusterId,
      resourceKind: resource.kind,
    });
  }
  return resource.name;
}

export function deepCopyResourceMap(resources: ResourceMap): ResourceMap {
  return {
    app: { ...resources.app },
    db: { ...resources.db },
    kube_cluster: { ...resources.kube_cluster },
    node: { ...resources.node },
    user_group: { ...resources.user_group },
    windows_desktop: { ...resources.windows_desktop },
    role: { ...resources.role },
    saml_idp_service_provider: { ...resources.saml_idp_service_provider },
    namespace: { ...resources.namespace },
    aws_ic_account_assignment: { ...resources.aws_ic_account_assignment },
    git_server: { ...resources.git_server },
  };
}

type ResourceDefinition = SharedUnifiedResource['resource'] | KubeResource;

/**
 * addOrRemoveIdentityCenterAssignments adds or removes assignments
 * based on permission sets available to an Identity Center account app.
 * @param resourceName is name of the app resource.
 * @param permSets is a list of permission sets.
 * @param newResources is a ResourceMap of access request cart.
 */
export function addOrRemoveIdentityCenterAssignments(
  resourceName: string,
  permSets: PermissionSet[],
  newResources: ResourceMap
) {
  permSets.forEach(ps => {
    if (newResources['aws_ic_account_assignment'][ps.assignmentId]) {
      delete newResources['aws_ic_account_assignment'][ps.assignmentId];
    } else {
      newResources['aws_ic_account_assignment'][ps.assignmentId] =
        `"${ps.name}" on "${resourceName}"`;
    }
  });
}

/**
 * RequestItem defines type for a resource
 * to be added to Resource Access Request.
 */
export type RequestItem = {
  kind: RequestableResourceKind;
  resourceId: string;
  /**
   * resourceName can refer to:
   *  - node's "hostname": used to refer to a friendlier readable name.
   * resourceName is optional for most kinds, if not provided,
   * it is assumed that "resourceId" is the same as "resourceName".
   */
  resourceName?: string;
};

/**
 * requestItems is a helper function to convert a single RequestItem to
 * a list of RequestItem.
 */
export function requestItems(
  kind: RequestableResourceKind,
  resourceId: string,
  resourceName?: string
): RequestItem[] {
  return [{ kind, resourceId, resourceName }];
}
