import { useState, useCallback, useEffect } from 'react';
import { FetchStatus, SortType, Page } from 'design/DataTable/types';
import useAttempt from 'shared/hooks/useAttemptNext';
import { isAbortError } from 'shared/utils/abortError';
import { SharedUnifiedResource } from 'shared/components/UnifiedResources/types';
import { useUnifiedResourcesFetch } from 'shared/components/UnifiedResources';
import { getNumAddedResources } from 'shared/components/AccessRequests/Shared/utils';
import { makeAdvancedSearchQueryForLabel } from 'shared/utils/advancedSearchLabelQuery';
import useStickyClusterId from 'teleport/useStickyClusterId';

import { App } from 'teleport/services/apps';
import { Desktop } from 'teleport/services/desktops';
import { Kube } from 'teleport/services/kube';
import { Database } from 'teleport/services/databases';
import { Node } from 'teleport/services/nodes';
import { UserGroup } from 'teleport/services/userGroups';

import cfg from 'teleport/config';

import {
  ResourceKind,
  ResourceMap,
  getEmptyResourceState,
} from 'shared/components/AccessRequests/NewRequest';

import Ctx from 'e-teleport/teleportContextE';

import type {
  ResourceLabel,
  ResourceFilter,
  ResourcesResponse,
  UnifiedResource,
} from 'teleport/services/agents';

export type {
  /** @deprecated Import `ResourceKind` directly. */
  ResourceKind,
};

const pageSize = 10;

export function useNewRequest(ctx: Ctx) {
  const { clusterId, isLeafCluster } = useStickyClusterId();
  const { attempt, setAttempt, handleError } = useAttempt(
    isLeafCluster ? 'processing' : ''
  );
  const { attempt: dryRunAttempt, setAttempt: setDryRunAttempt } =
    useAttempt('processing');
  const [selectedResource, setSelectedResource] = useState<ResourceKind>(
    isLeafCluster ? 'node' : 'role'
  );
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

  const [numAddedOnPage, setNumAddedOnPage] = useState(getNumAddedOnPage());

  const [usage, setUsage] = useState<{
    limit: number;
    used: number;
  } | null>(null);

  function fetchUsage() {
    if (cfg.isLegacyEnterprise() || cfg.isIgsEnabled) {
      // there are no limits on non usage-based or if IGS is enabled.
      return;
    }

    if (!ctx.storeUser.getBillingAccess().list) {
      return;
    }

    ctx.cloudService
      .fetchNonBillableSummaryInformation()
      .then(info => {
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

  useEffect(() => {
    setNumAddedOnPage(getNumAddedOnPage());
  }, [page, selectedResource, addedResources]);

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
    (kind: ResourceKind) => {
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

  // addOrRemoveResource adds the resource if it doesn't exist already in the map.
  // Else removes it. "resourceName" is optional, if not provided, it is assumed that
  // "resourceId" is the same as "resourceName" e.g: for resource type "node", we display
  // hostname to the user which isn't the id, but a more readable/identifiable name for
  // the user.
  function addOrRemoveResource(
    kind: ResourceKind,
    resourceId: string,
    resourceName?: string
  ) {
    // if this is adding a resource for the first time, we run a dry run again to
    // get updated dryrun return and clear the old attempt
    const newResources: ResourceMap = deepCopyResourceMap(addedResources);
    if (newResources[kind][resourceId]) {
      delete newResources[kind][resourceId];
    } else {
      newResources[kind][resourceId] = resourceName ? resourceName : resourceId;
    }

    setAddedResources(newResources);
  }

  function getAgentsFetchCallback(ctx: Ctx, resourceType: ResourceKind) {
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

  function unAddCurrentPage() {
    switch (selectedResource) {
      case 'node':
        (fetchedData.agents as Node[]).forEach(
          node => delete addedResources[selectedResource][node.id]
        );
        break;
      case 'app':
        (fetchedData.agents as App[]).forEach(
          app => delete addedResources[selectedResource][app.name]
        );
        break;
      case 'db':
        (fetchedData.agents as Database[]).forEach(
          db => delete addedResources[selectedResource][db.name]
        );
        break;
      case 'kube_cluster':
        (fetchedData.agents as Kube[]).forEach(
          kube => delete addedResources[selectedResource][kube.name]
        );
        break;
      case 'windows_desktop':
        (fetchedData.agents as Desktop[]).forEach(
          desktop => delete addedResources[selectedResource][desktop.name]
        );
        break;
      case 'user_group':
        (fetchedData.agents as UserGroup[]).forEach(
          userGroup => delete addedResources[selectedResource][userGroup.name]
        );
        break;
    }

    setAddedResources({
      ...addedResources,
      app: { ...addedResources.app },
      db: { ...addedResources.db },
      kube_cluster: { ...addedResources.kube_cluster },
      node: { ...addedResources.node },
      windows_desktop: { ...addedResources.windows_desktop },
      user_group: { ...addedResources.user_group },
    });
  }

  function getNumAddedOnPage() {
    let count = 0;
    for (const agent in fetchedData.agents) {
      switch (selectedResource) {
        case 'node':
          if (
            addedResources[selectedResource][
              (fetchedData.agents[agent] as Node).id
            ]
          ) {
            count++;
          }
          break;
        case 'app':
          if (
            addedResources[selectedResource][
              (fetchedData.agents[agent] as App).name
            ]
          ) {
            count++;
          }
          break;
        case 'db':
          if (
            addedResources[selectedResource][
              (fetchedData.agents[agent] as Database).name
            ]
          ) {
            count++;
          }
          break;
        case 'kube_cluster':
          if (
            addedResources[selectedResource][
              (fetchedData.agents[agent] as Kube).name
            ]
          ) {
            count++;
          }
          break;
        case 'windows_desktop':
          if (
            addedResources[selectedResource][
              (fetchedData.agents[agent] as Desktop).name
            ]
          ) {
            count++;
          }
          break;
        case 'user_group':
          if (
            addedResources[selectedResource][
              (fetchedData.agents[agent] as UserGroup).name
            ]
          ) {
            count++;
          }
          break;
      }
    }
    return count;
  }

  const requestableRoles = ctx.storeUser.getRequestableRoles();

  const addSelectedResources = (
    resources: {
      unifiedResourceId: string;
      resource: SharedUnifiedResource['resource'];
    }[]
  ) => {
    const allAdded = resources.every(
      ({ resource }) => addedResources[resource.kind]?.[getResourceId(resource)]
    );

    let newMap = { ...addedResources };
    if (allAdded) {
      resources.forEach(({ resource }) => {
        const key = getResourceId(resource);
        const kind = resource.kind;
        const name = kind === 'node' ? resource.hostname : key;
        delete newMap[kind][name];
      });
      setAddedResources(newMap);
      return;
    }

    resources.forEach(({ resource }) => {
      const key = getResourceId(resource);
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
    unAddCurrentPage,
    numAddedOnPage,
    addAllFetchAttempt: addAllFetchAttempt.attempt,
    fetchUsage,
    usage,
    ctx,
  };
}

function getEmptyFetchedDataState() {
  return {
    agents: [],
    startKey: '',
    totalCount: 0,
  };
}

function getDefaultSort(kind: ResourceKind): SortType {
  if (kind === 'node') {
    return { fieldName: 'hostname', dir: 'ASC' };
  }
  return { fieldName: 'name', dir: 'ASC' };
}

export type State = ReturnType<typeof useNewRequest>;

export function getResourceId(resource: SharedUnifiedResource['resource']) {
  if (resource.kind === 'node') {
    return resource.id;
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
  };
}
