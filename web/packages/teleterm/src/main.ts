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

import { spawn } from 'node:child_process';
import os from 'node:os';
import path from 'node:path';

import { app, globalShortcut, shell, nativeTheme } from 'electron';

import MainProcess from 'teleterm/mainProcess';
import { getRuntimeSettings } from 'teleterm/mainProcess/runtimeSettings';
import { enableWebHandlersProtection } from 'teleterm/mainProcess/protocolHandler';
import { LoggerColor, createFileLoggerService } from 'teleterm/services/logger';
import Logger from 'teleterm/logger';
import * as types from 'teleterm/types';
import {
  createConfigService,
  runConfigFileMigration,
} from 'teleterm/services/config';
import { createFileStorage } from 'teleterm/services/fileStorage';
import { WindowsManager } from 'teleterm/mainProcess/windowsManager';
import { TELEPORT_CUSTOM_PROTOCOL } from 'teleterm/ui/uri';

// Set the app as a default protocol client only if it wasn't started through `electron .`.
if (!process.defaultApp) {
  app.setAsDefaultProtocolClient(TELEPORT_CUSTOM_PROTOCOL);
}

if (app.requestSingleInstanceLock()) {
  initializeApp();
} else {
  console.log('Attempted to open a second instance of the app, exiting.');
  // All windows will be closed immediately without asking the user,
  // and the before-quit and will-quit events will not be emitted.
  app.exit(1);
}

function initializeApp(): void {
  updateSessionDataPath();
  let devRelaunchScheduled = false;
  const settings = getRuntimeSettings();
  const logger = initMainLogger(settings);
  logger.info(`Starting ${app.getName()} version ${app.getVersion()}`);
  const {
    appStateFileStorage,
    configFileStorage,
    configJsonSchemaFileStorage,
  } = createFileStorages(settings.userDataDir);

  runConfigFileMigration(configFileStorage);
  const configService = createConfigService({
    configFile: configFileStorage,
    jsonSchemaFile: configJsonSchemaFileStorage,
    platform: settings.platform,
  });

  nativeTheme.themeSource = configService.get('theme').value;
  const windowsManager = new WindowsManager(appStateFileStorage, settings);

  process.on('uncaughtException', (error, origin) => {
    logger.error(origin, error);
    app.quit();
  });

  // init main process
  const mainProcess = MainProcess.create({
    settings,
    logger,
    configService,
    appStateFileStorage,
    configFileStorage,
    windowsManager,
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

  app.on('will-quit', async event => {
    event.preventDefault();
    const disposeMainProcess = async () => {
      try {
        await mainProcess.dispose();
      } catch (e) {
        logger.error('Failed to gracefully dispose of main process', e);
      }
    };

    globalShortcut.unregisterAll();
    await Promise.all([appStateFileStorage.write(), disposeMainProcess()]); // none of them can throw
    app.exit();
  });

  app.on('quit', () => {
    if (devRelaunchScheduled) {
      const [bin, ...args] = process.argv;
      const child = spawn(bin, args, {
        env: process.env,
        detached: true,
        stdio: 'inherit',
      });
      child.unref();
    }
  });

  app.on('second-instance', () => {
    windowsManager.focusWindow();
  });

  // Since setUpDeepLinks adds another listener for second-instance, it's important to call it after
  // the listener which calls windowsManager.focusWindow. This way the focus will be brought to the
  // window before processing the listener for deep links.
  //
  // The setup must be done synchronously when starting the app, otherwise the listeners won't get
  // triggered on macOS if the app is not already running when the user opens a deep link.
  setUpDeepLinks(logger, windowsManager, settings);

  app.whenReady().then(() => {
    if (mainProcess.settings.dev) {
      // allow restarts on F6
      globalShortcut.register('F6', () => {
        devRelaunchScheduled = true;
        app.quit();
      });
    }

    enableWebHandlersProtection();

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
        if (url.protocol !== 'https:') {
          return false;
        }
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
}

/**
 * There is an outstanding issue about Electron storing its caches in the wrong location https://github.com/electron/electron/issues/8124.
 * Based on the Apple Developer docs (https://developer.apple.com/documentation/foundation/optimizing_your_app_s_data_for_icloud_backup/#3928528)
 * and the discussion under that issue, changing the location of `sessionData` to `~/Library/Caches` on macOS
 * and `XDG_CACHE_HOME` or `~/.cache`
 * (https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html) on Linux seems like a reasonable thing to do.
 */
function updateSessionDataPath() {
  switch (process.platform) {
    case 'linux': {
      const xdgCacheHome = process.env.XDG_CACHE_HOME;
      const cacheDirectory = xdgCacheHome || `${os.homedir()}/.cache`;
      app.setPath('sessionData', path.resolve(cacheDirectory, app.getName()));
      break;
    }
    case 'darwin': {
      app.setPath(
        'sessionData',
        path.resolve(os.homedir(), 'Library', 'Caches', app.getName())
      );
      break;
    }
    case 'win32': {
      const localAppData = process.env.LOCALAPPDATA;
      app.setPath('sessionData', path.resolve(localAppData, app.getName()));
    }
  }
}

function initMainLogger(settings: types.RuntimeSettings) {
  const service = createFileLoggerService({
    dev: settings.dev,
    dir: settings.logsDir,
    name: 'main',
    loggerNameColor: LoggerColor.Magenta,
  });

  Logger.init(service);

  return new Logger('Main');
}

function createFileStorages(userDataDir: string) {
  return {
    appStateFileStorage: createFileStorage({
      filePath: path.join(userDataDir, 'app_state.json'),
      debounceWrites: true,
    }),
    configFileStorage: createFileStorage({
      filePath: path.join(userDataDir, 'app_config.json'),
      debounceWrites: false,
      discardUpdatesOnLoadError: true,
    }),
    configJsonSchemaFileStorage: createFileStorage({
      filePath: path.join(userDataDir, 'schema_app_config.json'),
      debounceWrites: false,
    }),
  };
}

// Important: Deep links work only with a packaged version of the app.
//
// Technically, Windows could support deep links with a non-packaged version of the app, but for
// simplicity's sake we don't support this.
function setUpDeepLinks(
  logger: Logger,
  windowsManager: WindowsManager,
  settings: types.RuntimeSettings
) {
  // The setup is done according to the docs:
  // https://www.electronjs.org/docs/latest/tutorial/launch-app-from-url-in-another-app

  if (settings.platform === 'darwin') {
    // Deep link click on macOS.
    app.on('open-url', (event, url) => {
      // When macOS launches an app as a result of a deep link click, macOS does bring focus to the
      // _application_ itself if the app is already running. However, if the app has one window and
      // the window is minimized, it'll remain so. So we have to focus the window ourselves.
      windowsManager.focusWindow();

      logger.info(`Deep link launch from open-url, URL: ${url}`);
    });
    return;
  }

  // Do not handle deep links if the app was started from `electron .`, as custom protocol URLs
  // won't be forwarded to the app on Linux in this case.
  if (process.defaultApp) {
    return;
  }

  // Deep link click if the app is already opened (Windows or Linux).
  app.on('second-instance', (event, argv) => {
    // There's already a second-instance listener that gives focus to the main window, so we don't
    // do this in this listener.

    const url = findCustomProtocolUrlInArgv(argv);
    if (url) {
      logger.info(`Deep link launch from second-instance, URI: ${url}`);
    }
  });

  // Deep link click if the app is not running (Windows or Linux).
  const url = findCustomProtocolUrlInArgv(process.argv);

  if (!url) {
    return;
  }
  logger.info(`Deep link launch from process.argv, URL: ${url}`);
}

// We don't know the exact position of the URL is in argv. Chromium might inject its own arguments
// into argv. See https://www.electronjs.org/docs/latest/api/app#event-second-instance.
function findCustomProtocolUrlInArgv(argv: string[]) {
  return argv.find(arg => arg.startsWith(`${TELEPORT_CUSTOM_PROTOCOL}://`));
}
