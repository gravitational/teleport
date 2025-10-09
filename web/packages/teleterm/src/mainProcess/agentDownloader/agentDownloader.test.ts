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

/* eslint jest/no-conditional-expect: 0 */

import childProcess from 'node:child_process';
import fs from 'node:fs';
import fsPromises from 'node:fs/promises';
import { PassThrough } from 'node:stream';
import zlib from 'node:zlib';

import tarFs from 'tar-fs';

import Logger, { NullService } from 'teleterm/logger';

import { makeRuntimeSettings } from '../fixtures/mocks';
import { downloadAgent } from './agentDownloader';
import type { IFileDownloader } from './fileDownloader';

jest.mock('node:child_process');
jest.mock('node:fs');
jest.mock('node:fs/promises');
jest.mock('node:zlib');
jest.mock('tar-fs');

const LATEST_TELEPORT_VERSIONS_MOCK = [
  {
    releaseId: 'teleport@13.2.0',
    product: 'teleport',
    version: '13.2.0',
    notesMd: '',
    status: 'published',
    assets: [],
  },
  {
    releaseId: 'teleport@12.1.4',
    product: 'teleport',
    version: '12.1.4',
    notesMd: '',
    status: 'published',
    assets: [],
  },
];

beforeAll(() => {
  Logger.init(new NullService());
  // (Cannot spy the fetch property because it is not a function; undefined given instead)
  // eslint-disable-next-line jest/prefer-spy-on
  global.fetch = jest.fn().mockImplementation(() =>
    Promise.resolve({
      ok: true,
      json: () => Promise.resolve(LATEST_TELEPORT_VERSIONS_MOCK),
    })
  );
});

beforeEach(() => {
  jest.resetModules();
  process.env = {};
});

const testCases = [
  {
    name: 'Should not download agent when Connect version and read version are identical',
    connectVersion: '13.0.0',
    versionFromCache: '13.0.0',
    env: {},
    shouldDownloadBinary: false,
  },
  {
    name: 'Should download agent when Connect version and version from cache are different',
    connectVersion: '13.0.0',
    versionFromCache: '12.0.0',
    env: {},
    shouldDownloadBinary: 'teleport-v13.0.0-darwin-arm64-bin.tar.gz',
  },
  {
    name: 'Should download agent when version from cache is missing',
    connectVersion: '13.0.0',
    versionFromCache: undefined,
    env: {},
    shouldDownloadBinary: 'teleport-v13.0.0-darwin-arm64-bin.tar.gz',
  },
  {
    name: 'Should download the latest available agent version when Connect version is 1.0.0-dev',
    connectVersion: '1.0.0-dev',
    versionFromCache: undefined,
    env: {},
    shouldDownloadBinary: 'teleport-v13.2.0-darwin-arm64-bin.tar.gz',
  },
  {
    name: 'Should download agent version from env var when Connect is 1.0.0-dev and env var is provided',
    connectVersion: '1.0.0-dev',
    versionFromCache: undefined,
    env: {
      CONNECT_CMC_AGENT_VERSION: '12.1.0',
    },
    shouldDownloadBinary: 'teleport-v12.1.0-darwin-arm64-bin.tar.gz',
  },
];

test.each(testCases)(
  '$name',
  async ({ connectVersion, env, versionFromCache, shouldDownloadBinary }) => {
    const runtimeSettings = makeRuntimeSettings({
      tempDataDir: '/home/tmp',
      agentBinaryPath: '/home/teleport/teleport',
      sessionDataDir: '/home/cache',
      appVersion: connectVersion,
    });
    const fileDownloader: IFileDownloader = {
      run: jest.fn(() => Promise.resolve()),
    };
    jest
      .spyOn(childProcess, 'execFile')
      .mockImplementation((command, args, options, callback) => {
        if (versionFromCache) {
          // @ts-expect-error - it should be `callback(undefined, stdout, stderr)`,
          // but if I do this, asyncExec tries to read stdout.stdout (a string from string).
          callback(undefined, {
            stdout: versionFromCache,
            stderr: undefined,
          });
        } else {
          const error = new Error();
          error['code'] = 'ENOENT';
          callback(error, undefined, undefined);
        }
        return this;
      });
    jest.spyOn(fs, 'createReadStream').mockImplementation(getStreamMock);
    jest.spyOn(zlib, 'createUnzip').mockImplementation(getStreamMock);
    jest.spyOn(tarFs, 'extract').mockImplementation(getStreamMock);

    const agentTempDir = `${runtimeSettings.tempDataDir}/connect-my-computer-abc`;
    jest.spyOn(fsPromises, 'mkdtemp').mockResolvedValue(agentTempDir);

    const call = downloadAgent(fileDownloader, runtimeSettings, env);
    await expect(call).resolves.toBeUndefined();

    if (shouldDownloadBinary) {
      expect(fileDownloader.run).toHaveBeenCalledWith(
        `https://cdn.teleport.dev/${shouldDownloadBinary}`,
        agentTempDir
      );
      expect(fs.createReadStream).toHaveBeenCalledWith(
        `${agentTempDir}/${shouldDownloadBinary}`
      );
      expect(tarFs.extract).toHaveBeenCalledWith(
        runtimeSettings.sessionDataDir,
        expect.anything()
      );
      expect(fsPromises.rm).toHaveBeenCalledWith(agentTempDir, {
        recursive: true,
      });
    } else {
      expect(fileDownloader.run).not.toHaveBeenCalled();
      expect(fs.createReadStream).not.toHaveBeenCalled();
      expect(tarFs.extract).not.toHaveBeenCalled();
      expect(fsPromises.rm).not.toHaveBeenCalled();
    }
  }
);

function getStreamMock<T>() {
  const pt = new PassThrough();
  pt.end();
  return pt as T;
}
