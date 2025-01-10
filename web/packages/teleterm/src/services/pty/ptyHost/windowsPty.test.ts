/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { makeRuntimeSettings } from 'teleterm/mainProcess/fixtures/mocks';

import { getWindowsPty } from './windowsPty';

test.each([
  {
    name: 'uses conpty on supported Windows version',
    platform: 'win32' as const,
    osVersion: '10.0.22621',
    terminalOptions: { windowsBackend: 'auto' as const },
    expected: { useConpty: true, buildNumber: 22621 },
  },
  {
    name: 'uses winpty on unsupported Windows version',
    platform: 'win32' as const,
    osVersion: '10.0.18308',
    terminalOptions: { windowsBackend: 'auto' as const },
    expected: { useConpty: false, buildNumber: 18308 },
  },
  {
    name: 'uses winpty when Windows version is supported, but conpty is disabled in options',
    platform: 'win32' as const,
    osVersion: '10.0.22621',
    terminalOptions: { windowsBackend: 'winpty' as const },
    expected: { useConpty: false, buildNumber: 22621 },
  },
  {
    name: 'undefined on non-Windows OS',
    platform: 'darwin' as const,
    osVersion: '23.5.0',
    terminalOptions: { windowsBackend: 'auto' as const },
    expected: undefined,
  },
])('$name', ({ platform, osVersion, terminalOptions, expected }) => {
  const pty = getWindowsPty(
    makeRuntimeSettings({
      platform,
      osVersion,
    }),
    terminalOptions
  );
  expect(pty).toEqual(expected);
});
