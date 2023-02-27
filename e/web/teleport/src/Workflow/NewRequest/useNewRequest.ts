import { useState, useEffect } from 'react';
import { FetchStatus, SortType } from 'design/DataTable/types';
import useAttempt from 'shared/hooks/useAttemptNext';
import useStickyClusterId from 'teleport/useStickyClusterId';

import { App } from 'teleport/services/apps';
import { WindowsDesktopService } from 'teleport/services/desktops';
import { Kube } from 'teleport/services/kube';
import { Database } from 'teleport/services/databases';
import { Node } from 'teleport/services/nodes';

import Ctx from 'e-teleport/teleportContextE';

import type {
  AgentLabel,
  AgentFilter,
  AgentResponse,
  AgentKind,
  AgentIdKind,
} from 'teleport/services/agents';

const pageSize = 10;

export function useNewRequest(ctx: Ctx) {
  const { clusterId, isLeafCluster } = useStickyClusterId();
  const { attempt, setAttempt } = useAttempt(isLeafCluster ? 'processing' : '');
  const [selectedResource, setSelectedResource] = useState<ResourceKind>(
    isLeafCluster ? 'node' : 'role'
  );
  const [fetchStatus, setFetchStatus] = useState<FetchStatus>('');
  const [fetchedData, setFetchedData] = useState<AgentResponse<AgentKind>>(
    getEmptyFetchedDataState()
  );

  const [addedAll, setAddedAll] = useState(getDefaultAddedAll());
  const addAllFetchAttempt = useAttempt('');

  const [page, setPage] = useState<Page>({ keys: [], index: 0 });
  const [agentFilter, setAgentFilter] = useState<AgentFilter>({
    sort: getDefaultSort(selectedResource),
  });

  const [addedResources, setAddedResources] = useState<ResourceMap>(
    getEmptyResourceState()
  );

  const [numAddedOnPage, setNumAddedOnPage] = useState(getNumAddedOnPage());

  useEffect(() => {
    // No need to fetch anything for roles, it
    // already comes in a list from user context fetch.
    if (selectedResource === 'role') return;
    fetch();
  }, [agentFilter, clusterId]);

  useEffect(() => {
    // We cannot mix root and leaf cluster resources.
    // So we reset all states.
    setFetchedData(getEmptyFetchedDataState());
    clearAddedResources();
    setAgentFilter({
      sort: getDefaultSort(selectedResource),
    });
  }, [clusterId]);

  useEffect(() => {
    setNumAddedOnPage(getNumAddedOnPage());
  }, [page, selectedResource, addedResources]);

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

  function updateSearch(search: string) {
    setAgentFilter({ ...agentFilter, query: '', search });
  }

  function updateQuery(query: string) {
    setAgentFilter({ ...agentFilter, search: '', query });
  }

  function clearAddedResources() {
    setAddedResources(getEmptyResourceState());
  }

  function updateResourceKind(kind: ResourceKind) {
    setSelectedResource(kind);

    if (kind !== 'role') {
      // When switching to a new agent kind, reset agent filters.
      setAgentFilter({
        sort: getDefaultSort(kind),
        search: '',
        query: '',
      });

      // useEffect is used to use the latest changes
      // and re-fetch. We set the attempt here
      // to prevent a brief re-rendering of the table
      // with stale data before useEffect kicks in.
      setAttempt({ status: 'processing' });

      return;
    }
  }

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
    if (addedResources[kind][resourceId]) {
      delete addedResources[kind][resourceId];
      updateAddedAll(kind as AgentIdKind, false);
    } else {
      addedResources[kind][resourceId] = resourceName
        ? resourceName
        : resourceId;
    }

    setAddedResources({
      app: { ...addedResources.app },
      db: { ...addedResources.db },
      kube_cluster: { ...addedResources.kube_cluster },
      node: { ...addedResources.node },
      windows_desktop: { ...addedResources.windows_desktop },
      role: { ...addedResources.role },
    });
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

  function onAgentLabelClick(label: AgentLabel) {
    const query = addAgentLabelToQuery(agentFilter, label);
    setAgentFilter({ ...agentFilter, search: '', query });
  }

  function addAgents(agents: AgentKind[]) {
    switch (selectedResource) {
      case 'node':
        (agents as Node[]).forEach(
          node => (addedResources[selectedResource][node.id] = node.addr)
        );
        break;
      case 'app':
        (agents as App[]).forEach(
          app => (addedResources[selectedResource][app.name] = app.id)
        );
        break;
      case 'db':
        (agents as Database[]).forEach(
          db => (addedResources[selectedResource][db.name] = db.hostname)
        );
        break;
      case 'kube_cluster':
        (agents as Kube[]).forEach(
          kube => (addedResources[selectedResource][kube.name] = kube.name)
        );
        break;
      case 'windows_desktop':
        (agents as WindowsDesktopService[]).forEach(
          desktop =>
            (addedResources[selectedResource][desktop.name] = desktop.addr)
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
    });
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
        (fetchedData.agents as WindowsDesktopService[]).forEach(
          desktop => delete addedResources[selectedResource][desktop.name]
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
    });
  }

  function updateAddedAll(agentKind: AgentIdKind, isAddedAll: boolean) {
    addedAll[agentKind] = isAddedAll;
    setAddedAll({
      app: addedAll.app,
      db: addedAll.db,
      kube_cluster: addedAll.kube_cluster,
      node: addedAll.node,
      windows_desktop: addedAll.windows_desktop,
    });
  }

  function toggleAddCurrentPage() {
    if (numAddedOnPage === 0) {
      addAgents(fetchedData.agents);
    } else {
      unAddCurrentPage();
    }
    updateAddedAll(selectedResource as AgentIdKind, false);
  }

  function toggleAddAllPages() {
    if (!addedAll[selectedResource]) {
      const cb = getAgentsFetchCallback(ctx, selectedResource);
      addAllFetchAttempt.setAttempt({ status: 'processing' });
      setFetchStatus('loading');

      cb(clusterId, {
        ...agentFilter,
        limit: fetchedData.totalCount,
        searchAsRoles: 'yes',
      })
        .then(res => {
          addAgents(res.agents);
          addAllFetchAttempt.setAttempt({ status: 'success' });
          setFetchStatus('');
          updateAddedAll(selectedResource as AgentIdKind, true);
        })
        .catch((err: Error) => {
          addAllFetchAttempt.handleError(err);
          setFetchStatus('');
        });
    } else {
      updateAddedAll(selectedResource as AgentIdKind, false);
      addedResources[selectedResource] = {};
      setAddedResources({
        ...addedResources,
        app: { ...addedResources.app },
        db: { ...addedResources.db },
        kube_cluster: { ...addedResources.kube_cluster },
        node: { ...addedResources.node },
        windows_desktop: { ...addedResources.windows_desktop },
      });
    }
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
              (fetchedData.agents[agent] as WindowsDesktopService).name
            ]
          ) {
            count++;
          }
          break;
      }
    }
    return count;
  }

  // Calculate counts for our resource list.
  const requestableRoles = ctx.storeUser.getRequestableRoles();
  let fromPage = 0;
  let toPage = 0;
  let totalCount = 0;
  if (selectedResource !== 'role' && fetchedData.totalCount) {
    fromPage = page.index * pageSize + 1;
    toPage = fromPage + fetchedData.agents.length - 1;
    totalCount = fetchedData.totalCount;
  } else if (selectedResource === 'role' && requestableRoles.length > 0) {
    fromPage = 1;
    toPage = requestableRoles.length;
    totalCount = requestableRoles.length;
  }

  return {
    isLeafCluster,
    attempt,
    agents: fetchedData.agents,
    agentFilter,
    updateSort,
    updateQuery,
    updateSearch,
    fetchStatus,
    onAgentLabelClick,
    selectedResource,
    updateResourceKind,
    addedResources,
    addOrRemoveResource,
    pageCount: {
      to: toPage,
      from: fromPage,
      total: totalCount,
    },
    customSort: {
      dir: agentFilter.sort?.dir,
      fieldName: agentFilter.sort?.fieldName,
      onSort: updateSort,
    },
    nextPage: page.keys[page.index + 1] ? fetchNext : null,
    prevPage: page.index > 0 ? fetchPrev : null,
    clearAddedResources,
    requestableRoles,
    toggleAddCurrentPage,
    toggleAddAllPages,
    numOfPages: Math.ceil(fetchedData.totalCount / pageSize),
    addedAll,
    unAddCurrentPage,
    numAddedOnPage,
    addAllFetchAttempt: addAllFetchAttempt.attempt,
  };
}

export function getEmptyResourceState() {
  return {
    node: {},
    db: {},
    app: {},
    kube_cluster: {},
    windows_desktop: {},
    role: {},
  };
}

function getEmptyFetchedDataState() {
  return {
    agents: [],
    startKey: '',
    totalCount: 0,
  };
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
}

function addAgentLabelToQuery(filter: AgentFilter, label: AgentLabel) {
  const queryParts = [];

  // Add existing query
  if (filter.query) {
    queryParts.push(filter.query);
  }

  // If there is an existing simple search,
  // convert it to predicate language and add it
  if (filter.search) {
    queryParts.push(`search("${filter.search}")`);
  }

  // Create the label query.
  queryParts.push(`labels["${label.name}"] == "${label.value}"`);

  return queryParts.join(' && ');
}

function getDefaultSort(kind: ResourceKind): SortType {
  if (kind === 'node') {
    return { fieldName: 'hostname', dir: 'ASC' };
  }
  return { fieldName: 'name', dir: 'ASC' };
}

function getDefaultAddedAll(): AddedAll {
  return {
    app: false,
    node: false,
    db: false,
    kube_cluster: false,
    windows_desktop: false,
  };
}

// Page keeps track of our current agent list
//  start keys and current position.
type Page = {
  // keys are the list of start keys collected from
  // each page fetch.
  keys: string[];
  // index refers to the current index the page
  // is at in the list of keys.
  index: number;
};

type AddedAll = {
  [K in AgentIdKind]: boolean;
};

// ResourceKind describes resource kind's for both a search based access
// request and "role" based access request.
export type ResourceKind = AgentIdKind | 'role';

export type ResourceMap = {
  [K in ResourceKind]: Record<string, string>;
};

export type State = ReturnType<typeof useNewRequest>;
