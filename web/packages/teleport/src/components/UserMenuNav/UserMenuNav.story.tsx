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

import { getOSSFeatures } from 'teleport/features';
import { FeaturesContextProvider } from 'teleport/FeaturesContext';
import { makeUserContext } from 'teleport/services/user';
import TeleportContext from 'teleport/teleportContext';
import TeleportContextProvider from 'teleport/TeleportContextProvider';

import { UserMenuNav } from './UserMenuNav';

export default {
  title: 'Teleport/UserMenuNav',
};

export function UsernameOnly() {
  return renderUserMenuNav({
    userName: 'george',
    displayPrimary: '',
    displaySecondary: '',
  });
}

export function DisplayName() {
  return renderUserMenuNav({
    userName: '123456',
    displayPrimary: 'Jane Garcia',
    displaySecondary: 'jane@example.com',
  });
}

export function SecondaryOnly() {
  return renderUserMenuNav({
    userName: 'casey',
    displayPrimary: '',
    displaySecondary: 'casey@example.com',
  });
}

export function LongName() {
  return renderUserMenuNav({
    userName: 'long-canonical-username-used-for-display-testing@example.com',
    displayPrimary:
      'Josephine Alexandra Montgomery-Smith With A Very Long Display Name',
    displaySecondary: '',
  });
}

function renderUserMenuNav({
  userName,
  displayPrimary,
  displaySecondary,
}: {
  userName: string;
  displayPrimary: string;
  displaySecondary: string;
}) {
  const ctx = new TeleportContext();

  ctx.storeUser.state = makeUserContext({
    userName,
    displayPrimary,
    displaySecondary,
    cluster: {
      name: 'test-cluster',
      lastConnected: Date.now(),
    },
  });

  return (
    <MemoryRouter>
      <TeleportContextProvider ctx={ctx}>
        <FeaturesContextProvider value={getOSSFeatures()}>
          <UserMenuNav />
        </FeaturesContextProvider>
      </TeleportContextProvider>
    </MemoryRouter>
  );
}
