import { useState, useCallback, useEffect } from 'react';
import { FetchStatus, SortType, Page } from 'design/DataTable/types';
import useAttempt from 'shared/hooks/useAttemptNext';
import { isAbortError } from 'shared/utils/abortError';
import { SharedUnifiedResource } from 'shared/components/UnifiedResources/types';
import { useUnifiedResourcesFetch } from 'shared/components/UnifiedResources';
import { getNumAddedResources } from 'shared/components/AccessRequests/Shared/utils';
import { makeAdvancedSearchQueryForLabel } from 'shared/utils/advancedSearchLabelQuery';
import useStickyClusterId from 'teleport/useStickyClusterId';
import cfg from 'teleport/config';
import {
  PendingListItem,
  ResourceMap,
  getEmptyResourceState,
} from 'shared/components/AccessRequests/NewRequest';
import { RequestableResourceKind } from 'shared/components/AccessRequests/NewRequest/resource';
import { KubeResource } from 'teleport/services/kube';

import Ctx from 'e-teleport/teleportContextE';

import {
  AccessRequestResourceIdParam,
  getResourceIdUri,
  parseResourceIdUri,
} from './kube';

import type {
  ResourceLabel,
  ResourceFilter,
  ResourcesResponse,
  UnifiedResource,
} from 'teleport/services/agents';

const pageSize = 10;

export function useNewRequest(ctx: Ctx) {
  const { clusterId, isLeafCluster } = useStickyClusterId();
  const { attempt, setAttempt, handleError } = useAttempt(
    isLeafCluster ? 'processing' : ''
  );
  const { attempt: dryRunAttempt, setAttempt: setDryRunAttempt } =
    useAttempt('processing');
  const [selectedResource, setSelectedResource] =
    useState<RequestableResourceKind>(isLeafCluster ? 'node' : 'role');
  const {
    attempt: userGroupFetchAttempt,
    setAttempt: setUserGroupFetchAttempt,
  } = useAttempt('success');
  const [appsGrantedByUserGroup, setAppsGrantedByUserGroup] = useState<
    string[]
  >([]);
  const [fetchStatus, setFetchStatus] = useState<FetchStatus>('');
  const [fetchedData, setFetchedData] = useState<
    ResourcesResponse<UnifiedResource>
  >(getEmptyFetchedDataState());
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

  const addAllFetchAttempt = useAttempt('');

  const [page, setPage] = useState<Page>({ keys: [], index: 0 });
  const [agentFilter, setAgentFilter] = useState<ResourceFilter>({
    searchAsRoles: 'yes',
    sort: getDefaultSort(selectedResource),
  });

  const [addedResources, setAddedResources] = useState<ResourceMap>(
    getEmptyResourceState()
  );

  const [usage, setUsage] = useState<{
    limit: number;
    used: number;
  } | null>(null);

  function fetchUsage() {
    if (cfg.entitlements.AccessRequests.limit === 0) {
      // there are no limits
      return;
    }

    if (!ctx.storeUser.getBillingAccess().list) {
      return;
    }

    ctx.cloudService
      .fetchNonBillableSummaryInformation()
      .then(info => {
        //  todo (michellescripts) we do have the option to not fetch the limit, as it's present on the entitlement
        setUsage({
          limit: info.accessRequestUsage.monthlyLimit,
          used: info.accessRequestUsage.monthlyUsed,
        });
      })
      .catch(handleError);
  }

  useEffect(fetchUsage, []);

  const {
    fetch: unifiedFetch,
    resources,
    attempt: unifiedFetchAttempt,
    clear,
  } = useUnifiedResourcesFetch({
    fetchFunc: useCallback(
      async (paginationParams, signal) => {
        try {
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
        } catch (err) {
          // unified resources are not implemented on the cluster. We ignore
          // the error because the view is going to change anyway. Throw everything else
          if (
            (err?.response?.status === 404 &&
              err?.message.includes('unknown method ListUnifiedResources')) ||
            err?.response?.status === 501
          ) {
            return {
              startKey: '',
              agents: [],
              totalCount: 0,
            };
          }
          throw err;
        }
      },
      [clusterId, agentFilter, ctx.resourceService]
    ),
  });

  useEffect(() => {
    clear();
    // No need to fetch anything for roles, it
    // already comes in a list from user context fetch.
    // Also, we skip fetching resources for "resource" (unified resources)
    // because we fetch it separately using the infinite scroll
    if (selectedResource === 'role') return;
    if (selectedResource !== 'resource') {
      fetch();
    }
  }, [selectedResource, clear, clusterId, agentFilter]);

  useEffect(() => {
    // We cannot mix root and leaf cluster resources.
    // So we reset all states.
    setFetchedData(getEmptyFetchedDataState());
    clearAddedResources();
    setAgentFilter({
      searchAsRoles: 'yes',
      sort: getDefaultSort(selectedResource),
    });
  }, [clusterId]);

  // when the selected user_group changes, we need to fetch the
  // list of applications that the app grants access to to display
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

  // TODO (lisa): this is pretty hacky, maybe expose the ref for selector,
  // but that might require touching multiple files adding to an already bloated PR.
  useEffect(() => {
    const clusterSelectorEl = document.querySelector(
      '.teleport-cluster-selector'
    );

    if (!clusterSelectorEl) return;

    if (selectedResource === 'role') {
      // Mute cluster selector. Role based access requests can only
      // be made from root cluster.
      clusterSelectorEl.classList.add('mute');
    } else {
      // Unmute cluster selector since
      // search based access requests can be made from root and leaf clusters
      clusterSelectorEl.classList.remove('mute');
    }

    // Unset any global styling unmount.
    return () => {
      if (clusterSelectorEl) {
        clusterSelectorEl.classList.remove('mute');
      }
    };
  }, [selectedResource]);

  function updateSort(sort: SortType) {
    setAgentFilter({ ...agentFilter, sort });
  }

  function clearAddedResources() {
    setAddedResources(getEmptyResourceState());
  }

  const updateResourceKind = useCallback(
    (kind: RequestableResourceKind) => {
      setSelectedResource(kind);
      setAgentFilter({
        searchAsRoles: 'yes',
        sort: getDefaultSort(kind),
        search: '',
        query: '',
      });

      // because the role table is client side, we don't need to render
      // a loading indicator
      if (kind === 'role') {
        setAttempt({ status: 'success' });
      } else {
        // We set the attempt here to prevent a brief re-rendering of
        // the user_group table with state data before useEffect kicks in.
        // The unified resources table fetches on it's own with infinite scroll.
        setAttempt({ status: 'processing' });
      }
    },
    [setAttempt]
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

  function getAgentsFetchCallback(
    ctx: Ctx,
    resourceType: RequestableResourceKind
  ) {
    if (resourceType === 'app') {
      return ctx.appService.fetchApps;
    }

    if (resourceType === 'db') {
      return ctx.databaseService.fetchDatabases;
    }

    if (resourceType === 'node') {
      return ctx.nodeService.fetchNodes;
    }

    if (resourceType === 'kube_cluster') {
      return ctx.kubeService.fetchKubernetes;
    }

    if (resourceType === 'windows_desktop') {
      return ctx.desktopService.fetchDesktops;
    }

    if (resourceType === 'user_group') {
      return ctx.userGroupService.fetchUserGroups;
    }
  }

  function fetch() {
    const cb = getAgentsFetchCallback(ctx, selectedResource);
    setFetchStatus('loading');
    setAttempt({ status: 'processing' });

    cb(clusterId, {
      ...agentFilter,
      limit: pageSize,
      searchAsRoles: 'yes',
    })
      .then(res => {
        setFetchedData({
          ...fetchedData,
          agents: res.agents,
          startKey: res.startKey,
          totalCount: res.totalCount,
        });
        setPage({
          keys: ['', res.startKey],
          index: 0,
        });
        setAttempt({ status: 'success' });
        setFetchStatus('');
      })
      .catch((err: Error) => {
        setAttempt({ status: 'failed', statusText: err.message });
        setFetchedData(getEmptyFetchedDataState());
        setFetchStatus('');
      });
  }

  const fetchNext = () => {
    const cb = getAgentsFetchCallback(ctx, selectedResource);
    setFetchStatus('loading');

    cb(clusterId, {
      ...agentFilter,
      limit: pageSize,
      startKey: page.keys[page.index + 1],
      searchAsRoles: 'yes',
    })
      .then(res => {
        setFetchedData({
          ...fetchedData,
          agents: res.agents,
          startKey: res.startKey,
        });
        setPage({
          keys: [...page.keys, res.startKey],
          index: page.index + 1,
        });
        setFetchStatus('');
      })
      .catch((err: Error) => {
        setAttempt({ status: 'failed', statusText: err.message });
        setFetchStatus('');
      });
  };

  const fetchPrev = () => {
    const cb = getAgentsFetchCallback(ctx, selectedResource);
    setFetchStatus('loading');

    cb(clusterId, {
      ...agentFilter,
      limit: pageSize,
      startKey: page.keys[page.index - 1],
      searchAsRoles: 'yes',
    })
      .then(res => {
        setFetchedData({
          ...fetchedData,
          agents: res.agents,
          startKey: res.startKey,
        });
        setPage({
          keys: page.keys.slice(0, -1),
          index: page.index - 1,
        });
        setFetchStatus('');
      })
      .catch((err: Error) => {
        setAttempt({ status: 'failed', statusText: err.message });
        setFetchStatus('');
      });
  };

  function onAgentLabelClick(label: ResourceLabel) {
    const query = makeAdvancedSearchQueryForLabel(label, agentFilter);
    setAgentFilter({ ...agentFilter, search: '', query });
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
    isLeafCluster,
    numAddedResources,
    attempt,
    agents: fetchedData.agents,
    agentFilter,
    setAgentFilter,
    clusterId,
    addSelectedResources,
    updateSort,
    appsGrantedByUserGroup,
    userGroupFetchAttempt,
    fetchStatus,
    onAgentLabelClick,
    selectedResource,
    resources,
    unifiedFetch,
    unifiedFetchAttempt,
    updateResourceKind,
    dryRunAttempt,
    addedResources,
    addOrRemoveResource,
    customSort: {
      dir: agentFilter.sort?.dir,
      fieldName: agentFilter.sort?.fieldName,
      onSort: updateSort,
    },
    nextPage: page.keys[page.index + 1] ? fetchNext : null,
    prevPage: page.index > 0 ? fetchPrev : null,
    clearAddedResources,
    setAddedResources,
    requestableRoles,
    resourceRequestsDisabled,
    addAllFetchAttempt: addAllFetchAttempt.attempt,
    fetchUsage,
    usage,
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

function getEmptyFetchedDataState() {
  return {
    agents: [],
    startKey: '',
    totalCount: 0,
  };
}

function getDefaultSort(kind: RequestableResourceKind): SortType {
  if (kind === 'node') {
    return { fieldName: 'hostname', dir: 'ASC' };
  }
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
