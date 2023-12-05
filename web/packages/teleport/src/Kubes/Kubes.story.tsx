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
