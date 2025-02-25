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

import childProcess from 'node:child_process';
import fs from 'node:fs/promises';

import { makeRuntimeSettings } from 'teleterm/mainProcess/fixtures/mocks';
import { RootClusterUri } from 'teleterm/ui/uri';

import {
  createAgentConfigFile,
  disableDebugServiceStanza,
  generateAgentConfigPaths,
} from './createAgentConfigFile';

jest.mock('node:child_process');
jest.mock('node:fs');

beforeEach(() => {
  jest
    .spyOn(childProcess, 'execFile')
    .mockImplementation((command, args, options, callback) => {
      callback(null, '', '');
      return undefined;
    });
  jest.spyOn(fs, 'rm').mockImplementation(() => Promise.resolve());
  jest.spyOn(fs, 'mkdir').mockImplementation(() => Promise.resolve(undefined));
  jest.spyOn(fs, 'writeFile').mockImplementation(() => Promise.resolve());
});

test('teleport configure is called with proper arguments', async () => {
  const userDataDir = '/Users/test/Application Data/Teleport Connect';
  const agentBinaryPath =
    '/Users/test/Caches/Teleport Connect/teleport/teleport';
  const token = '8f50fd5d-38e8-4e96-baea-e9b882bb433b';
  const proxy = 'cluster.local:3080';
  const rootClusterUri: RootClusterUri = '/clusters/cluster.local';
  const username = 'testuser@acme.com';

  await expect(
    createAgentConfigFile(
      makeRuntimeSettings({
        agentBinaryPath,
        userDataDir,
      }),
      {
        token,
        proxy,
        rootClusterUri,
        username,
      }
    )
  ).resolves.toBeUndefined();

  expect(childProcess.execFile).toHaveBeenCalledWith(
    agentBinaryPath,
    [
      'node',
      'configure',
      `--output=stdout`,
      `--data-dir=${userDataDir}/agents/cluster.local/data`,
      `--proxy=${proxy}`,
      `--token=${token}`,
      `--labels=teleport.dev/connect-my-computer/owner=${username}`,
    ],
    {
      timeout: 10_000, // 10 seconds
    },
    expect.anything()
  );
  expect(fs.writeFile).toHaveBeenCalledWith(
    `${userDataDir}/agents/cluster.local/config.yaml`,
    // It'd be nice to make childProcess.execFile return certain output and then verify that this
    // argument includes that output + disableDebugServiceStanza. Alas, the promisified version of
    // execFile isn't easily mockable – stdout in tests is just "undefined" for some reason.
    expect.stringContaining(disableDebugServiceStanza)
  );
});

test('previous config file is removed before calling teleport configure', async () => {
  const userDataDir = '/Users/test/Application Data/Teleport Connect';
  const rootClusterUri: RootClusterUri = '/clusters/cluster.local';

  await expect(
    createAgentConfigFile(
      makeRuntimeSettings({
        userDataDir,
      }),
      {
        token: '',
        proxy: '',
        rootClusterUri,
        username: 'alice',
      }
    )
  ).resolves.toBeUndefined();

  expect(fs.rm).toHaveBeenCalledWith(
    `${userDataDir}/agents/cluster.local/config.yaml`,
    { force: true }
  );
});

test('throws when rootClusterUri does not contain a valid path segment', () => {
  expect(() =>
    generateAgentConfigPaths(
      makeRuntimeSettings({
        userDataDir: '/Users/test/Application Data/Teleport Connect',
      }),
      '/clusters/../not_valid'
    )
  ).toThrow('The agent config path is incorrect');
});

test('throws when rootClusterUri is undefined', () => {
  expect(() =>
    generateAgentConfigPaths(
      makeRuntimeSettings({
        userDataDir: '/Users/test/Application Data/Teleport Connect',
      }),
      '/clusters/'
    )
  ).toThrow('Incorrect root cluster URI');
});
