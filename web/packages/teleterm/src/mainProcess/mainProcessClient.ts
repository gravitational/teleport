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
import { CreateAgentConfigFileArgs } from 'teleterm/mainProcess/createAgentConfigFile';
import { RootClusterUri } from 'teleterm/ui/uri';

import { createConfigServiceClient } from '../services/config';

import { openTerminalContextMenu } from './contextMenus/terminalContextMenu';
import { openTabContextMenu } from './contextMenus/tabContextMenu';

import {
  MainProcessClient,
  ChildProcessAddresses,
  AgentProcessState,
  MainProcessIpc,
  RendererIpc,
  WindowsManagerIpc,
} from './types';

export default function createMainProcessClient(): MainProcessClient {
  return {
    /*
     * Listeners for messages received by the renderer from the main process.
     */
    subscribeToNativeThemeUpdate: listener => {
      const onThemeChange = (_, value: { shouldUseDarkColors: boolean }) =>
        listener(value);
      ipcRenderer.addListener(RendererIpc.NativeThemeUpdate, onThemeChange);
      return {
        cleanup: () =>
          ipcRenderer.removeListener(
            RendererIpc.NativeThemeUpdate,
            onThemeChange
          ),
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
      ipcRenderer.addListener(
        RendererIpc.ConnectMyComputerAgentUpdate,
        onChange
      );
      return {
        cleanup: () =>
          ipcRenderer.removeListener(
            RendererIpc.ConnectMyComputerAgentUpdate,
            onChange
          ),
      };
    },
    subscribeToDeepLinkLaunch: listener => {
      const ipcListener = (event, args) => {
        listener(args);
      };

      ipcRenderer.addListener(RendererIpc.DeepLinkLaunch, ipcListener);
      return {
        cleanup: () =>
          ipcRenderer.removeListener(RendererIpc.DeepLinkLaunch, ipcListener),
      };
    },

    /*
     * Messages sent from the renderer to the main process.
     */
    getRuntimeSettings() {
      return ipcRenderer.sendSync(MainProcessIpc.GetRuntimeSettings);
    },
    // TODO(ravicious): Convert the rest of IPC channels to use enums defined in types.ts such as
    // MainProcessIpc rather than hardcoded strings.
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
      return ipcRenderer.invoke(MainProcessIpc.DownloadConnectMyComputerAgent);
    },
    verifyAgent() {
      return ipcRenderer.invoke(MainProcessIpc.VerifyConnectMyComputerAgent);
    },
    createAgentConfigFile(args: CreateAgentConfigFileArgs) {
      return ipcRenderer.invoke(
        'main-process-connect-my-computer-create-agent-config-file',
        args
      );
    },
    isAgentConfigFileCreated(args: { rootClusterUri: RootClusterUri }) {
      return ipcRenderer.invoke(
        'main-process-connect-my-computer-is-agent-config-file-created',
        args
      );
    },
    removeAgentDirectory(args: { rootClusterUri: RootClusterUri }) {
      return ipcRenderer.invoke(
        'main-process-connect-my-computer-remove-agent-directory',
        args
      );
    },
    tryRemoveConnectMyComputerAgentBinary() {
      return ipcRenderer.invoke(
        MainProcessIpc.TryRemoveConnectMyComputerAgentBinary
      );
    },
    openAgentLogsDirectory(args: { rootClusterUri: RootClusterUri }) {
      return ipcRenderer.invoke('main-process-open-agent-logs-directory', args);
    },
    killAgent(args: { rootClusterUri: RootClusterUri }) {
      return ipcRenderer.invoke(
        'main-process-connect-my-computer-kill-agent',
        args
      );
    },
    runAgent(args: { rootClusterUri: RootClusterUri }) {
      return ipcRenderer.invoke(
        'main-process-connect-my-computer-run-agent',
        args
      );
    },
    getAgentState(args: { rootClusterUri: RootClusterUri }) {
      return ipcRenderer.sendSync(
        'main-process-connect-my-computer-get-agent-state',
        args
      );
    },
    getAgentLogs(args: { rootClusterUri: RootClusterUri }) {
      return ipcRenderer.sendSync(
        'main-process-connect-my-computer-get-agent-logs',
        args
      );
    },
    /**
     * Signals to the windows manager that the UI has been fully initialized, that is the user has
     * interacted with the relevant modals during startup and is free to use the app.
     */
    signalUserInterfaceReadiness(args: { success: boolean }) {
      ipcRenderer.send(WindowsManagerIpc.SignalUserInterfaceReadiness, args);
    },
  };
}
