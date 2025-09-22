/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { createHash } from 'node:crypto';

import { MacUpdater } from 'electron-updater';

import type { GetClusterVersionsResponse } from 'gen-proto-ts/teleport/lib/teleterm/auto_update/v1/auto_update_service_pb';
import { wait } from 'shared/utils/wait';

import Logger, { NullService } from 'teleterm/logger';

import { AppUpdateEvent, AppUpdater, AppUpdaterStorage } from './appUpdater';

const mockedAppVersion = '15.0.0';

jest.mock('electron', () => ({
  app: {
    // Should be false to avoid removing real Squirrel directory in `dispose`.
    isPackaged: false,
    getVersion: () => mockedAppVersion,
  },
  autoUpdater: { on: () => {} },
}));

beforeAll(() => {
  Logger.init(new NullService());

  // Mock fetching checksum.
  // Creates hash from the passed URL.
  global.fetch = jest.fn(async url => ({
    ok: true,
    text: async () =>
      `${createHash('sha-512').update(url).digest('hex')}  Teleport Connect-17.5.4-mac.zip`,
  })) as jest.Mock;
});

afterAll(() => {
  jest.restoreAllMocks();
});

function makeUpdaterStorage(
  initialValue: { managingClusterUri?: string } = {}
): AppUpdaterStorage {
  return {
    get: () => initialValue,
    put: newValue => (initialValue = newValue),
  };
}

// MacUpdater with mocked download and install features.
class MockedMacUpdater extends MacUpdater {
  constructor() {
    super(undefined, {
      appUpdateConfigPath: __dirname,
      baseCachePath: __dirname,
      isPackaged: false,
      userDataPath: '',
      onQuit(): void {},
      quit(): void {},
      relaunch(): void {},
      version: mockedAppVersion,
      whenReady: () => Promise.resolve(),
      name: 'Teleport Connect',
    });
    // Prevents electron-updater from creating .updaterId file.
    this.stagingUserIdPromise.value = Promise.resolve(
      '153432a8-93de-577c-a76a-3a042f1d7580'
    );
  }

  protected override async doDownloadUpdate(): Promise<string[]> {
    // Simulate download.
    await wait(10);
    this.dispatchUpdateDownloaded({
      ...this.updateInfoAndProvider.info,
      downloadedFile: 'some-update',
    });
    return ['some-update'];
  }

  override quitAndInstall() {}
}

function setUpAppUpdater(options: {
  clusters: GetClusterVersionsResponse;
  storage?: AppUpdaterStorage;
  processEnvVar?: string;
}) {
  const clusterGetter = async () => {
    return options.clusters;
  };

  const nativeUpdater = new MockedMacUpdater();

  const checkForUpdatesSpy = jest.spyOn(nativeUpdater, 'checkForUpdates');
  const downloadUpdateSpy = jest.spyOn(nativeUpdater, 'downloadUpdate');
  let lastEvent: { value?: AppUpdateEvent } = {};
  const appUpdater = new AppUpdater(
    options.storage || makeUpdaterStorage(),
    clusterGetter,
    async () => 'https://cdn.teleport.dev',
    event => {
      lastEvent.value = event;
    },
    options.processEnvVar,
    nativeUpdater
  );

  return {
    appUpdater,
    nativeUpdater,
    checkForUpdatesSpy,
    downloadUpdateSpy,
    lastEvent,
  };
}

test('auto-downloads update when all clusters are reachable', async () => {
  const setup = setUpAppUpdater({
    clusters: {
      reachableClusters: [
        {
          clusterUri: '/clusters/foo',
          toolsAutoUpdate: true,
          toolsVersion: '19.0.0',
          minToolsVersion: '18.0.0-aa',
        },
      ],
      unreachableClusters: [],
    },
  });

  await setup.appUpdater.checkForUpdates();
  expect(setup.lastEvent.value).toEqual(
    expect.objectContaining({
      kind: 'update-available',
      autoDownload: true,
    })
  );
  expect(setup.downloadUpdateSpy).toHaveBeenCalledTimes(1);

  await setup.downloadUpdateSpy.mock.results[0].value;
  expect(setup.lastEvent.value).toEqual(
    expect.objectContaining({
      kind: 'update-downloaded',
    })
  );
});

test('does not auto-download update when there are unreachable clusters', async () => {
  const setup = setUpAppUpdater({
    clusters: {
      reachableClusters: [
        {
          clusterUri: '/clusters/foo',
          toolsAutoUpdate: true,
          toolsVersion: '19.0.0',
          minToolsVersion: '18.0.0-aa',
        },
      ],
      unreachableClusters: [
        {
          clusterUri: '/clusters/bar',
          errorMessage: 'Network issue',
        },
      ],
    },
  });

  await setup.appUpdater.checkForUpdates();
  expect(setup.lastEvent.value).toEqual(
    expect.objectContaining({
      kind: 'update-available',
      autoDownload: false,
    })
  );
  expect(setup.downloadUpdateSpy).toHaveBeenCalledTimes(0);
});

test('does not auto-download update when env var is set to off', async () => {
  const setup = setUpAppUpdater({
    processEnvVar: 'off',
    clusters: {
      reachableClusters: [
        {
          clusterUri: '/clusters/foo',
          toolsAutoUpdate: true,
          toolsVersion: '19.0.0',
          minToolsVersion: '18.0.0-aa',
        },
      ],
      unreachableClusters: [],
    },
  });

  await setup.appUpdater.checkForUpdates({ noAutoDownload: true });
  expect(setup.lastEvent.value).toEqual(
    expect.objectContaining({
      kind: 'update-not-available',
      autoUpdatesStatus: expect.objectContaining({
        reason: 'disabled-by-env-var',
      }),
    })
  );
  expect(setup.downloadUpdateSpy).toHaveBeenCalledTimes(0);
});

test('does not auto-download update when all clusters are reachable and noAutoDownload is set', async () => {
  const setup = setUpAppUpdater({
    clusters: {
      reachableClusters: [
        {
          clusterUri: '/clusters/foo',
          toolsAutoUpdate: true,
          toolsVersion: '19.0.0',
          minToolsVersion: '18.0.0-aa',
        },
      ],
      unreachableClusters: [],
    },
  });

  await setup.appUpdater.checkForUpdates({ noAutoDownload: true });
  expect(setup.lastEvent.value).toEqual(
    expect.objectContaining({
      kind: 'update-available',
      autoDownload: false,
    })
  );
  expect(setup.downloadUpdateSpy).toHaveBeenCalledTimes(0);
});

test('discards previous update if a new one is found that should not auto-download', async () => {
  const clusters = {
    reachableClusters: [
      {
        clusterUri: '/clusters/foo',
        toolsAutoUpdate: true,
        toolsVersion: '19.0.0',
        minToolsVersion: '18.0.0-aa',
      },
    ],
    unreachableClusters: [],
  };

  const setup = setUpAppUpdater({
    clusters,
  });

  await setup.appUpdater.checkForUpdates();
  expect(setup.downloadUpdateSpy).toHaveBeenCalledTimes(1);
  await setup.downloadUpdateSpy.mock.results[0].value;
  expect(setup.lastEvent.value).toEqual(
    expect.objectContaining({
      kind: 'update-downloaded',
      update: expect.objectContaining({
        version: '19.0.0',
      }),
    })
  );

  clusters.reachableClusters = [
    {
      clusterUri: '/clusters/foo',
      toolsAutoUpdate: true,
      toolsVersion: '19.0.0',
      minToolsVersion: '18.0.0-aa',
    },

    // This cluster is on newer version, so it will be providing updates.
    {
      clusterUri: '/clusters/bar',
      toolsAutoUpdate: true,
      toolsVersion: '19.0.1',
      minToolsVersion: '18.0.0-aa',
    },
  ];
  await setup.appUpdater.checkForUpdates({ noAutoDownload: true });
  expect(setup.downloadUpdateSpy).toHaveBeenCalledTimes(1);
  expect(setup.lastEvent.value).toEqual(
    expect.objectContaining({
      autoDownload: false,
      kind: 'update-available',
      update: expect.objectContaining({
        version: '19.0.1',
      }),
    })
  );
  await setup.appUpdater.dispose();
  // Check if the app is set to discard the first downloaded update on close.
  expect(setup.nativeUpdater.autoInstallOnAppQuit).toBeFalsy();
});

test('discards previous update if the latest check returns no update', async () => {
  const clusters = {
    reachableClusters: [
      {
        clusterUri: '/clusters/foo',
        toolsAutoUpdate: true,
        toolsVersion: '19.0.0',
        minToolsVersion: '18.0.0-aa',
      },
    ],
    unreachableClusters: [],
  };

  const setup = setUpAppUpdater({
    clusters,
  });

  await setup.appUpdater.checkForUpdates();
  expect(setup.downloadUpdateSpy).toHaveBeenCalledTimes(1);
  await setup.downloadUpdateSpy.mock.results[0].value;
  expect(setup.lastEvent.value).toEqual(
    expect.objectContaining({
      kind: 'update-downloaded',
      update: expect.objectContaining({
        version: '19.0.0',
      }),
    })
  );

  clusters.reachableClusters = [];
  await setup.appUpdater.checkForUpdates();
  expect(setup.downloadUpdateSpy).toHaveBeenCalledTimes(1);
  expect(setup.lastEvent.value).toEqual(
    expect.objectContaining({
      kind: 'update-not-available',
    })
  );
  await setup.appUpdater.dispose();
  // Check if the app is set to discard the first downloaded update on close.
  expect(setup.nativeUpdater.autoInstallOnAppQuit).toBeFalsy();
});
