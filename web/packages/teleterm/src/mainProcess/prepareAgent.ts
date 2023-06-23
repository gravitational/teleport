import { pipeline } from 'node:stream/promises';
import { createReadStream } from 'node:fs';
import { realpath } from 'node:fs/promises';

import { BrowserWindow } from 'electron';
import { extract } from 'tar-fs';
import gunzip from 'gunzip-maybe';

import Logger from 'teleterm/logger';
import os from 'os';

const FILE_NAME = 'teleport-v13.1.2-darwin-arm64-bin';
const logger = new Logger('downloadAgent');

async function downloadAgent(
  window: Electron.CrossProcessExports.BrowserWindow,
  cachePath: string
): Promise<void> {
  return new Promise<void>((resolve, reject) => {
    window.webContents.session.on('will-download', (event, item) => {
      // Set the save path, making Electron not to prompt a save dialog.
      item.setSavePath(`${cachePath}/${FILE_NAME}.tar.gz`);
      logger.info(item.getSavePath());
      item.on('updated', (event, state) => {
        if (state === 'interrupted') {
          logger.info('Download is interrupted but can be resumed');
        } else if (state === 'progressing') {
          if (item.isPaused()) {
            logger.info('Download is paused');
          } else {
            logger.info(`Received bytes: ${item.getReceivedBytes()}`);
          }
        }
      });
      item.once('done', (event, state) => {
        if (state === 'completed') {
          logger.info(`Downloaded successfully ${JSON.stringify(event)}`);
          resolve();
        } else {
          logger.info(`Download failed: ${state}`);
          reject();
        }
      });
    });

    logger.info('Starting download!');
    window.webContents.downloadURL(
      `https://cdn.teleport.dev/${FILE_NAME}.tar.gz`
    );
  });
}

async function unpackAgent(cachePath: string): Promise<void> {
  await pipeline(
    createReadStream(`${cachePath}/${FILE_NAME}.tar.gz`),
    gunzip(),
    extract(`${cachePath}`)
  );
}

export async function prepareAgent(window: BrowserWindow): Promise<void> {
  const cachePath = `~/Library/Caches/Teleport Connect/`.replace(
    '~',
    os.homedir
  );
  console.log(cachePath);
  await downloadAgent(window, cachePath);
  await unpackAgent(cachePath);
  logger.info('Unpacked agent');
}
