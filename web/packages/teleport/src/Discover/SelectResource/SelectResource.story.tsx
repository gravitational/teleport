/**
 * Copyright 2023 Gravitational, Inc.
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

import {
  createTeleportContext,
  getAcl,
  noAccess,
} from 'teleport/mocks/contexts';
import { ContextProvider } from 'teleport';

import { UserContext } from 'teleport/User/UserContext';

import { makeDefaultUserPreferences } from 'teleport/services/userPreferences/userPreferences';

import {
  ClusterResource,
  UserPreferences,
} from 'teleport/services/userPreferences/types';
import { Acl } from 'teleport/services/user';

import { SelectResource } from './SelectResource';

export default {
  title: 'Teleport/Discover/SelectResource',
};

export const AllAccess = () => {
  return (
    <Provider>
      <SelectResource onSelect={() => null} />
    </Provider>
  );
};

export const NoAccess = () => {
  const customAcl = getAcl({ noAccess: true });
  return (
    <Provider customAcl={customAcl}>
      <SelectResource onSelect={() => null} />
    </Provider>
  );
};

export const PartialAccess = () => {
  const customAcl = getAcl();
  customAcl.dbServers = noAccess;
  return (
    <Provider customAcl={customAcl}>
      <SelectResource onSelect={() => null} />
    </Provider>
  );
};

export const PreferredDBAccess = () => {
  return (
    <Provider resources={[3]}>
      <SelectResource onSelect={() => null} />
    </Provider>
  );
};

export const InitRouteEntryServer = () => (
  <Provider entity="server">
    <SelectResource onSelect={() => null} />
  </Provider>
);

type ProviderProps = {
  customAcl?: Acl;
  entity?: string;
  resources?: ClusterResource[];
  children?: React.ReactNode;
};

const Provider = ({
  customAcl,
  entity,
  resources,
  children,
}: ProviderProps) => {
  const ctx = createTeleportContext({ customAcl: customAcl });
  const updatePreferences = () => Promise.resolve();
  const preferences: UserPreferences = makeDefaultUserPreferences();
  preferences.onboard = { preferredResources: resources };

  return (
    <MemoryRouter
      initialEntries={[{ pathname: '/test', state: { entity: entity } }]}
    >
      <UserContext.Provider value={{ preferences, updatePreferences }}>
        <ContextProvider ctx={ctx}>{children}</ContextProvider>
      </UserContext.Provider>
    </MemoryRouter>
  );
};
