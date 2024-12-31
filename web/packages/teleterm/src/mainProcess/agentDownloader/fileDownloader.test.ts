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

import EventEmitter from 'events';

import type { BrowserWindow, DownloadItem } from 'electron';

import Logger, { NullService } from 'teleterm/logger';

import { FileDownloader } from './fileDownloader';

const DOWNLOAD_DIR = '/temp';
const FILE_NAME = 'teleport-v13.1.0-darwin-arm64-bin.tar.gz';
const URL = `https://cdn.teleport.dev/${FILE_NAME}`;

beforeAll(() => {
  Logger.init(new NullService());
});

const getBrowserWindowMock = () => {
  const willDownloadEmitter = new EventEmitter();
  const downloadItemEmitter = new EventEmitter();
  const downloadItem: DownloadItem = {
    setSavePath: jest.fn(),
    getURL: () => URL,
    getFilename: () => {
      return FILE_NAME;
    },
    cancel: () => {
      downloadItemEmitter.emit('done', undefined, 'cancelled');
    },
    getReceivedBytes: () => 100,
    getTotalBytes: () => 200,
    on: (event, listener) => {
      downloadItemEmitter.on(event, listener);
      return this;
    },
    once: (event, listener) => {
      downloadItemEmitter.once(event, listener);
      return this;
    },
  } as unknown as DownloadItem;

  const browserWindow = {
    setProgressBar: jest.fn(),
    webContents: {
      downloadURL: jest.fn(() => {
        willDownloadEmitter.emit('will-download', undefined, downloadItem);
        // send some progress after 500 ms
        setTimeout(
          () => downloadItemEmitter.emit('updated', undefined, 'progressing'),
          500
        );
        // finish download after 1 s
        setTimeout(
          () => downloadItemEmitter.emit('done', undefined, 'completed'),
          1_000
        );
      }),
      session: {
        on: (event, listener) => willDownloadEmitter.on(event, listener),
        off: (event, listener) => willDownloadEmitter.off(event, listener),
      },
    },
  } as unknown as jest.MockedObjectDeep<BrowserWindow>;

  return {
    browserWindow,
    downloadItem,
    willDownloadEmitter,
    downloadItemEmitter,
  };
};

test('resolves a promise when download succeeds', async () => {
  jest.useFakeTimers();
  const { browserWindow, downloadItem } = getBrowserWindowMock();
  const downloader = new FileDownloader(browserWindow);
  const result = downloader.run(URL, DOWNLOAD_DIR);

  expect(browserWindow.webContents.downloadURL).toHaveBeenCalledWith(URL);

  jest.advanceTimersByTime(500);
  expect(browserWindow.setProgressBar).toHaveBeenCalledWith(0.5);

  jest.advanceTimersByTime(500);
  await expect(result).resolves.toBeUndefined();
  expect(browserWindow.setProgressBar).toHaveBeenCalledWith(-1);
  expect(downloadItem.setSavePath).toHaveBeenCalledWith(
    `${DOWNLOAD_DIR}/${FILE_NAME}`
  );
});

test('rejects a promise when an unexpected error occurs', async () => {
  const {
    browserWindow,
    downloadItem,
    downloadItemEmitter,
    willDownloadEmitter,
  } = getBrowserWindowMock();
  browserWindow.webContents.downloadURL.mockImplementation(() => {
    willDownloadEmitter.emit('will-download', undefined, downloadItem);
    // instead of emitting success event, emit error
    downloadItemEmitter.emit('done', undefined, 'interrupted');
  });
  const downloader = new FileDownloader(browserWindow);
  const result = downloader.run(URL, DOWNLOAD_DIR);

  expect(browserWindow.webContents.downloadURL).toHaveBeenCalledWith(URL);
  await expect(result).rejects.toThrow(`Failed to download ${URL}`);
  expect(browserWindow.setProgressBar).toHaveBeenCalledWith(-1, {
    mode: 'error',
  });
  expect(downloadItem.setSavePath).toHaveBeenCalled();
});
