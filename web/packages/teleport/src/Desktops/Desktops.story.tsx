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
