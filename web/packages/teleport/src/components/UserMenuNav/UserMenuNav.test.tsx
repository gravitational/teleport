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

import { setupServer } from 'msw/node';
import { rest } from 'msw';

import {
  render as testingRender,
  screen,
  fireEvent,
} from 'design/utils/testing';

import { Theme } from 'gen-proto-ts/teleport/userpreferences/v1/theme_pb';

import cfg from 'teleport/config';

import { FeaturesContextProvider } from 'teleport/FeaturesContext';
import { getOSSFeatures } from 'teleport/features';

import TeleportContextProvider from 'teleport/TeleportContextProvider';
import TeleportContext from 'teleport/teleportContext';

import { makeUserContext } from 'teleport/services/user';

import { mockUserContextProviderWith } from 'teleport/User/testHelpers/mockUserContextWith';
import { makeTestUserContext } from 'teleport/User/testHelpers/makeTestUserContext';

import { UserMenuNav } from './UserMenuNav';

const server = setupServer(
  rest.get(cfg.api.userPreferencesPath, (req, res, ctx) => {
    return res(
      ctx.json({
        theme: Theme.LIGHT,
        assist: {},
      })
    );
  })
);

beforeAll(() => server.listen());

beforeEach(() => mockUserContextProviderWith(makeTestUserContext()));

afterEach(() => server.resetHandlers());

afterAll(() => server.close());

describe('navigation items rendering', () => {
  test.each`
    path                  | menuName
    ${cfg.routes.account} | ${'Account Settings'}
    ${cfg.routes.support} | ${'Help & Support'}
  `(
    'there is an element `$menuName` that links to `$path`',
    async ({ path, menuName }) => {
      render(path);

      // Click on dropdown menu.
      fireEvent.click(await screen.findByText(/llama/i));

      // Only one checkmark should be rendered at a time.
      const targetEl = screen.getByText(menuName);

      expect(targetEl).toBeInTheDocument();
      expect(targetEl).toHaveAttribute('href', path);
    }
  );
});

function render(path: string) {
  const ctx = new TeleportContext();

  ctx.storeUser.state = makeUserContext({
    cluster: {
      name: 'test-cluster',
      lastConnected: Date.now(),
    },
  });

  testingRender(
    <MemoryRouter initialEntries={[path]}>
      <TeleportContextProvider ctx={ctx}>
        <FeaturesContextProvider value={getOSSFeatures()}>
          <UserMenuNav username="llama" />
        </FeaturesContextProvider>
      </TeleportContextProvider>
    </MemoryRouter>
  );
}
