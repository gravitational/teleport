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

import {
  initSelectedOptionsHelper,
  type State,
} from 'teleport/Discover/Shared/SetupAccess';

import { SetupAccess } from './SetupAccess';

export default {
  title: 'Teleport/Discover/Server/SetupAccess',
};

export const NoTraits = () => (
  <MemoryRouter>
    <SetupAccess {...props} initSelectedOptions={() => []} />
  </MemoryRouter>
);

export const WithTraits = () => (
  <MemoryRouter>
    <SetupAccess {...props} />
  </MemoryRouter>
);

export const NoAccess = () => (
  <MemoryRouter>
    <SetupAccess {...props} canEditUser={false} />
  </MemoryRouter>
);

export const SsoUser = () => (
  <MemoryRouter>
    <SetupAccess {...props} isSsoUser={true} />
  </MemoryRouter>
);

const props: State = {
  attempt: {
    status: 'success',
    statusText: '',
  },
  agentMeta: {} as any,
  onProceed: () => null,
  onPrev: () => null,
  fetchUserTraits: () => null,
  isSsoUser: false,
  canEditUser: true,
  getFixedOptions: () => [],
  getSelectableOptions: () => [],
  initSelectedOptions: trait =>
    initSelectedOptionsHelper({ trait, staticTraits, dynamicTraits }),
  dynamicTraits: {} as any,
  staticTraits: {} as any,
  resourceSpec: {} as any,
};

const staticTraits = {
  kubeUsers: [],
  kubeGroups: [],
  logins: ['staticLogin1', 'staticLogin2'],
  databaseUsers: [],
  databaseNames: [],
  windowsLogins: [],
  awsRoleArns: [],
};

const dynamicTraits = {
  kubeUsers: [],
  kubeGroups: [],
  logins: ['dynamicLogin1', 'dynamicLogin2'],
  databaseUsers: [],
  databaseNames: [],
  windowsLogins: [],
  awsRoleArns: [],
};
