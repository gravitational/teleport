import { BrowserWindow, Rectangle, screen } from 'electron';
import { getAssetPath } from 'teleterm/mainProcess/runtimeSettings';
import path from 'path';
import { FileStorage } from 'teleterm/services/fileStorage';
import { RuntimeSettings } from 'teleterm/mainProcess/types';
import theme from 'teleterm/ui/ThemeProvider/theme';

type WindowState = Rectangle;

export class WindowsManager {
  private storageKey = 'windowState';

  constructor(
    private fileStorage: FileStorage,
    private settings: RuntimeSettings
  ) {}

  createWindow(): void {
    const windowState = this.getWindowState();
    const window = new BrowserWindow({
      x: windowState.x,
      y: windowState.y,
      width: windowState.width,
      height: windowState.height,
      backgroundColor: theme.colors.primary.main,
      minWidth: 400,
      minHeight: 300,
      title: 'Teleport Terminal',
      icon: getAssetPath('icon.png'),
      webPreferences: {
        contextIsolation: true,
        nodeIntegration: false,
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
  }

  private saveWindowState(window: BrowserWindow): void {
    const windowState: WindowState = {
      ...window.getNormalBounds(),
    };

    this.fileStorage.put(this.storageKey, windowState);
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
