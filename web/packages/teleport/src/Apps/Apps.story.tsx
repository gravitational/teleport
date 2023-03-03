/**
 * Copyright 2020 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import { MemoryRouter } from 'react-router';

import { Apps } from './Apps';
import { State } from './useApps';
import { apps } from './fixtures';

export default {
  title: 'Teleport/Apps',
  excludeStories: ['props'],
};

export const Loaded = () => (
  <MemoryRouter>
    <Apps {...props} />
  </MemoryRouter>
);

export const Empty = () => (
  <MemoryRouter>
    <Apps {...props} fetchedData={{ agents: [] }} isSearchEmpty={true} />
  </MemoryRouter>
);

export const EmptyReadOnly = () => (
  <MemoryRouter>
    <Apps
      {...props}
      fetchedData={{ agents: [] }}
      canCreate={false}
      isSearchEmpty={true}
    />
  </MemoryRouter>
);

export const Loading = () => (
  <MemoryRouter>
    <Apps {...props} attempt={{ status: 'processing' }} />
  </MemoryRouter>
);

export const Failed = () => (
  <MemoryRouter>
    <Apps
      {...props}
      attempt={{ status: 'failed', statusText: 'some error message' }}
    />
  </MemoryRouter>
);

export const props: State = {
  fetchedData: {
    agents: apps,
    totalCount: apps.length,
  },
  fetchStatus: '',
  attempt: { status: 'success' },
  clusterId: 'im-a-cluster',
  isLeafCluster: false,
  isEnterprise: false,
  canCreate: true,
  fetchNext: () => null,
  fetchPrev: () => null,
  pageSize: apps.length,
  pageIndicators: {
    from: 1,
    to: apps.length,
    totalCount: apps.length,
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
};
