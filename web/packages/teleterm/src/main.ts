import { spawn } from 'child_process';
import { app, globalShortcut, shell } from 'electron';
import MainProcess from 'teleterm/mainProcess';
import { getRuntimeSettings } from 'teleterm/mainProcess/runtimeSettings';
import createLoggerService from 'teleterm/services/logger';
import Logger from 'teleterm/logger';
import * as types from 'teleterm/types';
import { ConfigServiceImpl } from 'teleterm/services/config';
import { createFileStorage } from 'teleterm/services/fileStorage';
import path from 'path';
import { WindowsManager } from 'teleterm/mainProcess/windowsManager';

const settings = getRuntimeSettings();
const logger = initMainLogger(settings);
const fileStorage = createFileStorage({
  filePath: path.join(settings.userDataDir, 'app_state.json'),
});
const configService = new ConfigServiceImpl();
const windowsManager = new WindowsManager(fileStorage, settings);

process.on('uncaughtException', error => {
  logger.error('', error);
  app.quit();
});

// init main process
const mainProcess = MainProcess.create({
  settings,
  logger,
  configService,
  fileStorage,
});

app.on(
  'certificate-error',
  (event, webContents, url, error, certificate, callback) => {
    // allow certs errors for localhost:8080
    if (
      settings.dev &&
      new URL(url).host === 'localhost:8080' &&
      error === 'net::ERR_CERT_AUTHORITY_INVALID'
    ) {
      event.preventDefault();
      callback(true);
    } else {
      callback(false);
      console.error(error);
    }
  }
);

app.on('will-quit', () => {
  fileStorage.putAllSync();
  globalShortcut.unregisterAll();
  mainProcess.dispose();
});

app.whenReady().then(() => {
  if (mainProcess.settings.dev) {
    // allow restarts on F6
    globalShortcut.register('F6', () => {
      mainProcess.dispose();
      const [bin, ...args] = process.argv;
      const child = spawn(bin, args, {
        env: process.env,
        detached: true,
        stdio: 'inherit',
      });
      child.unref();
      app.quit();
    });
  }

  windowsManager.createWindow();
});

// Limit navigation capabilities to reduce the attack surface.
// See TEL-Q122-19 from "Teleport Core Testing Q1 2022" security audit.
//
// See also points 12, 13 and 14 from the Electron's security tutorial.
// https://github.com/electron/electron/blob/v17.2.0/docs/tutorial/security.md#12-verify-webview-options-before-creation
app.on('web-contents-created', (_, contents) => {
  contents.on('will-navigate', (event, navigationUrl) => {
    logger.warn(`Navigation to ${navigationUrl} blocked by 'will-navigate'`);
    event.preventDefault();
  });

  // The usage of webview is blocked by default, but let's include the handler just in case.
  // https://github.com/electron/electron/blob/v17.2.0/docs/api/webview-tag.md#enabling
  contents.on('will-attach-webview', (event, _, params) => {
    logger.warn(
      `Opening a webview to ${params.src} blocked by 'will-attach-webview'`
    );
    event.preventDefault();
  });

  contents.setWindowOpenHandler(details => {
    const url = new URL(details.url);

    function isUrlSafe(): boolean {
      if (url.host === 'goteleport.com') {
        return true;
      }
      if (
        url.host === 'github.com' &&
        url.pathname.startsWith('/gravitational/')
      ) {
        return true;
      }
    }

    // Open links to documentation and GitHub issues in the external browser.
    // They need to have `target` set to `_blank`.
    if (isUrlSafe()) {
      shell.openExternal(url.toString());
    } else {
      logger.warn(
        `Opening a new window to ${url} blocked by 'setWindowOpenHandler'`
      );
    }

    return { action: 'deny' };
  });
});

function initMainLogger(settings: types.RuntimeSettings) {
  const service = createLoggerService({
    dev: settings.dev,
    dir: settings.userDataDir,
    name: 'main',
  });

  Logger.init(service);

  return new Logger('Main');
}
