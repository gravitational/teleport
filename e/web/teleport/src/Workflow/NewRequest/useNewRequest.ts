import { useState, useCallback, useEffect } from 'react';
import { SortType } from 'design/DataTable/types';
import useAttempt from 'shared/hooks/useAttemptNext';
import { isAbortError } from 'shared/utils/abortError';
import { SharedUnifiedResource } from 'shared/components/UnifiedResources/types';
import { useUnifiedResourcesFetch } from 'shared/components/UnifiedResources';
import { getNumAddedResources } from 'shared/components/AccessRequests/Shared/utils';
import useStickyClusterId from 'teleport/useStickyClusterId';
import cfg from 'teleport/config';
import {
  PendingListItem,
  ResourceMap,
  getEmptyResourceState,
} from 'shared/components/AccessRequests/NewRequest';
import { RequestableResourceKind } from 'shared/components/AccessRequests/NewRequest/resource';
import { KubeResource } from 'teleport/services/kube';
import { useAsync } from 'shared/hooks/useAsync';

import Ctx from 'e-teleport/teleportContextE';

import {
  AccessRequestResourceIdParam,
  getResourceIdUri,
  parseResourceIdUri,
} from './kube';

import type { ResourceFilter } from 'teleport/services/agents';

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
   * addOrRemoveResource adds the resource if it doesn't exist already in the map.
   * Else removes it.
   *
   * "resourceName" is optional for most kinds, if not provided,
   * it is assumed that "resourceId" is the same as "resourceName".
   */
  function addOrRemoveResource(
    kind: RequestableResourceKind,
    resourceId: string,
    /**
     * resourceName can refer to:
     *  - node's "hostname": used to refer to a friendlier readable name
     */
    resourceName?: string
  ) {
    const newResources: ResourceMap = deepCopyResourceMap(addedResources);
    const { id, val } = getResourceIdAndVal({
      resourceKind: kind,
      resourceName: resourceId,
      subResourceName: resourceName,
      teleportClusterName: clusterId,
    });
    if (newResources[kind][id]) {
      delete newResources[kind][id];
      // Delete all related namespaces as well.
      if (kind === 'kube_cluster') {
        const kubeNamespaceUris = Object.keys(newResources['namespace']);
        kubeNamespaceUris.forEach(uri => {
          const { resourceName } = parseResourceIdUri(uri).params;
          if (resourceName === id) {
            delete newResources['namespace'][uri];
          }
        });
      }
    } else {
      newResources[kind][id] = val;
    }

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
    addOrRemoveResource,
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
  if (resource.kind === 'node') {
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
  };
}

type ResourceDefinition = SharedUnifiedResource['resource'] | KubeResource;
