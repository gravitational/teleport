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

import { SetupAccessWrapper, type Props } from './SetupAccessWrapper';

export default {
  title: 'Teleport/Discover/Shared/SetupAccessWrapper',
};

export const HasAccessAndTraits = () => (
  <MemoryRouter
    future={{ v7_relativeSplatPath: true, v7_startTransition: true }}
  >
    <SetupAccessWrapper {...props} />
  </MemoryRouter>
);

export const HasAccessButNoTraits = () => (
  <MemoryRouter
    future={{ v7_relativeSplatPath: true, v7_startTransition: true }}
  >
    <SetupAccessWrapper {...props} hasTraits={false} />
  </MemoryRouter>
);

export const NoAccessAndNoTraits = () => (
  <MemoryRouter
    future={{ v7_relativeSplatPath: true, v7_startTransition: true }}
  >
    <SetupAccessWrapper {...props} canEditUser={false} hasTraits={false} />
  </MemoryRouter>
);

export const NoAccessButHasTraits = () => (
  <MemoryRouter
    future={{ v7_relativeSplatPath: true, v7_startTransition: true }}
  >
    <SetupAccessWrapper {...props} canEditUser={false} />
  </MemoryRouter>
);

export const SsoUserAndNoTraits = () => (
  <MemoryRouter
    future={{ v7_relativeSplatPath: true, v7_startTransition: true }}
  >
    <SetupAccessWrapper
      {...props}
      canEditUser={false}
      isSsoUser={true}
      hasTraits={false}
    />
  </MemoryRouter>
);

export const SsoUserButHasTraits = () => (
  <MemoryRouter
    future={{ v7_relativeSplatPath: true, v7_startTransition: true }}
  >
    <SetupAccessWrapper {...props} isSsoUser={true} />
  </MemoryRouter>
);

export const Processing = () => (
  <MemoryRouter
    future={{ v7_relativeSplatPath: true, v7_startTransition: true }}
  >
    <SetupAccessWrapper {...props} attempt={{ status: 'processing' }} />
  </MemoryRouter>
);

export const Failed = () => (
  <MemoryRouter
    future={{ v7_relativeSplatPath: true, v7_startTransition: true }}
  >
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
