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

import React from 'react';
import { MemoryRouter } from 'react-router';

import { Resource } from 'gen-proto-ts/teleport/userpreferences/v1/onboard_pb';
import { UserPreferences } from 'gen-proto-ts/teleport/userpreferences/v1/userpreferences_pb';

import { ContextProvider } from 'teleport';
import {
  createTeleportContext,
  getAcl,
  noAccess,
} from 'teleport/mocks/contexts';
import { Acl } from 'teleport/services/user';
import { makeDefaultUserPreferences } from 'teleport/services/userPreferences/userPreferences';
import { UserContext } from 'teleport/User/UserContext';

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
  resources?: Resource[];
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
  const getClusterPinnedResources = () => Promise.resolve([]);
  const updateClusterPinnedResources = () => Promise.resolve();
  const updateDiscoverResourcePreferences = () => Promise.resolve();
  const preferences: UserPreferences = makeDefaultUserPreferences();
  preferences.onboard.preferredResources = resources;

  return (
    <MemoryRouter
      initialEntries={[{ pathname: '/test', state: { entity: entity } }]}
    >
      <UserContext.Provider
        value={{
          preferences,
          updatePreferences,
          getClusterPinnedResources,
          updateClusterPinnedResources,
          updateDiscoverResourcePreferences,
        }}
      >
        <ContextProvider ctx={ctx}>{children}</ContextProvider>
      </UserContext.Provider>
    </MemoryRouter>
  );
};
