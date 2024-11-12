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

/**
 MIT License

 Copyright (c) 2015 - present Microsoft Corporation

 Permission is hereby granted, free of charge, to any person obtaining a copy
 of this software and associated documentation files (the "Software"), to deal
 in the Software without restriction, including without limitation the rights
 to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 copies of the Software, and to permit persons to whom the Software is
 furnished to do so, subject to the following conditions:

 The above copyright notice and this permission notice shall be included in all
 copies or substantial portions of the Software.

 THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 SOFTWARE.
 */

// Based on https://github.com/microsoft/vscode/blob/1.66.0/src/vs/platform/shell/node/shellEnv.ts

import { spawn } from 'child_process';

import { memoize } from 'shared/utils/highbar';

import Logger from 'teleterm/logger';
import { unique } from 'teleterm/ui/utils/uid';

const logger = new Logger('resolveShellEnv()');
const resolveShellMaxTime = 10_000; // 10s

export const resolveShellEnvCached = memoize(resolveShellEnv);

export class ResolveShellEnvTimeoutError extends Error {}

async function resolveShellEnv(
  shell: string
): Promise<typeof process.env | undefined> {
  if (process.platform === 'win32') {
    logger.info('skipped Windows platform');
    return;
  }
  // TODO(grzegorz) skip if already running from CLI

  const timeoutController = new AbortController();
  const timeout = setTimeout(() => {
    timeoutController.abort();
  }, resolveShellMaxTime);

  try {
    return await resolveUnixShellEnv(shell, timeoutController.signal);
  } finally {
    clearTimeout(timeout);
  }
}

async function resolveUnixShellEnv(
  shell: string,
  abortSignal: AbortSignal
): Promise<typeof process.env> {
  const runAsNode = process.env['ELECTRON_RUN_AS_NODE'];
  const noAttach = process.env['ELECTRON_NO_ATTACH_CONSOLE'];

  const mark = unique().replace(/-/g, '').substring(0, 12);
  const regex = new RegExp(mark + '(.*)' + mark);

  const env = {
    ...process.env,
    ELECTRON_RUN_AS_NODE: '1',
    ELECTRON_NO_ATTACH_CONSOLE: '1',
  };

  return new Promise<typeof process.env>((resolve, reject) => {
    const command = `'${process.execPath}' -p '"${mark}" + JSON.stringify(process.env) + "${mark}"'`;
    // When bash is run with -c, it is considered a non-interactive shell, and it does not read ~/.bashrc, unless is -i specified.
    // https://unix.stackexchange.com/questions/277312/is-the-shell-created-by-bash-i-c-command-interactive
    const shellArgs =
      shell === '/bin/tcsh' || shell === '/bin/csh' ? ['-ic'] : ['-ilc'];

    logger.info(`Reading shell ${shell} ${shellArgs} ${command}`);

    const child = spawn(shell, [...shellArgs, command], {
      detached: true,
      stdio: ['ignore', 'pipe', 'pipe'],
      env,
    });

    abortSignal.onabort = () => {
      child.kill();
      logger.warn('Reading shell env timed out');
      reject(new ResolveShellEnvTimeoutError());
    };

    child.on('error', err => {
      reject(err);
    });

    const buffers: Buffer[] = [];
    child.stdout.on('data', b => buffers.push(b));

    const stderr: Buffer[] = [];
    child.stderr.on('data', b => stderr.push(b));

    child.on('close', (code, signal) => {
      const raw = Buffer.concat(buffers).toString('utf8');
      const stderrStr = Buffer.concat(stderr).toString('utf8');

      if (code || signal) {
        logger.warn(
          `Unexpected exit code from spawned shell (code ${code}, signal ${signal})`,
          stderrStr
        );
        return reject(new Error(stderrStr));
      }

      const match = regex.exec(raw);
      const rawStripped = match ? match[1] : '{}';

      try {
        const env = JSON.parse(rawStripped);

        if (runAsNode) {
          env['ELECTRON_RUN_AS_NODE'] = runAsNode;
        } else {
          delete env['ELECTRON_RUN_AS_NODE'];
        }

        if (noAttach) {
          env['ELECTRON_NO_ATTACH_CONSOLE'] = noAttach;
        } else {
          delete env['ELECTRON_NO_ATTACH_CONSOLE'];
        }

        resolve(env);
      } catch (error) {
        logger.warn('Failed to parse stdout', error);
        reject(error);
      }
    });
  });
}
