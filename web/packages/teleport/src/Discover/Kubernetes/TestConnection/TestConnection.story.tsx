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

import { MemoryRouter } from 'react-router';

import { TestConnection } from './TestConnection';
import type { State } from './useTestConnection';

export default {
  title: 'Teleport/Discover/Kube/TestConnection',
};

export const InitWithLocal = () => (
  <MemoryRouter>
    <TestConnection {...props} />
  </MemoryRouter>
);

export const InitWithSso = () => (
  <MemoryRouter>
    <TestConnection {...props} authType="sso" />
  </MemoryRouter>
);

export const WithKubeUsers = () => (
  <MemoryRouter>
    <TestConnection
      {...props}
      kube={{
        kind: 'kube_cluster',
        name: 'some-kube-name',
        labels: [],
        users: ['user1', 'user2'],
        groups: [],
      }}
    />
  </MemoryRouter>
);

export const WithKubeGroups = () => (
  <MemoryRouter>
    <TestConnection
      {...props}
      kube={{
        kind: 'kube_cluster',
        name: 'some-kube-name',
        labels: [],
        users: [],
        groups: ['group1', 'group2'],
      }}
    />
  </MemoryRouter>
);

export const WithKubeUsersAndGroups = () => (
  <MemoryRouter>
    <TestConnection
      {...props}
      kube={{
        kind: 'kube_cluster',
        name: 'some-kube-name',
        labels: [],
        users: ['user1', 'user2'],
        groups: ['group1', 'group2'],
      }}
    />
  </MemoryRouter>
);

const props: State = {
  attempt: {
    status: 'success',
    statusText: '',
  },
  testConnection: () => null,
  nextStep: () => null,
  prevStep: () => null,
  diagnosis: null,
  canTestConnection: true,
  kube: {
    kind: 'kube_cluster',
    name: 'some-kube-name',
    labels: [],
    users: [],
    groups: [],
  },
  username: 'teleport-username',
  authType: 'local',
  clusterId: 'some-cluster-id',
  showMfaDialog: false,
  cancelMfaDialog: () => null,
};
