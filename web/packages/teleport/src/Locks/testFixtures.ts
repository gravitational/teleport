/*
Copyright 2023 Gravitational, Inc.

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

export const HOOK_LIST = [
  {
    name: '1ecfe67f-a59b-4309-b6fc-a9981891e82a',
    message: "you've been bad",
    expires: '2023-03-18T02:14:01.659948Z',
    createdAt: '',
    createdBy: '',
    targets: { user: 'worker' },
  },
  {
    name: '3ec76143-1ebb-4328-acbb-83799919e2a8',
    message: 'Forever gone',
    expires: '',
    createdAt: '2023-03-20T16:57:17.117411Z',
    createdBy: 'tele-admin-local',
    targets: { user: 'worker' },
  },
  {
    name: '5df33ee0-6368-4f9d-b8d1-9a4121830018',
    message: '',
    expires: '2023-03-20T21:10:17.529834Z',
    createdAt: '2023-03-20T19:10:17.533992Z',
    createdBy: 'tele-admin-local',
    targets: { user: 'worker' },
  },
  {
    name: '60626e99-e91b-41b2-89fe-bf5d16b0c622',
    message: 'No contractors allowed right now',
    expires: '2023-03-20T19:36:15.028132Z',
    createdAt: '2023-03-20T14:36:15.046728Z',
    createdBy: 'tele-admin',
    targets: { role: 'contractor' },
  },
];

export const HOOK_CREATED = {
  kind: 'lock',
  version: 'v2',
  metadata: { name: '1b807c9f-2144-4f7f-8d3e-9c1e14cb5b98' },
  spec: {
    target: { user: 'banned' },
    message: "you've been bad",
    expires: '2023-03-20T21:51:18.466627Z',
    created_at: '0001-01-01T00:00:00Z',
  },
};

// Responses from the service fetch methods
// ex) desktopService.fetchDesktops, nodeService.fetchNodes, etc.

export const MFA_DEVICES = [
  {
    id: '4bac1adb-fdaa-4c31-a989-317892a9d1bd',
    name: 'yubikey',
    description: 'Hardware Key',
    registeredDate: '2023-03-14T19:22:59.437Z',
    lastUsedDate: '2023-03-21T19:03:54.874Z',
  },
];

export const DESKTOPS = {
  agents: [
    {
      os: 'windows',
      name: 'watermelon',
      addr: 'localhost.watermelon',
      labels: [
        {
          name: 'env',
          value: 'test',
        },
        {
          name: 'os',
          value: 'os',
        },
        {
          name: 'unique-id',
          value: '47c38f49-b690-43fd-ac28-946e7a0a6188',
        },
        {
          name: 'windows-desktops',
          value: 'watermelon',
        },
      ],
      host_id: '47c38f49-b690-43fd-ac28-946e7a0a6188',
      logins: [],
    },
    {
      os: 'windows',
      name: 'banana',
      addr: 'localhost.banana',
      labels: [
        {
          name: 'env',
          value: 'test',
        },
        {
          name: 'os',
          value: 'linux',
        },
        {
          name: 'unique-id',
          value: '4c3bd959-8444-492a-a383-a29378da93c9',
        },
        {
          name: 'windows-desktops',
          value: 'banana',
        },
      ],
      host_id: '4c3bd959-8444-492a-a383-a29378da93c9',
      logins: [],
    },
  ],
  startKey: '',
  totalCount: 0,
};

export const NODES = {
  agents: [
    {
      id: 'e14baac6-15c1-42c2-a7d9-99410d21cf4c',
      clusterId: 'local-test2',
      hostname: 'node1.go.citadel',
      labels: ['special:apple', 'user:orange'],
      addr: '127.0.0.1:4022',
      tunnel: false,
      sshLogins: [],
    },
  ],
  startKey: '',
  totalCount: 0,
};

export const ROLES = [
  {
    id: 'role:admin',
    kind: 'role',
    name: 'admin',
    content: '',
  },
  {
    id: 'role:contractor',
    kind: 'role',
    name: 'contractor',
    content: '',
  },
  {
    id: 'role:locksmith',
    kind: 'role',
    name: 'locksmith',
    content: '',
  },
];

export const USERS = [
  {
    name: 'admin-local',
    roles: ['access', 'admin', 'auditor', 'editor'],
    authType: 'local',
  },
  {
    name: 'admin',
    roles: ['access', 'admin', 'auditor', 'editor', 'locksmith'],
    authType: 'local',
  },
  {
    name: 'worker',
    roles: ['access', 'contractor'],
    authType: 'local',
  },
];

export const mockedUseTeleportUtils = {
  mfaService: {
    fetchDevices: () => new Promise(resolve => resolve(MFA_DEVICES)),
  },
  desktopService: {
    fetchDesktops: () => new Promise(resolve => resolve(DESKTOPS)),
  },
  nodeService: {
    fetchNodes: () => new Promise(resolve => resolve(NODES)),
  },
  resourceService: {
    fetchRoles: () => new Promise(resolve => resolve(ROLES)),
  },
  userService: {
    fetchUsers: () => new Promise(resolve => resolve(USERS)),
  },
};

export const USER_RESULT = [
  { name: 'admin-local', roles: 'access, admin, auditor, editor' },
  { name: 'admin', roles: 'access, admin, auditor, editor, locksmith' },
  { name: 'worker', roles: 'access, contractor' },
];

export const ROLES_RESULT = [
  { name: 'admin' },
  { name: 'contractor' },
  { name: 'locksmith' },
];
