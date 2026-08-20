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
