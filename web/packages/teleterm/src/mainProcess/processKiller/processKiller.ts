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

import { ChildProcess } from 'node:child_process';
import { setTimeout } from 'node:timers/promises';

import Logger from 'teleterm/logger';

const logger = new Logger('processKiller');

/**
 * Tries to kill a process in a graceful way - by sending a SIGTERM signal, or using
 * {@link gracefullyKill} function if provided.
 * If the process doesn't close within the specified {@link timeout}, a SIGKILL signal is sent.
 */
export async function killProcess(
  process: ChildProcess,
  timeout = 5_000,
  gracefullyKill: (process: ChildProcess) => void = process =>
    process.kill('SIGTERM')
): Promise<void> {
  if (!isProcessRunning(process)) {
    logger.info(`Process is not running. Nothing to kill.`);
    return;
  }

  const processClose = promisifyProcessClose(process);

  async function startKillingSequence(): Promise<void> {
    gracefullyKill(process);

    await setTimeout(timeout);

    if (isProcessRunning(process)) {
      const timeoutInSeconds = timeout / 1_000;
      logger.error(
        `Process ${process.spawnfile} did not close within ${timeoutInSeconds} seconds. Sending SIGKILL.`
      );
      process.kill('SIGKILL');
    }
  }

  startKillingSequence();

  await processClose;
}

function promisifyProcessClose(childProcess: ChildProcess): Promise<void> {
  return new Promise(resolve => childProcess.once('close', resolve));
}

function isProcessRunning(process: ChildProcess): boolean {
  return process.exitCode === null && process.signalCode === null;
}
