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

import { ChildProcess } from 'node:child_process';
import { setTimeout } from 'node:timers/promises';

import Logger from 'teleterm/logger';

const logger = new Logger('terminateWithTimeout');

/**
 * Tries to kill a process in a graceful way - by sending a SIGTERM signal, or using
 * {@link gracefullyKill} function if provided.
 * If the process doesn't close within the specified {@link timeout}, a SIGKILL signal is sent.
 */
export async function terminateWithTimeout(
  process: ChildProcess,
  timeout = 5_000,
  gracefullyKill: (process: ChildProcess) => void = process =>
    process.kill('SIGTERM')
): Promise<void> {
  if (!isProcessRunning(process)) {
    logger.info(
      `Process ${process.spawnfile} is not running. Nothing to kill.`
    );
    return;
  }

  const controller = new AbortController();
  const processExit = promisifyProcessExit(process);

  gracefullyKill(process);

  // Wait for either exit or timeout.
  const hasExited = await Promise.race([
    processExit.then(() => controller.abort()).then(() => true),
    setTimeout(timeout, false, { signal: controller.signal }),
  ]);

  if (hasExited) {
    return;
  }

  const timeoutInSeconds = timeout / 1_000;
  logger.error(
    `Process ${process.spawnfile} did not exit within ${timeoutInSeconds} seconds. Sending SIGKILL.`
  );
  const killSucceeded = process.kill('SIGKILL');

  if (!killSucceeded) {
    throw new Error(`Sending SIGKILL to ${process.spawnfile} has failed`);
  }

  await processExit;
}

function promisifyProcessExit(childProcess: ChildProcess): Promise<void> {
  return new Promise(resolve => childProcess.once('exit', resolve));
}

function isProcessRunning(process: ChildProcess): boolean {
  return process.exitCode === null && process.signalCode === null;
}
