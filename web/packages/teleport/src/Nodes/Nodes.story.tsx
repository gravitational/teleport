/*
Copyright 2019 Gravitational, Inc.

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
import { Nodes } from './Nodes';
import { State } from './useNodes';
import { nodes } from './fixtures';

export default {
  title: 'Teleport/Nodes',
};

export const Loaded = () => <Nodes {...props} />;

export const Empty = () => <Nodes {...props} nodes={[]} />;

export const EmptyReadOnly = () => (
  <Nodes {...props} nodes={[]} canCreate={false} />
);

export const Loading = () => (
  <Nodes {...props} attempt={{ status: 'processing' }} />
);

export const Failed = () => (
  <Nodes
    {...props}
    attempt={{ status: 'failed', statusText: 'some error message' }}
  />
);

const props: State = {
  nodes,
  isLeafCluster: false,
  canCreate: true,
  attempt: { status: 'success' },
  getNodeLoginOptions: () => [{ login: 'root', url: 'fd' }],
  startSshSession: () => null,
  isAddNodeVisible: false,
  hideAddNode: () => null,
  showAddNode: () => null,
  clusterId: 'im-a-cluster',
  searchValue: '',
  setSearchValue: () => null,
};
