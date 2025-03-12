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
import { fileURLToPath } from 'node:url';
import * as path from 'path';

import { app, protocol } from 'electron';

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
];

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

export function enableWebHandlersProtection() {
  interceptFileProtocol();
  disableUnusedProtocols();
}
