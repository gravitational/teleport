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

import os from 'node:os';
import path from 'node:path';

import which from 'which';

import Logger from 'teleterm/logger';
import { CUSTOM_SHELL_ID } from 'teleterm/services/config/appConfigSchema';

export interface Shell {
  /**
   * Shell identifier, for example, pwsh.exe or zsh
   * (it doesn't have to be the same as the binary name).
   * Used as an identifier in the app config or in a document.
   * Must be unique.
   * */
  id: typeof CUSTOM_SHELL_ID | string;
  /** Shell executable, for example, C:\\Windows\system32\pwsh.exe, /bin/zsh. */
  binPath: string;
  /** Binary name of the shell executable, for example, pwsh.exe, zsh. */
  binName: string;
  /** Friendly name, for example, Windows PowerShell, zsh. */
  friendlyName: string;
}

export async function getAvailableShells(): Promise<Shell[]> {
  switch (process.platform) {
    case 'linux':
    case 'darwin':
      return getUnixShells();
    case 'win32': {
      return getWindowsShells();
    }
  }
}

export function getDefaultShell(availableShells: Shell[]): string {
  switch (process.platform) {
    case 'linux':
    case 'darwin': {
      // There is always a default shell.
      return availableShells.at(0).id;
    }
    case 'win32':
      if (availableShells.find(shell => shell.id === 'pwsh.exe')) {
        return 'pwsh.exe';
      }
      return 'powershell.exe';
  }
}

async function getUnixShells(): Promise<Shell[]> {
  const logger = new Logger();
  const { shell } = os.userInfo();
  const binName = path.basename(shell);
  if (!shell) {
    const fallbackShell = 'bash';
    logger.error(
      `Failed to read ${process.platform} platform default shell, using fallback: ${fallbackShell}.\n`
    );
    return [
      {
        id: fallbackShell,
        binPath: fallbackShell,
        friendlyName: fallbackShell,
        binName: fallbackShell,
      },
    ];
  }

  return [{ id: binName, binPath: shell, friendlyName: binName, binName }];
}

async function getWindowsShells(): Promise<Shell[]> {
  const shells = await Promise.all(
    [
      {
        binName: 'powershell.exe',
        friendlyName: 'Windows PowerShell (powershell.exe)',
      },
      {
        binName: 'pwsh.exe',
        friendlyName: 'PowerShell (pwsh.exe)',
      },
      {
        binName: 'cmd.exe',
        friendlyName: 'Command Prompt (cmd.exe)',
      },
      {
        binName: 'wsl.exe',
        friendlyName: 'WSL (wsl.exe)',
      },
    ].map(async shell => {
      const binPath = await which(shell.binName, { nothrow: true });
      if (!binPath) {
        return;
      }

      return {
        binPath,
        binName: shell.binName,
        id: shell.binName,
        friendlyName: shell.friendlyName,
      };
    })
  );

  return shells.filter(Boolean);
}

export function makeCustomShellFromPath(shellPath: string): Shell {
  const shellBinName = path.basename(shellPath);
  return {
    id: CUSTOM_SHELL_ID,
    binPath: shellPath,
    binName: shellBinName,
    friendlyName: shellBinName,
  };
}
