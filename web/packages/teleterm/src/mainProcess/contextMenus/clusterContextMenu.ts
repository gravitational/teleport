import { ipcMain, ipcRenderer, Menu } from 'electron';
import {
  ClusterContextMenuEventChannel,
  ClusterContextMenuEventType,
  ClusterContextMenuOptions,
} from '../types';

type MainClusterContextMenuOptions = Pick<
  ClusterContextMenuOptions,
  'isClusterConnected'
>;

export function subscribeToClusterContextMenuEvent(): void {
  ipcMain.handle(
    ClusterContextMenuEventChannel,
    (event, options: MainClusterContextMenuOptions) => {
      return new Promise(resolve => {
        Menu.buildFromTemplate([
          {
            label: 'Refresh',
            enabled: options.isClusterConnected,
            click: () => resolve(ClusterContextMenuEventType.Refresh),
          },
          {
            type: 'separator',
          },
          {
            label: 'Login',
            enabled: !options.isClusterConnected,
            click: () => resolve(ClusterContextMenuEventType.Login),
          },
          {
            label: 'Logout',
            enabled: options.isClusterConnected,
            click: () => resolve(ClusterContextMenuEventType.Logout),
          },
          {
            type: 'separator',
          },
          {
            label: 'Remove',
            click: () => resolve(ClusterContextMenuEventType.Remove),
          },
        ]).popup({
          callback: () => resolve(undefined),
        });
      });
    }
  );
}

export async function openClusterContextMenu(
  options: ClusterContextMenuOptions
): Promise<void> {
  const mainOptions: MainClusterContextMenuOptions = {
    isClusterConnected: options.isClusterConnected,
  };
  const eventType = await ipcRenderer.invoke(
    ClusterContextMenuEventChannel,
    mainOptions
  );
  switch (eventType) {
    case ClusterContextMenuEventType.Refresh:
      return options.onRefresh();
    case ClusterContextMenuEventType.Login:
      return options.onLogin();
    case ClusterContextMenuEventType.Logout:
      return options.onLogout();
    case ClusterContextMenuEventType.Remove:
      return options.onRemove();
  }
}