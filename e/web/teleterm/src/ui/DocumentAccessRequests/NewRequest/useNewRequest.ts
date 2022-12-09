import { useState, useEffect, useCallback } from 'react';

import { FetchStatus, SortType } from 'design/DataTable/types';

import useAttempt from 'shared/hooks/useAttemptNext';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import {
  makeDatabase,
  makeServer,
  makeKube,
} from 'teleterm/ui/services/clusters';
import { retryWithRelogin } from 'teleterm/ui/utils';

import { useWorkspaceContext } from 'teleterm/ui/Documents';
import { ServerSideParams } from 'teleterm/services/tshd/types';
import { routing } from 'teleterm/ui/uri';

import type {
  AgentLabel,
  AgentFilter,
  AgentResponse,
  AgentKind,
  AgentIdKind,
} from 'teleport/services/agents';

const pageSize = 10;

export default function useNewRequest() {
  const ctx = useAppContext();
  const { accessRequestsService, localClusterUri: clusterUri } =
    useWorkspaceContext();

  const isLeafCluster = routing.isLeafCluster(clusterUri);

  const { attempt, setAttempt } = useAttempt('processing');
  const [fetchStatus, setFetchStatus] = useState<FetchStatus>('');
  const [fetchedData, setFetchedData] = useState<AgentResponse<AgentKind>>(
    getEmptyFetchedDataState()
  );
  const [requestableRoles, setRequestableRoles] = useState<string[]>([]);
  const [selectedResource, setSelectedResource] =
    useState<ResourceKind>('node');
  const [agentFilter, setAgentFilter] = useState<AgentFilter>({
    sort: getDefaultSort(selectedResource),
  });
  const addedResources = accessRequestsService.getPendingAccessRequest();

  const [page, setPage] = useState<Page>({ keys: [], index: 0 });

  const [toResource, setToResource] = useState<string | null>(null);

  const retry = <T>(action: () => Promise<T>) =>
    retryWithRelogin(ctx, clusterUri, action);

  function makeAgent(source) {
    switch (selectedResource) {
      case 'node':
        return makeServer(source);
      case 'db':
        return makeDatabase(source);
      case 'kube_cluster':
        return makeKube(source);
      default:
        return source;
    }
  }

  function updateSort(sort: SortType) {
    setAgentFilter({ ...agentFilter, sort });
  }

  function updateSearch(search: string) {
    setAgentFilter({ ...agentFilter, query: '', search });
  }

  function updateQuery(query: string) {
    setAgentFilter({ ...agentFilter, search: '', query });
  }

  function getFetchCallback(params: ServerSideParams) {
    switch (selectedResource) {
      case 'node':
        return retry(() => ctx.clustersService.client.getServers(params));
      case 'db':
        return retry(() => ctx.clustersService.client.getDatabases(params));
      case 'kube_cluster':
        return retry(() => ctx.clustersService.fetchKubes(params));
      default: {
        throw new Error(`Fetch not implemented for: ${selectedResource}`);
      }
    }
  }

  const fetch = useCallback(async () => {
    try {
      // currently, we need to fetch roles for the current user
      // in the future it'd be nice to have this array of requestable roles
      // on the loggedInUser object
      if (selectedResource === 'role') {
        setFetchStatus('loading');
        const data = await retry(() =>
          ctx.clustersService.getRequestableRoles({
            rootClusterUri: ctx.workspacesService.getRootClusterUri(),
            resourceIds: [],
          })
        );
        setRequestableRoles(data.rolesList);
        setAttempt({ status: 'success' });
        setFetchStatus('');
      } else {
        setFetchStatus('loading');
        const data = await getFetchCallback({
          clusterUri,
          ...agentFilter,
          limit: pageSize,
          searchAsRoles: 'yes',
        });
        setFetchedData({
          agents: data.agentsList.map(makeAgent),
          startKey: data.startKey,
          totalCount: data.totalCount,
        });
        setPage({
          keys: ['', data.startKey],
          index: 0,
        });
        setAttempt({ status: 'success' });
        setFetchStatus('');
      }
    } catch (err) {
      setAttempt({ status: 'failed', statusText: err.message });
      setFetchStatus('');
    }
  }, [agentFilter, clusterUri, selectedResource]);

  useEffect(() => {
    fetch();
  }, [fetch]);

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

  function onAgentLabelClick(label: AgentLabel) {
    const query = addAgentLabelToQuery(agentFilter, label);
    setAgentFilter({ ...agentFilter, search: '', query });
  }

  function updateResourceKind(kind: ResourceKind) {
    setFetchedData(getEmptyFetchedDataState());
    setSelectedResource(kind);
    setAgentFilter({
      sort: getDefaultSort(kind),
      search: '',
      query: '',
    });
  }

  function handleConfirmChangeResource(kind: ResourceKind) {
    accessRequestsService.clearPendingAccessRequest();
    updateResourceKind(kind);
    setToResource(null);
  }

  function addOrRemoveResource(
    kind: ResourceKind,
    resourceId: string,
    resourceName?: string
  ) {
    accessRequestsService.addOrRemoveResource(kind, resourceId, resourceName);
  }

  async function fetchNext() {
    setFetchStatus('loading');
    try {
      const data = await getFetchCallback({
        clusterUri,
        ...agentFilter,
        limit: pageSize,
        searchAsRoles: 'yes',
        startKey: page.keys[page.index + 1],
      });
      setFetchedData({
        ...fetchedData,
        agents: data.agentsList.map(makeAgent),
        startKey: data.startKey,
      });
      setPage({
        keys: [...page.keys, data.startKey],
        index: page.index + 1,
      });
      setAttempt({ status: 'success' });
      setFetchStatus('');
    } catch (err) {
      setAttempt({ status: 'failed', statusText: err.message });
      setFetchStatus('');
    }
  }

  async function fetchPrev() {
    setFetchStatus('loading');
    try {
      const data = await getFetchCallback({
        clusterUri,
        ...agentFilter,
        limit: pageSize,
        searchAsRoles: 'yes',
        startKey: page.keys[page.index - 1],
      });
      setFetchedData({
        ...fetchedData,
        agents: data.agentsList.map(makeAgent),
        startKey: data.startKey,
      });
      setPage({
        keys: page.keys.slice(0, -1),
        index: page.index - 1,
      });
      setAttempt({ status: 'success' });
      setFetchStatus('');
    } catch (err) {
      setFetchStatus('');
      setAttempt({ status: 'failed', statusText: err.message });
    }
  }

  // Calculate counts for our resource list.
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
    agents: fetchedData.agents,
    agentFilter,
    updateSort,
    attempt,
    isLeafCluster,
    fetchStatus,
    updateQuery,
    updateSearch,
    toResource,
    handleConfirmChangeResource,
    setToResource,
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
    requestableRoles,
  };
}

function getEmptyFetchedDataState() {
  return {
    agents: [],
    startKey: '',
    totalCount: 0,
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

function getDefaultSort(kind: ResourceKind): SortType {
  if (kind === 'node') {
    return { fieldName: 'hostname', dir: 'ASC' };
  }
  return { fieldName: 'name', dir: 'ASC' };
}

export type ResourceKind = AgentIdKind | 'role';

export type State = ReturnType<typeof useNewRequest>;
