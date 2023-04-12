/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
  setParams: () => null,
  setSort: () => null,
  pathname: '',
  replaceHistory: () => null,
  isSearchEmpty: false,
  onLabelClick: () => null,
};
