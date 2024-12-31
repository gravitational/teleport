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

import path from 'node:path';

import { BrowserWindow, DownloadItem } from 'electron';

import Logger from 'teleterm/logger';

export interface IFileDownloader {
  run(url: string, downloadDirectory: string): Promise<void>;
}

export class FileDownloader implements IFileDownloader {
  private logger = new Logger('fileDownloader');

  constructor(private window: BrowserWindow) {}

  async run(url: string, downloadDirectory: string) {
    this.logger.info(
      `Starting download from ${url} (download directory: ${downloadDirectory}).`
    );

    let handler: ReturnType<typeof this.createDownloadHandler>;

    try {
      return await new Promise<void>((resolve, reject) => {
        handler = this.createDownloadHandler(url, downloadDirectory, {
          resolve: resolve,
          reject: reject,
        });
        this.window.webContents.session.on('will-download', handler);
        this.window.webContents.downloadURL(url);
      });
    } finally {
      this.window.webContents.session.off('will-download', handler);
    }
  }

  private createDownloadHandler(
    url: string,
    downloadDirectory: string,
    {
      resolve,
      reject,
    }: {
      resolve(): void;
      reject(error: Error): void;
    }
  ) {
    const onDownloadDone = () => {
      resolve();
      this.window.setProgressBar(-1);
      this.logger.info('Download finished');
    };

    const onDownloadError = (error: Error) => {
      reject(error);
      this.window.setProgressBar(-1, { mode: 'error' });
      this.logger.error(error);
    };

    return (_: Event, item: DownloadItem) => {
      const isExpectedUrl = item.getURL() === url;
      if (!isExpectedUrl) {
        // handle only expected URL
        return;
      }

      item.on('updated', (_, state) => {
        switch (state) {
          case 'interrupted':
            onDownloadError(new Error('Download failed: interrupted'));
            break;
          case 'progressing':
            this.onProgress(item.getReceivedBytes(), item.getTotalBytes());
        }
      });

      item.once('done', (_, state) => {
        switch (state) {
          case 'completed':
            onDownloadDone();
            break;
          case 'interrupted':
            // TODO(gzdunek): electron doesn't expose much information about why the download failed.
            // Fortunately, there is a PR in works that will add more info https://github.com/electron/electron/pull/38859.
            // Use DownloadItem.getLastReason() when it gets merged.
            onDownloadError(new Error(`Failed to download ${item.getURL()}`));
            break;
          case 'cancelled':
            onDownloadError(new Error('Download was cancelled.'));
        }
      });

      // Set the save path, making Electron not to prompt a save dialog.
      // We don't have to check if the filename contains forbidden characters, they are escaped by Chromium.
      // For example, downloading from the URL localhost:1234/%2Ftest ("/" encoded as %2F) gives _test filename.
      const filePath = path.join(downloadDirectory, item.getFilename());
      item.setSavePath(filePath);
    };
  }

  private onProgress(received: number, total: number) {
    const progress = received / total;
    this.window.setProgressBar(progress);
    this.logger.info(`Downloaded ${(progress * 100).toFixed(1)}%`);
  }
}
