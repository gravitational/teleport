/*
Copyright 2021 Gravitational, Inc.

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

import { AuthType } from 'teleport/services/user';

import { Kubes } from './Kubes';
import { State } from './useKubes';
import { kubes } from './fixtures';

export default {
  title: 'Teleport/Kubes',
  excludeStories: ['props'],
};

export const Loaded = () => (
  <MemoryRouter>
    <Kubes {...props} />
  </MemoryRouter>
);

export const Empty = () => (
  <MemoryRouter>
    <Kubes {...props} fetchedData={{ agents: [] }} isSearchEmpty={true} />
  </MemoryRouter>
);

export const EmptyReadOnly = () => (
  <MemoryRouter>
    <Kubes
      {...props}
      fetchedData={{ agents: [] }}
      canCreate={false}
      isSearchEmpty={true}
    />
  </MemoryRouter>
);

export const Loading = () => (
  <MemoryRouter>
    <Kubes {...props} attempt={{ status: 'processing' }} />
  </MemoryRouter>
);

export const Failed = () => (
  <MemoryRouter>
    <Kubes
      {...props}
      attempt={{ status: 'failed', statusText: 'server error' }}
    />
  </MemoryRouter>
);

export const props: State = {
  fetchedData: {
    agents: kubes,
    totalCount: kubes.length,
  },
  fetchStatus: '',
  attempt: { status: 'success' },
  username: 'sam',
  authType: 'local' as AuthType,
  clusterId: 'im-a-cluster',
  isLeafCluster: false,
  canCreate: true,
  fetchNext: () => null,
  fetchPrev: () => null,
  pageSize: kubes.length,
  pageIndicators: {
    from: 1,
    to: kubes.length,
    totalCount: kubes.length,
  },
  page: {
    index: 0,
    keys: [],
  },
  params: {
    search: '',
    query: '',
    sort: { fieldName: 'name', dir: 'ASC' },
  },
  setParams: () => null,
  setSort: () => null,
  pathname: '',
  replaceHistory: () => null,
  isSearchEmpty: false,
  onLabelClick: () => null,
  accessRequestId: null,
};
