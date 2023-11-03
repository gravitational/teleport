/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
