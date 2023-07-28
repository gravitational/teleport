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

import { fork } from 'node:child_process';
import path from 'node:path';

import Logger, { NullService } from 'teleterm/logger';

import { killProcess } from './processKiller';

beforeAll(() => {
  Logger.init(new NullService());
});

test('kills a process gracefully when possible', async () => {
  const process = fork(path.join(__dirname, 'testProcess.mjs'), {
    silent: true,
  });

  await killProcess(process);

  expect(process.killed).toBeTruthy();
  expect(process.signalCode).toBe('SIGTERM');
});

test('kills a process using SIGKILL when a graceful kill did not work', async () => {
  const process = fork(
    path.join(__dirname, 'testProcess.mjs'),
    ['ignore-sigterm'],
    {
      silent: true,
    }
  );

  // wait for the process to start and register callbacks
  await new Promise(resolve => process.stdout.once('data', resolve));

  await killProcess(process, 1_000);

  expect(process.killed).toBeTruthy();
  expect(process.signalCode).toBe('SIGKILL');
});

test('killing a process that has already stopped is noop', async () => {
  const process = fork(path.join(__dirname, 'testProcess-nonExisting.mjs'), {
    silent: true,
  });

  // wait for the process
  await new Promise(resolve => process.once('close', resolve));
  await killProcess(process, 1_000);

  expect(process.exitCode).toBe(1);
  expect(process.signalCode).toBeNull();
});
