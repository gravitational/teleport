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

import { Node } from 'teleport/services/nodes/types';

import { TestLayout } from './../Console.story';
import ConsoleCtx from './../consoleContext';
import DocumentNodes from './DocumentNodes';

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
  ctx.nodesService.fetchNodes = () => new Promise(() => null);
  return (
    <TestLayout ctx={ctx}>
      <DocumentNodes doc={doc} visible={true} />
    </TestLayout>
  );
};

export const Failed = () => {
  const ctx = createContext();
  ctx.nodesService.fetchNodes = () =>
    Promise.reject<any>(new Error('Failed to load nodes'));
  return (
    <TestLayout ctx={ctx}>
      <DocumentNodes doc={doc} visible={true} />
    </TestLayout>
  );
};

export function createContext(): ConsoleCtx {
  const ctx = new ConsoleCtx();

  ctx.clustersService.fetchClusters = () => {
    return Promise.resolve<any>(clusters);
  };

  ctx.nodesService.fetchNodes = () => {
    return Promise.resolve({ agents: nodes, totalCount: nodes.length });
  };

  return ctx;
}

const doc = {
  clusterId: 'cluster-1',
  created: new Date('2019-05-13T20:18:09Z'),
  kind: 'nodes',
  url: 'localhost',
  latency: {
    client: 0,
    server: 0,
  },
} as const;

const clusters = [
  {
    clusterId: 'cluster-1',
    connected: new Date(),
    connectedText: '',
    status: '',
    url: '',
  },
  {
    clusterId: 'cluster-2',
    connected: new Date(),
    connectedText: '',
    status: '',
    url: '',
  },
];

const nodes: Node[] = [
  {
    kind: 'node',
    subKind: 'teleport',
    tunnel: false,
    sshLogins: ['dev', 'root'],
    id: '104',
    clusterId: 'cluster-1',
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
    kind: 'node',
    subKind: 'teleport',
    tunnel: false,
    sshLogins: ['dev', 'root'],
    id: '170',
    clusterId: 'cluster-1',
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
    kind: 'node',
    subKind: 'teleport',
    tunnel: true,
    sshLogins: ['dev', 'root'],
    id: '192',
    clusterId: 'cluster-1',
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
    kind: 'node',
    subKind: 'teleport',
    tunnel: true,
    sshLogins: ['dev', 'root'],
    id: '64',
    clusterId: 'cluster-1',
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
    kind: 'node',
    subKind: 'teleport',
    tunnel: false,
    sshLogins: ['dev', 'root'],
    id: '81',
    clusterId: 'cluster-1',
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
