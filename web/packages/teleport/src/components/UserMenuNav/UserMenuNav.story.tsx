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
import * as Icons from 'design/Icon';

import TeleportContextProvider from 'teleport/TeleportContextProvider';
import TeleportContext from 'teleport/teleportContext';
import { makeUserContext } from 'teleport/services/user';
import { FeaturesContextProvider } from 'teleport/FeaturesContext';
import { getOSSFeatures } from 'teleport/features';

import { UserMenuNav } from './UserMenuNav';

export default {
  title: 'Teleport/UserMenuNav',
  args: { userContext: true },
};

export function Loaded() {
  const ctx = new TeleportContext();

  ctx.storeUser.state = makeUserContext({
    cluster: {
      name: 'test-cluster',
      lastConnected: Date.now(),
    },
  });

  return (
    <MemoryRouter>
      <TeleportContextProvider ctx={ctx}>
        <FeaturesContextProvider value={getOSSFeatures()}>
          <UserMenuNav {...props} />
        </FeaturesContextProvider>
      </TeleportContextProvider>
    </MemoryRouter>
  );
}

const props = {
  navItems: [
    {
      title: 'Nav Item 1',
      Icon: Icons.Apple,
      getLink: () => 'test',
    },
    {
      title: 'Nav Item 2',
      Icon: Icons.Cloud,
      getLink: () => 'test2',
    },
  ],
  username: 'george',
  logout: () => null,
};
