import { ipcMain, ipcRenderer, Menu } from 'electron';

import { TerminalContextMenuEventChannel } from '../types';

export function subscribeToTerminalContextMenuEvent(): void {
  ipcMain.on(TerminalContextMenuEventChannel, () => {
    Menu.buildFromTemplate([
      {
        label: 'Copy',
        role: 'copy',
      },
      {
        label: 'Paste',
        role: 'paste',
      },
    ]).popup();
  });
}

export function openTerminalContextMenu(): void {
  return ipcRenderer.send(TerminalContextMenuEventChannel);
}
