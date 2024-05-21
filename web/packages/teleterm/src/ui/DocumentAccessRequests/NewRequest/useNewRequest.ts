/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { useState, useEffect, useCallback } from 'react';

import { FetchStatus, SortType } from 'design/DataTable/types';

import useAttempt from 'shared/hooks/useAttemptNext';
import { makeAdvancedSearchQueryForLabel } from 'shared/utils/advancedSearchLabelQuery';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import {
  makeDatabase,
  makeServer,
  makeKube,
} from 'teleterm/ui/services/clusters';
import { retryWithRelogin } from 'teleterm/ui/utils';

import { useWorkspaceContext } from 'teleterm/ui/Documents';
import {
  GetResourcesParams,
  App as tshdApp,
} from 'teleterm/services/tshd/types';
import { routing } from 'teleterm/ui/uri';
import { useWorkspaceLoggedInUser } from 'teleterm/ui/hooks/useLoggedInUser';
import { getAppAddrWithProtocol } from 'teleterm/services/tshd/app';

import type {
  ResourceLabel,
  ResourceFilter as WeakAgentFilter,
  ResourcesResponse,
  ResourceIdKind,
  UnifiedResource,
} from 'teleport/services/agents';
import type * as teleportApps from 'teleport/services/apps';

const pageSize = 10;

type AgentFilter = WeakAgentFilter & { sort: SortType };

export default function useNewRequest() {
  const ctx = useAppContext();
  const { accessRequestsService, localClusterUri: clusterUri } =
    useWorkspaceContext();

  const loggedInUser = useWorkspaceLoggedInUser();

  const isLeafCluster = routing.isLeafCluster(clusterUri);

  const { attempt, setAttempt } = useAttempt('processing');
  const [fetchStatus, setFetchStatus] = useState<FetchStatus>('');
  const [fetchedData, setFetchedData] = useState<
    ResourcesResponse<UnifiedResource>
  >(getEmptyFetchedDataState());
  const requestableRoles = loggedInUser?.requestableRoles || [];
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
      case 'app': {
        const tshdApp: tshdApp = source;
        const app: Pick<
          teleportApps.App,
          'name' | 'labels' | 'description' | 'userGroups' | 'addrWithProtocol'
        > = {
          name: tshdApp.name,
          labels: tshdApp.labels,
          addrWithProtocol: getAppAddrWithProtocol(source),
          description: tshdApp.desc,
          //TODO(gzdunek): Enable requesting apps via user groups in Connect.
          // To make this work, we need
          // to fetch user groups while fetching the apps
          // and then return them for appropriate resources.
          // See how it was done in web/apps.go
          //
          // Additionally, to make this feature complete,
          // I think we should also add a tab for requesting the user groups.
          // For that, we would have to add a new RPC that lists them.
          //
          // https://github.com/gravitational/teleport.e/issues/3162
          userGroups: [],
        };
        return app;
      }
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

  function getFetchCallback(params: GetResourcesParams) {
    switch (selectedResource) {
      case 'node':
        return retry(() => ctx.resourcesService.fetchServers(params));
      case 'db':
        return retry(() => ctx.resourcesService.fetchDatabases(params));
      case 'kube_cluster':
        return retry(() => ctx.resourcesService.fetchKubes(params));
      case 'app':
        return retry(() => ctx.resourcesService.fetchApps(params));
      default: {
        throw new Error(`Fetch not implemented for: ${selectedResource}`);
      }
    }
  }

  const fetch = useCallback(async () => {
    if (selectedResource !== 'role') {
      try {
        setFetchStatus('loading');
        const data = await getFetchCallback({
          clusterUri,
          ...agentFilter,
          limit: pageSize,
          searchAsRoles: 'yes',
        });
        setFetchedData({
          agents: data.agents.map(makeAgent),
          startKey: data.startKey,
          totalCount: data.totalCount,
        });
        setPage({
          keys: ['', data.startKey],
          index: 0,
        });
        setAttempt({ status: 'success' });
        setFetchStatus('');
      } catch (err) {
        setAttempt({ status: 'failed', statusText: err.message });
        setFetchStatus('');
      }
    }
  }, [agentFilter, clusterUri, selectedResource]);

  useEffect(() => {
    fetch();
  }, [fetch]);

  function onAgentLabelClick(label: ResourceLabel) {
    const query = makeAdvancedSearchQueryForLabel(label, agentFilter);
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
        agents: data.agents.map(makeAgent),
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
        agents: data.agents.map(makeAgent),
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

export type ResourceKind = ResourceIdKind | 'role';

export type State = ReturnType<typeof useNewRequest>;
