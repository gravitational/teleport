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

import { SetupAccessWrapper } from './SetupAccessWrapper';

import type { Props } from './SetupAccessWrapper';

export default {
  title: 'Teleport/Discover/Shared/SetupAccessContainer',
};

export const HasAccessAndTraits = () => (
  <MemoryRouter>
    <SetupAccessWrapper {...props} />
  </MemoryRouter>
);

export const HasAccessButNoTraits = () => (
  <MemoryRouter>
    <SetupAccessWrapper {...props} hasTraits={false} />
  </MemoryRouter>
);

export const NoAccessAndNoTraits = () => (
  <MemoryRouter>
    <SetupAccessWrapper {...props} canEditUser={false} hasTraits={false} />
  </MemoryRouter>
);

export const NoAccessButHasTraits = () => (
  <MemoryRouter>
    <SetupAccessWrapper {...props} canEditUser={false} />
  </MemoryRouter>
);

export const SsoUserAndNoTraits = () => (
  <MemoryRouter>
    <SetupAccessWrapper
      {...props}
      canEditUser={false}
      isSsoUser={true}
      hasTraits={false}
    />
  </MemoryRouter>
);

export const SsoUserButHasTraits = () => (
  <MemoryRouter>
    <SetupAccessWrapper {...props} isSsoUser={true} />
  </MemoryRouter>
);

export const Processing = () => (
  <MemoryRouter>
    <SetupAccessWrapper {...props} attempt={{ status: 'processing' }} />
  </MemoryRouter>
);

export const Failed = () => (
  <MemoryRouter>
    <SetupAccessWrapper
      {...props}
      attempt={{ status: 'failed', statusText: 'some error message' }}
    />
  </MemoryRouter>
);

const props: Props = {
  isSsoUser: false,
  canEditUser: true,
  attempt: {
    status: 'success',
    statusText: '',
  },
  fetchUserTraits: () => null,
  headerSubtitle: 'Some kind of header subtitle',
  traitKind: 'Kubernetes',
  traitDescription: 'users and groups',
  hasTraits: true,
  onProceed: () => null,
  onPrev: () => null,
  children: <div>This is where trait selection children renders</div>,
};
