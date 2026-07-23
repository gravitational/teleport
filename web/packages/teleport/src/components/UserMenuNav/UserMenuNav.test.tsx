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

import {
  fireEvent,
  screen,
  render as testingRender,
} from 'design/utils/testing';

import cfg from 'teleport/config';
import { getOSSFeatures } from 'teleport/features';
import { FeaturesContextProvider } from 'teleport/FeaturesContext';
import { makeUserContext } from 'teleport/services/user';
import TeleportContext from 'teleport/teleportContext';
import TeleportContextProvider from 'teleport/TeleportContextProvider';
import { makeTestUserContext } from 'teleport/User/testHelpers/makeTestUserContext';
import { mockUserContextProviderWith } from 'teleport/User/testHelpers/mockUserContextWith';

import { UserMenuNav } from './UserMenuNav';

beforeEach(() => mockUserContextProviderWith(makeTestUserContext()));

describe('navigation items rendering', () => {
  test.each`
    path                  | menuName
    ${cfg.routes.account} | ${'Account Settings'}
    ${cfg.routes.support} | ${'Help & Support'}
  `(
    'there is an element `$menuName` that links to `$path`',
    async ({ path, menuName }) => {
      render(path);

      fireEvent.click(screen.getByRole('button', { name: 'User Menu' }));

      const targetEl = screen.getByText(menuName);

      expect(targetEl).toBeInTheDocument();
      expect(targetEl).toHaveAttribute('href', path);
    }
  );
});

describe('user identity rendering', () => {
  test('shows display primary over username and suppresses secondary', () => {
    render('/', {
      userName: '123456',
      displayPrimary: 'Jane Garcia',
      displaySecondary: 'jane@example.com',
    });

    expect(screen.getByText('Jane Garcia')).toBeInTheDocument();
    expect(screen.getByText('123456')).toBeInTheDocument();
    expect(screen.queryByText('jane@example.com')).not.toBeInTheDocument();
    expect(screen.getByText('J')).toBeInTheDocument();
  });

  test('shows username over secondary when primary is absent', () => {
    render('/', {
      userName: 'casey',
      displaySecondary: 'casey@example.com',
    });

    expect(screen.getByText('casey')).toBeInTheDocument();
    expect(screen.getByText('casey@example.com')).toBeInTheDocument();
    expect(screen.getByText('C')).toBeInTheDocument();
  });

  test('shows username only when display values are absent', () => {
    render('/', { userName: 'llama' });

    expect(screen.getAllByText('llama')).toHaveLength(1);
    expect(screen.getByText('L')).toBeInTheDocument();
  });
});

type UserContextOverrides = Partial<{
  userName: string;
  displayPrimary: string;
  displaySecondary: string;
}>;

function render(path: string, userContext: UserContextOverrides = {}) {
  const ctx = new TeleportContext();

  ctx.storeUser.state = makeUserContext({
    userName: 'llama',
    displayPrimary: '',
    displaySecondary: '',
    ...userContext,
    cluster: {
      name: 'test-cluster',
      lastConnected: Date.now(),
    },
  });

  testingRender(
    <MemoryRouter initialEntries={[path]}>
      <TeleportContextProvider ctx={ctx}>
        <FeaturesContextProvider value={getOSSFeatures()}>
          <UserMenuNav />
        </FeaturesContextProvider>
      </TeleportContextProvider>
    </MemoryRouter>
  );
}
