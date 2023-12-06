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
