/*
Copyright 2020 Gravitational, Inc.

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

import { Node } from 'teleport/services/nodes';

export const nodes: Node[] = [
  {
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
    tunnel: false,
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
    tunnel: false,
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
    tunnel: false,
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
];

export const joinToken = {
  id: '12',
  expiry: new Date(),
};
