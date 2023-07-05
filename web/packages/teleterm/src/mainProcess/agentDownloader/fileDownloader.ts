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

import path from 'node:path';

import { BrowserWindow, DownloadItem } from 'electron';

import Logger from 'teleterm/logger';

export interface IFileDownloader {
  run(url: string, downloadDirectory: string): Promise<void>;
}

export class FileDownloader implements IFileDownloader {
  private logger = new Logger('fileDownloader');
  private onResolve: () => void;
  private onReject: (error: Error) => void;

  constructor(private window: BrowserWindow) {}

  async run(url: string, downloadDirectory: string) {
    this.logger.info(
      `Starting download from ${url} (download directory: ${downloadDirectory}).`
    );
    const handler = this.createDownloadHandler(url, downloadDirectory);
    this.window.webContents.session.on('will-download', handler);

    try {
      return await new Promise<void>((resolve, reject) => {
        this.onResolve = resolve;
        this.onReject = reject;
        this.window.webContents.downloadURL(url);
      });
    } finally {
      this.window.webContents.session.off('will-download', handler);
    }
  }

  private createDownloadHandler(url: string, downloadDirectory: string) {
    return (_: Event, item: DownloadItem) => {
      const isExpectedUrl = item.getURL() === url;
      if (!isExpectedUrl) {
        // handle only expected URL
        return;
      }

      item.on('updated', (_, state) => {
        switch (state) {
          case 'interrupted':
            this.onDownloadError(new Error('Download failed: interrupted'));
            break;
          case 'progressing':
            this.onProgress(item.getReceivedBytes(), item.getTotalBytes());
        }
      });

      item.once('done', (_, state) => {
        switch (state) {
          case 'completed':
            this.onDownloadDone();
            break;
          case 'interrupted':
            // TODO(gzdunek): electron doesn't expose much information about why the download failed.
            // Fortunately, there is a PR in works that will add more info https://github.com/electron/electron/pull/38859.
            // Use Use DownloadItem.getLastReason() when merged.
            this.onDownloadError(
              new Error(
                `Download failed. Requested file may not exist or is temporary unavailable.`
              )
            );
            break;
          case 'cancelled':
            // We cancel the download only when filename is incorrect, so we can mention that in the error message.
            this.onDownloadError(
              new Error(
                `Download was cancelled. Filename ${item.getFilename()} is incorrect.`
              )
            );
        }
      });

      // The filename is taken from Content-Disposition header, so it is worth to check if it does not contain
      // any disallowed characters that could cause storing the file outside the download directory.
      if (path.basename(item.getFilename()) !== item.getFilename()) {
        item.cancel();
      } else {
        // set the save path, making Electron not to prompt a save dialog
        const filePath = path.join(downloadDirectory, item.getFilename());
        item.setSavePath(filePath);
      }
    };
  }

  private onProgress(received: number, total: number) {
    const progress = received / total;
    this.window.setProgressBar(progress);
    this.logger.info(`Downloaded ${(progress * 100).toFixed(1)}%`);
  }

  private onDownloadDone() {
    this.onResolve();
    this.window.setProgressBar(-1);
    this.logger.info('Download finished');
  }

  private onDownloadError(error: Error) {
    this.onReject(error);
    this.window.setProgressBar(-1, { mode: 'error' });
    this.logger.error(error);
  }
}
