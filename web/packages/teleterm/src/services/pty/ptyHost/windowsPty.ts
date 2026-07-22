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

import { RuntimeSettings } from 'teleterm/mainProcess/types';

import { TerminalOptions, WindowsPty } from '../types';

export const WIN_BUILD_STABLE_CONPTY = 18309;

export function getWindowsPty(
  runtimeSettings: RuntimeSettings,
  terminalOptions: TerminalOptions
): WindowsPty {
  if (runtimeSettings.platform !== 'win32') {
    return undefined;
  }

  const buildNumber = getWindowsBuildNumber(runtimeSettings.osVersion);
  const useConpty =
    terminalOptions.windowsBackend === 'auto' &&
    buildNumber >= WIN_BUILD_STABLE_CONPTY;
  return {
    useConpty,
    buildNumber,
  };
}

function getWindowsBuildNumber(osVersion: string): number {
  const parsedOsVersion = /(\d+)\.(\d+)\.(\d+)/g.exec(osVersion);
  if (parsedOsVersion?.length === 4) {
    return parseInt(parsedOsVersion[3]);
  }
  return 0;
}
