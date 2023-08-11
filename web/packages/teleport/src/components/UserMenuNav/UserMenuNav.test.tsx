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

import { setupServer } from 'msw/node';
import { rest } from 'msw';

import {
  render as testingRender,
  screen,
  fireEvent,
} from 'design/utils/testing';

import cfg from 'teleport/config';

import { FeaturesContextProvider } from 'teleport/FeaturesContext';
import { getOSSFeatures } from 'teleport/features';

import TeleportContextProvider from 'teleport/TeleportContextProvider';
import TeleportContext from 'teleport/teleportContext';

import { makeUserContext } from 'teleport/services/user';

import { ThemePreference } from 'teleport/services/userPreferences/types';

import { mockUserContextProviderWith } from 'teleport/User/testHelpers/mockUserContextWith';
import { makeTestUserContext } from 'teleport/User/testHelpers/makeTestUserContext';

import { UserMenuNav } from './UserMenuNav';

const server = setupServer(
  rest.get(cfg.api.userPreferencesPath, (req, res, ctx) => {
    return res(
      ctx.json({
        theme: ThemePreference.Light,
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
