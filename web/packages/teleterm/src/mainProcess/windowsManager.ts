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
import * as url from 'node:url';

import {
  app,
  BrowserWindow,
  ipcMain,
  Menu,
  nativeTheme,
  Rectangle,
  screen,
} from 'electron';

import { DeepLinkParseResult } from 'teleterm/deepLinks';
import Logger from 'teleterm/logger';
import {
  RendererIpc,
  RuntimeSettings,
  WindowsManagerIpc,
} from 'teleterm/mainProcess/types';
import { FileStorage } from 'teleterm/services/fileStorage';
import { darkTheme, lightTheme } from 'teleterm/ui/ThemeProvider/theme';

type WindowState = Rectangle;

export class WindowsManager {
  private storageKey = 'windowState';
  private logger = new Logger('WindowsManager');
  private selectionContextMenu: Menu;
  private inputContextMenu: Menu;
  private window?: BrowserWindow;
  private frontendAppInit: {
    /**
     * The promise is resolved after the UI is fully initialized, that is the user has interacted
     * with the relevant modals during startup and is free to use the app.
     */
    promise: Promise<void>;
    resolve: () => void;
    reject: (error: Error) => void;
  };
  private readonly windowUrl: string;

  constructor(
    private fileStorage: FileStorage,
    private settings: RuntimeSettings
  ) {
    this.selectionContextMenu = Menu.buildFromTemplate([{ role: 'copy' }]);
    this.frontendAppInit = {
      promise: undefined,
      resolve: undefined,
      reject: undefined,
    };
    this.frontendAppInit.promise = new Promise((resolve, reject) => {
      this.frontendAppInit.resolve = resolve;
      this.frontendAppInit.reject = reject;
    });

    ipcMain.once(
      WindowsManagerIpc.SignalUserInterfaceReadiness,
      (event, args) => {
        if (args.success) {
          this.frontendAppInit.resolve();
        } else {
          this.frontendAppInit.reject(
            new Error('Encountered an error while initializing frontend app')
          );
        }
      }
    );

    this.inputContextMenu = Menu.buildFromTemplate([
      { role: 'undo' },
      { role: 'redo' },
      { type: 'separator' },
      { role: 'cut' },
      { role: 'copy' },
      { role: 'paste' },
    ]);
    this.windowUrl = getWindowUrl(settings.dev);
  }

  createWindow(): void {
    const activeTheme = nativeTheme.shouldUseDarkColors
      ? darkTheme
      : lightTheme;
    const windowState = this.getWindowState();
    const window = new BrowserWindow({
      x: windowState.x,
      y: windowState.y,
      width: windowState.width,
      height: windowState.height,
      backgroundColor: activeTheme.colors.levels.sunken,
      minWidth: 490,
      minHeight: 300,
      show: false,
      autoHideMenuBar: true,
      title: 'Teleport Connect',
      webPreferences: {
        devTools: this.settings.debug,
        webgl: false,
        enableWebSQL: false,
        safeDialogs: true,
        contextIsolation: true,
        nodeIntegration: false,
        sandbox: false,
        preload: path.join(__dirname, '../preload/index.js'),
      },
    });

    window.once('close', () => {
      this.saveWindowState(window);
      this.frontendAppInit.reject(
        new Error('Window was closed before frontend app got initialized')
      );
    });

    // shows the window when the DOM is ready, so we don't have a brief flash of a blank screen
    window.once('ready-to-show', window.show);
    window.loadURL(this.windowUrl);
    window.webContents.on('context-menu', (_, props) => {
      this.popupUniversalContextMenu(window, props);
    });

    nativeTheme.on('updated', () => {
      window.webContents.send(RendererIpc.NativeThemeUpdate, {
        shouldUseDarkColors: nativeTheme.shouldUseDarkColors,
      });
    });

    window.webContents.session.setPermissionRequestHandler(
      (webContents, permission, callback, details) => {
        if (details.requestingUrl !== this.windowUrl) {
          this.logger.error(
            `requestingUrl ${details.requestingUrl} does not match the window URL ${this.windowUrl}`
          );
          return callback(false);
        }

        if (
          permission === 'clipboard-sanitized-write' ||
          permission === 'clipboard-read'
        ) {
          return callback(true);
        }
        return callback(false);
      }
    );

    this.window = window;
  }

  /**
   * dispose exists as a cleanup function that the MainProcess can call during 'will-quit' event of
   * the Electron app.
   *
   * dispose doesn't have to close the window as that's typically done by Electron itself. It should
   * however clean up any other remaining resources.
   */
  dispose() {
    this.frontendAppInit.reject(
      new Error('Main process was closed before frontend app got initialized')
    );
  }

  async launchDeepLink(
    deepLinkParseResult: DeepLinkParseResult
  ): Promise<void> {
    try {
      await this.whenFrontendAppIsReady();
    } catch (error) {
      this.logger.error(
        `Could not send the deep link to the frontend app: ${error.message}`
      );
      return;
    }

    this.window.webContents.send(
      RendererIpc.DeepLinkLaunch,
      deepLinkParseResult
    );
  }

  /**
   * focusWindow is for situations where the app has privileges to do so, for example in a scenario
   * where the user attempts to launch a second instance of the app – the same process that the user
   * interacted with asks for its window to receive focus.
   */
  focusWindow(): void {
    if (!this.window) {
      return;
    }

    if (this.window.isMinimized()) {
      this.window.restore();
    }

    this.window.focus();
  }

  /**
   * forceFocusWindow if for situations where Connect wants to essentially steal focus.
   *
   * One example would be 3rd party apps interacting with resources exposed by Connect, e.g.
   * gateways. If the user attempts to make a connection through a gateway but the certs have
   * expired, Connect should receive focus and show an appropriate message to the user.
   */
  forceFocusWindow(): void {
    if (!this.window) {
      return;
    }

    if (this.window.isFocused()) {
      return;
    }

    // On Windows, app.focus() doesn't work the same as on the other platforms.
    // If the window is minimized, app.focus() will bring it to the front and give it focus.
    // If the window is not minimized but simply covered by other another window, app.focus() will
    // flash the icon of Connect in the task bar.
    // To make things even more complicated, the app behaves like that only when it is packaged.
    // When it is in dev mode, it seems to work correctly (it is brought to the front every time).
    //
    // Ideally, we'd like the not minimized window to receive focus too. We considered two
    // workarounds to bring focus to a window that's not minimized:
    //
    // * win.minimized() followed by win.focus() – this reportedly doesn't work anymore (see the
    // comment linked below) though it did work at the time of implementing forceFocusWindow.
    // Admittedly, this seems like a hack and does cause the window to first minimize and then show
    // up which feels weird.
    // * win.setAlwaysOnTop(true) followed by win.show() – this does bring the window to the top
    // but doesn't give it focus. Super awkward because Connect shows up over another app that you
    // were using, you start typing to fill out whatever form Connect has shown you. But your
    // keystrokes go to the app that the Connect window just covered.
    //
    // Since we cannot reliably steal focus, let's just not attempt to do it and instead defer to
    // flashing the icon in the task bar.
    //
    // https://github.com/electron/electron/issues/2867#issuecomment-1080573240
    //
    // I don't understand why calling app.focus() on a minimized window gives it focus in the
    // first place. In theory it shouldn't work, see the links below:
    //
    // https://stackoverflow.com/a/72620653/742872
    // https://devblogs.microsoft.com/oldnewthing/20090220-00/?p=19083
    // https://github.com/electron/electron/issues/2867#issuecomment-142480964
    // https://github.com/electron/electron/issues/2867#issuecomment-142511956

    app.dock?.bounce('informational');

    // app.focus() alone doesn't un-minimize the window if the window is minimized.
    if (this.window.isMinimized()) {
      this.window.restore();
    }
    app.focus({ steal: true });
  }

  getWindow() {
    return this.window;
  }

  /**
   * whenFrontendAppIsReady is made to resemble app.whenReady from Electron.
   * For now it is kept private just to signal that it's not used by any other class, but can be
   * made public if needed.
   *
   * The promise is resolved after the UI is fully initialized, that is the user has interacted with
   * the relevant modals during startup and is free to use the app.
   */
  private whenFrontendAppIsReady(): Promise<void> {
    return this.frontendAppInit.promise;
  }

  private saveWindowState(window: BrowserWindow): void {
    const windowState: WindowState = {
      ...window.getNormalBounds(),
    };

    this.fileStorage.put(this.storageKey, windowState);
  }

  private popupUniversalContextMenu(
    window: BrowserWindow,
    props: Electron.ContextMenuParams
  ): void {
    // Taken from https://github.com/electron/electron/issues/4068#issuecomment-274159726
    // selectall was removed from menus because it doesn't make sense in our context.
    const { selectionText, isEditable } = props;

    if (isEditable) {
      this.inputContextMenu.popup({ window });
    } else if (selectionText && selectionText.trim() !== '') {
      this.selectionContextMenu.popup({ window });
    }
  }

  private getWindowState(): WindowState {
    const windowState = this.fileStorage.get(this.storageKey) as WindowState;
    const getDefaults = () => ({
      height: 720,
      width: 1280,
      x: undefined,
      y: undefined,
    });

    if (!windowState) {
      return getDefaults();
    }

    const getPositionAndSize = () => {
      const displayBounds = screen.getDisplayNearestPoint({
        x: windowState.x,
        y: windowState.y,
      }).bounds;

      const isWindowWithinDisplayBounds =
        windowState.x >= displayBounds.x &&
        windowState.y >= displayBounds.y &&
        windowState.x + windowState.width <=
          displayBounds.x + displayBounds.width &&
        windowState.y + windowState.height <=
          displayBounds.y + displayBounds.height;

      if (isWindowWithinDisplayBounds) {
        return {
          x: windowState.x,
          y: windowState.y,
          width: windowState.width,
          height: windowState.height,
        };
      }
    };

    return {
      ...getDefaults(),
      ...getPositionAndSize(),
    };
  }
}

/**
 * Returns a URL that will be loaded by `BrowserWindow`.
 * This URL points either to a dev server or to index.html file
 * for the packaged app.
 * */
function getWindowUrl(isDev: boolean): string {
  if (isDev) {
    return 'http://localhost:8080/';
  }

  // The returned URL is percent-encoded.
  // It is important because `details.requestingUrl` (in `setPermissionRequestHandler`)
  // to which we match the URL is also percent-encoded.
  return url
    .pathToFileURL(
      path.resolve(app.getAppPath(), __dirname, '../renderer/index.html')
    )
    .toString();
}
