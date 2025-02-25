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

import { Node } from 'teleport/services/nodes';

export const nodes: Node[] = [
  {
    kind: 'node',
    subKind: 'teleport',
    tunnel: false,
    sshLogins: ['dev', 'root'],
    id: '104',
    clusterId: 'one',
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
    tunnel: false,
    subKind: 'teleport',
    sshLogins: ['dev', 'root'],
    id: '170',
    clusterId: 'one',
    hostname: 'facuzguv',
    addr: '172.10.1.1:3022',
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
    id: '192',
    clusterId: 'one',
    hostname: 'duzsevkig',
    addr: '172.10.1.1:3022',
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
    tunnel: false,
    subKind: 'teleport',
    sshLogins: ['dev', 'root'],
    id: '64',
    clusterId: 'one',
    hostname: 'kuhinur',
    addr: '172.10.1.1:3022',
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
    tunnel: false,
    subKind: 'teleport',
    sshLogins: ['dev', 'root'],
    id: '81',
    clusterId: 'one',
    hostname: 'Vukuron',
    addr: '172.10.1.1:3022',
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
    id: '81',
    clusterId: 'one',
    hostname: 'zebpecda',
    addr: '172.10.1.1:3022',
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
    tunnel: true,
    subKind: 'teleport',
    sshLogins: ['dev', 'root'],
    id: '81',
    clusterId: 'one',
    hostname: 'Nerjaeb',
    addr: '172.10.1.1:3022',
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
];

export const moreNodes: Node[] = [
  {
    kind: 'node',
    subKind: 'teleport',
    tunnel: false,
    sshLogins: ['dev', 'root'],
    id: '104',
    clusterId: 'one',
    hostname: 'Ajetokev',
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
    tunnel: false,
    subKind: 'teleport',
    sshLogins: ['dev', 'root'],
    id: '170',
    clusterId: 'one',
    hostname: 'Vahkulhig',
    addr: '172.10.1.1:3022',
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
    id: '192',
    clusterId: 'one',
    hostname: 'Jugafol',
    addr: '172.10.1.1:3022',
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
    tunnel: false,
    subKind: 'teleport',
    sshLogins: ['dev', 'root'],
    id: '64',
    clusterId: 'one',
    hostname: 'Birnelop',
    addr: '172.10.1.1:3022',
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
    tunnel: false,
    subKind: 'teleport',
    sshLogins: ['dev', 'root'],
    id: '81',
    clusterId: 'one',
    hostname: 'Cavzuvha',
    addr: '172.10.1.1:3022',
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
    id: '81',
    clusterId: 'one',
    hostname: 'Soemki',
    addr: '172.10.1.1:3022',
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
    tunnel: true,
    subKind: 'teleport',
    sshLogins: ['dev', 'root'],
    id: '81',
    clusterId: 'one',
    hostname: 'Toaseha',
    addr: '172.10.1.1:3022',
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
];

export const joinToken = {
  id: '12',
  expiry: new Date(),
};
