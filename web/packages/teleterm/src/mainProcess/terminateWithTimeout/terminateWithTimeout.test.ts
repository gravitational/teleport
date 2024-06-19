/**
 * @jest-environment node
 */
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

import { fork } from 'node:child_process';
import path from 'node:path';

import Logger, { NullService } from 'teleterm/logger';

import { terminateWithTimeout } from './terminateWithTimeout';

beforeAll(() => {
  Logger.init(new NullService());
});

test('kills a process gracefully when possible', async () => {
  const process = fork(path.join(__dirname, 'testProcess.mjs'), {
    silent: true,
  });

  await terminateWithTimeout(process);

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

  await terminateWithTimeout(process, 50);

  expect(process.killed).toBeTruthy();
  expect(process.signalCode).toBe('SIGKILL');
});

test('killing a process that failed to start is noop', async () => {
  const process = fork(path.join(__dirname, 'testProcess-nonExisting.mjs'), {
    silent: true,
  });
  jest.spyOn(process, 'kill');

  // wait for the process
  await new Promise(resolve => process.once('exit', resolve));
  await terminateWithTimeout(process, 1_000);

  expect(process.exitCode).toBe(1);
  expect(process.signalCode).toBeNull();
  expect(process.kill).toHaveBeenCalledTimes(0);
});

test('killing a process that has been already killed is noop', async () => {
  const process = fork(path.join(__dirname, 'testProcess.mjs'), {
    silent: true,
  });
  jest.spyOn(process, 'kill');

  process.kill('SIGTERM');
  await new Promise(resolve => process.once('exit', resolve));
  expect(process.killed).toBeTruthy();
  expect(process.signalCode).toBe('SIGTERM');

  await terminateWithTimeout(process, 1_000);
  expect(process.kill).toHaveBeenCalledTimes(1); // called only once, in the test
});
