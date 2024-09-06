/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import React from 'react';
import { MemoryRouter } from 'react-router';

import { Nodes } from './Nodes';
import { State } from './useNodes';
import { nodes } from './fixtures';

export default {
  title: 'Teleport/Nodes',
  excludeStories: ['props'],
};

export const Loaded = () => (
  <MemoryRouter>
    <Nodes {...props} />
  </MemoryRouter>
);

export const Empty = () => (
  <MemoryRouter>
    <Nodes
      {...props}
      fetchedData={{ agents: [], totalCount: 0 }}
      isSearchEmpty={true}
    />
  </MemoryRouter>
);

export const EmptyReadOnly = () => (
  <MemoryRouter>
    <Nodes
      {...props}
      fetchedData={{ agents: [], totalCount: 0 }}
      isSearchEmpty={true}
      canCreate={false}
    />
  </MemoryRouter>
);

export const Loading = () => (
  <MemoryRouter>
    <Nodes {...props} attempt={{ status: 'processing' }} />
  </MemoryRouter>
);

export const Failed = () => (
  <MemoryRouter>
    <Nodes
      {...props}
      attempt={{ status: 'failed', statusText: 'some error message' }}
    />
  </MemoryRouter>
);

const props: State = {
  fetchedData: {
    agents: nodes,
    totalCount: nodes.length,
  },
  fetchStatus: '',
  isLeafCluster: false,
  canCreate: true,
  attempt: { status: 'success' },
  getNodeLoginOptions: () => [{ login: 'root', url: 'fd' }],
  startSshSession: () => null,
  clusterId: 'im-a-cluster',
  fetchNext: () => null,
  fetchPrev: () => null,
  pageSize: 15,
  pageIndicators: {
    from: 1,
    to: nodes.length,
    totalCount: nodes.length,
  },
  page: {
    index: 0,
    keys: [],
  },
  params: {
    search: '',
    query: '',
    sort: { fieldName: 'hostname', dir: 'ASC' },
  },
  modifyFetchedData: () => null,
  setParams: () => null,
  setSort: () => null,
  pathname: '',
  replaceHistory: () => null,
  isSearchEmpty: false,
  onLabelClick: () => null,
};
