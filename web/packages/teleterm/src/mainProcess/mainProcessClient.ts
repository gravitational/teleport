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
import { AgentConfigFileClusterProperties } from 'teleterm/mainProcess/createAgentConfigFile';
import { RootClusterUri } from 'teleterm/ui/uri';

import { createConfigServiceClient } from '../services/config';

import { openTerminalContextMenu } from './contextMenus/terminalContextMenu';
import { openTabContextMenu } from './contextMenus/tabContextMenu';

import {
  MainProcessClient,
  ChildProcessAddresses,
  AgentProcessState,
} from './types';

export default function createMainProcessClient(): MainProcessClient {
  return {
    /*
     * Listeners for messages received by the renderer from the main process.
     */
    subscribeToNativeThemeUpdate: listener => {
      const onThemeChange = (_, value: { shouldUseDarkColors: boolean }) =>
        listener(value);
      const channel = 'renderer-native-theme-update';
      ipcRenderer.addListener(channel, onThemeChange);
      return {
        cleanup: () => ipcRenderer.removeListener(channel, onThemeChange),
      };
    },
    subscribeToAgentUpdate: (rootClusterUri, listener) => {
      const onChange = (
        _,
        eventRootClusterUri: RootClusterUri,
        eventState: AgentProcessState
      ) => {
        if (eventRootClusterUri === rootClusterUri) {
          listener(eventState);
        }
      };
      const channel = 'renderer-connect-my-computer-agent-update';
      ipcRenderer.addListener(channel, onChange);
      return {
        cleanup: () => ipcRenderer.removeListener(channel, onChange),
      };
    },

    /*
     * Messages sent from the renderer to the main process.
     */
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
    downloadAgent() {
      return ipcRenderer.invoke(
        'main-process-connect-my-computer-download-agent'
      );
    },
    createAgentConfigFile(clusterProperties: AgentConfigFileClusterProperties) {
      return ipcRenderer.invoke(
        'main-process-connect-my-computer-create-agent-config-file',
        clusterProperties
      );
    },
    isAgentConfigFileCreated(clusterProperties: {
      rootClusterUri: RootClusterUri;
    }) {
      return ipcRenderer.invoke(
        'main-process-connect-my-computer-is-agent-config-file-created',
        clusterProperties
      );
    },
    removeAgentDirectory(clusterProperties: {
      rootClusterUri: RootClusterUri;
    }) {
      return ipcRenderer.invoke(
        'main-process-connect-my-computer-remove-agent-directory',
        clusterProperties
      );
    },
    openAgentLogsDirectory(clusterProperties: {
      rootClusterUri: RootClusterUri;
    }) {
      return ipcRenderer.invoke(
        'main-process-open-agent-logs-directory',
        clusterProperties
      );
    },
    killAgent(clusterProperties: { rootClusterUri: RootClusterUri }) {
      return ipcRenderer.invoke(
        'main-process-connect-my-computer-kill-agent',
        clusterProperties
      );
    },
    runAgent(clusterProperties: { rootClusterUri: RootClusterUri }) {
      return ipcRenderer.invoke(
        'main-process-connect-my-computer-run-agent',
        clusterProperties
      );
    },
    getAgentState(clusterProperties: { rootClusterUri: RootClusterUri }) {
      return ipcRenderer.sendSync(
        'main-process-connect-my-computer-get-agent-state',
        clusterProperties
      );
    },
    getAgentLogs(clusterProperties: { rootClusterUri: RootClusterUri }) {
      return ipcRenderer.sendSync(
        'main-process-connect-my-computer-get-agent-logs',
        clusterProperties
      );
    },
  };
}
