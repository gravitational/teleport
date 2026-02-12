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

import fs from 'fs/promises';
import { pathToFileURL } from 'node:url';
import * as path from 'path';

import { app, net, protocol } from 'electron';

import { getErrorMessage } from 'shared/utils/error';

import Logger from 'teleterm/logger';

const logger = new Logger('protocol handler');
const disabledSchemes = [
  'about',
  'content',
  'chrome',
  'cid',
  'data',
  'filesystem',
  'ftp',
  'gopher',
  'javascript',
  'mailto',
  'file',
];
const APP_FILE_SCHEMA = 'app-file';
const CONNECT_AUTHORITY = 'connect-app';

export const DEV_APP_WINDOW_URL = 'http://localhost:8080/';
export const PACKAGED_APP_WINDOW_URL = buildAppFileUri(
  'build/app/renderer/index.html'
);

/**
 * Builds a URI for the custom 'app-file://' scheme.
 * The path will be resolved against `app.getAppPath()`.
 */
export function buildAppFileUri(relativePath: string) {
  const baseUrl = `${APP_FILE_SCHEMA}://${CONNECT_AUTHORITY}`;

  const uri = new URL(relativePath, baseUrl);
  return uri.toString();
}

/** Disables protocols not used by the app. */
function disableUnusedProtocols() {
  disabledSchemes.forEach(scheme => {
    protocol.handle(scheme, () => {
      const message = `Denying request: Invalid scheme (${scheme})`;
      logger.error(message);
      return new Response(message, {
        status: 403,
        statusText: 'Forbidden Protocol',
      });
    });
  });
}

/**
 * Registers the 'http://' protocol handler.
 * Adds cross-origin headers to the document response to enable features requiring
 * cross-origin isolation.
 */
function handleHttpProtocol(): void {
  protocol.handle('http', async request => {
    const response = await net.fetch(request, {
      // Must be true to prevent the handler from calling itself and entering an infinite loop.
      bypassCustomProtocolHandlers: true,
    });
    if (request.url === DEV_APP_WINDOW_URL) {
      setCrossOriginIsolationHeaders(response.headers);
    }
    return response;
  });
}

/**
 * Registers the 'app-file://' protocol handler.
 * Serves application files from the build directory and adds
 * cross-origin header to the document response, enabling features that
 * require cross-origin isolation.
 */
function handleAppFileProtocol(): void {
  const appPath = app.getAppPath();

  protocol.handle(APP_FILE_SCHEMA, async request => {
    const filePath = decodeURIComponent(new URL(request.url).pathname).replace(
      // Remove the leading slash.
      /^\/+/,
      ''
    );
    const target = path.join(appPath, filePath);
    let realPath: string;
    try {
      realPath = await fs.realpath(target);
    } catch (error) {
      logger.error(`Failed to resolve path ${target}'`, error);
      return new Response(`Failed to resolve path: ${getErrorMessage(error)}`, {
        status: 400,
      });
    }

    const relative = path.relative(appPath, realPath);
    if (relative.startsWith('..') || path.isAbsolute(relative)) {
      const message = `Denying request to path '${realPath}' (not in installPath: '${appPath})'`;
      logger.error(message);
      return new Response(message, {
        status: 400,
      });
    }

    // Use net.fetch to serve local files.
    // It automatically determines and sets the correct Content-Type (MIME) header,
    // unlike fs.readFile.
    const response = await net.fetch(pathToFileURL(realPath).toString(), {
      // 'file' protocol was disabled in disableUnusedProtocols.
      // We can bypass it because we performed the path traversal checks.
      bypassCustomProtocolHandlers: true,
    });
    if (request.url === PACKAGED_APP_WINDOW_URL) {
      setCrossOriginIsolationHeaders(response.headers);
    }
    return response;
  });
}

/**
 * To use features like SharedArrayBuffer, the document must be in a secure context
 * and cross-origin isolated.
 */
function setCrossOriginIsolationHeaders(headers: Headers): void {
  headers.set('Cross-Origin-Opener-Policy', 'same-origin');
  headers.set('Cross-Origin-Embedder-Policy', 'require-corp');
}

/**
 * Registers the 'app-file://' protocol schema.
 * It's used to serve application files.
 */
export function registerAppFileProtocol(): void {
  protocol.registerSchemesAsPrivileged([
    {
      scheme: APP_FILE_SCHEMA,
      privileges: {
        standard: true,
        secure: true,
        supportFetchAPI: true,
      },
    },
  ]);
}

/**
 * Configures protocol handling:
 * - Disables unused web protocols.
 * - Registers handlers for `app-file://` and `http://` (for Vite dev server)
 * to enforce cross-origin isolation, enabling features like `SharedArrayBuffer`.
 */
export function setUpProtocolHandlers(dev: boolean): void {
  disableUnusedProtocols();
  handleAppFileProtocol();
  if (dev) {
    handleHttpProtocol();
  }
}
