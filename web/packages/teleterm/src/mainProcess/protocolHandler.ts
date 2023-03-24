/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { fileURLToPath } from 'node:url';
import * as path from 'path';
import fs from 'fs';

import { protocol, app } from 'electron';

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
