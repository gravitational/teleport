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

export enum Platform {
  Windows = 'Windows',
  macOS = 'macOS',
  Linux = 'Linux',
}

export enum UserAgent {
  Windows = 'Windows',
  macOS = 'Macintosh',
  Linux = 'Linux',
}

type PlatformType = {
  isWin: boolean;
  isMac: boolean;
  isLinux: boolean;
};

/**
 * @deprecated Use getPlatform instead.
 */
export function getPlatformType(): PlatformType {
  const platform = getPlatform();

  return {
    isWin: platform === Platform.Windows,
    isMac: platform === Platform.macOS,
    isLinux: platform === Platform.Linux,
  };
}

/**
 * getPlatform returns the platform of the user based on the browser user agent or the Node.js
 * binary.
 *
 * getPlatform must work in both environments. It must be defined within the design package and not
 * the shared package to avoid circular dependencies – the design package needs to be able to detect
 * the platform and the shared package depends on the design package.
 */
export function getPlatform(): Platform {
  // Browser environment.
  if (typeof window !== 'undefined') {
    const userAgent = window.navigator.userAgent;

    if (userAgent.includes(UserAgent.Windows)) {
      return Platform.Windows;
    }

    if (userAgent.includes(UserAgent.macOS)) {
      return Platform.macOS;
    }

    // Default to Linux. The assumption is that the other two platforms make it easy to identify
    // them, so if the execution gets to this point, we can just assume that it's neither of those
    // two and thus it must be Linux (or, more broadly speaking, some Unix variant).
    return Platform.Linux;
  }

  // Node.js environment.
  if (typeof process !== 'undefined') {
    switch (process.platform) {
      case 'win32':
        return Platform.Windows;
      case 'darwin':
        return Platform.macOS;
      default:
        return Platform.Linux;
    }
  }

  throw new Error('Expected either window or process to be defined');
}

/**
 * Converts Platform to a string used in Go as runtime.GOOS.
 * https://pkg.go.dev/runtime#GOOS
 */
export function platformToGOOS(platform: Platform) {
  switch (platform) {
    case Platform.Windows:
      return 'windows';
    case Platform.macOS:
      return 'darwin';
    case Platform.Linux:
      return 'linux';
    default:
      assertUnreachable(platform);
  }
}

function assertUnreachable(x: never): never {
  throw new Error(`Unhandled case: ${x}`);
}
