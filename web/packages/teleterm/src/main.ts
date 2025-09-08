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

import os from 'node:os';
import path from 'node:path';

import { app, dialog, nativeTheme, shell } from 'electron';

import { CUSTOM_PROTOCOL } from 'shared/deepLinks';
import { ensureError } from 'shared/utils/error';

import { parseDeepLink } from 'teleterm/deepLinks';
import Logger from 'teleterm/logger';
import MainProcess from 'teleterm/mainProcess';
import { enableWebHandlersProtection } from 'teleterm/mainProcess/protocolHandler';
import { manageRootClusterProxyHostAllowList } from 'teleterm/mainProcess/rootClusterProxyHostAllowList';
import { getRuntimeSettings } from 'teleterm/mainProcess/runtimeSettings';
import { WindowsManager } from 'teleterm/mainProcess/windowsManager';
import { createConfigService } from 'teleterm/services/config';
import { createFileStorage } from 'teleterm/services/fileStorage';
import { createFileLoggerService, LoggerColor } from 'teleterm/services/logger';
import * as types from 'teleterm/types';
import { assertUnreachable } from 'teleterm/ui/utils';

import { setTray } from './tray';

if (!app.isPackaged) {
  // Sets app name and data directories to Electron.
  // Allows running packaged and non-packaged Connect at the same time.
  app.setName('Electron');
}

// Set the app as a default protocol client only if it wasn't started through `electron .`.
if (!process.defaultApp) {
  app.setAsDefaultProtocolClient(CUSTOM_PROTOCOL);
}

// Fix a bug introduced in Electron 36.
// https://github.com/electron/electron/issues/46538#issuecomment-2808806722
app.commandLine.appendSwitch('gtk-version', '3');

if (app.requestSingleInstanceLock()) {
  initializeApp();
} else {
  console.log('Attempted to open a second instance of the app, exiting.');
  // All windows will be closed immediately without asking the user,
  // and the before-quit and will-quit events will not be emitted.
  app.exit(1);
}

async function initializeApp(): Promise<void> {
  updateSessionDataPath();
  const settings = await getRuntimeSettings();
  const logger = initMainLogger(settings);
  logger.info(`Starting ${app.getName()} version ${app.getVersion()}`);
  const {
    appStateFileStorage,
    configFileStorage,
    configJsonSchemaFileStorage,
  } = createFileStorages(settings.userDataDir);

  const configService = createConfigService({
    configFile: configFileStorage,
    jsonSchemaFile: configJsonSchemaFileStorage,
    settings,
  });

  nativeTheme.themeSource = configService.get('theme').value;
  const windowsManager = new WindowsManager(
    appStateFileStorage,
    settings,
    configService
  );

  process.on('uncaughtException', (error, origin) => {
    logger.error('Uncaught exception', origin, error);
    showDialogWithError(`Uncaught exception (${origin} origin)`, error);
    app.exit(1);
  });

  let mainProcess: MainProcess;
  try {
    mainProcess = MainProcess.create({
      settings,
      logger,
      configService,
      appStateFileStorage,
      configFileStorage,
      windowsManager,
    });
  } catch (error) {
    const message = 'Could not initialize the main process';
    logger.error(message, error);
    showDialogWithError(message, error);
    // app.exit(1) isn't equivalent to throwing an error, use an explicit return to stop further
    // execution. See https://github.com/gravitational/teleport/issues/56272.
    app.exit(1);
    return;
  }

  app.on('will-quit', async event => {
    event.preventDefault();
    const disposeMainProcess = async () => {
      try {
        await mainProcess.dispose();
      } catch (e) {
        logger.error('Failed to gracefully dispose of main process', e);
      }
    };

    await Promise.all([appStateFileStorage.write(), disposeMainProcess()]); // none of them can throw
    app.exit();
  });

  // On Windows/Linux: Re-launching the app while it's already running
  // triggers 'second-instance' (because of app.requestSingleInstanceLock()).
  //
  // On macOS: Re-launching the app (from places like Finder, Spotlight, or Dock)
  // does not trigger 'second-instance'. Instead, the system emits 'activate'.
  // However, launching the app outside the desktop manager (e.g., from the command
  // line) does trigger 'second-instance'.
  app.on('second-instance', () => {
    windowsManager.focusWindow();
  });
  app.on('activate', () => {
    windowsManager.focusWindow();
  });

  // Since setUpDeepLinks adds another listener for second-instance, it's important to call it after
  // the listener which calls windowsManager.focusWindow. This way the focus will be brought to the
  // window before processing the listener for deep links.
  setUpDeepLinks(logger, windowsManager, settings);

  const rootClusterProxyHostAllowList = new Set<string>();

  (async () => {
    const { terminalService } = await mainProcess.getTshdClients();

    manageRootClusterProxyHostAllowList({
      tshdClient: terminalService,
      logger,
      allowList: rootClusterProxyHostAllowList,
    });
  })().catch(error => {
    const message = 'Could not initialize the tshd client in the main process';
    logger.error(message, error);
    showDialogWithError(message, error);
    app.exit(1);
  });

  app
    .whenReady()
    .then(() => {
      enableWebHandlersProtection();

      windowsManager.createWindow();

      if (configService.get('runInBackground').value) {
        setTray(settings, { show: () => windowsManager.showWindow() });
      }
    })
    .catch(error => {
      const message = 'Could not create the main app window';
      logger.error(message, error);
      showDialogWithError(message, error);
      app.exit(1);
    });

  // Limit navigation capabilities to reduce the attack surface.
  // See TEL-Q122-19 from "Teleport Core Testing Q1 2022" security audit.
  //
  // See also points 12, 13 and 14 from the Electron's security tutorial.
  // https://github.com/electron/electron/blob/v17.2.0/docs/tutorial/security.md#12-verify-webview-options-before-creation
  app.on('web-contents-created', (_, contents) => {
    contents.on('will-navigate', (event, navigationUrl) => {
      // Allow reloading the renderer app in dev mode.
      if (settings.dev && new URL(navigationUrl).host === 'localhost:8080') {
        return;
      }
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

        // Allow opening links to the Web UIs of root clusters currently added in the app.
        if (rootClusterProxyHostAllowList.has(url.host)) {
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
        dialog.showErrorBox(
          'Cannot open this link',
          'The domain does not match any of the allowed domains. Check main.log for more details.'
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
      launchDeepLink(logger, windowsManager, url);
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
      launchDeepLink(logger, windowsManager, url);
    }
  });

  // Deep link click if the app is not running (Windows or Linux).
  const url = findCustomProtocolUrlInArgv(process.argv);

  if (!url) {
    return;
  }
  logger.info(`Deep link launch from process.argv, URL: ${url}`);
  launchDeepLink(logger, windowsManager, url);
}

// We don't know the exact position of the URL is in argv. Chromium might inject its own arguments
// into argv. See https://www.electronjs.org/docs/latest/api/app#event-second-instance.
function findCustomProtocolUrlInArgv(argv: string[]) {
  return argv.find(arg => arg.startsWith(`${CUSTOM_PROTOCOL}://`));
}

function launchDeepLink(
  logger: Logger,
  windowsManager: WindowsManager,
  rawUrl: string
): void {
  const result = parseDeepLink(rawUrl);

  if (result.status === 'error') {
    let reason: string;
    switch (result.reason) {
      case 'unknown-protocol': {
        reason = `unknown protocol of the deep link ("${result.protocol}")`;
        break;
      }
      case 'unsupported-url': {
        reason = 'unsupported URL received';
        break;
      }
      case 'malformed-url': {
        reason = `malformed URL (${result.error.message})`;
        break;
      }
      default: {
        assertUnreachable(result);
      }
    }

    logger.error(`Skipping deep link launch, ${reason}`);
  }

  // Always pass the result to the frontend app so that the error can be shown to the user.
  // Otherwise the app would receive focus but nothing would be visible in the UI.
  windowsManager.launchDeepLink(result);
}

function showDialogWithError(title: string, unknownError: unknown) {
  const error = ensureError(unknownError);
  // V8 includes the error message in the stack, so there's no need to append stack to message.
  const content = error.stack || error.message;
  dialog.showErrorBox(title, content);
}
