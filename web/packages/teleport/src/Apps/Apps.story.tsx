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
import { Apps } from './Apps';
import { State } from './useApps';
import { apps } from './fixtures';

export default {
  title: 'Teleport/Apps',
};

export const Loaded = () => <Apps {...props} />;

export const Empty = () => <Apps {...props} apps={[]} />;

export const EmptyReadOnly = () => (
  <Apps {...props} apps={[]} canCreate={false} />
);

export const Loading = () => (
  <Apps {...props} attempt={{ status: 'processing' }} />
);

export const Failed = () => (
  <Apps
    {...props}
    attempt={{ status: 'failed', statusText: 'some error message' }}
  />
);

const props: State = {
  apps,
  attempt: { status: 'success' },
  clusterId: 'im-a-cluster',
  isLeafCluster: false,
  isEnterprise: false,
  isAddAppVisible: false,
  canCreate: true,
  searchValue: '',
  setSearchValue: () => null,
  hideAddApp: () => null,
  showAddApp: () => null,
};
