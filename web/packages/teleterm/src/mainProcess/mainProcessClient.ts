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
    openConfigFile() {
      return ipcRenderer.invoke('main-process-open-config-file');
    },
    shouldUseDarkColors() {
      return ipcRenderer.sendSync('main-process-should-use-dark-colors');
    },
    subscribeToNativeThemeUpdate: listener => {
      const onThemeChange = (_, value: { shouldUseDarkColors: boolean }) =>
        listener(value);
      const channel = 'main-process-native-theme-update';
      ipcRenderer.addListener(channel, onThemeChange);
      return {
        cleanup: () => ipcRenderer.removeListener(channel, onThemeChange),
      };
    },
    subscribeToAgentStart: listener => {
      const onChange = (_, value: string) =>
        listener(value);
      const channel = 'agent-start';
      ipcRenderer.addListener(channel, onChange);
      return {
        cleanup: () => ipcRenderer.removeListener(channel, onChange),
      };
    },
  };
}
