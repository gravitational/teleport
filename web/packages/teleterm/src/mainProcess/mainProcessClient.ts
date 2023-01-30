import { ipcRenderer } from 'electron';

import { createFileStorageClient } from 'teleterm/services/fileStorage';

import { createConfigServiceClient } from '../services/config';

import { openTerminalContextMenu } from './contextMenus/terminalContextMenu';
import { MainProcessClient, ChildProcessAddresses } from './types';
import { openTabContextMenu } from './contextMenus/tabContextMenu';

export default function createMainProcessClient(): MainProcessClient {
  return {
    getRuntimeSettings() {
      return ipcRenderer.sendSync('main-process-get-runtime-settings');
    },
    getResolvedChildProcessAddresses(): Promise<ChildProcessAddresses> {
      return ipcRenderer.invoke(
        'main-process-get-resolved-child-process-addresses'
      );
    },
    showFileSaveDialog(filePath: string) {
      return ipcRenderer.invoke('main-process-show-file-save-dialog', filePath);
    },
    openTerminalContextMenu,
    openTabContextMenu,
    configService: createConfigServiceClient(),
    fileStorage: createFileStorageClient(),
    removeKubeConfig(options) {
      return ipcRenderer.invoke('main-process-remove-kube-config', options);
    },
    forceFocusWindow() {
      return ipcRenderer.invoke('main-process-force-focus-window');
    },
    symlinkTshMacOs() {
      return ipcRenderer.invoke('main-process-symlink-tsh-macos');
    },
    removeTshSymlinkMacOs() {
      return ipcRenderer.invoke('main-process-remove-tsh-symlink-macos');
    },
  };
}
