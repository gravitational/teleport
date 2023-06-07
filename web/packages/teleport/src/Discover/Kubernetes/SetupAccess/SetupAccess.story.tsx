/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import { MemoryRouter } from 'react-router';

import { initSelectedOptionsHelper } from 'teleport/Discover/Shared/SetupAccess';

import { SetupAccess } from './SetupAccess';

import type { State } from 'teleport/Discover/Shared/SetupAccess';

export default {
  title: 'Teleport/Discover/Kube/SetupAccess',
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
  kubeUsers: ['staticUser1', 'staticUser2'],
  kubeGroups: ['staticGroup1', 'staticGroup2'],
  logins: [],
  databaseUsers: [],
  databaseNames: [],
  windowsLogins: [],
  awsRoleArns: [],
};

const dynamicTraits = {
  kubeUsers: ['dynamicUser1', 'dynamicUser2'],
  kubeGroups: ['dynamicGroup1', 'dynamicGroup2'],
  logins: [],
  databaseUsers: [],
  databaseNames: [],
  windowsLogins: [],
  awsRoleArns: [],
};
