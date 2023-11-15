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
  createAgentJoinedFile,
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
  jest.spyOn(fs, 'rm').mockImplementation(() => Promise.resolve());
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
