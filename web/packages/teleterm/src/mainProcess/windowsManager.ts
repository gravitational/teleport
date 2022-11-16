import path from 'path';

import { BrowserWindow, Menu, Rectangle, screen } from 'electron';

import { FileStorage } from 'teleterm/services/fileStorage';
import { RuntimeSettings } from 'teleterm/mainProcess/types';
import theme from 'teleterm/ui/ThemeProvider/theme';

type WindowState = Rectangle;

export class WindowsManager {
  private storageKey = 'windowState';
  private selectionContextMenu: Menu;
  private inputContextMenu: Menu;
  private window?: BrowserWindow;

  constructor(
    private fileStorage: FileStorage,
    private settings: RuntimeSettings
  ) {
    this.selectionContextMenu = Menu.buildFromTemplate([{ role: 'copy' }]);

    this.inputContextMenu = Menu.buildFromTemplate([
      { role: 'undo' },
      { role: 'redo' },
      { type: 'separator' },
      { role: 'cut' },
      { role: 'copy' },
      { role: 'paste' },
    ]);
  }

  createWindow(): void {
    const windowState = this.getWindowState();
    const window = new BrowserWindow({
      x: windowState.x,
      y: windowState.y,
      width: windowState.width,
      height: windowState.height,
      backgroundColor: theme.colors.primary.darker,
      minWidth: 400,
      minHeight: 300,
      autoHideMenuBar: true,
      title: 'Teleport Connect Preview',
      webPreferences: {
        devTools: this.settings.dev,
        webgl: false,
        enableWebSQL: false,
        safeDialogs: true,
        contextIsolation: true,
        nodeIntegration: false,
        sandbox: false,
        preload: path.join(__dirname, 'preload.js'),
      },
    });

    window.once('close', () => {
      this.saveWindowState(window);
    });

    if (this.settings.dev) {
      window.loadURL('https://localhost:8080');
    } else {
      window.loadFile(path.join(__dirname, '../renderer/index.html'));
    }

    window.webContents.on('context-menu', (_, props) => {
      this.popupUniversalContextMenu(window, props);
    });

    window.webContents.session.setPermissionRequestHandler(
      (webContents, permission, callback) => {
        // deny all permissions requests, we currently do not require any
        return callback(false);
      }
    );

    this.window = window;
  }

  focusWindow(): void {
    if (!this.window) {
      return;
    }

    if (this.window.isMinimized()) {
      this.window.restore();
    }

    this.window.focus();
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
    const windowState = this.fileStorage.get<WindowState>(this.storageKey);
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
