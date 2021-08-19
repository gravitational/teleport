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
import { Databases } from './Databases';
import { State } from './useDesktops';
import { databases } from './fixtures';

export default {
  title: 'Teleport/Databases',
};

export const Loaded = () => <Databases {...props} />;

export const Empty = () => <Databases {...props} databases={[]} />;

export const EmptyReadOnly = () => (
  <Databases {...props} databases={[]} canCreate={false} />
);

export const Loading = () => (
  <Databases {...props} attempt={{ status: 'processing' }} />
);

export const Failed = () => (
  <Databases
    {...props}
    attempt={{ status: 'failed', statusText: 'Server Error' }}
  />
);

const props: State = {
  attempt: { status: 'success' },
  databases,
  clusterId: 'im-a-cluster',
  isLeafCluster: false,
  isEnterprise: false,
  canCreate: true,
  isAddDialogVisible: false,
  hideAddDialog: () => null,
  showAddDialog: () => null,
  username: 'sam',
  version: '6.1.3',
  authType: 'local',
  searchValue: '',
  setSearchValue: () => null,
};
