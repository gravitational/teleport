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

import { AuthType } from 'teleport/services/user';

import { Kubes } from './Kubes';
import { State } from './useKubes';
import { kubes } from './fixtures';

export default {
  title: 'Teleport/Kubes',
  excludeStories: ['props'],
};

export const Loaded = () => <Kubes {...props} />;

export const Empty = () => (
  <Kubes {...props} results={{ kubes: [] }} isSearchEmpty={true} />
);

export const EmptyReadOnly = () => (
  <Kubes
    {...props}
    results={{ kubes: [] }}
    canCreate={false}
    isSearchEmpty={true}
  />
);

export const Loading = () => (
  <Kubes {...props} attempt={{ status: 'processing' }} />
);

export const Failed = () => (
  <Kubes
    {...props}
    attempt={{ status: 'failed', statusText: 'server error' }}
  />
);

export const props: State = {
  results: {
    kubes,
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
  from: 1,
  to: kubes.length,
  params: {
    search: '',
    query: '',
    sort: { fieldName: 'name', dir: 'ASC' },
  },
  setParams: () => null,
  setSort: () => null,
  startKeys: [''],
  pathname: '',
  replaceHistory: () => null,
  isSearchEmpty: false,
  onLabelClick: () => null,
};
