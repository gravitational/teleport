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

import { ipcRenderer } from 'electron';

import { CreateAgentConfigFileArgs } from 'teleterm/mainProcess/createAgentConfigFile';
import { createFileStorageClient } from 'teleterm/services/fileStorage';
import { RootClusterUri } from 'teleterm/ui/uri';

import { createConfigServiceClient } from '../services/config';
import { openTabContextMenu } from './contextMenus/tabContextMenu';
import { openTerminalContextMenu } from './contextMenus/terminalContextMenu';
import {
  AgentProcessState,
  ChildProcessAddresses,
  MainProcessClient,
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
    saveTextToFile(args) {
      return ipcRenderer.invoke(MainProcessIpc.SaveTextToFile, args);
    },
    openTerminalContextMenu,
    openTabContextMenu,
    configService: createConfigServiceClient(),
    fileStorage: createFileStorageClient(),
    removeKubeConfig(options) {
      return ipcRenderer.invoke('main-process-remove-kube-config', options);
    },
    forceFocusWindow(args) {
      return ipcRenderer.invoke(MainProcessIpc.ForceFocusWindow, args);
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
    signalUserInterfaceReadiness(args: { success: boolean }) {
      ipcRenderer.send(WindowsManagerIpc.SignalUserInterfaceReadiness, args);
    },
    refreshClusterList() {
      ipcRenderer.send(MainProcessIpc.RefreshClusterList);
    },
    selectDirectoryForDesktopSession(args: {
      desktopUri: string;
      login: string;
    }) {
      return ipcRenderer.invoke(
        MainProcessIpc.SelectDirectoryForDesktopSession,
        args
      );
    },
  };
}
