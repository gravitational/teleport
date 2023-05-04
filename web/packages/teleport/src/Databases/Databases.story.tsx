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

import { Databases } from './Databases';
import { State } from './useDatabases';
import { databases } from './fixtures';

export default {
  title: 'Teleport/Databases',
  excludeStories: ['props'],
};

export const Loaded = () => (
  <MemoryRouter>
    <Databases {...props} />
  </MemoryRouter>
);

export const Empty = () => (
  <MemoryRouter>
    <Databases {...props} fetchedData={{ agents: [] }} isSearchEmpty={true} />
  </MemoryRouter>
);

export const EmptyReadOnly = () => (
  <MemoryRouter>
    <Databases
      {...props}
      fetchedData={{ agents: [] }}
      canCreate={false}
      isSearchEmpty={true}
    />
  </MemoryRouter>
);

export const Loading = () => (
  <MemoryRouter>
    <Databases {...props} attempt={{ status: 'processing' }} />
  </MemoryRouter>
);

export const Failed = () => (
  <MemoryRouter>
    <Databases
      {...props}
      attempt={{ status: 'failed', statusText: 'Server Error' }}
    />
  </MemoryRouter>
);

export const props: State = {
  fetchedData: {
    agents: databases,
    totalCount: databases.length,
  },
  fetchStatus: '',
  attempt: { status: 'success' },
  clusterId: 'im-a-cluster',
  isLeafCluster: false,
  canCreate: true,
  username: 'sam',
  authType: 'local',
  fetchNext: () => null,
  fetchPrev: () => null,
  pageSize: databases.length,
  pageIndicators: {
    from: 1,
    to: databases.length,
    totalCount: databases.length,
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
