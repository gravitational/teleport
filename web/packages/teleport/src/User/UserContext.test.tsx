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

import { setupServer } from 'msw/node';
import { rest } from 'msw';

import { render, screen, waitFor } from '@testing-library/react';

import '@testing-library/jest-dom';

import cfg from 'teleport/config';

import { UserContextProvider } from 'teleport/User';

import { ThemePreference } from 'teleport/services/userPreferences/types';
import { useUser } from 'teleport/User/UserContext';
import { KeysEnum } from 'teleport/services/localStorage';

function ThemeName() {
  const { preferences } = useUser();

  return (
    <div>
      theme: {preferences.theme === ThemePreference.Light ? 'light' : 'dark'}
    </div>
  );
}

describe('user context - success state', () => {
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
  beforeEach(() => localStorage.clear());
  afterEach(() => server.resetHandlers());
  afterAll(() => server.close());

  it('should render with the settings from the backend', async () => {
    render(
      <UserContextProvider>
        <ThemeName />
      </UserContextProvider>
    );

    const theme = await screen.findByText(/theme: light/i);

    expect(theme).toBeInTheDocument();
  });

  it('should migrate the previous theme setting from local storage', async () => {
    let updateBody = {};

    server.use(
      rest.put(cfg.api.userPreferencesPath, async (req, res, ctx) => {
        updateBody = await req.json();

        return res(ctx.status(200), ctx.json({}));
      })
    );

    localStorage.setItem(KeysEnum.THEME, 'dark');

    render(
      <UserContextProvider>
        <ThemeName />
      </UserContextProvider>
    );

    await waitFor(() =>
      expect(updateBody).toEqual({
        theme: ThemePreference.Dark,
      })
    );

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
      <UserContextProvider>
        <ThemeName />
      </UserContextProvider>
    );

    const theme = await screen.findByText(/theme: light/i);

    expect(theme).toBeInTheDocument();
  });

  it('should render with the theme from the previous local storage setting', async () => {
    localStorage.setItem(KeysEnum.THEME, 'dark');

    render(
      <UserContextProvider>
        <ThemeName />
      </UserContextProvider>
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
      <UserContextProvider>
        <ThemeName />
      </UserContextProvider>
    );

    const theme = await screen.findByText(/theme: dark/i);

    expect(theme).toBeInTheDocument();
  });
});
