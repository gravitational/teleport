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

import { State } from './useDesktops';
import { Desktops } from './Desktops';
import { desktops } from './fixtures';

export default {
  title: 'Teleport/Desktops',
  excludeStories: ['props'],
};

export const Loading = () => (
  <MemoryRouter>
    <Desktops {...props} attempt={{ status: 'processing' }} />
  </MemoryRouter>
);

export const Loaded = () => (
  <MemoryRouter>
    <Desktops {...props} />
  </MemoryRouter>
);

export const Empty = () => (
  <MemoryRouter>
    <Desktops {...props} fetchedData={{ agents: [] }} isSearchEmpty={true} />
  </MemoryRouter>
);

export const Failed = () => (
  <MemoryRouter>
    <Desktops
      {...props}
      attempt={{ status: 'failed', statusText: 'Server Error' }}
    />
  </MemoryRouter>
);

export const props: State = {
  fetchedData: {
    agents: desktops,
    totalCount: desktops.length,
  },
  fetchStatus: '',
  attempt: { status: 'success' },
  username: 'user',
  clusterId: 'im-a-cluster',
  canCreate: true,
  isLeafCluster: false,
  getWindowsLoginOptions: () => [{ login: '', url: '' }],
  openRemoteDesktopTab: () => {},
  fetchNext: () => null,
  fetchPrev: () => null,
  pageSize: desktops.length,
  pageIndicators: {
    from: 1,
    to: desktops.length,
    totalCount: desktops.length,
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
