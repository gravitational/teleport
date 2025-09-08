/**
 * @jest-environment node
 */
/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { BrowserWindow } from 'electron';

import { createMockConfigService } from 'teleterm/services/config/fixtures/mocks';
import { createMockFileStorage } from 'teleterm/services/fileStorage/fixtures/mocks';

import { makeRuntimeSettings } from './fixtures/mocks';
import { WindowsManager } from './windowsManager';

jest.mock('electron', () => ({
  Menu: {
    buildFromTemplate: jest.fn(),
  },
  ipcMain: {
    once: jest.fn(),
  },
}));

describe('waitForWindowFocus', () => {
  it('waits for the window to receive focus', async () => {
    const { windowsManager } = makeWindowsManager();

    const promise = windowsManager.waitForWindowFocus();

    windowsManager.focusWindow();

    await expect(promise).resolves.toBeUndefined();
  });

  it('returns early if signal is aborted', async () => {
    const { windowsManager, mockWindow } = makeWindowsManager();

    const abortController = new AbortController();
    abortController.abort();

    await expect(
      windowsManager.waitForWindowFocus(abortController.signal)
    ).resolves.toBeUndefined();

    expect(mockWindow.isFocused).not.toHaveBeenCalled();
  });

  it('resolves after a timeout', async () => {
    const { windowsManager } = makeWindowsManager();

    // The premise behind this test is that we never trigger window focus and expect the method to
    // automatically time out. If it doesn't do that, then the test itself will fail due to a timeout.
    await expect(
      windowsManager.waitForWindowFocus(undefined, 25)
    ).resolves.toBeUndefined();
  });
});

const makeWindowsManager = () => {
  const windowsManager = new WindowsManager(
    createMockFileStorage(),
    makeRuntimeSettings(),
    createMockConfigService({})
  );

  let isFocused = false;

  const mockWindow = {
    focus: jest.fn().mockImplementation(() => {
      isFocused = true;
    }),
    isFocused: jest.fn().mockImplementation(() => isFocused),
    isMinimized: jest.fn().mockReturnValue(false),
    isVisible: jest.fn().mockReturnValue(true),
    isDestroyed: jest.fn().mockReturnValue(false),
  } as Partial<BrowserWindow>;

  windowsManager['window'] = mockWindow as BrowserWindow;

  return { windowsManager, mockWindow };
};
