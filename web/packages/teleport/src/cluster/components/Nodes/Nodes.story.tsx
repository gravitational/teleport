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

type PropTypes = Parameters<typeof Nodes>[0];

export default {
  title: 'Teleport/ClusterNodes',
};

export function Loaded() {
  return render({ isSuccess: true });
}

export function Loading() {
  return render({ isProcessing: true });
}

export function Failed() {
  return render({ isFailed: true, message: 'server error' });
}

function render(attemptOptions: Partial<PropTypes['attempt']>) {
  const attempt = {
    isProcessing: false,
    isSuccess: false,
    isFailed: false,
    message: '',
    ...attemptOptions,
  };

  return (
    <Nodes
      attempt={attempt}
      getNodeLoginOptions={() => [{ login: 'root', url: 'fd' }]}
      nodes={nodes}
    />
  );
}

const nodes = [
  {
    tunnel: false,
    id: '104',
    clusterId: 'one',
    hostname: 'fujedu',
    addr: '172.10.1.20:3022',
    tags: [
      { name: 'cluster', value: 'one' },
      { name: 'kernel', value: '4.15.0-51-generic' },
    ],
  },
  {
    tunnel: false,
    id: '170',
    clusterId: 'one',
    hostname: 'facuzguv',
    addr: '172.10.1.1:3022',
    tags: [
      { name: 'cluster', value: 'one' },
      { name: 'kernel', value: '4.15.0-51-generic' },
    ],
  },
  {
    tunnel: false,
    id: '192',
    clusterId: 'one',
    hostname: 'duzsevkig',
    addr: '172.10.1.1:3022',
    tags: [
      { name: 'cluster', value: 'one' },
      { name: 'kernel', value: '4.15.0-51-generic' },
    ],
  },
  {
    tunnel: false,
    id: '64',
    clusterId: 'one',
    hostname: 'kuhinur',
    addr: '172.10.1.1:3022',
    tags: [
      { name: 'cluster', value: 'one' },
      { name: 'kernel', value: '4.15.0-51-generic' },
    ],
  },
  {
    tunnel: false,
    id: '81',
    clusterId: 'one',
    hostname: 'zebpecda',
    addr: '172.10.1.1:3022',
    tags: [
      { name: 'cluster', value: 'one' },
      { name: 'kernel', value: '4.15.0-51-generic' },
    ],
  },
  {
    tunnel: true,
    id: '81',
    clusterId: 'one',
    hostname: 'zebpecda',
    addr: '172.10.1.1:3022',
    tags: [
      { name: 'cluster', value: 'one' },
      { name: 'kernel', value: '4.15.0-51-generic' },
    ],
  },
  {
    tunnel: true,
    id: '81',
    clusterId: 'one',
    hostname: 'zebpecda',
    addr: '172.10.1.1:3022',
    tags: [
      { name: 'cluster', value: 'one' },
      { name: 'kernel', value: '4.15.0-51-generic' },
    ],
  },
];
