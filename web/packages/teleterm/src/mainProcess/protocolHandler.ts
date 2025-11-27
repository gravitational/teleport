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

import fs from 'fs';
import { fileURLToPath, pathToFileURL } from 'node:url';
import * as path from 'path';

import { app, net, protocol } from 'electron';

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

/**
 * Builds a URI for the custom 'app-file://' scheme.
 * The path will be resolved against `app.getAppPath()`.
 */
export function buildAppFileUri(relativePath: string) {
  const baseUrl = `${APP_FILE_SCHEMA}://${CONNECT_AUTHORITY}`;

  const uri = new URL(relativePath, baseUrl);
  return uri.toString();
}

// these protocols are not used within the app
function disableUnusedProtocols() {
  disabledSchemes.forEach(scheme => {
    protocol.interceptFileProtocol(scheme, (_request, callback) => {
      logger.error(`Denying request: Invalid scheme (${scheme})`);
      callback({ error: -3 });
    });
  });
}

// intercept, clean, and validate the requested file path.
function interceptFileProtocol() {
  const installPath = app.getAppPath();

  protocol.interceptFileProtocol('file', (request, callback) => {
    const target = path.normalize(fileURLToPath(request.url));
    const realPath = fs.existsSync(target) ? fs.realpathSync(target) : target;

    if (!path.isAbsolute(realPath)) {
      logger.error(`Denying request to non-absolute path '${realPath}'`);
      return callback({ error: -3 });
    }

    if (!realPath.startsWith(installPath)) {
      logger.error(
        `Denying request to path '${realPath}' (not in installPath: '${installPath})'`
      );
      return callback({ error: -3 });
    }

    return callback({
      path: realPath,
    });
  });
}

/**
 * Registers the 'app-file://' protocol handler.
 * Serves application files from the build directory and adds
 * cross-origin headers to responses, enabling features that
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

    // Use net.fetch to serve local files.
    // It automatically determines and sets the correct Content-Type (MIME) header,
    // unlike fs.readFile.
    const response = await net.fetch(pathToFileURL(target).toString());
    // To use features like SharedArrayBuffer, the document must be in a secure context
    // and cross-origin isolated.
    response.headers.set('Cross-Origin-Opener-Policy', 'same-origin');
    response.headers.set('Cross-Origin-Embedder-Policy', 'require-corp');
    return response;
  });
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
        codeCache: true,
      },
    },
  ]);
}

export function enableWebHandlersProtection() {
  interceptFileProtocol();
  disableUnusedProtocols();
  handleAppFileProtocol();
}
