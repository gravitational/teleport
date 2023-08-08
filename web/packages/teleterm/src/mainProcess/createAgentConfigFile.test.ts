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

import childProcess from 'node:child_process';
import fs from 'node:fs/promises';

import { RootClusterUri } from 'teleterm/ui/uri';
import { makeRuntimeSettings } from 'teleterm/mainProcess/fixtures/mocks';

import {
  createAgentConfigFile,
  generateAgentConfigPaths,
} from './createAgentConfigFile';

jest.mock('node:child_process');
jest.mock('node:fs');

beforeEach(() => {
  jest
    .spyOn(childProcess, 'execFile')
    .mockImplementation((command, args, options, callback) => {
      callback(undefined, '', '');
      return this;
    });
  jest.spyOn(fs, 'rm').mockResolvedValue();
});

test('teleport configure is called with proper arguments', async () => {
  const userDataDir = '/Users/test/Application Data/Teleport Connect';
  const agentBinaryPath =
    '/Users/test/Caches/Teleport Connect/teleport/teleport';
  const token = '8f50fd5d-38e8-4e96-baea-e9b882bb433b';
  const proxy = 'cluster.local:3080';
  const rootClusterUri: RootClusterUri = '/clusters/cluster.local';
  const labels = [
    {
      name: 'teleport.dev/connect-my-computer/owner',
      value: 'testuser@acme.com',
    },
    {
      name: 'env',
      value: 'dev',
    },
  ];

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
        labels,
      }
    )
  ).resolves.toBeUndefined();

  expect(childProcess.execFile).toHaveBeenCalledWith(
    agentBinaryPath,
    [
      'node',
      'configure',
      `--output=${userDataDir}/agents/cluster.local/config.yaml`,
      `--data-dir=${userDataDir}/agents/cluster.local/data`,
      `--proxy=${proxy}`,
      `--token=${token}`,
      `--labels=${labels[0].name}=${labels[0].value},${labels[1].name}=${labels[1].value}`,
    ],
    {
      timeout: 10_000, // 10 seconds
    },
    expect.anything()
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
        labels: [],
      }
    )
  ).resolves.toBeUndefined();

  expect(fs.rm).toHaveBeenCalledWith(
    `${userDataDir}/agents/cluster.local/config.yaml`
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
