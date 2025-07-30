/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
import * as styledComponents from 'styled-components';

import { lightTheme } from 'design/theme';
import { fireEvent, render, screen, waitFor } from 'design/utils/testing';
import { Theme } from 'gen-proto-ts/teleport/userpreferences/v1/theme_pb';

import { createTeleportContext } from 'teleport/mocks/contexts';
import TeleportContext from 'teleport/teleportContext';
import { makeTestUserContext } from 'teleport/User/testHelpers/makeTestUserContext';
import { mockUserContextProviderWith } from 'teleport/User/testHelpers/mockUserContextWith';

import { ContextProvider } from '..';
import { NotificationProvider, useNotification } from './NotificationContext';
import { Preferences } from './Preferences';

jest.mock('./NotificationContext', () => {
  const originalContext = jest.requireActual('./NotificationContext');
  return {
    ...originalContext,
    useNotification: jest.fn(),
  };
});

function renderComponent(ctx: TeleportContext, setErrorMessageFn = jest.fn()) {
  render(
    <ContextProvider ctx={ctx}>
      <NotificationProvider>
        <Preferences setErrorMessage={setErrorMessageFn} />
      </NotificationProvider>
    </ContextProvider>
  );
}

describe('Account/Preferences', () => {
  it('a user can change their theme', async () => {
    const userContext = makeTestUserContext();
    userContext.updatePreferences = jest
      .fn()
      .mockImplementation(async prefs => {
        if (prefs.theme !== undefined) {
          userContext.preferences.theme = prefs.theme;
        }
        return { success: true };
      });

    (useNotification as jest.Mock).mockReturnValue({
      addNotification: jest.fn(),
    });

    mockUserContextProviderWith(userContext);
    renderComponent(createTeleportContext());

    expect(userContext.preferences.theme).toBe(Theme.LIGHT);

    fireEvent.click(screen.getByRole('radio', { name: /dark/i }));

    await waitFor(() => {
      expect(userContext.preferences.theme).toBe(Theme.DARK);
    });
  });

  it('shows an error message when a theme update fails', async () => {
    const userContext = makeTestUserContext();
    userContext.updatePreferences = jest
      .fn()
      .mockRejectedValue(new Error('error'));
    mockUserContextProviderWith(userContext);

    const setErrorMessageFn = jest.fn();
    renderComponent(createTeleportContext(), setErrorMessageFn);

    fireEvent.click(screen.getByRole('radio', { name: /dark/i }));

    await waitFor(() => {
      expect(setErrorMessageFn).toHaveBeenCalledWith(
        'Failed to update the keyboard layout: error'
      );
    });
  });

  it('a user can change their keyboard layout', async () => {
    const userContext = makeTestUserContext();
    userContext.updatePreferences = jest
      .fn()
      .mockImplementation(async prefs => {
        if (prefs.keyboardLayout !== undefined) {
          userContext.preferences.keyboardLayout = prefs.keyboardLayout;
        }
        return { success: true };
      });
    mockUserContextProviderWith(userContext);

    const addNotification = jest.fn();
    (useNotification as jest.Mock).mockReturnValue({
      addNotification,
    });

    renderComponent(createTeleportContext());

    expect(userContext.preferences.keyboardLayout).toBe(0);

    const select = screen.getByLabelText('keyboard layout select');
    fireEvent.mouseDown(select);

    let ukOption;
    await waitFor(() => {
      ukOption = screen.getByText('United Kingdom', { exact: true });
    });
    expect(ukOption).toBeInTheDocument();
    fireEvent.click(ukOption);

    await waitFor(() => {
      expect(userContext.updatePreferences).toHaveBeenCalledWith(
        expect.objectContaining({ keyboardLayout: 0x00000809 })
      );
    });

    expect(addNotification).toHaveBeenCalledWith('success', {
      title: 'Change saved',
      isAutoRemovable: true,
    });
  });

  it("theme selection isn't shown if a custom theme is set", () => {
    jest.spyOn(styledComponents, 'useTheme').mockReturnValue({
      ...lightTheme,
      isCustomTheme: true,
    });

    const userContext = makeTestUserContext();
    mockUserContextProviderWith(userContext);

    renderComponent(createTeleportContext());

    expect(screen.queryByText(/theme/i)).not.toBeInTheDocument();

    jest.restoreAllMocks();
  });
});
