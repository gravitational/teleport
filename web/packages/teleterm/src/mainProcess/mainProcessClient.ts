import { ipcRenderer } from 'electron';
import { openTerminalContextMenu } from './contextMenus/terminalContextMenu';
import { MainProcessClient } from './types';
import { createConfigServiceClient } from '../services/config';
import { openTabContextMenu } from './contextMenus/tabContextMenu';
import { createFileStorageClient } from 'teleterm/services/fileStorage';

export default function createMainProcessClient(): MainProcessClient {
  return {
    getRuntimeSettings() {
      return ipcRenderer.sendSync('main-process-get-runtime-settings');
    },
    openTerminalContextMenu,
    openTabContextMenu,
    configService: createConfigServiceClient(),
    fileStorage: createFileStorageClient()
  };
}
