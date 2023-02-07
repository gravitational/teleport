/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { useState, useEffect, useMemo } from 'react';
import { SortType } from 'design/DataTable/types';
import { useAsync } from 'shared/hooks/useAsync';
import { AgentFilter, AgentLabel } from 'teleport/services/agents';

import { ServerSideParams } from 'teleterm/services/tshd/types';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import { retryWithRelogin } from 'teleterm/ui/utils';

import { useClusterContext } from '../clusterContext';

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

const limit = 15;

export function useServerSideResources<Agent>(
  defaultSort: SortType,
  fetchFunction: (params: ServerSideParams) => Promise<FetchResponse<Agent>>
) {
  const ctx = useAppContext();
  const { clusterUri } = useClusterContext();
  const [pageIndex, setPageIndex] = useState(0);
  const [keys, setKeys] = useState<string[]>([]);
  const [agentFilter, setAgentFilter] = useState<AgentFilter>({
    sort: defaultSort,
  });

  // startKey is used here as a way to paginate through agents returned from
  // their respective rpcs.
  const [fetchAttempt, fetch] = useAsync(
    (startKey: string, filter: AgentFilter) =>
      retryWithRelogin(ctx, clusterUri, () =>
        fetchFunction({
          ...filter,
          limit,
          clusterUri,
          startKey,
        })
      )
  );

  // If there is no startKey at the current page's index, there is no more data to get
  function nextPage() {
    const proposedKey = keys[pageIndex];
    if (proposedKey) {
      return () => {
        setPageIndex(pageIndex + 1);
      };
    }
    return null;
  }

  // If we are at the first page (index 0), we cannot fetch more previous data
  function prevPage() {
    const newPageIndex = pageIndex - 1;
    if (newPageIndex > -1) {
      return () => {
        setPageIndex(newPageIndex);
      };
    }
    return null;
  }

  useEffect(() => {
    const fetchAndUpdateKeys = async () => {
      const [response, err] = await fetch(keys[pageIndex - 1], agentFilter);
      // The error will be handled via the fetchAttempt outside.
      // Return early here as there are no keys to update.
      if (err) {
        return;
      }

      // when we receive data from fetch, we store the startKey (or lack of) according to the current
      // page index. think of this as "this page's nextKey".
      // "why don't we just name it nextKey then?"
      // Mostly because it's called startKey almost everywhere else in the UI, and also because we'd have the same issue
      // for prevPage if we swapped named, and this comment would be explaining "this page's startKey".
      const newKeys = [...keys];
      newKeys[pageIndex] = response.startKey;
      setKeys(newKeys);
    };
    fetchAndUpdateKeys();
  }, [agentFilter, pageIndex]);

  function updateAgentFilter(filter: AgentFilter) {
    setPageIndex(0);
    setAgentFilter(filter);
  }

  function updateSort(sort: SortType) {
    updateAgentFilter({ ...agentFilter, sort });
  }

  function updateSearch(search: string) {
    updateAgentFilter({ ...agentFilter, query: '', search });
  }

  function updateQuery(query: string) {
    updateAgentFilter({ ...agentFilter, search: '', query });
  }

  function onAgentLabelClick(label: AgentLabel) {
    const query = addAgentLabelToQuery(agentFilter, label);
    updateAgentFilter({ ...agentFilter, search: '', query });
  }

  const pageCount = useMemo(() => {
    const emptyPageCount = {
      from: 0,
      to: 0,
      total: 0,
    };
    if (!(fetchAttempt.data && fetchAttempt.data.totalCount)) {
      return emptyPageCount;
    }
    const startKeyIndex = keys.indexOf(fetchAttempt.data.startKey);
    if (startKeyIndex < 0) {
      return emptyPageCount;
    }
    const from = startKeyIndex * limit + 1;
    return {
      from,
      to: from + fetchAttempt.data.agentsList.length - 1,
      total: fetchAttempt.data.totalCount,
    };
  }, [fetchAttempt, keys]);

  const customSort = {
    dir: agentFilter.sort?.dir,
    fieldName: agentFilter.sort?.fieldName,
    onSort: updateSort,
  };

  return {
    fetchAttempt,
    fetch,
    updateSearch,
    updateSort,
    updateQuery,
    agentFilter,
    prevPage: prevPage(),
    nextPage: nextPage(),
    onAgentLabelClick,
    customSort,
    pageCount,
  };
}

type FetchResponse<T> = {
  agentsList: Array<T>;
  totalCount: number;
  startKey: string;
};
