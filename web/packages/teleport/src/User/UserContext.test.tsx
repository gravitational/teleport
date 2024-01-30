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

import { setupServer } from 'msw/node';
import { rest } from 'msw';
import { MemoryRouter } from 'react-router';

import { render, screen, waitFor } from '@testing-library/react';

import '@testing-library/jest-dom';

import { Theme } from 'gen-proto-ts/teleport/userpreferences/v1/theme_pb';

import cfg from 'teleport/config';

import { UserContextProvider } from 'teleport/User';

import { useUser } from 'teleport/User/UserContext';
import { KeysEnum } from 'teleport/services/storageService';

function ThemeName() {
  const { preferences } = useUser();

  return (
    <div>theme: {preferences.theme === Theme.LIGHT ? 'light' : 'dark'}</div>
  );
}

describe('user context - success state', () => {
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
  beforeEach(() => localStorage.clear());
  afterEach(() => server.resetHandlers());
  afterAll(() => server.close());

  it('should render with the settings from the backend', async () => {
    render(
      <MemoryRouter>
        <UserContextProvider>
          <ThemeName />
        </UserContextProvider>
      </MemoryRouter>
    );

    const theme = await screen.findByText(/theme: light/i);

    expect(theme).toBeInTheDocument();
  });

  it('should migrate the previous theme setting from local storage', async () => {
    let updateBody: { theme?: Theme } = {};

    server.use(
      rest.put(cfg.api.userPreferencesPath, async (req, res, ctx) => {
        updateBody = await req.json();

        return res(ctx.status(200), ctx.json({}));
      })
    );

    localStorage.setItem(KeysEnum.THEME, 'dark');

    render(
      <MemoryRouter>
        <UserContextProvider>
          <ThemeName />
        </UserContextProvider>
      </MemoryRouter>
    );

    await waitFor(() => expect(updateBody.theme).toEqual(Theme.DARK));

    const theme = await screen.findByText(/theme: dark/i);

    expect(theme).toBeInTheDocument();
  });
});

describe('user context - error state', () => {
  const server = setupServer(
    rest.get(cfg.api.userPreferencesPath, (req, res, ctx) => {
      return res(ctx.status(500));
    })
  );

  beforeAll(() => server.listen());
  beforeEach(() => localStorage.clear());
  afterEach(() => server.resetHandlers());
  afterAll(() => server.close());

  it('should render with the default settings', async () => {
    render(
      <MemoryRouter>
        <UserContextProvider>
          <ThemeName />
        </UserContextProvider>
      </MemoryRouter>
    );

    const theme = await screen.findByText(/theme: light/i);

    expect(theme).toBeInTheDocument();
  });

  it('should render with the theme from the previous local storage setting', async () => {
    localStorage.setItem(KeysEnum.THEME, 'dark');

    render(
      <MemoryRouter>
        <UserContextProvider>
          <ThemeName />
        </UserContextProvider>
      </MemoryRouter>
    );

    const theme = await screen.findByText(/theme: dark/i);

    expect(theme).toBeInTheDocument();
  });

  it('should render with the settings from local storage', async () => {
    localStorage.setItem(
      KeysEnum.USER_PREFERENCES,
      JSON.stringify({
        theme: 'dark',
        assist: {},
      })
    );

    render(
      <MemoryRouter>
        <UserContextProvider>
          <ThemeName />
        </UserContextProvider>
      </MemoryRouter>
    );

    const theme = await screen.findByText(/theme: dark/i);

    expect(theme).toBeInTheDocument();
  });
});
