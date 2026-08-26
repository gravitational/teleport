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

import * as Icons from 'design/Icon';

import { getOSSFeatures } from 'teleport/features';
import { FeaturesContextProvider } from 'teleport/FeaturesContext';
import { makeUserContext } from 'teleport/services/user';
import TeleportContext from 'teleport/teleportContext';
import TeleportContextProvider from 'teleport/TeleportContextProvider';

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
  iconSize: 24,
  username: 'george',
  logout: () => null,
};
