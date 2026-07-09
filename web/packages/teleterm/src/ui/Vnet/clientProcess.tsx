/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import { useEffect, useState } from 'react';

import { useAppContext } from 'teleterm/ui/appContextProvider';

/**
 * appIconCache memoizes resolved icon data URLs by executable path across the
 * lifetime of the renderer. Icons don't change while the app runs, and the
 * connection lists re-render on every stream update, so caching avoids
 * repeatedly crossing the IPC boundary for the same program. The empty string
 * (no icon available) is cached too, so unresolved paths aren't retried.
 */
const appIconCache = new Map<string, Promise<string>>();

/**
 * useAppIcon resolves the icon of the local program at the given executable
 * path to a data URL through the main process. It returns an empty string until
 * the icon loads, or permanently if the platform or path yields no icon; the
 * caller renders the icon only when the string is non-empty.
 */
export function useAppIcon(path: string | undefined): string {
  const { mainProcessClient } = useAppContext();
  const [icon, setIcon] = useState('');

  useEffect(() => {
    if (!path) {
      setIcon('');
      return;
    }

    let cached = appIconCache.get(path);
    if (!cached) {
      cached = mainProcessClient.getAppIcon(path);
      appIconCache.set(path, cached);
    }

    let canceled = false;
    cached.then(
      dataUrl => {
        if (!canceled) {
          setIcon(dataUrl);
        }
      },
      () => {
        // getAppIcon already logs failures in the main process and resolves to
        // an empty string, so a rejection here is unexpected; drop the cached
        // entry so a later render can retry.
        appIconCache.delete(path);
      }
    );
    return () => {
      canceled = true;
    };
  }, [path, mainProcessClient]);

  return icon;
}

/**
 * processDisplayName returns a human-friendly name for a local process
 * executable path, used to show which program opened a connection.
 *
 * For macOS app bundles it uses the name of the outermost .app bundle, e.g.
 * "Google Chrome" for a deeply nested helper executable
 * (".../Google Chrome.app/.../Google Chrome Helper.app/Contents/MacOS/..."),
 * rather than the executable's own basename ("Google Chrome Helper"). This
 * mirrors the icon lookup in the main process' getAppIcon handler. Otherwise it
 * uses the executable's basename with its first letter capitalized, e.g. "Curl"
 * for "/usr/bin/curl".
 */
export function processDisplayName(path: string): string {
  if (!path) {
    return '';
  }
  const appBundle = path.match(/^(?:.*?\/)?([^/]+)\.app(?:\/|$)/);
  if (appBundle) {
    return appBundle[1];
  }
  const segments = path.split('/');
  const segment = segments[segments.length - 1] || path;
  return segment[0].toUpperCase() + segment.substring(1);
}
