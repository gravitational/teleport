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

import { SelectResource } from './SelectResource';

export default {
  title: 'Teleport/Discover/SelectResource',
};

export const AllAccess = () => (
  <Provider>
    <SelectResource onSelect={() => null} />
  </Provider>
);

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

export const InitRouteEntryServer = () => (
  <Provider entity="server">
    <SelectResource onSelect={() => null} />
  </Provider>
);

const Provider = props => {
  const ctx = createTeleportContext({ customAcl: props.customAcl });

  return (
    <MemoryRouter
      initialEntries={[{ pathname: '/test', state: { entity: props.entity } }]}
    >
      <ContextProvider ctx={ctx}>{props.children}</ContextProvider>
    </MemoryRouter>
  );
};
