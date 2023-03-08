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

import { Node } from 'teleport/services/nodes/types';

import DocumentNodes from './DocumentNodes';
import ConsoleCtx from './../consoleContext';
import { TestLayout } from './../Console.story';

export default {
  title: 'Teleport/Console/DocumentNodes',
  excludeStories: ['createContext'],
};

export const Document = ({ value }: { value: ConsoleCtx }) => {
  const ctx = value || createContext();
  return (
    <TestLayout ctx={ctx}>
      <DocumentNodes doc={doc} visible={true} />
    </TestLayout>
  );
};

export const Loading = () => {
  const ctx = createContext();
  ctx.fetchNodes = () => new Promise(() => null);
  return (
    <TestLayout ctx={ctx}>
      <DocumentNodes doc={doc} visible={true} />
    </TestLayout>
  );
};

export const Failed = () => {
  const ctx = createContext();
  ctx.fetchNodes = () => Promise.reject<any>(new Error('Failed to load nodes'));
  return (
    <TestLayout ctx={ctx}>
      <DocumentNodes doc={doc} visible={true} />
    </TestLayout>
  );
};

export function createContext() {
  const ctx = new ConsoleCtx();

  ctx.fetchClusters = () => {
    return Promise.resolve<any>(clusters);
  };
  ctx.nodesService.fetchNodes = () => {
    return Promise.resolve({ agents: nodes, totalCount: nodes.length });
  };

  return ctx;
}

const doc = {
  clusterId: 'cluseter-1',
  created: new Date('2019-05-13T20:18:09Z'),
  kind: 'nodes',
  url: 'localhost',
} as const;

const clusters = [
  {
    clusterId: 'cluseter-1',
    connected: new Date(),
    connectedText: '',
    status: '',
    url: '',
  },
  {
    clusterId: 'cluseter-2',
    connected: new Date(),
    connectedText: '',
    status: '',
    url: '',
  },
];

const nodes: Node[] = [
  {
    tunnel: false,
    sshLogins: ['dev', 'root'],
    id: '104',
    clusterId: 'cluseter-1',
    hostname: 'fujedu',
    addr: '172.10.1.20:3022',
    labels: [
      {
        name: 'cluster',
        value: 'one',
      },
      {
        name: 'kernel',
        value: '4.15.0-51-generic',
      },
    ],
  },
  {
    tunnel: false,
    sshLogins: ['dev', 'root'],
    id: '170',
    clusterId: 'cluseter-1',
    hostname: 'facuzguv',
    addr: '172.10.1.42:3022',
    labels: [
      {
        name: 'cluster',
        value: 'one',
      },
      {
        name: 'kernel',
        value: '4.15.0-51-generic',
      },
    ],
  },
  {
    tunnel: true,
    sshLogins: ['dev', 'root'],
    id: '192',
    clusterId: 'cluseter-1',
    hostname: 'duzsevkig',
    addr: '172.10.1.156:3022',
    labels: [
      {
        name: 'cluster',
        value: 'one',
      },
      {
        name: 'kernel',
        value: '4.15.0-51-generic',
      },
    ],
  },
  {
    tunnel: true,
    sshLogins: ['dev', 'root'],
    id: '64',
    clusterId: 'cluseter-1',
    hostname: 'kuhinur',
    addr: '172.10.1.145:3022',
    labels: [
      {
        name: 'cluster',
        value: 'one',
      },
      {
        name: 'kernel',
        value: '4.15.0-51-generic',
      },
    ],
  },
  {
    tunnel: false,
    sshLogins: ['dev', 'root'],
    id: '81',
    clusterId: 'cluseter-1',
    hostname: 'zebpecda',
    addr: '172.10.1.24:3022',
    labels: [
      {
        name: 'cluster',
        value: 'one',
      },
      {
        name: 'kernel',
        value: '4.15.0-51-generic',
      },
      {
        name: 'lortavma',
        value: 'one',
      },
      {
        name: 'lenisret',
        value: '4.15.0-51-generic',
      },
      {
        name: 'lofdevod',
        value: 'one',
      },
      {
        name: 'llhurlaz',
        value: '4.15.0-51-generic',
      },
    ],
  },
];
