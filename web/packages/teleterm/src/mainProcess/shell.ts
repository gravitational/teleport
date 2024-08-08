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

import fs from 'node:fs';
import os from 'node:os';

import which from 'which';

import Logger from 'teleterm/logger';

export interface Shell {
  /**
   * Shell identifier, for example, pwsh.exe or zsh
   * (it doesn't have to be the same as the binary name).
   * Used as an identifier in the app config or in a document.
   * Must be unique.
   * */
  id: string;
  /** Friendly name, for example, Windows PowerShell, zsh. */
  friendlyName: string;
  /** Shell executable, for example, C:\\Windows\system32\pwsh.exe, /bin/zsh. */
  binPath: string;
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
  const logger = new Logger();
  switch (process.platform) {
    case 'linux':
    case 'darwin': {
      const fallbackShell = 'bash';
      const { shell } = os.userInfo();
      const shellId = availableShells.find(
        availableShell => availableShell.binPath === shell
      )?.id;

      if (!shellId) {
        logger.error(
          `Failed to read ${process.platform} platform default shell, using fallback: ${fallbackShell}.\n`
        );

        return fallbackShell;
      }

      return shellId;
    }
    case 'win32':
      if (availableShells.find(shell => shell.id === 'pwsh.exe')) {
        return 'pwsh.exe';
      }
      return 'powershell.exe';
  }
}

async function getUnixShells(): Promise<Shell[]> {
  const shells = await fs.promises.readFile('/etc/shells', {
    encoding: 'utf-8',
  });
  return shells
    .split(os.EOL)
    .map(line => line.trim())
    .filter(line => line && !line.startsWith('#'))
    .map(binPath => {
      const name = binPath.split('/').at(-1);
      return {
        id: name,
        friendlyName: name,
        binPath: binPath,
      };
    });
}

async function getWindowsShells(): Promise<Shell[]> {
  const shells = await Promise.all(
    [
      {
        id: 'powershell.exe',
        friendlyName: 'Windows PowerShell (powershell.exe)',
      },
      {
        id: 'pwsh.exe',
        friendlyName: 'PowerShell (pwsh.exe)',
      },
      {
        id: 'cmd.exe',
        friendlyName: 'Command Prompt (cmd.exe)',
      },
      {
        id: 'wsl.exe',
        friendlyName: 'WSL (wsl.exe)',
      },
    ].map(async shell => {
      const binPath = await which(shell.id, { nothrow: true });
      if (!binPath) {
        return;
      }

      return {
        binPath,
        id: shell.id,
        friendlyName: shell.friendlyName,
      };
    })
  );

  return shells.filter(Boolean);
}
