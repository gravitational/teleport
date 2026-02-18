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

import { http, HttpResponse } from 'msw';
import { MemoryRouter } from 'react-router';

import '@testing-library/jest-dom';

import { ThemeProvider } from 'styled-components';

import lightTheme from 'design/theme/themes/lightTheme';
import { enableMswServer, render, screen, server } from 'design/utils/testing';
import { Theme } from 'gen-proto-ts/teleport/userpreferences/v1/theme_pb';

import cfg from 'teleport/config';
import { KeysEnum } from 'teleport/services/storageService';
import { UserContextProvider } from 'teleport/User';
import { useUser } from 'teleport/User/UserContext';

enableMswServer();

function ThemeName() {
  const { preferences } = useUser();

  return (
    <div>theme: {preferences.theme === Theme.LIGHT ? 'light' : 'dark'}</div>
  );
}

describe('user context - success state', () => {
  beforeEach(() => {
    server.use(
      http.get(cfg.api.userPreferencesPath, () => {
        return HttpResponse.json({
          theme: Theme.LIGHT,
        });
      })
    );
    localStorage.clear();
  });

  it('should render with the settings from the backend', async () => {
    render(
      <ThemeProvider theme={lightTheme}>
        <MemoryRouter>
          <UserContextProvider>
            <ThemeName />
          </UserContextProvider>
        </MemoryRouter>
      </ThemeProvider>
    );

    const theme = await screen.findByText(/theme: light/i);

    expect(theme).toBeInTheDocument();
  });
});

describe('user context - error state', () => {
  beforeEach(() => {
    server.use(
      http.get(cfg.api.userPreferencesPath, () => {
        return HttpResponse.json(null, { status: 500 });
      })
    );
    localStorage.clear();
  });

  it('should render with the default settings', async () => {
    render(
      <ThemeProvider theme={lightTheme}>
        <MemoryRouter>
          <UserContextProvider>
            <ThemeName />
          </UserContextProvider>
        </MemoryRouter>
      </ThemeProvider>
    );

    const theme = await screen.findByText(/theme: light/i);

    expect(theme).toBeInTheDocument();
  });

  it('should render with the settings from local storage', async () => {
    localStorage.setItem(
      KeysEnum.USER_PREFERENCES,
      JSON.stringify({
        theme: 'dark',
      })
    );

    render(
      <ThemeProvider theme={lightTheme}>
        <MemoryRouter>
          <UserContextProvider>
            <ThemeName />
          </UserContextProvider>
        </MemoryRouter>
      </ThemeProvider>
    );

    const theme = await screen.findByText(/theme: dark/i);

    expect(theme).toBeInTheDocument();
  });
});
