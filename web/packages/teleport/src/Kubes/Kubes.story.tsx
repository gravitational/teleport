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
};

export const Loaded = () => <Kubes {...props} />;

export const Empty = () => <Kubes {...props} kubes={[]} />;

export const EmptyReadOnly = () => (
  <Kubes {...props} kubes={[]} canCreate={false} />
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

const props: State = {
  attempt: { status: 'success' },
  kubes: kubes,
  username: 'sam',
  authType: 'local' as AuthType,
  clusterId: 'im-a-cluster',
  isLeafCluster: false,
  canCreate: true,
  searchValue: '',
  setSearchValue: () => null,
};
