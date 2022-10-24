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

import { LoginTrait } from './LoginTrait';
import { State } from './useLoginTrait';

export default {
  title: 'Teleport/Discover/Kube/LoginTrait',
};

export const NoAccessAndNoTraits = () => (
  <MemoryRouter>
    <LoginTrait
      {...props}
      canEditUser={false}
      dynamicTraits={{ users: [], groups: [] }}
      staticTraits={{ users: [], groups: [] }}
    />
  </MemoryRouter>
);

export const NoAccessButHasTraits = () => (
  <MemoryRouter>
    <LoginTrait {...props} canEditUser={false} />
  </MemoryRouter>
);

export const SsoUserAndNoTraits = () => (
  <MemoryRouter>
    <LoginTrait
      {...props}
      canEditUser={false}
      isSsoUser={true}
      dynamicTraits={{ users: [], groups: [] }}
      staticTraits={{ users: [], groups: [] }}
    />
  </MemoryRouter>
);

export const SsoUserButHasTraits = () => (
  <MemoryRouter>
    <LoginTrait {...props} isSsoUser={true} />
  </MemoryRouter>
);

export const DynamicOnly = () => (
  <MemoryRouter>
    <LoginTrait {...props} staticTraits={{ users: [], groups: [] }} />
  </MemoryRouter>
);

export const StaticOnly = () => (
  <MemoryRouter>
    <LoginTrait {...props} dynamicTraits={{ users: [], groups: [] }} />
  </MemoryRouter>
);

export const GroupsOnly = () => (
  <MemoryRouter>
    <LoginTrait
      {...props}
      dynamicTraits={{ users: [], groups: [...props.dynamicTraits.groups] }}
      staticTraits={{ users: [], groups: [...props.staticTraits.groups] }}
    />
  </MemoryRouter>
);

export const UsersOnly = () => (
  <MemoryRouter>
    <LoginTrait
      {...props}
      dynamicTraits={{ groups: [], users: [...props.dynamicTraits.users] }}
      staticTraits={{ groups: [], users: [...props.staticTraits.users] }}
    />
  </MemoryRouter>
);

export const NoTraits = () => (
  <MemoryRouter>
    <LoginTrait
      {...props}
      dynamicTraits={{ users: [], groups: [] }}
      staticTraits={{ users: [], groups: [] }}
    />
  </MemoryRouter>
);

export const Processing = () => (
  <MemoryRouter>
    <LoginTrait {...props} attempt={{ status: 'processing' }} />
  </MemoryRouter>
);

export const Failed = () => (
  <MemoryRouter>
    <LoginTrait
      {...props}
      attempt={{ status: 'failed', statusText: 'some error message' }}
    />
  </MemoryRouter>
);

const props: State = {
  attempt: {
    status: 'success',
    statusText: '',
  },
  dynamicTraits: {
    users: ['root', 'llama', 'george_washington_really_long_name_testing'],
    groups: ['group1', 'group2'],
  },
  staticTraits: { users: ['staticUser1'], groups: ['staticGroup1'] },
  nextStep: () => null,
  fetchLoginTraits: () => null,
  canEditUser: true,
  isSsoUser: false,
};
