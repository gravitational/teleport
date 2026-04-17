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

import { mkdtempDisposable } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

import {
  launchApp,
  login,
  withDefaultAppConfig,
  initializeDataDir,
} from '../helpers/connect';

const bold = (s: string) => `\x1b[1m${s}\x1b[22m`;
const green = (s: string) => `\x1b[32m${s}\x1b[39m`;

function info(msg: string) {
  process.stdout.write(`${green('✓')} ${msg}\n`);
}

await using dataDir = await mkdtempDisposable(
  join(tmpdir(), 'connect-e2e-browse-')
);
info(`created temporary CONNECT_DATA_DIR: ${bold(dataDir.path)}`);
await initializeDataDir(dataDir.path, withDefaultAppConfig({}));

const launched = await launchApp(dataDir.path);
await login(launched.page);
info('Teleport Connect opened and authenticated');
info('close the app window or press Ctrl+C to exit');

await new Promise<void>(resolve => launched.electronApp.once('close', resolve));
info('Teleport Connect closed');
