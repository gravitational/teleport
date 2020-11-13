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
import { nodes } from './fixtures';

type PropTypes = Parameters<typeof Nodes>[0];

export default {
  title: 'Teleport/Nodes',
};

export function Loaded() {
  return render({ status: 'success' });
}

export function Loading() {
  return render({ status: 'processing' });
}

export function Failed() {
  return render({ status: 'failed', statusText: 'server error' });
}

export function Empty() {
  return render({ status: 'success' }, []);
}

function render(
  attemptOptions: Partial<PropTypes['attempt']>,
  nodeList = nodes
) {
  const props = {
    isEnterprise: true,
    canCreate: true,
    searchValue: '',
    setSearchValue: () => null,
    attempt: {
      status: '' as any,
      statusText: '',
      ...attemptOptions,
    },
    nodes: nodeList,
    getNodeLoginOptions: () => [{ login: 'root', url: 'fd' }],
    startSshSession: () => null,
    isAddNodeVisible: false,
    hideAddNode: () => null,
    showAddNode: () => null,
  };

  return <Nodes {...props} />;
}
